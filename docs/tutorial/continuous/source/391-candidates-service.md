# services/search/internal/logic/candidates/candidates_service.go

阶段05：[功能讲义](../05-search.md) · [分步编写](../write/391-candidates-service.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：编排用例、权限、事务及未知结果恢复；跨服务调用不能替代持久事实。

主要声明：`Generations`、`Indexes`、`Candidates`、`New`、`SearchCandidates`、`enter`、`leave`、`randomToken`、`operationDeadline`、`transact`、`CloseCursor`、`closeHash`、`Cleanup`、`Ready`、`sameBatch`、`retainOpened`、`confirmQuery`、`fail`、`keyLock`、`keyLocks`、`acquire`、`reserve`、`matchesQuery`、`beginQuery`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package candidates

import (
	"bytes"
	pb "collaboration/contracts/internalv1"
	"collaboration/search/internal/client/elastic"
	domain "collaboration/search/internal/domain/cursor"
	repository "collaboration/search/internal/repository/cursor"
	generation "collaboration/search/internal/repository/generation"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// candidates.go：保留原实现，按业务合并文件。

type Generations interface {
	Ready(context.Context) error
	Do(context.Context, func(context.Context, generation.Target) error) error
}

type Indexes interface {
	OpenPIT(context.Context, string, time.Duration) (string, error)
	Candidates(context.Context, string, string, json.RawMessage, int, time.Duration) (elastic.CandidateBatch, error)
	ClosePIT(context.Context, string) error
}

// Candidates拥有单链资源和跨系统提交编排；SQL不假装与ES原子，索引候选也不等于读取权限。
type Candidates struct {
	database         *sql.DB
	generation       Generations
	indexes          Indexes
	alias            string
	codec            *domain.Codec
	ttl              time.Duration
	admission        chan struct{}
	queries          keyLocks
	closingAdmission chan struct{}
	sweepMu          sync.Mutex
	recovered        atomic.Bool
}

func New(database *sql.DB, generations Generations, indexes Indexes, alias string, key []byte, ttl time.Duration) (*Candidates, error) {
	if database == nil || generations == nil || indexes == nil || alias == "" || ttl < time.Millisecond || ttl > 2*time.Minute {
		return nil, errors.New("SEARCH_CANDIDATE_CONFIG_INVALID")
	}
	codec, err := domain.NewCodec(key)
	if err != nil {
		return nil, err
	}
	return &Candidates{database: database, generation: generations, indexes: indexes, alias: alias, codec: codec, ttl: ttl,
		admission: make(chan struct{}, 2), closingAdmission: make(chan struct{}, 1)}, nil
}

func (c *Candidates) SearchCandidates(ctx context.Context, request *pb.SearchCandidatesRequest) (*pb.SearchCandidatesReply, error) {
	query, err := domain.ValidateQuery(request)
	if err != nil {
		return nil, err
	}
	// HTTP page大小属于Doc；Search固定批100，避免小批More与已冻结批锚点出现两套语义。
	if query.PageSize != 100 {
		return nil, errors.New("SEARCH_CANDIDATE_BATCH_SIZE_INVALID")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := c.enter(ctx); err != nil {
		return nil, err
	}
	defer c.leave()
	var anchor domain.Batch
	var action domain.Action
	var record repository.Record
	newChain := len(request.Cursor) == 0
	if newChain {
		record, anchor.Chain, err = c.reserve(ctx, query)
	} else {
		var signed []byte
		action, signed, err = domain.ParseEnvelope(request.Cursor)
		if err == nil {
			anchor, err = c.codec.Open(signed)
		}
	}
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(anchor.Chain[:])
	release, err := c.queries.acquire(ctx, hash)
	if err != nil {
		return nil, err
	}
	defer release()
	var reply *pb.SearchCandidatesReply
	err = c.generation.Do(ctx, func(ctx context.Context, target generation.Target) error {
		var err error
		record, err = c.beginQuery(ctx, hash, query, target, newChain)
		if err != nil {
			return err
		}
		before := anchor.After
		if newChain || action == domain.ActionReplay {
			before = anchor.Before
		}
		if newChain {
			pit, openErr := c.indexes.OpenPIT(ctx, record.Physical, time.Until(record.Expires))
			if openErr != nil {
				c.fail(ctx, record, pit, true)
				return openErr
			}
			record.PIT = pit
			// Open成功的句柄先落SQL再查候选；中间退出仍能沿原行关闭，不重复分配资源。
			if err := c.retainOpened(ctx, record); err != nil {
				c.fail(ctx, record, pit, false)
				return err
			}
		}
		batch, queryErr := c.indexes.Candidates(ctx, record.PIT, query.Query, before, 100, time.Until(record.Expires))
		if batch.PIT != "" {
			record.PIT = batch.PIT
		}
		if queryErr != nil {
			c.fail(ctx, record, record.PIT, true)
			return queryErr
		}
		for _, candidate := range batch.Candidates {
			if candidate.PhysicalIndex != record.Physical {
				c.fail(ctx, record, record.PIT, true)
				return errors.New("SEARCH_CURSOR_PHYSICAL_MISMATCH")
			}
		}
		digest, err := domain.DigestCandidates(before, batch.End, batch.Candidates, batch.More)
		if err != nil {
			c.fail(ctx, record, record.PIT, true)
			return err
		}
		if !newChain && action == domain.ActionReplay && !sameBatch(anchor, batch, digest) {
			c.fail(ctx, record, record.PIT, false)
			return errors.New("SEARCH_CURSOR_REPLAY_CHANGED")
		}
		signed, err := c.codec.Seal(domain.Batch{Chain: anchor.Chain, Before: before, After: batch.End, Count: uint8(len(batch.Candidates)), More: batch.More, Digest: digest})
		if err != nil {
			c.fail(ctx, record, record.PIT, false)
			return err
		}
		if err := c.confirmQuery(ctx, record, batch.End); err != nil {
			c.fail(ctx, record, record.PIT, true)
			return err
		}
		reply = &pb.SearchCandidatesReply{Candidates: batch.Candidates, Continuation: signed, IndexGeneration: record.Generation,
			PhysicalIndex: record.Physical, ExpiresUnixMs: record.Expires.UnixMilli(), HasMore: batch.More, Complete: true}
		return nil
	})
	if err != nil {
		// 代次拒绝发生在ES调用之前时，同样收尾原预留；没有新查询可继续这条链。
		if newChain {
			c.fail(ctx, record, record.PIT, true)
		}
		return nil, err
	}
	if ctx.Err() != nil {
		return nil, errors.New("SEARCH_CANDIDATE_REPLY_UNKNOWN")
	}
	return reply, nil
}

func (c *Candidates) enter(ctx context.Context) error {
	// 先限候选并发，再申请任何SQL锁；池8连接要给消费、Ready和关闭器保留资源。
	select {
	case c.admission <- struct{}{}:
		return nil
	case <-ctx.Done():
		return errors.New("SEARCH_CANDIDATE_ADMISSION_UNAVAILABLE")
	}
}
func (c *Candidates) leave() { <-c.admission }

func randomToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", errors.New("SEARCH_CURSOR_ENTROPY_UNAVAILABLE")
	}
	return hex.EncodeToString(raw[:]), nil
}

func operationDeadline(ctx context.Context, expires time.Time) int64 {
	deadline, ok := ctx.Deadline()
	if !ok || expires.Before(deadline) {
		deadline = expires
	}
	return deadline.UnixMilli()
}

func (c *Candidates) transact(ctx context.Context, work func(*sql.Tx) error) error {
	tx, err := c.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return errors.New("SEARCH_CURSOR_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	if err := work(tx); err != nil {
		return err
	}
	if tx.Commit() != nil {
		return errors.New("SEARCH_CURSOR_COMMIT_UNKNOWN")
	}
	return nil
}

// closing.go：保留原实现，按业务合并文件。

func (c *Candidates) CloseCursor(ctx context.Context, request *pb.CloseCursorRequest) (*pb.OperationResult, error) {
	if request == nil || domain.ValidateScope(request.Scope) != nil {
		return nil, errors.New("SEARCH_SCOPE_INVALID")
	}
	anchor, err := c.codec.Open(request.Cursor)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	hash := sha256.Sum256(anchor.Chain[:])
	record, err := c.closeHash(ctx, hash, request.Scope)
	if err != nil {
		return nil, err
	}
	state, code := pb.OperationResult_PROCESSING, "CURSOR_CLOSING"
	if record.State == repository.Closed {
		state, code = pb.OperationResult_SUCCEEDED, "CURSOR_CLOSED"
	}
	return &pb.OperationResult{State: state, OperationId: hex.EncodeToString(hash[:]), Generation: record.Generation, BusinessCode: code}, nil
}

func (c *Candidates) closeHash(ctx context.Context, hash [32]byte, scope *pb.SearchScope) (repository.Record, error) {
	// 关闭器与显式Close共享一个槽，先准入再SQL；不与查询mutex共用，才能在ES在途时关闭。
	select {
	case c.closingAdmission <- struct{}{}:
		defer func() { <-c.closingAdmission }()
	case <-ctx.Done():
		return repository.Record{}, errors.New("SEARCH_CURSOR_CLOSE_BUSY")
	}
	var record repository.Record
	claimed := false
	claim, err := randomToken()
	if err != nil {
		return record, err
	}
	err = c.transact(ctx, func(tx *sql.Tx) error {
		var exists bool
		var err error
		if scope == nil {
			record, exists, err = repository.TryLock(ctx, tx, hash)
		} else {
			record, exists, err = repository.Lock(ctx, tx, hash)
		}
		if err != nil {
			return err
		}
		if !exists {
			if scope == nil {
				return nil
			}
			return errors.New("SEARCH_CURSOR_INVALID")
		}
		if scope != nil && (hex.EncodeToString(record.ScopeHash[:]) != scope.ScopeHash || !bytes.Equal(record.Session, scope.SessionRef)) {
			return errors.New("SEARCH_CURSOR_BINDING_MISMATCH")
		}
		if record.State == repository.Closed {
			return nil
		}
		if scope == nil && record.Expires.After(time.Now()) {
			// Due只是旧观察，领取前必须锁内重核；原query刚确认或新close仍在途就让位。
			active := (record.Control.Phase == domain.PhaseOpening || record.Control.Phase == domain.PhaseInflight) && record.Control.Deadline > time.Now().UnixMilli()
			if active || (record.State == repository.Open && record.Control.Phase == domain.PhaseIdle) ||
				(record.State == repository.Closing && record.Control.RetryAt > time.Now().UnixMilli()) {
				return nil
			}
		}
		uncertain := record.Control.Uncertain || record.Control.Phase != domain.PhaseIdle
		record.State = repository.Closing
		record.Control = domain.Control{Version: 1, Phase: domain.PhaseInflight, Tail: record.Control.Tail,
			Nonce: claim, Deadline: operationDeadline(ctx, record.Expires), Uncertain: uncertain}
		if err := repository.Save(ctx, tx, record); err != nil {
			return err
		}
		claimed = true
		return nil
	})
	if err != nil || !claimed {
		return record, err
	}
	var closeErr error
	if record.PIT != "" {
		closeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		closeErr = c.indexes.ClosePIT(closeCtx, record.PIT)
		cancel()
	}
	// ES无论成功还是未知都先回SQL确认原claim；中途query迟到改了最新句柄必须留给下一轮。
	finish, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	defer cancel()
	err = c.transact(finish, func(tx *sql.Tx) error {
		current, exists, err := repository.Lock(finish, tx, hash)
		if err != nil {
			return err
		}
		if !exists || current.State != repository.Closing || current.Control.Nonce != claim {
			return errors.New("SEARCH_CURSOR_CLOSE_CONFIRMATION_STALE")
		}
		expired := !current.Expires.After(time.Now())
		current.Control.Phase = domain.PhaseIdle
		current.Control.Nonce = ""
		current.Control.Deadline = 0
		if closeErr == nil {
			current.PIT = ""
		}
		if expired || (closeErr == nil && !current.Control.Uncertain) {
			// 绝对期限兜底未知句柄，只证明应用链不可再用；不承诺ES reaper立即排空所有context。
			current.State = repository.Closed
			current.PIT = ""
			current.Control.RetryAt = 0
		} else if closeErr == nil {
			// 关闭已知句柄不能证明未知轮换句柄不存在，占额保留到原绝对期限。
			current.Control.RetryAt = current.Expires.UnixMilli()
		} else {
			current.Control.RetryAt = min(time.Now().Add(time.Second).UnixMilli(), current.Expires.UnixMilli())
		}
		if err := repository.Save(finish, tx, current); err != nil {
			return err
		}
		record = current
		return nil
	})
	return record, err
}

func (c *Candidates) Cleanup(ctx context.Context) error {
	c.sweepMu.Lock()
	defer c.sweepMu.Unlock()
	var due [][32]byte
	if err := c.transact(ctx, func(tx *sql.Tx) error {
		if err := repository.Check(ctx, tx); err != nil {
			return err
		}
		var err error
		due, err = repository.Due(ctx, tx, time.Now().UTC(), 16)
		return err
	}); err != nil {
		return err
	}
	var firstError error
	for _, hash := range due {
		if ctx.Err() != nil {
			if firstError == nil {
				firstError = errors.New("SEARCH_CURSOR_RECOVERY_UNAVAILABLE")
			}
			break
		}
		// 单项锁/确认故障有独立预算，保留首错但仍处理后续项；不能用坏首行垄断整轮。
		item, cancel := context.WithTimeout(ctx, 2*time.Second)
		_, err := c.closeHash(item, hash, nil)
		cancel()
		if err != nil && firstError == nil {
			firstError = err
		}
	}
	if err := c.transact(ctx, func(tx *sql.Tx) error { return repository.DeleteClosed(ctx, tx, time.Now().UTC(), 16) }); err != nil {
		return err
	}
	if firstError != nil {
		return firstError
	}
	c.recovered.Store(true)
	return nil
}

func (c *Candidates) Ready(ctx context.Context) error {
	if !c.recovered.Load() {
		return errors.New("SEARCH_CURSOR_RECOVERY_REQUIRED")
	}
	if err := c.generation.Ready(ctx); err != nil {
		return err
	}
	return c.transact(ctx, func(tx *sql.Tx) error { return repository.Check(ctx, tx) })
}

// continuation.go：保留原实现，按业务合并文件。

func sameBatch(anchor domain.Batch, batch elastic.CandidateBatch, digest [32]byte) bool {
	return anchor.Count == uint8(len(batch.Candidates)) && anchor.More == batch.More && anchor.Digest == digest && bytes.Equal(anchor.After, batch.End)
}

func (c *Candidates) retainOpened(ctx context.Context, opened repository.Record) error {
	return c.transact(ctx, func(tx *sql.Tx) error {
		current, exists, err := repository.Lock(ctx, tx, opened.Hash)
		if err != nil {
			return err
		}
		if !exists || current.State != repository.Open || current.Control.Nonce != opened.Control.Nonce || current.Control.Phase != domain.PhaseOpening {
			return errors.New("SEARCH_CURSOR_OPENING_CHANGED")
		}
		current.PIT = opened.PIT
		current.Control.Phase = domain.PhaseInflight
		return repository.Save(ctx, tx, current)
	})
}

func (c *Candidates) confirmQuery(ctx context.Context, queried repository.Record, end json.RawMessage) error {
	return c.transact(ctx, func(tx *sql.Tx) error {
		current, exists, err := repository.Lock(ctx, tx, queried.Hash)
		if err != nil {
			return err
		}
		// Close/worker只能使状态收紧；迟到的ES回复必须匹配原在途nonce，绝不重新OPEN。
		if !exists || current.State != repository.Open || current.Control.Nonce != queried.Control.Nonce ||
			current.Control.Phase != domain.PhaseInflight || current.Control.Deadline <= time.Now().UnixMilli() || !current.Expires.After(time.Now()) {
			return errors.New("SEARCH_CURSOR_CONFIRMATION_STALE")
		}
		current.PIT = queried.PIT
		current.Control = domain.Control{Version: 1, Phase: domain.PhaseIdle, Tail: append(json.RawMessage(nil), end...)}
		return repository.Save(ctx, tx, current)
	})
}

func (c *Candidates) fail(ctx context.Context, record repository.Record, pit string, uncertain bool) {
	if record.Hash == ([32]byte{}) {
		return
	}
	// 仅做有界SQL补偿；原请求取消后不继续ES或重新打开资源。SQL再未知由持久inflight期限恢复。
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	defer cancel()
	_ = c.transact(cleanup, func(tx *sql.Tx) error {
		current, exists, err := repository.Lock(cleanup, tx, record.Hash)
		if err != nil || !exists {
			return err
		}
		if current.State == repository.Closed {
			return nil
		}
		if current.State == repository.Open && current.Control.Nonce != record.Control.Nonce {
			// COMMIT已成功但回执丢失时，重读可确认IDLE且nonce已清空，保留可重试批。
			// 较新的在途nonce同样不能被旧补偿收紧；只有原nonce仍在途才标记未知关闭。
			return nil
		}
		if pit != "" {
			current.PIT = pit
		}
		current.State = repository.Closing
		current.Control = domain.Control{Version: 1, Phase: domain.PhaseIdle, Tail: current.Control.Tail, Uncertain: current.Control.Uncertain || uncertain}
		return repository.Save(cleanup, tx, current)
	})
}

// coordination.go：保留原实现，按业务合并文件。

type keyLock struct {
	token      chan struct{}
	references int
}
type keyLocks struct {
	mu      sync.Mutex
	entries map[[32]byte]*keyLock
}

func (locks *keyLocks) acquire(ctx context.Context, key [32]byte) (func(), error) {
	locks.mu.Lock()
	if locks.entries == nil {
		locks.entries = make(map[[32]byte]*keyLock)
	}
	entry := locks.entries[key]
	if entry == nil {
		entry = &keyLock{token: make(chan struct{}, 1)}
		locks.entries[key] = entry
	}
	entry.references++
	locks.mu.Unlock()
	drop := func() {
		locks.mu.Lock()
		defer locks.mu.Unlock()
		entry.references--
		if entry.references == 0 {
			delete(locks.entries, key)
		}
	}
	select {
	case entry.token <- struct{}{}:
		return func() { <-entry.token; drop() }, nil
	case <-ctx.Done():
		drop()
		return nil, errors.New("SEARCH_CURSOR_BUSY")
	}
}

// registration.go：保留原实现，按业务合并文件。

func (c *Candidates) reserve(ctx context.Context, query domain.Query) (repository.Record, [32]byte, error) {
	var chain [32]byte
	if _, err := rand.Read(chain[:]); err != nil {
		return repository.Record{}, chain, errors.New("SEARCH_CURSOR_ENTROPY_UNAVAILABLE")
	}
	nonce, err := randomToken()
	if err != nil {
		return repository.Record{}, chain, err
	}
	queryHash, _ := hex.DecodeString(query.QueryHash)
	scopeHash, _ := hex.DecodeString(query.Scope.ScopeHash)
	record := repository.Record{Hash: sha256.Sum256(chain[:]), PageSize: query.PageSize, Session: query.Scope.SessionRef,
		Expires: time.Now().Add(c.ttl).UTC().Truncate(time.Millisecond), State: repository.Open}
	copy(record.ScopeHash[:], scopeHash)
	copy(record.QueryHash[:], queryHash)
	record.Control = domain.Control{Version: 1, Phase: domain.PhaseOpening, Nonce: nonce, Deadline: operationDeadline(ctx, record.Expires)}
	err = c.transact(ctx, func(tx *sql.Tx) error {
		// 排他代次锁串行化所有新链配额；提交后再进入共享Guard，避免反向嵌套锁自锁。
		target, exists, err := generation.Find(ctx, tx, c.alias, true)
		if err != nil {
			return err
		}
		if !exists {
			return errors.New("SEARCH_GENERATION_MISSING")
		}
		if err := generation.CheckSwitching(ctx, tx); err != nil {
			return err
		}
		record.Physical, record.Generation = target.Physical, target.Generation
		return repository.Create(ctx, tx, &record, 128, 1024)
	})
	return record, chain, err
}

func matchesQuery(record repository.Record, query domain.Query) bool {
	return hex.EncodeToString(record.QueryHash[:]) == query.QueryHash && hex.EncodeToString(record.ScopeHash[:]) == query.Scope.ScopeHash &&
		bytes.Equal(record.Session, query.Scope.SessionRef) && record.PageSize == query.PageSize
}

func (c *Candidates) beginQuery(ctx context.Context, hash [32]byte, query domain.Query, target generation.Target, newChain bool) (repository.Record, error) {
	var record repository.Record
	err := c.transact(ctx, func(tx *sql.Tx) error {
		var exists bool
		var err error
		record, exists, err = repository.Lock(ctx, tx, hash)
		if err != nil {
			return err
		}
		if !exists {
			return errors.New("SEARCH_CURSOR_INVALID")
		}
		if !matchesQuery(record, query) {
			return errors.New("SEARCH_CURSOR_BINDING_MISMATCH")
		}
		if record.State != repository.Open {
			return errors.New("SEARCH_CURSOR_CLOSED")
		}
		if !record.Expires.After(time.Now()) {
			return errors.New("SEARCH_CURSOR_EXPIRED")
		}
		if record.Physical != target.Physical || record.Generation != target.Generation {
			return errors.New("SEARCH_CURSOR_STALE")
		}
		if newChain {
			if record.Control.Phase != domain.PhaseOpening {
				return errors.New("SEARCH_CURSOR_OPENING_CHANGED")
			}
			return nil
		}
		if record.Control.Phase != domain.PhaseIdle {
			return errors.New("SEARCH_CURSOR_BUSY")
		}
		nonce, err := randomToken()
		if err != nil {
			return err
		}
		record.Control.Phase = domain.PhaseInflight
		record.Control.Nonce = nonce
		record.Control.Deadline = operationDeadline(ctx, record.Expires)
		return repository.Save(ctx, tx, record)
	})
	return record, err
}
````

UTF-8/LF全文SHA-256：`59353f307175dc23e04022741c8a03a405ae35e9ff5b80bdc640e4302f48e1e7`。[参考实现](../../../../services/search/internal/logic/candidates/candidates_service.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
