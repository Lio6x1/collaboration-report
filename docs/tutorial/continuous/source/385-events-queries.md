# services/search/internal/repository/event/events_queries.go

阶段05：[功能讲义](../05-search.md) · [分步编写](../write/385-events-queries.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`readApplication`、`LockApplication`、`SaveApplication`、`Complete`、`AdvanceCheckpoint`、`BindLogIdentity`、`validClusterID`、`Receipt`、`Receive`、`SetApplying`、`SetRetry`、`transition`、`Checkpoint`、`CheckRecovery`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package event

import (
	"bytes"
	domain "collaboration/search/internal/domain/event"
	index "collaboration/search/internal/domain/index"
	"context"
	"database/sql"
	"errors"
	"github.com/go-sql-driver/mysql"
	"math"
	"strconv"
)

// application.go：保留原实现，按业务合并文件。

func readApplication(ctx context.Context, tx *sql.Tx, physical string, document uint64) (index.State, error) {
	var value index.State
	var digest []byte
	var release sql.NullString
	var visibility string
	err := tx.QueryRowContext(ctx, `SELECT applied_seq,authority_digest,CAST(release_id AS CHAR),visibility,visibility_policy,revision
		FROM index_application WHERE physical_index=? AND document_id=? FOR UPDATE`, physical, document).
		Scan(&value.Sequence, &digest, &release, &visibility, &value.Policy, &value.Revision)
	if err != nil {
		return value, err
	}
	if len(digest) != 32 || value.Revision == 0 {
		return value, errors.New("SEARCH_INDEX_STATE_INVALID")
	}
	copy(value.Digest[:], digest)
	value.Visible = visibility == "PUBLIC"
	if release.Valid {
		value.Release, err = strconv.ParseUint(release.String, 10, 64)
		if err != nil || value.Release == 0 {
			return value, errors.New("SEARCH_INDEX_STATE_INVALID")
		}
	}
	if (visibility != "PUBLIC" && visibility != "HIDDEN") || (value.Visible != (value.Release != 0)) ||
		(value.Policy != "PUBLIC" && value.Policy != "SPACE_MEMBERS" && value.Policy != "COLLABORATORS") {
		return value, errors.New("SEARCH_INDEX_STATE_INVALID")
	}
	return value, nil
}

// LockApplication按物理索引隔离同文档。新行的零摘要仅存在于未提交事务，
// 调用方必须ES成功并SaveApplication后才可提交；失败回滚不会留下虚假水位。
func LockApplication(ctx context.Context, tx *sql.Tx, physical string, document uint64) (index.State, error) {
	if !physicalName.MatchString(physical) || document == 0 {
		return index.State{}, errors.New("SEARCH_INDEX_BINDING_INVALID")
	}
	value, err := readApplication(ctx, tx, physical, document)
	if err == nil {
		if value.Digest == ([32]byte{}) {
			return value, errors.New("SEARCH_INDEX_STATE_UNBOUND")
		}
		return value, nil
	}
	if err != sql.ErrNoRows {
		return value, errors.New("SEARCH_APPLICATION_READ_FAILED")
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO index_application(physical_index,document_id,applied_seq,authority_digest,release_id,visibility,visibility_policy)
		VALUES(?,?,0,?,NULL,'HIDDEN','PUBLIC')`, physical, document, make([]byte, 32)); err != nil {
		return value, errors.New("SEARCH_APPLICATION_LOCK_FAILED")
	}
	return index.State{Revision: 1}, nil
}

func SaveApplication(ctx context.Context, tx *sql.Tx, physical string, document uint64, next index.State) error {
	if !physicalName.MatchString(physical) || document == 0 {
		return errors.New("SEARCH_INDEX_BINDING_INVALID")
	}
	current, err := readApplication(ctx, tx, physical, document)
	if err != nil {
		return errors.New("SEARCH_APPLICATION_READ_FAILED")
	}
	apply, err := index.ShouldApply(current, index.Authority{Sequence: next.Sequence, Release: next.Release, Visible: next.Visible, Policy: next.Policy, Digest: next.Digest})
	if err != nil {
		return err
	}
	if next.Sequence < current.Sequence {
		return errors.New("SEARCH_APPLICATION_REGRESSED")
	}
	if !apply {
		return nil
	}
	if current.Revision == math.MaxUint64 {
		return errors.New("SEARCH_INDEX_REVISION_EXHAUSTED")
	}
	visibility := "HIDDEN"
	var release any
	if next.Visible {
		visibility, release = "PUBLIC", next.Release
	}
	result, err := tx.ExecContext(ctx, `UPDATE index_application SET applied_seq=?,authority_digest=?,release_id=?,visibility=?,visibility_policy=?,
		revision=?,updated_at=UTC_TIMESTAMP(6) WHERE physical_index=? AND document_id=? AND revision=?`, next.Sequence, next.Digest[:], release, visibility, next.Policy,
		current.Revision+1, physical, document, current.Revision)
	if err != nil {
		return errors.New("SEARCH_APPLICATION_WRITE_FAILED")
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return errors.New("SEARCH_APPLICATION_CHANGED")
	}
	return nil
}

// Complete只认可已经在同事务保存的实际应用状态；RPC/ES未知不能先写成功回执。
func Complete(ctx context.Context, tx *sql.Tx, consumer, id, physical string, applied index.State) error {
	if !consumerName.MatchString(consumer) || !physicalName.MatchString(physical) || len(id) == 0 || len(id) > 96 {
		return errors.New("SEARCH_APPLICATION_BINDING_INVALID")
	}
	var document, sequence uint64
	var state string
	if tx.QueryRowContext(ctx, "SELECT document_id,aggregate_seq,state FROM consumed_event WHERE consumer_name=? AND event_id=? FOR UPDATE", consumer, id).
		Scan(&document, &sequence, &state) != nil {
		return errors.New("SEARCH_EVENT_RECEIPT_MISSING")
	}
	if state != "RECEIVED" && state != "APPLYING" && state != "RETRY" {
		return errors.New("SEARCH_EVENT_STATE_CHANGED")
	}
	actual, err := readApplication(ctx, tx, physical, document)
	if err != nil || actual.Sequence < sequence || actual.Sequence != applied.Sequence || actual.Digest != applied.Digest ||
		actual.Release != applied.Release || actual.Visible != applied.Visible || actual.Policy != applied.Policy || actual.Digest == ([32]byte{}) {
		return errors.New("SEARCH_APPLICATION_NOT_DURABLE")
	}
	result, err := tx.ExecContext(ctx, `UPDATE event_origin SET applied_index=?,applied_seq=?,authority_digest=?
		WHERE consumer_name=? AND event_id=? AND applied_index IS NULL AND applied_seq IS NULL AND authority_digest IS NULL`,
		physical, actual.Sequence, actual.Digest[:], consumer, id)
	if err != nil {
		return errors.New("SEARCH_ORIGIN_COMPLETION_FAILED")
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return errors.New("SEARCH_ORIGIN_COMPLETION_FAILED")
	}
	if _, err = tx.ExecContext(ctx, "UPDATE consumed_event SET state='SUCCEEDED',last_error=NULL,retry_at=NULL WHERE consumer_name=? AND event_id=?", consumer, id); err != nil {
		return errors.New("SEARCH_EVENT_COMPLETION_FAILED")
	}
	return nil
}

// checkpoint.go：保留原实现，按业务合并文件。

// AdvanceCheckpoint核对整条记录所有行成功，才写SQL下一offset；Kafka实际提交仍在tx.Commit之后。
// 原SQL已成功但Kafka未知时允许较早记录重读，不允许越过同分区未完成的中间记录。
func AdvanceCheckpoint(ctx context.Context, tx *sql.Tx, consumer string, position Position, events []domain.Event) error {
	if err := validatePosition(consumer, position); err != nil {
		return err
	}
	if position.Offset == math.MaxInt64 || len(events) == 0 || len(events) > 1024 {
		return errors.New("SEARCH_CHECKPOINT_INPUT_INVALID")
	}
	for _, value := range events {
		if err := validateEvent(value); err != nil {
			return err
		}
		var state, topic string
		var digest []byte
		var partition int
		var offset int64
		if tx.QueryRowContext(ctx, `SELECT c.state,o.raw_digest,o.kafka_topic,o.kafka_partition,o.kafka_offset
			FROM consumed_event c JOIN event_origin o ON o.consumer_name=c.consumer_name AND o.event_id=c.event_id
			WHERE c.consumer_name=? AND c.event_id=? FOR UPDATE`, consumer, value.ID).
			Scan(&state, &digest, &topic, &partition, &offset) != nil || state != "SUCCEEDED" ||
			!bytes.Equal(digest, value.RawDigest[:]) || topic != position.Topic || partition != position.Partition || offset > position.Offset {
			return errors.New("SEARCH_RECORD_ROWS_NOT_COMPLETE")
		}
	}
	var next, revision uint64
	err := tx.QueryRowContext(ctx, `SELECT next_offset,revision FROM consumer_checkpoint
		WHERE consumer_name=? AND kafka_topic=? AND partition_id=? FOR UPDATE`, consumer, position.Topic, position.Partition).Scan(&next, &revision)
	if err == sql.ErrNoRows {
		if position.Offset != 0 {
			return errors.New("SEARCH_OFFSET_GAP")
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO consumer_checkpoint(consumer_name,kafka_topic,partition_id,next_offset)
			VALUES(?,?,?,1)`, consumer, position.Topic, position.Partition); err != nil {
			return errors.New("SEARCH_CHECKPOINT_WRITE_FAILED")
		}
		return nil
	}
	if err != nil {
		return errors.New("SEARCH_CHECKPOINT_READ_FAILED")
	}
	if next > uint64(math.MaxInt64) || revision == 0 {
		return errors.New("SEARCH_CHECKPOINT_UNBOUND")
	}
	if uint64(position.Offset) > next {
		return errors.New("SEARCH_OFFSET_GAP")
	}
	if uint64(position.Offset) < next {
		return nil
	}
	if revision == math.MaxUint64 {
		return errors.New("SEARCH_CHECKPOINT_REVISION_EXHAUSTED")
	}
	result, err := tx.ExecContext(ctx, `UPDATE consumer_checkpoint SET next_offset=?,revision=?,updated_at=UTC_TIMESTAMP(6)
		WHERE consumer_name=? AND kafka_topic=? AND partition_id=? AND revision=?`, position.Offset+1, revision+1,
		consumer, position.Topic, position.Partition, revision)
	if err != nil {
		return errors.New("SEARCH_CHECKPOINT_WRITE_FAILED")
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return errors.New("SEARCH_CHECKPOINT_CHANGED")
	}
	return nil
}

