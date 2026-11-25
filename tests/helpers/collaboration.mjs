import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import grpc from '@grpc/grpc-js';
import loader from '@grpc/proto-loader';
import { state, runtime, root } from '../../scripts/runtime-lib.mjs';
export const env = state();
const options = { longs: String, enums: String, defaults: true, keepCase: true };
const api = grpc.loadPackageDefinition(
  loader.loadSync(resolve(root, 'contracts/proto/internal.proto'), options),
).collaboration.v1;
export function documentRPC() {
  const creds = grpc.credentials.createSsl(
    readFileSync(resolve(runtime, 'pki/ca.pem')),
    readFileSync(resolve(runtime, 'pki/collaboration-key.pem')),
    readFileSync(resolve(runtime, 'pki/collaboration.pem')),
  );
  return new api.DocumentInternal('127.0.0.1:' + env.ports.documentRpc, creds, {
    'grpc.ssl_target_name_override': 'localhost',
    'grpc.default_authority': 'localhost',
    'grpc.initial_reconnect_backoff_ms': 100,
  });
}
export function rpcCall(client, method, input) {
  return new Promise((done, reject) =>
    client[method](input, { deadline: Date.now() + 2500 }, (err, value) =>
      err ? reject(err) : done(value),
    ),
  );
}
