"""教师工具回归：解释必须绑定真实语法与作用域，不能凭字符串猜字段用途。"""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[3]


class OutlineEvidenceTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.temp = tempfile.TemporaryDirectory()
        path = Path(cls.temp.name) / 'example.go'
        path.write_text('''package lesson
// 登录前核验。
func Check(email string, count int) (bool, error) {
    normalized := normalize(email)
    if count < 1 { return false, ErrLimited }
    if err := save(normalized); err != nil { return false, err }
    return true, nil
}
func Other(email string) string { return email }
func Captured(email string) string {
    callback := func() string { return normalize(email) }
    return callback()
}
type Embedded struct { Credential }
type Callback func(string, uint64) (bool, error)
func Literal() string { return "a  b" }
''', encoding='utf-8')
        env = {**os.environ, 'GOWORK': 'off'}
        result = subprocess.run([os.environ.get('TEACHING_GO', 'go'), 'run', str(HERE / 'outline.go')],
                                input=json.dumps([str(path)]).encode(), capture_output=True, env=env, check=True)
        cls.decls = json.loads(result.stdout)[0]['decls']

    @classmethod
    def tearDownClass(cls):
        cls.temp.cleanup()

    def test_parameter_use_excludes_other_function_scope(self):
        check = self.decls[0]
        self.assertEqual(check['params'][0]['uses'], ['normalize(email)'])

    def test_return_slots_keep_actual_values_in_order(self):
        check = self.decls[0]
        self.assertEqual(check['results'][0]['uses'], ['false', 'true'])
        self.assertEqual(check['results'][1]['uses'], ['ErrLimited', 'err', 'nil'])

    def test_branch_fact_contains_condition_and_return(self):
        facts = self.decls[0]['facts']
        branch = next(f for f in facts if f['kind'] == 'condition' and f['text'] == 'count < 1')
        self.assertEqual(branch['outcome'], 'return false, ErrLimited')
        self.assertGreater(branch['start'], self.decls[0]['start'])

    def test_closure_captures_parameter_but_not_outer_return_slot(self):
        item = self.decls[2]
        self.assertIn('normalize(email)', item['params'][0]['uses'])
        self.assertEqual(item['results'][0]['uses'], ['callback()'])

    def test_embedded_fields_function_types_and_literal_spaces(self):
        self.assertEqual(self.decls[3]['fields'][0]['name'], 'Credential')
        self.assertEqual([p['type'] for p in self.decls[4]['params']], ['string', 'uint64'])
        self.assertEqual(self.decls[5]['results'][0]['uses'], ['"a  b"'])


