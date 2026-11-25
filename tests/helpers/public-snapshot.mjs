import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import grpc from '@grpc/grpc-js';
import loader from '@grpc/proto-loader';
import { state, runtime, root } from '../../scripts/runtime-lib.mjs';

// Search只以实际私有mTLS身份领取权威当前快照；不把浏览器Cookie或伪造角色头当成服务身份。
export function publicSnapshotRPC() {
  const api = grpc.loadPackageDefinition(
    loader.loadSync(resolve(root, 'contracts/proto/internal.proto'), {
      longs: String,
      enums: String,
      defaults: true,
      keepCase: true,
    }),
  ).collaboration.v1;
  return new api.DocumentInternal(
    '127.0.0.1:' + state().ports.documentRpc,
    grpc.credentials.createSsl(
      readFileSync(resolve(runtime, 'pki/ca.pem')),
      readFileSync(resolve(runtime, 'pki/search-key.pem')),
      readFileSync(resolve(runtime, 'pki/search.pem')),
    ),
    { 'grpc.ssl_target_name_override': 'localhost', 'grpc.max_receive_message_length': 32 << 20 },
  );
}
