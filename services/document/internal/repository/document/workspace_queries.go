// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"errors"
)

// directory.go：保留原实现，按业务合并文件。

func ExplicitKBVisibility(ctx context.Context, q Queryer, kb, actor uint64) (bool, error) {
	var visible bool
	err := q.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM knowledge_base_member WHERE knowledge_base_id=? AND user_id=? AND state='ACTIVE') OR EXISTS(SELECT 1 FROM knowledge_base_creation_grant WHERE knowledge_base_id=? AND user_id=? AND state='ACTIVE')", kb, actor, kb, actor).Scan(&visible)
	return visible, err
}
func DerivedDirectorySources(ctx context.Context, q Queryer, kb, actor uint64) ([]domain.DirectorySource, error) {
	rows, err := q.QueryContext(ctx, "SELECT d.id,d.acl_revision FROM document d WHERE d.knowledge_base_id=? AND d.lifecycle='ACTIVE' AND (d.responsible_user_id=? OR EXISTS(SELECT 1 FROM content_grant g WHERE g.document_id=d.id AND g.user_id=? AND g.state='ACTIVE' AND (g.capability<>'EDIT' OR EXISTS(SELECT 1 FROM collaboration_unit u JOIN structure_entry e ON e.unit_id=u.id AND e.document_id=u.document_id AND e.structure_id=d.current_structure_id WHERE u.document_id=d.id AND u.retired_structure_id IS NULL AND ((g.scope_kind='UNIT' AND g.scope_id=u.id) OR (g.scope_kind='CHAPTER' AND g.scope_id=u.chapter_id)))))) ORDER BY d.id LIMIT 401", kb, actor, actor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sources []domain.DirectorySource
	for rows.Next() {
		var source domain.DirectorySource
		if rows.Scan(&source.ID, &source.Epoch) != nil {
			return nil, errors.New("DIRECTORY_SOURCE_UNAVAILABLE")
		}
		sources = append(sources, source)
	}
	return sources, rows.Err()
}
func KBCreationAllowed(ctx context.Context, tx *sql.Tx, kb, actor uint64) (bool, error) {
	var allowed bool
	err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM knowledge_base_creation_grant WHERE knowledge_base_id=? AND user_id=? AND state='ACTIVE')", kb, actor).Scan(&allowed)
	return allowed, err
}
func KnowledgeBaseCandidates(ctx context.Context, db *sql.DB, space uint64, last string, budget int, search ...string) ([]uint64, error) {
	query := ""
	if len(search) > 0 {
		query = search[0]
	}
	rows, err := db.QueryContext(ctx, "SELECT id FROM knowledge_base WHERE space_id=? AND lifecycle='ACTIVE' AND id>? AND (?='' OR LOCATE(?,LOWER(name))>0) ORDER BY id LIMIT ?", space, last, query, query, budget)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var candidates []uint64
	for rows.Next() {
		var id uint64
		if rows.Scan(&id) != nil {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "DIRECTORY_UNAVAILABLE")
		}
		candidates = append(candidates, id)
	}
	return candidates, rows.Err()
}
func DocumentCandidates(ctx context.Context, db *sql.DB, space, kb uint64, last string, budget int) ([]uint64, error) {
	statement := "SELECT id FROM document WHERE space_id=? AND lifecycle='ACTIVE' AND id>?"
	args := []any{space, last}
	if kb != 0 {
		statement += " AND knowledge_base_id=?"
		args = append(args, kb)
	}
	statement += " ORDER BY id LIMIT ?"
	args = append(args, budget)
	rows, err := db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var candidates []uint64
	for rows.Next() {
		var id uint64
		if rows.Scan(&id) != nil {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "DIRECTORY_UNAVAILABLE")
		}
		candidates = append(candidates, id)
	}
	return candidates, rows.Err()
}
func KBMemberCandidates(ctx context.Context, db *sql.DB, kb uint64, last string, budget int) ([]dto.Member, error) {
	rows, err := db.QueryContext(ctx, "SELECT CAST(user_id AS CHAR),role,state,CAST(revision AS CHAR) FROM knowledge_base_member WHERE knowledge_base_id=? AND state='ACTIVE' AND user_id>? ORDER BY user_id LIMIT ?", kb, last, budget)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var candidates []dto.Member
	for rows.Next() {
		var member dto.Member
		if rows.Scan(&member.UserId, &member.Role, &member.State, &member.Revision) != nil {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "KB_MEMBERS_UNAVAILABLE")
		}
		candidates = append(candidates, member)
	}
	return candidates, rows.Err()
}
func LockedKBMember(ctx context.Context, tx *sql.Tx, kb uint64, user string) (string, string, error) {
	var revision, state string
	err := tx.QueryRowContext(ctx, "SELECT CAST(revision AS CHAR),state FROM knowledge_base_member WHERE knowledge_base_id=? AND user_id=? FOR UPDATE", kb, user).Scan(&revision, &state)
	return revision, state, err
}