// log_identity.go：保留原实现，按业务合并文件。

// BindLogIdentity在调用方事务内固定消费者的物理日志，绝不覆盖旧身份。
// 旧checkpoint的首次升级需要调用方先验证历史来源；当前broker身份不能证明旧offset所属日志。
func BindLogIdentity(ctx context.Context, tx *sql.Tx, consumer, topic, cluster string, id [16]byte) error {
	if tx == nil || !consumerName.MatchString(consumer) || !topicName.MatchString(topic) ||
		!validClusterID(cluster) || id == ([16]byte{}) {
		return errors.New("SEARCH_LOG_IDENTITY_INVALID")
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO consumer_log_identity(consumer_name,kafka_topic,cluster_id,topic_id)
		VALUES(?,?,?,?)`, consumer, topic, cluster, id[:])
	if err != nil {
		// 只接受主键冲突；INSERT IGNORE会同时吞掉截断等错误，不能用于身份凭据。
		var duplicate *mysql.MySQLError
		if !errors.As(err, &duplicate) || duplicate.Number != 1062 {
			return errors.New("SEARCH_LOG_IDENTITY_WRITE_FAILED")
		}
	}
	var storedTopic, storedCluster string
	var storedID []byte
	var revision uint64
	// 当前读避免REPEATABLE READ旧快照漏掉并发首次绑定；共享锁只需SELECT权限。
	// 本表没有UPDATE/DELETE授权，无需通过更新原事实来取得锁。
	err = tx.QueryRowContext(ctx, `SELECT kafka_topic,cluster_id,topic_id,revision FROM consumer_log_identity
		WHERE consumer_name=? FOR SHARE`, consumer).Scan(&storedTopic, &storedCluster, &storedID, &revision)
	if err != nil {
		return errors.New("SEARCH_LOG_IDENTITY_READ_FAILED")
	}
	if storedTopic != topic || storedCluster != cluster || !bytes.Equal(storedID, id[:]) || revision != 1 {
		return errors.New("SEARCH_LOG_IDENTITY_CHANGED")
	}
	return nil
}

func validClusterID(value string) bool {
	if len(value) == 0 || len(value) > 160 {
		return false
	}
	for _, character := range value {
		if character < 33 || character > 126 {
			return false
		}
	}
	return true
}

// origin.go：保留原实现，按业务合并文件。

type Receipt struct {
	State    string
	Physical string
	Applied  index.State
}

// Receive接受调用方同一事务：原回执和原行一起落定，不把重复投递的后来内容覆盖原事实。
func Receive(ctx context.Context, tx *sql.Tx, consumer string, value domain.Event, position Position) (Receipt, error) {
	var result Receipt
	if err := validatePosition(consumer, position); err != nil {
		return result, err
	}
	if err := validateEvent(value); err != nil {
		return result, err
	}
	var document, sequence uint64
	var rawDigest, original, authority []byte
	var topic, offset, physical, applied sql.NullString
	var partition sql.NullInt64
	err := tx.QueryRowContext(ctx, `SELECT c.document_id,c.aggregate_seq,c.state,
		o.raw_digest,o.original_row,o.kafka_topic,o.kafka_partition,CAST(o.kafka_offset AS CHAR),
		o.applied_index,CAST(o.applied_seq AS CHAR),o.authority_digest
		FROM consumed_event c LEFT JOIN event_origin o ON o.consumer_name=c.consumer_name AND o.event_id=c.event_id
		WHERE c.consumer_name=? AND c.event_id=? FOR UPDATE`, consumer, value.ID).
		Scan(&document, &sequence, &result.State, &rawDigest, &original, &topic, &partition, &offset, &physical, &applied, &authority)
	if err == sql.ErrNoRows {
		if _, err = tx.ExecContext(ctx, "INSERT INTO consumed_event(consumer_name,event_id,document_id,aggregate_seq,state) VALUES(?,?,?,?,'RECEIVED')", consumer, value.ID, value.Document, value.Sequence); err != nil {
			return result, errors.New("SEARCH_LEDGER_WRITE_FAILED")
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO event_origin(consumer_name,event_id,raw_digest,original_row,kafka_topic,kafka_partition,kafka_offset)
			VALUES(?,?,?,?,?,?,?)`, consumer, value.ID, value.RawDigest[:], value.Original, position.Topic, position.Partition, position.Offset); err != nil {
			return result, errors.New("SEARCH_ORIGIN_WRITE_FAILED")
		}
		result.State = "RECEIVED"
		return result, nil
	}
	if err != nil {
		return result, errors.New("SEARCH_LEDGER_READ_FAILED")
	}
	if document != value.Document || sequence != value.Sequence || len(rawDigest) != 32 ||
		!bytes.Equal(rawDigest, value.RawDigest[:]) || !bytes.Equal(original, value.Original) ||
		!topic.Valid || topic.String != position.Topic || !partition.Valid || partition.Int64 != int64(position.Partition) || !offset.Valid {
		return result, errors.New("SEARCH_ORIGINAL_EVENT_CONFLICT")
	}
	first, err := strconv.ParseInt(offset.String, 10, 64)
	if err != nil || first < 0 {
		return result, errors.New("SEARCH_ORIGIN_POSITION_UNBOUND")
	}
	switch result.State {
	case "RECEIVED", "APPLYING", "RETRY", "REJECTED":
		if physical.Valid || applied.Valid || len(authority) != 0 {
			return result, errors.New("SEARCH_APPLICATION_RECEIPT_UNBOUND")
		}
	case "SUCCEEDED":
		if !physical.Valid || !physicalName.MatchString(physical.String) || !applied.Valid || len(authority) != 32 {
			return result, errors.New("SEARCH_APPLICATION_RECEIPT_UNBOUND")
		}
		result.Physical = physical.String
		result.Applied.Sequence, err = strconv.ParseUint(applied.String, 10, 64)
		if err != nil || result.Applied.Sequence < sequence {
			return result, errors.New("SEARCH_APPLICATION_RECEIPT_UNBOUND")
		}
		copy(result.Applied.Digest[:], authority)
		if result.Applied.Digest == ([32]byte{}) {
			return result, errors.New("SEARCH_APPLICATION_RECEIPT_UNBOUND")
		}
	default:
		return result, errors.New("SEARCH_EVENT_STATE_UNKNOWN")
	}
	return result, nil
}

