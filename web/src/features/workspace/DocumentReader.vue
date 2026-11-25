<script setup lang="ts">
import {
  UserFacingError,
  ApiError,
  api,
  explain,
  transportGeneration,
  useAction,
  actionsBlocked,
  sessionContextID,
} from '../../lib/http';
import {
  ref,
  computed,
  watch,
  nextTick,
  onMounted,
  onUnmounted,
  defineAsyncComponent,
  toRaw,
} from 'vue';
import { schema, toPM } from '../../../../packages/content/index.mjs';
import { projectAnchor } from '../comments/anchors';
import type {
  Anchor,
  Document,
  MainView,
  StructureEntry,
  MainReadingRevision,
  FixedContent,
  EditableUnitPage,
  EditableUnit,
  ConflictContext,
  Grant,
  DocumentCollaborator,
  DocumentCollaboratorPage,
  CommentThread,
} from '../../../../contracts/http/generated';
import ActionStatus from '../../components/ActionStatus.vue';
import AppPopover from '../../components/AppPopover.vue';
import AppDialog from '../../components/AppDialog.vue';
import MemberPicker from '../../components/MemberPicker.vue';
// 编辑器仅在实际编辑时加载；阅读按需使用共享只读正文模型，不建立协作连接。
const UnitEditor = defineAsyncComponent(() => import('../editor/UnitEditor.vue'));
const ContentView = defineAsyncComponent(() => import('../../lib/content/ContentView.vue'));
const ReviewPanel = defineAsyncComponent(() => import('../review/ReviewPanel.vue'));
const CommentsPanel = defineAsyncComponent(() => import('../comments/CommentsPanel.vue'));
const MainControls = defineAsyncComponent(() => import('../main/MainControls.vue'));
const MainStyleControls = defineAsyncComponent(() => import('../main/MainStyleControls.vue'));
const StructureControls = defineAsyncComponent(() => import('../structure/StructureControls.vue'));
const ConflictWorkbench = defineAsyncComponent(() => import('../conflict/ConflictWorkbench.vue'));
const PublicationPanel = defineAsyncComponent(() => import('../publication/PublicationPanel.vue'));
const SharingPanel = defineAsyncComponent(() => import('./SharingPanel.vue'));
const ExportPanel = defineAsyncComponent(() => import('../exports/ExportPanel.vue'));
const PersonalTransferOffer = defineAsyncComponent(
  () => import('../transfers/PersonalTransferOffer.vue'),
);
import { user } from '../account/session';
import { accountNumber } from '../account/identity';
import { documentPath, editorPath, workspaceReturn } from './navigation';
import { chapterGroups, unitTitle } from '../structure/reading';
const props = defineProps<{
    id: string;
    editorTarget?: { unit: string; kind: 'MAIN' | 'BRANCH' };
  }>(),
  emit = defineEmits<{
    changed: [];
    deleted: [];
  }>();
const panes = [
  { key: 'read', name: '正文' },
  { key: 'collaboration', name: '协作' },
  { key: 'review', name: '审核' },
  { key: 'comments', name: '评论' },
  { key: 'publication', name: '版本与发布' },
  { key: 'settings', name: '文档设置' },
];
const initialPane = new URLSearchParams(location.search).get('tab');
const pane = ref(panes.some((item) => item.key === initialPane) ? initialPane! : 'read');
const readingCommentsOpen = ref(false);
const readingCommentSidebar = ref<HTMLElement | null>(null);
const editChoices = ref(false),
  exportVisible = ref(false),
  trashDialogOpen = ref(false);
const collaborationPane = ref<'members' | 'invitations' | 'applications' | 'transfer'>('members');
const collaborators = ref<DocumentCollaborator[]>([]),
  collaboratorCursor = ref<string | null>(null);
const collaboratorQuery = ref('');
const collaboratorOptions = computed(() =>
  collaborators.value.map((person) => ({ ...person, label: collaboratorLabel(person) })),
);
const selectedCollaborator = computed(() =>
  collaborators.value.find((person) => person.user_id === target.value),
);
function collaboratorLabel(person: DocumentCollaborator) {
  const name = person.user_id === user.value?.id ? user.value.display_name : person.display_name;
  if (person.user_id === user.value?.id) return name + '（我）';
  // 姓名用于展示，ID仍是权限目标；只有重名时显示编号帮助区分。
  return collaborators.value.some(
    (other) => other.user_id !== person.user_id && other.display_name === name,
  )
    ? name + '（编号 ' + accountNumber(person.user_id) + '）'
    : name;
}
async function openCommentAnchor(anchor: Anchor, thread?: string) {
  focusedAnchor.value = anchor;
  focusedCommentThread.value = thread ?? '';
  if (pane.value === 'read') await scrollCommentAnchor(anchor);
}
async function openReadingComments(thread?: string) {
  readingCommentsOpen.value = true;
  await nextTick();
  if (thread) await commentsPanel.value?.openThread(thread);
  // 手机采用纵向堆叠，展开后把评论带到可见处；桌面只滚动到邻近侧栏，不离开正文页签。
  readingCommentSidebar.value?.scrollIntoView({ block: 'nearest' });
}
async function scrollCommentAnchor(anchor: Anchor) {
  const position = resolveCommentAnchor(anchor);
  if (!position) return;
  await nextTick();
  const unit = window.document.getElementById('unit-' + position.unit);
  (unit?.querySelector('.comment-anchor-highlight, .comment-anchor-caret') ?? unit)?.scrollIntoView(
    { block: 'center', behavior: 'smooth' },
  );
}
async function locateCommentAnchor(anchor: Anchor) {
  if (!resolveCommentAnchor(anchor)) {
    error.value = '这条评论对应历史正文，请在评论中的原文预览查看位置。';
    return;
  }
  focusedAnchor.value = anchor;
  pane.value = 'read';
  readingCommentsOpen.value = true;
  await scrollCommentAnchor(anchor);
}
const document = ref<Document | null>(null),
  main = ref<MainView | null>(null),
  bodies = ref<Record<string, FixedContent>>({}),
  editable = ref<EditableUnit[]>([]),
  editCursor = ref<string | null>(null),
  actions = ref<string[]>([]);
