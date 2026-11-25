import hljs from 'highlight.js/lib/core';
import go from 'highlight.js/lib/languages/go';
import javascript from 'highlight.js/lib/languages/javascript';
import typescript from 'highlight.js/lib/languages/typescript';
import json from 'highlight.js/lib/languages/json';
import sql from 'highlight.js/lib/languages/sql';
import bash from 'highlight.js/lib/languages/bash';
import powershell from 'highlight.js/lib/languages/powershell';
import python from 'highlight.js/lib/languages/python';
import java from 'highlight.js/lib/languages/java';
import c from 'highlight.js/lib/languages/c';
import cpp from 'highlight.js/lib/languages/cpp';
import css from 'highlight.js/lib/languages/css';
import xml from 'highlight.js/lib/languages/xml';
import yaml from 'highlight.js/lib/languages/yaml';
import markdown from 'highlight.js/lib/languages/markdown';
import type { Node as ProseMirrorNode } from 'prosemirror-model';
import { Plugin, PluginKey, type Command } from 'prosemirror-state';
import { exitCode } from 'prosemirror-commands';
import { Decoration, DecorationSet, type EditorView, type NodeView } from 'prosemirror-view';
import './code.css';

// 只装配菜单中可选的语法，不自动猜语言，也不让正文决定加载或执行任何脚本。
const grammars = {
  go,
  javascript,
  typescript,
  json,
  sql,
  bash,
  powershell,
  python,
  java,
  c,
  cpp,
  css,
  xml,
  yaml,
  markdown,
};
for (const [name, grammar] of Object.entries(grammars)) hljs.registerLanguage(name, grammar);
const languages = [
  ['go', 'Go'],
  ['javascript', 'JavaScript'],
  ['typescript', 'TypeScript'],
  ['json', 'JSON'],
  ['sql', 'SQL'],
  ['bash', 'Shell / Bash'],
  ['powershell', 'PowerShell'],
  ['python', 'Python'],
  ['java', 'Java'],
  ['c', 'C'],
  ['cpp', 'C++'],
  ['css', 'CSS'],
  ['xml', 'HTML / XML'],
  ['yaml', 'YAML'],
  ['markdown', 'Markdown'],
] as const;
const aliases: Record<string, string> = {
  golang: 'go',
  js: 'javascript',
  jsx: 'javascript',
  ts: 'typescript',
  tsx: 'typescript',
  sh: 'bash',
  shell: 'bash',
  ps1: 'powershell',
  pwsh: 'powershell',
  py: 'python',
  h: 'c',
  'c++': 'cpp',
  html: 'xml',
  yml: 'yaml',
  md: 'markdown',
};
function knownLanguage(value: string | null) {
  const name = (value ?? '').trim().toLowerCase();
  return Object.hasOwn(grammars, name) ? name : Object.hasOwn(aliases, name) ? aliases[name] : null;
}

export type CodeToken = { from: number; to: number; className: string };

