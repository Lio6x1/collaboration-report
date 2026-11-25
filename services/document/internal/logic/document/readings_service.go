// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	codec "collaboration/contracts/content"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"collaboration/platform"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"time"
)

// manifest_read.go：保留原实现，按业务合并文件。

func (s *Readings) Manifest(ctx context.Context, actor Actor, document, manifest uint64) ([]*pb.SourceEpoch, ReadBody, error) {
	access, err := s.access.Authorize(ctx, actor, document, "READ_MANIFEST")
	if err != nil {
		return nil, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx, admission string) (any, error) {
		current, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, err
		}
		structure, style, err := repository.ManifestRevision(ctx, tx, document, manifest)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.Violation(domain.EntityNotFound, "MANIFEST_NOT_FOUND")
		}
		if err != nil {
			return nil, err
		}
		// 全文审核/批注可读已成为MAIN的共享历史；局部EDIT必须证明每个
		// 原单元的完整范围，不能凭一个当前子节读出拆分前整章或其它章节。
		if !access.Permissions.Manage && !access.Permissions.Review && !access.Permissions.Comment {
			units, err := repository.ManifestUnits(ctx, tx, domain.TextID(manifest), document)
			if err != nil {
				return nil, err
			}
			if len(units) == 0 {
				return nil, domain.Violation(domain.PermissionDenied, "MANIFEST_SCOPE_NOT_ALLOWED")
			}
			for _, unit := range units {
				id, err := domain.CanonicalID(unit["unit_id"])
				if err != nil {
					return nil, err
				}
				allowed, err := historicalContentRangeCovered(ctx, tx, access, document, id, s.validator)
				if err != nil {
					return nil, err
				}
				if !allowed {
					return nil, domain.Violation(domain.PermissionDenied, "MANIFEST_SCOPE_NOT_ALLOWED")
				}
			}
		}
		// 仅固定原清单，不Capture当前Yjs、不推进业务指针。Document元数据
		// 仍是当前事实，MainView独立指向原结构/样式，并绑定最终输出准入。
		return readingFixedManifest(ctx, tx, current, document, domain.TextID(manifest), structure, style, admission)
	}
	return access.Sources, run, nil
}

// read.go：保留原实现，按业务合并文件。

type Readings struct {
	access    *Access
	mainViews *MainReadViews
	validator *codec.Validator
}

func NewReadings(access *Access, mainViews *MainReadViews, validator *codec.Validator) *Readings {
	return &Readings{access: access, mainViews: mainViews, validator: validator}
}

// DocumentReading保存业务动作摘要；HTTP头的名称和编码由传输适配器处理。
type DocumentReading struct {
	Document dto.Document
	Actions  []string
}

func (s *Readings) Document(ctx context.Context, actor Actor, id uint64) ([]*pb.SourceEpoch, ReadBody, error) {
	access, err := s.access.Authorize(ctx, actor, id, "READ_DOCUMENT")
	if err != nil {
		return nil, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		document, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, err
		}
		actions := []string{"READ_MAIN"}
		// 前端只展示真实全文批注资格；单元 EDIT 不能隐含 COMMENT。
		if access.Permissions.Comment {
			actions = append(actions, "COMMENT")
		}
		if access.Permissions.Manage {
			actions = append(actions, "RENAME", "DELETE", "CHANGE_VISIBILITY", "CHANGE_AUTHORIZATION")
			spaceID, _ := domain.CanonicalID(document.SpaceId)
			space, err := repository.SpaceRow(ctx, tx, spaceID, false)
			if err != nil {
				return nil, err
			}
			if space.Kind == "ORGANIZATION" {
				actions = append(actions, "TRANSFER_RESPONSIBLE")
			} else {
				actions = append(actions, "MANAGE_INVITATIONS")
			}
		}
		if len(access.Permissions.Edit) > 0 {
			actions = append(actions, "VIEW_EDITABLE_UNITS")
		}
		return DocumentReading{Document: document, Actions: actions}, nil
	}
	return access.Sources, run, nil
}
func (s *Readings) Main(ctx context.Context, actor Actor, id uint64) ([]*pb.SourceEpoch, ReadBody, error) {
	access, err := s.access.Authorize(ctx, actor, id, "READ_MAIN")
	if err != nil {
		return nil, nil, err
	}
	var captured *mainReadCapture
	if !access.Document.MainLocked {
		captured, err = s.mainViews.Capture(ctx, actor, access, id)
		if err != nil {
			return nil, nil, err
		}
		access = captured.Fixed.Access
	}
	run := func(ctx context.Context, tx *sql.Tx, admission string) (any, error) {
		if captured != nil {
			return s.mainViews.Publish(ctx, tx, actor, captured, admission)
		}
		document, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, err
		}
		if !document.MainLocked {
			return nil, domain.Violation(domain.StateConflict, "MAIN_SOURCE_CHANGED")
		}
		return readingMainView(ctx, tx, document, id, document.CurrentManifestId, admission)
	}
	return access.Sources, run, nil
}

