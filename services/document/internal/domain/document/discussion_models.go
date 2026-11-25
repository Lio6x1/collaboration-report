// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	"encoding/json"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// comment_notification.go：保留原实现，按业务合并文件。

// CommentNotification固定创建/回复时的原讨论与显式@集合；恢复不能读取后来编辑的正文重新猜收件人。
type CommentNotification struct {
	Document        uint64   `json:"document_id"`
	Thread          uint64   `json:"thread_id"`
	Comment         uint64   `json:"comment_id"`
	ContextKind     string   `json:"context_kind"`
	ContextID       string   `json:"context_id"`
	Version         uint64   `json:"content_version_id"`
	Actor           uint64   `json:"actor_id"`
	Recipients      []uint64 `json:"recipients"`
	ReplyRecipients []uint64 `json:"reply_recipients,omitempty"`
	// 超过一批的收件人逐批提交，恢复只推进已持久化游标，不依赖进程内列表。
	DeliveryAfter uint64 `json:"delivery_after,omitempty"`
}

// comments.go：保留原实现，按业务合并文件。

// 普通批注只描述讨论，不携带正式change_request的状态或解除权限。
func ValidateCommentBody(body string) error {
	if !utf8.ValidString(body) || strings.TrimSpace(body) == "" || utf8.RuneCountInString(body) > 8000 {
		return Violation(InvalidRequest, "COMMENT_BODY_INVALID")
	}
	return nil
}

func ValidateCommentAnchor(anchor dto.Anchor, body []byte) error {
	invalid := Violation(InvalidRequest, "COMMENT_ANCHOR_INVALID")
	if anchor.SchemaVersion != "anchor-v1" {
		return invalid
	}
	if _, err := CanonicalID(anchor.ContentVersionId); err != nil {
		return invalid
	}
	if anchor.Quote != nil && (!utf8.ValidString(*anchor.Quote) || utf8.RuneCountInString(*anchor.Quote) > 500) {
		return invalid
	}
	if anchor.BlockId == nil {
		if anchor.StartOffset != nil || anchor.EndOffset != nil || anchor.Quote != nil {
			return invalid
		}
		return nil
	}
	if !ValidName(*anchor.BlockId, 96) || (anchor.StartOffset == nil) != (anchor.EndOffset == nil) {
		return invalid
	}
	var root structNode
	if json.Unmarshal(body, &root) != nil {
		return Violation(TemporarilyUnavailable, "COMMENT_CONTENT_INVALID")
	}
	var found *structNode
	var visit func(*structNode)
	visit = func(node *structNode) {
		if id, ok := node.Attrs["id"].(string); ok && id == *anchor.BlockId {
			found = node
		}
		for i := range node.Content {
			visit(&node.Content[i])
		}
	}
	visit(&root)
	if found == nil {
		return invalid
	}
	if anchor.StartOffset == nil {
		if anchor.Quote != nil {
			return invalid
		}
		return nil
	}
	// 浏览器选区偏移按UTF-16单位核验；原子图片/公式占一个占位符，不能按UTF-8字节截断中文。
	var text func(structNode) string
	text = func(node structNode) string {
		if node.Type == "text" {
			return node.Text
		}
		if node.Type == "image" || node.Type == "formula" {
			return "\ufffc"
		}
		if node.Type == "hard_break" {
			return "\n"
		}
		var out strings.Builder
		for _, child := range node.Content {
			out.WriteString(text(child))
		}
		return out.String()
	}
	units := utf16.Encode([]rune(text(*found)))
	start, end := *anchor.StartOffset, *anchor.EndOffset
	if start < 0 || end < start || end > int64(len(units)) {
		return invalid
	}
	boundary := func(index int64) bool {
		return index == 0 || index == int64(len(units)) || !(units[index-1] >= 0xd800 && units[index-1] <= 0xdbff && units[index] >= 0xdc00 && units[index] <= 0xdfff)
	}
	if !boundary(start) || !boundary(end) {
		return invalid
	}
	if anchor.Quote != nil && string(utf16.Decode(units[start:end])) != *anchor.Quote {
		return invalid
	}
	return nil
}

type structNode struct {
	Type    string         `json:"type"`
	Text    string         `json:"text"`
	Attrs   map[string]any `json:"attrs"`
	Content []structNode   `json:"content"`
}
