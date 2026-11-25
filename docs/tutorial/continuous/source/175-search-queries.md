# services/document/internal/repository/document/search_queries.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/175-search-queries.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`SearchPublicationFacts`、`SearchManifestMetadata`、`searchFactRows`、`sortedSearchFactIDs`、`searchPublicationFactsBatch`、`searchKnowledgeBaseFacts`、`searchSpaceFacts`、`validOrderedSearchSources`、`LockSourceGatesBatch`、`PendingSourceTransitionsBatch`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	"collaboration/platform"
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
)

// search_facts.go：保留原实现，按业务合并文件。

// SearchPublicationFacts只使用调用方原事务。一批候选由用例在同一只读一致快照下
// 加载；不能逐项另开事务，也不能把未知KB/Space/SQL事实降级为确定拒绝候选。
func SearchPublicationFacts(ctx context.Context, tx *sql.Tx, ids []uint64) (map[uint64]domain.PublicationFacts, error) {
	invalid := domain.Violation(domain.InvalidRequest, "SEARCH_PUBLICATION_FACTS_INVALID")
	if tx == nil || len(ids) > 100 {
		return nil, invalid
	}
	seen := make(map[uint64]bool, len(ids))
	for _, id := range ids {
		if id == 0 || seen[id] {
			return nil, invalid
		}
		seen[id] = true
	}
	// 保留同一原快照及完整事实，只将N项Doc/KB/Space往返合并为至多三次查询。
	// 最终授权事务仍逐行当前读/锁定；这里的批量读取不能替代最终资格复核。
	return searchPublicationFactsBatch(ctx, tx, ids)
}

// SearchManifestMetadata不读取canonical_body/body_json/provenance。用例必须先以
// 固定structure及ManifestUnits证明清单完整，再将这里的ByteSize交给预算规则；
// 预留成功后才读正文并严格验证实际len/hash/schema，不能让错误byte_size绕过预算。
func SearchManifestMetadata(ctx context.Context, tx *sql.Tx, manifest, document uint64, expectedUnits int) ([]domain.SearchManifestUnit, error) {
	invalid := domain.Violation(domain.TemporarilyUnavailable, "SEARCH_FIXED_METADATA_INVALID")
	if tx == nil || manifest == 0 || document == 0 || expectedUnits < 0 || expectedUnits > domain.SearchManifestMaxUnits {
		return nil, invalid
	}
	// 即使固定结构为空也核验manifest归属；缺manifest不能成为零单位成功。
	var owner string
	if err := tx.QueryRowContext(ctx, "SELECT CAST(document_id AS CHAR) FROM main_manifest WHERE id=?", manifest).Scan(&owner); err != nil {
		return nil, invalid
	}
	ownerID, err := domain.CanonicalID(owner)
	if err != nil || ownerID != document {
		return nil, invalid
	}
	// 不按e.document_id筛选，否则同manifest的异常跨Doc额外行会被隐藏；LEFT JOIN
	// 保留缺失版本事实。多读一条即可证明全清单超数，异常清单仍不无限加载。
	rows, err := tx.QueryContext(ctx, `SELECT CAST(e.document_id AS CHAR),CAST(e.unit_id AS CHAR),
		CAST(e.content_version_id AS CHAR),CAST(v.document_id AS CHAR),CAST(v.unit_id AS CHAR),CAST(v.byte_size AS CHAR)
		FROM manifest_entry e LEFT JOIN content_version v ON v.id=e.content_version_id
		WHERE e.manifest_id=? ORDER BY e.unit_id LIMIT ?`, manifest, expectedUnits+1)
	if err != nil {
		return nil, invalid
	}
	defer rows.Close()
	units := make([]domain.SearchManifestUnit, 0, expectedUnits)
	seen := make(map[uint64]bool, expectedUnits)
	for rows.Next() {
		if len(units) >= expectedUnits {
			return nil, invalid
		}
		var entryDoc, entryUnit, entryVersion string
		var versionDoc, versionUnit, versionBytes sql.NullString
		if rows.Scan(&entryDoc, &entryUnit, &entryVersion, &versionDoc, &versionUnit, &versionBytes) != nil ||
			!versionDoc.Valid || !versionUnit.Valid || !versionBytes.Valid {
			return nil, invalid
		}
		entryDocument, e1 := domain.CanonicalID(entryDoc)
		unit, e2 := domain.CanonicalID(entryUnit)
		version, e3 := domain.CanonicalID(entryVersion)
		contentDocument, e4 := domain.CanonicalID(versionDoc.String)
		contentUnit, e5 := domain.CanonicalID(versionUnit.String)
		byteSize, e6 := domain.CanonicalID(versionBytes.String)
		if e1 != nil || e2 != nil || e3 != nil || e4 != nil || e5 != nil || e6 != nil ||
			entryDocument != document || contentDocument != document || contentUnit != unit || seen[unit] {
			return nil, invalid
		}
		seen[unit] = true
		units = append(units, domain.SearchManifestUnit{Unit: unit, Version: version, ByteSize: byteSize})
	}
	if rows.Err() != nil || len(units) != expectedUnits {
		return nil, invalid
	}
	return units, nil
}

