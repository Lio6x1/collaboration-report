package account

import (
	"errors"
	"regexp"
)

var (
	operationID            = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{15,95}$`)
	identityPurpose        = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,39}$`)
	ErrIdentityBinding     = errors.New("IDENTITY_BINDING_INVALID")
	ErrIdentityDenied      = errors.New("IDENTITY_NOT_CURRENT")
	ErrIdentityUnavailable = errors.New("IDENTITY_UNAVAILABLE")
)

func ValidOperationID(value string) bool     { return operationID.MatchString(value) }
func ValidIdentityPurpose(value string) bool { return identityPurpose.MatchString(value) }
