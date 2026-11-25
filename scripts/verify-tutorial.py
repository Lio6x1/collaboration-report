"""核验唯一当前课程的导航、公开支持及手写源码，不读取历史Git对象或运行配置。"""
import importlib.util
import json
from pathlib import Path
import re
import subprocess
import sys
from urllib.parse import unquote

sys.dont_write_bytecode = True
ROOT = Path(__file__).resolve().parent.parent
COURSE = ROOT / 'docs/tutorial/continuous'


def module(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value


def prose(text):
    """围栏可包含三反引号样例；仅同种且足够长的结束符关闭当前围栏。"""
    result, fence = [], None
    for line in text.splitlines():
        opening = re.match(r'^\s*(`{3,}|~{3,})', line)
        if fence is None:
            if opening:
                fence = opening.group(1)
            else:
                result.append(line)
        elif re.fullmatch(r'\s*' + re.escape(fence[0]) + '{' + str(len(fence)) + r',}\s*', line):
            fence = None
    if fence:
        raise ValueError('UNCLOSED_CODE_FENCE')
    return '\n'.join(result)


def check():
    course = module('current_course', COURSE / 'course.py')
    publisher = module('course_publisher', ROOT / 'tools/sync-continuous-tutorial.py')
    mapping = course.load()
    course.supports(mapping)
    for entry in mapping['sources']:
        course.source(entry)
        if course.digest(course.normalized(ROOT / entry['path'])) != entry['sha256']:
            raise ValueError('COURSE_SOURCE_STALE: ' + entry['path'])
        if '](' + entry['page'] + ')' not in (COURSE / (entry['phase'] + '-files.md')).read_text(encoding='utf-8'):
            raise ValueError('COURSE_SOURCE_INDEX_MISSING: ' + entry['path'])
    for entry in mapping['support']:
        if course.digest((ROOT / entry['path']).read_bytes()) != entry['sha256']:
            raise ValueError('COURSE_SUPPORT_STALE: ' + entry['path'])
    listed = subprocess.check_output(['git', 'ls-files', '--cached', '--others', '--exclude-standard'],
                                     cwd=ROOT, encoding='utf-8').splitlines()
    actual = {p for p in listed if publisher.public(p) and (ROOT / p).is_file()}
    if {e['path'] for e in mapping['sources'] + mapping['support']} != actual:
        raise ValueError('COURSE_PUBLIC_PATHS_DIFFER')
    pages = sorted(set([ROOT / 'README.md', ROOT / 'AGENTS.md'] +
                       [p for name in ('docs','configs','contracts','deploy','services','packages')
                        for p in (ROOT / name).rglob('*.md')
                        if not {'node_modules','.runtime','dist','__pycache__'} & set(p.parts)]))
    links, failures = 0, []
    for page in pages:
        text = page.read_text(encoding='utf-8-sig')
        if not text.strip():
            raise ValueError('EMPTY_DOCUMENT: ' + str(page))
        if page.is_relative_to(COURSE) and re.search(r'^\s*git\s+(clone|switch|restore|checkout)\b', text, re.M):
            raise ValueError('HISTORICAL_BUSINESS_IMPORT: ' + str(page))
        for target in re.findall(r'\[[^\]\n]+\]\(([^)\n]+)\)', prose(text)):
            target = target.strip().strip('<>')
            if re.match(r'^(?:[a-z]+:|#|/)', target, re.I):
                continue
            relative = unquote(target.split('#', 1)[0])
            if relative and not (page.parent / relative).resolve().exists():
                failures.append(f'{page.relative_to(ROOT)}: {target}')
            links += 1
    if failures:
        raise ValueError('BROKEN_DOCUMENT_LINKS:\n' + '\n'.join(failures))
    if any((ROOT / 'docs/tutorial' / name).exists() for name in ('backend','chapters','answers','notes')):
        raise ValueError('RETIRED_COURSE_STILL_PRESENT')
    return {'state':'PASS','scope':'CURRENT_DOCUMENTS_AND_PUBLIC_SOURCE', 'pages':len(pages),
            'links':links,'handwrittenFiles':len(mapping['sources']), 'supportFiles':len(mapping['support']),
            'servicesStarted':False,'pedagogyAcceptance':False}


if __name__ == '__main__':
    try:
        print(json.dumps(check(), ensure_ascii=False))
    except (OSError, ValueError, KeyError, subprocess.CalledProcessError) as error:
        print('TUTORIAL_CHECK_FAILED: ' + str(error), file=sys.stderr)
        sys.exit(1)