// search_facts_batch.go：保留原实现，按业务合并文件。

func searchFactRows(ctx context.Context, tx *sql.Tx, table, columns string, ids []uint64) (*sql.Rows, error) {
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	// table/columns只取本文件固定常量；所有候选与关联ID用参数绑定，最多原批100项。
	marks := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	return tx.QueryContext(ctx, "SELECT "+columns+" FROM "+table+" WHERE id IN ("+marks+") ORDER BY id", args...)
}

func sortedSearchFactIDs(ids map[uint64]bool) []uint64 {
	ordered := make([]uint64, 0, len(ids))
	for id := range ids {
		ordered = append(ordered, id)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	return ordered
}

func searchPublicationFactsBatch(ctx context.Context, tx *sql.Tx, ids []uint64) (map[uint64]domain.PublicationFacts, error) {
	facts := make(map[uint64]domain.PublicationFacts, len(ids))
	if len(ids) == 0 {
		return facts, nil
	}
	unknown := domain.Violation(domain.TemporarilyUnavailable, "SEARCH_PUBLICATION_FACTS_UNKNOWN")
	rows, err := searchFactRows(ctx, tx, "document", publicationDocumentColumns, ids)
	if err != nil {
		return nil, unknown
	}
	kbIDs, spaceIDs := make(map[uint64]bool), make(map[uint64]bool)
	for rows.Next() {
		d, sequence, err := scanPublicationDocument(rows)
		id, e1 := domain.CanonicalID(d.Id)
		space, e2 := domain.CanonicalID(d.SpaceId)
		if err != nil || e1 != nil || e2 != nil {
			rows.Close()
			return nil, unknown
		}
		facts[id] = domain.PublicationFacts{Document: d, Sequence: sequence}
		spaceIDs[space] = true
		if d.KnowledgeBaseId != nil {
			kb, err := domain.CanonicalID(*d.KnowledgeBaseId)
			if err != nil {
				rows.Close()
				return nil, domain.Violation(domain.TemporarilyUnavailable, "PUBLIC_KNOWLEDGE_BASE_UNKNOWN")
			}
			kbIDs[kb] = true
		}
	}
	readErr := rows.Err()
	rows.Close()
	if readErr != nil {
		return nil, unknown
	}
	kbs, err := searchKnowledgeBaseFacts(ctx, tx, sortedSearchFactIDs(kbIDs))
	if err != nil {
		return nil, err
	}
	spaces, err := searchSpaceFacts(ctx, tx, sortedSearchFactIDs(spaceIDs))
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		value, present := facts[id]
		if !present {
			continue // 确定缺Doc仅排除该候选；关联缺失/SQL未知使整批失败。
		}
		if value.Document.KnowledgeBaseId != nil {
			kbID, _ := domain.CanonicalID(*value.Document.KnowledgeBaseId)
			kb, present := kbs[kbID]
			if !present || kb.SpaceId != value.Document.SpaceId {
				return nil, domain.Violation(domain.TemporarilyUnavailable, "PUBLIC_KNOWLEDGE_BASE_UNKNOWN")
			}
			value.KnowledgeBase = &kb
		}
		spaceID, _ := domain.CanonicalID(value.Document.SpaceId)
		space, present := spaces[spaceID]
		if !present {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "PUBLIC_SPACE_UNKNOWN")
		}
		value.Space = space
		facts[id] = value
	}
	return facts, nil
}

func searchKnowledgeBaseFacts(ctx context.Context, tx *sql.Tx, ids []uint64) (map[uint64]dto.KnowledgeBase, error) {
	values := make(map[uint64]dto.KnowledgeBase, len(ids))
	if len(ids) == 0 {
		return values, nil
	}
	unknown := domain.Violation(domain.TemporarilyUnavailable, "PUBLIC_KNOWLEDGE_BASE_UNKNOWN")
	rows, err := searchFactRows(ctx, tx, "knowledge_base", publicationKnowledgeBaseColumns, ids)
	if err != nil {
		return nil, unknown
	}
	defer rows.Close()
	for rows.Next() {
		value, err := scanPublicationKnowledgeBase(rows)
		id, invalid := domain.CanonicalID(value.Id)
		if err != nil || invalid != nil {
			return nil, unknown
		}
		values[id] = value
	}
	if rows.Err() != nil {
		return nil, unknown
	}
	return values, nil
}

