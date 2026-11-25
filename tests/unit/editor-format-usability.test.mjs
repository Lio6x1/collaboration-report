import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { pathToFileURL } from 'node:url';
import ts from 'typescript';
import { EditorState, TextSelection } from 'prosemirror-state';
import { schema, fromPM, normalizeContent } from '../../packages/content/index.mjs';
import { prosemirrorJSONToYDoc, yDocToProsemirrorJSON } from 'y-prosemirror';

const require = createRequire(import.meta.url);
const httpURL =
  'data:text/javascript;base64,' +
  Buffer.from(
    ts.transpileModule(
      readFileSync(new URL('../../web/src/lib/http.ts', import.meta.url), 'utf8').replace(
        "from 'vue'",
        `from '${pathToFileURL(require.resolve('vue')).href}'`,
      ),
      { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022 } },
    ).outputText,
  ).toString('base64');
const source = readFileSync(
  new URL('../../web/src/features/editor/commands.ts', import.meta.url),
  'utf8',
)
  .replace("from '../../lib/http'", `from '${httpURL}'`)
  .replace(
    /from '(prosemirror-[^']+)'/g,
    (_, name) => `from '${pathToFileURL(require.resolve(name)).href}'`,
  )
  .replaceAll(
    "'../../../../packages/content/index.mjs'",
    `'${new URL('../../packages/content/index.mjs', import.meta.url).href}'`,
  )
  .replaceAll(
    "'../../../../packages/content/markdown.mjs'",
    `'${new URL('../../packages/content/markdown.mjs', import.meta.url).href}'`,
  );
const commands = await import(
  'data:text/javascript;base64,' +
    Buffer.from(
      ts.transpileModule(source, {
        compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022 },
      }).outputText,
    ).toString('base64')
);
const codeSource = readFileSync(
  new URL('../../web/src/lib/content/code.ts', import.meta.url),
  'utf8',
)
  .replace("import './code.css';", '')
  .replace(
    /from '(highlight\.js[^']*|prosemirror-[^']+)'/g,
    (_, name) => `from '${pathToFileURL(require.resolve(name)).href}'`,
  );
const code = await import(
  'data:text/javascript;base64,' +
    Buffer.from(
      ts.transpileModule(codeSource, {
        compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022 },
      }).outputText,
    ).toString('base64')
);
function editor(editable = true) {
  const doc = schema.nodes.doc.create(null, [
    schema.nodes.heading.create({ id: 'existing-block', level: 2 }, schema.text('正文格式')),
  ]);
  let state = EditorState.create({ schema, doc });
  state = state.apply(state.tr.setSelection(TextSelection.create(state.doc, 1, 5)));
  return {
    editable,
    get state() {
      return state;
    },
    dispatch(tr) {
      state = state.apply(tr);
    },
    focus() {},
  };
}

// 恢复正文段落必须沿原稳定块身份保存，否则后续审核会误识别为删旧建新。
test('the normal paragraph action preserves heading text and the stable block identity', () => {
  const view = editor();
  commands.paragraph(view);
  const body = normalizeContent(fromPM(view.state.doc.toJSON()), { workspaceKind: 'BRANCH' });
  assert.equal(body.content[0].type, 'paragraph');
  assert.equal(body.content[0].attrs.id, 'existing-block');
  assert.equal(body.content[0].content[0].text, '正文格式');
});

// PM的editable仅限制DOM输入；工具栏主动dispatch也必须遵守加载/过期/候选禁写状态。
test('format and paragraph toolbar commands cannot write into a noneditable view', () => {
  const view = editor(false);
  const before = view.state.doc.toJSON();
  commands.format(view, 'strong');
  commands.heading(view);
  assert.deepEqual(view.state.doc.toJSON(), before);
});

// 按钮状态必须来自真实选区；切换段落、对齐和缩进后，互斥选项只能选中一个。
test('toolbar selection follows the actual block and character formatting', () => {
  const view = editor();
  assert.equal(commands.toolbarState(view.state).block, 'heading');
  assert.equal(commands.toolbarState(view.state).alignment, 'left');
  assert.equal(commands.toolbarState(view.state).indent, false);
  commands.paragraph(view);
  commands.paragraphAlignment(view, 'center');
  commands.firstLineIndent(view, true);
  commands.format(view, 'strong');
  let selected = commands.toolbarState(view.state);
  assert.equal(selected.block, 'paragraph');
  assert.equal(selected.alignment, 'center');
  assert.equal(selected.indent, true);
  assert.equal(selected.marks.strong, true);
  commands.firstLineIndent(view, false);
  commands.paragraphAlignment(view, 'right');
  selected = commands.toolbarState(view.state);
  assert.equal(selected.indent, false);
  assert.equal(selected.alignment, 'right');
});

