// 远端安装不产生新本地UPDATE；相同二进制身份在刷新和重连时原样保留。
export const REMOTE = Symbol('durable-server');
export const encode = (bytes: Uint8Array) => {
  let value = '';
  for (let i = 0; i < bytes.length; i += 8192)
    value += String.fromCharCode(...bytes.subarray(i, i + 8192));
  return btoa(value);
};
export const decode = (value: string) => Uint8Array.from(atob(value), (c) => c.charCodeAt(0));
export const digest = async (bytes: Uint8Array) =>
  Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', new Uint8Array(bytes).buffer)))
    .map((x) => x.toString(16).padStart(2, '0'))
    .join('');
