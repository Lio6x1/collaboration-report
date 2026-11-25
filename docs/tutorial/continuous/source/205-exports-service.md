# services/document/internal/logic/document/exports_service.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/205-exports-service.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：编排用例、权限、事务及未知结果恢复；跨服务调用不能替代持久事实。

主要声明：`ExportRenderer`、`Exports`、`NewExports`、`WorkerCount`、`exportAdmission`、`prepare`、`mainExportView`、`register`、`exportActorID`、`exportStoredInput`、`Create`、`DrainDocument`、`expireArtifacts`、`exportDocument`、`lookup`、`Job`、`Download`、`exportGuardKey`、`exportDocumentGuard`、`Recover`、`recoverJob`、`convertJob`、`exportSourceClosed`、`workerAuthority`、`finishExport`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	codec "collaboration/contracts/content"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	resources "collaboration/document/internal/repository/resource"
	"collaboration/document/internal/storage/local"
	"collaboration/platform"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"time"
)

// export.go：保留原实现，按业务合并文件。

type ExportRenderer interface {
	Render(context.Context, domain.ExportRecord, domain.ExportDocument) (string, uint64, [32]byte, error)
	Cleanup(context.Context, domain.ExportRecord) error
	Files() ResourceFiles
}

// Exports编排权威准入与持久作业；格式转换/文件存储通过构造注入，HTTP与SQL分别留在适配层。
type Exports struct {
	db             *sql.DB
	identity       *Identity
	access         *Access
	public         *PublicReading
	outputs        *Outputs
	resources      *Resources
	renderer       ExportRenderer
	workers        int
	temporaryLimit uint64
}

func NewExports(db *sql.DB, identity *Identity, access *Access, public *PublicReading, outputs *Outputs, resources *Resources, renderer ExportRenderer, workers int) (*Exports, error) {
	if workers == 0 {
		workers = 1
	}
	if workers < 1 || workers > 8 {
		return nil, errors.New("EXPORT_WORKERS_INVALID")
	}
	// 这是持久预留的磁盘预算，不代表转换器的内存上限。
	// 每worker留256MiB，沿用单任务192MiB预留和32MiB制品限制。
	return &Exports{db: db, identity: identity, access: access, public: public, outputs: outputs, resources: resources, renderer: renderer,
		workers: workers, temporaryLimit: uint64(workers) * repository.ExportTemporaryLimit}, nil
}

func (s *Exports) WorkerCount() int { return s.workers }

type exportAdmission struct {
	sources []*pb.SourceEpoch
	check   func(context.Context, *sql.Tx) (domain.ExportSnapshot, error)
}

