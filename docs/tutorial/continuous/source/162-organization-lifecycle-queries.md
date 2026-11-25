# services/document/internal/repository/document/organization_lifecycle_queries.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/162-organization-lifecycle-queries.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`DiscoverOrganizationClose`、`SaveOrganizationClose`、`CompleteOrganizationClose`、`LockOrganizationCleanupDocument`、`OrganizationCloseDrained`、`OrganizationCloseReady`、`OrganizationCloseGateMatches`、`RecordOrganizationClose`、`StartOrganizationCleaning`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	domain "collaboration/document/internal/domain/document"
	"collaboration/platform"
	"context"
	"database/sql"
	"errors"
	"slices"
)

// organization_close.go：保留原实现，按业务合并文件。

// 候选只读取原组织空间；Account组织来源ACK不冻结本人身份或个人空间。
// 外部KB协作者仍可能完成旧写入，必须在来源锁和业务锁后重新核对候选。
func DiscoverOrganizationClose(ctx context.Context, q Queryer, binding platform.OrganizationClosureBinding) (AccountRetentionCandidates, error) {
	c := AccountRetentionCandidates{Spaces: []uint64{}, KnowledgeBases: []uint64{}, Documents: []uint64{}, Sources: []domain.LeaseSource{}}
	statement := `SELECT id,kind,COALESCE(account_owner_id,0),COALESCE(organization_id,0),lifecycle FROM space WHERE kind='ORGANIZATION' AND organization_id=?`
	args := []any{binding.Organization}
	rows, err := q.QueryContext(ctx, statement+" ORDER BY id", args...)
	if err != nil {
		return c, err
	}
	for rows.Next() {
		var id, owner, org uint64
		var kind, state string
		if err = rows.Scan(&id, &kind, &owner, &org, &state); err != nil {
			break
		}
		if kind != "ORGANIZATION" || org != binding.Organization {
			continue
		}
		if state != "ACTIVE" {
			err = errors.New("ACCOUNT_RETENTION_SPACE_TRANSITION_PENDING")
			break
		}
		c.Spaces = append(c.Spaces, id)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return c, err
	}
	for _, space := range c.Spaces {
		rows, err = q.QueryContext(ctx, "SELECT id,acl_revision,lifecycle FROM knowledge_base WHERE space_id=? ORDER BY id", space)
		if err != nil {
			return c, err
		}
		for rows.Next() {
			var id, epoch uint64
			var kbState string
			if err = rows.Scan(&id, &epoch, &kbState); err != nil {
				break
			}
			if kbState != "ACTIVE" && kbState != "CLOSED" {
				err = errors.New("ORGANIZATION_CLOSE_KB_TRANSITION_PENDING")
				break
			}
			c.KnowledgeBases = append(c.KnowledgeBases, id)
			c.Sources = append(c.Sources, domain.LeaseSource{Key: "kb-acl:" + domain.TextID(id), Epoch: domain.TextID(epoch)})
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		if err != nil {
			return c, err
		}
		rows, err = q.QueryContext(ctx, "SELECT id,acl_revision,publication_epoch FROM document WHERE space_id=? AND lifecycle<>'PURGED' ORDER BY id", space)
		if err != nil {
			return c, err
		}
		for rows.Next() {
			var id, acl, publication uint64
			if err = rows.Scan(&id, &acl, &publication); err != nil {
				break
			}
			c.Documents = append(c.Documents, id)
			c.Sources = append(c.Sources, domain.LeaseSource{Key: "doc-acl:" + domain.TextID(id), Epoch: domain.TextID(acl)}, domain.LeaseSource{Key: "publication:" + domain.TextID(id), Epoch: domain.TextID(publication)})
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		if err != nil {
			return c, err
		}
	}
	slices.Sort(c.KnowledgeBases)
	slices.Sort(c.Documents)
	slices.SortFunc(c.Sources, func(a, b domain.LeaseSource) int {
		if a.Key < b.Key {
			return -1
		}
		if a.Key > b.Key {
			return 1
		}
		return 0
	})
	return c, nil
}

func SaveOrganizationClose(ctx context.Context, tx *sql.Tx, operation string, raw []byte, plan domain.AccountRetentionPlan) error {
	return saveRetentionPlan(ctx, tx, operation, raw, plan, "ORGANIZATION_CLOSE")
}

// organization_close_complete.go：保留原实现，按业务合并文件。

func CompleteOrganizationClose(ctx context.Context, tx *sql.Tx, operation string) error {
	result, err := tx.ExecContext(ctx, "UPDATE lifecycle_operation SET state='SUCCEEDED',generation=generation+1 WHERE operation_id=? AND kind='ORGANIZATION_CLOSE' AND state='CLEANING'", operation)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return errors.New("ACCOUNT_RETENTION_STATE_CHANGED")
	}
	return nil
}

// organization_close_document.go：保留原实现，按业务合并文件。

// 来源锁、原lifecycle锁之后按Doc→KB→Space顺序锁定，固定父关联与关闭代次。
// 不从“父对象已不存在”推断清理完成；丢失或改变的metadata必须交给原流程恢复。
func LockOrganizationCleanupDocument(ctx context.Context, tx *sql.Tx, plan domain.OrganizationClosePlan, item domain.AccountRetentionDocument, scopes []domain.AccountRetentionScope) error {
	epochs := map[string]uint64{}
	for _, scope := range scopes {
		epochs[scope.Key] = scope.Epoch
	}
	var space, kb, acl, publication uint64
	var state string
	err := tx.QueryRowContext(ctx, `SELECT space_id,COALESCE(knowledge_base_id,0),lifecycle,acl_revision,publication_epoch
		FROM document WHERE id=? FOR UPDATE`, item.Document).Scan(&space, &kb, &state, &acl, &publication)
	if err != nil {
		return err
	}
	if space != item.Space || kb != item.KnowledgeBase || state != "CLOSING" || acl != epochs["doc-acl:"+domain.TextID(item.Document)] || publication != epochs["publication:"+domain.TextID(item.Document)] {
		return errors.New("ACCOUNT_DOCUMENT_METADATA_CHANGED")
	}
	if kb != 0 {
		var parent, revision uint64
		if err = tx.QueryRowContext(ctx, "SELECT space_id,lifecycle,acl_revision FROM knowledge_base WHERE id=? FOR UPDATE", kb).Scan(&parent, &state, &revision); err != nil {
			return err
		}
		if parent != space || state != "CLOSING" || revision != epochs["kb-acl:"+domain.TextID(kb)] {
			return errors.New("ACCOUNT_DOCUMENT_PARENT_CHANGED")
		}
	}
	var kind string
	var organization uint64
	if err = tx.QueryRowContext(ctx, `SELECT kind,lifecycle,COALESCE(organization_id,0)
		FROM space WHERE id=? FOR UPDATE`, space).Scan(&kind, &state, &organization); err != nil {
		return err
	}
	closingOrganization := kind == "ORGANIZATION" && organization == plan.Binding.Organization
	if state != "CLOSING" || !closingOrganization {
		return errors.New("ACCOUNT_DOCUMENT_SPACE_CHANGED")
	}
	return nil
}

// organization_close_drains.go：保留原实现，按业务合并文件。

// 不能只数ACK：原task、原授权变更、gate和两方摘要必须同时匹配固定计划。
// 调用方先锁来源；任务只能从非终态推进到终态，未知提交通过原记录恢复。
func OrganizationCloseDrained(ctx context.Context, tx *sql.Tx, plan domain.OrganizationClosePlan) (bool, error) {
	return retentionDrained(ctx, tx, plan.Retention, plan.Binding.User, "ORGANIZATION_RETENTION")
}

// organization_close_ready.go：保留原实现，按业务合并文件。

// Ready只检查本流程的非终态记录；新原号仍由Account恢复器推动，Doc已有通用来源worker排空。
func OrganizationCloseReady(ctx context.Context, db *sql.DB) error {
	var after string
	for {
		rows, err := db.QueryContext(ctx, `SELECT operation_id,COALESCE(actor_id,0),state,expected_revision,fixed_plan FROM lifecycle_operation
		 WHERE kind='ORGANIZATION_CLOSE' AND state NOT IN ('SUCCEEDED','REJECTED') AND operation_id>? ORDER BY operation_id LIMIT 100`, after)
		if err != nil {
			return err
		}
		count := 0
		for rows.Next() {
			var operation, state string
			var actor, epoch uint64
			var raw []byte
			if err = rows.Scan(&operation, &actor, &state, &epoch, &raw); err != nil {
				break
			}
			plan, decodeErr := domain.DecodeOrganizationClosePlan(raw)
			if decodeErr != nil || plan.Binding.Operation != operation || plan.Binding.User != actor || plan.Binding.Epoch != epoch || (state != "PLANNED" && state != "CLEANING") {
				err = errors.New("ORGANIZATION_CLOSE_PLAN_INVALID")
				break
			}
			after, count = operation, count+1
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		if err != nil {
			return err
		}
		if count < 100 {
			return nil
		}
	}
}

// organization_close_source.go：保留原实现，按业务合并文件。

// 原组织来源由 Account 的真实 FenceAccess 安装，不写入身份gate或伪造墓碑。
func OrganizationCloseGateMatches(ctx context.Context, q Queryer, b platform.OrganizationClosureBinding) (bool, error) {
	var matches bool
	err := q.QueryRowContext(ctx, `SELECT max_epoch=? AND state='BLOCKED' AND last_revocation_id=? FROM access_gate WHERE scope_key=?`, b.Epoch, b.Operation, b.Scope()).Scan(&matches)
	return matches, err
}

func RecordOrganizationClose(ctx context.Context, tx *sql.Tx, b platform.OrganizationClosureBinding, raw []byte) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO lifecycle_operation(operation_id,actor_id,kind,state,expected_revision,fixed_plan)
	 VALUES(?,?,'ORGANIZATION_CLOSE','FROZEN',?,?)`, b.Operation, b.User, b.Epoch, string(raw))
	return err
}

func StartOrganizationCleaning(ctx context.Context, tx *sql.Tx, operation string) error {
	_, err := tx.ExecContext(ctx, "UPDATE lifecycle_operation SET state='CLEANING',generation=generation+1 WHERE operation_id=? AND kind='ORGANIZATION_CLOSE' AND state='PLANNED'", operation)
	return err
}
````

UTF-8/LF全文SHA-256：`58cc28c0587de9e869833a73a6864a934e94c737f9d75993120ff614ac9634ff`。[参考实现](../../../../services/document/internal/repository/document/organization_lifecycle_queries.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
