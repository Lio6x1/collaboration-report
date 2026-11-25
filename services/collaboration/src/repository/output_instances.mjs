import { query } from './sql.mjs';

// 所有事实使用调用方传入的同一连接；仓储不另开事务或改变原查询期限。

export function listOutputInstances(connection) {
  return query(
    connection,
    "SELECT business_key,payload FROM durable_task WHERE kind='NODE_INSTANCE'",
  );
}

export function settleExitedOutputs(connection, instance) {
  return query(
    connection,
    "UPDATE delivery_session SET state='SETTLED',stop_reason='EXIT_CONFIRMED' WHERE instance_id=? AND state<>'SETTLED'",
    [instance],
  );
}

export function recordOutputExit(connection, payload, instance) {
  return query(
    connection,
    "UPDATE durable_task SET payload=? WHERE kind='NODE_INSTANCE' AND business_key=?",
    [payload, instance],
  );
}

export function insertOutputInstance(connection, instance, payload) {
  return query(
    connection,
    "INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES('NODE_INSTANCE',?,?,'SUCCEEDED',UTC_TIMESTAMP(6))",
    [instance, payload],
  );
}
