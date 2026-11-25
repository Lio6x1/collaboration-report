"""连续课程材料的只读核验；不使用Git历史、不读取.runtime。"""
from pathlib import Path, PurePosixPath
import hashlib
import json
import os
import re
from zipfile import ZipFile

HERE = Path(__file__).resolve().parent


def digest(value):
    return hashlib.sha256(value).hexdigest()


def safe_relative(path):
    p = PurePosixPath(path)
    if (not p.parts or p.is_absolute() or '..' in p.parts or '\\' in path or ':' in path
            or any(s in {'.runtime', 'node_modules', '.git', 'private', 'dist', '__pycache__'} for s in p.parts)):
        raise ValueError('COURSE_PATH_INVALID')
    return p


def handwritten(path):
    if path == 'packages/content/validate.mjs':
        return False
    return ((path.startswith('db/migrations/') and path.endswith('.sql')) or
            (path.startswith(('services/', 'packages/', 'contracts/content/'))
             and path.endswith(('.go', '.mjs', '.ts'))
             and not path.endswith(('_test.go', '.pb.go', '_grpc.pb.go'))
             and not any(s in {'tests', 'test', 'generated'} for s in PurePosixPath(path).parts)))


def normalized(path):
    return (path.read_text(encoding='utf-8-sig').rstrip() + '\n').encode('utf-8')


def load():
    mapping = json.loads((HERE / 'source-map.json').read_text(encoding='utf-8'))
    if mapping['scope'] != 'CURRENT_CONTINUOUS_EMPTY_PROJECT':
        raise ValueError('COURSE_SCOPE_INVALID')
    all_entries = mapping['sources'] + mapping['support']
    paths = [e['path'] for e in all_entries]
    if len(paths) != len(set(paths)):
        raise ValueError('COURSE_PATH_DUPLICATED')
    for entry in all_entries:
        safe_relative(entry['path'])
    if any(not handwritten(e['path']) for e in mapping['sources']):
        raise ValueError('COURSE_SOURCE_CLASSIFICATION_INVALID')
    if any(handwritten(e['path']) for e in mapping['support']):
        raise ValueError('BUSINESS_IN_SUPPORT_REJECTED')
    return mapping


def source(entry):
    page = (HERE / entry['page']).resolve()
    if not page.is_relative_to(HERE / 'source'):
        raise ValueError('COURSE_SOURCE_PAGE_INVALID')
    matches = re.findall(r'^````' + entry['language'] + r'\n([\s\S]*?)^````\s*$',
                         page.read_text(encoding='utf-8'), re.M)
    if len(matches) != 1:
        raise ValueError('COURSE_SOURCE_FENCE_INVALID: ' + entry['path'])
    raw = matches[0].encode('utf-8')
    if digest(raw) != entry['sha256']:
        raise ValueError('COURSE_SOURCE_DIGEST_INVALID: ' + entry['path'])
    return raw


def supports(mapping):
    archive = HERE / 'support.zip'
    if digest(archive.read_bytes()) != mapping['support_zip_sha256']:
        raise ValueError('COURSE_SUPPORT_ARCHIVE_CHANGED')
    values = {}
    with ZipFile(archive) as bundle:
        names = bundle.namelist()
        if len(names) != len(set(names)) or set(names) != {e['path'] for e in mapping['support']}:
            raise ValueError('COURSE_SUPPORT_PATHS_DIFFER')
        for entry in mapping['support']:
            raw = bundle.read(entry['path'])
            if digest(raw) != entry['sha256']:
                raise ValueError('COURSE_SUPPORT_DIGEST_INVALID: ' + entry['path'])
            values[entry['path']] = raw
    return values


def target_path(value):
    original = Path(value).absolute()
    if any(p.is_symlink() or (hasattr(p, 'is_junction') and p.is_junction())
           for p in (original, *original.parents)):
        raise ValueError('COURSE_TARGET_SYMLINK_REJECTED')
    target = original.resolve()
    reference = HERE.parents[2]
    if target == reference or target.is_relative_to(reference):
        raise ValueError('COURSE_REFERENCE_TARGET_REJECTED')
    return target


def check_learned(target, mapping, through='05', exact=False):
    marker = target / '.learning-course.json'
    if not marker.is_file():
        raise ValueError('COURSE_TARGET_NOT_INITIALIZED')
    if json.loads(marker.read_text(encoding='utf-8'))['target'] != str(target):
        raise ValueError('COURSE_TARGET_MARKER_MISMATCH')
    chosen = [e for e in mapping['sources'] if e['phase'] <= through]
    missing, different = [], []
    for entry in chosen:
        path = target / entry['path']
        if not path.is_file() or path.is_symlink():
            missing.append(entry['path'])
        elif exact and digest(normalized(path)) != entry['sha256']:
            different.append(entry['path'])
    if missing or different:
        raise ValueError(json.dumps({'missing': missing, 'different': different}, ensure_ascii=False))
    if exact:
        expected = {e['path'] for e in chosen + mapping['support']}
        actual = set()
        for name in ('services', 'packages', 'contracts', 'db', 'scripts', 'tools', 'tests', 'web', 'configs', 'deploy'):
            for folder, dirs, files in os.walk(target / name, followlinks=False):
                dirs[:] = [d for d in dirs if d not in {'.runtime', 'node_modules', '.git', 'private', 'dist', '__pycache__'}]
                for file in files:
                    actual.add((Path(folder) / file).relative_to(target).as_posix())
        for name in ('.gitattributes', '.gitignore', '.prettierrc.json', 'go.work', 'go.work.sum', 'package.json', 'package-lock.json'):
            if (target / name).is_file():
                actual.add(name)
        if actual != expected:
            raise ValueError('COURSE_PUBLIC_PATHS_DIFFER: extra=' + str(len(actual - expected))
                             + ', missing=' + str(len(expected - actual)))
        # 精确对照阶段不能偷带后面的手写业务，或漏核明确支持；运行产物不在公开清单内。
        for entry in mapping['sources']:
            if entry['phase'] > through and (target / entry['path']).exists():
                raise ValueError('FUTURE_BUSINESS_PRESENT: ' + entry['path'])
        for entry in mapping['support']:
            path = target / entry['path']
            if not path.is_file() or path.is_symlink() or digest(path.read_bytes()) != entry['sha256']:
                raise ValueError('COURSE_INSTALLED_SUPPORT_DIFFERS: ' + entry['path'])
    return {'state': 'PASS', 'through': through, 'writtenFiles': len(chosen),
            'exact': exact, 'businessAcceptance': False}