func (s *Exports) prepare(ctx context.Context, actor *Actor, document uint64, input domain.ExportInput, creating bool) (exportAdmission, error) {
	var admitted exportAdmission
	if input.Target == "MAIN" {
		if actor == nil {
			return admitted, domain.Violation(domain.SessionRequired, "SESSION_REQUIRED")
		}
		access, err := s.access.Authorize(ctx, *actor, document, "EXPORT_MAIN")
		if err != nil {
			return admitted, err
		}
		admitted.sources = access.Sources
		admitted.check = func(ctx context.Context, tx *sql.Tx) (domain.ExportSnapshot, error) {
			var fixed domain.ExportSnapshot
			current, err := ValidateAccess(ctx, tx, access)
			if err != nil {
				return fixed, err
			}
			manifest, err := domain.CanonicalID(input.Manifest)
			if err != nil {
				return fixed, err
			}
			if creating {
				allowed := current.MainLocked && current.CurrentManifestId == input.Manifest
				if !allowed {
					allowed, err = s.mainExportView(ctx, tx, document, manifest, *actor)
				}
				if err != nil {
					return fixed, err
				}
				if !allowed {
					return fixed, domain.Violation(domain.StateConflict, "EXPORT_MAIN_VIEW_EXPIRED")
				}
			}
			fixed, err = repository.ExportManifestFacts(ctx, tx, document, manifest)
			if err != nil {
				return fixed, err
			}
			fixed.Title = current.Title
			fixed.Space, _ = domain.CanonicalID(current.SpaceId)
			fixed.Target = "MAIN"
			fixed.Format = input.Format
			fixed.Captured = time.Now().UTC()
			fixed.Sources = admitted.sources
			return fixed, nil
		}
		return admitted, nil
	}
	facts, err := s.public.facts(ctx, document)
	if err != nil {
		return admitted, err
	}
	policy, err := domain.PublicPolicy(facts)
	if err != nil {
		return admitted, err
	}
	if !domain.HasCurrentPublication(facts, policy) || facts.Document.CurrentReleaseId == nil || *facts.Document.CurrentReleaseId != input.Release {
		return admitted, domain.Violation(domain.EntityNotFound, "EXPORT_PUBLIC_NOT_CURRENT")
	}
	_, admitted.sources, err = s.public.audience(ctx, actor, facts, policy)
	if err != nil {
		return admitted, err
	}
	admitted.check = func(ctx context.Context, tx *sql.Tx) (domain.ExportSnapshot, error) {
		var fixed domain.ExportSnapshot
		current, err := repository.PublicationFactsRow(ctx, tx, document, true)
		if err != nil {
			return fixed, err
		}
		if publicFactsBinding(current) != publicFactsBinding(facts) || !domain.HasCurrentPublication(current, policy) {
			return fixed, domain.Violation(domain.StateConflict, "EXPORT_PUBLIC_CHANGED")
		}
		release, _ := domain.CanonicalID(input.Release)
		value, err := repository.FixedReleaseRow(ctx, tx, document, release)
		if err != nil {
			return fixed, err
		}
		manifest, _ := domain.CanonicalID(value.Release.ManifestId)
		fixed, err = repository.ExportManifestFacts(ctx, tx, document, manifest)
		if err != nil {
			return fixed, err
		}
		fixed.Space, _ = domain.CanonicalID(current.Space.Id)
		fixed.Publication, _ = domain.CanonicalID(current.Document.PublicationEpoch)
		fixed.Release = release
		fixed.Title = value.Title
		fixed.Target = "PUBLIC"
		fixed.Format = input.Format
		fixed.Captured = time.Now().UTC()
		fixed.Sources = admitted.sources
		return fixed, nil
	}
	return admitted, nil
}

func (s *Exports) mainExportView(ctx context.Context, tx *sql.Tx, doc, manifest uint64, actor Actor) (bool, error) {
	return repository.ExportMainView(ctx, tx, doc, manifest, actor.ID, actor.Proof.SessionRef)
}

func (s *Exports) register(ctx context.Context, tx *sql.Tx, actor *Actor, fixed domain.ExportSnapshot, handler string, edge *domain.OutputInstance) (*ResponseRegistration, error) {
	if fixed.Target == "PUBLIC" {
		return s.outputs.RegisterPublication(ctx, tx, fixed.Sources, fixed.Document, fixed.Publication, fixed.Release, handler, edge)
	}
	admission := platform.NewTraceID()
	if err := InsertAdmission(ctx, tx, *actor, fixed.Sources, fixed.Document, "EXPORT_MAIN", admission); err != nil {
		return nil, err
	}
	return s.outputs.Register(ctx, tx, fixed.Sources, fixed.Document, admission, handler, edge)
}

func exportActorID(actor *Actor) uint64 {
	if actor == nil {
		return 0
	}
	return actor.ID
}

func exportStoredInput(value domain.ExportRecord) domain.ExportInput {
	input := domain.ExportInput{Operation: value.Operation, Target: value.Snapshot.Target, Format: value.Job.Format}
	if input.Target == "MAIN" {
		input.Manifest = value.Job.ManifestId
	} else if value.Job.ReleaseId != nil {
		input.Release = *value.Job.ReleaseId
	}
	return input
}

// export_create.go：保留原实现，按业务合并文件。

