import grpc from '@grpc/grpc-js';
import loader from '@grpc/proto-loader';
import { resolve } from 'node:path';
import { authorizeCall } from './rpc_authorization.mjs';
import { fenceRooms } from '../application/fences.mjs';
import { RPCFailure } from '../domain/shared_protocol.mjs';

// 生成契约和服务身份在传输边界处理；未实现方法保持UNIMPLEMENTED。
export function createRPC(cfg, store, engine) {
  const rpc = new grpc.Server({
    // 三份固定输入每份最多16MiB；响应候选仍最多16MiB，不能沿用单份接收预算。
    'grpc.max_receive_message_length': 64 << 20,
    'grpc.max_send_message_length': 32 << 20,
  });
  const options = { longs: String, enums: String, defaults: true, keepCase: true };
  const business = grpc.loadPackageDefinition(
    loader.loadSync(resolve(cfg.root, 'contracts/proto/internal.proto'), options),
  ).collaboration.v1.CollaborationInternal;
  const health = grpc.loadPackageDefinition(
    loader.loadSync(resolve(cfg.root, 'contracts/proto/health.proto'), options),
  ).grpc.health.v1.Health;
  const protect = (handler) => async (call, callback) => {
    const error = authorizeCall(call.getAuthContext(), call.getPath(), call.getDeadline());
    if (error) {
      callback(error);
      return;
    }
    try {
      await handler(call, callback);
    } catch (err) {
      callback(
        err instanceof RPCFailure
          ? { code: err.code, details: err.details }
          : { code: grpc.status.UNAVAILABLE, details: 'SERVICE_UNAVAILABLE' },
      );
    }
  };
  const handlers = Object.fromEntries(
    Object.keys(business.service).map((method) => [
      method,
      protect(async (_call, callback) =>
        callback({ code: grpc.status.UNIMPLEMENTED, details: 'FEATURE_NOT_IMPLEMENTED' }),
      ),
    ]),
  );
  handlers.FenceAccess = protect(async (call, callback) =>
    callback(null, await fenceRooms(engine, call.request)),
  );
  handlers.GetFenceResult = handlers.FenceAccess;
  handlers.CaptureSubmission = protect(async (call, callback) =>
    callback(null, await engine.captures.capture(call.request)),
  );
  handlers.GetCapture = protect(async (call, callback) =>
    callback(null, await engine.captures.get(call.request)),
  );
  handlers.FinalizeCapture = protect(async (call, callback) =>
    callback(null, await engine.captures.finalize(call.request)),
  );
  handlers.ComputeMergeCandidate = protect(async (call, callback) =>
    callback(null, await engine.mergeCandidates.compute(call.request)),
  );
  handlers.CleanupCandidate = protect(async (call, callback) =>
    callback(null, await engine.candidateCleanup.clean(call.request)),
  );
  handlers.CaptureMainView = protect(async (call, callback) =>
    callback(null, await engine.mainCaptures.capture(call.request)),
  );
  handlers.ReconcileMainCommit = protect(async (call, callback) =>
    callback(null, await engine.mainInstallations.reconcile(call.request)),
  );
  handlers.PrepareMainCommit = protect(async (call, callback) =>
    callback(null, await engine.mainCommits.prepare(call.request)),
  );
  handlers.CaptureDraftPreview = protect(async (call, callback) =>
    callback(
      null,
      await (
        call.request.context?.target?.purpose === 'STRUCTURE_PREVIEW'
          ? engine.structurePreviews
          : engine.draftPreviews
      ).capture(call.request),
    ),
  );
  handlers.ApplyDraftResolution = protect(async (call, callback) =>
    callback(null, await engine.draftTransitions.apply(call.request)),
  );
  rpc.addService(business.service, handlers);
  rpc.addService(health.service, {
    Check: protect(async (_call, callback) => {
      try {
        await store.checkReady();
        callback(null, { status: 'SERVING' });
      } catch {
        callback(null, { status: 'NOT_SERVING' });
      }
    }),
  });

  return rpc;
}
