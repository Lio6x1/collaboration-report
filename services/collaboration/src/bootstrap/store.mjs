import mysql from 'mysql2/promise';
import { Readiness as ReadinessRepository } from '../repository/readiness.mjs';
import { Readiness } from '../application/readiness.mjs';
export function createStore(config) {
  const pool = mysql.createPool({
    ...config.database,
    waitForConnections: true,
    queueLimit: 32,
    connectTimeout: 2000,
  });
  const readiness = new Readiness(
    new ReadinessRepository(pool, Boolean(config.expectedAdditionalMigrations?.['2'])),
    config.expectedSQLSHA,
    config.expectedAdditionalMigrations,
  );
  return {
    pool,
    async checkReady() {
      await readiness.check(this);
    },
  };
}
