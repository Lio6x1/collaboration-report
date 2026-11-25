# services/search/internal/repository/event/validation.go

阶段05：[功能讲义](../05-search.md) · [分步编写](../write/386-validation.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`Position`、`validatePosition`、`validateEvent`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package event

import (
	"bytes"
	domain "collaboration/search/internal/domain/event"
	"crypto/sha256"
	"errors"
	"regexp"
)

type Position = domain.Position

var consumerName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,39}$`)
var topicName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,159}$`)
var physicalName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,159}$`)
var errorCode = regexp.MustCompile(`^[A-Z][A-Z0-9_]{1,63}$`)

func validatePosition(consumer string, position Position) error {
	if !consumerName.MatchString(consumer) || !topicName.MatchString(position.Topic) || position.Partition < 0 || position.Offset < 0 {
		return errors.New("SEARCH_MESSAGE_POSITION_INVALID")
	}
	return nil
}

func validateEvent(value domain.Event) error {
	stored, err := domain.RestoreEvent(value.Original)
	if err != nil || sha256.Sum256(value.Original) != value.RawDigest || !bytes.Equal(stored.Original, value.Original) ||
		stored.ID != value.ID || stored.Type != value.Type || stored.Document != value.Document ||
		stored.Sequence != value.Sequence || stored.Release != value.Release ||
		stored.PublicationEpoch != value.PublicationEpoch || !stored.OccurredAt.Equal(value.OccurredAt) {
		return errors.New("SEARCH_ORIGINAL_EVENT_INVALID")
	}
	return nil
}
````

UTF-8/LF全文SHA-256：`9add2f003ef9edf9dc6d86c21a363ae9909e6a0a4f2b04c699493c1287e29248`。[参考实现](../../../../services/search/internal/repository/event/validation.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
