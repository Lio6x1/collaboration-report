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