// 文档协作者来自本服务的现行授权事实；UNION同时去掉多个授权范围和负责人产生的重复项。
// 这里只读取Document私有库，不把账号资料表联进SQL，也不把候选当成新的访问授权。
func DocumentCollaboratorCandidates(ctx context.Context, q Queryer, document uint64, last string, budget int) ([]uint64, error) {
	rows, err := q.QueryContext(ctx, `SELECT candidate.user_id FROM (
		SELECT user_id FROM content_grant WHERE document_id=? AND state='ACTIVE'
		UNION SELECT responsible_user_id FROM document WHERE id=? AND responsible_user_id IS NOT NULL
		UNION SELECT s.account_owner_id FROM document d JOIN space s ON s.id=d.space_id
			WHERE d.id=? AND s.kind='PERSONAL' AND s.account_owner_id IS NOT NULL
	) candidate WHERE candidate.user_id>? ORDER BY candidate.user_id LIMIT ?`, document, document, document, last, budget)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var candidates []uint64
	for rows.Next() {
		var user uint64
		if err := rows.Scan(&user); err != nil {
			return nil, err
		}
		candidates = append(candidates, user)
	}
	return candidates, rows.Err()
}

// initialization.go：保留原实现，按业务合并文件。

func InsertID(ctx context.Context, tx *sql.Tx, statement string, args ...any) (uint64, error) {
	result, err := tx.ExecContext(ctx, statement, args...)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	return uint64(id), err
}
func CreateDocument(ctx context.Context, tx *sql.Tx, space, kb, actor uint64, title, visibility string, locked bool) (uint64, error) {
	var knowledgeBase any
	if kb != 0 {
		knowledgeBase = kb
	}
	return InsertID(ctx, tx, "INSERT INTO document(space_id,knowledge_base_id,responsible_user_id,title,visibility_policy,lifecycle,main_locked,publication_state) VALUES(?,?,?,?,?,'INITIALIZING',?,'UNPUBLISHED')", space, knowledgeBase, actor, title, visibility, locked)
}
func CreateInitialStructure(ctx context.Context, tx *sql.Tx, document uint64, operation string) (uint64, error) {
	return InsertID(ctx, tx, "INSERT INTO structure_revision(document_id,source_operation_id,schema_version) VALUES(?,?,'structure-v1')", document, operation)
}
func CreateInitialStyle(ctx context.Context, tx *sql.Tx, document uint64, style, operation string) (uint64, error) {
	return InsertID(ctx, tx, "INSERT INTO style_revision(document_id,style_json,schema_version,source_operation_id) VALUES(?,?,'style-v1',?)", document, style, operation)
}
func CreateInitialManifest(ctx context.Context, tx *sql.Tx, document, structure, style uint64, operation string) (uint64, error) {
	return InsertID(ctx, tx, "INSERT INTO main_manifest(document_id,structure_id,style_id,source_operation_id,product_key,reliable_cut_ref) VALUES(?,?,?,?,'INITIAL',?)", document, structure, style, operation, "INITIAL:"+operation)
}
func CreateInitialChapter(ctx context.Context, tx *sql.Tx, document, structure uint64) (uint64, error) {
	return InsertID(ctx, tx, "INSERT INTO chapter(document_id,created_structure_id) VALUES(?,?)", document, structure)
}
func CreateInitialUnit(ctx context.Context, tx *sql.Tx, document, chapter, structure uint64) (uint64, error) {
	return InsertID(ctx, tx, "INSERT INTO collaboration_unit(document_id,chapter_id,kind,created_structure_id) VALUES(?,?,'CHAPTER',?)", document, chapter, structure)
}
func InsertInitialStructureEntry(ctx context.Context, tx *sql.Tx, document, structure, chapter, unit uint64) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO structure_entry(structure_id,document_id,node_id,chapter_id,kind,title,sort_order,unit_id) VALUES(?,?,?,?,'CHAPTER','第一章',1,?)", structure, document, chapter, chapter, unit)
	return err
}
func InsertInitialManifestEntry(ctx context.Context, tx *sql.Tx, document, manifest, unit, version uint64) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO manifest_entry(manifest_id,document_id,unit_id,content_version_id) VALUES(?,?,?,?)", manifest, document, unit, version)
	return err
}
func ActivateDocument(ctx context.Context, tx *sql.Tx, document, structure, style, manifest uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE document SET current_structure_id=?,current_style_id=?,current_manifest_id=?,lifecycle='ACTIVE' WHERE id=? AND lifecycle='INITIALIZING'", structure, style, manifest, document)
	return err
}
func InsertInitialContent(ctx context.Context, tx *sql.Tx, document, unit, structure uint64, operation string, content domain.InitialContent) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO content_version(id,document_id,unit_id,structure_id,body_json,canonical_body,body_hash,schema_version,canonicalization_version,byte_size,source_operation_id,source_kind,source_ref,provenance_json,provenance_schema) VALUES(?,?,?,?,?,?,?,'content-v1','canonical-v1',?,?,'INITIAL',?,?,'provenance-v1')", content.Version, document, unit, structure, string(content.Canonical), content.Canonical, content.Hash[:], len(content.Canonical), operation, domain.TextID(unit), string(content.Provenance))
	if err != nil {
		return err
	}
	return ChargeStoredVersion(ctx, tx, document, uint64(len(content.Canonical)), operation)
}
func CreateInitialBranch(ctx context.Context, tx *sql.Tx, document, unit, version uint64) (uint64, error) {
	return InsertID(ctx, tx, "INSERT INTO branch(document_id,unit_id,lifecycle,initial_base_version_id) VALUES(?,?,'PREPARING',?)", document, unit, version)
}
func InsertInitialLineage(ctx context.Context, tx *sql.Tx, document, unit, branch, manifest uint64, operation string, content domain.InitialContent) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,event_kind,source_operation_id,base_manifest_id,base_version_id,provenance_json,provenance_schema) VALUES(?,?,?,?,'CREATED',?,?,?,?,'provenance-v1')", content.Lineage, branch, unit, document, operation, manifest, content.Version, string(content.Provenance))
	return err
}
func ActivateInitialBranch(ctx context.Context, tx *sql.Tx, branch, lineage uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE branch SET current_lineage_id=?,lifecycle='ACTIVE' WHERE id=?", lineage, branch)
	return err
}
func BindInitialBranch(ctx context.Context, tx *sql.Tx, document, unit, branch uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE collaboration_unit SET active_branch_id=? WHERE id=? AND document_id=?", branch, unit, document)
	return err
}
func LockedCreationKnowledgeBase(ctx context.Context, tx *sql.Tx, id uint64) (domain.CreationKnowledgeBase, error) {
	return CreationKnowledgeBase(ctx, tx, id, true)
}