class JavaScriptEvidenceTests(unittest.TestCase):
    def test_expression_return_void_and_constructor_are_distinct(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'returns.mjs'
            path.write_text('''export const normalize = (value) => value.trim();
export async function save(value) { await persist(value); }
export class Model { constructor(value) { this.value = value; } }
export const spaced = () => 'a  b';
''', encoding='utf-8')
            result = subprocess.run([os.environ.get('TEACHING_NODE', 'node'), str(HERE / 'outline.mjs')],
                                    input=json.dumps([str(path)]).encode(), capture_output=True, check=True)
        declarations = json.loads(result.stdout)[0]['decls']
        self.assertEqual(declarations[0]['results'][0]['uses'], ['value.trim()'])
        self.assertIn('undefined', declarations[1]['results'][0]['comment'])
        self.assertIn('实例', declarations[2]['members'][0]['results'][0]['comment'])
        self.assertEqual(declarations[2]['fields'][0]['name'], 'value')
        self.assertEqual(declarations[2]['fields'][0]['uses'], ['this.value = value'])
        self.assertEqual(declarations[3]['results'][0]['uses'], ["'a  b'"])

    def test_class_method_has_scoped_inputs_and_actual_return(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'example.mjs'
            path.write_text('''export class Room {
  save(sequence) {
    const ignore = (sequence) => other(sequence);
    const captured = () => remember(sequence);
    { const sequence = 10; unrelated(sequence); }
    if (sequence < 1) throw new Error('BAD_SEQUENCE');
    return this.persist(sequence);
  }
}
''', encoding='utf-8')
            result = subprocess.run([os.environ.get('TEACHING_NODE', 'node'), str(HERE / 'outline.mjs')],
                                    input=json.dumps([str(path)]).encode(), capture_output=True, check=True)
        member = json.loads(result.stdout)[0]['decls'][0]['members'][0]
        self.assertEqual(member['symbols'], ['Room.save'])
        self.assertIn('this.persist(sequence)', member['params'][0]['uses'])
        self.assertFalse(any('other(sequence)' in use for use in member['params'][0]['uses']))
        self.assertIn('remember(sequence)', member['params'][0]['uses'])
        self.assertFalse(any('unrelated(sequence)' in use for use in member['params'][0]['uses']))
        self.assertEqual(member['results'][0]['uses'], ['this.persist(sequence)'])


class SQLExplanationTests(unittest.TestCase):
    def test_checksum_is_column_and_foreign_key_is_not_index(self):
        from sql_explanations import sql_explanation
        text = sql_explanation('CREATE TABLE item (checksum BINARY(32) NOT NULL, FOREIGN KEY(id) REFERENCES parent(id))', {}, {'symbols': ['item']})
        self.assertIn('| <code>checksum</code> |', text)
        self.assertIn('引用', text)
        self.assertNotIn('访问路径', text)

    def test_split_keeps_type_and_quoted_commas_inside_column(self):
        from sql_explanations import table_parts
        values = table_parts("CREATE TABLE item (id BIGINT NOT NULL, amount DECIMAL(10,2), note VARCHAR(40) DEFAULT 'a,b', PRIMARY KEY(id), CHECK(amount IN (1,2))) ENGINE=InnoDB;")
        self.assertEqual(len(values), 5)
        self.assertEqual(values[2], "note VARCHAR(40) DEFAULT 'a,b'")
        self.assertEqual(values[3], 'PRIMARY KEY(id)')


class ExplanationPublishingTests(unittest.TestCase):
    def test_long_expression_is_bounded_and_cannot_become_markdown_link(self):
        from explanations import code
        text = code('client[method](input) ' + 'x' * 400)
        self.assertNotIn('[method](', text)
        self.assertLess(len(text), 400)
        self.assertIn('完整代码', text)

    def test_missing_or_stale_notes_and_placeholder_cannot_publish(self):
        from explanations import load_notes, require_complete
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'module.json'
            path.write_text(json.dumps({'files': {'a.go': {'purpose': '用途', 'why': '原因',
                'flow': ['调用顺序'], 'pitfalls': ['边界'], 'source_sha256': 'old'}}}), encoding='utf-8')
            with self.assertRaisesRegex(ValueError, 'OUTDATED'):
                load_notes(directory, [{'path': 'a.go', 'sha256': 'new'}])
            with self.assertRaisesRegex(ValueError, 'MISSING'):
                load_notes(directory, [{'path': 'b.go', 'sha256': 'new'}])
        with self.assertRaisesRegex(ValueError, 'EXPLANATION_GAP'):
            require_complete('a.go', '<!-- explanation-gap -->')

    def test_parameter_positions_and_unknown_field_are_checked(self):
        from explanations import validate_symbols, field_table
        declaration = {'kind': 'type', 'symbols': ['Callback'], 'params': [
            {'name': '(匿名/未命名)', 'type': 'string'}, {'name': '(匿名/未命名)', 'type': 'uint64'}]}
        detail = {'params': {'0': '权限来源键', '1': '来源代次'}}
        note = {'symbols': {'Callback': detail}}
        validate_symbols('a.go', note, [declaration])
        rendered = field_table('输入', declaration['params'], note, detail, 'params')
        self.assertLess(rendered.index('权限来源键'), rendered.index('来源代次'))
        note['symbols']['Callback']['fields'] = {'madeUp': '不存在'}
        with self.assertRaisesRegex(ValueError, 'FIELD_NOT_FOUND'):
            validate_symbols('a.go', note, [declaration])


if __name__ == '__main__':
    unittest.main()
