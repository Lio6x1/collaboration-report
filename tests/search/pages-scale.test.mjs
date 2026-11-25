import assert from 'node:assert/strict';
import test from 'node:test';
import {
  partitionScaleIDs,
  scaleProvenance,
  boundedScaleGroups,
  searchPageScaleFixture,
} from '../helpers/search-page-scale.mjs';

test('纯规模规则：按原uint64字符串排序，不能把数值顺序当ES keyword顺序', () => {
  const ids = Array.from({ length: 2304 }, (_, i) => String(i + 1));
  ids.push('18446744073709551615');
  const { denied, allowed } = partitionScaleIDs(ids.reverse());
  assert.equal(denied.length, 1200);
  assert.equal(allowed.length, 1105);
  assert.deepEqual([...denied, ...allowed], [...ids].sort());
  assert.ok(denied.includes('18446744073709551615'));
  assert.throws(() => partitionScaleIDs([...ids.slice(1), ids[1]]));
  assert.throws(() => partitionScaleIDs([...ids.slice(1), '18446744073709551616']));
  assert.throws(() => partitionScaleIDs(ids.slice(1)));
});

test('纯规模规则：provenance属于当前自有unit/manifest/version，Unicode范围不按UTF16', () => {
  const row = { unit: '21', manifest: '22', version: '23' };
  const value = scaleProvenance(row, 'scale-owned', 'scale-block', '中😀');
  assert.equal(value.lineage_id, '21');
  assert.equal(value.base_manifest_id, '22');
  assert.equal(value.base_content_version_id, '23');
  assert.deepEqual(value.accepted_captures, []);
  assert.deepEqual(value.block_sources, [
    {
      block_id: 'scale-block',
      origin_content_version_id: '23',
      origin_block_id: 'scale-block',
      relation: 'INSERTED',
      source_range: { start: 0, end: 2 },
    },
  ]);
  assert.throws(() => scaleProvenance({ ...row, version: '0' }, 'scale-owned', 'block', 'x'));
});

test('纯规模规则：批次同时受行数和实际编码字节约束，单行超额必须拒绝', () => {
  const rows = ['a', 'b', 'c', 'd', 'e'];
  assert.deepEqual(
    boundedScaleGroups(rows, (part) => part.join(''), 3, 2),
    [['a', 'b'], ['c', 'd'], ['e']],
  );
  assert.deepEqual(
    boundedScaleGroups(['中', '😀'], (part) => part.join(''), 4, 100),
    [['中'], ['😀']],
  );
  assert.throws(() => boundedScaleGroups(['😀'], (part) => part.join(''), 3, 100));
});

// 此处2305项明确是自有SQL/ES样本；只有模板经过真实创建/MAIN保存/正式发布/消费。
// 验收对象是原Edge→Doc→Search入口，不以直接ES遍历或纯扫描器冒充产品页结果。
test(
  '实际规模搜索页：1200拒绝后1105合法，跨B100及终批REPLAY后缀完整无漏重',
  { timeout: 240000 },
  async (t) => {
    const f = await searchPageScaleFixture((facts) => t.diagnostic(JSON.stringify(facts)));
    let chainStarted;
    try {
      const { checkedSearchPage } = await import('../helpers/search-pages.mjs');
      const observedPage = async (index, cursor, replay = false) => {
        const started = Date.now();
        if (chainStarted === undefined) {
          chainStarted = started;
          t.diagnostic(JSON.stringify({ kind: 'SCALE_CHAIN_START', startedAtMs: started }));
        }
        // 每次只发原请求一次；日志只含固定页号/耗时，不打印加密cursor或敏感正文。
        t.diagnostic(JSON.stringify({ kind: 'SCALE_PAGE_START', index, replay }));
        try {
          const page = await checkedSearchPage(f, null, { limit: 92, cursor });
          t.diagnostic(
            JSON.stringify({
              kind: 'SCALE_PAGE_COMPLETE',
              index,
              replay,
              elapsedMs: Date.now() - started,
              count: page.items.length,
              has_more: page.has_more,
            }),
          );
          return page;
        } catch (error) {
          t.diagnostic(
            JSON.stringify({
              kind: 'SCALE_PAGE_FAILED',
              index,
              replay,
              elapsedMs: Date.now() - started,
            }),
          );
          throw error;
        }
      };
      const seen = new Set();
      let cursor;
      for (let index = 0; index < 13; index++) {
        const page = await observedPage(index + 1, cursor);
        if (index === 0) await f.checkChain();
        assert.equal(page.items.length, index === 12 ? 1 : 92, 'SCALE_PAGE_MUST_BE_FULL');
        assert.equal(page.has_more, index !== 12);
        assert.equal(page.next_cursor === null, index === 12);
        assert.deepEqual(
          page.items.map((item) => item.document_id),
          f.allowed.slice(index * 92, (index + 1) * 92),
          'SCALE_EXACT_ORDER_AND_SUFFIX',
        );
        for (const item of page.items) {
          assert.ok(!seen.has(item.document_id), 'SCALE_DUPLICATED_DOCUMENT');
          seen.add(item.document_id);
        }
        if (index === 12) {
          // 相同终批HTTP锚点再读一遍；随机AEAD外壳可以不同，业务项/EOF必须一致。
          const replay = await observedPage(13, cursor, true);
          assert.deepEqual(replay.items, page.items);
          assert.equal(replay.has_more, false);
          assert.equal(replay.next_cursor, null);
        }
        cursor = page.next_cursor;
      }
      assert.equal(seen.size, 1105);
      assert.deepEqual([...seen].sort(), [...f.allowed].sort());
      assert.ok(f.denied.every((id) => !seen.has(id)));
      await f.checkChain();
      t.diagnostic(
        'ACTUAL_EDGE_DOC_SEARCH_SCALE pages=13 returned=1105 rejected_prefix=1200 terminal_prefix=4 terminal_suffix=1',
      );
    } finally {
      if (chainStarted !== undefined) {
        t.diagnostic(
          JSON.stringify({
            kind: 'SCALE_CHAIN_END',
            startedAtMs: chainStarted,
            elapsedMs: Date.now() - chainStarted,
          }),
        );
      }
      await f.close();
    }
  },
);
