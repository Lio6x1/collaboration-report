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
