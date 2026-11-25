# services/collaboration/src/bootstrap/store.mjs

阶段04：[功能讲义](../04-collaboration.md) · [分步编写](../write/359-store.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：注册路由、RPC及恢复器，连接入口和完整业务。

主要声明：`createStore`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````javascript
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
````

UTF-8/LF全文SHA-256：`c969eb3ef868de29fa7705cf1d3c4b9634f84fece6b774ba3ccc90338d5d9d46`。[参考实现](../../../../services/collaboration/src/bootstrap/store.mjs)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