func SetApplying(ctx context.Context, tx *sql.Tx, consumer, id string) error {
	return transition(ctx, tx, consumer, id, "APPLYING", "")
}
func SetRetry(ctx context.Context, tx *sql.Tx, consumer, id, code string) error {
	if !errorCode.MatchString(code) {
		return errors.New("SEARCH_RETRY_CODE_INVALID")
	}
	return transition(ctx, tx, consumer, id, "RETRY", code)
}
func transition(ctx context.Context, tx *sql.Tx, consumer, id, state, code string) error {
	if !consumerName.MatchString(consumer) || len(id) == 0 || len(id) > 96 {
		return errors.New("SEARCH_EVENT_BINDING_INVALID")
	}
	result, err := tx.ExecContext(ctx, `UPDATE consumed_event SET state=?,last_error=NULLIF(?,''),
		retry_at=CASE WHEN ?='RETRY' THEN DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND) ELSE NULL END
		WHERE consumer_name=? AND event_id=? AND state IN ('RECEIVED','APPLYING','RETRY')`, state, code, state, consumer, id)
	if err != nil {
		return errors.New("SEARCH_EVENT_TRANSITION_FAILED")
	}
	count, err := result.RowsAffected()
	if err != nil || count > 1 {
		return errors.New("SEARCH_EVENT_TRANSITION_FAILED")
	}
	// 同一APPLYING重试可返回0 affected rows；必须再读实际状态，不能把缺行解释为成功。
	var actual string
	if tx.QueryRowContext(ctx, "SELECT state FROM consumed_event WHERE consumer_name=? AND event_id=? FOR UPDATE", consumer, id).Scan(&actual) != nil || actual != state {
		return errors.New("SEARCH_EVENT_STATE_CHANGED")
	}
	return nil
}

