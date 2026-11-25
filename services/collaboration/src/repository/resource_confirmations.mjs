import { query } from './sql.mjs';

// 所有事实使用调用方传入的同一连接；仓储不另开事务或改变原查询期限。

export function queueResourceConfirmation(connection, operation, payload) {
  return query(
    connection,
    "INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES('RESOURCE_CONFIRM',?,?,'QUEUED',UTC_TIMESTAMP(6))",
    [operation, payload],
  );
}

export function markResourceConfirmed(connection, operation) {
  return query(
    connection,
    "UPDATE durable_task SET state='SUCCEEDED',last_error=NULL WHERE kind='RESOURCE_CONFIRM' AND business_key=?",
    [operation],
  );
}

export function retryResourceConfirmation(connection, state, error, operation) {
  return query(
    connection,
    "UPDATE durable_task SET state=?,retry_at=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND),attempts=attempts+1,last_error=? WHERE kind='RESOURCE_CONFIRM' AND business_key=?",
    [state, error, operation],
  );
}

export function pendingResourceConfirmations(connection, id) {
  return query(
    connection,
    "SELECT business_key,payload FROM durable_task WHERE kind='RESOURCE_CONFIRM' AND state<>'SUCCEEDED' AND " +
      (id ? 'business_key=?' : 'retry_at<=UTC_TIMESTAMP(6) ORDER BY retry_at LIMIT 32'),
    id ? [id] : [],
  );
}
