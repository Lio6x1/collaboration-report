"""保护学习成果和业务/支持边界；不启动服务，不连接真实实例。"""
import copy
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch
sys.dont_write_bytecode = True
import course


class CourseBoundaryTests(unittest.TestCase):
    def test_current_material_classification_and_full_source_fences(self):
        mapping = course.load()
        course.supports(mapping)
        for entry in mapping['sources']:
            course.source(entry)
        self.assertFalse(course.handwritten('packages/content/validate.mjs'))
        self.assertIn('packages/content/validate.mjs', {e['path'] for e in mapping['support']})

    def test_manifest_cannot_hide_handwritten_business_in_support(self):
        mapping = copy.deepcopy(course.load())
        mapping['support'].append(mapping['sources'].pop())
        with patch.object(Path, 'read_text', return_value=json.dumps(mapping)):
            with self.assertRaisesRegex(ValueError, 'BUSINESS_IN_SUPPORT_REJECTED'):
                course.load()

    def test_initializer_installs_support_without_handwritten_business(self):
        with tempfile.TemporaryDirectory(prefix='course-support-boundary-') as folder:
            target = Path(folder).resolve() / 'empty-learning-project'
            result = subprocess.run([sys.executable, str(course.HERE / 'prepare.py'),
                                     '--target', str(target), '--write'], capture_output=True)
            self.assertEqual(result.returncode, 0, result.stderr.decode(errors='replace'))
            output = json.loads(result.stdout)
            mapping = course.load()
            self.assertEqual(output['support'], len(mapping['support']))
            self.assertEqual(output['businessWritten'], 0)
            self.assertFalse(output['servicesStarted'])
            self.assertTrue((target / 'packages/content/validate.mjs').is_file())
            self.assertFalse(any((target / e['path']).exists() for e in mapping['sources']))

    def test_escaping_paths_and_reference_target_are_rejected(self):
        for path in ('../outside', '/outside', 'C:/outside', 'web/../../outside', '.runtime/key', 'web\\file'):
            with self.subTest(path=path), self.assertRaises(ValueError):
                course.safe_relative(path)
        with self.assertRaisesRegex(ValueError, 'REFERENCE_TARGET_REJECTED'):
            course.target_path(str(course.HERE.parents[2]))

    def test_initializer_refuses_existing_learning_work_without_overwriting(self):
        with tempfile.TemporaryDirectory(prefix='course-boundary-') as folder:
            target = Path(folder)
            original = target / 'my-work.txt'
            original.write_text('我的成果', encoding='utf-8')
            result = subprocess.run([sys.executable, str(course.HERE / 'prepare.py'),
                                     '--target', str(target), '--write'], capture_output=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn(b'COURSE_TARGET_MUST_BE_EMPTY', result.stderr)
            self.assertEqual(original.read_text(encoding='utf-8'), '我的成果')
            self.assertEqual(list(target.iterdir()), [original])

    def test_teacher_reference_refuses_to_replace_handwritten_source(self):
        with tempfile.TemporaryDirectory(prefix='teacher-boundary-') as folder:
            target = Path(folder).resolve()
            (target / '.learning-course.json').write_text(json.dumps({'target': str(target),
                'mode': 'HANDWRITE'}), encoding='utf-8')
            entry = course.load()['sources'][0]
            original = target / entry['path']
            original.parent.mkdir(parents=True)
            original.write_text('自己的实现', encoding='utf-8')
            result = subprocess.run([sys.executable, str(course.HERE / 'teacher-reference.py'),
                                     '--target', str(target), '--through', '01', '--write'], capture_output=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn(b'LEARNER_WORK_MUST_NOT_BE_OVERWRITTEN', result.stderr)
            self.assertEqual(original.read_text(encoding='utf-8'), '自己的实现')


if __name__ == '__main__':
    unittest.main()