// Revision复用全文阅读资格及最终输出登记；变化摘要不成为浏览器的授权凭据。
// Node调用在Doc事务外完成，避免跨服务等待占用来源/文档锁。
func (s *Readings) Revision(ctx context.Context, actor Actor, id uint64) ([]*pb.SourceEpoch, ReadBody, error) {
	access, err := s.access.Authorize(ctx, actor, id, "READ_MAIN")
	if err != nil {
		return nil, nil, err
	}
	tail := ""
	if !access.Document.MainLocked {
		tail, err = s.mainViews.client.MainRevision(ctx, id)
		if err != nil {
			return nil, nil, err
		}
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		current, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, err
		}
		original := access.Document
		// 跨服务采样期间主线代次发生变化就重试，不能把旧尾部套到新主线。
		if current.MainLocked != original.MainLocked || current.ControlRevision != original.ControlRevision || current.CurrentManifestId != original.CurrentManifestId || current.CurrentStyleId != original.CurrentStyleId {
			return nil, domain.Violation(domain.StateConflict, "MAIN_SOURCE_CHANGED")
		}
		// 身份证明中的来源是集合，返回顺序不代表权限发生变化。
		// 仅为摘要排序副本，保留原准入及输出所绑定的来源。
		sources := append([]*pb.SourceEpoch(nil), access.Sources...)
		sort.Slice(sources, func(i, j int) bool { return sources[i].ScopeKey < sources[j].ScopeKey })
		raw, err := json.Marshal(struct {
			Document dto.Document
			Tail     string
			Sources  []*pb.SourceEpoch
		}{current, tail, sources})
		if err != nil {
			return nil, err
		}
		digest := sha256.Sum256(raw)
		return dto.MainReadingRevision{Revision: hex.EncodeToString(digest[:])}, nil
	}
	return access.Sources, run, nil
}

func validateReadingUnits(structure dto.StructureRevision, units []map[string]string) error {
	expected := make(map[string]bool)
	for _, entry := range structure.Entries {
		if entry.UnitId != nil {
			expected[*entry.UnitId] = true
		}
	}
	if len(expected) != len(units) || (len(expected) == 0 && len(structure.Entries) != 0) {
		return errors.New("MANIFEST_INCOMPLETE")
	}
	for _, unit := range units {
		if !expected[unit["unit_id"]] {
			return errors.New("MANIFEST_INCOMPLETE")
		}
		delete(expected, unit["unit_id"])
	}
	return nil
}

func readingMainView(ctx context.Context, tx *sql.Tx, document dto.Document, id uint64, manifest, admission string) (dto.MainView, error) {
	return readingFixedManifest(ctx, tx, document, id, manifest, document.CurrentStructureId, document.CurrentStyleId, admission)
}

