<script setup lang="ts">
import {
  ApiError,
  UserFacingError,
  api,
  useAction,
  actionsBlocked,
  transportEpoch,
} from '../../lib/http';
import {
  ref,
  reactive,
  computed,
  watch,
  onMounted,
  onUnmounted,
  defineAsyncComponent,
  nextTick,
} from 'vue';
import { createCursorPage, pageCursor, acceptPage, resetPage } from '../../lib/pagination';
import type {
  Space,
  SpacePage,
  OrganizationPage,
  KnowledgeBase,
  KnowledgeBasePage,
  Document,
  DocumentPage,
  DocumentApplicationPage,
  DocumentApplication,
} from '../../../../contracts/http/generated';
import ActionStatus from '../../components/ActionStatus.vue';
import AppIcon from '../../components/AppIcon.vue';
import AppDialog from '../../components/AppDialog.vue';
import PaginationControls from '../../components/PaginationControls.vue';
import { documentPath, workspaceRoute, workspacePath, type WorkspaceSection } from './navigation';
const KnowledgeBasePanel = defineAsyncComponent(() => import('./KnowledgeBasePanel.vue'));
const TrashPanel = defineAsyncComponent(() => import('../trash/TrashPanel.vue'));
const props = defineProps<{ organization?: { id: string; name: string } }>();
const emit = defineEmits<{ search: [space: string]; open: [document: string] }>();
let active = true,
  directoryGeneration = 0;
const current = (generation: number) => active && generation === directoryGeneration;
const initialLocation = workspaceRoute(location.search, props.organization?.id);
const section = ref<WorkspaceSection>(initialLocation.section);
const createVisible = ref(false),
  applicationVisible = ref(location.pathname === '/join-document');
const createKind = ref<'document' | 'knowledgeBase'>('document');
const libraryHome = computed(() => section.value === 'libraries');
const kbQuery = ref('');
const kbFilter = ref('');
const kbPage = reactive(createCursorPage()),
  docPage = reactive(createCursorPage()),
  appPage = reactive(createCursorPage()),
  kbLoading = ref(false),
  docLoading = ref(false),
  appLoading = ref(false);
let kbRequest = 0,
  docRequest = 0,
  appRequest = 0;
const spaces = ref<Space[]>([]),
  spaceKind = ref<Space['kind']>(props.organization ? 'ORGANIZATION' : 'PERSONAL'),
  spaceCursor = ref<string | null>(null),
  orgNames = ref<Record<string, string>>({}),
  spaceId = ref(initialLocation.space),
  kbs = ref<KnowledgeBase[]>([]),
  kbId = ref(initialLocation.knowledgeBase),
  currentKB = ref<KnowledgeBase | null>(null),
  kbActions = ref<string[]>([]),
  docs = ref<Document[]>([]),
  kbName = ref(''),
  docTitle = ref(''),
  docTitleInput = ref<HTMLInputElement | null>(null),
  inviteToken = ref(
    location.pathname === '/join-document'
      ? (new URLSearchParams(location.search).get('invitation_token') ?? '')
      : '',
  ),
  reason = ref(''),
  ownApps = ref<DocumentApplication[]>([]),
  action = useAction();
const { busy, error, notice, pending, retry, mutate } = action;
const space = computed(() => spaces.value.find((s) => s.id === spaceId.value)),
  canCreateDoc = computed(() =>
    kbId.value
      ? currentKB.value?.id === kbId.value &&
        currentKB.value.space_id === spaceId.value &&
        kbActions.value.includes('CREATE_DOCUMENT')
      : space.value?.kind === 'PERSONAL' && space.value.actions.includes('CREATE_DOCUMENT'),
  );