func (s *Exports) Create(ctx context.Context, actor *Actor, id uint64, input domain.ExportInput, raw dto.ExportCreate, path, handler string, bind BindOutput) (dto.ExportJob, error) {
	var empty dto.ExportJob
	if err := input.Validate(); err != nil {
		return empty, err
	}
	observed, lookupErr := repository.ExportOperation(ctx, s.db, input.Operation)
	if lookupErr != nil && !errors.Is(lookupErr, sql.ErrNoRows) {
		return empty, lookupErr
	}
	if lookupErr == nil && (observed.Requester != exportActorID(actor) || observed.Job.DocumentId != domain.TextID(id)) {
		return empty, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
	}
	admitted, err := s.prepare(ctx, actor, id, input, lookupErr != nil)
	if err != nil {
		return empty, err
	}
	if actor != nil {
		if err = s.identity.Revalidate(ctx, actor.Proof); err != nil {
			return empty, err
		}
	}
	edge, err := s.outputs.EdgeInstance(ctx, handler)
	if err != nil || edge == nil {
		return empty, domain.Violation(domain.TemporarilyUnavailable, "EDGE_OUTPUT_UNAVAILABLE")
	}
	fingerprint, err := domain.OperationFingerprint(path, json.RawMessage(raw))
	if err != nil {
		return empty, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return empty, err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, input.Operation); err != nil {
		return empty, err
	}
	if err = LockSources(ctx, tx, admitted.sources); err != nil {
		return empty, err
	}
	// 原job先复核公开/私有资格，但不再要求已经到期的原阅读视图；只重放已登记的固定版本。
	prior, priorErr := repository.ExportOperation(ctx, tx, input.Operation)
	if priorErr == nil {
		if prior.Requester != exportActorID(actor) || prior.Job.DocumentId != domain.TextID(id) || !bytes.Equal(prior.Fingerprint[:], fingerprint[:]) {
			return empty, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
		}
	} else if !errors.Is(priorErr, sql.ErrNoRows) {
		return empty, priorErr
	}
	fixed, err := admitted.check(ctx, tx)
	if err != nil {
		return empty, err
	}
	var job dto.ExportJob
	if priorErr == nil {
		job = prior.Job
	} else {
		// 不与其他业务种类共用同号，匿名导出也必须参与全局原号gate。
		conflict, err := repository.OperationExists(ctx, tx, input.Operation)
		if err != nil {
			return empty, err
		}
		if conflict {
			return empty, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
		}
		if err = repository.CheckExportQueue(ctx, tx); err != nil {
			return empty, err
		}
		jobID, err := repository.InsertExport(ctx, tx, exportActorID(actor), input.Operation, fingerprint, fixed)
		if err != nil {
			return empty, err
		}
		if err = repository.PinExportResources(ctx, tx, jobID, fixed); err != nil {
			return empty, err
		}
		var identityRef string
		if actor != nil {
			identityRef = actor.Proof.AdmissionId
		}
		if err = repository.InsertExportReceipt(ctx, tx, exportActorID(actor), identityRef, input.Operation, path, fingerprint, jobID); err != nil {
			return empty, err
		}
		job = dto.ExportJob{Id: domain.TextID(jobID), DocumentId: domain.TextID(id), ManifestId: domain.TextID(fixed.Manifest), Format: input.Format, State: "QUEUED"}
		if fixed.Release != 0 {
			release := domain.TextID(fixed.Release)
			job.ReleaseId = &release
		}
	}
	registration, err := s.register(ctx, tx, actor, fixed, handler, edge)
	if err != nil {
		return empty, err
	}
	output := bind(registration, fixed.Sources)
	if err = tx.Commit(); err != nil {
		output.Abort()
		return empty, errors.New("EXPORT_REGISTRATION_COMMIT_UNKNOWN")
	}
	return job, nil
}

// export_drain.go：保留原实现，按业务合并文件。

