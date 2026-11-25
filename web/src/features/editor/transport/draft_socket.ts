import type { EditSession } from '../../../../../contracts/http/generated';
import type { DraftMessage } from '../domain/model';
import { UserFacingError } from '../../../lib/http';
type Events = {
  message: (raw: string, connection: DraftSocket) => void;
  closed: (connection: DraftSocket) => void;
};

// 传输适配只验证同入口地址、发送OPEN和桥接事件；持久ACK/队列与重连资格由控制器编排。
export class DraftSocket {
  private readonly socket: WebSocket;
  constructor(session: EditSession, client: string, events: Events) {
    const url = new URL(session.wss_url);
    if (
      url.origin !== location.origin.replace(/^https:/, 'wss:') ||
      url.pathname !== '/collaboration' ||
      url.search
    )
      throw new UserFacingError('编辑连接暂时无法建立，请重新打开编辑器');
    const socket = new WebSocket(url);
    this.socket = socket;
    socket.onopen = () =>
      socket.send(
        JSON.stringify({
          type: 'OPEN',
          protocol: 'collab-wss-v1',
          ticket: session.ticket,
          client_session_id: client,
          connection_id: crypto.randomUUID(),
          workspace_epoch: session.workspace_epoch,
        }),
      );
    socket.onmessage = (event) => events.message(event.data, this);
    socket.onclose = () => events.closed(this);
    socket.onerror = () => socket.close();
  }
  get open() {
    return this.socket.readyState === WebSocket.OPEN;
  }
  send(input: DraftMessage) {
    this.socket.send(JSON.stringify(input));
  }
  close() {
    this.socket.close();
  }
}
