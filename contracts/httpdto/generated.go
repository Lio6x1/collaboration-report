// Code generated from fixed OpenAPI; DO NOT EDIT.
package httpdto
import "encoding/json"
type ID string
type Revision string
type OperationID string
type Error struct {
Code string `json:"code"`
Message string `json:"message"`
TraceId string `json:"trace_id"`
OperationId *string `json:"operation_id,omitempty"`
RetryAction string `json:"retry_action"`
}
type Action struct {
OperationId string `json:"operation_id"`
}
type VersionedAction struct {
OperationId string `json:"operation_id"`
ExpectedRevision string `json:"expected_revision"`
}
type NamedCreate struct {
OperationId string `json:"operation_id"`
Name string `json:"name"`
}
type Operation struct {
OperationId string `json:"operation_id"`
State string `json:"state"`
Result *struct {
RequestId *string `json:"request_id,omitempty"`
RoundId *string `json:"round_id,omitempty"`
ContentVersionId *string `json:"content_version_id,omitempty"`
MergeRecordId *string `json:"merge_record_id,omitempty"`
ManifestId *string `json:"manifest_id,omitempty"`
ReleaseId *string `json:"release_id,omitempty"`
ResourceId *string `json:"resource_id,omitempty"`
EntityId *string `json:"entity_id,omitempty"`
MainSyncState *string `json:"main_sync_state,omitempty"`
AuthorizationChangeId *string `json:"authorization_change_id,omitempty"`
ExportJobId *string `json:"export_job_id,omitempty"`
ConflictContextRef *string `json:"conflict_context_ref,omitempty"`
AttemptId *string `json:"attempt_id,omitempty"`
} `json:"result,omitempty"`
CurrentState *struct {
ReviewState *string `json:"review_state,omitempty"`
CurrentRoundId *string `json:"current_round_id,omitempty"`
Revision *string `json:"revision,omitempty"`
PublicationState *string `json:"publication_state,omitempty"`
ApplicationState *string `json:"application_state,omitempty"`
} `json:"current_state,omitempty"`
Error *Error `json:"error,omitempty"`
PollUrl *string `json:"poll_url,omitempty"`
}
type Login struct {
Email string `json:"email"`
Password string `json:"password"`
}
type LoginResult struct {
UserId string `json:"user_id"`
DisplayName string `json:"display_name"`
CsrfToken string `json:"csrf_token"`
ExpiresAt string `json:"expires_at"`
}
type ChallengeCreate struct {
Purpose string `json:"purpose"`
Email string `json:"email"`
}
type Challenge struct {
ChallengeId string `json:"challenge_id"`
ExpiresAt string `json:"expires_at"`
RetryAfterSeconds int64 `json:"retry_after_seconds"`
}
type Signup struct {
OperationId string `json:"operation_id"`
ChallengeId string `json:"challenge_id"`
Code string `json:"code"`
Email string `json:"email"`
Password string `json:"password"`
DisplayName string `json:"display_name"`
}
type ResetPassword struct {
OperationId string `json:"operation_id"`
ChallengeId string `json:"challenge_id"`
Code string `json:"code"`
NewPassword string `json:"new_password"`
}
type PasswordChange struct {
OperationId string `json:"operation_id"`
ExpectedCredentialEpoch string `json:"expected_credential_epoch"`
CurrentPassword string `json:"current_password"`
NewPassword string `json:"new_password"`
}
type EmailChange struct {
OperationId string `json:"operation_id"`
ExpectedCredentialEpoch string `json:"expected_credential_epoch"`
ChallengeId string `json:"challenge_id"`
Code string `json:"code"`
NewEmail string `json:"new_email"`
}
type User struct {
Id string `json:"id"`
Email *string `json:"email,omitempty"`
DisplayName string `json:"display_name"`
State string `json:"state"`
CredentialEpoch string `json:"credential_epoch"`
IdentityEpoch string `json:"identity_epoch"`
}
type Organization struct {
Id string `json:"id"`
Name string `json:"name"`
OwnerId string `json:"owner_id"`
Lifecycle string `json:"lifecycle"`
Revision string `json:"revision"`
MemberRevision string `json:"member_revision"`
CurrentMember *OrganizationMembership `json:"current_member,omitempty"`
}
type Member struct {
UserId string `json:"user_id"`
DisplayName string `json:"display_name"`
Role string `json:"role"`
State string `json:"state"`
Revision string `json:"revision"`
KnowledgeBaseCreation *string `json:"knowledge_base_creation,omitempty"`
}
type OrganizationEdit struct {
OperationId string `json:"operation_id"`
ExpectedRevision string `json:"expected_revision"`
Name string `json:"name"`
}
type MemberRole struct {
OperationId string `json:"operation_id"`
ExpectedRevision string `json:"expected_revision"`
Role string `json:"role"`
}
type OwnerChange struct {
OperationId string `json:"operation_id"`
ExpectedRevision string `json:"expected_revision"`
TargetUserId string `json:"target_user_id"`
}
type InvitationCreate struct {
OperationId string `json:"operation_id"`
ExpectedMemberRevision string `json:"expected_member_revision"`
ExpiresInHours int64 `json:"expires_in_hours"`
}
type InvitationAccept struct {
OperationId string `json:"operation_id"`
InvitationToken string `json:"invitation_token"`
Reason string `json:"reason"`
}
type ApplicationCreate struct {
OperationId string `json:"operation_id"`
Reason string `json:"reason"`
}
type ApplicationReview struct {
OperationId string `json:"operation_id"`
ExpectedRevision string `json:"expected_revision"`
Decision string `json:"decision"`
Reason string `json:"reason"`
}
type Invitation struct {
Id string `json:"id"`
OrganizationId string `json:"organization_id"`
State string `json:"state"`
ExpiresAt string `json:"expires_at"`
Revision string `json:"revision"`
}
type Application struct {
Id string `json:"id"`
OrganizationId string `json:"organization_id"`
ApplicantId string `json:"applicant_id"`
Reason string `json:"reason"`
State string `json:"state"`
Revision string `json:"revision"`
SourceInvitationId *string `json:"source_invitation_id,omitempty"`
InvitationValidatedAt *string `json:"invitation_validated_at,omitempty"`
}
type KnowledgeBase struct {
Id string `json:"id"`
SpaceId string `json:"space_id"`
Name string `json:"name"`
Lifecycle string `json:"lifecycle"`
AclRevision string `json:"acl_revision"`
VisibilityPolicy string `json:"visibility_policy"`
VisibilityRevision string `json:"visibility_revision"`
}
type KBCreate struct {
OperationId string `json:"operation_id"`
Name string `json:"name"`
VisibilityPolicy *string `json:"visibility_policy,omitempty"`
}
type KBRole struct {
OperationId string `json:"operation_id"`
ExpectedAclRevision string `json:"expected_acl_revision"`
UserId string `json:"user_id"`
Role string `json:"role"`
}
type CreationGrant struct {
OperationId string `json:"operation_id"`
ExpectedRevision string `json:"expected_revision"`
UserId string `json:"user_id"`
Allowed bool `json:"allowed"`
}
type DocumentCreate struct {
OperationId string `json:"operation_id"`
SpaceId string `json:"space_id"`
KnowledgeBaseId *string `json:"knowledge_base_id,omitempty"`
Title string `json:"title"`
VisibilityPolicy *string `json:"visibility_policy,omitempty"`
}
type Document struct {
Id string `json:"id"`
Title string `json:"title"`
SpaceId string `json:"space_id"`
KnowledgeBaseId *string `json:"knowledge_base_id,omitempty"`
ResponsibleUserId *string `json:"responsible_user_id,omitempty"`
Lifecycle string `json:"lifecycle"`
MainLocked bool `json:"main_locked"`
CurrentStructureId string `json:"current_structure_id"`
CurrentStyleId string `json:"current_style_id"`
CurrentManifestId string `json:"current_manifest_id"`
CurrentReleaseId *string `json:"current_release_id,omitempty"`
PublicationState string `json:"publication_state"`
ControlRevision string `json:"control_revision"`
AclRevision string `json:"acl_revision"`
PublicationEpoch string `json:"publication_epoch"`
VisibilityPolicy string `json:"visibility_policy"`
VisibilityRevision string `json:"visibility_revision"`
}
type DocumentEdit struct {
OperationId string `json:"operation_id"`
ExpectedControlRevision string `json:"expected_control_revision"`
Title string `json:"title"`
}
type Grant struct {
UserId string `json:"user_id"`
ScopeKind string `json:"scope_kind"`
ScopeId string `json:"scope_id"`
Capability string `json:"capability"`
}
type AuthorizationChange struct {
OperationId string `json:"operation_id"`
ExpectedAclRevision string `json:"expected_acl_revision"`
Changes []struct {
Grant Grant `json:"grant"`
Action string `json:"action"`
} `json:"changes"`
}
type AuthorizationResult struct {
OperationId string `json:"operation_id"`
State string `json:"state"`
ScopeKey string `json:"scope_key"`
Epoch string `json:"epoch"`
MissingParticipants []string `json:"missing_participants"`
}
type EditSessionCreate struct {
OperationId string `json:"operation_id"`
WorkspaceKind string `json:"workspace_kind"`
ClientSessionId string `json:"client_session_id"`
ExpectedStructureId string `json:"expected_structure_id"`
}
type EditSession struct {
Ticket string `json:"ticket"`
TicketExpiresAt string `json:"ticket_expires_at"`
WssUrl string `json:"wss_url"`
RoomId string `json:"room_id"`
WorkspaceEpoch string `json:"workspace_epoch"`
BranchId *string `json:"branch_id,omitempty"`
LineageId *string `json:"lineage_id,omitempty"`
BaseManifestId *string `json:"base_manifest_id,omitempty"`
BaseVersionId *string `json:"base_version_id,omitempty"`
}
type Submission json.RawMessage
type Withdrawal struct {
OperationId string `json:"operation_id"`
ExpectedRoundId string `json:"expected_round_id"`
Reason *string `json:"reason,omitempty"`
}
type RequestChanges struct {
OperationId string `json:"operation_id"`
RoundId string `json:"round_id"`
ContentVersionId string `json:"content_version_id"`
Type string `json:"type"`
Reason string `json:"reason"`
}
type ResolveChanges struct {
OperationId string `json:"operation_id"`
ExpectedRoundId string `json:"expected_round_id"`
ExpectedChangeRevision string `json:"expected_change_revision"`
Reason string `json:"reason"`
}
type ApproveMerge struct {
OperationId string `json:"operation_id"`
RoundId string `json:"round_id"`
ContentVersionId string `json:"content_version_id"`
OwnChangesToResolve []struct {
Id string `json:"id"`
Revision string `json:"revision"`
} `json:"own_changes_to_resolve"`
}
type ReviewRequest struct {
Id string `json:"id"`
DocumentId string `json:"document_id"`
UnitId string `json:"unit_id"`
BranchId string `json:"branch_id"`
Status string `json:"status"`
CurrentRoundId string `json:"current_round_id"`
Revision string `json:"revision"`
ArchivedAt *string `json:"archived_at"`
Actions []string `json:"actions"`
BlockingChangeCount *int64 `json:"blocking_change_count,omitempty"`
CurrentRound *ReviewRound `json:"current_round,omitempty"`
Summary *ReviewRequestSummary `json:"summary,omitempty"`
}
type ReviewRound struct {
Id string `json:"id"`
RequestId string `json:"request_id"`
RoundNo string `json:"round_no"`
ContentVersionId string `json:"content_version_id"`
BaseManifestId string `json:"base_manifest_id"`
BaseVersionId string `json:"base_version_id"`
StructureId string `json:"structure_id"`
LineageId string `json:"lineage_id"`
SubmitterId string `json:"submitter_id"`
Content FixedContent `json:"content"`
Structure StructureRevision `json:"structure"`
Style StyleRevision `json:"style"`
}
type FixedContent struct {
Id string `json:"id"`
DocumentId string `json:"document_id"`
UnitId string `json:"unit_id"`
StructureId string `json:"structure_id"`
SchemaVersion string `json:"schema_version"`
CanonicalizationVersion string `json:"canonicalization_version"`
BodyJson map[string]json.RawMessage `json:"body_json"`
BodyHash string `json:"body_hash"`
ByteSize string `json:"byte_size"`
ResourceIds []string `json:"resource_ids"`
Provenance map[string]json.RawMessage `json:"provenance"`
}
type MainView struct {
ReadViewId string `json:"read_view_id"`
ExpiresAt string `json:"expires_at"`
Document Document `json:"document"`
ManifestId string `json:"manifest_id"`
StructureId string `json:"structure_id"`
StyleId string `json:"style_id"`
Units []struct {
UnitId string `json:"unit_id"`
ContentVersionId string `json:"content_version_id"`
} `json:"units"`
Structure StructureRevision `json:"structure"`
Style StyleRevision `json:"style"`
}
type EditableUnit struct {
UnitId string `json:"unit_id"`
Path string `json:"path"`
BranchId *string `json:"branch_id,omitempty"`
State string `json:"state"`
WorkspaceEpoch string `json:"workspace_epoch"`
LineageId *string `json:"lineage_id,omitempty"`
}
type ReleaseCreate struct {
OperationId string `json:"operation_id"`
ExpectedPublicationRevision string `json:"expected_publication_revision"`
ExpectedControlRevision string `json:"expected_control_revision"`
}
type Publication struct {
OperationId string `json:"operation_id"`
ExpectedPublicationRevision string `json:"expected_publication_revision"`
Action string `json:"action"`
ReleaseId *string `json:"release_id,omitempty"`
}
type Release struct {
Id string `json:"id"`
DocumentId string `json:"document_id"`
ManifestId string `json:"manifest_id"`
PublisherId string `json:"publisher_id"`
CreatedAt string `json:"created_at"`
}
type PublicDocument struct {
DocumentId string `json:"document_id"`
ReleaseId string `json:"release_id"`
PublicationEpoch string `json:"publication_epoch"`
Title string `json:"title"`
ManifestId string `json:"manifest_id"`
Units []FixedContent `json:"units"`
Structure StructureRevision `json:"structure"`
Style StyleRevision `json:"style"`
}
type MainLock struct {
OperationId string `json:"operation_id"`
ExpectedControlRevision string `json:"expected_control_revision"`
Locked bool `json:"locked"`
}
type StyleChange struct {
OperationId string `json:"operation_id"`
ExpectedStyleId string `json:"expected_style_id"`
ExpectedControlRevision string `json:"expected_control_revision"`
Style struct {
BodyFont string `json:"body_font"`
WesternFont string `json:"western_font"`
BodySizePt int64 `json:"body_size_pt"`
HeadingSizesPt []int64 `json:"heading_sizes_pt"`
Page string `json:"page"`
MarginCm string `json:"margin_cm"`
TitleFont *string `json:"title_font,omitempty"`
TitleSizePt *int64 `json:"title_size_pt,omitempty"`
} `json:"style"`
}
type ChapterCreate struct {
OperationId string `json:"operation_id"`
ExpectedStructureId string `json:"expected_structure_id"`
Title string `json:"title"`
AfterChapterId *string `json:"after_chapter_id,omitempty"`
}
type StructureChange struct {
OperationId string `json:"operation_id"`
ExpectedStructureId string `json:"expected_structure_id"`
ExpectedControlRevision string `json:"expected_control_revision"`
Changes []struct {
Type string `json:"type"`
ChapterId string `json:"chapter_id"`
Title *string `json:"title,omitempty"`
AfterChapterId *string `json:"after_chapter_id,omitempty"`
PreviewCutRef *string `json:"preview_cut_ref,omitempty"`
} `json:"changes"`
}
type DiscardDraft struct {
OperationId string `json:"operation_id"`
ExpectedBranchId string `json:"expected_branch_id"`
ExpectedRoundId *string `json:"expected_round_id,omitempty"`
ExpectedStructureId string `json:"expected_structure_id"`
PreviewCutRef string `json:"preview_cut_ref"`
ChangesToResolve []struct {
Id string `json:"id"`
Revision string `json:"revision"`
Reason string `json:"reason"`
} `json:"changes_to_resolve"`
}
type PullMain struct {
OperationId string `json:"operation_id"`
ExpectedBranchId string `json:"expected_branch_id"`
ExpectedLineageId string `json:"expected_lineage_id"`
ExpectedStructureId string `json:"expected_structure_id"`
}
type ReverseChange struct {
OperationId string `json:"operation_id"`
ExpectedManifestId string `json:"expected_manifest_id"`
MergeRecordId string `json:"merge_record_id"`
Reason string `json:"reason"`
}
type ResourceUpload struct {
OperationId string `json:"operation_id"`
ExpectedQuotaRevision string `json:"expected_quota_revision"`
ByteSize string `json:"byte_size"`
MediaType string `json:"media_type"`
Checksum string `json:"checksum"`
}
type UploadHandle struct {
ResourceId string `json:"resource_id"`
UploadUrl string `json:"upload_url"`
ExpiresAt string `json:"expires_at"`
}
type UploadComplete struct {
OperationId string `json:"operation_id"`
Checksum string `json:"checksum"`
ByteSize string `json:"byte_size"`
}
type Resource struct {
Id string `json:"id"`
State string `json:"state"`
ByteSize string `json:"byte_size"`
Checksum *string `json:"checksum,omitempty"`
MediaType string `json:"media_type"`
Revision string `json:"revision"`
}
type ExportCreate json.RawMessage
type ExportJob struct {
Id string `json:"id"`
DocumentId string `json:"document_id"`
ManifestId string `json:"manifest_id"`
ReleaseId *string `json:"release_id,omitempty"`
Format string `json:"format"`
State string `json:"state"`
LastError *Error `json:"last_error,omitempty"`
}
type Anchor struct {
SchemaVersion string `json:"schema_version"`
ContentVersionId string `json:"content_version_id"`
BlockId *string `json:"block_id,omitempty"`
StartOffset *int64 `json:"start_offset,omitempty"`
EndOffset *int64 `json:"end_offset,omitempty"`
Quote *string `json:"quote,omitempty"`
}
type CommentThreadCreate struct {
OperationId string `json:"operation_id"`
ContextKind string `json:"context_kind"`
ContextId string `json:"context_id"`
Anchor Anchor `json:"anchor"`
Body string `json:"body"`
Mentions []string `json:"mentions"`
}
type CommentThread struct {
Id string `json:"id"`
DocumentId string `json:"document_id"`
ContextKind string `json:"context_kind"`
ContextId string `json:"context_id"`
Anchor Anchor `json:"anchor"`
AuthorId string `json:"author_id"`
Status string `json:"status"`
Revision string `json:"revision"`
}
type CommentCreate struct {
OperationId string `json:"operation_id"`
Body string `json:"body"`
ReplyToId *string `json:"reply_to_id,omitempty"`
Mentions []string `json:"mentions"`
}
type Comment struct {
Id string `json:"id"`
ThreadId string `json:"thread_id"`
AuthorId string `json:"author_id"`
Body string `json:"body"`
Revision string `json:"revision"`
}
type CommentEdit struct {
OperationId string `json:"operation_id"`
ExpectedRevision string `json:"expected_revision"`
Body string `json:"body"`
}
type ThreadState struct {
OperationId string `json:"operation_id"`
ExpectedRevision string `json:"expected_revision"`
State string `json:"state"`
}
type Notification struct {
Id string `json:"id"`
EventId string `json:"event_id"`
Kind string `json:"kind"`
TargetId *string `json:"target_id,omitempty"`
Title string `json:"title"`
ReadAt *string `json:"read_at"`
}
type Quota struct {
SpaceId string `json:"space_id"`
LimitBytes string `json:"limit_bytes"`
UsedBytes string `json:"used_bytes"`
ReservedBytes string `json:"reserved_bytes"`
Revision string `json:"revision"`
}
type TrashAction struct {
OperationId string `json:"operation_id"`
ExpectedControlRevision string `json:"expected_control_revision"`
Reason *string `json:"reason,omitempty"`
}
type PurgeAction struct {
OperationId string `json:"operation_id"`
ExpectedControlRevision string `json:"expected_control_revision"`
Confirmed bool `json:"confirmed"`
}
type AccountClose struct {
OperationId string `json:"operation_id"`
ExpectedIdentityEpoch string `json:"expected_identity_epoch"`
CurrentPassword string `json:"current_password"`
}
type AccountClosePlan struct {
OperationId string `json:"operation_id"`
State string `json:"state"`
RequiredSuccessions []struct {
Kind string `json:"kind"`
TargetId string `json:"target_id"`
} `json:"required_successions"`
RetainedReleaseIds []string `json:"retained_release_ids"`
}
type SearchResult struct {
DocumentId string `json:"document_id"`
ReleaseId string `json:"release_id"`
Title string `json:"title"`
Snippet string `json:"snippet"`
ChapterId *string `json:"chapter_id,omitempty"`
}
type OrganizationPage struct {
Items []Organization `json:"items"`
NextCursor *string `json:"next_cursor"`
HasMore bool `json:"has_more"`
}
type MemberPage struct {
Items []Member `json:"items"`
NextCursor *string `json:"next_cursor"`
HasMore bool `json:"has_more"`
}
type InvitationPage struct {
Items []Invitation `json:"items"`
NextCursor *string `json:"next_cursor"`
HasMore bool `json:"has_more"`
}
type ApplicationPage struct {
Items []Application `json:"items"`
NextCursor *string `json:"next_cursor"`
HasMore bool `json:"has_more"`
}
type KnowledgeBasePage struct {
Items []KnowledgeBase `json:"items"`
NextCursor *string `json:"next_cursor"`
HasMore bool `json:"has_more"`
}
type DocumentPage struct {
Items []Document `json:"items"`
NextCursor *string `json:"next_cursor"`
HasMore bool `json:"has_more"`
}
type ReviewRequestPage struct {
Items []ReviewRequest `json:"items"`
NextCursor *string `json:"next_cursor"`
HasMore bool `json:"has_more"`
}
type ReviewRoundPage struct {
Items []ReviewRound `json:"items"`
NextCursor *string `json:"next_cursor"`
HasMore bool `json:"has_more"`
}
type EditableUnitPage struct {
Items []EditableUnit `json:"items"`
NextCursor *string `json:"next_cursor"`
HasMore bool `json:"has_more"`
}
type ReleasePage struct {
Items []Release `json:"items"`
NextCursor *string `json:"next_cursor"`
HasMore bool `json:"has_more"`
}
type CommentThreadPage struct {
Items []CommentThread `json:"items"`
NextCursor *string `json:"next_cursor"`
HasMore bool `json:"has_more"`
}
type CommentPage struct {
Items []Comment `json:"items"`
NextCursor *string `json:"next_cursor"`
HasMore bool `json:"has_more"`
}
type NotificationPage struct {
Items []Notification `json:"items"`
NextCursor *string `json:"next_cursor"`
HasMore bool `json:"has_more"`
}
type SearchResultPage struct {
Items []SearchResult `json:"items"`
NextCursor *string `json:"next_cursor"`
HasMore bool `json:"has_more"`
}
type VisibilityChange struct {
OperationId string `json:"operation_id"`
ExpectedVisibilityRevision string `json:"expected_visibility_revision"`
Policy string `json:"policy"`
}
type Space struct {
Id string `json:"id"`
Kind string `json:"kind"`
OrganizationId *string `json:"organization_id,omitempty"`
OwnerUserId *string `json:"owner_user_id,omitempty"`
Lifecycle string `json:"lifecycle"`
Actions []string `json:"actions"`
}
type SpacePage struct {
Items []Space `json:"items"`
NextCursor *string `json:"next_cursor"`
HasMore bool `json:"has_more"`
}
type StructureEntry struct {
NodeId string `json:"node_id"`
ParentNodeId *string `json:"parent_node_id,omitempty"`
ChapterId string `json:"chapter_id"`
UnitId *string `json:"unit_id,omitempty"`
Kind string `json:"kind"`
Title string `json:"title"`
SortOrder int64 `json:"sort_order"`
SplitLevel2 bool `json:"split_level2"`
}
type StructureRevision struct {
Id string `json:"id"`
DocumentId string `json:"document_id"`
SchemaVersion string `json:"schema_version"`
Entries []StructureEntry `json:"entries"`
}
type StyleRevision struct {
Id string `json:"id"`
DocumentId string `json:"document_id"`
SchemaVersion string `json:"schema_version"`
Style struct {
BodyFont string `json:"body_font"`
WesternFont string `json:"western_font"`
BodySizePt int64 `json:"body_size_pt"`
HeadingSizesPt []int64 `json:"heading_sizes_pt"`
Page string `json:"page"`
MarginCm string `json:"margin_cm"`
TitleFont *string `json:"title_font,omitempty"`
TitleSizePt *int64 `json:"title_size_pt,omitempty"`
} `json:"style"`
}
type DiscardPreviewCreate struct {
OperationId string `json:"operation_id"`
ExpectedBranchId string `json:"expected_branch_id"`
ExpectedStructureId string `json:"expected_structure_id"`
}
type DiscardPreview struct {
PreviewCutRef string `json:"preview_cut_ref"`
DocumentId string `json:"document_id"`
UnitId string `json:"unit_id"`
BranchId string `json:"branch_id"`
WorkspaceEpoch string `json:"workspace_epoch"`
LineageId string `json:"lineage_id"`
BaseManifestId string `json:"base_manifest_id"`
DraftContentVersionId string `json:"draft_content_version_id"`
MainManifestId string `json:"main_manifest_id"`
StructureId string `json:"structure_id"`
CutDigest string `json:"cut_digest"`
ExpiresAt string `json:"expires_at"`
BlockingChanges []struct {
Id string `json:"id"`
Revision string `json:"revision"`
AuthorId string `json:"author_id"`
Reason string `json:"reason"`
} `json:"blocking_changes"`
}
type StructurePreviewCreate struct {
OperationId string `json:"operation_id"`
ExpectedStructureId string `json:"expected_structure_id"`
ExpectedControlRevision string `json:"expected_control_revision"`
Changes []struct {
Type string `json:"type"`
ChapterId string `json:"chapter_id"`
Title *string `json:"title,omitempty"`
AfterChapterId *string `json:"after_chapter_id,omitempty"`
PreviewCutRef *string `json:"preview_cut_ref,omitempty"`
} `json:"changes"`
}
type StructurePreview struct {
PreviewCutRef string `json:"preview_cut_ref"`
StructureId string `json:"structure_id"`
CutDigest string `json:"cut_digest"`
ExpiresAt string `json:"expires_at"`
BlockingUnits []struct {
UnitId string `json:"unit_id"`
Reason string `json:"reason"`
} `json:"blocking_units"`
}
type ChangeRequest struct {
Id string `json:"id"`
RequestId string `json:"request_id"`
RoundId string `json:"round_id"`
ContentVersionId string `json:"content_version_id"`
AuthorId string `json:"author_id"`
Status string `json:"status"`
Reason string `json:"reason"`
ResolutionReason *string `json:"resolution_reason,omitempty"`
Revision string `json:"revision"`
CreatedAt string `json:"created_at"`
}
type ChangeRequestPage struct {
Items []ChangeRequest `json:"items"`
NextCursor *string `json:"next_cursor"`
HasMore bool `json:"has_more"`
}
type ReviewAction struct {
Id string `json:"id"`
RequestId string `json:"request_id"`
RoundId string `json:"round_id"`
ActorId string `json:"actor_id"`
Type string `json:"type"`
ContentVersionId *string `json:"content_version_id,omitempty"`
Reason *string `json:"reason,omitempty"`
ChangeRequestId *string `json:"change_request_id,omitempty"`
MergeRecordId *string `json:"merge_record_id,omitempty"`
CreatedAt string `json:"created_at"`
}
type ReviewActionPage struct {
Items []ReviewAction `json:"items"`
NextCursor *string `json:"next_cursor"`
HasMore bool `json:"has_more"`
}
type Conflict struct {
Id string `json:"id"`
BlockId string `json:"block_id"`
NodePath string `json:"node_path"`
Kind string `json:"kind"`
Reason string `json:"reason"`
}
type ConflictContext struct {
ConflictRef string `json:"conflict_ref"`
OperationId string `json:"operation_id"`
AttemptId string `json:"attempt_id"`
DocumentId string `json:"document_id"`
UnitId string `json:"unit_id"`
RequestId string `json:"request_id"`
RoundId string `json:"round_id"`
Base FixedContent `json:"base"`
Submitted FixedContent `json:"submitted"`
Main FixedContent `json:"main"`
Structure StructureRevision `json:"structure"`
Style StyleRevision `json:"style"`
Conflicts []Conflict `json:"conflicts"`
InputDigest string `json:"input_digest"`
ExpiresAt string `json:"expires_at"`
}
type ConflictResolution struct {
OperationId string `json:"operation_id"`
ExpectedAttemptId string `json:"expected_attempt_id"`
ExpectedInputDigest string `json:"expected_input_digest"`
ExpectedRoundId string `json:"expected_round_id"`
ResolutionContent map[string]json.RawMessage `json:"resolution_content"`
ResolvedConflictIds []string `json:"resolved_conflict_ids"`
}
type InvitationCreated struct {
Invitation Invitation `json:"invitation"`
ShareUrl string `json:"share_url"`
OperationId string `json:"operation_id"`
}
type InvitationGrant struct {
ScopeKind string `json:"scope_kind"`
ScopeId string `json:"scope_id"`
Capability string `json:"capability"`
}
type DocumentInvitationCreate struct {
OperationId string `json:"operation_id"`
ExpectedAclRevision string `json:"expected_acl_revision"`
ExpiresInHours *int64 `json:"expires_in_hours,omitempty"`
RequestedGrants []InvitationGrant `json:"requested_grants"`
}
type DocumentInvitation struct {
Id string `json:"id"`
DocumentId string `json:"document_id"`
State string `json:"state"`
ExpiresAt string `json:"expires_at"`
RequestedGrants []InvitationGrant `json:"requested_grants"`
Revision string `json:"revision"`
CreatedAt *string `json:"created_at,omitempty"`
}
type DocumentInvitationPage struct {
Items []DocumentInvitation `json:"items"`
NextCursor *string `json:"next_cursor"`
HasMore bool `json:"has_more"`
}
type DocumentInvitationCreated struct {
Invitation DocumentInvitation `json:"invitation"`
ShareUrl string `json:"share_url"`
OperationId string `json:"operation_id"`
}
type DocumentApplicationCreate struct {
OperationId string `json:"operation_id"`
InvitationToken string `json:"invitation_token"`
Reason string `json:"reason"`
}
type DocumentApplication struct {
Id string `json:"id"`
DocumentId string `json:"document_id"`
InvitationId string `json:"invitation_id"`
ApplicantId string `json:"applicant_id"`
InvitationValidatedAt string `json:"invitation_validated_at"`
RequestedGrants []InvitationGrant `json:"requested_grants"`
State string `json:"state"`
Reason string `json:"reason"`
Revision string `json:"revision"`
ApplicantDisplayName *string `json:"applicant_display_name,omitempty"`
CreatedAt *string `json:"created_at,omitempty"`
}
type DocumentApplicationPage struct {
Items []DocumentApplication `json:"items"`
NextCursor *string `json:"next_cursor"`
HasMore bool `json:"has_more"`
}
type OrganizationMembership struct {
UserId string `json:"user_id"`
Role string `json:"role"`
State string `json:"state"`
Revision string `json:"revision"`
}
type PersonalTransferMetadata struct {
Id string `json:"id"`
DocumentId string `json:"document_id"`
DocumentTitle string `json:"document_title"`
SourceUserId string `json:"source_user_id"`
TargetUserId string `json:"target_user_id"`
SourceSpaceId string `json:"source_space_id"`
ExpectedControlRevision string `json:"expected_control_revision"`
Revision string `json:"revision"`
State string `json:"state"`
ExpiresAt string `json:"expires_at"`
TargetSpaceId *string `json:"target_space_id,omitempty"`
TargetKnowledgeBaseId *string `json:"target_knowledge_base_id,omitempty"`
AcceptOperationId *string `json:"accept_operation_id,omitempty"`
}
type PersonalTransferPage struct {
Items []PersonalTransferMetadata `json:"items"`
NextCursor *string `json:"next_cursor"`
}
type PersonalTransferAccept struct {
OperationId string `json:"operation_id"`
ExpectedRevision string `json:"expected_revision"`
TargetKnowledgeBaseId string `json:"target_knowledge_base_id"`
ExpectedQuotaRevision string `json:"expected_quota_revision"`
}
type MainReadingRevision struct {
Revision string `json:"revision"`
}
type ReviewRequestSummary struct {
Title string `json:"title"`
Excerpt string `json:"excerpt"`
RoundNo string `json:"round_no"`
SubmitterId string `json:"submitter_id"`
SubmittedAt string `json:"submitted_at"`
}
type DocumentCollaborator struct {
UserId string `json:"user_id"`
DisplayName string `json:"display_name"`
}
type DocumentCollaboratorPage struct {
Items []DocumentCollaborator `json:"items"`
HasMore bool `json:"has_more"`
NextCursor *string `json:"next_cursor,omitempty"`
}
// RequiredFields maps each required field to whether JSON null is allowed.
var RequiredFields = map[string]map[string]bool{
"Error": {"code": false,"message": false,"trace_id": false,"retry_action": false},
"Action": {"operation_id": false},
"VersionedAction": {"operation_id": false,"expected_revision": false},
"NamedCreate": {"operation_id": false,"name": false},
"Operation": {"operation_id": false,"state": false},
"Login": {"email": false,"password": false},
"LoginResult": {"user_id": false,"display_name": false,"csrf_token": false,"expires_at": false},
"ChallengeCreate": {"purpose": false,"email": false},
"Challenge": {"challenge_id": false,"expires_at": false,"retry_after_seconds": false},
"Signup": {"operation_id": false,"challenge_id": false,"code": false,"email": false,"password": false,"display_name": false},
"ResetPassword": {"operation_id": false,"challenge_id": false,"code": false,"new_password": false},
"PasswordChange": {"operation_id": false,"expected_credential_epoch": false,"current_password": false,"new_password": false},
"EmailChange": {"operation_id": false,"expected_credential_epoch": false,"challenge_id": false,"code": false,"new_email": false},
"User": {"id": false,"display_name": false,"state": false,"credential_epoch": false,"identity_epoch": false},
"Organization": {"id": false,"name": false,"owner_id": false,"lifecycle": false,"revision": false,"member_revision": false},
"Member": {"user_id": false,"display_name": false,"role": false,"state": false,"revision": false},
"OrganizationEdit": {"operation_id": false,"expected_revision": false,"name": false},
"MemberRole": {"operation_id": false,"expected_revision": false,"role": false},
"OwnerChange": {"operation_id": false,"expected_revision": false,"target_user_id": false},
"InvitationCreate": {"operation_id": false,"expected_member_revision": false,"expires_in_hours": false},
"InvitationAccept": {"operation_id": false,"invitation_token": false,"reason": false},
"ApplicationCreate": {"operation_id": false,"reason": false},
"ApplicationReview": {"operation_id": false,"expected_revision": false,"decision": false,"reason": false},
"Invitation": {"id": false,"organization_id": false,"state": false,"expires_at": false,"revision": false},
"Application": {"id": false,"organization_id": false,"applicant_id": false,"reason": false,"state": false,"revision": false},
"KnowledgeBase": {"id": false,"space_id": false,"name": false,"lifecycle": false,"acl_revision": false,"visibility_policy": false,"visibility_revision": false},
"KBCreate": {"operation_id": false,"name": false},
"KBRole": {"operation_id": false,"expected_acl_revision": false,"user_id": false,"role": false},
"CreationGrant": {"operation_id": false,"expected_revision": false,"user_id": false,"allowed": false},
"DocumentCreate": {"operation_id": false,"space_id": false,"title": false},
"Document": {"id": false,"title": false,"space_id": false,"lifecycle": false,"main_locked": false,"current_structure_id": false,"current_style_id": false,"current_manifest_id": false,"publication_state": false,"control_revision": false,"acl_revision": false,"publication_epoch": false,"visibility_policy": false,"visibility_revision": false},
"DocumentEdit": {"operation_id": false,"expected_control_revision": false,"title": false},
"Grant": {"user_id": false,"scope_kind": false,"scope_id": false,"capability": false},
"AuthorizationChange": {"operation_id": false,"expected_acl_revision": false,"changes": false},
"AuthorizationResult": {"operation_id": false,"state": false,"scope_key": false,"epoch": false,"missing_participants": false},
"EditSessionCreate": {"operation_id": false,"workspace_kind": false,"client_session_id": false,"expected_structure_id": false},
"EditSession": {"ticket": false,"ticket_expires_at": false,"wss_url": false,"room_id": false,"workspace_epoch": false},
"Withdrawal": {"operation_id": false,"expected_round_id": false},
"RequestChanges": {"operation_id": false,"round_id": false,"content_version_id": false,"type": false,"reason": false},
"ResolveChanges": {"operation_id": false,"expected_round_id": false,"expected_change_revision": false,"reason": false},
"ApproveMerge": {"operation_id": false,"round_id": false,"content_version_id": false,"own_changes_to_resolve": false},
"ReviewRequest": {"id": false,"document_id": false,"unit_id": false,"branch_id": false,"status": false,"current_round_id": false,"revision": false,"archived_at": true,"actions": false},
"ReviewRound": {"id": false,"request_id": false,"round_no": false,"content_version_id": false,"base_manifest_id": false,"base_version_id": false,"structure_id": false,"lineage_id": false,"submitter_id": false,"content": false,"structure": false,"style": false},
"FixedContent": {"id": false,"document_id": false,"unit_id": false,"structure_id": false,"schema_version": false,"canonicalization_version": false,"body_json": false,"body_hash": false,"byte_size": false,"resource_ids": false,"provenance": false},
"MainView": {"read_view_id": false,"expires_at": false,"document": false,"manifest_id": false,"structure_id": false,"style_id": false,"units": false,"structure": false,"style": false},
"EditableUnit": {"unit_id": false,"path": false,"state": false,"workspace_epoch": false},
"ReleaseCreate": {"operation_id": false,"expected_publication_revision": false,"expected_control_revision": false},
"Publication": {"operation_id": false,"expected_publication_revision": false,"action": false},
"Release": {"id": false,"document_id": false,"manifest_id": false,"publisher_id": false,"created_at": false},
"PublicDocument": {"document_id": false,"release_id": false,"publication_epoch": false,"title": false,"manifest_id": false,"units": false,"structure": false,"style": false},
"MainLock": {"operation_id": false,"expected_control_revision": false,"locked": false},
"StyleChange": {"operation_id": false,"expected_style_id": false,"expected_control_revision": false,"style": false},
"ChapterCreate": {"operation_id": false,"expected_structure_id": false,"title": false},
"StructureChange": {"operation_id": false,"expected_structure_id": false,"expected_control_revision": false,"changes": false},
"DiscardDraft": {"operation_id": false,"expected_branch_id": false,"expected_structure_id": false,"preview_cut_ref": false,"changes_to_resolve": false},
"PullMain": {"operation_id": false,"expected_branch_id": false,"expected_lineage_id": false,"expected_structure_id": false},
"ReverseChange": {"operation_id": false,"expected_manifest_id": false,"merge_record_id": false,"reason": false},
"ResourceUpload": {"operation_id": false,"expected_quota_revision": false,"byte_size": false,"media_type": false,"checksum": false},
"UploadHandle": {"resource_id": false,"upload_url": false,"expires_at": false},
"UploadComplete": {"operation_id": false,"checksum": false,"byte_size": false},
"Resource": {"id": false,"state": false,"byte_size": false,"media_type": false,"revision": false},
"ExportJob": {"id": false,"document_id": false,"manifest_id": false,"format": false,"state": false},
"Anchor": {"schema_version": false,"content_version_id": false},
"CommentThreadCreate": {"operation_id": false,"context_kind": false,"context_id": false,"anchor": false,"body": false,"mentions": false},
"CommentThread": {"id": false,"document_id": false,"context_kind": false,"context_id": false,"anchor": false,"author_id": false,"status": false,"revision": false},
"CommentCreate": {"operation_id": false,"body": false,"mentions": false},
"Comment": {"id": false,"thread_id": false,"author_id": false,"body": false,"revision": false},
"CommentEdit": {"operation_id": false,"expected_revision": false,"body": false},
"ThreadState": {"operation_id": false,"expected_revision": false,"state": false},
"Notification": {"id": false,"event_id": false,"kind": false,"title": false,"read_at": true},
"Quota": {"space_id": false,"limit_bytes": false,"used_bytes": false,"reserved_bytes": false,"revision": false},
"TrashAction": {"operation_id": false,"expected_control_revision": false},
"PurgeAction": {"operation_id": false,"expected_control_revision": false,"confirmed": false},
"AccountClose": {"operation_id": false,"expected_identity_epoch": false,"current_password": false},
"AccountClosePlan": {"operation_id": false,"state": false,"required_successions": false,"retained_release_ids": false},
"SearchResult": {"document_id": false,"release_id": false,"title": false,"snippet": false},
"OrganizationPage": {"items": false,"next_cursor": true,"has_more": false},
"MemberPage": {"items": false,"next_cursor": true,"has_more": false},
"InvitationPage": {"items": false,"next_cursor": true,"has_more": false},
"ApplicationPage": {"items": false,"next_cursor": true,"has_more": false},
"KnowledgeBasePage": {"items": false,"next_cursor": true,"has_more": false},
"DocumentPage": {"items": false,"next_cursor": true,"has_more": false},
"ReviewRequestPage": {"items": false,"next_cursor": true,"has_more": false},
"ReviewRoundPage": {"items": false,"next_cursor": true,"has_more": false},
"EditableUnitPage": {"items": false,"next_cursor": true,"has_more": false},
"ReleasePage": {"items": false,"next_cursor": true,"has_more": false},
"CommentThreadPage": {"items": false,"next_cursor": true,"has_more": false},
"CommentPage": {"items": false,"next_cursor": true,"has_more": false},
"NotificationPage": {"items": false,"next_cursor": true,"has_more": false},
"SearchResultPage": {"items": false,"next_cursor": true,"has_more": false},
"VisibilityChange": {"operation_id": false,"expected_visibility_revision": false,"policy": false},
"Space": {"id": false,"kind": false,"lifecycle": false,"actions": false},
"SpacePage": {"items": false,"next_cursor": true,"has_more": false},
"StructureEntry": {"node_id": false,"chapter_id": false,"kind": false,"title": false,"sort_order": false,"split_level2": false},
"StructureRevision": {"id": false,"document_id": false,"schema_version": false,"entries": false},
"StyleRevision": {"id": false,"document_id": false,"schema_version": false,"style": false},
"DiscardPreviewCreate": {"operation_id": false,"expected_branch_id": false,"expected_structure_id": false},
"DiscardPreview": {"preview_cut_ref": false,"document_id": false,"unit_id": false,"branch_id": false,"workspace_epoch": false,"lineage_id": false,"base_manifest_id": false,"draft_content_version_id": false,"main_manifest_id": false,"structure_id": false,"cut_digest": false,"expires_at": false,"blocking_changes": false},
"StructurePreviewCreate": {"operation_id": false,"expected_structure_id": false,"expected_control_revision": false,"changes": false},
"StructurePreview": {"preview_cut_ref": false,"structure_id": false,"cut_digest": false,"expires_at": false,"blocking_units": false},
"ChangeRequest": {"id": false,"request_id": false,"round_id": false,"content_version_id": false,"author_id": false,"status": false,"reason": false,"revision": false,"created_at": false},
"ChangeRequestPage": {"items": false,"next_cursor": true,"has_more": false},
"ReviewAction": {"id": false,"request_id": false,"round_id": false,"actor_id": false,"type": false,"created_at": false},
"ReviewActionPage": {"items": false,"next_cursor": true,"has_more": false},
"Conflict": {"id": false,"block_id": false,"node_path": false,"kind": false,"reason": false},
"ConflictContext": {"conflict_ref": false,"operation_id": false,"attempt_id": false,"document_id": false,"unit_id": false,"request_id": false,"round_id": false,"base": false,"submitted": false,"main": false,"structure": false,"style": false,"conflicts": false,"input_digest": false,"expires_at": false},
"ConflictResolution": {"operation_id": false,"expected_attempt_id": false,"expected_input_digest": false,"expected_round_id": false,"resolution_content": false,"resolved_conflict_ids": false},
"InvitationCreated": {"invitation": false,"share_url": false,"operation_id": false},
"InvitationGrant": {"scope_kind": false,"scope_id": false,"capability": false},
"DocumentInvitationCreate": {"operation_id": false,"expected_acl_revision": false,"requested_grants": false},
"DocumentInvitation": {"id": false,"document_id": false,"state": false,"expires_at": false,"requested_grants": false,"revision": false},
"DocumentInvitationPage": {"items": false,"next_cursor": true,"has_more": false},
"DocumentInvitationCreated": {"invitation": false,"share_url": false,"operation_id": false},
"DocumentApplicationCreate": {"operation_id": false,"invitation_token": false,"reason": false},
"DocumentApplication": {"id": false,"document_id": false,"invitation_id": false,"applicant_id": false,"invitation_validated_at": false,"requested_grants": false,"state": false,"reason": false,"revision": false},
"DocumentApplicationPage": {"items": false,"next_cursor": true,"has_more": false},
"OrganizationMembership": {"user_id": false,"role": false,"state": false,"revision": false},
"PersonalTransferMetadata": {"id": false,"document_id": false,"document_title": false,"source_user_id": false,"target_user_id": false,"source_space_id": false,"expected_control_revision": false,"revision": false,"state": false,"expires_at": false},
"PersonalTransferPage": {"items": false,"next_cursor": true},
"PersonalTransferAccept": {"operation_id": false,"expected_revision": false,"target_knowledge_base_id": false,"expected_quota_revision": false},
"MainReadingRevision": {"revision": false},
"ReviewRequestSummary": {"title": false,"excerpt": false,"round_no": false,"submitter_id": false,"submitted_at": false},
"DocumentCollaborator": {"user_id": false,"display_name": false},
"DocumentCollaboratorPage": {"items": false,"has_more": false},
}
