# services/collaboration/src/domain/shared_protocol.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/301-shared-protocol.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：定义模型、输入输出和纯规则；把状态与副作用分开。

主要声明：`RPCFailure`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
import { createHash } from 'node:crypto';

// 来源 errors.mjs：原业务声明与函数体保留。
// 只传播稳定业务码；驱动原文不能进入HTTP/WSS/RPC错误或日志。
export class RPCFailure extends Error {
  constructor(code, details) {
    const safe =
      typeof details === 'string' && /^[A-Z][A-Z0-9_]{2,63}$/.test(details)
        ? details
        : 'SERVICE_UNAVAILABLE';
    super(safe);
    this.code = safe === details ? code : 14;
    this.details = safe;
  }
}

// 来源 bytes.mjs：原业务声明与函数体保留。

export const hash = (value) => createHash('sha256').update(value).digest();

// 来源 protocol.mjs：原业务声明与函数体保留。

export const json = (value) => (typeof value === 'string' ? JSON.parse(value) : value);
export const validID = (value) =>
  typeof value === 'string' && /^[A-Za-z0-9][A-Za-z0-9_-]{15,95}$/.test(value);
export const deny = (code) => new RPCFailure(7, code);
export const bad = (code) => new RPCFailure(3, code);
````

UTF-8/LF全文SHA-256：`5519708e412227da3ce4758cb110ddef19baa4c2528aed3094047113472a8955`。[参考实现](../../../../services/collaboration/src/domain/shared_protocol.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
