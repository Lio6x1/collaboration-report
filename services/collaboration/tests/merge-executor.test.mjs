import test from 'node:test';
import assert from 'node:assert/strict';
import { MergeExecutor } from '../src/application/merge_executor.mjs';

const body = (value) => ({
  type: 'doc',
  content: [{ type: 'paragraph', attrs: { id: 'p' }, content: [{ type: 'text', text: value }] }],
});
// 实际富文本计算在worker线程执行。主事件循环定时任务必须继续，不能饿死来源gate和WSS。
test('heavy merge computation keeps the main event loop responsive', async () => {
  const text = '甲'.repeat(30000),
    executor = new MergeExecutor();
  let ticks = 0;
  const timer = setInterval(() => ticks++, 10);
  try {
    const result = await executor.compute({
      base: body(text),
      submitted: body('新' + text),
      main: body(text + '后'),
    });
    assert.ok(ticks >= 3, '纯计算期间主事件循环应继续运行');
    if (result.body) {
      assert.deepEqual(result.conflicts, []);
      assert.equal(result.body.content[0].content[0].text, '新' + text + '后');
    } else
      assert.ok(result.conflicts.some((conflict) => conflict.kind === 'MERGE_COMPLEXITY_LIMIT'));
    assert.equal(executor.active, 0);
  } finally {
    clearInterval(timer);
  }
});
