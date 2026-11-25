"""教师材料完整性实验专用，不是学习者取用业务的主路线。"""
import argparse
import json
import sys
sys.dont_write_bytecode = True
from course import load, source, target_path, check_learned


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--target', required=True)
    parser.add_argument('--through', required=True, choices=['01', '02', '03', '04', '05'])
    parser.add_argument('--write', action='store_true')
    args = parser.parse_args()
    target, mapping = target_path(args.target), load()
    marker = target / '.learning-course.json'
    if not marker.is_file():
        raise ValueError('INITIALIZE_EMPTY_SUPPORT_FIRST')
    info = json.loads(marker.read_text(encoding='utf-8'))
    # 首次教师实验只接受完全没有手写源码的目录，以后仅续写剩余阶段。
    if info['mode'] != 'TEACHER_REFERENCE' and any((target / e['path']).exists() for e in mapping['sources']):
        raise ValueError('LEARNER_WORK_MUST_NOT_BE_OVERWRITTEN')
    if info['target'] != str(target):
        raise ValueError('COURSE_TARGET_MARKER_MISMATCH')
    pending = []
    for entry in mapping['sources']:
        if entry['phase'] > args.through:
            continue
        raw = source(entry)
        output = target / entry['path']
        if output.exists():
            from course import normalized, digest
            if output.is_symlink() or digest(normalized(output)) != entry['sha256']:
                raise ValueError('REFERENCE_FILE_CHANGED: ' + entry['path'])
        else:
            pending.append((output, raw))
    if not args.write:
        print(json.dumps({'state': 'PREVIEW', 'newReferenceFiles': len(pending)}))
        return
    info['mode'] = 'TEACHER_REFERENCE'
    marker.write_text(json.dumps(info, indent=2) + '\n', encoding='utf-8')
    for path, raw in pending:
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(raw)
    print(json.dumps(check_learned(target, mapping, args.through, exact=True)))


if __name__ == '__main__':
    try:
        main()
    except (ValueError, KeyError, OSError) as error:
        print('TEACHER_REFERENCE_FAILED: ' + str(error), file=sys.stderr)
        sys.exit(1)
