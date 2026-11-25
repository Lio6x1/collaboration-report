# 分步编写 `db/migrations/service-grants.sql`

先按 [F02 · 私有库、业务约束与连续迁移](../lessons/foundation/02-private-databases.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/017-service-grants.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

给四个应用账号分配各自私有表的最小读写权限。

迁移/root账号只负责部署，运行服务不能拥有跨库任意查询或修改固定事实的权限。

写之前先按这条执行路线理解各段的连接：

1. 每个collab_*账号只获得自己数据库表权限；schema_migration只读。
2. verification_consumption/admission_source及固定版本表不授通用UPDATE；audit_event/outbox只追加读取。
3. 新增重建制品/物理日志身份表仍保留只追加或受限更新能力。
4. document_sharing_archive仅SELECT、INSERT；不能修改或删除原归档事实。

本文件需要守住的边界：

- 密码由部署秘密单独创建，不能写进本文件；不能为了测试方便授予全部库的ALL。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'db/migrations' -Force | Out-Null
if ((Test-Path -LiteralPath 'db/migrations/service-grants.sql') -and (Get-Item -LiteralPath 'db/migrations/service-grants.sql').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'db/migrations/service-grants.sql')) { New-Item -ItemType File -Path 'db/migrations/service-grants.sql' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：SQL-1

插入位置：空文件第 1 行。

Create users/passwords separately from deployment secrets. No passwords in this file.

本步声明：`GRANT SELECT ON account_db.schema_migration TO 'collab_account'@'%';`。

<!-- write-step: 1 bytes=0:158 -->
````sql
-- Create users/passwords separately from deployment secrets. No passwords in this file.
GRANT SELECT ON account_db.schema_migration TO 'collab_account'@'%';
````

## 第 2 步：SQL-2

插入位置：第 1 步代码之后，紧接追加；完成稿约第 3 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.\`user\` TO 'collab_account'@'%';`。

<!-- write-step: 2 bytes=158:241 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.`user` TO 'collab_account'@'%';
````

## 第 3 步：SQL-3

插入位置：第 2 步代码之后，紧接追加；完成稿约第 4 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.\`session\` TO 'collab_account'@'%';`。

<!-- write-step: 3 bytes=241:327 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.`session` TO 'collab_account'@'%';
````

## 第 4 步：SQL-4

插入位置：第 3 步代码之后，紧接追加；完成稿约第 5 行。

本步声明：`GRANT SELECT, INSERT, DELETE ON account_db.\`verification_consumption\` TO 'collab_account'@'%';`。

<!-- write-step: 4 bytes=327:422 -->
````sql
GRANT SELECT, INSERT, DELETE ON account_db.`verification_consumption` TO 'collab_account'@'%';
````

## 第 5 步：SQL-5

插入位置：第 4 步代码之后，紧接追加；完成稿约第 6 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.\`organization\` TO 'collab_account'@'%';`。

<!-- write-step: 5 bytes=422:513 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.`organization` TO 'collab_account'@'%';
````

## 第 6 步：SQL-6

插入位置：第 5 步代码之后，紧接追加；完成稿约第 7 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.\`organization_member\` TO 'collab_account'@'%';`。

<!-- write-step: 6 bytes=513:611 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.`organization_member` TO 'collab_account'@'%';
````

## 第 7 步：SQL-7

插入位置：第 6 步代码之后，紧接追加；完成稿约第 8 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.\`organization_invitation\` TO 'collab_account'@'%';`。

<!-- write-step: 7 bytes=611:713 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.`organization_invitation` TO 'collab_account'@'%';
````

## 第 8 步：SQL-8

插入位置：第 7 步代码之后，紧接追加；完成稿约第 9 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.\`organization_application\` TO 'collab_account'@'%';`。

<!-- write-step: 8 bytes=713:816 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.`organization_application` TO 'collab_account'@'%';
````

## 第 9 步：SQL-9

插入位置：第 8 步代码之后，紧接追加；完成稿约第 10 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.\`organization_creation_grant\` TO 'collab_account'@'%';`。

<!-- write-step: 9 bytes=816:922 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.`organization_creation_grant` TO 'collab_account'@'%';
````

## 第 10 步：SQL-10

插入位置：第 9 步代码之后，紧接追加；完成稿约第 11 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.\`identity_admission\` TO 'collab_account'@'%';`。

<!-- write-step: 10 bytes=922:1019 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.`identity_admission` TO 'collab_account'@'%';
````

## 第 11 步：SQL-11

插入位置：第 10 步代码之后，紧接追加；完成稿约第 12 行。

本步声明：`GRANT SELECT, INSERT, DELETE ON account_db.\`admission_source\` TO 'collab_account'@'%';`。

<!-- write-step: 11 bytes=1019:1106 -->
````sql
GRANT SELECT, INSERT, DELETE ON account_db.`admission_source` TO 'collab_account'@'%';
````

## 第 12 步：SQL-12

插入位置：第 11 步代码之后，紧接追加；完成稿约第 13 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.\`account_close\` TO 'collab_account'@'%';`。

<!-- write-step: 12 bytes=1106:1198 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.`account_close` TO 'collab_account'@'%';
````

## 第 13 步：SQL-13

插入位置：第 12 步代码之后，紧接追加；完成稿约第 14 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.\`organization_transition\` TO 'collab_account'@'%';`。

<!-- write-step: 13 bytes=1198:1300 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.`organization_transition` TO 'collab_account'@'%';
````

## 第 14 步：SQL-14

插入位置：第 13 步代码之后，紧接追加；完成稿约第 15 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.\`operation_receipt\` TO 'collab_account'@'%';`。

<!-- write-step: 14 bytes=1300:1396 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.`operation_receipt` TO 'collab_account'@'%';
````

## 第 15 步：SQL-15

插入位置：第 14 步代码之后，紧接追加；完成稿约第 16 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.\`durable_task\` TO 'collab_account'@'%';`。

<!-- write-step: 15 bytes=1396:1487 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.`durable_task` TO 'collab_account'@'%';
````

## 第 16 步：SQL-16

插入位置：第 15 步代码之后，紧接追加；完成稿约第 17 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.\`access_gate\` TO 'collab_account'@'%';`。

<!-- write-step: 16 bytes=1487:1577 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.`access_gate` TO 'collab_account'@'%';
````

## 第 17 步：SQL-17

插入位置：第 16 步代码之后，紧接追加；完成稿约第 18 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.\`authorization_change\` TO 'collab_account'@'%';`。

<!-- write-step: 17 bytes=1577:1676 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.`authorization_change` TO 'collab_account'@'%';
````

## 第 18 步：SQL-18

插入位置：第 17 步代码之后，紧接追加；完成稿约第 19 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.\`authorization_ack\` TO 'collab_account'@'%';`。

<!-- write-step: 18 bytes=1676:1772 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.`authorization_ack` TO 'collab_account'@'%';
````

## 第 19 步：SQL-19

插入位置：第 18 步代码之后，紧接追加；完成稿约第 20 行。

本步声明：`GRANT SELECT, INSERT ON account_db.\`outbox\` TO 'collab_account'@'%';`。

<!-- write-step: 19 bytes=1772:1841 -->
````sql
GRANT SELECT, INSERT ON account_db.`outbox` TO 'collab_account'@'%';
````

## 第 20 步：SQL-20

插入位置：第 19 步代码之后，紧接追加；完成稿约第 21 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.\`notification\` TO 'collab_account'@'%';`。

<!-- write-step: 20 bytes=1841:1932 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON account_db.`notification` TO 'collab_account'@'%';
````

## 第 21 步：SQL-21

插入位置：第 20 步代码之后，紧接追加；完成稿约第 22 行。

本步声明：`GRANT SELECT, INSERT ON account_db.\`audit_event\` TO 'collab_account'@'%';`。

<!-- write-step: 21 bytes=1932:2006 -->
````sql
GRANT SELECT, INSERT ON account_db.`audit_event` TO 'collab_account'@'%';
````

## 第 22 步：SQL-22

插入位置：第 21 步代码之后，紧接追加；完成稿约第 23 行。

本步声明：`GRANT SELECT, INSERT, UPDATE ON account_db.\`platform_governance_grant\` TO 'collab_account'@'%';`。

<!-- write-step: 22 bytes=2006:2102 -->
````sql
GRANT SELECT, INSERT, UPDATE ON account_db.`platform_governance_grant` TO 'collab_account'@'%';
````

## 第 23 步：SQL-23

插入位置：第 22 步代码之后，紧接追加；完成稿约第 24 行。

本步声明：`GRANT SELECT ON document_db.schema_migration TO 'collab_document'@'%';`。

<!-- write-step: 23 bytes=2102:2173 -->
````sql
GRANT SELECT ON document_db.schema_migration TO 'collab_document'@'%';
````

## 第 24 步：SQL-24

插入位置：第 23 步代码之后，紧接追加；完成稿约第 25 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`space\` TO 'collab_document'@'%';`。

<!-- write-step: 24 bytes=2173:2259 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`space` TO 'collab_document'@'%';
````

## 第 25 步：SQL-25

插入位置：第 24 步代码之后，紧接追加；完成稿约第 26 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`knowledge_base\` TO 'collab_document'@'%';`。

<!-- write-step: 25 bytes=2259:2354 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`knowledge_base` TO 'collab_document'@'%';
````

## 第 26 步：SQL-26

插入位置：第 25 步代码之后，紧接追加；完成稿约第 27 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`knowledge_base_member\` TO 'collab_document'@'%';`。

<!-- write-step: 26 bytes=2354:2456 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`knowledge_base_member` TO 'collab_document'@'%';
````

## 第 27 步：SQL-27

插入位置：第 26 步代码之后，紧接追加；完成稿约第 28 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`knowledge_base_creation_grant\` TO 'collab_document'@'%';`。

<!-- write-step: 27 bytes=2456:2566 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`knowledge_base_creation_grant` TO 'collab_document'@'%';
````

## 第 28 步：SQL-28

插入位置：第 27 步代码之后，紧接追加；完成稿约第 29 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`admin_retirement\` TO 'collab_document'@'%';`。

<!-- write-step: 28 bytes=2566:2663 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`admin_retirement` TO 'collab_document'@'%';
````

## 第 29 步：SQL-29

插入位置：第 28 步代码之后，紧接追加；完成稿约第 30 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`document\` TO 'collab_document'@'%';`。

<!-- write-step: 29 bytes=2663:2752 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`document` TO 'collab_document'@'%';
````

## 第 30 步：SQL-30

插入位置：第 29 步代码之后，紧接追加；完成稿约第 31 行。

本步声明：`GRANT SELECT, INSERT, DELETE ON document_db.\`structure_revision\` TO 'collab_document'@'%';`。

<!-- write-step: 30 bytes=2752:2843 -->
````sql
GRANT SELECT, INSERT, DELETE ON document_db.`structure_revision` TO 'collab_document'@'%';
````

## 第 31 步：SQL-31

插入位置：第 30 步代码之后，紧接追加；完成稿约第 32 行。

本步声明：`GRANT SELECT, INSERT, DELETE ON document_db.\`style_revision\` TO 'collab_document'@'%';`。

<!-- write-step: 31 bytes=2843:2930 -->
````sql
GRANT SELECT, INSERT, DELETE ON document_db.`style_revision` TO 'collab_document'@'%';
````

## 第 32 步：SQL-32

插入位置：第 31 步代码之后，紧接追加；完成稿约第 33 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`chapter\` TO 'collab_document'@'%';`。

<!-- write-step: 32 bytes=2930:3018 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`chapter` TO 'collab_document'@'%';
````

## 第 33 步：SQL-33

插入位置：第 32 步代码之后，紧接追加；完成稿约第 34 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`collaboration_unit\` TO 'collab_document'@'%';`。

<!-- write-step: 33 bytes=3018:3117 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`collaboration_unit` TO 'collab_document'@'%';
````

## 第 34 步：SQL-34

插入位置：第 33 步代码之后，紧接追加；完成稿约第 35 行。

本步声明：`GRANT SELECT, INSERT, DELETE ON document_db.\`structure_entry\` TO 'collab_document'@'%';`。

<!-- write-step: 34 bytes=3117:3205 -->
````sql
GRANT SELECT, INSERT, DELETE ON document_db.`structure_entry` TO 'collab_document'@'%';
````

## 第 35 步：SQL-35

插入位置：第 34 步代码之后，紧接追加；完成稿约第 36 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`content_grant\` TO 'collab_document'@'%';`。

<!-- write-step: 35 bytes=3205:3299 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`content_grant` TO 'collab_document'@'%';
````

## 第 36 步：SQL-36

插入位置：第 35 步代码之后，紧接追加；完成稿约第 37 行。

本步声明：`GRANT SELECT, INSERT, DELETE ON document_db.\`content_version\` TO 'collab_document'@'%';`。

<!-- write-step: 36 bytes=3299:3387 -->
````sql
GRANT SELECT, INSERT, DELETE ON document_db.`content_version` TO 'collab_document'@'%';
````

## 第 37 步：SQL-37

插入位置：第 36 步代码之后，紧接追加；完成稿约第 38 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`draft_body_capacity\` TO 'collab_document'@'%';`。

<!-- write-step: 37 bytes=3387:3487 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`draft_body_capacity` TO 'collab_document'@'%';
````

## 第 38 步：SQL-38

插入位置：第 37 步代码之后，紧接追加；完成稿约第 39 行。

本步声明：`GRANT SELECT, INSERT, UPDATE ON document_db.\`draft_body_reservation\` TO 'collab_document'@'%';`。

<!-- write-step: 38 bytes=3487:3582 -->
````sql
GRANT SELECT, INSERT, UPDATE ON document_db.`draft_body_reservation` TO 'collab_document'@'%';
````

## 第 39 步：SQL-39

插入位置：第 38 步代码之后，紧接追加；完成稿约第 40 行。

本步声明：`GRANT SELECT, INSERT, DELETE ON document_db.\`reading_body_capacity\` TO 'collab_document'@'%';`。

<!-- write-step: 39 bytes=3582:3676 -->
````sql
GRANT SELECT, INSERT, DELETE ON document_db.`reading_body_capacity` TO 'collab_document'@'%';
````

## 第 40 步：SQL-40

插入位置：第 39 步代码之后，紧接追加；完成稿约第 41 行。

本步声明：`GRANT SELECT, INSERT, DELETE ON document_db.\`main_manifest\` TO 'collab_document'@'%';`。

<!-- write-step: 40 bytes=3676:3762 -->
````sql
GRANT SELECT, INSERT, DELETE ON document_db.`main_manifest` TO 'collab_document'@'%';
````

## 第 41 步：SQL-41

插入位置：第 40 步代码之后，紧接追加；完成稿约第 42 行。

本步声明：`GRANT SELECT, INSERT, DELETE ON document_db.\`manifest_entry\` TO 'collab_document'@'%';`。

<!-- write-step: 41 bytes=3762:3849 -->
````sql
GRANT SELECT, INSERT, DELETE ON document_db.`manifest_entry` TO 'collab_document'@'%';
````

## 第 42 步：SQL-42

插入位置：第 41 步代码之后，紧接追加；完成稿约第 43 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`branch\` TO 'collab_document'@'%';`。

<!-- write-step: 42 bytes=3849:3936 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`branch` TO 'collab_document'@'%';
````

## 第 43 步：SQL-43

插入位置：第 42 步代码之后，紧接追加；完成稿约第 44 行。

本步声明：`GRANT SELECT, INSERT, DELETE ON document_db.\`branch_lineage\` TO 'collab_document'@'%';`。

<!-- write-step: 43 bytes=3936:4023 -->
````sql
GRANT SELECT, INSERT, DELETE ON document_db.`branch_lineage` TO 'collab_document'@'%';
````

## 第 44 步：SQL-44

插入位置：第 43 步代码之后，紧接追加；完成稿约第 45 行。

本步声明：`GRANT SELECT, INSERT, DELETE ON document_db.\`unit_lineage\` TO 'collab_document'@'%';`。

<!-- write-step: 44 bytes=4023:4108 -->
````sql
GRANT SELECT, INSERT, DELETE ON document_db.`unit_lineage` TO 'collab_document'@'%';
````

## 第 45 步：SQL-45

插入位置：第 44 步代码之后，紧接追加；完成稿约第 46 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`review_request\` TO 'collab_document'@'%';`。

<!-- write-step: 45 bytes=4108:4203 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`review_request` TO 'collab_document'@'%';
````

## 第 46 步：SQL-46

插入位置：第 45 步代码之后，紧接追加；完成稿约第 47 行。

本步声明：`GRANT SELECT, INSERT, DELETE ON document_db.\`review_round\` TO 'collab_document'@'%';`。

<!-- write-step: 46 bytes=4203:4288 -->
````sql
GRANT SELECT, INSERT, DELETE ON document_db.`review_round` TO 'collab_document'@'%';
````

## 第 47 步：SQL-47

插入位置：第 46 步代码之后，紧接追加；完成稿约第 48 行。

本步声明：`GRANT SELECT, INSERT, DELETE ON document_db.\`request_submitter\` TO 'collab_document'@'%';`。

<!-- write-step: 47 bytes=4288:4378 -->
````sql
GRANT SELECT, INSERT, DELETE ON document_db.`request_submitter` TO 'collab_document'@'%';
````

## 第 48 步：SQL-48

插入位置：第 47 步代码之后，紧接追加；完成稿约第 49 行。

本步声明：`GRANT SELECT, INSERT, DELETE ON document_db.\`review_action\` TO 'collab_document'@'%';`。

<!-- write-step: 48 bytes=4378:4464 -->
````sql
GRANT SELECT, INSERT, DELETE ON document_db.`review_action` TO 'collab_document'@'%';
````

## 第 49 步：SQL-49

插入位置：第 48 步代码之后，紧接追加；完成稿约第 50 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`change_request\` TO 'collab_document'@'%';`。

<!-- write-step: 49 bytes=4464:4559 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`change_request` TO 'collab_document'@'%';
````

## 第 50 步：SQL-50

插入位置：第 49 步代码之后，紧接追加；完成稿约第 51 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`unit_review_guard\` TO 'collab_document'@'%';`。

<!-- write-step: 50 bytes=4559:4657 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`unit_review_guard` TO 'collab_document'@'%';
````

## 第 51 步：SQL-51

插入位置：第 50 步代码之后，紧接追加；完成稿约第 52 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`operation_receipt\` TO 'collab_document'@'%';`。

<!-- write-step: 51 bytes=4657:4755 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`operation_receipt` TO 'collab_document'@'%';
````

## 第 52 步：SQL-52

插入位置：第 51 步代码之后，紧接追加；完成稿约第 53 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`submission_operation\` TO 'collab_document'@'%';`。

<!-- write-step: 52 bytes=4755:4856 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`submission_operation` TO 'collab_document'@'%';
````

## 第 53 步：SQL-53

插入位置：第 52 步代码之后，紧接追加；完成稿约第 54 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`merge_operation\` TO 'collab_document'@'%';`。

<!-- write-step: 53 bytes=4856:4952 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`merge_operation` TO 'collab_document'@'%';
````

## 第 54 步：SQL-54

插入位置：第 53 步代码之后，紧接追加；完成稿约第 55 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`merge_attempt\` TO 'collab_document'@'%';`。

<!-- write-step: 54 bytes=4952:5046 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`merge_attempt` TO 'collab_document'@'%';
````

## 第 55 步：SQL-55

插入位置：第 54 步代码之后，紧接追加；完成稿约第 56 行。

本步声明：`GRANT SELECT, INSERT, DELETE, UPDATE(approval_action_id) ON document_db.\`merge_record\` TO 'collab_document'@'%';`。

<!-- write-step: 55 bytes=5046:5159 -->
````sql
GRANT SELECT, INSERT, DELETE, UPDATE(approval_action_id) ON document_db.`merge_record` TO 'collab_document'@'%';
````

## 第 56 步：SQL-56

插入位置：第 55 步代码之后，紧接追加；完成稿约第 57 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`main_commit_barrier\` TO 'collab_document'@'%';`。

<!-- write-step: 56 bytes=5159:5259 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`main_commit_barrier` TO 'collab_document'@'%';
````

## 第 57 步：SQL-57

插入位置：第 56 步代码之后，紧接追加；完成稿约第 58 行。

本步声明：`GRANT SELECT, INSERT, DELETE ON document_db.\`release\` TO 'collab_document'@'%';`。

<!-- write-step: 57 bytes=5259:5339 -->
````sql
GRANT SELECT, INSERT, DELETE ON document_db.`release` TO 'collab_document'@'%';
````

## 第 58 步：SQL-58

插入位置：第 57 步代码之后，紧接追加；完成稿约第 59 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`resource\` TO 'collab_document'@'%';`。

<!-- write-step: 58 bytes=5339:5428 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`resource` TO 'collab_document'@'%';
````

## 第 59 步：SQL-59

插入位置：第 58 步代码之后，紧接追加；完成稿约第 60 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`resource_reference\` TO 'collab_document'@'%';`。

<!-- write-step: 59 bytes=5428:5527 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`resource_reference` TO 'collab_document'@'%';
````

## 第 60 步：SQL-60

插入位置：第 59 步代码之后，紧接追加；完成稿约第 61 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`resource_protection\` TO 'collab_document'@'%';`。

<!-- write-step: 60 bytes=5527:5627 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`resource_protection` TO 'collab_document'@'%';
````

## 第 61 步：SQL-61

插入位置：第 60 步代码之后，紧接追加；完成稿约第 62 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`document_invitation\` TO 'collab_document'@'%';`。

<!-- write-step: 61 bytes=5627:5727 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`document_invitation` TO 'collab_document'@'%';
````

## 第 62 步：SQL-62

插入位置：第 61 步代码之后，紧接追加；完成稿约第 63 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`document_application\` TO 'collab_document'@'%';`。

<!-- write-step: 62 bytes=5727:5828 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`document_application` TO 'collab_document'@'%';
````

## 第 63 步：SQL-63

插入位置：第 62 步代码之后，紧接追加；完成稿约第 64 行。

本步声明：`GRANT SELECT, INSERT ON document_db.\`document_sharing_archive\` TO 'collab_document'@'%';`。

<!-- write-step: 63 bytes=5828:5917 -->
````sql
GRANT SELECT, INSERT ON document_db.`document_sharing_archive` TO 'collab_document'@'%';
````

## 第 64 步：SQL-64

插入位置：第 63 步代码之后，紧接追加；完成稿约第 65 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`quota_account\` TO 'collab_document'@'%';`。

<!-- write-step: 64 bytes=5917:6011 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`quota_account` TO 'collab_document'@'%';
````

## 第 65 步：SQL-65

插入位置：第 64 步代码之后，紧接追加；完成稿约第 66 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`quota_reservation\` TO 'collab_document'@'%';`。

<!-- write-step: 65 bytes=6011:6109 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`quota_reservation` TO 'collab_document'@'%';
````

## 第 66 步：SQL-66

插入位置：第 65 步代码之后，紧接追加；完成稿约第 67 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`temporary_budget\` TO 'collab_document'@'%';`。

<!-- write-step: 66 bytes=6109:6206 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`temporary_budget` TO 'collab_document'@'%';
````

## 第 67 步：SQL-67

插入位置：第 66 步代码之后，紧接追加；完成稿约第 68 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`comment_thread\` TO 'collab_document'@'%';`。

<!-- write-step: 67 bytes=6206:6301 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`comment_thread` TO 'collab_document'@'%';
````

## 第 68 步：SQL-68

插入位置：第 67 步代码之后，紧接追加；完成稿约第 69 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`comment\` TO 'collab_document'@'%';`。

<!-- write-step: 68 bytes=6301:6389 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`comment` TO 'collab_document'@'%';
````

## 第 69 步：SQL-69

插入位置：第 68 步代码之后，紧接追加；完成稿约第 70 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`export_job\` TO 'collab_document'@'%';`。

<!-- write-step: 69 bytes=6389:6480 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`export_job` TO 'collab_document'@'%';
````

## 第 70 步：SQL-70

插入位置：第 69 步代码之后，紧接追加；完成稿约第 71 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`lifecycle_operation\` TO 'collab_document'@'%';`。

<!-- write-step: 70 bytes=6480:6580 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`lifecycle_operation` TO 'collab_document'@'%';
````

## 第 71 步：SQL-71

插入位置：第 70 步代码之后，紧接追加；完成稿约第 72 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`read_view\` TO 'collab_document'@'%';`。

<!-- write-step: 71 bytes=6580:6670 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`read_view` TO 'collab_document'@'%';
````

## 第 72 步：SQL-72

插入位置：第 71 步代码之后，紧接追加；完成稿约第 73 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`document_admission\` TO 'collab_document'@'%';`。

<!-- write-step: 72 bytes=6670:6769 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`document_admission` TO 'collab_document'@'%';
````

## 第 73 步：SQL-73

插入位置：第 72 步代码之后，紧接追加；完成稿约第 74 行。

本步声明：`GRANT SELECT, INSERT, DELETE ON document_db.\`admission_source\` TO 'collab_document'@'%';`。

<!-- write-step: 73 bytes=6769:6858 -->
````sql
GRANT SELECT, INSERT, DELETE ON document_db.`admission_source` TO 'collab_document'@'%';
````

## 第 74 步：SQL-74

插入位置：第 73 步代码之后，紧接追加；完成稿约第 75 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`collaboration_ticket\` TO 'collab_document'@'%';`。

<!-- write-step: 74 bytes=6858:6959 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`collaboration_ticket` TO 'collab_document'@'%';
````

## 第 75 步：SQL-75

插入位置：第 74 步代码之后，紧接追加；完成稿约第 76 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`delivery_session\` TO 'collab_document'@'%';`。

<!-- write-step: 75 bytes=6959:7056 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`delivery_session` TO 'collab_document'@'%';
````

## 第 76 步：SQL-76

插入位置：第 75 步代码之后，紧接追加；完成稿约第 77 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`output_instance\` TO 'collab_document'@'%';`。

<!-- write-step: 76 bytes=7056:7152 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`output_instance` TO 'collab_document'@'%';
````

## 第 77 步：SQL-77

插入位置：第 76 步代码之后，紧接追加；完成稿约第 78 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`access_gate\` TO 'collab_document'@'%';`。

<!-- write-step: 77 bytes=7152:7244 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`access_gate` TO 'collab_document'@'%';
````

## 第 78 步：SQL-78

插入位置：第 77 步代码之后，紧接追加；完成稿约第 79 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`authorization_change\` TO 'collab_document'@'%';`。

<!-- write-step: 78 bytes=7244:7345 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`authorization_change` TO 'collab_document'@'%';
````

## 第 79 步：SQL-79

插入位置：第 78 步代码之后，紧接追加；完成稿约第 80 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`authorization_ack\` TO 'collab_document'@'%';`。

<!-- write-step: 79 bytes=7345:7443 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`authorization_ack` TO 'collab_document'@'%';
````

## 第 80 步：SQL-80

插入位置：第 79 步代码之后，紧接追加；完成稿约第 81 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`durable_task\` TO 'collab_document'@'%';`。

<!-- write-step: 80 bytes=7443:7536 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`durable_task` TO 'collab_document'@'%';
````

## 第 81 步：SQL-81

插入位置：第 80 步代码之后，紧接追加；完成稿约第 82 行。

本步声明：`GRANT SELECT, INSERT ON document_db.\`outbox\` TO 'collab_document'@'%';`。

<!-- write-step: 81 bytes=7536:7607 -->
````sql
GRANT SELECT, INSERT ON document_db.`outbox` TO 'collab_document'@'%';
````

## 第 82 步：SQL-82

插入位置：第 81 步代码之后，紧接追加；完成稿约第 83 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`notification\` TO 'collab_document'@'%';`。

<!-- write-step: 82 bytes=7607:7700 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`notification` TO 'collab_document'@'%';
````

## 第 83 步：SQL-83

插入位置：第 82 步代码之后，紧接追加；完成稿约第 84 行。

本步声明：`GRANT SELECT, INSERT ON document_db.\`audit_event\` TO 'collab_document'@'%';`。

<!-- write-step: 83 bytes=7700:7776 -->
````sql
GRANT SELECT, INSERT ON document_db.`audit_event` TO 'collab_document'@'%';
````

## 第 84 步：SQL-84

插入位置：第 83 步代码之后，紧接追加；完成稿约第 85 行。

本步声明：`GRANT SELECT ON collaboration_db.schema_migration TO 'collab_collaboration'@'%';`。

<!-- write-step: 84 bytes=7776:7857 -->
````sql
GRANT SELECT ON collaboration_db.schema_migration TO 'collab_collaboration'@'%';
````

## 第 85 步：SQL-85

插入位置：第 84 步代码之后，紧接追加；完成稿约第 86 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON collaboration_db.\`room\` TO 'collab_collaboration'@'%';`。

<!-- write-step: 85 bytes=7857:7952 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON collaboration_db.`room` TO 'collab_collaboration'@'%';
````

## 第 86 步：SQL-86

插入位置：第 85 步代码之后，紧接追加；完成稿约第 87 行。

本步声明：`GRANT SELECT, INSERT, DELETE ON collaboration_db.\`update_log\` TO 'collab_collaboration'@'%';`。

<!-- write-step: 86 bytes=7952:8045 -->
````sql
GRANT SELECT, INSERT, DELETE ON collaboration_db.`update_log` TO 'collab_collaboration'@'%';
````

## 第 87 步：SQL-87

插入位置：第 86 步代码之后，紧接追加；完成稿约第 88 行。

本步声明：`GRANT SELECT, INSERT, DELETE, UPDATE(validated_at) ON collaboration_db.\`checkpoint\` TO 'collab_collaboration'@'%';`。

<!-- write-step: 87 bytes=8045:8160 -->
````sql
GRANT SELECT, INSERT, DELETE, UPDATE(validated_at) ON collaboration_db.`checkpoint` TO 'collab_collaboration'@'%';
````

## 第 88 步：SQL-88

插入位置：第 87 步代码之后，紧接追加；完成稿约第 89 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON collaboration_db.\`collaboration_grant\` TO 'collab_collaboration'@'%';`。

<!-- write-step: 88 bytes=8160:8270 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON collaboration_db.`collaboration_grant` TO 'collab_collaboration'@'%';
````

## 第 89 步：SQL-89

插入位置：第 88 步代码之后，紧接追加；完成稿约第 90 行。

本步声明：`GRANT SELECT, INSERT, DELETE ON collaboration_db.\`grant_source\` TO 'collab_collaboration'@'%';`。

<!-- write-step: 89 bytes=8270:8365 -->
````sql
GRANT SELECT, INSERT, DELETE ON collaboration_db.`grant_source` TO 'collab_collaboration'@'%';
````

## 第 90 步：SQL-90

插入位置：第 89 步代码之后，紧接追加；完成稿约第 91 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON collaboration_db.\`flush_receipt\` TO 'collab_collaboration'@'%';`。

<!-- write-step: 90 bytes=8365:8469 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON collaboration_db.`flush_receipt` TO 'collab_collaboration'@'%';
````

## 第 91 步：SQL-91

插入位置：第 90 步代码之后，紧接追加；完成稿约第 92 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON collaboration_db.\`capture_operation\` TO 'collab_collaboration'@'%';`。

<!-- write-step: 91 bytes=8469:8577 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON collaboration_db.`capture_operation` TO 'collab_collaboration'@'%';
````

## 第 92 步：SQL-92

插入位置：第 91 步代码之后，紧接追加；完成稿约第 93 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON collaboration_db.\`main_barrier\` TO 'collab_collaboration'@'%';`。

<!-- write-step: 92 bytes=8577:8680 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON collaboration_db.`main_barrier` TO 'collab_collaboration'@'%';
````

## 第 93 步：SQL-93

插入位置：第 92 步代码之后，紧接追加；完成稿约第 94 行。

本步声明：`GRANT SELECT, INSERT, DELETE ON collaboration_db.\`install_receipt\` TO 'collab_collaboration'@'%';`。

<!-- write-step: 93 bytes=8680:8778 -->
````sql
GRANT SELECT, INSERT, DELETE ON collaboration_db.`install_receipt` TO 'collab_collaboration'@'%';
````

## 第 94 步：SQL-94

插入位置：第 93 步代码之后，紧接追加；完成稿约第 95 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON collaboration_db.\`delivery_session\` TO 'collab_collaboration'@'%';`。

<!-- write-step: 94 bytes=8778:8885 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON collaboration_db.`delivery_session` TO 'collab_collaboration'@'%';
````

## 第 95 步：SQL-95

插入位置：第 94 步代码之后，紧接追加；完成稿约第 96 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON collaboration_db.\`access_gate\` TO 'collab_collaboration'@'%';`。

<!-- write-step: 95 bytes=8885:8987 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON collaboration_db.`access_gate` TO 'collab_collaboration'@'%';
````

## 第 96 步：SQL-96

插入位置：第 95 步代码之后，紧接追加；完成稿约第 97 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON collaboration_db.\`authorization_change\` TO 'collab_collaboration'@'%';`。

<!-- write-step: 96 bytes=8987:9098 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON collaboration_db.`authorization_change` TO 'collab_collaboration'@'%';
````

## 第 97 步：SQL-97

插入位置：第 96 步代码之后，紧接追加；完成稿约第 98 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON collaboration_db.\`authorization_ack\` TO 'collab_collaboration'@'%';`。

<!-- write-step: 97 bytes=9098:9206 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON collaboration_db.`authorization_ack` TO 'collab_collaboration'@'%';
````

## 第 98 步：SQL-98

插入位置：第 97 步代码之后，紧接追加；完成稿约第 99 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON collaboration_db.\`durable_task\` TO 'collab_collaboration'@'%';`。

<!-- write-step: 98 bytes=9206:9309 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON collaboration_db.`durable_task` TO 'collab_collaboration'@'%';
````

## 第 99 步：SQL-99

插入位置：第 98 步代码之后，紧接追加；完成稿约第 100 行。

本步声明：`GRANT SELECT ON search_db.schema_migration TO 'collab_search'@'%';`。

<!-- write-step: 99 bytes=9309:9376 -->
````sql
GRANT SELECT ON search_db.schema_migration TO 'collab_search'@'%';
````

## 第 100 步：SQL-100

插入位置：第 99 步代码之后，紧接追加；完成稿约第 101 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON search_db.\`consumed_event\` TO 'collab_search'@'%';`。

<!-- write-step: 100 bytes=9376:9467 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON search_db.`consumed_event` TO 'collab_search'@'%';
````

## 第 101 步：SQL-101

插入位置：第 100 步代码之后，紧接追加；完成稿约第 102 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON search_db.\`document_index_revision\` TO 'collab_search'@'%';`。

<!-- write-step: 101 bytes=9467:9567 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON search_db.`document_index_revision` TO 'collab_search'@'%';
````

## 第 102 步：SQL-102

插入位置：第 101 步代码之后，紧接追加；完成稿约第 103 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON search_db.\`rebuild_job\` TO 'collab_search'@'%';`。

<!-- write-step: 102 bytes=9567:9655 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON search_db.`rebuild_job` TO 'collab_search'@'%';
````

## 第 103 步：SQL-103

插入位置：第 102 步代码之后，紧接追加；完成稿约第 104 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON search_db.\`index_generation\` TO 'collab_search'@'%';`。

<!-- write-step: 103 bytes=9655:9748 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON search_db.`index_generation` TO 'collab_search'@'%';
````

## 第 104 步：SQL-104

插入位置：第 103 步代码之后，紧接追加；完成稿约第 105 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON search_db.\`cursor_registry\` TO 'collab_search'@'%';`。

<!-- write-step: 104 bytes=9748:9840 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON search_db.`cursor_registry` TO 'collab_search'@'%';
````

## 第 105 步：SQL-105

插入位置：第 104 步代码之后，紧接追加；完成稿约第 106 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON search_db.\`durable_task\` TO 'collab_search'@'%';`。

<!-- write-step: 105 bytes=9840:9929 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON search_db.`durable_task` TO 'collab_search'@'%';
````

## 第 106 步：SQL-106

插入位置：第 105 步代码之后，紧接追加；完成稿约第 107 行。

003新增事实仍仅对Search应用账号开放，不提供Doc/Account私有表读取权限。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON search_db.\`event_origin\` TO 'collab_search'@'%';`。

<!-- write-step: 106 bytes=9929:10117 -->
````sql
-- 003新增事实仍仅对Search应用账号开放，不提供Doc/Account私有表读取权限。
GRANT SELECT, INSERT, UPDATE, DELETE ON search_db.`event_origin` TO 'collab_search'@'%';
````

## 第 107 步：SQL-107

插入位置：第 106 步代码之后，紧接追加；完成稿约第 109 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON search_db.\`index_application\` TO 'collab_search'@'%';`。

<!-- write-step: 107 bytes=10117:10211 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON search_db.`index_application` TO 'collab_search'@'%';
````

## 第 108 步：SQL-108

插入位置：第 107 步代码之后，紧接追加；完成稿约第 110 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON search_db.\`consumer_checkpoint\` TO 'collab_search'@'%';`。

<!-- write-step: 108 bytes=10211:10307 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON search_db.`consumer_checkpoint` TO 'collab_search'@'%';
````

## 第 109 步：SQL-109

插入位置：第 108 步代码之后，紧接追加；完成稿约第 111 行。

004快照原条目只追加；两侧应用账号仍无对方私有库权限。

本步声明：`GRANT SELECT, INSERT, UPDATE ON document_db.\`search_snapshot_job\` TO 'collab_document'@'%';`。

<!-- write-step: 109 bytes=10307:10481 -->
````sql
-- 004快照原条目只追加；两侧应用账号仍无对方私有库权限。
GRANT SELECT, INSERT, UPDATE ON document_db.`search_snapshot_job` TO 'collab_document'@'%';
````

## 第 110 步：SQL-110

插入位置：第 109 步代码之后，紧接追加；完成稿约第 113 行。

本步声明：`GRANT SELECT, INSERT ON document_db.\`search_snapshot_entry\` TO 'collab_document'@'%';`。

<!-- write-step: 110 bytes=10481:10567 -->
````sql
GRANT SELECT, INSERT ON document_db.`search_snapshot_entry` TO 'collab_document'@'%';
````

## 第 111 步：SQL-111

插入位置：第 110 步代码之后，紧接追加；完成稿约第 114 行。

本步声明：`GRANT SELECT, INSERT, UPDATE ON search_db.\`rebuild_binding\` TO 'collab_search'@'%';`。

<!-- write-step: 111 bytes=10567:10651 -->
````sql
GRANT SELECT, INSERT, UPDATE ON search_db.`rebuild_binding` TO 'collab_search'@'%';
````

## 第 112 步：SQL-112

插入位置：第 111 步代码之后，紧接追加；完成稿约第 115 行。

本步声明：`GRANT SELECT, INSERT, UPDATE ON search_db.\`rebuild_progress\` TO 'collab_search'@'%';`。

<!-- write-step: 112 bytes=10651:10736 -->
````sql
GRANT SELECT, INSERT, UPDATE ON search_db.`rebuild_progress` TO 'collab_search'@'%';
````

## 第 113 步：SQL-113

插入位置：第 112 步代码之后，紧接追加；完成稿约第 116 行。

005物理日志绑定不可改写或删除，应用只能首次追加并核对原身份。

本步声明：`GRANT SELECT, INSERT ON search_db.\`consumer_log_identity\` TO 'collab_search'@'%';`。

<!-- write-step: 113 bytes=10736:10912 -->
````sql
-- 005物理日志绑定不可改写或删除，应用只能首次追加并核对原身份。
GRANT SELECT, INSERT ON search_db.`consumer_log_identity` TO 'collab_search'@'%';
````

## 第 114 步：SQL-114

插入位置：第 113 步代码之后，紧接追加；完成稿约第 118 行。

008导出固定计划与临时额度只属于Doc，其他服务仍不能读取它的私有事实。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`export_control\` TO 'collab_document'@'%';`。

<!-- write-step: 114 bytes=10912:11110 -->
````sql
-- 008导出固定计划与临时额度只属于Doc，其他服务仍不能读取它的私有事实。
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`export_control` TO 'collab_document'@'%';
````

## 第 115 步：SQL-115

插入位置：第 114 步代码之后，紧接追加；完成稿约第 120 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`export_budget\` TO 'collab_document'@'%';`。

<!-- write-step: 115 bytes=11110:11204 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`export_budget` TO 'collab_document'@'%';
````

## 第 116 步：SQL-116

插入位置：第 115 步代码之后，紧接追加；完成稿约第 121 行。

010：归属迁移与容量分配只属于 Doc 私有库，其他服务仍无表访问权限。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`personal_transfer\` TO 'collab_document'@'%';`。

<!-- write-step: 116 bytes=11204:11401 -->
````sql
-- 010：归属迁移与容量分配只属于 Doc 私有库，其他服务仍无表访问权限。
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`personal_transfer` TO 'collab_document'@'%';
````

## 第 117 步：SQL-117

插入位置：第 116 步代码之后，紧接追加；完成稿约第 123 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`document_capacity\` TO 'collab_document'@'%';`。

<!-- write-step: 117 bytes=11401:11499 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`document_capacity` TO 'collab_document'@'%';
````

## 第 118 步：SQL-118

插入位置：第 117 步代码之后，紧接追加；完成稿约第 124 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`resource_space_allocation\` TO 'collab_document'@'%';`。

<!-- write-step: 118 bytes=11499:11605 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`resource_space_allocation` TO 'collab_document'@'%';
````

## 第 119 步：SQL-119

插入位置：第 118 步代码之后，紧接追加；完成稿约第 125 行。

本步声明：`GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.\`document_resource_allocation\` TO 'collab_document'@'%';`。

<!-- write-step: 119 bytes=11605:11714 -->
````sql
GRANT SELECT, INSERT, UPDATE, DELETE ON document_db.`document_resource_allocation` TO 'collab_document'@'%';
````

## 文件完成后的检查

SQL 此时只编写，不在体验实例执行。所有编号迁移由 06 阶段在自己独立实例按顺序安装，不能为了检查语法删库或重复安装。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/foundation/02-private-databases.md)。
