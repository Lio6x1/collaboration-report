import test from 'node:test';
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { readFileSync, writeFileSync, mkdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { root } from '../../scripts/runtime-lib.mjs';
import {
  organizationFixture,
  documentFixture,
  get,
  post,
  json,
  operation,
} from '../helpers/workspace.mjs';
import { openReviewWorkspace, writeDraftBody } from '../helpers/review.mjs';
import { unlockStructurePreview } from '../helpers/structure-previews.mjs';
import { binaryResourceGet } from '../helpers/resource.mjs';

const paragraph = (id, text = '') => ({
  type: 'paragraph',
  attrs: { id },
  ...(text ? { content: [{ type: 'text', text }] } : {}),
});
const cell = (id, colspan = 1) => ({
  type: 'table_cell',
  attrs: { id, colspan, rowspan: 1, colwidth: null },
  content: [paragraph(id + '-p')],
});

// 重现用户截图：合并的空表格、正文紧接长代码；不能让代码整块跳页或让空表收缩。
// 通过真实WSS保存和worker生成文件，PDF按页面文字与几何位置检查，不以CSS字面量代替排版。
test(
  'long highlighted code continues across PDF pages and empty merged tables keep content width',
  { timeout: 90000 },
  async () => {
    const owner = JSON.parse(readFileSync(process.env.COLLABORATION_TEST_ACCOUNT, 'utf8'));
    const signedIn = await post('/v1/account/login', {
      email: owner.email,
      password: owner.password,
    });
    assert.equal(signedIn.status, 200);
    owner.id = json(signedIn).user_id;
    owner.headers = {
      cookie: signedIn.headers['set-cookie'].map((value) => value.split(';')[0]).join('; '),
      'x-csrf-token': json(signedIn).csrf_token,
    };
    const f = await organizationFixture(0, owner);
    const d = await documentFixture(f);
    await unlockStructurePreview({ d, user: owner });
    const main = await openReviewWorkspace(f, d, owner, 'MAIN');
    const output = resolve(root, 'artifacts/reported-usability/export-pagination');
    mkdirSync(output, { recursive: true });
    const lines = Array.from(
      { length: 70 },
      (_, index) => `print("CODE_LINE_${String(index + 1).padStart(3, '0')}")`,
    );
    const longLine = 'LONG_LINE_START_' + 'abcdefghij'.repeat(20) + '_LONG_LINE_END';
    try {
      assert.equal(
        (
          await writeDraftBody(main, {
            type: 'doc',
            content: [
              {
                type: 'table',
                attrs: { id: 'empty-table' },
                content: [
                  {
                    type: 'table_row',
                    attrs: { id: 'merged-row' },
                    content: [cell('merged-cell', 2)],
                  },
                  {
                    type: 'table_row',
                    attrs: { id: 'normal-row' },
                    content: [cell('left-cell'), cell('right-cell')],
                  },
                ],
              },
              paragraph('before-code', 'BEFORE_CODE'),
              {
                type: 'code_block',
                attrs: { id: 'long-code', language: 'python' },
                content: [{ type: 'text', text: lines.join('\n') + `\nprint("${longLine}")` }],
              },
              paragraph('after-code', 'AFTER_CODE'),
            ],
          })
        ).type,
        'SAVED',
      );
      const view = json(await get(`/v1/documents/${d.id}/main`, owner.headers));
      for (const format of ['PDF', 'DOCX']) {
        const registered = await post(
          `/v1/documents/${d.id}/exports`,
          {
            operation_id: operation('pagination-' + format),
            target: 'MAIN',
            format,
            expected_manifest_id: view.manifest_id,
          },
          owner.headers,
        );
        assert.equal(registered.status, 202, registered.body);
        let job = json(registered);
        const deadline = Date.now() + 60000;
        while (['QUEUED', 'RUNNING'].includes(job.state) && Date.now() < deadline) {
          await new Promise((done) => setTimeout(done, 150));
          const polled = await get(`/v1/export-jobs/${job.id}`, owner.headers);
          assert.equal(polled.status, 200, polled.body);
          job = json(polled);
        }
        assert.equal(job.state, 'SUCCEEDED', JSON.stringify(job.last_error));
        const file = await binaryResourceGet(`/v1/export-jobs/${job.id}/download`, owner.headers);
        assert.equal(file.status, 200);
        writeFileSync(resolve(output, 'long-code.' + format.toLowerCase()), file.body);
      }
      const facts = JSON.parse(
        execFileSync(
          process.env.EXPORT_TEST_PYTHON ?? 'python',
          [
            '-c',
            `
import json, sys, zipfile, xml.etree.ElementTree as ET
import pdfplumber
with pdfplumber.open(sys.argv[1]) as pdf:
    first = pdf.pages[0]
    before = next(word for word in first.extract_words() if word['text'] == 'BEFORE_CODE')
    borders = [item for item in first.rects + first.lines if item['top'] < before['top'] and item['height'] < 2]
    width = max(item['x1'] for item in borders) - min(item['x0'] for item in borders) if borders else 0
    result = {'pages': len(pdf.pages), 'first': first.extract_text(), 'text': '\\n'.join(page.extract_text() for page in pdf.pages), 'table_width': width}
with zipfile.ZipFile(sys.argv[2]) as z:
    ns = {'w': 'http://schemas.openxmlformats.org/wordprocessingml/2006/main'}
    document = ET.fromstring(z.read('word/document.xml'))
    result['word_text'] = ''.join(item.text or '' for item in document.findall('.//w:t', ns))
    result['word_colspans'] = [item.attrib for item in document.findall('.//w:gridSpan', ns)]
print(json.dumps(result))
`,
            resolve(output, 'long-code.pdf'),
            resolve(output, 'long-code.docx'),
          ],
          { encoding: 'utf8', windowsHide: true },
        ),
      );
      writeFileSync(resolve(output, 'layout-facts.json'), JSON.stringify(facts, null, 2));
      assert.match(facts.first, /CODE_LINE_001/, '代码应从第一页剩余空间开始，不能整块移到下一页');
      assert.ok(facts.pages >= 2, '夹具须确实覆盖跨页代码');
      for (let index = 1; index <= 70; index++) {
        const marker = `CODE_LINE_${String(index).padStart(3, '0')}`;
        assert.equal(facts.text.split(marker).length - 1, 1, marker + '须完整且只出现一次');
        assert.ok(facts.word_text.includes(marker));
      }
      assert.ok(facts.text.includes('LONG_LINE_END'), '长代码行必须换行而非被页面裁切');
      assert.match(facts.text, /AFTER_CODE/);
      assert.ok(facts.table_width > 430, '空表格应铺满A4正文区域，而非按空内容收缩');
      assert.equal(facts.word_colspans.length, 1, 'Word仍保留可编辑的原生合并表格');
    } finally {
      main.close();
    }
  },
);
