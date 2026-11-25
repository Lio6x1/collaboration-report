"""用真实 TLS 和小 TCP 接收窗保持在途交付；只输出状态，Cookie/正文留在内存。"""

import json
import socket
import ssl
import sys
import threading
import time
from pathlib import Path


def report(state, **facts):
    print(json.dumps({"state": state, **facts}), flush=True)


def main():
    request = json.loads(sys.stdin.readline())
    root = Path(request["root"])
    deadline = time.monotonic() + 3.0
    connection = None

    def remaining():
        value = deadline - time.monotonic()
        if value <= 0:
            raise TimeoutError("ACTUAL_HTTP_BUDGET_EXPIRED")
        return value

    try:
        context = ssl.create_default_context(cafile=str(root / ".runtime/pki/ca.pem"))
        transport = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        transport.setsockopt(socket.SOL_SOCKET, socket.SO_RCVBUF, 1024)
        transport.settimeout(remaining())
        transport.connect(("127.0.0.1", int(request["port"])))
        connection = context.wrap_socket(transport, server_hostname="localhost")
        connection.settimeout(remaining())
        fields = [
            f"GET {request['path']} HTTP/1.1",
            f"Host: localhost:{request['port']}",
            "Connection: close",
        ]
        for name, value in request["headers"].items():
            if any(character in name + value for character in "\r\n"):
                raise ValueError("HEADER_INVALID")
            fields.append(f"{name}: {value}")
        connection.sendall(("\r\n".join(fields) + "\r\n\r\n").encode("ascii"))
        report("REQUEST_SENT", tlsVerified=True, receiveBufferBytes=connection.getsockopt(socket.SOL_SOCKET, socket.SO_RCVBUF))
        head = bytearray()
        while not head.endswith(b"\r\n\r\n"):
            connection.settimeout(remaining())
            try:
                part = connection.recv(1)
            except TimeoutError:
                report("ENDED", complete=False, receivedBytes=0, reason="TIMEOUT")
                return
            except OSError:
                report("ENDED", complete=False, receivedBytes=0, reason="TLS_ERROR")
                return
            if not part:
                report("ENDED", complete=False, receivedBytes=0, reason="EOF")
                return
            if len(head) >= 65536:
                raise ValueError("HEADERS_INCOMPLETE")
            head.extend(part)
        rows = head.decode("iso-8859-1").split("\r\n")
        status = int(rows[0].split(" ")[1])
        headers = {}
        for row in rows[1:]:
            if ":" in row:
                name, value = row.split(":", 1)
                headers[name.lower()] = value.strip()
        report("HEADERS", status=status, tlsVerified=True)

        resumed = threading.Event()
        threading.Thread(target=lambda: (sys.stdin.readline(), resumed.set()), daemon=True).start()
        resumed.wait(remaining())
        body = bytearray()
        reason = "EOF"
        try:
            while True:
                connection.settimeout(remaining())
                part = connection.recv(65536)
                if not part:
                    break
                body.extend(part)
                if len(body) > 32 * 1024 * 1024:
                    raise ValueError("CLIENT_BODY_BUDGET_EXCEEDED")
        except TimeoutError:
            reason = "TIMEOUT"
        except OSError:
            reason = "TLS_ERROR"
        # Connection: close 固定结束；chunked 的结束块只用于完整性，不解析/输出私有 JSON。
        if headers.get("transfer-encoding", "").lower() == "chunked":
            complete = body.endswith(b"\r\n0\r\n\r\n")
        else:
            complete = "content-length" in headers and len(body) == int(headers["content-length"])
        report("ENDED", complete=complete, receivedBytes=len(body), reason=reason, status=status)
    finally:
        if connection is not None:
            try:
                connection.shutdown(socket.SHUT_RDWR)
            except OSError:
                pass
            connection.close()


try:
    main()
except Exception:
    report("FAILED", code="ACTUAL_HTTP_CLIENT_FAILED")
    sys.exit(1)