// read_checkpoint.go：保留原实现，按业务合并文件。

// Checkpoint读的是本消费者已落定的下一offset。没有行只代表从0开始，
// 不能把当前Kafka记录位置或broker最新位置当成已消费过的历史。
func Checkpoint(ctx context.Context, tx *sql.Tx, consumer, topic string, partition int) (int64, error) {
	if err := validatePosition(consumer, Position{Topic: topic, Partition: partition}); err != nil {
		return 0, err
	}
	var next, revision uint64
	err := tx.QueryRowContext(ctx, `SELECT next_offset,revision FROM consumer_checkpoint
		WHERE consumer_name=? AND kafka_topic=? AND partition_id=?`, consumer, topic, partition).Scan(&next, &revision)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, errors.New("SEARCH_CHECKPOINT_READ_FAILED")
	}
	if next > math.MaxInt64 || revision == 0 {
		return 0, errors.New("SEARCH_CHECKPOINT_UNBOUND")
	}
	return int64(next), nil
}

// recovery.go：保留原实现，按业务合并文件。

// 启动前核验本消费者已有账本，缺原行、改摘要、错误topic和假成功都拒绝。
// 未完成但原事实完整的行允许从原Kafka位置恢复；这不是“待办为空”的就绪判断。
// MEDIUMBLOB的摘要按原字节核验；JSON_VALID需显式解释为UTF-8文本，不能检查binary字符集。
func CheckRecovery(ctx context.Context, tx *sql.Tx, consumer, topic string) error {
	if err := validatePosition(consumer, Position{Topic: topic}); err != nil {
		return err
	}
	var id string
	var partition uint64
	checkpointErr := tx.QueryRowContext(ctx, `SELECT partition_id FROM consumer_checkpoint WHERE consumer_name=?
		AND (kafka_topic<>? OR partition_id>=? OR next_offset>9223372036854775807 OR revision=0) LIMIT 1`, consumer, topic, domain.PartitionCount).Scan(&partition)
	if checkpointErr == nil {
		return errors.New("SEARCH_CHECKPOINT_UNBOUND")
	}
	if checkpointErr != sql.ErrNoRows {
		return errors.New("SEARCH_RECOVERY_LEDGER_READ_FAILED")
	}
	err := tx.QueryRowContext(ctx, `SELECT c.event_id FROM consumed_event c
		LEFT JOIN event_origin o ON o.consumer_name=c.consumer_name AND o.event_id=c.event_id
		LEFT JOIN index_application a ON a.physical_index=o.applied_index AND a.document_id=c.document_id
		WHERE c.consumer_name=? AND (o.event_id IS NULL OR o.kafka_topic<>? OR o.kafka_partition>=?
		OR o.kafka_offset>9223372036854775806 OR o.raw_digest<>UNHEX(SHA2(o.original_row,256))
		OR NOT JSON_VALID(CAST(o.original_row AS CHAR CHARACTER SET utf8mb4)) OR c.state='REJECTED'
		OR (c.state='SUCCEEDED' AND (o.applied_index IS NULL OR o.applied_seq IS NULL OR o.authority_digest IS NULL
			OR o.applied_seq<c.aggregate_seq OR o.authority_digest=UNHEX(REPEAT('00',32)) OR a.document_id IS NULL
			OR a.applied_seq<o.applied_seq OR (a.applied_seq=o.applied_seq AND a.authority_digest<>o.authority_digest)))
		OR (c.state<>'SUCCEEDED' AND (o.applied_index IS NOT NULL OR o.applied_seq IS NOT NULL OR o.authority_digest IS NOT NULL)))
		LIMIT 1`, consumer, topic, domain.PartitionCount).Scan(&id)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return errors.New("SEARCH_RECOVERY_LEDGER_READ_FAILED")
	}
	return errors.New("SEARCH_ORIGIN_RECOVERY_REQUIRED")
}
````

UTF-8/LF全文SHA-256：`5be24621ac3513cab192ce004814be9400ea4f1c9d3e2da702399efdead34df7`。[参考实现](../../../../services/search/internal/repository/event/events_queries.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