func CreationKnowledgeBase(ctx context.Context, q Queryer, id uint64, lock bool) (domain.CreationKnowledgeBase, error) {
	var kb domain.CreationKnowledgeBase
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	err := q.QueryRowContext(ctx, "SELECT space_id,lifecycle,acl_revision FROM knowledge_base WHERE id=?"+suffix, id).Scan(&kb.Space, &kb.Lifecycle, &kb.Revision)
	return kb, err
}

func KnowledgeBaseACL(ctx context.Context, q Queryer, id uint64) (uint64, error) {
	var revision uint64
	err := q.QueryRowContext(ctx, "SELECT acl_revision FROM knowledge_base WHERE id=?", id).Scan(&revision)
	return revision, err
}
func KnowledgeBaseCreationState(ctx context.Context, tx *sql.Tx, id, user uint64) (string, error) {
	var state string
	err := tx.QueryRowContext(ctx, "SELECT state FROM knowledge_base_creation_grant WHERE knowledge_base_id=? AND user_id=?", id, user).Scan(&state)
	return state, err
}

// knowledge_base.go：保留原实现，按业务合并文件。

func KnowledgeBaseRow(ctx context.Context, q Queryer, id uint64, lock bool) (dto.KnowledgeBase, error) {
	var kb dto.KnowledgeBase
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	err := q.QueryRowContext(ctx, "SELECT CAST(id AS CHAR),CAST(space_id AS CHAR),name,lifecycle,CAST(acl_revision AS CHAR),visibility_policy,CAST(visibility_revision AS CHAR) FROM knowledge_base WHERE id=?"+suffix, id).Scan(&kb.Id, &kb.SpaceId, &kb.Name, &kb.Lifecycle, &kb.AclRevision, &kb.VisibilityPolicy, &kb.VisibilityRevision)
	if err != nil || kb.Lifecycle != "ACTIVE" {
		return kb, domain.Violation(domain.EntityNotFound, "KNOWLEDGE_BASE_NOT_FOUND")
	}
	return kb, nil
}
func KnowledgeBaseRole(ctx context.Context, q Queryer, id, user uint64) (string, error) {
	var role string
	err := q.QueryRowContext(ctx, "SELECT role FROM knowledge_base_member WHERE knowledge_base_id=? AND user_id=? AND state='ACTIVE'", id, user).Scan(&role)
	return role, err
}
func CreateKnowledgeBase(ctx context.Context, tx *sql.Tx, space uint64, name, visibility string) (uint64, error) {
	result, err := tx.ExecContext(ctx, "INSERT INTO knowledge_base(space_id,name,lifecycle,visibility_policy) VALUES(?,?,'ACTIVE',?)", space, name, visibility)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	return uint64(id), err
}
func InsertInitialKBAdministrator(ctx context.Context, tx *sql.Tx, id, actor uint64) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO knowledge_base_member(knowledge_base_id,user_id,role,state) VALUES(?,?,'ADMIN','ACTIVE')", id, actor)
	return err
}
func AvailableKBAdministrators(ctx context.Context, tx *sql.Tx, id uint64) (int, error) {
	// 调用方已持有KB锁。角色事务可能更早建立RR快照，必须用锁定当前读
	// 看到刚提交的注销预留，否则仍会把待退出者算作最后管理员的接任者。
	candidate := AdministratorRetirementCandidate{KnowledgeBase: id}
	err := lockAdministratorRetirementFacts(ctx, tx, 0, &candidate)
	return candidate.Available, err
}
func KBAdministratorReservations(ctx context.Context, tx *sql.Tx, id, user uint64) (int, error) {
	var state string
	err := tx.QueryRowContext(ctx, "SELECT state FROM admin_retirement WHERE knowledge_base_id=? AND user_id=? FOR UPDATE", id, user).Scan(&state)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if state == "RESERVED" {
		return 1, nil
	}
	return 0, nil
}
func SetKBRole(ctx context.Context, tx *sql.Tx, id, user uint64, role, state string) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO knowledge_base_member(knowledge_base_id,user_id,role,state) VALUES(?,?,?,?) ON DUPLICATE KEY UPDATE role=VALUES(role),state=VALUES(state),revision=revision+1", id, user, role, state)
	return err
}
func SetKBCreationGrant(ctx context.Context, tx *sql.Tx, id, user uint64, state string) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO knowledge_base_creation_grant(knowledge_base_id,user_id,state) VALUES(?,?,?) ON DUPLICATE KEY UPDATE state=VALUES(state),revision=revision+1", id, user, state)
	return err
}
func AdvanceKnowledgeBaseACL(ctx context.Context, tx *sql.Tx, id uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE knowledge_base SET acl_revision=acl_revision+1 WHERE id=?", id)
	return err
}
func RenameKnowledgeBase(ctx context.Context, tx *sql.Tx, id uint64, name string) error {
	_, err := tx.ExecContext(ctx, "UPDATE knowledge_base SET name=?,acl_revision=acl_revision+1 WHERE id=?", name, id)
	return err
}

