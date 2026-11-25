# services/document/internal/domain/document/errors.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/092-errors.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：定义模型、输入输出和纯规则；把状态与副作用分开。

主要声明：`FailureKind`、`RuleViolation`、`Error`、`Violation`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package document

import "errors"

var ErrSourcesRequired = errors.New("SOURCES_REQUIRED")

// RuleViolation只表达业务条件，HTTP/RPC适配层决定传输状态。
type FailureKind uint8

const (
	InvalidRequest FailureKind = iota
	SessionRequired
	PermissionDenied
	EntityNotFound
	StateConflict
	TemporarilyUnavailable
)

type RuleViolation struct {
	Kind FailureKind
	Code string
}

func (e RuleViolation) Error() string { return e.Code }
func Violation(kind FailureKind, code string) error {
	return RuleViolation{Kind: kind, Code: code}
}
````

UTF-8/LF全文SHA-256：`982219ee6abff35bb6d27c0386602f0364f13409a0487f3a593a03eabbac7785`。[参考实现](../../../../services/document/internal/domain/document/errors.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
