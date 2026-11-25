// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"crypto/sha256"
	"encoding/base64"
	"sort"
)

// authorization.go：保留原实现，按业务合并文件。

// Closure固定已提交的关闭计划；恢复必须继续原编号、原范围及原代次。
type Closure struct {
	Scope  string `json:"scope"`
	Epoch  uint64 `json:"epoch"`
	Entity uint64 `json:"entity"`
	Action string `json:"action"`
}
type AuthorizationTask struct {
	ID        uint64
	Operation string
	Payload   []byte
}
type AuthorizationStatus struct{ State, Scope, Epoch, ReceiptState string }
type TaskOwnership struct{ Instance, State string }

// identity.go：保留原实现，按业务合并文件。

// SessionVerification只携带账号权威验证所需元数据，业务层不保存HTTP请求。
type SessionVerification struct {
	Cookie, Origin, CSRF string
	Mutation             bool
}

type VerifiedSession struct {
	ActorID       uint64
	ExpiresUnixMs int64
}

// permissions.go：保留原实现，按业务合并文件。

// 权限来源取并集；撤销直接授权不能抹掉仍然有效的上级来源。
type GrantFact struct {
	User       uint64
	Kind       string
	Scope      uint64
	Capability string
}
type UnitFact struct{ ID, Chapter uint64 }
type PermissionFacts struct {
	Actor                                                             uint64
	Responsible, PersonalOwner, OrganizationAdmin, KnowledgeBaseAdmin bool
	Grants                                                            []GrantFact
	Units                                                             []UnitFact
}
type Permissions struct {
	Read, Comment, Review, Manage bool
	Edit                          map[uint64]bool
}

func ResolvePermissions(f PermissionFacts) Permissions {
	p := Permissions{Edit: map[uint64]bool{}, Manage: f.Responsible || f.PersonalOwner || f.OrganizationAdmin || f.KnowledgeBaseAdmin}
	for _, g := range f.Grants {
		if g.User != f.Actor {
			continue
		}
		switch g.Capability {
		case "MANAGE":
			if g.Kind == "DOCUMENT" {
				p.Manage = true
			}
		case "REVIEW":
			if g.Kind == "DOCUMENT" {
				p.Review = true
				p.Comment = true
				p.Read = true
			}
		case "COMMENT":
			if g.Kind == "DOCUMENT" {
				p.Comment = true
				p.Read = true
			}
		case "EDIT":
			for _, u := range f.Units {
				if (g.Kind == "UNIT" && g.Scope == u.ID) || (g.Kind == "CHAPTER" && g.Scope == u.Chapter) {
					p.Edit[u.ID] = true
					p.Read = true
				}
			}
		}
	}
	if p.Manage {
		p.Read = true
		p.Comment = true
		p.Review = true
		for _, u := range f.Units {
			p.Edit[u.ID] = true
		}
	}
	return p
}

// 草稿历史必须从稳定lineage证明原范围被当前权限完整覆盖；空映射不是空范围成功。
func CoversHistoricalRange(p Permissions, descendants []uint64, proven bool) bool {
	if p.Manage {
		return true
	}
	if !proven || len(descendants) == 0 {
		return false
	}
	for _, unit := range descendants {
		if !p.Edit[unit] {
			return false
		}
	}
	return true
}

type KBRoleFact struct {
	User        uint64
	Role, State string
}

func KnowledgeBaseNotificationTargets(roles []KBRoleFact, _ []uint64) []uint64 {
	seen := map[uint64]bool{}
	var list []uint64
	for _, r := range roles {
		if r.Role == "ADMIN" && r.State == "ACTIVE" && !seen[r.User] {
			seen[r.User] = true
			list = append(list, r.User)
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i] < list[j] })
	return list
}

// sharing.go：保留原实现，按业务合并文件。

// TokenHash只接受规范的32字节随机令牌；不同编码不能绕过同一邀请的持久身份。
func TokenHash(token string) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != 32 || base64.RawURLEncoding.EncodeToString(raw) != token {
		return nil, Violation(InvalidRequest, "INVITATION_TOKEN_INVALID")
	}
	hash := sha256.Sum256(raw)
	return hash[:], nil
}
