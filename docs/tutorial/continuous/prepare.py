"""向空学习目录安装明确支持；绝不写业务源码。默认只预检。"""
import argparse
import json
import sys
sys.dont_write_bytecode = True
from course import load, supports, target_path, check_learned


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--target', required=True)
    parser.add_argument('--write', action='store_true')
    parser.add_argument('--check', choices=['01', '02', '03', '04', '05'])
    parser.add_argument('--exact', action='store_true')
    args = parser.parse_args()
    target, mapping = target_path(args.target), load()
    if args.check:
        if args.write:
            raise ValueError('CHECK_IS_READ_ONLY')
        print(json.dumps(check_learned(target, mapping, args.check, args.exact)))
        return
    if target.exists() and (not target.is_dir() or any(target.iterdir())):
        raise ValueError('COURSE_TARGET_MUST_BE_EMPTY')
    payload = supports(mapping)
    if not args.write:
        print(json.dumps({'state': 'PREVIEW', 'support': len(payload), 'businessWritten': 0}))
        return
    target.mkdir(parents=True, exist_ok=True)
    # 标记先落盘：若磁盘写入中断，不把非空目录当作可再次覆盖的空目录。
    (target / '.learning-course.json').write_text(json.dumps({'target': str(target),
        'mode': 'HANDWRITE', 'scope': mapping['scope']}, indent=2) + '\n', encoding='utf-8')
    for path, raw in payload.items():
        output = target / path
        output.parent.mkdir(parents=True, exist_ok=True)
        output.write_bytes(raw)
    print(json.dumps({'state': 'PREPARED', 'support': len(payload), 'businessWritten': 0,
                      'servicesStarted': False}))


if __name__ == '__main__':
    try:
        main()
    except (ValueError, KeyError, OSError) as error:
        print('COURSE_PREPARE_FAILED: ' + str(error), file=sys.stderr)
        sys.exit(1)
