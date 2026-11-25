// 业务事务把同一连接交给仓储；查询期限保持原三秒，不通过放宽期限掩盖未知提交。
export const query = (connection, sql, args = []) => connection.query({ sql, timeout: 3000 }, args);

export class Transactions {
  constructor(pool) {
    this.pool = pool;
  }

  async run(body) {
    const connection = await this.pool.getConnection();
    try {
      await query(connection, 'START TRANSACTION');
      const result = await body(connection);
      await query(connection, 'COMMIT');
      return result;
    } finally {
      try {
        await query(connection, 'ROLLBACK');
      } catch {}
      connection.release();
    }
  }
}
