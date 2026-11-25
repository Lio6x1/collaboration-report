import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { runtime, state } from '../../scripts/runtime-lib.mjs';
import { request } from './https.mjs';

export async function collaborationMetrics() {
  const response = await request(state().ports.collaborationHttp, '/infra/metrics', {
    cert: readFileSync(resolve(runtime, 'pki/document.pem')),
    key: readFileSync(resolve(runtime, 'pki/document-key.pem')),
  });
  assert.equal(response.status, 200);
  return JSON.parse(response.body);
}
