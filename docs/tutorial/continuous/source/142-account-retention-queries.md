# services/document/internal/repository/document/account_retention_queries.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/142-account-retention-queries.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`AccountRetentionCandidates`、`DiscoverAccountRetention`、`LockAccountRetentionEntities`、`SaveAccountRetention`、`saveRetentionPlan`、`LockAccountRetentionCompletion`、`CompleteAccountRetentionEntities`、`CompleteAccountRetention`、`AccountRetentionDrained`、`retentionDrained`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	"collaboration/platform"
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"slices"
	"strings"
)

// account_retention.go：保留原实现，按业务合并文件。

type AccountRetentionCandidates struct {
	Spaces         []uint64
	KnowledgeBases []uint64
	Documents      []uint64
	Sources        []domain.LeaseSource
}

// 候选读取不授予清理资格。Account已确认身份排空，因此自有个人空间和唯一成员
// 组织不能再合法建库/建文档；现有协作者的修改仍须在来源锁后当前读复核。
func DiscoverAccountRetention(ctx context.Context, q Queryer, binding domain.AccountRetirementBinding) (AccountRetentionCandidates, error) {
	c := AccountRetentionCandidates{Spaces: []uint64{}, KnowledgeBases: []uint64{}, Documents: []uint64{}, Sources: []domain.LeaseSource{}}
	statement := `SELECT id,kind,COALESCE(account_owner_id,0),COALESCE(organization_id,0),lifecycle FROM space
		WHERE (kind='PERSONAL' AND account_owner_id=?)`
	args := []any{binding.User}
	if len(binding.ClosingOrganizations) > 0 {
		statement += " OR (kind='ORGANIZATION' AND organization_id IN (" + strings.TrimSuffix(strings.Repeat("?,", len(binding.ClosingOrganizations)), ",") + "))"
		for _, org := range binding.ClosingOrganizations {
			args = append(args, org)
		}
	}
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
		if kind == "ORGANIZATION" && !slices.Contains(binding.ClosingOrganizations, org) {
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
		rows, err = q.QueryContext(ctx, "SELECT id,acl_revision FROM knowledge_base WHERE space_id=? ORDER BY id", space)
		if err != nil {
			return c, err
		}
		for rows.Next() {
			var id, epoch uint64
			if err = rows.Scan(&id, &epoch); err != nil {
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

// 所有来源锁先于所有Doc、KB、Space锁。避免逐文档读取父对象导致跨文档锁序倒置。
func LockAccountRetentionEntities(ctx context.Context, tx *sql.Tx, c AccountRetentionCandidates) error {
	for _, group := range []struct {
		table string
		ids   []uint64
	}{{"document", c.Documents}, {"knowledge_base", c.KnowledgeBases}, {"space", c.Spaces}} {
		for _, id := range group.ids {
			var actual uint64
			if err := tx.QueryRowContext(ctx, "SELECT id FROM "+group.table+" WHERE id=? FOR UPDATE", id).Scan(&actual); err != nil {
				return err
			}
		}
	}
	return nil
}

func SaveAccountRetention(ctx context.Context, tx *sql.Tx, operation string, raw []byte, plan domain.AccountRetentionPlan) error {
	return saveRetentionPlan(ctx, tx, operation, raw, plan, "ACCOUNT_CLOSE")
}

// 调用方持有原来源/实体锁；这里只复用写入，不能另开事务或隐式提交。
func saveRetentionPlan(ctx context.Context, tx *sql.Tx, operation string, raw []byte, plan domain.AccountRetentionPlan, kind string) error {
	for _, d := range plan.Documents {
		if _, err := tx.ExecContext(ctx, `UPDATE document SET lifecycle='CLOSING',main_locked=TRUE,acl_revision=acl_revision+1,
			publication_epoch=publication_epoch+1,control_revision=control_revision+1 WHERE id=?`, d.Document); err != nil {
			return err
		}
	}
	for _, kb := range plan.KnowledgeBases {
		if _, err := tx.ExecContext(ctx, "UPDATE knowledge_base SET lifecycle='CLOSING',acl_revision=acl_revision+1 WHERE id=?", kb); err != nil {
			return err
		}
	}
	for _, space := range plan.Spaces {
		if _, err := tx.ExecContext(ctx, "UPDATE space SET lifecycle='CLOSING' WHERE id=?", space); err != nil {
			return err
		}
	}
	result, err := tx.ExecContext(ctx, "UPDATE lifecycle_operation SET state='PLANNED',fixed_plan=?,generation=generation+1 WHERE operation_id=? AND kind=? AND state='FROZEN'", string(raw), operation, kind)
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

// account_retention_complete.go：保留原实现，按业务合并文件。

// 所有来源和原lifecycle行均已锁定，先锁全部Doc→KB→Space；读取不会自行
// 选择新文档或新的公开版本。保留原metadata行，上传恢复仍使用原document绑定。
func LockAccountRetentionCompletion(ctx context.Context, tx *sql.Tx, plan domain.AccountRetentionPlan) error {
	candidates := AccountRetentionCandidates{Spaces: plan.Spaces, KnowledgeBases: plan.KnowledgeBases}
	for _, item := range plan.Documents {
		candidates.Documents = append(candidates.Documents, item.Document)
	}
	if err := LockAccountRetentionEntities(ctx, tx, candidates); err != nil {
		return err
	}
	for _, kb := range plan.KnowledgeBases {
		var state string
		if err := tx.QueryRowContext(ctx, "SELECT lifecycle FROM knowledge_base WHERE id=?", kb).Scan(&state); err != nil {
			return err
		}
		if state != "CLOSING" {
			return errors.New("ACCOUNT_RETENTION_PARENT_CHANGED")
		}
	}
	for _, space := range plan.Spaces {
		var state string
		if err := tx.QueryRowContext(ctx, "SELECT lifecycle FROM space WHERE id=?", space).Scan(&state); err != nil {
			return err
		}
		if state != "CLOSING" {
			return errors.New("ACCOUNT_RETENTION_SPACE_CHANGED")
		}
	}
	return nil
}

// 只在全部原清理回执及固定依赖核验之后调用。更高代次只开放已固定公开
// 正式版的输出；原身份/私有文档来源保持BLOCKED，内部写入仍被lifecycle拒绝。
func CompleteAccountRetentionEntities(ctx context.Context, tx *sql.Tx, plan domain.AccountRetentionPlan) error {
	retainedKB, retainedSpace := map[uint64]bool{}, map[uint64]bool{}
	for _, item := range plan.Documents {
		if item.Release == 0 {
			if _, err := tx.ExecContext(ctx, "UPDATE document SET lifecycle='PURGED',deleted_at=COALESCE(deleted_at,UTC_TIMESTAMP(6)),purge_after=NULL WHERE id=? AND lifecycle='CLOSING'", item.Document); err != nil {
				return err
			}
			continue
		}
		retainedSpace[item.Space] = true
		if item.KnowledgeBase != 0 {
			retainedKB[item.KnowledgeBase] = true
		}
		var acl, publication uint64
		if err := tx.QueryRowContext(ctx, "SELECT acl_revision,publication_epoch FROM document WHERE id=?", item.Document).Scan(&acl, &publication); err != nil {
			return err
		}
		if acl == math.MaxUint64 || publication == math.MaxUint64 {
			return domain.Violation(domain.StateConflict, "ACCOUNT_RETENTION_EPOCH_EXHAUSTED")
		}
		if _, err := tx.ExecContext(ctx, "UPDATE document SET lifecycle='PUBLIC_RETAINED',acl_revision=?,publication_epoch=?,main_locked=TRUE,deleted_at=NULL,purge_after=NULL WHERE id=? AND lifecycle='CLOSING'", acl+1, publication+1, item.Document); err != nil {
			return err
		}
		for _, source := range []struct {
			key   string
			epoch uint64
		}{{"doc-acl:" + domain.TextID(item.Document), acl + 1}, {"publication:" + domain.TextID(item.Document), publication + 1}} {
			if err := ReopenSourceGate(ctx, tx, source.key, source.epoch); err != nil {
				return err
			}
		}
	}
	for _, kb := range plan.KnowledgeBases {
		state := "CLOSED"
		if retainedKB[kb] {
			state = "PUBLIC_RETAINED"
			var acl uint64
			if err := tx.QueryRowContext(ctx, "SELECT acl_revision FROM knowledge_base WHERE id=?", kb).Scan(&acl); err != nil {
				return err
			}
			if acl == math.MaxUint64 {
				return domain.Violation(domain.StateConflict, "ACCOUNT_RETENTION_EPOCH_EXHAUSTED")
			}
			if _, err := tx.ExecContext(ctx, "UPDATE knowledge_base SET acl_revision=? WHERE id=?", acl+1, kb); err != nil {
				return err
			}
			if err := ReopenSourceGate(ctx, tx, "kb-acl:"+domain.TextID(kb), acl+1); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, "UPDATE knowledge_base SET lifecycle=?,name=IF(?='CLOSED','已关闭知识库',name) WHERE id=?", state, state, kb); err != nil {
			return err
		}
		// 关闭父库的其他外部成员也不能继续持有可变管理关系；不清除存续库。
		for _, table := range []string{"knowledge_base_member", "knowledge_base_creation_grant"} {
			if _, err := tx.ExecContext(ctx, "UPDATE "+table+" SET state='CLOSED',revision=revision+1 WHERE knowledge_base_id=? AND state<>'CLOSED'", kb); err != nil {
				return err
			}
		}
	}
	for _, space := range plan.Spaces {
		state := "CLOSED"
		if retainedSpace[space] {
			state = "PUBLIC_RETAINED"
		}
		if _, err := tx.ExecContext(ctx, "UPDATE space SET lifecycle=? WHERE id=? AND lifecycle='CLOSING'", state, space); err != nil {
			return err
		}
	}
	return nil
}

func CompleteAccountRetention(ctx context.Context, tx *sql.Tx, operation string) error {
	result, err := tx.ExecContext(ctx, "UPDATE lifecycle_operation SET state='SUCCEEDED',generation=generation+1 WHERE operation_id=? AND kind='ACCOUNT_CLOSE' AND state='CLEANING'", operation)
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

// account_retention_drains.go：保留原实现，按业务合并文件。

// 不能只数ACK：原task、原授权变更、gate和两方摘要必须同时匹配固定计划。
// 调用方先锁来源；任务只能从非终态推进到终态，未知提交通过原记录恢复。
func AccountRetentionDrained(ctx context.Context, tx *sql.Tx, plan domain.AccountRetirementPlan) (bool, error) {
	return retentionDrained(ctx, tx, plan.Retention, plan.Binding.User, "ACCOUNT_RETENTION")
}

func retentionDrained(ctx context.Context, tx *sql.Tx, retention *domain.AccountRetentionPlan, user uint64, action string) (bool, error) {
	if retention == nil {
		return false, errors.New("ACCOUNT_RETENTION_PLAN_REQUIRED")
	}
	for _, scope := range retention.Scopes {
		var taskOperation, taskState, authState, key, gateState, revocation string
		var epoch, gateEpoch, actor uint64
		var payload, target, participants []byte
		err := tx.QueryRowContext(ctx, `SELECT t.operation_id,t.state,t.payload,c.state,c.scope_key,c.epoch,c.target_permission,g.max_epoch,g.state,COALESCE(g.last_revocation_id,''),COALESCE(c.actor_id,0),c.participant_plan
			FROM durable_task t JOIN authorization_change c ON c.revocation_id=t.business_key JOIN access_gate g ON g.scope_key=c.scope_key
			WHERE t.kind='AUTHORIZATION_CHANGE' AND t.business_key=?`, scope.Operation).Scan(&taskOperation, &taskState, &payload, &authState, &key, &epoch, &target, &gateEpoch, &gateState, &revocation, &actor, &participants)
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		var closure domain.Closure
		var binding map[string]string
		var participantPlan []string
		request := &pb.FenceRequest{RevocationId: scope.Operation, ClosedScopes: []*pb.SourceEpoch{{ScopeKey: scope.Key, Epoch: scope.Epoch}}, Generation: 1, TargetDigest: platform.SourceClosureDigest(scope.Key, scope.Epoch)}
		if json.Unmarshal(payload, &closure) != nil || json.Unmarshal(target, &binding) != nil || json.Unmarshal(participants, &participantPlan) != nil {
			return false, errors.New("ACCOUNT_RETENTION_DRAIN_BINDING_CHANGED")
		}
		taskMatches := taskOperation == scope.Operation && closure.Scope == scope.Key && closure.Epoch == scope.Epoch && closure.Action == action && closure.Entity == 0
		authorizationMatches := actor == user && key == scope.Key && epoch == scope.Epoch && slices.Equal(participantPlan, []string{"edge", "collaboration"})
		gateMatches := gateEpoch == scope.Epoch && gateState == "BLOCKED" && revocation == scope.Operation
		digestMatches := binding["generation"] == "1" && binding["bootstrap_target_digest"] == hex.EncodeToString(request.TargetDigest)
		if !taskMatches || !authorizationMatches || !gateMatches || !digestMatches {
			return false, errors.New("ACCOUNT_RETENTION_DRAIN_BINDING_CHANGED")
		}
		if taskState != "SUCCEEDED" || authState != "SUCCEEDED" {
			return false, nil
		}
		for _, participant := range []string{"edge", "collaboration"} {
			var digest []byte
			err := tx.QueryRowContext(ctx, "SELECT result_digest FROM authorization_ack WHERE revocation_id=? AND participant=? AND scope_key=? AND epoch=? AND phase='OUTPUT_STOPPED'", scope.Operation, participant, scope.Key, scope.Epoch).Scan(&digest)
			if errors.Is(err, sql.ErrNoRows) {
				return false, nil
			}
			if err != nil {
				return false, err
			}
			if !bytes.Equal(digest, platform.BootstrapDigest(request, participant)) {
				return false, errors.New("ACCOUNT_RETENTION_ACK_CHANGED")
			}
		}
	}
	return true, nil
}
````

UTF-8/LF全文SHA-256：`cafc2e6f15a8b52bab3b6e48a65ce37a6b31d7c5d1b26a26d9cfc0eb6bdb805e`。[参考实现](../../../../services/document/internal/repository/document/account_retention_queries.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
