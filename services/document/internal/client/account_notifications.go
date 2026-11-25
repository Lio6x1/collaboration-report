package client

import (
	"bytes"
	"collaboration/platform"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
)

var accountNotificationEventPattern = regexp.MustCompile(`^account-owner:[a-f0-9]{64}$`)

// Events只通过已有document服务mTLS客户端读取Account原始事实，不构造用户会话。
// 没有高水位：迟提交的小ID事件仍会留在Account未确认队列中，供下一轮获取。
func (c *NotificationRecipients) Events(ctx context.Context) ([]platform.AccountNotificationEvent, error) {
	var value platform.AccountNotificationEventsResponse
	if err := c.accountNotificationPost(ctx, "/infra/notification/events", struct{}{}, &value); err != nil {
		return nil, err
	}
	if value.Events == nil || len(value.Events) > 100 {
		return nil, errors.New("ACCOUNT_NOTIFICATION_RESPONSE_INVALID")
	}
	seen := make(map[uint64]bool, len(value.Events))
	for _, event := range value.Events {
		// uint64解码已拒绝符号、前导零、小数和溢出；零值及重复原始行号仍须显式拒绝。
		if event.ID == 0 || event.RecipientID == 0 || event.OrganizationID == 0 || seen[event.ID] || event.Kind != "ORG_OWNER_TRANSFER" || !accountNotificationEventPattern.MatchString(event.EventID) {
			return nil, errors.New("ACCOUNT_NOTIFICATION_RESPONSE_INVALID")
		}
		seen[event.ID] = true
	}
	return value.Events, nil
}

func (c *NotificationRecipients) AckEvents(ctx context.Context, ids []uint64) error {
	if len(ids) == 0 || len(ids) > 100 {
		return errors.New("ACCOUNT_NOTIFICATION_ACK_INVALID")
	}
	seen := make(map[uint64]bool, len(ids))
	for _, id := range ids {
		if id == 0 || seen[id] {
			return errors.New("ACCOUNT_NOTIFICATION_ACK_INVALID")
		}
		seen[id] = true
	}
	var value platform.AccountNotificationAckResponse
	if err := c.accountNotificationPost(ctx, "/infra/notification/events/ack", platform.AccountNotificationAckRequest{IDs: ids}, &value); err != nil {
		return err
	}
	if !value.Acknowledged {
		return errors.New("ACCOUNT_NOTIFICATION_ACK_INVALID")
	}
	return nil
}

func (c *NotificationRecipients) accountNotificationPost(ctx context.Context, path string, input, output any) error {
	raw, err := json.Marshal(input)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return errors.New("ACCOUNT_NOTIFICATION_UNAVAILABLE")
	}
	// 多读一个字节检测超限，不能让LimitReader的人工EOF掩盖响应尾部。
	const limit = 128 << 10
	body, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return err
	}
	if len(body) > limit {
		return errors.New("ACCOUNT_NOTIFICATION_RESPONSE_INVALID")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if decoder.Decode(output) != nil || decoder.Decode(new(any)) != io.EOF {
		return errors.New("ACCOUNT_NOTIFICATION_RESPONSE_INVALID")
	}
	return nil
}