func searchSpaceFacts(ctx context.Context, tx *sql.Tx, ids []uint64) (map[uint64]dto.Space, error) {
	values := make(map[uint64]dto.Space, len(ids))
	if len(ids) == 0 {
		return values, nil
	}
	unknown := domain.Violation(domain.TemporarilyUnavailable, "PUBLIC_SPACE_UNKNOWN")
	rows, err := searchFactRows(ctx, tx, "space", publicationSpaceColumns, ids)
	if err != nil {
		return nil, unknown
	}
	defer rows.Close()
	for rows.Next() {
		value, err := scanPublicationSpace(rows)
		id, invalid := domain.CanonicalID(value.Id)
		if err != nil || invalid != nil {
			return nil, unknown
		}
		values[id] = value
	}
	if rows.Err() != nil {
		return nil, unknown
	}
	return values, nil
}

// search_sources.go：保留原实现，按业务合并文件。

// 一页最多100项及一个lookahead；来源为Doc/KB和身份的去重并集。
// 上限只约束SQL包大小，超限整页失败，不截断任何权限来源。
const maxSearchOutputSources = 1024

func validOrderedSearchSources(sources []domain.LeaseSource) bool {
	if len(sources) == 0 || len(sources) > maxSearchOutputSources {
		return false
	}
	for i, source := range sources {
		if _, err := domain.CanonicalID(source.Epoch); err != nil || (!platform.ValidDocumentScope(source.Key) && !platform.ValidAccountOutputScope(source.Key)) {
			return false
		}
		if i > 0 && sources[i-1].Key >= source.Key {
			return false
		}
	}
	return true
}

// 调用方原事务持有整批排他gate锁；按与旧逐项路径相同的scope顺序取得锁。
// 不能用INSERT IGNORE（共享锁升级），也不能覆盖已有BLOCKED或更高epoch。
func LockSourceGatesBatch(ctx context.Context, tx *sql.Tx, sources []domain.LeaseSource) (map[string]SourceGate, error) {
	if tx == nil || !validOrderedSearchSources(sources) {
		return nil, errors.New("SOURCE_BINDING_INVALID")
	}
	values, keys := make([]string, len(sources)), make([]any, len(sources))
	args := make([]any, 0, len(sources)*2)
	for i, source := range sources {
		values[i] = "(?,?,'OPEN')"
		args = append(args, source.Key, source.Epoch)
		keys[i] = source.Key
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO access_gate(scope_key,max_epoch,state) VALUES"+strings.Join(values, ",")+" ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)", args...); err != nil {
		return nil, err
	}
	marks := strings.TrimSuffix(strings.Repeat("?,", len(keys)), ",")
	rows, err := tx.QueryContext(ctx, "SELECT scope_key,max_epoch,state FROM access_gate FORCE INDEX(PRIMARY) WHERE scope_key IN ("+marks+") ORDER BY scope_key FOR UPDATE", keys...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	gates := make(map[string]SourceGate, len(sources))
	for rows.Next() {
		var key string
		var gate SourceGate
		if rows.Scan(&key, &gate.Epoch, &gate.State) != nil || len(gates) >= len(sources) || key != sources[len(gates)].Key || gate.Epoch == 0 || (gate.State != "OPEN" && gate.State != "BLOCKED") {
			return nil, errors.New("GATE_UNAVAILABLE")
		}
		gates[key] = gate
	}
	if rows.Err() != nil || len(gates) != len(sources) {
		return nil, errors.New("GATE_UNAVAILABLE")
	}
	return gates, nil
}

// 必须在全部gate排他锁之后调用。使用scope索引当前读，不能读事务旧快照，
// 也不能让优化器走state索引先锁无关过渡；RR仍可能取得gap/next-key锁。
func PendingSourceTransitionsBatch(ctx context.Context, tx *sql.Tx, keys []string) (bool, error) {
	if tx == nil || len(keys) > maxSearchOutputSources {
		return false, errors.New("SOURCE_BINDING_INVALID")
	}
	if len(keys) == 0 {
		return false, nil
	}
	args := make([]any, len(keys))
	for i, key := range keys {
		if !platform.ValidDocumentScope(key) || (i > 0 && keys[i-1] >= key) {
			return false, errors.New("SOURCE_BINDING_INVALID")
		}
		args[i] = key
	}
	marks := strings.TrimSuffix(strings.Repeat("?,", len(keys)), ",")
	var state string
	err := tx.QueryRowContext(ctx, "SELECT state FROM authorization_change FORCE INDEX(auth_scope_epoch) WHERE scope_key IN ("+marks+") AND state NOT IN ('SUCCEEDED','REJECTED') ORDER BY scope_key,epoch LIMIT 1 FOR UPDATE", args...).Scan(&state)
	if err == sql.ErrNoRows {
		return PersonalTransferSourcesPending(ctx, tx, keys)
	}
	return err == nil, err
}
````

UTF-8/LF全文SHA-256：`6c41832afacd7df4092f5fa2beec496117c7c965757d9a2b435c7e90cabc11bb`。[参考实现](../../../../services/document/internal/repository/document/search_queries.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
