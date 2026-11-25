# 阶段01文件与答案：公共底座、正文契约与数据库

先读[本阶段功能讲义](01-foundation.md)，按讲义安排的创建与实现顺序学习。本表是33个文件的查找目录，表内序号只用于定位，不是教学顺序。逐段编写页与完整答案分开；已完成的阶段保留，不切换Git、不覆盖整棵服务树。

| 索引 | 学习工程内目标 | 分步操作 | 章末核对 |
| --- | --- | --- | --- |
| 1 | `contracts/content/canonical.go` | [逐段编写](write/001-canonical.md) | [完整答案](source/001-canonical.md) |
| 2 | `contracts/content/decode.go` | [逐段编写](write/002-decode.md) | [完整答案](source/002-decode.md) |
| 3 | `contracts/content/validator.go` | [逐段编写](write/003-validator.md) | [完整答案](source/003-validator.md) |
| 4 | `db/migrations/001.empty-only.down.sql` | [逐段编写](write/004-001.empty-only.down.md) | [完整答案](source/004-001.empty-only.down.md) |
| 5 | `db/migrations/001.up.sql` | [逐段编写](write/005-001.up.md) | [完整答案](source/005-001.up.md) |
| 6 | `db/migrations/002.up.sql` | [逐段编写](write/006-002.up.md) | [完整答案](source/006-002.up.md) |
| 7 | `db/migrations/003.up.sql` | [逐段编写](write/007-003.up.md) | [完整答案](source/007-003.up.md) |
| 8 | `db/migrations/004.up.sql` | [逐段编写](write/008-004.up.md) | [完整答案](source/008-004.up.md) |
| 9 | `db/migrations/005.up.sql` | [逐段编写](write/009-005.up.md) | [完整答案](source/009-005.up.md) |
| 10 | `db/migrations/006.up.sql` | [逐段编写](write/010-006.up.md) | [完整答案](source/010-006.up.md) |
| 11 | `db/migrations/007.up.sql` | [逐段编写](write/011-007.up.md) | [完整答案](source/011-007.up.md) |
| 12 | `db/migrations/008.up.sql` | [逐段编写](write/012-008.up.md) | [完整答案](source/012-008.up.md) |
| 13 | `db/migrations/009.up.sql` | [逐段编写](write/013-009.up.md) | [完整答案](source/013-009.up.md) |
| 14 | `db/migrations/010.up.sql` | [逐段编写](write/014-010.up.md) | [完整答案](source/014-010.up.md) |
| 15 | `db/migrations/011.up.sql` | [逐段编写](write/015-011.up.md) | [完整答案](source/015-011.up.md) |
| 16 | `db/migrations/012.up.sql` | [逐段编写](write/016-012.up.md) | [完整答案](source/016-012.up.md) |
| 17 | `db/migrations/013.up.sql` | [逐段编写](write/411-013.up.md) | [完整答案](source/411-013.up.md) |
| 18 | `db/migrations/service-grants.sql` | [逐段编写](write/017-service-grants.md) | [完整答案](source/017-service-grants.md) |
| 19 | `packages/platform/account_closure.go` | [逐段编写](write/018-account-closure.md) | [完整答案](source/018-account-closure.md) |
| 20 | `packages/platform/authority_rpc_protocol.go` | [逐段编写](write/019-authority-rpc-protocol.md) | [完整答案](source/019-authority-rpc-protocol.md) |
| 21 | `packages/platform/fence.go` | [逐段编写](write/020-fence.md) | [完整答案](source/020-fence.md) |
| 22 | `packages/platform/operation_receipt.go` | [逐段编写](write/021-operation-receipt.md) | [完整答案](source/021-operation-receipt.md) |
| 23 | `packages/platform/peer_clients_protocol.go` | [逐段编写](write/022-peer-clients-protocol.md) | [完整答案](source/022-peer-clients-protocol.md) |
| 24 | `packages/platform/rebuildartifact/artifact.go` | [逐段编写](write/023-artifact.md) | [完整答案](source/023-artifact.md) |
| 25 | `packages/platform/rebuildartifact/files.go` | [逐段编写](write/024-files.md) | [完整答案](source/024-files.md) |
| 26 | `packages/platform/rebuildmeta/binding.go` | [逐段编写](write/025-binding.md) | [完整答案](source/025-binding.md) |
| 27 | `packages/platform/rebuildmeta/descriptor.go` | [逐段编写](write/026-descriptor.md) | [完整答案](source/026-descriptor.md) |
| 28 | `packages/platform/rebuildmeta/entry.go` | [逐段编写](write/027-entry.md) | [完整答案](source/027-entry.md) |
| 29 | `packages/platform/schema_readiness.go` | [逐段编写](write/028-schema-readiness.md) | [完整答案](source/028-schema-readiness.md) |
| 30 | `packages/platform/security.go` | [逐段编写](write/029-security.md) | [完整答案](source/029-security.md) |
| 31 | `packages/platform/service.go` | [逐段编写](write/030-service.md) | [完整答案](source/030-service.md) |
| 32 | `packages/platform/store.go` | [逐段编写](write/031-store.md) | [完整答案](source/031-store.md) |
| 33 | `packages/platform/store_diagnostics.go` | [逐段编写](write/032-store-diagnostics.md) | [完整答案](source/032-store-diagnostics.md) |
