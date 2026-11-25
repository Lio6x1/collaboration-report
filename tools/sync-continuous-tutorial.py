"""发布当前完整连续课程；公开支持与手写业务严格分开，不读取任何运行配置。"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
from zipfile import ZIP_DEFLATED, ZipFile, ZipInfo

ROOT = Path(__file__).resolve().parent.parent
DEST = ROOT / 'docs/tutorial/continuous'
PREFIXES = ('services/', 'packages/', 'contracts/', 'db/', 'scripts/', 'tools/',
            'tests/', 'web/', 'configs/', 'deploy/')
ROOT_FILES = {'.gitattributes', '.gitignore', '.prettierrc.json', 'go.work',
              'go.work.sum', 'package.json', 'package-lock.json'}
PHASES = {
    '01': ('公共底座、正文契约与数据库', '01-foundation.md'),
    '02': ('账号、验证码、组织与会话', '02-account.md'),
    '03': ('文档、权限与全部工作流', '03-document.md'),
    '04': ('协作、保存与多人结构操作', '04-collaboration.md'),
    '05': ('发布搜索投影与重建', '05-search.md'),
}


def public(path):
    return (path.startswith(PREFIXES) or path in ROOT_FILES) and not any(
        p in {'.runtime', 'node_modules', '.git', 'private', 'dist', '__pycache__'}
        for p in Path(path).parts)


def business(path):
    # AJV 生成入口由 scripts/generate-content.mjs 维护，不能作为手写业务。
    if path == 'packages/content/validate.mjs':
        return False
    if path.startswith('db/migrations/') and path.endswith('.sql'):
        return True
    return (path.startswith(('services/', 'packages/', 'contracts/content/'))
            and path.endswith(('.go', '.mjs', '.ts'))
            and not path.endswith(('_test.go', '.pb.go', '_grpc.pb.go'))
            and not any(p in {'tests', 'test', 'generated'} for p in Path(path).parts))


def phase(path):
    if path.startswith('services/account/'):
        return '02'
    if path.startswith('services/document/'):
        return '03'
    if path.startswith(('services/collaboration/', 'packages/content/')):
        return '04'
    if path.startswith('services/search/'):
        return '05'
    return '01'


def order(path):
    # 同一阶段先纯类型/独立依赖，再仓储与业务，最后适配和装配；一个包完成后才编译。
    layers = ('domain', 'client', 'exporter', 'storage', 'repository', 'logic',
              'application', 'handler', 'server', 'transport', 'worker', 'svc', 'bootstrap')
    parts = Path(path).parts
    rank = next((i for i, layer in enumerate(layers) if layer in parts), len(layers))
    return phase(path), rank, path


def normalized(path):
    return (path.read_text(encoding='utf-8-sig').rstrip() + '\n').encode('utf-8')


def digest(value):
    return hashlib.sha256(value).hexdigest()


def role(path):
    parts = Path(path).parts
    for layer, description in (
        ('domain', '定义模型、输入输出和纯规则；把状态与副作用分开。'),
        ('repository', '集中SQL和行锁；使用用例传入的同一事务，不隐式提交。'),
        ('logic', '编排用例、权限、事务及未知结果恢复；跨服务调用不能替代持久事实。'),
        ('application', '组织协作用例；持久保存、原操作恢复与房间并发有明确顺序。'),
        ('client', '适配跨服务或外部工具；保留期限、认证与错误语义。'),
        ('handler', '适配HTTP输入、真实身份和结果；调用业务层，避免在这里另写SQL。'),
        ('transport', '接入HTTP/RPC/WSS；只在业务确认后返回相应成功事实。'),
        ('svc', '创建依赖、注入构造函数；同一私有Store贯穿本服务。'),
        ('bootstrap', '注册路由、RPC及恢复器，连接入口和完整业务。'),
        ('worker', '领取持久任务并恢复原操作；控制并发、进程取消与关闭。'),
        ('exporter', '转换固定版本，管理进程和产物；不能重新读取变化中的正文。'),
        ('storage', '按明确授权操作资源；不将内部存储路径当作公开下载链接。'),
    ):
        if layer in parts:
            return description
    if path.endswith('.sql'):
        return '定义私有库、约束、索引或逐表授权；编号迁移只向前执行，不能修改已安装摘要。'
    return '本模块的公共规则、适配或进程入口；按阶段讲解核对调用者和接线位置。'


def support_kind(path):
    if path.startswith('web/'):
        return '现成前端'
    if path.endswith('_test.go') or '/tests/' in path or path.startswith('tests/'):
        return '已有测试与测试夹具'
    if path == 'packages/content/validate.mjs' or '/generated/' in path or path.endswith(('.pb.go', '_grpc.pb.go')):
        return '生成物（由原契约维护）'
    if path.startswith(('configs/', 'deploy/')):
        return '公开配置示例与部署样板'
    if path.startswith(('scripts/', 'tools/')):
        return '明确取用的开发、契约、环境和检查工具'
    return '依赖锁定、契约原文或辅助资料'


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--write', action='store_true')
    args = parser.parse_args()
    listed = subprocess.check_output(
        ['git', 'ls-files', '--cached', '--others', '--exclude-standard'], cwd=ROOT,
        encoding='utf-8').splitlines()
    # git 的 cached 清单也含已删除的旧工具；课程只发布工作树当前实际版本。
    paths = sorted({p for p in listed if public(p) and (ROOT / p).exists()})
    if any(not (ROOT / p).is_file() or (ROOT / p).is_symlink() for p in paths):
        raise ValueError('PUBLIC_SOURCE_MUST_BE_REGULAR_FILES')
    sources = sorted(filter(business, paths), key=order)
    supports = [p for p in paths if not business(p)]
    if not args.write:
        print(json.dumps({'state': 'PREVIEW', 'sources': len(sources), 'support': len(supports)}))
        return
    DEST.mkdir(parents=True, exist_ok=True)
    previous = json.loads((DEST / 'source-map.json').read_text(encoding='utf-8'))
    previous_pages = {e['path']: e['page'] for e in previous['sources']}
    next_number = max(int(Path(p).name.split('-')[0]) for p in previous_pages.values()) + 1
    entries = []
    for i, path in enumerate(sources, 1):
        value = normalized(ROOT / path)
        language = ('sql' if path.endswith('.sql') else 'go' if path.endswith('.go')
                    else 'typescript' if path.endswith('.ts') else 'javascript')
        lesson = phase(path)
        page = previous_pages.get(path)
        if page is None:
            page = f'source/{next_number:03d}-{Path(path).stem.replace("_", "-")}.md'
            next_number += 1
        symbols = re.findall(r'^(?:func\s+(?:\([^\n]*?\)\s+)?|type\s+|export\s+(?:async\s+)?(?:function|class)\s+)([A-Za-z_]\w*)', value.decode(), re.M)
        declarations = '、'.join('`' + s + '`' for s in dict.fromkeys(symbols)) or '完整定义见下方源码'
        output = DEST / page
        implementation = Path(os.path.relpath(ROOT / path, output.parent)).as_posix()
        text = f'''# {path}

阶段{lesson}：[功能讲义](../{PHASES[lesson][1]}) · [分步编写](../write/{Path(page).name}) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：{role(path)}

主要声明：{declarations}。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````{language}
{value.decode()}````

UTF-8/LF全文SHA-256：`{digest(value)}`。[参考实现]({implementation})仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
'''
        output.parent.mkdir(parents=True, exist_ok=True)
        output.write_text(text, encoding='utf-8', newline='\n')
        entries.append({'path': path, 'phase': lesson, 'page': page, 'language': language,
                        'sha256': digest(value)})
    support_entries = []
    with ZipFile(DEST / 'support.zip', 'w', compression=ZIP_DEFLATED) as archive:
        for path in supports:
            raw = (ROOT / path).read_bytes()
            info = ZipInfo(path, date_time=(2026, 10, 11, 0, 0, 0))
            info.compress_type = ZIP_DEFLATED
            archive.writestr(info, raw)
            support_entries.append({'path': path, 'sha256': digest(raw), 'kind': support_kind(path)})
    mapping = {'scope': 'CURRENT_CONTINUOUS_EMPTY_PROJECT', 'date': '2026-10-11',
               'sources': entries, 'support': support_entries,
               'support_zip_sha256': digest((DEST / 'support.zip').read_bytes())}
    (DEST / 'source-map.json').write_text(json.dumps(mapping, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
    for key, (title, lecture) in PHASES.items():
        chosen = [e for e in entries if e['phase'] == key]
        rows = '\n'.join(f'| {i} | `{e["path"]}` | [逐段编写](write/{Path(e["page"]).name}) | [完整答案]({e["page"]}) |'
                         for i, e in enumerate(chosen, 1))
        (DEST / f'{key}-files.md').write_text(f'''# 阶段{key}文件与答案：{title}

先读[本阶段功能讲义]({lecture})，按讲义安排的创建与实现顺序学习。本表是{len(chosen)}个文件的查找目录，表内序号只用于定位，不是教学顺序。逐段编写页与完整答案分开；已完成的阶段保留，不切换Git、不覆盖整棵服务树。

| 索引 | 学习工程内目标 | 分步操作 | 章末核对 |
| --- | --- | --- | --- |
{rows}
''', encoding='utf-8')
    rows = '\n'.join(f'| `{e["path"]}` | {e["kind"]} |' for e in support_entries)
    (DEST / 'support-index.md').write_text(f'''# 精确支持清单

共{len(support_entries)}项。由[空目录初始化](00-start.md)取用；压缩包不含{len(entries)}个亲写后端/SQL文件，不含运行目录、依赖安装目录、真实凭据或Git历史。清单和SHA在[source-map.json](source-map.json)。脚本与测试是明确取用的工具，业务源码按阶段亲写；生成契约仍由原契约和生成器维护。

| 路径 | 为什么可取用 |
| --- | --- |
{rows}
''', encoding='utf-8')
    print(json.dumps({'state': 'PUBLISHED', 'sources': len(entries), 'support': len(support_entries)}))


if __name__ == '__main__':
    main()