// knowledge_base_children.go：保留原实现，按业务合并文件。

func KnowledgeBaseChildren(ctx context.Context, q Queryer, id uint64, locked bool) ([]domain.KnowledgeBaseChild, error) {
	statement := "SELECT id,acl_revision FROM document WHERE knowledge_base_id=? AND lifecycle='ACTIVE' ORDER BY id LIMIT 401"
	if locked {
		statement = "SELECT id,acl_revision,current_release_id FROM document WHERE knowledge_base_id=? AND lifecycle='ACTIVE' ORDER BY id FOR UPDATE"
	}
	rows, err := q.QueryContext(ctx, statement, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var children []domain.KnowledgeBaseChild
	for rows.Next() {
		var child domain.KnowledgeBaseChild
		if locked {
			var release sql.NullInt64
			err = rows.Scan(&child.ID, &child.Epoch, &release)
			child.HasRelease = release.Valid
		} else {
			err = rows.Scan(&child.ID, &child.Epoch)
		}
		if err != nil {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "KB_CHILDREN_UNAVAILABLE")
		}
		children = append(children, child)
	}
	return children, rows.Err()
}
func CloseKnowledgeBaseForDeletion(ctx context.Context, tx *sql.Tx, id uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE knowledge_base SET lifecycle='CLOSING',acl_revision=acl_revision+1 WHERE id=?", id)
	return err
}
func CloseKnowledgeBaseChildren(ctx context.Context, tx *sql.Tx, id uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE document SET lifecycle='CLOSING',control_revision=control_revision+1,acl_revision=acl_revision+1 WHERE knowledge_base_id=? AND lifecycle='ACTIVE'", id)
	return err
}

