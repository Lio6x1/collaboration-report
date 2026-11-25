"""教师发布器：提取实际语法树，发布逐段写入页并核对手写讲义中的代码。"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
from explanations import load_notes, validate_symbols, bind_type_contexts, file_explanation, declaration_explanation, execution_facts, require_complete
from sql_explanations import sql_explanation

sys.dont_write_bytecode = True
COURSE = Path(__file__).resolve().parent.parent
ROOT = COURSE.parents[2]
MARKER = re.compile(r'<!-- lesson-code: ([^\n]+?) :: ([^\n]+?) -->\s*.*?<!-- /lesson-code -->', re.S)


def normalized(path):
    return (path.read_text(encoding='utf-8-sig').rstrip() + '\n').encode('utf-8')


def run_outline(command, paths):
    env = os.environ.copy()
    env['GOWORK'] = 'off'
    env['GOCACHE'] = str(ROOT / 'artifacts/teacher-led/go-cache')
    value = subprocess.run(command, input=json.dumps(paths).encode(), stdout=subprocess.PIPE,
                           stderr=subprocess.PIPE, cwd=ROOT, env=env, timeout=180)
    if value.returncode:
        raise RuntimeError(value.stderr.decode('utf-8', errors='replace'))
    return json.loads(value.stdout)


def sql_outline(source):
    # SQL 按整个分号行分组；DELIMITER 过程作为完整块，绝不把过程里的分号当结束。
    decls, start, position, delimiter = [], 0, 0, ';'
    lines = source.decode().splitlines(keepends=True)
    for line in lines:
        position += len(line.encode())
        stripped = line.strip()
        if stripped.upper().startswith('DELIMITER '):
            delimiter = stripped.split(None, 1)[1]
        if stripped.endswith(delimiter) and not stripped.startswith('--'):
            text = source[start:position].decode()
            substantive = '\n'.join(s for s in text.splitlines() if not s.strip().startswith('--')).strip()
            if substantive:
                names = re.findall(r'(?:CREATE TABLE|ALTER TABLE|CREATE DATABASE|USE|CREATE PROCEDURE)\s+`?([\w]+)', substantive, re.I)
                decls.append({'start': start, 'end': position, 'symbols': names or [f'SQL-{len(decls)+1}'],
                              'kind': 'sql', 'signature': substantive.splitlines()[0],
                              'comment': '\n'.join(s.strip('- ') for s in text.splitlines() if s.strip().startswith('--')),
                              'fields': [], 'params': [], 'results': [], 'cuts': []})
                start = position
    if start < len(source):
        decls.append({'start': start, 'end': len(source), 'symbols': ['SQL-tail'], 'kind': 'sql',
                      'signature': '迁移尾部', 'comment': '', 'fields': [], 'params': [], 'results': [], 'cuts': []})
    return decls


def segments(source, decls):
    """只有语法树确认的声明/语句起点可切割，所有片段按字节精确覆盖源码。"""
    nonimports = [d for d in decls if d['kind'] != 'import']
    starts = sorted({d['start'] for d in nonimports})
    boundaries = {0, len(source), *starts}
    descriptions = {d['start']: d for d in nonimports}
    for i, d in enumerate(nonimports):
        end = starts[starts.index(d['start'])+1] if starts.index(d['start'])+1 < len(starts) else len(source)
        last = d['start']
        for cut in sorted(set(d.get('cuts') or [])):
            if last < cut < end and source[last:cut].count(b'\n') >= 45:
                boundaries.add(cut)
                descriptions[cut] = {**d, 'continued': True}
                last = cut
    points = sorted(boundaries)
    chunks = []
    for start, end in zip(points, points[1:]):
        if start == end:
            continue
        value = source[start:end]
        # 空白由其相邻代码段携带，不产生没有学习动作的步骤。
        if chunks and not value.strip():
            chunks[-1]['end'] = end
            chunks[-1]['code'] += value.decode()
            continue
        chunks.append({'start': start, 'end': end, 'code': value.decode(),
                       'declaration': descriptions.get(start)})
    if ''.join(c['code'] for c in chunks).encode() != source:
        raise ValueError('TEACHING_SEGMENTS_DO_NOT_RECONSTRUCT_SOURCE')
    return chunks


def write_page(entry, unit, chunks, note):
    page = COURSE / entry['page'].replace('source/', 'write/')
    lecture = Path(os.path.relpath(COURSE / unit['page'], page.parent)).as_posix()
    answer = '../' + entry['page']
    language = entry['language']
    text = f'# 分步编写 `{entry["path"]}`\n\n先按 [{unit["id"]} · {unit["title"]}]({lecture}) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对]({answer})。所有相对文件路径都相对于自己的同一个学习工程。\n\n'
    text += file_explanation(note)
    text += '## 创建文件与录入规则\n\n'
    parent = Path(entry['path']).parent.as_posix()
    text += f'''```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path '{parent}' -Force | Out-Null
if ((Test-Path -LiteralPath '{entry['path']}') -and (Get-Item -LiteralPath '{entry['path']}').Length -gt 0) {{ throw '文件已有内容，先核对当前学习进度，勿覆盖' }}
if (-not (Test-Path -LiteralPath '{entry['path']}')) {{ New-Item -ItemType File -Path '{entry['path']}' | Out-Null }}
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

'''
    metadata = []
    for i, chunk in enumerate(chunks, 1):
        d = chunk['declaration']
        title = ('package 与 import / 文件头' if not d else '、'.join(d['symbols']))
        if d and d.get('continued'):
            title += '（接着实现）'
        line = source_line = sum(c['code'].count('\n') for c in chunks[:i-1])+1
        text += f'## 第 {i} 步：{title}\n\n'
        text += ('插入位置：空文件第 1 行。\n\n' if i == 1 else f'插入位置：第 {i-1} 步代码之后，紧接追加；完成稿约第 {line} 行。\n\n')
        if d and not d.get('continued'):
            if d.get('comment'):
                text += d['comment'].strip() + '\n\n'
            if d.get('signature'):
                text += '本步声明：`' + d['signature'].replace('`','\\`').replace('\n',' ') + '`。\n\n'
            text += declaration_explanation(d, note)
        elif d:
            text += '继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。\n\n'
        if d:
            if language == 'sql':
                text += sql_explanation(chunk['code'], note, d)
            for member in d.get('members') or []:
                if chunk['start'] <= member['start'] < chunk['end']:
                    text += '**本段进入 ' + '、'.join(member['symbols']) + '：**\n\n'
                    text += declaration_explanation(member, note)
            text += execution_facts(d, chunk['start'], chunk['end'])
        text += f'<!-- write-step: {i} bytes={chunk["start"]}:{chunk["end"]} -->\n````{language}\n{chunk["code"]}````\n\n'
        metadata.append({'step': i, 'start': chunk['start'], 'end': chunk['end'],
                         'symbol': d['symbols'] if d else [], 'line': source_line})
    text += '## 文件完成后的检查\n\n'
    if language == 'go':
        text += f"```powershell\ngofmt -w '{entry['path']}'\nif ($LASTEXITCODE -ne 0) {{ throw '本文件语法尚未写完整' }}\n```\n\n"
        text += 'gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。\n\n'
    elif language == 'javascript':
        text += f"```powershell\nnode --check '{entry['path']}'\nif ($LASTEXITCODE -ne 0) {{ throw '本文件 JavaScript 语法错误' }}\n```\n\n语法通过不证明 import 目标已经存在，也不执行数据库、WSS 或任务。依赖写齐后使用功能讲义的测试命令。\n\n"
    else:
        text += 'SQL 此时只编写，不在体验实例执行。所有编号迁移由 06 阶段在自己独立实例按顺序安装，不能为了检查语法删库或重复安装。\n\n'
    text += '若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](' + lecture + ')。\n'
    return page, text, metadata


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--write', action='store_true')
    parser.add_argument('--phase', choices=['01','02','03','04','05'])
    parser.add_argument('--go', default=shutil.which('go'))
    parser.add_argument('--node', default=shutil.which('node'))
    args = parser.parse_args()
    mapping = json.loads((COURSE / 'source-map.json').read_text(encoding='utf-8'))
    entries = [e for e in mapping['sources'] if not args.phase or e['phase'] == args.phase]
    notes = load_notes(COURSE/'maintenance/notes', entries)
    paths = [e['path'] for e in entries]
    units = []
    for file in sorted((COURSE/'lessons').glob('*-units.json')):
        units.extend(json.loads(file.read_text(encoding='utf-8')))
    assignments = {}
    for unit in units:
        for path in unit['files']:
            if path in assignments:
                raise ValueError('DUPLICATE_TEACHING_ASSIGNMENT: '+path)
            assignments[path] = unit
    missing = sorted(set(paths)-set(assignments))
    if missing:
        raise ValueError('TEACHING_UNIT_MISSING: '+', '.join(missing))
    outline = {}
    go_paths = [p for p in paths if p.endswith('.go')]
    js_paths = [p for p in paths if p.endswith(('.mjs','.ts'))]
    if go_paths:
        for result in run_outline([args.go,'run',str(COURSE/'maintenance/outline.go')],go_paths):
            outline[result['path']] = result['decls']
            notes[result['path']]['_imports'] = result['imports']
    if js_paths:
        for result in run_outline([args.node,str(COURSE/'maintenance/outline.mjs')],js_paths):
            outline[result['path']] = result['decls']
    for p in paths:
        if p.endswith('.sql'):
            outline[p] = sql_outline(normalized(ROOT/p))
    bind_type_contexts(outline, notes)
    differences, manifest, writes = [], [], []
    for e in entries:
        source = normalized(ROOT/e['path'])
        if hashlib.sha256(source).hexdigest()!=e['sha256']:
            raise ValueError('SOURCE_MAP_OUTDATED: '+e['path'])
        chunks = segments(source,outline[e['path']])
        validate_symbols(e['path'], notes[e['path']], outline[e['path']])
        page,text,steps = write_page(e,assignments[e['path']],chunks,notes[e['path']])
        require_complete(e['path'], text)
        manifest.append({'path':e['path'],'unit':assignments[e['path']]['id'],
                         'page':page.relative_to(COURSE).as_posix(),'sha256':e['sha256'],'steps':steps})
        if not page.exists() or page.read_text(encoding='utf-8')!=text:
            differences.append(page.relative_to(COURSE).as_posix())
            writes.append((page, text))
    # 手写讲义不由工具代写，只同步显式指定的真实声明。
    selected_pages = {u['page'] for u in units if any(p in paths for p in u['files'])}
    marker_count = 0
    for rel in sorted(selected_pages):
        page = COURSE/rel
        original = page.read_text(encoding='utf-8')
        def replace(match):
            nonlocal marker_count
            path,symbol = match.groups()
            if path not in outline:
                raise ValueError('LESSON_CODE_OUTSIDE_PHASE: '+path)
            found = [d for d in outline[path] if symbol in d['symbols']]
            if len(found)!=1:
                raise ValueError('LESSON_SYMBOL_NOT_UNIQUE: '+path+' :: '+symbol)
            d = found[0]; raw=normalized(ROOT/path)[d['start']:d['end']].decode()
            language='go' if path.endswith('.go') else 'sql' if path.endswith('.sql') else 'javascript'
            marker_count += 1
            return f'<!-- lesson-code: {path} :: {symbol} -->\n````{language}\n{raw}\n````\n<!-- /lesson-code -->'
        rendered = MARKER.sub(replace,original)
        if rendered!=original:
            differences.append(rel)
            writes.append((page, rendered))
    suffix = '-'+args.phase if args.phase else ''
    index = COURSE/f'write-map{suffix}.json'
    encoded = json.dumps({'scope':'TEACHER_LED_SEGMENTS','files':manifest},ensure_ascii=False,indent=2)+'\n'
    if not index.exists() or index.read_text(encoding='utf-8')!=encoded:
        differences.append(index.name)
        writes.append((index, encoded))
    # 全部源码、讲解和讲义声明检查后才落盘，避免中途缺口留下半套新页面。
    if args.write:
        for page, rendered in writes:
            page.parent.mkdir(parents=True, exist_ok=True)
            page.write_text(rendered, encoding='utf-8', newline='\n')
    print(json.dumps({'state':'WRITTEN' if args.write else 'PASS' if not differences else 'STALE',
                      'files':len(manifest),'steps':sum(len(m['steps']) for m in manifest),
                      'bound_code_declarations':marker_count,'difference_count':len(differences),
                      'differences':differences[:10]},ensure_ascii=False))
    if differences and not args.write: raise SystemExit(1)


if __name__=='__main__':
    main()
