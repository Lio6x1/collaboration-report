"""核验人工说明的源码绑定并列出尚无说明/使用证据的字段，不用覆盖数字冒称教学验收。"""
import argparse
import json
from pathlib import Path
import shutil

from explanations import load_notes, bind_type_contexts, validate_symbols, field_table, symbol_note
from teaching import COURSE, ROOT, normalized, sql_outline, run_outline
from sql_explanations import sql_explanation, table_parts


def inspect(go, node):
    mapping = json.loads((COURSE/'source-map.json').read_text(encoding='utf-8'))
    entries = mapping['sources']
    notes = load_notes(COURSE/'maintenance/notes', entries)
    files = run_outline([go, 'run', str(COURSE/'maintenance/outline.go')],
                        [e['path'] for e in entries if e['path'].endswith('.go')])
    files += run_outline([node, str(COURSE/'maintenance/outline.mjs')],
                         [e['path'] for e in entries if e['path'].endswith(('.mjs', '.ts'))])
    files += [{'path': e['path'], 'decls': sql_outline(normalized(ROOT/e['path']))}
              for e in entries if e['path'].endswith('.sql')]
    for file in files:
        notes[file['path']]['_imports'] = file.get('imports', {})
    outline = {file['path']: file['decls'] for file in files}
    bind_type_contexts(outline, notes)
    gaps, tables, errors = [], 0, []
    for entry in entries:
        path = entry['path']
        note = notes[path]
        try:
            validate_symbols(path, note, outline[path])
        except ValueError as error:
            errors.append(str(error))
        for declaration in outline[path]:
            if entry['language'] == 'sql':
                source = normalized(ROOT/path)[declaration['start']:declaration['end']].decode()
                tables += bool(table_parts(source))
                try:
                    sql_explanation(source, note, declaration)
                except ValueError as error:
                    errors.append(path + ': ' + str(error))
            for item in [declaration] + (declaration.get('members') or []):
                detail = symbol_note(note, item)
                for category in ('fields', 'params', 'results'):
                    for index, field in enumerate(item.get(category) or []):
                        selected = detail
                        if category == 'results':
                            selected = {**detail, 'results': {'0': detail.get('results', {}).get(str(index))}}
                        elif category == 'params':
                            values = detail.get('params', {})
                            selected = {**detail, 'params': {field['name']: values.get(field['name']) or values.get(str(index))}}
                        rendered = field_table('', [field], note, selected, category)
                        if '<!-- explanation-gap -->' in rendered:
                            gaps.append({'path': path, 'symbol': item['symbols'][0], 'category': category,
                                         'name': field['name'], 'type': field['type'], 'index': index})
    return {'state': 'PASS' if not gaps and not errors else 'INCOMPLETE', 'files': len(entries),
            'authored_symbols': sum(len(notes[e['path']].get('symbols', {})) for e in entries),
            'tables': tables, 'gap_count': len(gaps), 'gaps': gaps, 'errors': errors,
            'pedagogy_acceptance': False}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--go', default=shutil.which('go'))
    parser.add_argument('--node', default=shutil.which('node'))
    parser.add_argument('--report', help='将详细缺口保存到教师artifacts目录；不修改教程')
    args = parser.parse_args()
    result = inspect(args.go, args.node)
    if args.report:
        report = Path(args.report).resolve()
        if not report.is_relative_to((ROOT/'artifacts').resolve()):
            raise ValueError('REPORT_MUST_BE_IN_TEACHER_ARTIFACTS')
        report.parent.mkdir(parents=True, exist_ok=True)
        report.write_text(json.dumps(result, ensure_ascii=False, indent=2)+'\n', encoding='utf-8')
    print(json.dumps({**result, 'gaps': result['gaps'][:5]}, ensure_ascii=False))
    if result['state'] != 'PASS':
        raise SystemExit(1)


if __name__ == '__main__':
    main()
