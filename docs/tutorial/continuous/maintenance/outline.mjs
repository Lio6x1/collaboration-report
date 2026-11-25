// 教师工具：使用项目已有 Babel 解析器，不用正则猜测模板字符串里的花括号。
import fs from 'node:fs';
import { parse } from '@babel/parser';
const paths = JSON.parse(fs.readFileSync(0, 'utf8'));
const output = paths.map((path) => {
  const source =
    fs
      .readFileSync(path, 'utf8')
      .replace(/^\uFEFF/, '')
      .trimEnd()
      .replaceAll('\r\n', '\n') + '\n';
  const tree = parse(source, {
    sourceType: 'module',
    plugins: path.endsWith('.ts') ? ['typescript'] : [],
  });
  const byte = (value) => Buffer.byteLength(source.slice(0, value), 'utf8');
  const compact = (node) => source.slice(node.start, node.end).trim();
  const children = (node) => Object.entries(node).filter(([key]) =>
    !['loc', 'leadingComments', 'trailingComments', 'innerComments'].includes(key)
  ).flatMap(([, value]) => Array.isArray(value) ? value : [value])
    .filter((value) => value && typeof value.type === 'string');
  const bindings = (node) => {
    if (!node) return [];
    if (node.type === 'Identifier') return [node.name];
    if (node.type === 'AssignmentPattern') return bindings(node.left);
    if (node.type === 'RestElement') return bindings(node.argument);
    if (node.type === 'ObjectPattern') return node.properties.flatMap((p) => bindings(p.value ?? p.argument));
    if (node.type === 'ArrayPattern') return node.elements.flatMap(bindings);
    return [];
  };
  const evidence = (fn) => {
    const params = (fn?.params ?? []).map((p) => ({
      name: compact(p), type: p.type === 'ObjectPattern' ? '解构对象' : p.type === 'AssignmentPattern' ? '带默认值的参数' : 'JavaScript 参数',
      comment: '', uses: [], bindings: bindings(p),
    }));
    const results = fn?.params ? [{ name: '(返回值)', type: fn.async ? 'Promise' : 'JavaScript 返回值', comment: '', uses: [] }] : [];
    const facts = [];
    const add = (list, value) => { if (!list.includes(value)) list.push(value); };
    const walk = (node, stack = [], depth = 0, shadowed = new Set()) => {
      if (!node) return;
      // 回调可捕获外层参数，但它自己的参数/块级声明会遮蔽同名值。
      if (/Function|Method/.test(node.type)) {
        const local = new Set([...shadowed, ...(node.params ?? []).flatMap(bindings), ...bindings(node.id)]);
        walk(node.body, [...stack, node], depth + 1, local);
        return;
      }
      if (node.type === 'BlockStatement') {
        const names = node.body.flatMap((item) => item.type === 'VariableDeclaration' && item.kind !== 'var'
          ? item.declarations.flatMap((d) => bindings(d.id))
          : /FunctionDeclaration|ClassDeclaration/.test(item.type) ? bindings(item.id) : []);
        shadowed = new Set([...shadowed, ...names]);
      }
      if (node.type === 'Identifier') {
        const parent = stack.at(-1);
        const isKey = parent && ((/MemberExpression/.test(parent.type) && parent.property === node && !parent.computed) ||
          (parent.type === 'ObjectProperty' && parent.key === node && !parent.computed && !parent.shorthand));
        if (!isKey && !shadowed.has(node.name)) for (const param of params) if (param.bindings.includes(node.name)) {
          const use = stack.findLast((item) => /CallExpression|BinaryExpression|LogicalExpression|MemberExpression|ReturnStatement|AssignmentExpression/.test(item.type));
          if (use) add(param.uses, compact(use));
        }
      }
      const factsBefore = facts.length;
      if (node.type === 'IfStatement') {
        const first = node.consequent.type === 'BlockStatement' ? node.consequent.body[0] : node.consequent;
        facts.push({ start: byte(node.start), kind: 'condition', text: compact(node.test),
          outcome: first && /ReturnStatement|ThrowStatement|ContinueStatement|BreakStatement/.test(first.type) ? compact(first) : '' });
      }
      if (node.type === 'ReturnStatement') {
        facts.push({ start: byte(node.start), kind: 'return', text: compact(node) });
        if (depth === 0 && results.length) add(results[0].uses, node.argument ? compact(node.argument) : 'undefined');
      }
      if (node.type === 'ThrowStatement') facts.push({ start: byte(node.start), kind: 'throw', text: compact(node) });
      if (node.type === 'AwaitExpression') facts.push({ start: byte(node.start), kind: 'await', text: compact(node.argument) });
      if (depth > 0) for (const fact of facts.slice(factsBefore)) fact.scope = '内部函数';
      for (const child of children(node)) walk(child, [...stack, node], depth, shadowed);
    };
    if (fn?.params) {
      walk(fn.body);
      if (fn.type === 'ArrowFunctionExpression' && fn.body.type !== 'BlockStatement') {
        add(results[0].uses, compact(fn.body));
        results[0].comment = '箭头函数的表达式体就是返回值；调用者接收下方表达式的计算结果。';
      } else if (fn.kind === 'constructor') {
        results[0].comment = '构造器由new调用时初始化实例；这里没有普通业务方法的结果值。字段保存依赖，后续实例方法使用这些依赖。';
      } else if (!results[0].uses.length) {
        results[0].comment = fn.async
          ? '没有显式return值；正常走到末尾时Promise兑现为undefined。await失败或throw会拒绝Promise，调用者仍需处理失败。'
          : '没有显式return值；正常走到末尾时结果为undefined。throw会中断调用并交给外层处理，不能把无返回值理解为没有错误路径。';
      }
    }
    return { params, results, facts };
  };
  const classFields = (node) => {
    if (node.type !== 'ClassDeclaration') return [];
    const fields = new Map();
    const visit = (item) => {
      if (!item || /Function|Method/.test(item.type)) return;
      if (item.type === 'AssignmentExpression' && item.left.type === 'MemberExpression' &&
          item.left.object.type === 'ThisExpression' && !item.left.computed) {
        const name = item.left.property.name;
        const field = fields.get(name) ?? { name, type: '实例字段', comment: '', uses: [] };
        field.uses.push(compact(item));
        fields.set(name, field);
      }
      for (const child of children(item)) visit(child);
    };
    for (const member of node.body.body) if (member.kind === 'constructor') visit(member.body);
    return [...fields.values()];
  };
  const decls = tree.program.body.map((outer) => {
    const node = outer.declaration ?? outer;
    let names = [node.id?.name].filter(Boolean);
    if (node.type === 'VariableDeclaration')
      names = node.declarations.flatMap((d) => (d.id.name ? [d.id.name] : []));
    if (!names.length)
      names = [outer.type === 'ImportDeclaration' ? 'import' : `statement-${outer.start}`];
    const comments = (outer.leadingComments ?? []).filter(
      (c) => source.slice(c.end, outer.start).trim() === '',
    );
    const start = comments.length ? comments[0].start : outer.start;
    const fn = node.type === 'VariableDeclaration' ? node.declarations[0]?.init : node;
    const cuts = [];
    const visit = (child) => {
      if (!child || typeof child !== 'object') return;
      if (typeof child.type === 'string' && /Statement$|Declaration$/.test(child.type)) {
        const line = source.lastIndexOf('\n', child.start) + 1;
        if (source.slice(line, child.start).trim() === '') cuts.push(line);
      }
      for (const [key, value] of Object.entries(child)) {
        if (['loc', 'leadingComments', 'trailingComments', 'innerComments'].includes(key)) continue;
        if (Array.isArray(value)) value.forEach(visit);
        else if (value && typeof value === 'object') visit(value);
      }
    };
    visit(fn?.body);
    return {
      start: byte(start),
      end: byte(outer.end),
      symbols: names,
      kind: outer.type === 'ImportDeclaration' ? 'import' : node.type,
      signature: source.slice(outer.start, fn?.body?.start ?? outer.end).split('\n')[0],
      comment: comments.map((c) => c.value.trim()).join('\n'),
      fields: classFields(node),
      ...evidence(fn),
      members: node.type === 'ClassDeclaration' ? node.body.body.filter((m) => /Method/.test(m.type)).map((m) => ({
        start: byte(m.start), end: byte(m.end), symbols: [`${names[0]}.${compact(m.key)}`],
        signature: source.slice(m.start, m.body.start).trim(), kind: 'method',
        ...evidence(m),
      })) : [],
      cuts: cuts.map(byte),
    };
  });
  return { path, decls };
});
process.stdout.write(JSON.stringify(output));
