# services/account/internal/handler/account/organization_handlers.go

阶段02：[功能讲义](../02-account.md) · [分步编写](../write/076-organization-handlers.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：适配HTTP输入、真实身份和结果；调用业务层，避免在这里另写SQL。

主要声明：`Applications`、`NewApplications`、`Apply`、`AcceptInvitation`、`Decide`、`writeApplication`、`List`、`applicationDTO`、`Invitations`、`NewInvitations`、`Create`、`Cancel`、`invitationDTO`、`OrganizationClosures`、`NewOrganizationClosures`、`Close`、`Confirmation`、`ParsePage`、`NextCursor`、`Organizations`、`NewOrganizations`、`OrganizationDTO`、`Get`、`Members`、`OrganizationTransitions`、`NewOrganizationTransitions`、`Role`、`Remove`、`Leave`、`TransferOwner`、`writeTransition`、`OrganizationActions`、`NewOrganizationActions`、`Edit`、`ChangeCreationGrant`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	domain "collaboration/account/internal/domain/account"
	"collaboration/account/internal/handler/httpx"
	logic "collaboration/account/internal/logic/account"
	dto "collaboration/contracts/httpdto"
	"collaboration/platform"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// application.go：保留原实现，按业务合并文件。

type Applications struct {
	authentication *Authentication
	requests       *MutationRequests
	logic          *logic.Applications
	page           ParsePage
	nextCursor     NextCursor
}

func NewApplications(authentication *Authentication, requests *MutationRequests, service *logic.Applications, page ParsePage, cursor NextCursor) *Applications {
	return &Applications{
		authentication: authentication,
		requests:       requests,
		logic:          service,
		page:           page,
		nextCursor:     cursor,
	}
}

func (h *Applications) Apply(w http.ResponseWriter, r *http.Request) {
	organization, ok := httpx.PathID(w, r, "organizationId")
	if !ok {
		return
	}
	var input dto.ApplicationCreate
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	if len([]rune(input.Reason)) > 2000 {
		httpx.Failure(w, 400, "APPLICATION_REASON_INVALID")
		return
	}
	mutation, ok := h.requests.Bind(w, r, input.OperationId, input)
	if !ok {
		return
	}
	result, err := h.logic.Apply(r.Context(), mutation, organization, input)
	writeApplication(w, result, err)
}

func (h *Applications) AcceptInvitation(w http.ResponseWriter, r *http.Request) {
	var input dto.InvitationAccept
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	hash, err := domain.TokenHash(input.InvitationToken)
	if err != nil || len([]rune(input.Reason)) > 2000 {
		httpx.Failure(w, 400, "INVITATION_INPUT_INVALID")
		return
	}
	mutation, ok := h.requests.Bind(w, r, input.OperationId, input)
	if !ok {
		return
	}
	result, err := h.logic.AcceptInvitation(r.Context(), mutation, hash, input)
	writeApplication(w, result, err)
}

func (h *Applications) Decide(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathID(w, r, "applicationId")
	if !ok {
		return
	}
	var input dto.ApplicationReview
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	if (input.Decision != "ACCEPT" && input.Decision != "REJECT") || len([]rune(input.Reason)) > 2000 {
		httpx.Failure(w, 400, "APPLICATION_DECISION_INVALID")
		return
	}
	mutation, ok := h.requests.Bind(w, r, input.OperationId, input)
	if !ok {
		return
	}
	result, err := h.logic.Decide(r.Context(), mutation, id, input)
	writeApplication(w, result, err)
}

func writeApplication(w http.ResponseWriter, result json.RawMessage, err error) {
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	platform.WriteJSON(w, 200, result)
}

func (h *Applications) List(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authentication.Authenticate(w, r, false)
	if !ok {
		return
	}
	organization := ""
	if r.URL.Path != "/v1/account/applications" {
		organization, ok = httpx.PathID(w, r, "organizationId")
		if !ok {
			return
		}
	}
	directory, err := h.logic.OpenDirectory(r.Context(), session.UserID, organization)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	binding := "applications:own:" + session.UserID
	if organization != "" {
		binding = "applications:" + session.UserID + ":" + organization + ":" + directory.Revision()
	}
	limit, last, ok := h.page(w, r, binding)
	if !ok {
		return
	}
	items, more, err := directory.List(r.Context(), last, limit)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	page := dto.ApplicationPage{Items: []dto.Application{}, HasMore: more}
	for _, application := range items {
		page.Items = append(page.Items, applicationDTO(application))
	}
	if more {
		page.NextCursor = h.nextCursor(binding, items[len(items)-1].ID)
	}
	platform.WriteJSON(w, 200, page)
}

func applicationDTO(value domain.OrganizationApplication) dto.Application {
	application := dto.Application{
		Id:                 value.ID,
		OrganizationId:     value.OrganizationID,
		ApplicantId:        value.ApplicantID,
		Reason:             value.Reason,
		State:              value.State,
		Revision:           value.Revision,
		SourceInvitationId: value.SourceInvitationID,
	}
	if value.InvitationValidatedAt != nil {
		formatted := value.InvitationValidatedAt.UTC().Format(time.RFC3339Nano)
		application.InvitationValidatedAt = &formatted
	}
	return application
}

// invitation.go：保留原实现，按业务合并文件。

type Invitations struct {
	authentication *Authentication
	requests       *MutationRequests
	logic          *logic.Invitations
	origin         string
	page           ParsePage
	nextCursor     NextCursor
}

func NewInvitations(authentication *Authentication, requests *MutationRequests, service *logic.Invitations, origin string, page ParsePage, cursor NextCursor) *Invitations {
	return &Invitations{
		authentication: authentication,
		requests:       requests,
		logic:          service,
		origin:         origin,
		page:           page,
		nextCursor:     cursor,
	}
}

func (h *Invitations) Create(w http.ResponseWriter, r *http.Request) {
	organization, ok := httpx.PathID(w, r, "organizationId")
	if !ok {
		return
	}
	var input dto.InvitationCreate
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	if input.ExpiresInHours < 1 || input.ExpiresInHours > 720 {
		httpx.Failure(w, 400, "INVITATION_EXPIRY_INVALID")
		return
	}
	mutation, ok := h.requests.Bind(w, r, input.OperationId, input)
	if !ok {
		return
	}
	raw, err := h.logic.Create(r.Context(), mutation, organization, input)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	var operation dto.Operation
	if json.Unmarshal(raw, &operation) != nil || operation.Result == nil || operation.Result.EntityId == nil {
		httpx.Failure(w, 503, "INVITATION_RESULT_UNAVAILABLE")
		return
	}
	// 通用receipt的成功不授予分享链接读取权；本次输出前重新核验会话及管理员来源。
	session, ok := h.authentication.Authenticate(w, r, false)
	if !ok {
		return
	}
	share, err := h.logic.Share(r.Context(), session.UserID, organization, *operation.Result.EntityId)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	platform.WriteJSON(w, 200, dto.InvitationCreated{
		OperationId: input.OperationId,
		ShareUrl:    h.origin + "/join?invitation_token=" + share.Token,
		Invitation: dto.Invitation{
			Id:             share.ID,
			OrganizationId: organization,
			State:          "OPEN",
			ExpiresAt:      share.Expires.UTC().Format(time.RFC3339),
			Revision:       "1",
		},
	})
}

func (h *Invitations) Cancel(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathID(w, r, "invitationId")
	if !ok {
		return
	}
	var input dto.VersionedAction
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	mutation, ok := h.requests.Bind(w, r, input.OperationId, input)
	if !ok {
		return
	}
	result, err := h.logic.Cancel(r.Context(), mutation, id, input)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	platform.WriteJSON(w, 200, result)
}

func (h *Invitations) List(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authentication.Authenticate(w, r, false)
	if !ok {
		return
	}
	organization, ok := httpx.PathID(w, r, "organizationId")
	if !ok {
		return
	}
	directory, err := h.logic.OpenDirectory(r.Context(), session.UserID, organization)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	binding := "invitations:" + session.UserID + ":" + organization + ":" + directory.Revision()
	limit, last, ok := h.page(w, r, binding)
	if !ok {
		return
	}
	items, more, err := directory.List(r.Context(), last, limit)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	page := dto.InvitationPage{Items: []dto.Invitation{}, HasMore: more}
	for _, invitation := range items {
		page.Items = append(page.Items, invitationDTO(invitation))
	}
	if more {
		page.NextCursor = h.nextCursor(binding, items[len(items)-1].ID)
	}
	platform.WriteJSON(w, 200, page)
}

func invitationDTO(value domain.OrganizationInvitation) dto.Invitation {
	return dto.Invitation{
		Id:             value.ID,
		OrganizationId: value.OrganizationID,
		State:          value.State,
		ExpiresAt:      value.Expires.UTC().Format(time.RFC3339),
		Revision:       value.Revision,
	}
}

// organization_close.go：保留原实现，按业务合并文件。

type OrganizationClosures struct {
	requests *MutationRequests
	logic    *logic.OrganizationClosures
}

func NewOrganizationClosures(requests *MutationRequests, service *logic.OrganizationClosures) *OrganizationClosures {
	return &OrganizationClosures{requests: requests, logic: service}
}
func (h *OrganizationClosures) Close(w http.ResponseWriter, r *http.Request) {
	organization, ok := httpx.PathID(w, r, "organizationId")
	if !ok {
		return
	}
	var input dto.VersionedAction
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	mutation, ok := h.requests.Bind(w, r, input.OperationId, input)
	if !ok {
		return
	}
	result, err := h.logic.Begin(r.Context(), mutation, organization, input.ExpectedRevision)
	writeTransition(w, result, err)
}
func (h *OrganizationClosures) Confirmation(w http.ResponseWriter, r *http.Request) {
	if !platform.HTTPCaller(r, "document") {
		httpx.Failure(w, 403, "CONTROL_IDENTITY_REQUIRED")
		return
	}
	var binding platform.OrganizationClosureBinding
	if !httpx.ReadJSON(w, r, &binding) {
		return
	}
	confirmed, err := h.logic.Confirmed(r.Context(), binding)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	platform.WriteJSON(w, 200, platform.OrganizationClosureConfirmation{Confirmed: confirmed})
}

// organization_read.go：保留原实现，按业务合并文件。

type ParsePage func(http.ResponseWriter, *http.Request, string) (int, string, bool)
type NextCursor func(string, string) *string

type Organizations struct {
	authentication *Authentication
	logic          *logic.Organizations
	page           ParsePage
	nextCursor     NextCursor
}

func NewOrganizations(authentication *Authentication, service *logic.Organizations, page ParsePage, cursor NextCursor) *Organizations {
	return &Organizations{authentication: authentication, logic: service, page: page, nextCursor: cursor}
}

func OrganizationDTO(value domain.Organization) dto.Organization {
	var current *dto.OrganizationMembership
	if m := value.CurrentMember; m != nil {
		current = &dto.OrganizationMembership{UserId: m.UserID, Role: m.Role, State: m.State, Revision: m.Revision}
	}
	return dto.Organization{
		CurrentMember:  current,
		Id:             value.ID,
		Name:           value.Name,
		OwnerId:        value.OwnerID,
		Lifecycle:      value.Lifecycle,
		Revision:       value.Revision,
		MemberRevision: value.MemberRevision,
	}
}

func (h *Organizations) Get(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authentication.Authenticate(w, r, false)
	if !ok {
		return
	}
	id, ok := httpx.PathID(w, r, "organizationId")
	if !ok {
		return
	}
	org, err := h.logic.Get(r.Context(), session.UserID, id)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	platform.WriteJSON(w, 200, OrganizationDTO(org))
}

func (h *Organizations) List(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authentication.Authenticate(w, r, false)
	if !ok {
		return
	}
	binding := "organizations:" + session.UserID
	limit, last, ok := h.page(w, r, binding)
	if !ok {
		return
	}
	items, more, err := h.logic.List(r.Context(), session.UserID, last, limit)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	page := dto.OrganizationPage{Items: []dto.Organization{}, HasMore: more}
	for _, item := range items {
		page.Items = append(page.Items, OrganizationDTO(item))
	}
	if more {
		page.NextCursor = h.nextCursor(binding, items[len(items)-1].ID)
	}
	platform.WriteJSON(w, 200, page)
}

func (h *Organizations) Members(w http.ResponseWriter, r *http.Request) {
	session, ok := h.authentication.Authenticate(w, r, false)
	if !ok {
		return
	}
	id, ok := httpx.PathID(w, r, "organizationId")
	if !ok {
		return
	}
	directory, err := h.logic.OpenMembers(r.Context(), session.UserID, id)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	if len([]rune(query)) > 100 {
		httpx.Failure(w, 400, "DIRECTORY_QUERY_INVALID")
		return
	}
	binding := "members:" + session.UserID + ":" + id + ":" + directory.Revision() + ":" + url.QueryEscape(query)
	limit, last, ok := h.page(w, r, binding)
	if !ok {
		return
	}
	items, more, err := directory.List(r.Context(), last, limit, query)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	page := dto.MemberPage{Items: []dto.Member{}, HasMore: more}
	for _, item := range items {
		page.Items = append(page.Items, dto.Member{
			UserId:                item.UserID,
			DisplayName:           item.DisplayName,
			Role:                  item.Role,
			State:                 item.State,
			Revision:              item.Revision,
			KnowledgeBaseCreation: &item.KnowledgeBaseCreation,
		})
	}
	if more {
		page.NextCursor = h.nextCursor(binding, items[len(items)-1].UserID)
	}
	platform.WriteJSON(w, 200, page)
}

// organization_transition.go：保留原实现，按业务合并文件。

type OrganizationTransitions struct {
	authentication *Authentication
	requests       *MutationRequests
	logic          *logic.OrganizationTransitions
}

func NewOrganizationTransitions(authentication *Authentication, requests *MutationRequests, service *logic.OrganizationTransitions) *OrganizationTransitions {
	return &OrganizationTransitions{authentication: authentication, requests: requests, logic: service}
}

func (h *OrganizationTransitions) Role(w http.ResponseWriter, r *http.Request) {
	organization, ok := httpx.PathID(w, r, "organizationId")
	if !ok {
		return
	}
	user, ok := httpx.PathID(w, r, "userId")
	if !ok {
		return
	}
	var input dto.MemberRole
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	if input.Role != "MEMBER" && input.Role != "ADMIN" {
		httpx.Failure(w, 400, "ROLE_INVALID")
		return
	}
	mutation, ok := h.requests.Bind(w, r, input.OperationId, input)
	if !ok {
		return
	}
	result, err := h.logic.ChangeMember(r.Context(), mutation, organization, user, input.ExpectedRevision, "ORG_ROLE", input.Role)
	writeTransition(w, result, err)
}

func (h *OrganizationTransitions) Remove(w http.ResponseWriter, r *http.Request) {
	organization, ok := httpx.PathID(w, r, "organizationId")
	if !ok {
		return
	}
	user, ok := httpx.PathID(w, r, "userId")
	if !ok {
		return
	}
	var input dto.VersionedAction
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	mutation, ok := h.requests.Bind(w, r, input.OperationId, input)
	if !ok {
		return
	}
	result, err := h.logic.ChangeMember(r.Context(), mutation, organization, user, input.ExpectedRevision, "ORG_REMOVE", "")
	writeTransition(w, result, err)
}

func (h *OrganizationTransitions) Leave(w http.ResponseWriter, r *http.Request) {
	organization, ok := httpx.PathID(w, r, "organizationId")
	if !ok {
		return
	}
	var input dto.VersionedAction
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	// 目标只能取自真实会话，保持原先在通用操作号核验之前的身份检查顺序。
	session, ok := h.authentication.Authenticate(w, r, true)
	if !ok {
		return
	}
	mutation, ok := h.requests.Bind(w, r, input.OperationId, input)
	if !ok {
		return
	}
	result, err := h.logic.ChangeMember(r.Context(), mutation, organization, session.UserID, input.ExpectedRevision, "ORG_LEAVE", "")
	writeTransition(w, result, err)
}

func (h *OrganizationTransitions) TransferOwner(w http.ResponseWriter, r *http.Request) {
	organization, ok := httpx.PathID(w, r, "organizationId")
	if !ok {
		return
	}
	var input dto.OwnerChange
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	if !domain.ValidEntityID(input.TargetUserId) {
		httpx.Failure(w, 400, "ENTITY_ID_INVALID")
		return
	}
	mutation, ok := h.requests.Bind(w, r, input.OperationId, input)
	if !ok {
		return
	}
	result, err := h.logic.TransferOwner(r.Context(), mutation, organization, input)
	writeTransition(w, result, err)
}

func writeTransition(w http.ResponseWriter, result json.RawMessage, err error) {
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	platform.WriteJSON(w, 202, result)
}

// organization_write.go：保留原实现，按业务合并文件。

type OrganizationActions struct {
	requests *MutationRequests
	logic    *logic.OrganizationActions
}

func NewOrganizationActions(requests *MutationRequests, service *logic.OrganizationActions) *OrganizationActions {
	return &OrganizationActions{requests: requests, logic: service}
}

func (h *OrganizationActions) Create(w http.ResponseWriter, r *http.Request) {
	var input dto.NamedCreate
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	name, valid := domain.NormalizeOrganizationName(input.Name)
	if !valid {
		httpx.Failure(w, 400, "ORGANIZATION_NAME_INVALID")
		return
	}
	input.Name = name
	mutation, ok := h.requests.Bind(w, r, input.OperationId, input)
	if !ok {
		return
	}
	result, err := h.logic.Create(r.Context(), mutation, input)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	platform.WriteJSON(w, 201, result)
}

func (h *OrganizationActions) Edit(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathID(w, r, "organizationId")
	if !ok {
		return
	}
	var input dto.OrganizationEdit
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	name, valid := domain.NormalizeOrganizationName(input.Name)
	if !valid {
		httpx.Failure(w, 400, "ORGANIZATION_NAME_INVALID")
		return
	}
	input.Name = name
	mutation, ok := h.requests.Bind(w, r, input.OperationId, input)
	if !ok {
		return
	}
	result, err := h.logic.Edit(r.Context(), mutation, id, input)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	platform.WriteJSON(w, 200, result)
}

func (h *OrganizationActions) ChangeCreationGrant(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathID(w, r, "organizationId")
	if !ok {
		return
	}
	var input dto.CreationGrant
	if !httpx.ReadJSON(w, r, &input) {
		return
	}
	if !domain.ValidEntityID(input.UserId) {
		httpx.Failure(w, 400, "ENTITY_ID_INVALID")
		return
	}
	mutation, ok := h.requests.Bind(w, r, input.OperationId, input)
	if !ok {
		return
	}
	result, err := h.logic.ChangeCreationGrant(r.Context(), mutation, id, input)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	platform.WriteJSON(w, 200, result)
}
````

UTF-8/LF全文SHA-256：`013522840218453876b04c2e35463288994180f827083a3cc7a64c9357a47099`。[参考实现](../../../../services/account/internal/handler/account/organization_handlers.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