// 来源真正关闭后，生命周期恢复器可以排空本篇已终态的导出副本。
// 只等待本篇实际转换；其他文档不断导出也不能饿死本篇治理排空。
// 旧全局键仍取共享门以排除旧版本串行worker，删除文件先于归还预算。
func (s *Exports) DrainDocument(ctx context.Context, document uint64) error {
	files := s.renderer.Files()
	release, err := files.AcquireRead(ctx, exportGuardKey("collaboration-report-export-worker-v1"))
	if err != nil {
		return err
	}
	defer release()
	unlock, err := files.Acquire(ctx, exportDocumentGuard(document))
	if err != nil {
		return err
	}
	defer unlock()
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	err = repository.ExpireClosedDocumentExports(ctx, tx, document)
	if err != nil {
		tx.Rollback()
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	if err = s.expireArtifacts(ctx); err != nil {
		return err
	}
	pending, err := repository.DocumentExportControlsPending(ctx, s.db, document)
	if err != nil {
		return err
	}
	if pending {
		return domain.Violation(domain.StateConflict, "EXPORT_DOCUMENT_DRAIN_PENDING")
	}
	return nil
}

// export_expiry.go：保留原实现，按业务合并文件。

func (s *Exports) expireArtifacts(ctx context.Context) error {
	// 多worker可并行转换，但回收必须串行，否则可能重复归还同一制品预算。
	unlock, err := s.renderer.Files().Acquire(ctx, exportGuardKey("collaboration-report-export-maintenance-v1"))
	if errors.Is(err, local.ErrBusy) {
		return nil
	}
	if err != nil {
		return err
	}
	defer unlock()
	items, err := repository.ExpiredExports(ctx, s.db)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.Key != "" {
			files := s.renderer.Files()
			unlock, err := files.Acquire(ctx, item.Key)
			if err != nil {
				continue
			} // 正在实际发送的共享guard不能被临时TTL越过。
			err = files.Delete(ctx, item.Key)
			unlock()
			if err != nil {
				return err
			}
		}
		tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
		if err != nil {
			return err
		}
		err = repository.RemoveExpiredExport(ctx, tx, item)
		if err == nil {
			err = tx.Commit()
		} else {
			tx.Rollback()
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// export_fixed.go：保留原实现，按业务合并文件。

func (s *Exports) exportDocument(ctx context.Context, value domain.ExportRecord) (domain.ExportDocument, error) {
	fixed := value.Snapshot
	document := domain.ExportDocument{Title: fixed.Title, Target: fixed.Target, Format: fixed.Format, Manifest: domain.TextID(fixed.Manifest), Captured: fixed.Captured}
	if fixed.Release != 0 {
		document.Release = domain.TextID(fixed.Release)
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return document, err
	}
	defer tx.Rollback()
	if err = LockSources(ctx, tx, fixed.Sources); err != nil {
		return document, err
	}
	if err = s.workerAuthority(ctx, tx, value); err != nil {
		return document, err
	}
	document.Structure, err = repository.StructureRow(ctx, tx, domain.TextID(fixed.Structure), domain.TextID(fixed.Document))
	if err != nil {
		return document, err
	}
	document.Style, err = repository.StyleRow(ctx, tx, domain.TextID(fixed.Style), domain.TextID(fixed.Document))
	if err != nil {
		return document, err
	}
	units, err := repository.ManifestUnits(ctx, tx, domain.TextID(fixed.Manifest), fixed.Document)
	if err != nil {
		return document, err
	}
	ids := map[uint64]bool{}
	var bytes uint64
	for _, unit := range units {
		version, _ := domain.CanonicalID(unit["content_version_id"])
		unitID, _ := domain.CanonicalID(unit["unit_id"])
		content, err := repository.VersionContent(ctx, tx, version, fixed.Document)
		if err != nil {
			return document, err
		}
		if err = canonicalVersion(content, version); err != nil {
			return document, err
		}
		if err = validateFixedMergeContent(s.public.validator, content, fixed.Document, unitID, fixed.Structure); err != nil {
			return document, domain.Violation(domain.StateConflict, "EXPORT_FIXED_CONTENT_INVALID")
		}
		if content.ByteSize > 24<<20 || bytes > (24<<20)-content.ByteSize {
			return document, domain.Violation(domain.StateConflict, "EXPORT_CONTENT_LIMIT")
		}
		bytes += content.ByteSize
		body, err := repository.ContentRow(ctx, tx, unit["content_version_id"], domain.TextID(fixed.Document))
		if err != nil {
			return document, err
		}
		// 防止JSON列和参与hash的canonical列漂移；不能把副本损坏静默导出。
		raw, err := json.Marshal(body.BodyJson)
		if err != nil {
			return document, err
		}
		decoded, err := codec.Decode(raw)
		if err != nil {
			return document, err
		}
		canonical, err := codec.Canonical(decoded)
		if err != nil {
			return document, err
		}
		digest := sha256.Sum256(canonical)
		if hex.EncodeToString(digest[:]) != body.BodyHash {
			return document, domain.Violation(domain.StateConflict, "EXPORT_FIXED_CONTENT_INVALID")
		}
		tree, ok := decoded.(map[string]any)
		if !ok {
			return document, domain.Violation(domain.StateConflict, "EXPORT_FIXED_CONTENT_INVALID")
		}
		document.Units = append(document.Units, domain.ExportUnit{ID: body.UnitId, Body: tree})
		for _, id := range content.ResourceIds {
			ids[id] = true
		}
	}
	ordered := make([]uint64, 0, len(ids))
	for id := range ids {
		ordered = append(ordered, id)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	var assetBytes uint64
	for _, id := range ordered {
		object, err := resources.ObjectRow(ctx, tx, id, true)
		if err != nil {
			return document, err
		}
		allowed, err := resources.ResourceAllowedForDocument(ctx, tx, id, fixed.Document, fixed.Space)
		if err != nil {
			return document, err
		}
		if !allowed || object.State != "READY" || object.Size > 32<<20 || assetBytes > (32<<20)-object.Size {
			return document, domain.Violation(domain.StateConflict, "EXPORT_ASSET_LIMIT")
		}
		assetBytes += object.Size
		document.Assets = append(document.Assets, domain.ExportAsset{ID: domain.TextID(id), Key: object.Key, MediaType: object.MediaType, Size: object.Size, Checksum: object.Checksum})
	}
	if err = tx.Commit(); err != nil {
		return document, err
	}
	// 资源EXPORT pin已与原job提交；OS共享guard与完整checksum保护这里的数据库外读取。
	for index := range document.Assets {
		asset := &document.Assets[index]
		unlock, err := s.resources.files.AcquireRead(ctx, asset.Key)
		if err != nil {
			return document, err
		}
		file, err := s.resources.files.Open(ctx, asset.Key, asset.Size, asset.Checksum)
		if err != nil {
			unlock()
			return document, err
		}
		asset.Data, err = io.ReadAll(io.LimitReader(file, int64(asset.Size)+1))
		file.Close()
		unlock()
		if err != nil {
			return document, err
		}
		if uint64(len(asset.Data)) != asset.Size {
			return document, domain.Violation(domain.StateConflict, "EXPORT_RESOURCE_INVALID")
		}
	}
	return document, nil
}

// export_query.go：保留原实现，按业务合并文件。

func (s *Exports) lookup(ctx context.Context, actor *Actor, id uint64) (domain.ExportRecord, exportAdmission, error) {
	value, err := repository.ExportRow(ctx, s.db, id, false)
	if errors.Is(err, sql.ErrNoRows) {
		err = domain.Violation(domain.EntityNotFound, "EXPORT_NOT_FOUND")
	}
	if err != nil {
		return value, exportAdmission{}, err
	}
	if value.Requester != 0 && value.Requester != exportActorID(actor) {
		return value, exportAdmission{}, domain.Violation(domain.EntityNotFound, "EXPORT_NOT_FOUND")
	}
	if !value.Expires.After(time.Now().UTC()) {
		return value, exportAdmission{}, domain.Violation(domain.EntityNotFound, "EXPORT_EXPIRED")
	}
	admitted, err := s.prepare(ctx, actor, value.Snapshot.Document, exportStoredInput(value), false)
	return value, admitted, err
}

func (s *Exports) Job(ctx context.Context, actor *Actor, id uint64, handler string, bind BindOutput) (dto.ExportJob, error) {
	var empty dto.ExportJob
	value, admitted, err := s.lookup(ctx, actor, id)
	if err != nil {
		return empty, err
	}
	if actor != nil {
		if err = s.identity.Revalidate(ctx, actor.Proof); err != nil {
			return empty, err
		}
	}
	edge, err := s.outputs.EdgeInstance(ctx, handler)
	if err != nil || edge == nil {
		return empty, domain.Violation(domain.TemporarilyUnavailable, "EDGE_OUTPUT_UNAVAILABLE")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return empty, err
	}
	defer tx.Rollback()
	if err = LockSources(ctx, tx, admitted.sources); err != nil {
		return empty, err
	}
	fixed, err := admitted.check(ctx, tx)
	if err != nil {
		return empty, err
	}
	current, err := repository.ExportRow(ctx, tx, id, true)
	if err != nil {
		return empty, err
	}
	if current.Operation != value.Operation || current.Requester != value.Requester || !current.Expires.After(time.Now().UTC()) {
		return empty, domain.Violation(domain.StateConflict, "EXPORT_JOB_CHANGED")
	}
	registration, err := s.register(ctx, tx, actor, fixed, handler, edge)
	if err != nil {
		return empty, err
	}
	output := bind(registration, fixed.Sources)
	if err = tx.Commit(); err != nil {
		output.Abort()
		return empty, err
	}
	return current.Job, nil
}

func (s *Exports) Download(ctx context.Context, actor *Actor, id uint64, handler string, bind BindOutput) (*ResourceDownload, string, error) {
	value, admitted, err := s.lookup(ctx, actor, id)
	if err != nil {
		return nil, "", err
	}
	if value.Job.State != "SUCCEEDED" || value.ArtifactKey == "" || value.Size == 0 || value.Size > 32<<20 {
		return nil, "", domain.Violation(domain.StateConflict, "EXPORT_NOT_READY")
	}
	edge, err := s.outputs.EdgeInstance(ctx, handler)
	if err != nil || edge == nil {
		return nil, "", domain.Violation(domain.TemporarilyUnavailable, "EDGE_OUTPUT_UNAVAILABLE")
	}
	files := s.renderer.Files()
	// 与资源读取相同：共享文件guard覆盖完整校验和实际发送；数据库外IO后再次准入。
	release, err := files.AcquireRead(ctx, value.ArtifactKey)
	if err != nil {
		return nil, "", err
	}
	file, err := files.Open(ctx, value.ArtifactKey, value.Size, value.Checksum)
	if err != nil {
		release()
		return nil, "", err
	}
	download := &ResourceDownload{ID: id, Size: value.Size, Reader: file, release: release}
	accepted := false
	defer func() {
		if !accepted {
			download.Close()
		}
	}()
	if actor != nil {
		if err = s.identity.Revalidate(ctx, actor.Proof); err != nil {
			return nil, "", err
		}
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, "", err
	}
	defer tx.Rollback()
	if err = LockSources(ctx, tx, admitted.sources); err != nil {
		return nil, "", err
	}
	fixed, err := admitted.check(ctx, tx)
	if err != nil {
		return nil, "", err
	}
	current, err := repository.ExportRow(ctx, tx, id, true)
	if err != nil {
		return nil, "", err
	}
	if current.Job.State != "SUCCEEDED" || current.Generation != value.Generation || current.ArtifactKey != value.ArtifactKey || current.Checksum != value.Checksum || current.Size != value.Size || !current.Expires.After(time.Now().UTC()) {
		return nil, "", domain.Violation(domain.StateConflict, "EXPORT_JOB_CHANGED")
	}
	registration, err := s.register(ctx, tx, actor, fixed, handler, edge)
	if err != nil {
		return nil, "", err
	}
	output := bind(registration, fixed.Sources)
	if err = tx.Commit(); err != nil {
		output.Abort()
		return nil, "", err
	}
	extension := "zip"
	download.MediaType = "application/zip"
	if value.Job.Format == "DOCX" {
		extension = "docx"
		download.MediaType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	}
	if value.Job.Format == "PDF" {
		extension = "pdf"
		download.MediaType = "application/pdf"
	}
	accepted = true
	return download, "document-" + value.Job.DocumentId + "-export-" + value.Job.Id + "." + extension, nil
}

// export_worker.go：保留原实现，按业务合并文件。

func exportGuardKey(value string) string {
	key := sha256.Sum256([]byte(value))
	return hex.EncodeToString(key[:])
}

func exportDocumentGuard(document uint64) string {
	return exportGuardKey("collaboration-report-export-document-v1\n" + domain.TextID(document))
}

// 沿用旧全局键共享门，排除旧串行worker；新转换与排空在对应文档门上互斥。
// 每job另取跨进程排他锁，覆盖实际子进程、临时文件和最终SQL，不靠TTL猜进程退出。
func (s *Exports) Recover(ctx context.Context) error {
	files := s.renderer.Files()
	release, err := files.AcquireRead(ctx, exportGuardKey("collaboration-report-export-worker-v1"))
	if errors.Is(err, local.ErrBusy) {
		return nil
	}
	if err != nil {
		return err
	}
	defer release()
	if err = s.expireArtifacts(ctx); err != nil {
		return err
	}
	ids, err := repository.NextExports(ctx, s.db)
	if err != nil {
		return err
	}
	for _, id := range ids {
		unlock, err := files.Acquire(ctx, exportGuardKey("collaboration-report-export-job-v1\n"+domain.TextID(id)))
		if errors.Is(err, local.ErrBusy) {
			continue // 最早的RUNNING仍在转换；后续任务不能被它挡住。
		}
		if err != nil {
			return err
		}
		handled, err := s.recoverJob(ctx, id)
		unlock()
		if handled || err != nil {
			return err
		}
	}
	return nil
}

// 候选查询和取得锁之间可能已有worker完成任务，持锁重读后只处理非终态。
func (s *Exports) recoverJob(ctx context.Context, id uint64) (bool, error) {
	value, err := repository.ExportRow(ctx, s.db, id, false)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if value.Job.State != "QUEUED" && value.Job.State != "RUNNING" {
		return false, nil
	}
	document, err := domain.CanonicalID(value.Job.DocumentId)
	if err != nil {
		return false, err
	}
	unlock, err := s.renderer.Files().AcquireRead(ctx, exportDocumentGuard(document))
	if errors.Is(err, local.ErrBusy) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer unlock()
	return true, s.convertJob(ctx, id, value)
}

func (s *Exports) convertJob(ctx context.Context, id uint64, value domain.ExportRecord) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	err = LockSources(ctx, tx, value.Snapshot.Sources)
	if err != nil {
		tx.Rollback()
		if exportSourceClosed(err) {
			return s.finishExport(ctx, value, "CANCELLED", "EXPORT_SOURCE_CLOSED", "", 0, [32]byte{})
		}
		return err
	}
	if err = s.workerAuthority(ctx, tx, value); err != nil {
		tx.Rollback()
		if exportSourceClosed(err) {
			return s.finishExport(ctx, value, "CANCELLED", "EXPORT_SOURCE_CLOSED", "", 0, [32]byte{})
		}
		return err
	}
	claimed, err := repository.ClaimExport(ctx, tx, id, s.temporaryLimit)
	if err != nil {
		tx.Rollback()
		_ = repository.DeferExport(ctx, s.db, id)
		return err
	}
	if err = tx.Commit(); err != nil {
		return errors.New("EXPORT_CLAIM_COMMIT_UNKNOWN")
	}
	// 提交后只持文件guard，不跨格式转换持数据库锁。
	document, err := s.exportDocument(ctx, claimed)
	var artifact string
	var size uint64
	var checksum [32]byte
	if err == nil {
		artifact, size, checksum, err = s.renderer.Render(ctx, claimed, document)
	}
	if err != nil {
		if ctx.Err() != nil {
			return err
		} // 进程/父任务退出仍保留RUNNING，让原job恢复。
		if exportSourceClosed(err) {
			return s.finishExport(ctx, claimed, "CANCELLED", "EXPORT_SOURCE_CLOSED", "", 0, [32]byte{})
		}
		var rule domain.RuleViolation
		if errors.As(err, &rule) {
			return s.finishExport(ctx, claimed, "REJECTED", rule.Code, "", 0, [32]byte{})
		}
		_ = repository.DeferExport(ctx, s.db, id)
		return err
	}
	// 最后重查原来源与当前正式指针。转换时撤权先等待实际进程结束，再删制品并取消。
	check, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	err = LockSources(ctx, check, claimed.Snapshot.Sources)
	if err == nil {
		err = s.workerAuthority(ctx, check, claimed)
	}
	if err == nil {
		err = repository.FinishExport(ctx, check, id, claimed.Generation, "SUCCEEDED", "", artifact, size, checksum)
	}
	if err == nil {
		err = check.Commit()
	} else {
		check.Rollback()
	}
	if err == nil {
		return nil
	}
	if exportSourceClosed(err) {
		files := s.renderer.Files()
		unlock, lockErr := files.Acquire(ctx, artifact)
		if lockErr != nil {
			return lockErr
		}
		removeErr := files.Delete(ctx, artifact)
		unlock()
		if removeErr != nil {
			return removeErr
		}
		return s.finishExport(ctx, claimed, "CANCELLED", "EXPORT_SOURCE_CLOSED", "", 0, [32]byte{})
	}
	// SQL提交未知不能删除可能已SUCCEEDED的原制品或释放预留，下一轮读取原状态。
	return err
}

func exportSourceClosed(err error) bool {
	var rule domain.RuleViolation
	return errors.As(err, &rule) && (rule.Code == "SOURCE_CLOSED" || rule.Code == "EXPORT_PUBLIC_NOT_CURRENT" || rule.Code == "EXPORT_DOCUMENT_CLOSED")
}

func (s *Exports) workerAuthority(ctx context.Context, tx *sql.Tx, value domain.ExportRecord) error {
	fixed := value.Snapshot
	if !value.Expires.After(time.Now().UTC()) {
		return domain.Violation(domain.PermissionDenied, "EXPORT_DOCUMENT_CLOSED")
	}
	facts, err := repository.PublicationFactsRow(ctx, tx, fixed.Document, true)
	if err != nil {
		return err
	}
	if fixed.Target == "MAIN" {
		if facts.Document.Lifecycle != "ACTIVE" {
			return domain.Violation(domain.PermissionDenied, "EXPORT_DOCUMENT_CLOSED")
		}
		return nil
	}
	policy, err := domain.PublicPolicy(facts)
	if err != nil {
		return err
	}
	if !domain.HasCurrentPublication(facts, policy) || facts.Document.CurrentReleaseId == nil || *facts.Document.CurrentReleaseId != domain.TextID(fixed.Release) || facts.Document.PublicationEpoch != domain.TextID(fixed.Publication) {
		return domain.Violation(domain.PermissionDenied, "EXPORT_PUBLIC_NOT_CURRENT")
	}
	return nil
}

func (s *Exports) finishExport(ctx context.Context, value domain.ExportRecord, state, code, key string, size uint64, checksum [32]byte) error {
	if state != "SUCCEEDED" {
		if err := s.renderer.Cleanup(ctx, value); err != nil {
			return err
		}
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	id, _ := domain.CanonicalID(value.Job.Id)
	if err = repository.FinishExport(ctx, tx, id, value.Generation, state, code, key, size, checksum); err != nil {
		return err
	}
	return tx.Commit()
}
````

UTF-8/LF全文SHA-256：`d347f8ea5596b1a7ad1bb8884368822c35f501a374224a1b0852f05b7adb3117`。[参考实现](../../../../services/document/internal/logic/document/exports_service.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