// highlight.js只输出已转义的文字与span。这里只读取这套固定格式生成“位置+类名”，
// 绝不把高亮HTML插入页面；PM继续拥有原文。还原文字不完全相等时退回纯文本，避免错位。
export function codeTokens(text: string, language: string | null): CodeToken[] {
  const name = knownLanguage(language);
  if (!name || !text) return [];
  let html: string;
  try {
    html = hljs.highlight(text, { language: name, ignoreIllegals: true }).value;
  } catch {
    return [];
  }
  const stack: string[][] = [],
    tokens: CodeToken[] = [];
  let source = '';
  const entities: Record<string, string> = {
    '&amp;': '&',
    '&lt;': '<',
    '&gt;': '>',
    '&quot;': '"',
    '&#x27;': "'",
    '&#39;': "'",
  };
  for (const part of html.matchAll(/<span class="([^"]*)">|(<\/span>)|([^<]+)/g)) {
    if (part[1] !== undefined) {
      stack.push(part[1].split(/\s+/).filter((name) => /^hljs-[a-z0-9_-]+$/.test(name)));
    } else if (part[2]) stack.pop();
    else {
      const value = part[3].replace(/&(?:amp|lt|gt|quot|#x27|#39);/g, (entity) => entities[entity]);
      const className = [...new Set(stack.flat())].join(' ');
      if (className && value)
        tokens.push({ from: source.length, to: source.length + value.length, className });
      source += value;
    }
  }
  return source === text ? tokens : [];
}

const highlightKey = new PluginKey<DecorationSet>('code-highlight');
export function codeHighlightPlugin() {
  // PM节点不可变；只为实际改动过的代码重新分词，普通选区和其他段落更新复用原结果。
  const cache = new WeakMap<ProseMirrorNode, CodeToken[]>();
  function decorations(doc: ProseMirrorNode) {
    const result: Decoration[] = [];
    doc.descendants((node, pos) => {
      if (node.type.name !== 'code_block') return;
      let tokens = cache.get(node);
      if (!tokens) {
        tokens = codeTokens(node.textContent, node.attrs.language);
        cache.set(node, tokens);
      }
      for (const token of tokens)
        result.push(
          Decoration.inline(pos + 1 + token.from, pos + 1 + token.to, { class: token.className }),
        );
      return false;
    });
    return DecorationSet.create(doc, result);
  }
  return new Plugin({
    key: highlightKey,
    state: {
      init: (_, state) => decorations(state.doc),
      apply: (tr, previous) => (tr.docChanged ? decorations(tr.doc) : previous),
    },
    props: { decorations: (state) => highlightKey.getState(state) },
    view(editor) {
      const refresh = () => {
        for (const select of Array.from(
          editor.dom.querySelectorAll<HTMLSelectElement>('.code-language-select'),
        ))
          select.disabled = !editor.editable;
      };
      refresh();
      return { update: refresh };
    },
  });
}

export function setCodeLanguage(view: EditorView, pos: number, language: string | null) {
  if (!view.editable || (language !== null && !languages.some(([value]) => value === language)))
    return;
  const current = view.state.doc.nodeAt(pos);
  if (current?.type.name !== 'code_block' || current.attrs.language === language) return;
  // 单个属性事务保留ID和全部原文，仍经过Yjs、稳定身份插件及可靠保存队列。
  view.dispatch(view.state.tr.setNodeMarkup(pos, undefined, { ...current.attrs, language }));
}

async function copyText(text: string, owner: Document) {
  try {
    if (owner.defaultView?.navigator.clipboard) {
      await owner.defaultView.navigator.clipboard.writeText(text);
      return;
    }
  } catch {
    /* 普通HTTP或剪贴板权限不可用时，继续尝试用户点击触发的兼容复制。 */
  }
  const input = owner.createElement('textarea');
  input.value = text;
  input.readOnly = true;
  input.style.cssText = 'position:fixed;left:-10000px;top:0;opacity:0';
  input.setAttribute('aria-label', '临时代码复制');
  const active = owner.activeElement as HTMLElement | null;
  const selection = owner.getSelection();
  const ranges = selection
    ? Array.from({ length: selection.rangeCount }, (_, index) =>
        selection.getRangeAt(index).cloneRange(),
      )
    : [];
  owner.body.append(input);
  try {
    input.select();
    if (!owner.execCommand('copy')) throw Error('COPY_FAILED');
  } finally {
    input.remove();
    active?.focus({ preventScroll: true });
    selection?.removeAllRanges();
    for (const range of ranges) selection?.addRange(range);
  }
}

// 阅读、共同草稿和人工候选共用节点视图。contentDOM交给PM维护，控件不进入正文或复制内容。
export function codeNodeView(
  initial: ProseMirrorNode,
  view: EditorView,
  getPos: () => number | undefined,
): NodeView {
  let node = initial,
    destroyed = false;
  const owner = view.dom.ownerDocument;
  const dom = owner.createElement('div');
  dom.className = 'code-block';
  const controls = owner.createElement('div');
  controls.className = 'code-block-controls';
  controls.contentEditable = 'false';
  const label = owner.createElement('label');
  label.textContent = '代码语言';
  const select = owner.createElement('select');
  select.className = 'code-language-select';
  select.setAttribute('aria-label', '代码语言');
  for (const [value, text] of [['', '纯文本'], ...languages]) {
    const option = owner.createElement('option');
    option.value = value;
    option.textContent = text;
    select.append(option);
  }
  label.append(select);
  const hint = owner.createElement('span');
  hint.className = 'code-edit-hint';
  hint.textContent = 'Tab 缩进 · Ctrl+Enter 返回正文';
  const status = owner.createElement('span');
  status.className = 'code-copy-status';
  status.setAttribute('role', 'status');
  const copy = owner.createElement('button');
  copy.type = 'button';
  copy.className = 'code-copy-button';
  copy.textContent = '复制代码';
  copy.addEventListener('mousedown', (event) => event.preventDefault());
  copy.addEventListener('click', async () => {
    try {
      await copyText(node.textContent, owner);
      if (!destroyed) status.textContent = '已复制';
    } catch {
      if (!destroyed) status.textContent = '复制失败，请选中代码手动复制';
    }
  });
  controls.append(label, hint, status, copy);
  const pre = owner.createElement('pre');
  const contentDOM = owner.createElement('code');
  pre.append(contentDOM);
  dom.append(controls, pre);
  function refresh() {
    const language: string | null = node.attrs.language;
    const displayed = knownLanguage(language) ?? language ?? '';
    select.querySelector('option[data-unlisted]')?.remove();
    if (displayed && !languages.some(([value]) => value === displayed)) {
      const option = owner.createElement('option');
      option.value = displayed;
      option.textContent = displayed + '（纯文本显示）';
      option.dataset.unlisted = 'true';
      select.append(option);
    }
    select.value = displayed;
    select.disabled = !view.editable;
    pre.dataset.codeLanguage = language ?? '';
    dom.dataset.codeLanguage = language ?? '';
    status.textContent = '';
  }
  select.addEventListener('change', () => {
    const pos = getPos();
    if (typeof pos === 'number' && view.state.doc.nodeAt(pos)?.attrs.id === node.attrs.id)
      setCodeLanguage(view, pos, select.value || null);
    refresh();
  });
  refresh();
  return {
    dom,
    contentDOM,
    update(next) {
      if (next.type !== node.type) return false;
      node = next;
      refresh();
      return true;
    },
    stopEvent: (event) =>
      event.target instanceof owner.defaultView!.Node && controls.contains(event.target),
    ignoreMutation: (mutation) =>
      controls.contains(mutation.target) ||
      (mutation.type === 'attributes' && (mutation.target === dom || mutation.target === pre)),
    destroy: () => {
      destroyed = true;
    },
  };
}

const indentCode: Command = (state, dispatch) => {
  const { $from, $to } = state.selection;
  if ($from.parent.type.name !== 'code_block' || !$from.sameParent($to)) return false;
  dispatch?.(state.tr.insertText('\t').scrollIntoView());
  return true;
};
export const codeKeymap: Record<string, Command> = { Tab: indentCode, 'Mod-Enter': exitCode };