const organizationSpaces = computed(() => spaces.value.filter((s) => s.kind === 'ORGANIZATION'));
const spaceLabel = computed(() => {
  if (props.organization) return '组织空间 · ' + props.organization.name;
  if (space.value?.kind === 'PERSONAL') return '个人空间';
  if (space.value) return '组织空间 · ' + spaceName(space.value);
  return spaceKind.value === 'ORGANIZATION' ? '组织空间' : '个人空间';
});
const canCreate = computed(
  () => space.value?.actions.includes('CREATE_KNOWLEDGE_BASE') || canCreateDoc.value,
);
const canManageKB = computed(() =>
  kbActions.value.some((action) =>
    ['RENAME', 'DELETE', 'CHANGE_VISIBILITY', 'CHANGE_CREATION_GRANTS', 'CHANGE_ROLES'].includes(
      action,
    ),
  ),
);
async function openLibrary(id: string) {
  if (actionsBlocked.value) return;
  kbId.value = id;
  section.value = 'documents';
  createVisible.value = false;
  await action.run(changeKB, '');
  synchronizeLocation();
}
function showLibraries() {
  // 返回目录立即清除上个库的动作和文档；封面只来自已授权列表，点击后仍重读真实资格。
  directoryGeneration++;
  section.value = 'libraries';
  kbId.value = '';
  currentKB.value = null;
  kbActions.value = [];
  clearDocs();
  clearOwnApps();
  createVisible.value = false;
  synchronizeLocation();
}
function coverTone(id: string) {
  // 稳定的书籍封面配色无需逐库预取文件，不伪造后端尚未提供的封面上传能力。
  return Number(BigInt(id) % 5n);
}
function spaceName(s: Space) {
  return s.kind === 'PERSONAL'
    ? '个人空间'
    : (orgNames.value[s.organization_id ?? ''] ?? '组织 ' + s.organization_id);
}
async function listSpaces(append = false) {
  const generation = directoryGeneration;
  if (props.organization) {
    spaces.value = [];
    spaceId.value = '';
    spaceCursor.value = null;
    // 组织详情只解析所选组织的有权空间，遇到目标便停止；不因个人空间排在首页就显示个人文档。
    // 这里只续读小页空间描述，不预取其它空间的知识库、文档或正文。
    let cursor: string | null = null;
    const visited = new Set<string>();
    do {
      const page: SpacePage = await api<SpacePage>(
        '/v1/me/spaces?limit=20' + (cursor ? '&cursor=' + encodeURIComponent(cursor) : ''),
      );
      if (!current(generation)) return;
      const matching = page.items.find(
        (item) => item.kind === 'ORGANIZATION' && item.organization_id === props.organization!.id,
      );
      if (matching) {
        spaces.value = [matching];
        spaceId.value = matching.id;
        spaceKind.value = 'ORGANIZATION';
        return;
      }
      cursor = page.next_cursor;
      if (cursor && visited.has(cursor))
        throw new UserFacingError('空间目录游标未前进，请刷新后重试。');
      if (cursor) visited.add(cursor);
    } while (cursor);
    return;
  }
  const p = await api<SpacePage>(
    '/v1/me/spaces?limit=20' +
      (append && spaceCursor.value ? '&cursor=' + encodeURIComponent(spaceCursor.value) : ''),
  );
  if (!current(generation)) return;
  spaces.value = append ? [...spaces.value, ...p.items] : p.items;
  spaceCursor.value = p.next_cursor;
  // 刷新地址中的空间可能位于后页；只在当前有权目录中解析，不直接信任URL编号。
  if (!append && spaceId.value && !spaces.value.some((item) => item.id === spaceId.value)) {
    const visited = new Set<string>();
    while (spaceCursor.value && !spaces.value.some((item) => item.id === spaceId.value)) {
      const cursor = spaceCursor.value;
      if (visited.has(cursor)) throw new UserFacingError('空间目录游标未前进，请重新读取。');
      visited.add(cursor);
      const next = await api<SpacePage>(
        '/v1/me/spaces?limit=20&cursor=' + encodeURIComponent(cursor),
      );
      if (!current(generation)) return;
      spaces.value.push(...next.items);
      spaceCursor.value = next.next_cursor;
    }
  }
  // 类型选择只划分已授权目录，不能在组织空间不可用时悄悄回退到个人数据。
  // URL恢复先核空间真实类型；后页尚未加载与确实没有组织空间分别提示。
  if (!spaceId.value || !spaces.value.some((s) => s.id === spaceId.value))
    spaceId.value = spaces.value.find((s) => s.kind === spaceKind.value)?.id ?? '';
  if (space.value) spaceKind.value = space.value.kind;
  if (spaceKind.value === 'ORGANIZATION' && section.value === 'applications')
    section.value = 'libraries';
  // 名称来自账号服务的当前组织目录；不靠跨库SQL，也不把用户提交的角色当权威。
  let cursor: string | null = null;
  const names: Record<string, string> = {};
  for (let n = 0; n < 100; n++) {
    const p: OrganizationPage = await api<OrganizationPage>(
      '/v1/organizations?limit=100' + (cursor ? '&cursor=' + encodeURIComponent(cursor) : ''),
    );
    if (!current(generation)) return;
    for (const o of p.items) names[o.id] = o.name;
    if (!p.has_more) break;
    cursor = p.next_cursor;
    if (n === 99) throw new UserFacingError('组织列表较大，请选择具体组织后继续。');
  }
  orgNames.value = names;
}
async function listKBs(index = 0) {
  if (!spaceId.value) return;
  const generation = directoryGeneration,
    requestedSpace = spaceId.value,
    filter = kbFilter.value,
    cursor = pageCursor(kbPage, index),
    request = ++kbRequest;
  const valid = () =>
    current(generation) &&
    request === kbRequest &&
    requestedSpace === spaceId.value &&
    filter === kbFilter.value;
  kbLoading.value = true;
  kbs.value = [];
  try {
    const p = await api<KnowledgeBasePage>(
      '/v1/spaces/' +
        requestedSpace +
        '/knowledge-bases?limit=12' +
        (cursor ? '&cursor=' + encodeURIComponent(cursor) : '') +
        (filter ? '&q=' + encodeURIComponent(filter) : ''),
    );
    if (!valid()) return;
    kbs.value = p.items;
    acceptPage(kbPage, index, cursor, p.next_cursor);
  } catch (failure) {
    if (!valid()) return;
    clearDeniedDirectory(failure, generation);
    throw failure;
  } finally {
    if (request === kbRequest) kbLoading.value = false;
  }
}
async function searchKBs() {
  // 搜索由服务端在授权目录分页前执行；修改条件不能沿用旧查询的游标。
  kbFilter.value = kbQuery.value.trim();
  clearKBs();
  await listKBs();
}
async function listDocs(index = 0) {
  if (!spaceId.value) return;
  const generation = directoryGeneration,
    requestedSpace = spaceId.value,
    requestedKB = kbId.value,
    cursor = pageCursor(docPage, index),
    request = ++docRequest;
  const base = requestedKB ? '/v1/knowledge-bases/' + requestedKB : '/v1/spaces/' + requestedSpace;
  const valid = () =>
    current(generation) &&
    request === docRequest &&
    requestedSpace === spaceId.value &&
    requestedKB === kbId.value;
  docLoading.value = true;
  docs.value = [];
  try {
    const p = await api<DocumentPage>(
      base + '/documents?limit=20' + (cursor ? '&cursor=' + encodeURIComponent(cursor) : ''),
    );
    if (!valid()) return;
    docs.value = p.items;
    acceptPage(docPage, index, cursor, p.next_cursor);
  } catch (failure) {
    if (!valid()) return;
    clearDeniedDirectory(failure, generation);
    throw failure;
  } finally {
    if (request === docRequest) docLoading.value = false;
  }
}
function clearKBs() {
  kbRequest++;
  kbs.value = [];
  kbLoading.value = false;
  resetPage(kbPage);
}
function clearDocs() {
  docRequest++;
  docs.value = [];
  docLoading.value = false;
  resetPage(docPage);
}
function clearOwnApps() {
  appRequest++;
  ownApps.value = [];
  appLoading.value = false;
  resetPage(appPage);
}
function clearDirectory() {
  currentKB.value = null;
  kbActions.value = [];
  clearKBs();
  clearDocs();
}
function clearDeniedDirectory(failure: unknown, generation: number) {
  if (
    current(generation) &&
    failure instanceof ApiError &&
    [401, 403, 404, 410].includes(failure.status)
  ) {
    // 撤权不能保留旧标题或创建动作，让目录看起来仍然可用；短暂断网不等同于已失去资格。
    clearDirectory();
    clearOwnApps();
    section.value = 'libraries';
    createVisible.value = false;
    kbId.value = '';
    spaceId.value = '';
    spaces.value = [];
    spaceCursor.value = null;
  }
}
async function listOwnApps(index = 0) {
  const generation = directoryGeneration,
    requestedSpace = spaceId.value,
    cursor = pageCursor(appPage, index),
    request = ++appRequest;
  const valid = () =>
    current(generation) && request === appRequest && requestedSpace === spaceId.value;
  appLoading.value = true;
  ownApps.value = [];
  try {
    const p = await api<DocumentApplicationPage>(
      '/v1/document-applications?limit=20' +
        (cursor ? '&cursor=' + encodeURIComponent(cursor) : ''),
    );
    if (!valid()) return;
    ownApps.value = p.items;
    acceptPage(appPage, index, cursor, p.next_cursor);
  } catch (failure) {
    if (!valid()) return;
    if (failure instanceof ApiError && [401, 403, 404, 410].includes(failure.status))
      clearOwnApps();
    throw failure;
  } finally {
    if (request === appRequest) appLoading.value = false;
  }
}
// 父范围变化时先丢弃旧目录和动作摘要；旧空间的文档不能在新空间查询失败后继续冒充有效结果。
async function changeSpace() {
  const generation = ++directoryGeneration;
  kbId.value = '';
  section.value = 'libraries';
  kbQuery.value = '';
  kbFilter.value = '';
  createVisible.value = false;
  clearDirectory();
  clearOwnApps();
  // 选择来自已授权目录，先记录新位置，避免读请求未结束就刷新时又恢复到旧空间。
  // 地址只恢复选择，知识库内容和创建资格仍必须重新读取，不能由URL授予权限。
  synchronizeLocation();
  await listKBs();
  if (!current(generation)) return;
}
async function selectSpaceKind(kind: Space['kind']) {
  if (actionsBlocked.value || kind === spaceKind.value) return;
  spaceKind.value = kind;
  spaceId.value = spaces.value.find((item) => item.kind === kind)?.id ?? '';
  await action.run(async () => {
    await changeSpace();
    synchronizeLocation();
  }, '');
}
async function moreSpaces() {
  const previousSpace = spaceId.value;
  await action.run(async () => {
    await listSpaces(true);
    // 当前类型的首个空间可能才出现在后页；选中后再读它自己的知识库。
    if (previousSpace !== spaceId.value) await changeSpace();
    synchronizeLocation();
  }, '');
}
async function changeKB() {
  const generation = ++directoryGeneration,
    requestedSpace = spaceId.value,
    requestedKB = kbId.value;
  kbActions.value = [];
  clearDocs();
  currentKB.value = null;
  if (requestedKB) {
    let available: string[] = [];
    try {
      const detail = await api<KnowledgeBase>(
        '/v1/knowledge-bases/' + requestedKB,
        'GET',
        undefined,
        (h) => {
          try {
            available = JSON.parse(h.get('X-Available-Actions') ?? '[]');
          } catch {
            available = [];
          }
        },
      );
      if (!current(generation)) return;
      // 名称用于选择，真实归属和创建资格来自当前知识库详情，不能从组织所有者身份推导。
      if (detail.space_id !== requestedSpace)
        throw new UserFacingError('知识库不属于当前空间，请刷新目录后重新选择。');
      currentKB.value = detail;
      kbActions.value = available;
      // 当前库与当前卡片页独立保存；地址可恢复后页库，但不能把它混进首页网格。
    } catch (failure) {
      clearDeniedDirectory(failure, generation);
      throw failure;
    }
  }
  if (!current(generation)) return;
  if (section.value === 'settings' && !canManageKB.value) section.value = 'documents';
  if (section.value === 'documents') await listDocs();
}
async function refresh() {
  const generation = ++directoryGeneration,
    previousSpace = spaceId.value,
    previousKB = kbId.value;
  clearDirectory();
  clearOwnApps();
  await listSpaces();
  if (!current(generation)) return;
  await Promise.all([
    listKBs(),
    ...(section.value === 'applications' && !props.organization ? [listOwnApps()] : []),
  ]);
  if (!current(generation)) return;
  // 刷新时已选库可能来自第二页；仍核验它的详情，不能仅因首页没列出便悄悄切回全部文档。
  kbId.value = previousSpace === spaceId.value ? previousKB : '';
  await changeKB();
  synchronizeLocation(true);
}
function onRefresh() {
  void action.run(refresh, props.organization ? '组织内容已刷新' : '工作台已刷新');
}
onMounted(() => {
  void action.run(refresh, '');
  window.addEventListener('collab-refresh', onRefresh);
  window.addEventListener('popstate', restoreLocation);
});
onUnmounted(() => {
  active = false;
  directoryGeneration++;
  window.removeEventListener('collab-refresh', onRefresh);
  window.removeEventListener('popstate', restoreLocation);
});
watch(
  [() => props.organization?.id, transportEpoch],
  () => {
    // 换账号或组织立即释放旧页与游标；迟到HTTP结果不能回填新会话的工作台。
    directoryGeneration++;
    clearDirectory();
    clearOwnApps();
    kbQuery.value = '';
    kbFilter.value = '';
    kbId.value = '';
    spaceId.value = '';
    spaces.value = [];
    spaceCursor.value = null;
    orgNames.value = {};
    section.value = 'libraries';
    createVisible.value = false;
    applicationVisible.value = false;
  },
  { flush: 'sync' },
);
async function createKB() {
  const destination = spaceId.value,
    name = kbName.value;
  await action.run(async () => {
    const result = await mutate(
      () => '/v1/spaces/' + destination + '/knowledge-bases',
      () => ({ name }),
    );
    kbName.value = '';
    await listKBs();
    if ('result' in result && result.result?.entity_id) {
      kbId.value = result.result.entity_id;
      section.value = 'documents';
      await changeKB();
    }
    createVisible.value = false;
    synchronizeLocation();
  }, '知识库已创建');
}
// 等到初始化事务成功才刷新目录；ACTIVE意味着结构、样式、清单和正文已原子建好。
async function createDoc() {
  if (!canCreateDoc.value) return;
  // 首次点击固定目标；同号续接只能核验原归属，不能因当前目录变化把新文档切到另一库。
  const destination = { space: spaceId.value, knowledgeBase: kbId.value };
  const title = docTitle.value;
  await action.run(async () => {
    const result = await mutate(
      () => '/v1/documents',
      () => ({
        space_id: destination.space,
        ...(destination.knowledgeBase ? { knowledge_base_id: destination.knowledgeBase } : {}),
        title,
      }),
    );
    const id = 'result' in result ? result.result?.entity_id : undefined;
    if (!id) throw new UserFacingError('文档已登记，创建结果尚未返回文档编号');
    const created = await api<Document>('/v1/documents/' + id);
    if (
      created.lifecycle !== 'ACTIVE' ||
      created.space_id !== destination.space ||
      (created.knowledge_base_id ?? '') !== destination.knowledgeBase
    )
      throw new UserFacingError('新文档归属核验未通过，请续接原操作并刷新目录');
    // 新对象可能排在目录首页之外，直接使用已核验编号打开，不依赖列表分页找到它。
    docTitle.value = '';
    await listDocs();
    createVisible.value = false;
    emit('open', id);
  }, '完整文档主线、结构与样式已初始化');
}
async function focusNewDocument() {
  createKind.value = 'document';
  createVisible.value = true;
  await nextTick();
  docTitleInput.value?.scrollIntoView({ behavior: 'smooth', block: 'center' });
  docTitleInput.value?.focus({ preventScroll: true });
}
async function applyDocument() {
  await action.run(async () => {
    await mutate(
      () => '/v1/document-invitations/applications',
      () => ({ invitation_token: inviteToken.value, reason: reason.value }),
    );
    inviteToken.value = '';
    section.value = 'applications';
    await listOwnApps();
    applicationVisible.value = false;
    synchronizeLocation(true);
  }, '文档申请已提交，审批前不能访问正文');
}
function synchronizeLocation(replace = false) {
  if (!active) return;
  const path = workspacePath(
    { section: section.value, space: spaceId.value, knowledgeBase: kbId.value },
    props.organization?.id,
  );
  if (location.pathname + location.search === path) return;
  if (replace) history.replaceState(null, '', path);
  else history.pushState(null, '', path);
}
async function selectSection(next: WorkspaceSection) {
  if (actionsBlocked.value) return;
  if (next === 'settings') {
    if (!kbId.value || !canManageKB.value) return;
    section.value = next;
    synchronizeLocation();
    return;
  }
  directoryGeneration++;
  section.value = next;
  createVisible.value = false;
  kbId.value = '';
  currentKB.value = null;
  kbActions.value = [];
  clearDocs();
  clearOwnApps();
  synchronizeLocation();
  if (next === 'documents') await action.run(() => listDocs(), '');
  if (next === 'applications' && !props.organization) await action.run(() => listOwnApps(), '');
}
function showCreation() {
  createKind.value = canCreateDoc.value ? 'document' : 'knowledgeBase';
  createVisible.value = true;
}
function restoreLocation() {
  // App与目录各恢复自己负责的地址；离开本工作区的历史事件不发旧范围请求。
  if (location.pathname !== (props.organization ? '/organizations' : '/')) return;
  if (
    (new URLSearchParams(location.search).get('organization_id') ?? '') !==
    (props.organization?.id ?? '')
  )
    return;
  if (actionsBlocked.value) {
    synchronizeLocation(true);
    return;
  }
  const target = workspaceRoute(location.search, props.organization?.id);
  kbQuery.value = '';
  kbFilter.value = '';
  section.value = target.section;
  spaceId.value = target.space;
  kbId.value = target.knowledgeBase;
  createVisible.value = false;
  applicationVisible.value = false;
  void action.run(refresh, '');
}
const contentTitle = computed(() =>
  section.value === 'libraries'
    ? '全部知识库'
    : section.value === 'applications'
      ? '协作申请'
      : section.value === 'trash'
        ? '回收站'
        : section.value === 'settings'
          ? '知识库设置'
          : (currentKB.value?.name ?? '全部文档'),
);
function kbDeleted() {
  directoryGeneration++;
  section.value = 'libraries';
  kbId.value = '';
  currentKB.value = null;
  kbActions.value = [];
  clearDocs();
  createVisible.value = false;
  void action.run(async () => {
    await listKBs();
    synchronizeLocation(true);
  }, '知识库已关闭');
}
</script>
<template>
  <div class="workspace-shell" :class="{ 'workspace-embedded': organization }">
    <aside class="workspace-sidebar">
      <div class="workspace-identity">
        <span class="workspace-monogram"><AppIcon name="page" /></span>
        <div>
          <strong>{{ organization?.name ?? '我的空间' }}</strong
          ><small>整理知识，协同写作</small>
        </div>
      </div>
      <section v-if="!organization" class="workspace-scope-picker" aria-label="空间切换">
        <div class="workspace-scope" role="group" aria-label="空间类型">
          <button
            type="button"
            :aria-pressed="spaceKind === 'PERSONAL'"
            :disabled="actionsBlocked"
            @click="selectSpaceKind('PERSONAL')"
          >
            个人空间
          </button>
          <button
            type="button"
            :aria-pressed="spaceKind === 'ORGANIZATION'"
            :disabled="actionsBlocked"
            @click="selectSpaceKind('ORGANIZATION')"
          >
            组织空间
          </button>
        </div>
        <label v-if="spaceKind === 'ORGANIZATION'" class="workspace-space"
          >组织
          <select
            v-model="spaceId"
            :disabled="actionsBlocked || !organizationSpaces.length"
            aria-label="组织"
            @change="
              action.run(async () => {
                await changeSpace();
                synchronizeLocation();
              }, '')
            "
          >
            <option v-if="!spaceId" value="">
              {{ spaceCursor ? '请继续加载组织空间' : '暂无可选组织' }}
            </option>
            <option v-for="s in organizationSpaces" :key="s.id" :value="s.id">
              {{ spaceName(s) }}
            </option>
          </select>
        </label>
        <button
          v-if="spaceCursor && spaceKind === 'ORGANIZATION'"
          class="text-action"
          :disabled="actionsBlocked"
          @click="moreSpaces"
        >
          更多组织空间
        </button>
      </section>
      <nav aria-label="工作空间导航" class="workspace-navigation">
        <button
          :aria-current="libraryHome || !!kbId ? 'page' : undefined"
          :disabled="actionsBlocked"
          @click="showLibraries"
        >
          <AppIcon name="page" />知识库
        </button>
        <button
          :aria-current="section === 'documents' && !kbId ? 'page' : undefined"
          :disabled="actionsBlocked"
          @click="selectSection('documents')"
        >
          <AppIcon name="page" />全部文档
        </button>
        <button
          v-if="!organization && spaceKind === 'PERSONAL'"
          :aria-current="section === 'applications' ? 'page' : undefined"
          :disabled="actionsBlocked"
          @click="selectSection('applications')"
        >
          <AppIcon name="people" />协作申请
        </button>
        <button
          :aria-current="section === 'trash' ? 'page' : undefined"
          :disabled="actionsBlocked || !spaceId"
          @click="selectSection('trash')"
        >
          <AppIcon name="trash" />回收站
        </button>
      </nav>
      <button
        class="workspace-search"
        :disabled="actionsBlocked || !spaceId"
        @click="emit('search', spaceId)"
      >
        <AppIcon name="search" />{{ organization ? '搜索当前组织' : '搜索当前空间' }}
      </button>
    </aside>
    <div class="workspace-content">
      <header class="workspace-content-heading">
        <div>
          <nav class="library-breadcrumb" aria-label="知识库位置">
            <button
              v-if="kbId"
              class="text-action"
              :disabled="actionsBlocked"
              @click="showLibraries"
            >
              返回全部知识库
            </button>
            <span v-if="kbId" aria-hidden="true">/</span><span>{{ spaceLabel }}</span>
            <span v-if="kbId" aria-hidden="true">/</span
            ><span v-if="kbId">{{ currentKB?.name }}</span>
          </nav>
          <h1>{{ contentTitle }}</h1>
          <p class="hint">
            {{
              section === 'libraries'
                ? '从一本知识库，开始今天的阅读与创作。'
                : section === 'applications'
                  ? '查看你发起的个人文档协作申请。'
                  : section === 'trash'
                    ? '查找和恢复当前空间中你有权管理的已删除文档。'
                    : section === 'settings'
                      ? '管理知识库信息、成员和访问范围。'
                      : '选择一篇文档，继续你的工作。'
            }}
          </p>
        </div>
        <div class="page-actions">
          <button :disabled="actionsBlocked" @click="onRefresh">
            {{ busy ? '刷新中…' : organization ? '刷新组织内容' : '刷新工作台' }}
          </button>
          <button
            v-if="section === 'settings'"
            :disabled="actionsBlocked"
            @click="
              section = 'documents';
              synchronizeLocation();
            "
          >
            返回文档目录
          </button>
          <button
            v-else-if="kbId && canManageKB"
            :disabled="actionsBlocked"
            @click="selectSection('settings')"
          >
            <AppIcon name="settings" />知识库设置
          </button>
          <button
            v-if="canCreate && ['libraries', 'documents'].includes(section)"
            class="primary"
            :disabled="actionsBlocked || !spaceId"
            @click="showCreation"
          >
            <AppIcon name="plus" />新建
          </button>
          <button
            v-if="section === 'applications'"
            class="primary"
            :disabled="actionsBlocked"
            @click="applicationVisible = true"
          >
            <AppIcon name="plus" />申请文档协作
          </button>
        </div>
      </header>
      <ActionStatus
        v-if="!pending && !createVisible && !applicationVisible"
        :busy="busy"
        :error="error"
        :notice="notice"
        :pending="pending"
        :retry="retry"
      />
      <section
        v-if="libraryHome && spaceId"
        class="library-gallery"
        aria-label="知识库目录"
        :aria-busy="busy || kbLoading"
      >
        <div class="section-title library-tools">
          <span class="count-label">本页 {{ kbs.length }} 个知识库</span>
          <form class="inline-form" role="search" @submit.prevent="action.run(searchKBs, '')">
            <label class="library-search"
              ><AppIcon name="search" /><input
                v-model="kbQuery"
                type="search"
                aria-label="搜索知识库"
                placeholder="搜索全部知识库"
                maxlength="100"
                :disabled="actionsBlocked || kbLoading"
            /></label>
            <button :disabled="actionsBlocked || kbLoading">搜索</button>
          </form>
        </div>
        <ul class="library-grid">
          <li v-for="library in kbs" :key="library.id">
            <button
              class="library-book"
              :class="'cover-tone-' + coverTone(library.id)"
              :aria-label="'打开知识库：' + library.name"
              :disabled="actionsBlocked"
              @click="openLibrary(library.id)"
            >
              <span class="book-kicker">KNOWLEDGE / 知识库</span
              ><span class="book-title">{{ library.name }}</span>
              <span class="book-art" aria-hidden="true"><AppIcon name="page" /></span
              ><span class="book-open">打开知识库<AppIcon name="arrow" /></span>
            </button>
          </li>
        </ul>
        <p v-if="!kbs.length && !busy && !kbLoading" class="empty">
          {{ kbFilter ? '没有匹配的知识库。' : '这里还没有你可访问的知识库。' }}
        </p>
        <PaginationControls
          label="知识库"
          :page="kbPage.index + 1"
          :has-next="!!kbPage.next"
          :busy="actionsBlocked || kbLoading"
          @change="(index) => action.run(() => listKBs(index), '')"
        />
      </section>
      <p v-if="!spaceId && !busy && !error" class="empty">
        {{
          organization
            ? '当前组织暂无你可访问的内容空间，请联系组织管理员。'
            : spaceKind === 'ORGANIZATION'
              ? spaceCursor
                ? '当前页尚未加载到组织空间，请继续加载更多组织空间。'
                : '你还没有可访问的组织空间，可在组织页面创建或申请加入组织。'
              : '暂无可访问的个人空间，请稍后重新读取。'
        }}
      </p>
      <KnowledgeBasePanel
        v-if="section === 'settings' && kbId && canManageKB"
        :key="kbId"
        :id="kbId"
        :organization="space?.organization_id"
        @create-document="focusNewDocument"
        @changed="
          action.run(async () => {
            await listKBs();
            await changeKB();
          }, '')
        "
        @deleted="kbDeleted"
      />
      <section
        v-if="section === 'documents'"
        class="directory-panel"
        :aria-busy="busy || docLoading"
      >
        <div class="section-title">
          <span class="count-label">{{
            busy || docLoading ? '正在读取…' : '本页 ' + docs.length + ' 篇文档'
          }}</span>
        </div>
        <ul class="entity-list document-cards">
          <li v-for="d in docs" :key="d.id" class="document-card">
            <a
              class="entity-button"
              :href="documentPath(d.id)"
              @click.prevent="!actionsBlocked && emit('open', d.id)"
            >
              <span class="card-top"
                ><span class="card-icon"><AppIcon name="page" /></span
                ><span
                  class="status-pill"
                  :class="{ 'status-draft': d.publication_state !== 'PUBLISHED' }"
                  >{{ d.publication_state === 'PUBLISHED' ? '已发布' : '未发布' }}</span
                ></span
              >
              <span class="card-title">{{ d.title }}</span
              ><span class="card-bottom"
                ><span>{{ d.main_locked ? '主线锁定' : '主线开放' }}</span
                ><AppIcon name="arrow"
              /></span>
            </a>
          </li>
        </ul>
        <div v-if="!docs.length && !error" class="empty-state" :role="busy ? 'status' : undefined">
          <span class="empty-icon" aria-hidden="true"><AppIcon name="page" /></span>
          <h3>{{ busy ? '正在整理文档目录…' : '这里还没有可见文档' }}</h3>
          <p>可以从左侧切换范围，或从右上角新建文档。</p>
        </div>
        <PaginationControls
          label="文档"
          :page="docPage.index + 1"
          :has-next="!!docPage.next"
          :busy="actionsBlocked || docLoading"
          @change="(index) => action.run(() => listDocs(index), '')"
        />
      </section>
      <TrashPanel v-if="section === 'trash' && spaceId" :key="spaceId" :space="spaceId" />
      <section
        v-if="section === 'applications' && !organization"
        aria-label="个人协作申请"
        class="panel"
        :aria-busy="busy || appLoading"
      >
        <div class="section-title">
          <h2>我的申请</h2>
          <button :disabled="actionsBlocked" @click="action.run(() => listOwnApps(), '')">
            刷新申请
          </button>
        </div>
        <ul class="entity-list">
          <li v-for="a in ownApps" :key="a.id">
            <span
              >文档 {{ a.document_id }} ·
              {{
                a.state === 'PENDING'
                  ? '待审批'
                  : a.state === 'ACCEPTED'
                    ? '已授权'
                    : a.state === 'REJECTED'
                      ? '未通过'
                      : '已取消'
              }}</span
            ><small>{{ a.reason }}</small
            ><button
              v-if="a.state === 'ACCEPTED'"
              :disabled="actionsBlocked"
              @click="emit('open', a.document_id)"
            >
              打开已授权文档
            </button>
          </li>
        </ul>
        <p v-if="!ownApps.length && !busy && !appLoading" class="empty">
          还没有申请记录。收到文档邀请后，可从右上角申请协作。
        </p>
        <PaginationControls
          label="个人协作申请"
          :page="appPage.index + 1"
          :has-next="!!appPage.next"
          :busy="actionsBlocked || appLoading"
          @change="(index) => action.run(() => listOwnApps(index), '')"
        />
      </section>
    </div>
  </div>
  <AppDialog v-model="createVisible" title="新建内容" :blocked="actionsBlocked">
    <nav class="document-tabs" aria-label="新建内容类型">
      <button
        v-if="canCreateDoc"
        type="button"
        :aria-pressed="createKind === 'document'"
        :disabled="actionsBlocked"
        @click="createKind = 'document'"
      >
        文档
      </button>
      <button
        v-if="space?.actions.includes('CREATE_KNOWLEDGE_BASE')"
        type="button"
        :aria-pressed="createKind === 'knowledgeBase'"
        :disabled="actionsBlocked"
        @click="createKind = 'knowledgeBase'"
      >
        知识库
      </button>
    </nav>
    <ActionStatus
      v-if="!pending"
      :busy="busy"
      :error="error"
      :notice="notice"
      :pending="pending"
      :retry="retry"
    />
    <p class="hint">
      创建到：{{ createKind === 'document' && currentKB ? currentKB.name : spaceLabel }}
    </p>
    <form v-show="createKind === 'document'" @submit.prevent="createDoc">
      <fieldset :disabled="actionsBlocked || !canCreateDoc">
        <label
          >文档标题<input
            ref="docTitleInput"
            v-model="docTitle"
            required
            maxlength="240"
            autofocus /></label
        ><button class="primary" :disabled="actionsBlocked || !canCreateDoc">创建文档</button>
      </fieldset>
    </form>
    <form v-show="createKind === 'knowledgeBase'" @submit.prevent="createKB">
      <fieldset :disabled="actionsBlocked || !space?.actions.includes('CREATE_KNOWLEDGE_BASE')">
        <label>知识库名称<input v-model="kbName" required maxlength="160" /></label
        ><button class="primary" :disabled="actionsBlocked">创建知识库</button>
      </fieldset>
    </form>
  </AppDialog>
  <AppDialog v-model="applicationVisible" title="申请文档协作" :blocked="actionsBlocked">
    <p class="hint">使用文档所有者提供的邀请信息申请，审批通过后获得相应权限。</p>
    <ActionStatus
      v-if="!pending"
      :busy="busy"
      :error="error"
      :notice="notice"
      :pending="pending"
      :retry="retry"
    />
    <form @submit.prevent="applyDocument">
      <fieldset :disabled="actionsBlocked">
        <label>文档邀请信息<input v-model="inviteToken" required autocomplete="off" /></label
        ><label>文档申请说明<textarea v-model="reason" required maxlength="2000"></textarea></label
        ><button class="primary" :disabled="actionsBlocked">提交个人文档申请</button>
      </fieldset>
    </form>
  </AppDialog>
