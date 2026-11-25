// 私有持久证明仅从真实Node日志及唯一固定确认任务取得；保留原pool.query调用和SQL。
export async function committedUpdate(pool, input) {
  const [rows] = await pool.query(
    'SELECT CAST(seq AS CHAR) AS seq,payload_hash FROM update_log WHERE room_id=? AND epoch=? AND client_session_id=? AND client_update_id=?',
    [input.room_id, input.workspace_epoch, input.client_session_id, input.client_update_id],
  );
  return rows[0];
}
export async function confirmationsForReceipt(pool, receipt) {
  const [rows] = await pool.query(
    "SELECT business_key,payload FROM durable_task WHERE kind='RESOURCE_CONFIRM' AND JSON_UNQUOTE(JSON_EXTRACT(payload,'$.node_receipt_ref'))=?",
    [receipt],
  );
  return rows;
}