// 多段格式不同时明确显示混合，不能把任何一个选项冒充当前统一状态。
test('mixed paragraph selections have no single active alignment or indentation', () => {
  const doc = schema.nodes.doc.create(null, [
    schema.nodes.paragraph.create(
      { id: 'one', align: 'left', firstLineIndent: false },
      schema.text('左侧'),
    ),
    schema.nodes.heading.create(
      { id: 'two', level: 2, align: 'right', firstLineIndent: true },
      schema.text('右侧'),
    ),
  ]);
  let state = EditorState.create({ schema, doc });
  state = state.apply(state.tr.setSelection(TextSelection.create(doc, 1, doc.content.size - 1)));
  const selected = commands.toolbarState(state);
  assert.equal(selected.block, 'mixed');
  assert.equal(selected.alignment, 'mixed');
  assert.equal(selected.indent, 'mixed');
});

// 代码入口与语言切换必须沿原块身份跨PM/Yjs/content-v1往返，不能把高亮写进正文。
test('code block entry, language changes and paragraph recovery preserve stable content', () => {
  const view = editor();
  commands.format(view, 'strong');
  commands.codeBlock(view);
  assert.equal(commands.toolbarState(view.state).block, 'code_block');
  assert.equal(commands.toolbarState(view.state).alignment, null);
  code.setCodeLanguage(view, 0, 'go');
  const doc = prosemirrorJSONToYDoc(schema, view.state.doc.toJSON());
  try {
    const saved = normalizeContent(fromPM(yDocToProsemirrorJSON(doc)), { workspaceKind: 'BRANCH' });
    assert.equal(saved.content[0].type, 'code_block');
    assert.equal(saved.content[0].attrs.id, 'existing-block');
    assert.equal(saved.content[0].attrs.language, 'go');
    assert.deepEqual(saved.content[0].content, [{ type: 'text', text: '正文格式' }]);
  } finally {
    doc.destroy();
  }
  commands.paragraph(view);
  assert.equal(view.state.doc.firstChild.type.name, 'paragraph');
  assert.equal(view.state.doc.firstChild.attrs.id, 'existing-block');
  assert.equal(view.state.doc.textContent, '正文格式');
});

// 公式不能被文本块兼容性清理静默删掉；此时在含公式段落后开一个新的代码块。
test('code block entry preserves inline atoms and gives the new block its own identity', () => {
  const original = schema.nodes.paragraph.create({ id: 'formula-paragraph' }, [
    schema.text('表达式'),
    schema.nodes.formula.create({ latex: 'x^2' }),
  ]);
  let state = EditorState.create({ schema, doc: schema.nodes.doc.create(null, original) });
  state = state.apply(state.tr.setSelection(TextSelection.create(state.doc, 1)));
  const view = {
    editable: true,
    get state() {
      return state;
    },
    dispatch(tr) {
      state = state.apply(tr);
    },
    focus() {},
  };
  commands.codeBlock(view);
  assert.deepEqual(state.doc.firstChild.toJSON(), original.toJSON());
  assert.equal(state.doc.child(1).type.name, 'code_block');
  assert.ok(state.doc.child(1).attrs.id);
  assert.notEqual(state.doc.child(1).attrs.id, 'formula-paragraph');
  assert.equal(commands.toolbarState(state).block, 'code_block');
});

