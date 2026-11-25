import net from 'node:net';
// 开发安装/测试的有限RESP2客户端。二进制DUMP必须按字节处理，不能经UTF-8往返。
const budget = 2 * 1024 * 1024;
function encode(args) {
  const parts = [Buffer.from('*' + args.length + '\r\n')];
  for (const arg of args) {
    const value = Buffer.isBuffer(arg) ? arg : Buffer.from(String(arg));
    parts.push(Buffer.from('$' + value.length + '\r\n'), value, Buffer.from('\r\n'));
  }
  return Buffer.concat(parts);
}
function parse(buffer, offset, binaryBulk, depth = 0) {
  if (depth > 4) throw Error('REDIS_REPLY_UNSUPPORTED');
  const end = buffer.indexOf('\r\n', offset);
  if (end < 0) return null;
  const line = buffer.subarray(offset, end).toString('ascii'),
    start = end + 2;
  if (line[0] === '-')
    throw Error(line.includes('NOPERM') ? 'REDIS_PERMISSION_DENIED' : 'REDIS_COMMAND_REJECTED');
  if (line[0] === '+') return { value: line.slice(1), end: start };
  if (line[0] === ':') return { value: Number(line.slice(1)), end: start };
  const count = Number(line.slice(1));
  if (!Number.isSafeInteger(count) || count < -1 || count > budget)
    throw Error('REDIS_REPLY_UNSUPPORTED');
  if (line[0] === '$') {
    if (count === -1) return { value: null, end: start };
    if (buffer.length < start + count + 2) return null;
    if (buffer.subarray(start + count, start + count + 2).toString() !== '\r\n')
      throw Error('REDIS_REPLY_UNSUPPORTED');
    const value = buffer.subarray(start, start + count);
    return {
      value: binaryBulk ? Buffer.from(value) : value.toString('utf8'),
      end: start + count + 2,
    };
  }
  if (line[0] === '*') {
    if (count === -1) return { value: null, end: start };
    if (count > 1024) throw Error('REDIS_REPLY_UNSUPPORTED');
    const value = [];
    let cursor = start;
    for (let i = 0; i < count; i++) {
      const item = parse(buffer, cursor, binaryBulk, depth + 1);
      if (!item) return null;
      value.push(item.value);
      cursor = item.end;
    }
    return { value, end: cursor };
  }
  throw Error('REDIS_REPLY_UNSUPPORTED');
}
export async function redisCommand(port, user, password, args, { binaryBulk = false } = {}) {
  return new Promise((done, reject) => {
    const socket = net.connect(port, '127.0.0.1');
    let buffer = Buffer.alloc(0),
      replies = 0,
      settled = false;
    const finish = (err, value) => {
      if (settled) return;
      settled = true;
      socket.destroy();
      err ? reject(err) : done(value);
    };
    socket.setTimeout(2000, () => finish(Error('REDIS_TIMEOUT')));
    socket.on('connect', () =>
      socket.write(Buffer.concat([encode(['AUTH', user, password]), encode(args)])),
    );
    socket.on('data', (data) => {
      buffer = Buffer.concat([buffer, data]);
      if (buffer.length > budget) {
        finish(Error('REDIS_REPLY_TOO_LARGE'));
        return;
      }
      try {
        for (;;) {
          const item = parse(buffer, 0, binaryBulk);
          if (!item) return;
          buffer = buffer.subarray(item.end);
          if (++replies === 2) {
            finish(null, item.value);
            return;
          }
        }
      } catch (err) {
        finish(err);
      }
    });
    socket.on('error', () => finish(Error('REDIS_UNAVAILABLE')));
    socket.on('close', () => finish(Error('REDIS_UNAVAILABLE')));
  });
}