func readingFixedManifest(ctx context.Context, tx *sql.Tx, document dto.Document, id uint64, manifest, structureID, styleID, admission string) (dto.MainView, error) {
	var value dto.MainView
	structure, err := repository.StructureRow(ctx, tx, structureID, document.Id)
	if err != nil {
		return value, err
	}
	style, err := repository.StyleRow(ctx, tx, styleID, document.Id)
	if err != nil {
		return value, err
	}
	units, err := repository.ManifestUnits(ctx, tx, manifest, id)
	if err != nil {
		return value, err
	}
	if err = validateReadingUnits(structure, units); err != nil {
		return value, err
	}
	// 固定read view绑定最终输出的真实准入，当前业务指针与阅读清单分别表达各自事实。
	view := domain.ReadView{ID: platform.NewTraceID(), Document: id, Manifest: manifest, Admission: admission, Expires: time.Now().UTC().Add(30 * time.Second)}
	original, err := repository.LockedAdmission(ctx, tx, admission)
	if err != nil {
		return value, err
	}
	if original.State != "OPEN" || !original.Expires.After(time.Now()) {
		return value, domain.Violation(domain.PermissionDenied, "READ_VIEW_NOT_CURRENT")
	}
	// 对外声明的期限不能晚于原证明的实际期限；最长仍是既定30秒。
	if original.Expires.Before(view.Expires) {
		view.Expires = original.Expires
	}
	if err = repository.InsertReadView(ctx, tx, view); err != nil {
		return value, err
	}
	value = dto.MainView{ReadViewId: view.ID, ExpiresAt: view.Expires.Format(time.RFC3339Nano), Document: document, ManifestId: manifest, StructureId: structureID, StyleId: styleID, Structure: structure, Style: style}
	value.Units = make([]struct {
		UnitId           string `json:"unit_id"`
		ContentVersionId string `json:"content_version_id"`
	}, 0, len(units))
	for _, unit := range units {
		value.Units = append(value.Units, struct {
			UnitId           string `json:"unit_id"`
			ContentVersionId string `json:"content_version_id"`
		}{UnitId: unit["unit_id"], ContentVersionId: unit["content_version_id"]})
	}
	return value, nil
}
func (s *Readings) Unit(ctx context.Context, actor Actor, id, unit uint64, view string) ([]*pb.SourceEpoch, ReadBody, error) {
	access, err := s.access.Authorize(ctx, actor, id, "READ_FIXED_UNIT")
	if err != nil {
		return nil, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		if _, err := ValidateAccess(ctx, tx, access); err != nil {
			return nil, err
		}
		fixed, err := repository.LockedReadView(ctx, tx, view, id)
		if err != nil || fixed.Actor != actor.ID || !bytes.Equal(fixed.Session, actor.Proof.SessionRef) || fixed.State != "OPEN" || fixed.AdmissionState != "OPEN" || !fixed.Expires.After(time.Now()) {
			return nil, domain.Violation(domain.PermissionDenied, "READ_VIEW_NOT_CURRENT")
		}
		version, err := repository.ReadViewVersion(ctx, tx, fixed.Manifest, id, unit)
		if err != nil {
			return nil, domain.Violation(domain.EntityNotFound, "UNIT_NOT_IN_READ_VIEW")
		}
		return repository.ContentRow(ctx, tx, version, domain.TextID(id))
	}
	return access.Sources, run, nil
}

// read_transaction.go：保留原实现，按业务合并文件。

// 目录和正文都在来源锁下读取并登记输出，不能在提交之前释放敏感结果。
func (s *Transactions) Read(ctx context.Context, actor Actor, sources []*pb.SourceEpoch, document uint64, purpose, handler string, run ReadBody, bind BindOutput) (any, error) {
	if err := s.identity.Revalidate(ctx, actor.Proof); err != nil {
		return nil, err
	}
	edge, err := s.outputs.EdgeInstance(ctx, handler)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := LockSources(ctx, tx, sources); err != nil {
		return nil, err
	}
	admission := ""
	if document != 0 {
		admission = platform.NewTraceID()
		if err := InsertAdmission(ctx, tx, actor, sources, document, purpose, admission); err != nil {
			return nil, err
		}
	}
	value, err := run(ctx, tx, admission)
	if err != nil {
		return nil, err
	}
	registration, err := s.outputs.Register(ctx, tx, sources, document, admission, handler, edge)
	if err != nil {
		return nil, err
	}
	output := bind(registration, sources)
	if err := tx.Commit(); err != nil {
		output.Abort()
		return nil, err
	}
	return value, nil
}
func InsertAdmission(ctx context.Context, tx *sql.Tx, actor Actor, sources []*pb.SourceEpoch, document uint64, purpose, admission string) error {
	target, _ := json.Marshal(actor.Proof.Target)
	capabilities, _ := json.Marshal([]string{purpose})
	record := domain.ReadAdmission{ID: admission, IdentityAdmissionRef: actor.Proof.AdmissionId, ActorID: actor.ID, SessionRef: actor.Proof.SessionRef, DocumentID: document, Purpose: purpose, CapabilitiesJSON: capabilities, TargetJSON: target, OperationID: actor.Proof.OperationId, ExpiresUnixMs: actor.Proof.ExpiresUnixMs}
	if err := repository.InsertReadAdmission(ctx, tx, record); err != nil {
		return err
	}
	for _, source := range sources {
		if err := repository.InsertAdmissionSource(ctx, tx, admission, source.ScopeKey, source.Epoch); err != nil {
			return err
		}
	}
	return nil
}
