// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	"bytes"
	domain "collaboration/account/internal/domain/account"
	"collaboration/account/internal/handler/httpx"
	logic "collaboration/account/internal/logic/account"
	"collaboration/platform"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// departure_status.go：保留原实现，按业务合并文件。

func (h *Organizations) DepartureStatus(w http.ResponseWriter, r *http.Request) {
	if !platform.HTTPCaller(r, "document") {
		httpx.Failure(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return
	}
	query := r.URL.Query()
	organization, user, operation := query.Get("organization_id"), query.Get("user_id"), query.Get("operation_id")
	if !domain.ValidEntityID(organization) || !domain.ValidEntityID(user) || !domain.ValidOperationID(operation) {
		httpx.Failure(w, 400, "DEPARTURE_BINDING_INVALID")
		return
	}
	state, err := h.logic.DepartureStatus(r.Context(), organization, user, operation, query.Get("epoch"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	platform.WriteJSON(w, 200, map[string]string{"state": state, "operation_id": operation})
}

// notification_administrators.go：保留原实现，按业务合并文件。

func (h *NotificationRecipients) Administrators(w http.ResponseWriter, r *http.Request) {
	// 仅实际Document服务证书可枚举候选人，浏览器Cookie和自报身份不能进入目录。
	if !platform.HTTPCaller(r, "document") {
		httpx.Failure(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return
	}
	var input platform.NotificationAdministratorsRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || decoder.Decode(new(any)) != io.EOF {
		httpx.Failure(w, 400, "NOTIFICATION_ADMINISTRATORS_INVALID")
		return
	}
	result, err := h.logic.Administrators(r.Context(), input)
	if err != nil {
		if errors.Is(err, domain.ErrNotificationAdministratorsInput) {
			httpx.Failure(w, 400, "NOTIFICATION_ADMINISTRATORS_INVALID")
		} else {
			httpx.Failure(w, 503, "NOTIFICATION_ADMINISTRATORS_UNAVAILABLE")
		}
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	platform.WriteJSON(w, 200, result)
}

// notification_events.go：保留原实现，按业务合并文件。

type NotificationEvents struct{ logic *logic.NotificationEvents }

func NewNotificationEvents(service *logic.NotificationEvents) *NotificationEvents {
	return &NotificationEvents{logic: service}
}

func notificationEventInput(w http.ResponseWriter, r *http.Request, input any) bool {
	// 证书角色是唯一身份来源；即便携带有效用户Cookie也不能访问内部投递队列。
	if !platform.HTTPCaller(r, "document") {
		httpx.Failure(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	decoder.DisallowUnknownFields()
	var raw json.RawMessage
	if decoder.Decode(&raw) != nil || len(raw) == 0 || raw[0] != '{' || decoder.Decode(new(any)) != io.EOF {
		httpx.Failure(w, 400, "NOTIFICATION_EVENTS_INVALID")
		return false
	}
	// 第二次严格解码保留字段检查，同时排除JSON null当作空对象的隐式接受。
	strict := json.NewDecoder(bytes.NewReader(raw))
	strict.DisallowUnknownFields()
	if strict.Decode(input) != nil {
		httpx.Failure(w, 400, "NOTIFICATION_EVENTS_INVALID")
		return false
	}
	w.Header().Set("Cache-Control", "no-store")
	return true
}

func (h *NotificationEvents) Events(w http.ResponseWriter, r *http.Request) {
	if !notificationEventInput(w, r, &struct{}{}) {
		return
	}
	result, err := h.logic.Events(r.Context())
	if err != nil {
		httpx.Failure(w, 503, "NOTIFICATION_EVENTS_UNAVAILABLE")
		return
	}
	platform.WriteJSON(w, 200, result)
}

func (h *NotificationEvents) Acknowledge(w http.ResponseWriter, r *http.Request) {
	var input platform.AccountNotificationAckRequest
	if !notificationEventInput(w, r, &input) {
		return
	}
	if err := h.logic.Acknowledge(r.Context(), input); err != nil {
		if errors.Is(err, logic.ErrNotificationEventsInput) {
			httpx.Failure(w, 400, "NOTIFICATION_EVENTS_INVALID")
		} else {
			httpx.Failure(w, 503, "NOTIFICATION_EVENTS_UNAVAILABLE")
		}
		return
	}
	platform.WriteJSON(w, 200, platform.AccountNotificationAckResponse{Acknowledged: true})
}

// notification_recipients.go：保留原实现，按业务合并文件。

type NotificationRecipients struct{ logic *logic.NotificationRecipients }

func NewNotificationRecipients(service *logic.NotificationRecipients) *NotificationRecipients {
	return &NotificationRecipients{logic: service}
}

func (h *NotificationRecipients) Check(w http.ResponseWriter, r *http.Request) {
	// 只接受实际document mTLS证书；不使用Cookie，不向浏览器开放后台成员枚举。
	if !platform.HTTPCaller(r, "document") {
		httpx.Failure(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return
	}
	var input platform.NotificationRecipientsRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		httpx.Failure(w, 400, "NOTIFICATION_RECIPIENTS_INVALID")
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		httpx.Failure(w, 400, "NOTIFICATION_RECIPIENTS_INVALID")
		return
	}
	result, err := h.logic.Check(r.Context(), input)
	if err != nil {
		if errors.Is(err, domain.ErrNotificationRecipientsInput) {
			httpx.Failure(w, 400, "NOTIFICATION_RECIPIENTS_INVALID")
		} else {
			httpx.Failure(w, 503, "NOTIFICATION_RECIPIENTS_UNAVAILABLE")
		}
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	platform.WriteJSON(w, 200, result)
}
