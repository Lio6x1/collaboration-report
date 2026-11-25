"""教师实验：从逐段操作页按顺序复原源码，禁止接管学习者已有业务文件。"""
import argparse
import hashlib
import json
from pathlib import Path
import re
import sys

sys.dont_write_bytecode = True
COURSE = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(COURSE))
from course import load, normalized, target_path


def read_steps(entry):
    path = COURSE / entry['page'].replace('source/', 'write/')
    page = path.read_text(encoding='utf-8')
    pattern = (r'<!-- write-step: (\d+) bytes=(\d+):(\d+) -->\n````'
               + re.escape(entry['language']) + r'\n([\s\S]*?)^````\s*$')
    values = re.findall(pattern, page, re.M)
    if not values:
        raise ValueError('TEACHING_STEPS_MISSING: ' + entry['path'])
    chunks, previous = [], 0
    for number, (index, start, end, code) in enumerate(values, 1):
        encoded = code.encode('utf-8')
        if int(index) != number or int(start) != previous or int(end)-int(start) != len(encoded):
            raise ValueError('TEACHING_STEP_BOUNDARY_INVALID: '+entry['path']+' #'+index)
        chunks.append(encoded)
        previous = int(end)
    raw = b''.join(chunks)
    if hashlib.sha256(raw).hexdigest()!=entry['sha256']:
        raise ValueError('TEACHING_STEPS_SOURCE_DIFFERS: '+entry['path'])
    return chunks


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--target')
    parser.add_argument('--through', choices=['01','02','03','04','05'], default='05')
    parser.add_argument('--write', action='store_true')
    args = parser.parse_args()
    if args.write and not args.target:
        parser.error('--write requires --target')
    mapping = load()
    selected = [e for e in mapping['sources'] if e['phase'] <= args.through]
    restored = [(e,read_steps(e)) for e in selected]
    pending = restored
    if args.target:
        target = target_path(args.target)
        marker = target / '.learning-course.json'
        info = json.loads(marker.read_text(encoding='utf-8'))
        if info.get('target')!=str(target):
            raise ValueError('COURSE_TARGET_MARKER_MISMATCH')
        # 只接空业务支持目录或本工具自己的实验；不接 teacher-reference 或手写到一半的目录。
        exists = [e for e in mapping['sources'] if (target/e['path']).exists()]
        if exists and info.get('mode')!='TEACHER_STEPS':
            raise ValueError('LEARNER_WORK_MUST_NOT_BE_OVERWRITTEN')
        pending = []
        for entry,chunks in restored:
            output = target/entry['path']
            if output.exists():
                if output.is_symlink() or normalized(output)!=b''.join(chunks):
                    raise ValueError('TEACHER_STEP_FILE_CHANGED: '+entry['path'])
            else:
                pending.append((entry,chunks))
        if args.write:
            info['mode']='TEACHER_STEPS'
            marker.write_text(json.dumps(info,ensure_ascii=False,indent=2)+'\n',encoding='utf-8')
            for entry,chunks in pending:
                output=target/entry['path']
                output.parent.mkdir(parents=True,exist_ok=True)
                with output.open('xb') as writer:
                    for chunk in chunks:
                        writer.write(chunk)
    print(json.dumps({'state':'RESTORED' if args.write else 'PASS', 'through':args.through,
                      'files':len(restored),'steps':sum(len(c) for _,c in restored),
                      'new_files':len(pending),'pedagogy_acceptance':False},ensure_ascii=False))


if __name__=='__main__':
    main()