// 接口按同级sort_order返回条目，并非全文阅读顺序。章与子节必须按父节点重组；
// 拆分后的章没有unit_id，仍是正文标题和写作范围的父级，不能因无独立正文而丢掉。
const readingChapters = computed(() => chapterGroups(main.value?.structure.entries ?? []));
const readingUnits = computed(() => readingChapters.value.flatMap((group) => group.units));
function unitLabel(entry?: StructureEntry) {
  if (!entry) return '';
  const title = unitTitle(entry);
  const chapter = readingChapters.value.find(
    (group) => group.chapter.chapter_id === entry.chapter_id,
  );
  return entry.kind === 'CHAPTER' || !chapter ? title : chapter.chapter.title + ' · ' + title;
}
const title = ref(''),
  target = ref(''),
  scope = ref(''),
  capability = ref<'EDIT' | 'REVIEW' | 'COMMENT' | 'MANAGE'>('COMMENT'),
  grantAction = ref<'GRANT' | 'REVOKE'>('GRANT'),
  policy = ref('COLLABORATORS'),
  action = useAction();
const { busy, error, notice, pending, retry, write, mutate } = action;
const managing = computed(() => actions.value.includes('CHANGE_AUTHORIZATION'));
// COMMENT来自服务端汇总后的实际权限来源，不从可编辑单元或角色名称推导。
const commenting = computed(() => actions.value.includes('COMMENT'));
const commentUnit = ref('');
const commentsPanel = ref<{
  hasDraft: boolean;
  beginComment: () => void;
  openThread: (thread: string) => Promise<void>;
} | null>(null);
const mainCommentThreads = ref<CommentThread[]>([]);
const commentAnchor = ref<Anchor | null>(null);
const focusedAnchor = ref<Anchor | null>(null);
const focusedCommentThread = ref('');
// 仅重建当前可靠正文的只读模型；投影用于界面定位，不进入写接口、不改旧讨论的固定版本。
const commentDocuments = computed(() =>
  Object.entries(bodies.value).map(([unit, body]) => ({
    unit,
    version: body.id,
    doc: schema.nodeFromJSON(toPM(toRaw(body.body_json))),
  })),
);
function resolveCommentAnchor(anchor: Anchor) {
  const exact = commentDocuments.value.find((item) => item.version === anchor.content_version_id);
  if (exact) {
    const projected = projectAnchor(exact.doc, exact.version, anchor);
    return projected ? { unit: exact.unit, anchor: projected } : null;
  }
  const matches = commentDocuments.value.flatMap(({ unit, version, doc }) => {
    const projected = projectAnchor(doc, version, anchor);
    return projected ? [{ unit, anchor: projected }] : [];
  });
  return matches.length === 1 ? matches[0]! : null;
}
const focusedCommentLocation = computed(() =>
  focusedAnchor.value ? resolveCommentAnchor(focusedAnchor.value) : null,
);
const commentLocations = computed(() =>
  Object.fromEntries(
    mainCommentThreads.value.map((thread) => {
      const position = resolveCommentAnchor(thread.anchor);
      const original = Object.entries(bodies.value).find(
        ([, body]) => body.id === thread.anchor.content_version_id,
      );
      const unit = position?.unit ?? original?.[0];
      const title = unitLabel(readingUnits.value.find((entry) => entry.unit_id === unit));
      const kind: 'current' | 'matched' | 'unplaced' | 'historical' = position
        ? position.anchor.content_version_id === thread.anchor.content_version_id
          ? 'current'
          : 'matched'
        : original && !thread.anchor.block_id
          ? 'unplaced'
          : 'historical';
      return [
        thread.id,
        {
          kind,
          label: position
            ? title + (kind === 'matched' ? ' · 原引用仍在此处' : ' · 正文引用')
            : original && !thread.anchor.block_id
              ? title + ' · 整章讨论'
              : '历史引用 · 查看原文',
        },
      ];
    }),
  ),
);
const commentMarkers = computed(() =>
  mainCommentThreads.value
    .map((thread, index) => ({
      threadId: thread.id,
      anchor: resolveCommentAnchor(thread.anchor)?.anchor ?? thread.anchor,
      number: index + 1,
      status: thread.status,
      label: '查看讨论 ' + thread.id + (thread.status === 'RESOLVED' ? '（已解决）' : '（进行中）'),
    }))
    .filter((marker) => marker.status !== 'DELETED' && marker.anchor.block_id),
);
const focusedCommentHint = computed(() => {
  if (!focusedAnchor.value) return '';
  return !!focusedCommentLocation.value
    ? focusedAnchor.value.block_id
      ? '黄色高亮处为这条评论引用的原文。'
      : '这条讨论对应当前正文，未指定具体文字位置。'
    : !focusedAnchor.value.block_id &&
        Object.values(bodies.value).some(
          (body) => body.id === focusedAnchor.value?.content_version_id,
        )
      ? '这条评论针对整章，未选定具体文字。'
      : '这条评论引用历史正文，请在讨论中的“查看原文位置”核验。';
});
function selectCommentAnchor(anchor: Anchor | null, unit: string) {
  if (anchor) {
    commentUnit.value = unit;
    commentAnchor.value = anchor;
  } else if (commentUnit.value === unit) commentAnchor.value = null;
}
const commentUnits = computed(() =>
  readingUnits.value
    .filter((entry) => entry.unit_id && bodies.value[entry.unit_id])
    .map((entry) => ({ ...entry, title: unitLabel(entry) })),
);
const commentVersion = computed(() => {
  const unit =
    commentUnits.value.find((entry) => entry.unit_id === commentUnit.value) ??
    commentUnits.value[0];
  return unit?.unit_id ? (bodies.value[unit.unit_id]?.id ?? '') : '';
});
watch(commentVersion, (version) => {
  // 切换单元时保留刚从该单元选中的范围，只清除属于旧固定版本的锚点。
  if (commentAnchor.value?.content_version_id !== version) commentAnchor.value = null;
  if (focusedAnchor.value?.content_version_id !== version) focusedAnchor.value = null;
});
const editingUnit = ref<string | null>(null);
const editingKind = ref<'BRANCH' | 'MAIN'>('BRANCH');
const editor = ref<{ confirmSave: () => Promise<unknown> } | null>(null);
const conflictWorkbench = ref<{ accept: (context: ConflictContext) => Promise<void> } | null>(null);
const conflictOpen = ref(false);
const initialRequest = new URLSearchParams(location.search).get('request_id') ?? '';
const initialReview = ref(/^[1-9][0-9]{0,19}$/.test(initialRequest) ? initialRequest : '');
function reviewSubmitted(request: string) {
  initialReview.value = request;
  reviewRefresh.value++;
  if (!props.editorTarget) pane.value = 'review';
}
async function reloadReverse(request: string) {
  initialReview.value = request;
  await reloadResolution();
}
function canRepair(unit: string) {
  // 修复要求当前原单元的完整EDIT；管理资格不能把已退役的历史unit重新加入编辑范围。
  return editable.value.some((value) => value.unit_id === unit);
}
async function openConflict(value: ConflictContext) {
  if (!conflictWorkbench.value) throw new UserFacingError('修复工作台尚未加载，请稍后打开');
  // 切换专用工作区只隐藏列表；返回列表不销毁固定上下文与未保存的人工候选。
  conflictOpen.value = true;
  await conflictWorkbench.value.accept(value);
}
async function reloadResolution() {
  // 新BRANCH代次已经安装，旧BRANCH只能保留队列并关闭；仍有效的MAIN输入需先可靠保存。
  if (editingKind.value === 'MAIN' && editor.value) await editor.value.confirmSave();
  editingUnit.value = null;
  await load();
}
async function prepareMainLock() {
  if (editingKind.value === 'MAIN' && editor.value) await editor.value.confirmSave();
}
async function reloadPublication() {
  // 发布只更换正式指针，不安装新MAIN。刷新元数据时保留正在编辑的原工作区和后续本机输入。
  document.value = await api<Document>('/v1/documents/' + props.id, 'GET', undefined, readActions);
  emit('changed');
}
async function prepareStructure() {
  if (editor.value) await editor.value.confirmSave();
}
async function reloadMerge() {
  // 计算期间编辑器仍可保留后续输入。实际合并READY后，先可靠保存再卸载和重读主线。
  if (editor.value) await editor.value.confirmSave();
  editingUnit.value = null;
  await load();
}
async function reloadMainControl() {
  // 关闭代次已经实际完成后再刷新。旧编辑器卸载会保留本机队列，不能跨工作区复用视图。
  editingUnit.value = null;
  await load();
}
async function reloadDraftControl() {
  editingUnit.value = null;
  reviewRefresh.value++;
  await load();
}
async function closeEditor() {
  await saveAndNavigate(documentPath(props.id));
}
async function saveAndNavigate(path: string) {
  await action.run(async () => {
    if (editor.value) await editor.value.confirmSave();
    location.assign(path);
  }, '正文已保存');
}
async function loadCollaborators(append = false) {
  const page = await api<DocumentCollaboratorPage>(
    '/v1/documents/' +
      props.id +
      '/collaborators?limit=20' +
      (append && collaboratorCursor.value
        ? '&cursor=' + encodeURIComponent(collaboratorCursor.value)
        : ''),
  );
  const entries = append ? [...collaborators.value, ...page.items] : page.items;
  collaborators.value = [...new Map(entries.map((entry) => [entry.user_id, entry])).values()];
  collaboratorCursor.value = page.next_cursor ?? null;
}
async function collaborationDirectory() {
  if (managing.value) await loadCollaborators();
}
function selectPane(next: string) {
  pane.value = next;
  exportVisible.value = false;
  editChoices.value = false;
  if (next === 'collaboration') void action.run(collaborationDirectory, '');
}
const reviewRefresh = ref(0);
function readActions(h: Headers) {
  try {
    actions.value = JSON.parse(h.get('X-Available-Actions') ?? '[]');
  } catch {
    actions.value = [];
  }
}
async function loadEditable(append = false) {
  const p = await api<EditableUnitPage>(
    '/v1/documents/' +
      props.id +
      '/editable-units?limit=100' +
      (append && editCursor.value ? '&cursor=' + encodeURIComponent(editCursor.value) : ''),
  );
  editable.value = append ? [...editable.value, ...p.items] : p.items;
  editCursor.value = p.next_cursor;
}
const checkingReading = ref(false),
  readingUpdate = ref(false),
  readingSyncError = ref('');
