"""只读教学材料审计：覆盖、分段复原和PowerShell语法；不声称代替人工教学审查。"""
import json
from pathlib import Path
import re
import subprocess
import sys

sys.dont_write_bytecode = True
from rehearse import COURSE, load, read_steps


def powershell_blocks():
    blocks = []
    for page in sorted(COURSE.rglob('*.md')):
        fence, language, lines = None, None, []
        for line in page.read_text(encoding='utf-8').splitlines(keepends=True):
            value = line.rstrip('\r\n')
            if fence is None:
                opening = re.match(r'^(`{3,}|~{3,})([^\s]*)[^\n]*$', value)
                if opening:
                    fence, language = opening.groups()
                    lines = []
            elif re.fullmatch(re.escape(fence[0]) + '{' + str(len(fence)) + r',}[ \t]*', value):
                if language.lower() == 'powershell':
                    blocks.append({'page': page.relative_to(COURSE).as_posix(), 'code': ''.join(lines)})
                fence = None
            else:
                lines.append(line)
        if fence is not None:
            raise ValueError('UNCLOSED_MARKDOWN_FENCE: ' + str(page))
    return blocks


def main():
    mapping = load()
    expected = {e['path'] for e in mapping['sources']}
    units = []
    for path in sorted((COURSE / 'lessons').glob('*-units.json')):
        units.extend(json.loads(path.read_text(encoding='utf-8')))
    assigned = [p for u in units for p in u['files']]
    if len(assigned) != len(set(assigned)) or set(assigned) != expected:
        raise ValueError('TEACHING_ASSIGNMENTS_MUST_EXACTLY_COVER_HANDWRITTEN_FILES')
    by_path = {p: u for u in units for p in u['files']}
    steps = 0
    for entry in mapping['sources']:
        steps += len(read_steps(entry))
        page = COURSE / by_path[entry['path']]['page']
        relative = Path(entry['page']).name
        if '../../write/' + relative not in page.read_text(encoding='utf-8'):
            raise ValueError('LESSON_WRITE_ENTRY_MISSING: ' + entry['path'])
    if 'packages/content/validate.mjs' in expected:
        raise ValueError('GENERATED_AJV_MUST_NOT_BE_HANDWRITTEN')
    # 只解析文本，不执行教材代码，也不修改本机脚本执行策略。
    command = r'''
    $ErrorActionPreference='Stop'
    [Console]::OutputEncoding=New-Object System.Text.UTF8Encoding($false)
    $blocks=0
    $failures=@()
    foreach ($item in ([Console]::In.ReadToEnd() | ConvertFrom-Json)) {
            $tokens=$null
            $errors=$null
            $null=[Management.Automation.Language.Parser]::ParseInput($item.code,[ref]$tokens,[ref]$errors)
            $blocks++
            foreach ($errorItem in $errors) {
                $failures+=@{page=$item.page;line=$errorItem.Extent.StartLineNumber;error=$errorItem.Message}
            }
    }
    @{state=$(if($failures.Count){'FAIL'}else{'PASS'});powershellBlocks=$blocks;executed=$false;failures=$failures} | ConvertTo-Json -Depth 5 -Compress
    if ($failures.Count) { exit 1 }
    '''
    result = subprocess.run(['powershell.exe', '-NoProfile', '-NonInteractive', '-Command', command],
                            input=json.dumps(powershell_blocks()).encode(), capture_output=True, timeout=120)
    if result.returncode:
        raise ValueError(result.stdout.decode('utf-8', errors='replace') + result.stderr.decode('utf-8', errors='replace'))
    powershell = json.loads(result.stdout.decode('utf-8-sig'))
    print(json.dumps({'state': 'PASS', 'units': len(units), 'handwritten_files': len(expected),
                      'support_files': len(mapping['support']), 'exact_steps': steps,
                      'powershell': powershell, 'pedagogy_acceptance': False}, ensure_ascii=False))


if __name__ == '__main__':
    main()