</template>

<style scoped>
.library-breadcrumb,
.library-footer,
.library-search {
  display: flex;
  align-items: center;
  gap: 12px;
}
.library-breadcrumb {
  flex: 1;
  flex-wrap: wrap;
}
.library-gallery {
  margin: 12px 0;
}
.library-search {
  min-width: 220px;
  max-width: 360px;
}
.library-search input {
  margin: 0;
}
.library-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 28px;
  list-style: none;
  padding: 0;
  margin: 28px 0;
}
.library-grid li {
  min-width: 0;
  max-width: 250px;
}
.library-book {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: stretch;
  text-align: left;
  width: 100%;
  aspect-ratio: 3 / 4;
  padding: 22px 22px 18px 28px;
  overflow: hidden;
  border: 1px solid #ffffff30;
  border-radius: 5px 15px 15px 5px;
  color: #fffefa;
  background: linear-gradient(145deg, #58705f, #233e35);
  box-shadow:
    0 10px 22px #263e3314,
    inset 7px 0 0 #00000012;
  transition:
    transform 0.18s ease,
    box-shadow 0.18s ease;
}
.library-book:hover:not(:disabled) {
  transform: translateY(-5px);
  box-shadow: 0 16px 28px #263e3329;
}
.library-book:focus-visible {
  outline: 3px solid var(--accent);
  outline-offset: 5px;
}
.cover-tone-1 {
  background: linear-gradient(140deg, #9a6f49, #593b2f);
}
.cover-tone-2 {
  background: linear-gradient(140deg, #626f86, #283547);
}
.cover-tone-3 {
  background: linear-gradient(140deg, #8d747e, #503e55);
}
.cover-tone-4 {
  background: linear-gradient(140deg, #898b62, #474e36);
}
.book-kicker {
  position: relative;
  z-index: 1;
  font-size: 11px;
  letter-spacing: 0.12em;
  opacity: 0.75;
}
.book-title {
  position: relative;
  z-index: 1;
  margin-top: 24px;
  font-size: clamp(21px, 2vw, 27px);
  line-height: 1.35;
  font-weight: 650;
  overflow-wrap: anywhere;
  display: -webkit-box;
  -webkit-line-clamp: 4;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.book-art {
  position: absolute;
  right: -30px;
  bottom: 36px;
  width: 180px;
  height: 180px;
  border: 1px solid #ffffff18;
  border-radius: 50%;
  transform: rotate(-20deg);
  display: grid;
  place-items: center;
}
.book-art :deep(svg) {
  width: 110px;
  height: 110px;
  opacity: 0.07;
}
.book-open {
  position: relative;
  z-index: 1;
  margin-top: auto;
  padding-top: 24px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  font-size: 14px;
  opacity: 0.85;
}
.library-footer {
  flex-wrap: wrap;
  justify-content: space-between;
}
@media (max-width: 600px) {
  .library-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 16px;
  }
  .library-book {
    padding: 16px 14px 14px 19px;
    aspect-ratio: 3 / 4.3;
  }
  .book-title {
    margin-top: 16px;
    font-size: 20px;
  }
  .book-kicker {
    font-size: 9px;
  }
  .book-open {
    font-size: 12px;
  }
  .library-search {
    width: 100%;
    min-width: 0;
  }
}
@media (prefers-reduced-motion: reduce) {
  .library-book {
    transition: none;
  }
}
</style>