let readingRevision = '',
  readingGeneration = 0,
  readerAlive = true,
  readingTimer: ReturnType<typeof setTimeout> | undefined;

async function readRevision() {
  const value = await api<MainReadingRevision>('/v1/documents/' + props.id + '/main-revision');
  return value.revision;
}

async function fetchReading(revision?: string) {
  // 版本提示必须在捕获正文之前读取。中间再有SAVED，下次检查仍会发现变化，
  // 不能先读旧正文再把较新的尾部记成“已经显示”，否则会漏掉更新。
  if (revision === undefined) {
    try {
      revision = props.editorTarget ? '' : await readRevision();
    } catch (failure) {
      if (failure instanceof ApiError && [401, 403, 404, 410].includes(failure.status))
        throw failure;
      // 变化提示短暂不可用不阻断原正文读取；留空使下一次成功检查重新核对正文。
      revision = '';
    }
  }
  let available: string[] = [];
  const metadata = await api<Document>('/v1/documents/' + props.id, 'GET', undefined, (headers) => {
    try {
      available = JSON.parse(headers.get('X-Available-Actions') ?? '[]');
    } catch {}
  });
  const [view, editPage] = await Promise.all([
    api<MainView>('/v1/documents/' + props.id + '/main'),
    api<EditableUnitPage>('/v1/documents/' + props.id + '/editable-units?limit=100'),
  ]);
  const content: Record<string, FixedContent> = {};
  for (const unit of view.units) {
    content[unit.unit_id] = await api<FixedContent>(
      '/v1/documents/' + props.id + '/read-views/' + view.read_view_id + '/units/' + unit.unit_id,
    );
  }
  return { metadata, available, view, content, editPage, revision };
}
function installReading(snapshot: Awaited<ReturnType<typeof fetchReading>>) {
  const previous = document.value;
  // 背景更新不重建整页，不覆盖尚未提交的文档标题/范围表单。
  if (!previous || title.value === previous.title) title.value = snapshot.metadata.title;
  if (!previous || policy.value === previous.visibility_policy)
    policy.value = snapshot.metadata.visibility_policy;
  document.value = snapshot.metadata;
  actions.value = snapshot.available;
  main.value = snapshot.view;
  bodies.value = snapshot.content;
  editable.value = snapshot.editPage.items;
  editCursor.value = snapshot.editPage.next_cursor;
  readingRevision = snapshot.revision;
  readingUpdate.value = false;
  readingSyncError.value = '';
}
function protectReading() {
  // 评论引用固定正文，选区和草稿存在时不悄悄换版本。其它模块中的审核/设置表单
  // 同样不被背景更新打断；回到正文并完成当前输入后自动补上最新内容。
  return (
    !!props.editorTarget ||
    pane.value !== 'read' ||
    exportVisible.value ||
    trashDialogOpen.value ||
    !!commentAnchor.value ||
    !!focusedAnchor.value ||
    !!commentsPanel.value?.hasDraft ||
    window.getSelection()?.isCollapsed === false
  );
}
async function syncReading(force = false) {
  if (
    !readerAlive ||
    props.editorTarget ||
    !main.value ||
    checkingReading.value ||
    (!force && (actionsBlocked.value || window.document.hidden))
  )
    return;
  const generation = readingGeneration,
    identity = transportGeneration();
  const current = () =>
    readerAlive && generation === readingGeneration && identity === transportGeneration();
  checkingReading.value = true;
  try {
    const revision = await readRevision();
    if (!current()) return;
    readingSyncError.value = '';
    if (!force && revision === readingRevision) {
      readingUpdate.value = false;
      return;
    }
    if (!force && protectReading()) {
      readingUpdate.value = true;
      return;
    }
    if (force && commentsPanel.value?.hasDraft)
      throw new UserFacingError('评论草稿或文字定位已保留，请完成当前评论后再更新正文。');
    const snapshot = await fetchReading(revision);
    // 请求期间可能开始写评论、执行写操作或离开文档，再检查一次再安装。
    if (!current()) return;
    if (!force && (actionsBlocked.value || protectReading())) {
      readingUpdate.value = true;
      return;
    }
    const position = window.scrollY;
    if (force) {
      // 用户主动刷新才退出原文字定位；自动检查绝不清除正在使用的选区。
      commentAnchor.value = null;
      focusedAnchor.value = null;
      window.getSelection()?.removeAllRanges();
    }
    installReading(snapshot);
    await nextTick();
    if (current()) window.scrollTo({ top: position, behavior: 'instant' });
  } catch (failure) {
    if (!current()) return;
    if (failure instanceof ApiError && [401, 403, 404, 410].includes(failure.status)) {
      // 确认失去权限/文档已删除时清除旧正文，断网和短暂503则保留阅读位置。
      document.value = null;
      main.value = null;
      bodies.value = {};
      actions.value = [];
      editable.value = [];
      error.value = explain(failure);
      if (force) throw failure;
    } else if (force) throw failure;
    else readingSyncError.value = '暂时无法检查新内容，连接恢复后会自动重试。';
  } finally {
    checkingReading.value = false;
  }
}
function scheduleReading() {
  // 串行、可见页面每5秒检查一次摘要；没有变化时不Capture、不下载正文。
  readingTimer = setTimeout(async () => {
    await syncReading();
    if (readerAlive) scheduleReading();
  }, 5000);
}
function resumeReading() {
  if (!window.document.hidden) void syncReading();
}
async function load() {
  const generation = ++readingGeneration;
  // 先清空旧正文。资格变化/自然过期时不保留“刷新失败但旧正文仍冒充当前”的界面。
  document.value = null;
  commentAnchor.value = null;
  focusedAnchor.value = null;
  mainCommentThreads.value = [];
  main.value = null;
  bodies.value = {};
  editable.value = [];
  actions.value = [];
  collaborators.value = [];
  collaboratorCursor.value = null;
  const snapshot = await fetchReading();
  if (!readerAlive || generation !== readingGeneration) return;
  installReading(snapshot);
  const d = snapshot.metadata,
    view = snapshot.view;
  scope.value = scope.value || 'DOCUMENT:' + props.id;
  // 编辑路由刷新后必须重新查到目标，不能把首页没出现误判为无资格。
  // 仅为指定BRANCH按需续页，实际编辑准入仍由原edit-session入口再次核验。
  while (
    props.editorTarget?.kind === 'BRANCH' &&
    editCursor.value &&
    !editable.value.some((entry) => entry.unit_id === props.editorTarget?.unit)
  )
    await loadEditable(true);
  if (pane.value === 'settings' && !managing.value) pane.value = 'read';
  if (pane.value === 'collaboration' && !props.editorTarget) await collaborationDirectory();
  if (props.editorTarget) {
    const { unit, kind } = props.editorTarget;
    const exists = view.structure.entries.some((entry) => entry.unit_id === unit);
    const allowed =
      kind === 'MAIN'
        ? managing.value && !d.main_locked
        : editable.value.some((entry) => entry.unit_id === unit && entry.state === 'READY');
    if (!exists || !allowed)
      throw new UserFacingError(
        kind === 'MAIN'
          ? '主线已锁定或你没有编辑资格，请返回文档设置核对。'
          : '该单元目前没有支线编辑资格，请返回阅读页刷新范围。',
      );
    editingKind.value = kind;
    editingUnit.value = unit;
  }
}
function refresh() {
  void action.run(() => syncReading(true), '阅读与资格已刷新');
}
onMounted(() => {
  void action.run(load, '');
  scheduleReading();
  window.addEventListener('collab-refresh', refresh);
  window.document.addEventListener('visibilitychange', resumeReading);
});
onUnmounted(() => {
  readerAlive = false;
  ++readingGeneration;
  clearTimeout(readingTimer);
  window.removeEventListener('collab-refresh', refresh);
  window.document.removeEventListener('visibilitychange', resumeReading);
});
async function rename() {
  await action.run(async () => {
    await mutate(
      () => '/v1/documents/' + props.id,
      () => ({ expected_control_revision: document.value!.control_revision, title: title.value }),
      'PATCH',
    );
    await load();
    emit('changed');
  }, '文档标题已更新');
}
// 撤销的是直接来源，不能推断剩余能力；等待关闭ACK后重读服务端计算的能力并集。
async function authorize() {
  if (!selectedCollaborator.value || actionsBlocked.value) return;
  await action.run(async () => {
    const [kind, id] = scope.value.split(':');
    if (!id) throw new UserFacingError('请选择当前文档内的范围');
    const g = { scope_kind: kind as Grant['scope_kind'], scope_id: id };
    const grant = {
      ...g,
      user_id: target.value,
      capability: g.scope_kind === 'DOCUMENT' ? capability.value : 'EDIT',
    };
    await mutate(
      () => '/v1/documents/' + props.id + '/authorization-changes',
      () => ({
        expected_acl_revision: document.value!.acl_revision,
        changes: [{ grant, action: grantAction.value }],
      }),
    );
    await load();
  }, '授权已更新，旧输出已关闭');
}
async function transfer() {
  if (!selectedCollaborator.value || actionsBlocked.value) return;
  await action.run(async () => {
    await mutate(
      () => '/v1/documents/' + props.id + '/responsible-transfers',
      () => ({ expected_revision: document.value!.control_revision, target_user_id: target.value }),
    );
    emit('changed');
    await load();
  }, '负责人已转交');
}
async function changeVisibility() {
  await action.run(async () => {
    await mutate(
      () => '/v1/documents/' + props.id + '/visibility',
      () => ({
        expected_visibility_revision: document.value!.visibility_revision,
        policy: policy.value,
      }),
    );
    await load();
  }, '受众范围已更新，旧输出已关闭');
}
async function remove() {
  if (!document.value || !managing.value || actionsBlocked.value) return;
  const documentID = props.id,
    revision = document.value.control_revision;
  await action.run(async () => {
    // 确认时固定目标和修订；未知结果续接原请求，不因弹窗或正文更新生成另一份删除。
    await mutate(
      () => '/v1/documents/' + documentID,
      () => ({ expected_revision: revision }),
      'DELETE',
    );
    trashDialogOpen.value = false;
    emit('deleted');
  }, '文档已移入回收状态');
}
</script>
<template>
  <section class="document-reader" :class="{ 'editor-page': !!editorTarget }">
    <nav class="breadcrumb" aria-label="当前位置">
      <a :href="workspaceReturn(user?.id ?? '', sessionContextID())">工作台</a
      ><span aria-hidden="true">/</span>
      <a v-if="editorTarget" :href="documentPath(id)" @click.prevent="closeEditor">{{
        document?.title ?? '文档'
      }}</a>
      <span v-else>文档阅读</span>
      <template v-if="editorTarget"
        ><span aria-hidden="true">/</span
        ><span>{{ editorTarget.kind === 'MAIN' ? '编辑主线' : '编辑共同支线' }}</span></template
      >
    </nav>
    <header class="document-heading">
      <div>
        <span class="eyebrow">{{
          editorTarget ? 'WRITING STUDIO / 写作工作区' : 'DOCUMENT / 文档'
        }}</span>
        <h1>{{ document?.title ?? '正在打开文档…' }}</h1>
        <p v-if="document" class="document-meta">
          <span class="status-pill">{{ document.main_locked ? '主线已锁定' : '主线开放' }}</span>
          <span>{{ document.publication_state === 'PUBLISHED' ? '已有正式版' : '尚未发布' }}</span>
        </p>
      </div>
      <div v-if="!editorTarget && main" class="page-actions">
        <button :disabled="actionsBlocked || checkingReading" @click="refresh">
          {{ busy ? '刷新中…' : '刷新阅读' }}
        </button>
        <button
          :disabled="actionsBlocked"
          :aria-expanded="exportVisible"
          aria-controls="document-export"
          @click="
            exportVisible = !exportVisible;
            editChoices = false;
          "
        >
          导出文档
        </button>
        <button
          class="primary"
          :disabled="actionsBlocked"
          :aria-expanded="editChoices"
          aria-controls="document-editor-choices"
          @click="
            editChoices = !editChoices;
            exportVisible = false;
          "
        >
          编辑文档
        </button>
        <AppPopover
          v-if="managing"
          class="document-more-actions"
          label="更多操作"
          :disabled="actionsBlocked"
        >
          <template #default="{ close }">
            <button
              type="button"
              class="danger"
              @click="
                close();
                trashDialogOpen = true;
              "
            >
              移入回收站
            </button>
          </template>
        </AppPopover>
      </div>
      <button v-else-if="editorTarget" :disabled="actionsBlocked" @click="closeEditor">
        保存并返回文档
      </button>
    </header>
    <ActionStatus
      v-if="!pending && !trashDialogOpen"
      :busy="busy"
      :error="error"
      :notice="notice"
      :pending="pending"
      :retry="retry"
    />
    <p v-if="!editorTarget && readingUpdate && main" class="reading-sync hint" role="status">
      正文有更新。当前评论、选区和操作已保留，完成后返回正文将自动同步。
    </p>
    <p
      v-else-if="!editorTarget && readingSyncError && main"
      class="reading-sync hint"
      role="status"
    >
      {{ readingSyncError }}
    </p>
    <template v-if="document && main">
      <template v-if="editorTarget">
        <div class="editor-context">
          <span>{{
            editingKind === 'MAIN' ? '主线正文 · 直接保存' : '共同支线 · 与协作者实时同步'
          }}</span>
          <span>{{
            unitLabel(readingUnits.find((entry) => entry.unit_id === editorTarget?.unit))
          }}</span>
        </div>
        <UnitEditor
          ref="editor"
          v-if="editingUnit && user"
          :key="editingKind + ':' + editingUnit"
          :actor="user.id"
          :space="document.space_id"
          :document="props.id"
          :unit="editingUnit"
          :kind="editingKind"
          :structure="main.structure.id"
          :reload-draft="reloadDraftControl"
          :managing="managing"
          :commenting="commenting"
          :main-body="bodies[editingUnit]"
          :auto-pull-main="editingKind === 'BRANCH'"
          :style="{
            '--document-font': main.style.style.body_font + ', Times New Roman, serif',
            '--document-size': main.style.style.body_size_pt + 'pt',
          }"
          @close="closeEditor"
          @submitted="reviewSubmitted"
        />

        <p v-if="initialReview" class="success" role="status">
          已完成送审。<button
            @click="saveAndNavigate(documentPath(id) + '?tab=review&request_id=' + initialReview)"
          >
            保存并查看审核记录
          </button>
        </p>
      </template>
      <template v-else>
        <section
          v-if="editChoices"
          id="document-editor-choices"
          class="panel edit-choices"
          aria-label="选择编辑范围"
        >
          <div class="section-title">
            <div>
              <h2>选择写作工作区</h2>
              <p class="hint">
                在当前页面进入编辑。刷新可恢复当前工作区，保存并返回后显示最新正文。
              </p>
            </div>
            <button @click="editChoices = false">收起</button>
          </div>
          <section
            v-for="group in readingChapters"
            :key="group.chapter.node_id"
            class="editor-chapter"
            :aria-label="'写作章节：' + group.chapter.title"
          >
            <h3>{{ group.chapter.title }}</h3>
            <ul class="editor-choice-list">
              <li v-for="entry in group.units" :key="entry.node_id">
                <span>{{ entry.kind === 'CHAPTER' ? '整章正文' : unitTitle(entry) }}</span>
                <div class="page-actions">
                  <a
                    v-if="
                      editable.some(
                        (item) => item.unit_id === entry.unit_id && item.state === 'READY',
                      )
                    "
                    class="button-link"
                    :href="editorPath(id, entry.unit_id!, 'BRANCH')"
                    >编辑共同支线</a
                  >
                  <a
                    v-if="managing && !document.main_locked"
                    class="button-link"
                    :href="editorPath(id, entry.unit_id!, 'MAIN')"
                    >编辑主线</a
                  >
                </div>
              </li>
            </ul>
          </section>
          <p v-if="!editable.length" class="hint">当前没有共同支线编辑资格。</p>
          <p v-if="managing && document.main_locked" class="hint">
            直接编辑主线前，请在“文档设置”中解锁主线。
          </p>
          <button
            v-if="editCursor"
            :disabled="actionsBlocked"
            @click="action.run(() => loadEditable(true), '')"
          >
            更多编辑范围
          </button>
        </section>
        <div v-show="exportVisible" id="document-export">
          <ExportPanel
            :document="props.id"
            target="MAIN"
            :reference="main.manifest_id"
            :main-view="main"
            :main-bodies="bodies"
          />
        </div>
        <nav class="document-tabs" role="tablist" aria-label="文档功能">
          <button
            v-for="item in panes.filter((item) => item.key !== 'settings' || managing)"
            :id="'tab-' + item.key"
            :key="item.key"
            role="tab"
            :aria-selected="pane === item.key"
            :aria-controls="'pane-' + item.key"
            :class="{ selected: pane === item.key }"
            :disabled="actionsBlocked"
            @click="selectPane(item.key)"
          >
            {{ item.name }}
          </button>
        </nav>
        <div v-show="pane === 'read'" id="pane-read" role="tabpanel" aria-labelledby="tab-read">
          <div class="reading-comment-tools">
            <p class="hint">点击正文旁的评论标记查看讨论，也可以展开评论侧栏。</p>
            <button
              type="button"
              :aria-expanded="readingCommentsOpen"
              aria-controls="reading-comment-sidebar"
              :disabled="actionsBlocked"
              @click="readingCommentsOpen ? (readingCommentsOpen = false) : openReadingComments()"
            >
              {{ readingCommentsOpen ? '收起评论' : '查看评论' }}
            </button>
          </div>
          <div v-if="commenting && commentAnchor" class="selection-comment-action" role="status">
            <span>已选中文字</span
            ><button
              @mousedown.prevent
              @click="openReadingComments().then(() => commentsPanel?.beginComment())"
            >
              评论所选文字
            </button>
          </div>
          <div class="reading-layout" :class="{ 'with-comments': readingCommentsOpen }">
            <nav class="document-outline" aria-label="固定目录">
              <details open>
                <summary>文档目录</summary>
                <ol>
                  <li v-for="group in readingChapters" :key="group.chapter.node_id">
                    <a :href="'#chapter-' + group.chapter.node_id">{{ group.chapter.title }}</a>
                    <ol v-if="group.chapter.split_level2">
                      <li v-for="e in group.units" :key="e.node_id">
                        <a :href="'#unit-' + e.unit_id">{{ unitTitle(e) }}</a>
                      </li>
                    </ol>
                  </li>
                </ol>
                <p class="hint">
                  A4 · {{ main.style.style.body_font === 'SimSun' ? '宋体' : '仿宋' }} · 正文
                  {{ main.style.style.body_size_pt }}pt · 页边距 {{ main.style.style.margin_cm }}cm
                </p>
              </details>
            </nav>
            <div
              class="paper"
              :style="{
                fontFamily: main.style.style.body_font + ', Times New Roman, serif',
                fontSize: main.style.style.body_size_pt + 'pt',
              }"
            >
              <section
                v-for="group in readingChapters"
                :id="'chapter-' + group.chapter.node_id"
                :key="group.chapter.node_id"
                :aria-label="'章节：' + group.chapter.title"
              >
                <h3>{{ group.chapter.title }}</h3>
                <div
                  v-for="e in group.units"
                  :id="'unit-' + e.unit_id"
                  :key="e.node_id"
                  class="reading-unit"
                >
                  <h4 v-if="e.kind === 'SECTION'" class="reading-section-title">{{ e.title }}</h4>
                  <ContentView
                    v-if="bodies[e.unit_id!]"
                    :content="bodies[e.unit_id!]"
                    :style="main.style"
                    :resource-context="{ document: props.id, kind: 'MAIN', id: main.manifest_id }"
                    label="主线正文"
                    :selectable="commenting"
                    :anchor="
                      focusedCommentLocation?.anchor.content_version_id === bodies[e.unit_id!].id
                        ? focusedCommentLocation.anchor
                        : null
                    "
                    :comment-markers="
                      commentMarkers.filter(
                        (marker) => marker.anchor.content_version_id === bodies[e.unit_id!].id,
                      )
                    "
                    :active-comment="focusedAnchor ? focusedCommentThread : ''"
                    @selection="selectCommentAnchor($event, e.unit_id!)"
                    @open-comment="openReadingComments($event)"
                  />
                </div>
              </section>
            </div>
            <aside
              v-show="readingCommentsOpen"
              id="reading-comment-sidebar"
              ref="readingCommentSidebar"
              class="reading-comment-sidebar"
              aria-label="正文评论侧栏"
            >
              <div class="section-title">
                <h2>正文评论</h2>
                <button type="button" @click="readingCommentsOpen = false">收起评论侧栏</button>
              </div>
              <p v-if="focusedCommentHint" class="hint" role="status">
                {{ focusedCommentHint }}
                <button type="button" @click="focusedAnchor = null">清除正文定位</button>
              </p>
              <div id="reading-comment-content"></div>
            </aside>
          </div>
        </div>
        <div
          v-if="pane === 'collaboration'"
          id="pane-collaboration"
          class="panel document-section"
          role="tabpanel"
          aria-labelledby="tab-collaboration"
        >
          <h2>协作</h2>
          <p class="hint">邀请伙伴、分配权限与转交文档；内容送审请前往“审核”，交流请前往“评论”。</p>
          <nav class="document-tabs" aria-label="文档协作分区">
            <button
              :aria-current="collaborationPane === 'members' ? 'page' : undefined"
              :disabled="actionsBlocked"
              @click="collaborationPane = 'members'"
            >
              协作者与权限
            </button>
            <template v-if="actions.includes('MANAGE_INVITATIONS')">
              <button
                :aria-current="collaborationPane === 'invitations' ? 'page' : undefined"
                :disabled="actionsBlocked"
                @click="collaborationPane = 'invitations'"
              >
                邀请链接
              </button>
              <button
                :aria-current="collaborationPane === 'applications' ? 'page' : undefined"
                :disabled="actionsBlocked"
                @click="collaborationPane = 'applications'"
              >
                加入申请
              </button>
              <button
                v-if="document.responsible_user_id === user?.id"
                :aria-current="collaborationPane === 'transfer' ? 'page' : undefined"
                :disabled="actionsBlocked"
                @click="collaborationPane = 'transfer'"
              >
                转交文档
              </button>
            </template>
          </nav>
          <section
            v-if="managing"
            v-show="collaborationPane === 'members'"
            class="collaborator-management"
          >
            <form @submit.prevent="authorize">
              <fieldset :disabled="actionsBlocked">
                <MemberPicker
                  v-model="target"
                  v-model:query="collaboratorQuery"
                  :members="collaboratorOptions"
                  search-label="查找协作者"
                  options-label="选择协作者"
                  empty-text="还没有可选协作者。请先邀请对方加入，再分配协作权限。"
                  more-label="更多协作者"
                  :has-more="!!collaboratorCursor"
                  :cursor="collaboratorCursor"
                  :loading="busy"
                  :disabled="actionsBlocked"
                  @more="action.run(() => loadCollaborators(true), '')"
                />
                <label
                  >授权范围<select v-model="scope">
                    <option :value="'DOCUMENT:' + props.id">整个文档（批注、审核或管理）</option>
                    <option
                      v-for="{ chapter: e } in readingChapters"
                      :key="'c' + e.node_id"
                      :value="'CHAPTER:' + e.chapter_id"
                    >
                      整章编辑：{{ e.title }}
                    </option>
                    <option
                      v-for="e in readingUnits"
                      :key="'u' + e.node_id"
                      :value="'UNIT:' + e.unit_id"
                    >
                      单元编辑：{{ unitLabel(e) }}
                    </option>
                  </select></label
                ><label v-if="scope.startsWith('DOCUMENT:')"
                  >文档能力<select v-model="capability">
                    <option value="COMMENT">批注（含主线读取）</option>
                    <option value="REVIEW">审核（含批注）</option>
                    <option value="MANAGE">管理（不能通过邀请授予）</option>
                  </select></label
                ><label
                  >增量动作<select v-model="grantAction">
                    <option value="GRANT">授予</option>
                    <option value="REVOKE">撤销直接授权</option>
                  </select></label
                >
                <p class="hint">
                  编辑不自动包含批注；移除单元直接授权后，整章或上级管理继承仍可能生效。
                </p>
                <button :disabled="actionsBlocked || !selectedCollaborator">提交授权变更</button>
              </fieldset>
            </form>
            <button
              v-if="actions.includes('TRANSFER_RESPONSIBLE')"
              :disabled="actionsBlocked || !selectedCollaborator"
              @click="transfer"
            >
              转交给所选协作者
            </button>
          </section>
          <p v-if="!managing" class="hint">
            当前协作权限由文档管理员分配。可从“编辑文档”进入已授权的共同支线。
          </p>
          <PersonalTransferOffer
            v-show="collaborationPane === 'transfer'"
            v-if="
              document &&
              actions.includes('MANAGE_INVITATIONS') &&
              document.responsible_user_id === user?.id
            "
            :document="id"
            :revision="document.control_revision"
          />
          <SharingPanel
            v-if="
              actions.includes('MANAGE_INVITATIONS') &&
              (collaborationPane === 'invitations' || collaborationPane === 'applications')
            "
            :document="document"
            :entries="main.structure.entries"
            :pane="collaborationPane"
            :reload="load"
          />
        </div>
        <div
          v-if="pane === 'review'"
          v-show="!conflictOpen"
          id="pane-review"
          class="panel document-section"
          role="tabpanel"
          aria-labelledby="tab-review"
        >
          <div class="section-title">
            <div>
              <h2>审核</h2>
              <p class="hint">固定送审内容，清楚追踪每一次修改。</p>
            </div>
            <button :disabled="actionsBlocked" @click="conflictOpen = true">处理合并冲突</button>
          </div>
          <ReviewPanel
            v-if="user"
            :key="props.id"
            :document="props.id"
            :actor="user.id"
            :managing="managing"
            :can-comment="commenting"
            :space="document.space_id"
            :initial-request="initialReview"
            :can-reverse="canRepair"
            :refresh="reviewRefresh"
            :prepare-merge="prepareStructure"
            :reload-merge="reloadMerge"
            :reload-reverse="reloadReverse"
            :open-conflict="openConflict"
          />
        </div>
        <div
          v-show="pane === 'comments'"
          id="pane-comments"
          class="panel document-section"
          role="tabpanel"
          aria-labelledby="tab-comments"
        >
          <h2>评论</h2>
          <p class="hint">集中查看本文档的讨论；阅读时也可以在“正文”中展开评论侧栏。</p>
          <div id="main-comment-content"></div>
        </div>
        <!-- 同一评论实例在两个入口间移动，避免重复请求及切换时丢失未提交的草稿/回复。 -->
        <Teleport
          :to="
            pane === 'read' && readingCommentsOpen
              ? '#reading-comment-content'
              : '#main-comment-content'
          "
        >
          <section v-if="user && commentVersion" aria-label="文档主线讨论">
            <CommentsPanel
              ref="commentsPanel"
              compact
              :visible="pane === 'comments' || (pane === 'read' && readingCommentsOpen)"
              :locations="commentLocations"
              :document="props.id"
              context-kind="MAIN"
              :context-id="main.manifest_id"
              :version="commentVersion"
              :anchor="commentAnchor"
              @clear-anchor="commentAnchor = null"
              @open-anchor="openCommentAnchor"
              @locate-anchor="locateCommentAnchor"
              @threads-changed="mainCommentThreads = $event"
              locatable
              :actor="user.id"
              :can-comment="commenting"
              :can-manage="managing"
              browse-main-history
              :space="document.space_id"
            >
              <template #creation-context>
                <label
                  >评论对应章节
                  <select
                    :value="
                      commentUnits.some((entry) => entry.unit_id === commentUnit)
                        ? commentUnit
                        : commentUnits[0]?.unit_id
                    "
                    :disabled="actionsBlocked"
                    @change="commentUnit = ($event.target as HTMLSelectElement).value"
                  >
                    <option
                      v-for="entry in commentUnits"
                      :key="entry.node_id"
                      :value="entry.unit_id"
                    >
                      {{ entry.title }}
                    </option>
                  </select>
                </label>
              </template>
            </CommentsPanel>
          </section>
        </Teleport>
        <section v-show="pane === 'review' && conflictOpen" class="panel document-section">
          <div class="section-title">
            <h2>处理合并冲突</h2>
            <button :disabled="actionsBlocked" @click="conflictOpen = false">返回审核列表</button>
          </div>
          <ConflictWorkbench
            ref="conflictWorkbench"
            :document="props.id"
            :can-edit="canRepair"
            :prepare="prepareStructure"
            :reload="reloadResolution"
          />
        </section>
        <div
          v-if="pane === 'publication'"
          id="pane-publication"
          role="tabpanel"
          aria-labelledby="tab-publication"
        >
          <PublicationPanel
            :document="document"
            :structure="main.structure"
            :managing="managing"
            :reload="reloadPublication"
            :prepare="prepareMainLock"
          />
        </div>
        <div
          v-if="pane === 'settings' && managing"
          id="pane-settings"
          class="panel document-section"
          role="tabpanel"
          aria-labelledby="tab-settings"
        >
          <h2>文档设置</h2>
          <!-- 先确认文档身份与受众，再操作目录结构，避免用户进入设置即被拆合操作打断。 -->
          <section v-if="managing" class="management">
            <h3>文档信息</h3>
            <div>
              <section>
                <form class="inline-form" @submit.prevent="rename">
                  <fieldset :disabled="actionsBlocked">
                    <label>新的文档标题<input v-model="title" required maxlength="240" /></label
                    ><button :disabled="actionsBlocked">更新文档标题</button>
                  </fieldset>
                </form>
                <form @submit.prevent="changeVisibility">
                  <fieldset :disabled="actionsBlocked">
                    <label
                      >文档正式版受众上限<select v-model="policy">
                        <option value="COLLABORATORS">协作者</option>
                        <option value="SPACE_MEMBERS">空间成员</option>
                        <option value="PUBLIC">公开</option>
                      </select></label
                    >
                    <p class="hint">
                      与知识库范围取交集。设为公开不等于发布，也不授予内部主线读取权。
                    </p>
                    <button :disabled="actionsBlocked">更新文档范围</button>
                  </fieldset>
                </form>
              </section>
            </div>
          </section>
          <StructureControls
            v-if="managing"
            :view="main"
            :reload="reloadMainControl"
            :prepare="prepareStructure"
          />
          <MainControls
            :document="document"
            :reload="reloadMainControl"
            :prepare-lock="prepareMainLock"
          />
          <MainStyleControls :view="main" :reload="reloadMainControl" :prepare="prepareMainLock" />
        </div>
      </template>
    </template>
    <AppDialog v-model="trashDialogOpen" title="移入回收站" :blocked="actionsBlocked">
      <p>将「{{ document?.title ?? '此文档' }}」移入回收站？</p>
      <p class="hint">正式版将停止访问，正文与历史在保留期内可以从所在空间的回收站恢复。</p>
      <div class="button-row">
        <button type="button" :disabled="actionsBlocked" @click="trashDialogOpen = false">
          取消
        </button>
        <button
          type="button"
          class="danger"
          :disabled="actionsBlocked || !document || !managing"
          @click="remove"
        >
          确认移入回收站
        </button>
      </div>
      <ActionStatus
        :busy="busy"
        :error="error"
        :notice="notice"
        :pending="pending"
        :retry="retry"
      />
    </AppDialog>
  </section>
