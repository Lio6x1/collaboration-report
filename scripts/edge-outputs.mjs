const epoch = (value) => {
  if (typeof value !== 'string' || !/^[1-9][0-9]{0,19}$/.test(value)) throw Error('SOURCE_INVALID');
  const number = BigInt(value);
  if (number > 18446744073709551615n) throw Error('SOURCE_INVALID');
  return number;
};
const scope = (value) => {
  if (
    typeof value !== 'string' ||
    !/^(?:session:[a-f0-9]{64}|(?:publication|platform-admin):[1-9][0-9]{0,19}|(?:identity|credential|doc-acl|kb-acl|kb-create|space-owner):[1-9][0-9]*(?::[1-9][0-9]*)?|org-(?:member|admin|owner|create):[1-9][0-9]*:[1-9][0-9]*)$/.test(
      value,
    )
  )
    throw Error('SOURCE_INVALID');
  // 发布和平台治理来源各绑定一个真实编号，不能用额外范围段或溢出编号绕过关闭水位。
  if (value.startsWith('publication:') || value.startsWith('platform-admin:'))
    epoch(value.slice(value.indexOf(':') + 1));
  return value;
};

// 入口拥有实际浏览器输出；文档服务调用关闭时，必须等socket/handler结束再返回。
// 水位持久化不包含Cookie或正文。旧关闭跨重启保留，时间经过不会让旧准入复活。
export class EdgeOutputs {
  constructor(storage) {
    this.storage = storage;
    this.gates = storage.load();
    this.handlers = new Map();
  }
  attach(id, handler) {
    if (this.handlers.has(id)) throw Error('HANDLER_CONFLICT');
    this.handlers.set(id, { ...handler, lease: null, stopping: false });
  }
  open(input) {
    const handler = this.handlers.get(input.handler_id);
    if (!handler) throw Error('HANDLER_UNKNOWN');
    if (
      typeof input.lease_id !== 'string' ||
      input.lease_id.length > 96 ||
      !Array.isArray(input.sources) ||
      !input.sources.length ||
      input.sources.length > 512
    )
      throw Error('LEASE_INVALID');
    const sources = input.sources
      .map((s) => ({ scope_key: scope(s.scope_key), epoch: epoch(s.epoch).toString() }))
      .sort((a, b) => a.scope_key.localeCompare(b.scope_key));
    if (new Set(sources.map((s) => s.scope_key)).size !== sources.length)
      throw Error('SOURCE_INVALID');
    for (const s of sources) {
      const gate = this.gates[s.scope_key];
      if (
        gate &&
        (epoch(s.epoch) < epoch(gate.epoch) || (s.epoch === gate.epoch && gate.state === 'BLOCKED'))
      )
        throw Error('SOURCE_CLOSED');
    }
    const binding = JSON.stringify({ lease_id: input.lease_id, sources });
    if (handler.lease && handler.lease !== binding) throw Error('LEASE_CONFLICT');
    if (handler.stopping) throw Error('SOURCE_CLOSED');
    for (const s of sources) {
      const gate = this.gates[s.scope_key];
      if (!gate || epoch(s.epoch) > epoch(gate.epoch))
        this.gates[s.scope_key] = { epoch: s.epoch, state: 'OPEN' };
    }
    this.storage.save(this.gates);
    handler.lease = binding;
    handler.sources = sources;
    return { state: 'OPEN' };
  }
  allowed(id) {
    const h = this.handlers.get(id);
    return Boolean(
      h?.lease &&
      !h.stopping &&
      h.sources.every((s) => {
        const g = this.gates[s.scope_key];
        return g && g.state === 'OPEN' && g.epoch === s.epoch;
      }),
    );
  }
  async fence(source) {
    const key = scope(source.scope_key),
      closed = epoch(source.epoch),
      prior = this.gates[key];
    if (!prior || closed >= epoch(prior.epoch)) {
      this.gates[key] = { epoch: closed.toString(), state: 'BLOCKED' };
      this.storage.save(this.gates);
    }
    const pending = [];
    for (const h of this.handlers.values())
      if (h.sources?.some((s) => s.scope_key === key && epoch(s.epoch) <= closed)) {
        h.stopping = true;
        h.stop();
        pending.push(h.closed);
      }
    await Promise.all(pending);
    return { state: 'OUTPUT_STOPPED' };
  }
  detach(id) {
    this.handlers.delete(id);
  }
}