// 列表项必须以段落开头，不能用代码替换首段破坏结构；在该段后插入代码可保留原列表身份。
test('code block entry preserves the required first paragraph of a list item', () => {
  const paragraph = schema.nodes.paragraph.create(
    { id: 'list-paragraph' },
    schema.text('列表正文'),
  );
  const item = schema.nodes.list_item.create({ id: 'list-item' }, paragraph);
  const list = schema.nodes.bullet_list.create({ id: 'list' }, item);
  let state = EditorState.create({ schema, doc: schema.nodes.doc.create(null, list) });
  state = state.apply(state.tr.setSelection(TextSelection.create(state.doc, 3)));
  const view = {
    editable: true,
    get state() {
      return state;
    },
    dispatch(tr) {
      state = state.apply(tr);
    },
    focus() {},
  };
  commands.codeBlock(view);
  const saved = normalizeContent(fromPM(state.doc.toJSON()), { workspaceKind: 'BRANCH' });
  const savedItem = saved.content[0].content[0];
  assert.deepEqual(state.doc.firstChild.firstChild.firstChild.toJSON(), paragraph.toJSON());
  assert.equal(savedItem.attrs.id, 'list-item');
  assert.equal(savedItem.content[1].type, 'code_block');
  assert.notEqual(savedItem.content[1].attrs.id, 'list-paragraph');
});

// 高亮只返回原文偏移和固定类名；HTML样式的恶意代码也不能成为可执行DOM。
test('syntax tokens preserve escaped source and unknown languages remain plain text', () => {
  const text = 'const html = "<script>alert(1)</script> &amp; 😀";\n// 注释';
  const tokens = code.codeTokens(text, 'js');
  assert.ok(
    tokens.some(
      (token) =>
        token.className.includes('hljs-keyword') && text.slice(token.from, token.to) === 'const',
    ),
  );
  assert.ok(
    tokens.some(
      (token) =>
        token.className.includes('hljs-string') &&
        text.slice(token.from, token.to).includes('<script>'),
    ),
  );
  assert.ok(
    tokens.some(
      (token) =>
        token.className.includes('hljs-comment') && text.slice(token.from, token.to) === '// 注释',
    ),
  );
  for (const token of tokens) {
    assert.match(token.className, /^hljs-[a-z0-9_-]+(?: hljs-[a-z0-9_-]+)*$/);
    assert.ok(token.from >= 0 && token.to <= text.length);
  }
  assert.deepEqual(code.codeTokens(text, null), []);
  assert.deepEqual(code.codeTokens(text, 'unsupported-script'), []);
});

// 显示装饰和只读边界共同保证：打开阅读、高亮和禁写状态下操作菜单都不会改变正文。
test('code decorations do not mutate saved text and language actions respect editability', () => {
  const doc = schema.nodes.doc.create(
    null,
    schema.nodes.code_block.create(
      { id: 'saved-code', language: 'go' },
      schema.text('package main\nfunc main() {}'),
    ),
  );
  const plugin = code.codeHighlightPlugin();
  let state = EditorState.create({ schema, doc, plugins: [plugin] });
  assert.ok(plugin.getState(state).find().length > 0);
  assert.deepEqual(state.doc.toJSON(), doc.toJSON());
  const view = {
    editable: false,
    get state() {
      return state;
    },
    dispatch(tr) {
      state = state.apply(tr);
    },
    focus() {},
  };
  code.setCodeLanguage(view, 0, 'javascript');
  commands.codeBlock(view);
  assert.deepEqual(state.doc.toJSON(), doc.toJSON());
  view.editable = true;
  code.setCodeLanguage(view, 0, 'arbitrary-language');
  assert.deepEqual(state.doc.toJSON(), doc.toJSON());
  code.setCodeLanguage(view, 0, null);
  assert.equal(plugin.getState(state).find().length, 0);
  assert.equal(state.doc.firstChild.attrs.id, 'saved-code');
  assert.equal(state.doc.textContent, doc.textContent);
});

// Tab只处理代码文本，正文和表格继续使用原键盘行为；插入缩进也不更换块ID。
test('Tab indents code through a normal transaction and leaves paragraph navigation intact', () => {
  const doc = schema.nodes.doc.create(
    null,
    schema.nodes.code_block.create(
      { id: 'tab-code', language: 'go' },
      schema.text('fmt.Println("hello")'),
    ),
  );
  let state = EditorState.create({ schema, doc });
  state = state.apply(state.tr.setSelection(TextSelection.create(doc, 1)));
  assert.equal(
    code.codeKeymap.Tab(state, (tr) => {
      state = state.apply(tr);
    }),
    true,
  );
  assert.equal(state.doc.textContent, '\tfmt.Println("hello")');
  assert.equal(state.doc.firstChild.attrs.id, 'tab-code');
  assert.equal(code.codeKeymap.Tab(editor().state), false);
});
