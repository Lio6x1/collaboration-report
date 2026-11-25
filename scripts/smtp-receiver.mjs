import net from 'node:net';
import { randomUUID } from 'node:crypto';
import { resolve } from 'node:path';
import { state, runtime, saveJSON } from './runtime-lib.mjs';

// 仅用于本机开发的隔离收件器；不会转发邮件。收件正文含验证码，只存忽略目录。
const env = state(),
  owner = process.env.COLLAB_RUN_OWNER;
if (!owner || !env.ports.smtp) throw Error('SMTP_CONFIG_REQUIRED');
const server = net.createServer((socket) => {
  socket.setTimeout(5000, () => socket.destroy());
  let buffer = '',
    body = [],
    recipient = '',
    sender = '',
    data = false,
    bytes = 0;
  const reply = (value) => socket.write(value + '\r\n');
  reply('220 localhost collaboration-dev ' + owner);
  socket.on('data', (chunk) => {
    bytes += chunk.length;
    if (bytes > 16384) {
      socket.destroy();
      return;
    }
    buffer += chunk.toString('utf8');
    while (buffer.includes('\r\n')) {
      const end = buffer.indexOf('\r\n'),
        line = buffer.slice(0, end);
      buffer = buffer.slice(end + 2);
      if (data) {
        if (line !== '.') {
          body.push(line.startsWith('..') ? line.slice(1) : line);
          continue;
        }
        try {
          saveJSON(resolve(runtime, 'smtp/inbox', randomUUID() + '.json'), {
            sender,
            recipient,
            body: body.join('\r\n'),
            receivedAt: new Date().toISOString(),
          });
          reply('250 stored');
        } catch {
          reply('451 storage unavailable');
        }
        data = false;
        body = [];
        continue;
      }
      const upper = line.toUpperCase();
      if (upper.startsWith('EHLO ') || upper.startsWith('HELO '))
        reply('250 localhost collaboration-dev ' + owner);
      else if (upper.startsWith('MAIL FROM:')) {
        sender = line.slice(10);
        recipient = '';
        reply('250 ok');
      } else if (upper.startsWith('RCPT TO:')) {
        recipient = line.slice(8);
        reply('250 ok');
      } else if (upper === 'DATA' && sender && recipient) {
        data = true;
        reply('354 end with dot');
      } else if (upper === 'QUIT') {
        reply('221 bye');
        socket.end();
      } else if (upper === 'RSET') {
        sender = '';
        recipient = '';
        body = [];
        reply('250 ok');
      } else if (upper === 'NOOP') reply('250 ok');
      else reply('500 unsupported');
    }
  });
  socket.on('error', () => {});
});
server.listen(env.ports.smtp, '127.0.0.1');
server.on('error', () => {
  console.error('{"code":"SMTP_RECEIVER_FAILED"}');
  process.exitCode = 1;
});