// management.go：保留原实现，按业务合并文件。

func RenameDocument(ctx context.Context, tx *sql.Tx, id uint64, title string) error {
	_, err := tx.ExecContext(ctx, "UPDATE document SET title=?,control_revision=control_revision+1 WHERE id=?", title, id)
	return err
}
func TransferDocumentResponsibility(ctx context.Context, tx *sql.Tx, id, user uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE document SET responsible_user_id=?,control_revision=control_revision+1,acl_revision=acl_revision+1 WHERE id=?", user, id)
	return err
}
func CloseDocument(ctx context.Context, tx *sql.Tx, id uint64) error {
	// 历史release依赖保留在回收站；只撤下当前展示，不修改不可变正式内容。
	_, err := tx.ExecContext(ctx, "UPDATE document SET lifecycle='CLOSING',control_revision=control_revision+1,acl_revision=acl_revision+1,publication_epoch=publication_epoch+1,publication_state='UNPUBLISHED',current_release_id=NULL WHERE id=? AND lifecycle='ACTIVE'", id)
	return err
}
func NotifyResponsibilityTransfer(ctx context.Context, tx *sql.Tx, user uint64, operation string, target []byte) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO notification(recipient_id,event_id,kind,target_json) VALUES(?,?,'RESPONSIBLE_TRANSFER',?)", user, operation, string(target))
	return err
}

// space.go：保留原实现，按业务合并文件。

func EnsureSpaceGate(ctx context.Context, tx *sql.Tx, key string) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO access_gate(scope_key,max_epoch,state) VALUES(?,1,'OPEN') ON DUPLICATE KEY UPDATE scope_key=VALUES(scope_key)", key)
	return err
}
func LockSpaceGate(ctx context.Context, tx *sql.Tx, key string) error {
	var revision uint64
	if tx.QueryRowContext(ctx, "SELECT revision FROM access_gate WHERE scope_key=? FOR UPDATE", key).Scan(&revision) != nil {
		return errors.New("SPACE_UNAVAILABLE")
	}
	return nil
}
func spaceOwnerField(kind string) string {
	if kind == "ORGANIZATION" {
		return "organization_id"
	}
	return "account_owner_id"
}
func ExistingSpace(ctx context.Context, tx *sql.Tx, kind string, owner uint64) (uint64, error) {
	var id uint64
	err := tx.QueryRowContext(ctx, "SELECT id FROM space WHERE kind=? AND "+spaceOwnerField(kind)+"=? ORDER BY id LIMIT 1", kind, owner).Scan(&id)
	return id, err
}
func CreateSpace(ctx context.Context, tx *sql.Tx, kind string, owner uint64) (uint64, error) {
	result, err := tx.ExecContext(ctx, "INSERT INTO space(kind,"+spaceOwnerField(kind)+",lifecycle) VALUES(?,?,'ACTIVE')", kind, owner)
	if err != nil {
		return 0, err
	}
	id, _ := result.LastInsertId()
	return uint64(id), nil
}