</template>

<style scoped>
.document-more-actions :deep(.popover-panel) {
  left: auto;
  right: 0;
  width: min(260px, calc(100vw - 64px));
}
.document-more-actions :deep(.popover-panel > button) {
  width: 100%;
  text-align: left;
}
@media (max-width: 760px) {
  .document-more-actions :deep(.popover-panel) {
    left: 16px;
    right: 16px;
    width: auto;
  }
}
.editor-chapter + .editor-chapter {
  margin-top: 1.5rem;
}
.reading-unit + .reading-unit {
  margin-top: 2rem;
}
.reading-section-title {
  font-size: 1.3em;
  margin: 0 0 1rem;
}
.document-outline ol {
  display: block;
}
.document-outline ol ol {
  margin: 0.5rem 0;
  padding-left: 1.5rem;
}
.reading-comment-tools {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 12px;
}
.reading-comment-tools .hint {
  margin: 0;
}
.reading-layout.with-comments {
  grid-template-columns: minmax(0, 1fr) minmax(300px, 360px);
  gap: 24px;
}
.with-comments > .document-outline {
  grid-column: 1 / -1;
  position: static;
  padding: 0 0 12px;
  border-right: 0;
  border-bottom: 1px solid var(--line);
}
.reading-comment-sidebar {
  position: sticky;
  top: 26px;
  max-height: calc(100vh - 52px);
  min-width: 0;
  overflow: auto;
  overflow-wrap: anywhere;
  padding: 18px;
  border: 1px solid var(--line);
  border-radius: 7px;
  background: var(--paper);
  font-family: var(--ui-font-family, sans-serif);
  font-size: var(--ui-font-control);
}
.reading-comment-sidebar .section-title {
  flex-wrap: wrap;
  gap: 10px;
}
.reading-comment-sidebar :deep(.comments-panel) {
  margin-top: 16px;
}
.reading-comment-sidebar :deep(select) {
  max-width: 100%;
}
@media (max-width: 1100px) {
  /* 窄屏把评论放在正文上方独立滚动，不把A4阅读区压成窄列。 */
  .reading-layout.with-comments {
    grid-template-columns: minmax(0, 1fr);
  }
  .with-comments > .document-outline {
    grid-row: 1;
  }
  .with-comments > .reading-comment-sidebar {
    position: static;
    grid-row: 2;
    max-height: 55vh;
  }
  .with-comments > .paper {
    grid-row: 3;
  }
}
.review-conflict-disclosure {
  margin: 24px 0;
  padding: 16px 18px;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--paper);
}
.review-conflict-disclosure > summary {
  cursor: pointer;
  font-weight: 600;
}
.review-conflict-disclosure > summary .hint {
  margin-left: 10px;
  font-weight: 400;
}
@media (max-width: 520px) {
  .review-conflict-disclosure > summary .hint {
    display: block;
    margin: 4px 0 0;
  }
}
</style>
