import {
  UserFacingError,
  actionsBlocked,
  ApiError,
  explain,
  transportEpoch,
  useAction,
} from '../../lib/http';
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue';
import { createCursorPage, pageCursor, acceptPage, resetPage } from '../../lib/pagination';
import type {
  Anchor,
  Comment,
  CommentCreate,
  CommentEdit,
  CommentThread,
  CommentThreadCreate,
  ThreadState,
  VersionedAction,
} from '../../../../contracts/http/generated';
import {
  commentPath,
  commentsPath,
  readComments,
  readThreads,
  readThread,
  threadsPath,
  threadStatePath,
} from './api';

export type CommentsTarget = {
  document: string;
  contextKind: 'MAIN' | 'BRANCH' | 'REVIEW';
  contextId: string;
  version: string;
  canComment: boolean;
  actor: string;
  canManage?: boolean;
  browseMainHistory?: boolean;
  browseBranchHistory?: boolean;
  anchor?: Anchor | null;
  space?: string;
  initialThread?: CommentThread;
  threadOnly?: boolean;
};

export function useComments(props: CommentsTarget) {
  const action = useAction();
  const threads = ref<CommentThread[]>([]);
  const selected = ref<CommentThread | null>(null);
  const comments = ref<Comment[]>([]);
  const threadCursor = ref<string | null>(null);
  const threadPage = reactive(createCursorPage());
  const commentCursor = ref<string | null>(null);
  const loadingThreads = ref(false);
  const loadingComments = ref(false);
  const readingError = ref('');
  const previews = ref<
    Record<string, { body: string; author: string; loading?: boolean; error?: string }>
  >({});
  const previewQueue: {
    thread: CommentThread;
    generation: number;
    entry: (typeof previews.value)[string];
  }[] = [];
  let previewGeneration = 0;
  let previewWorkers = 0;
  const newBody = ref('');
  const newMentions = ref<string[]>([]);
  const replyMentions = ref<string[]>([]);
  const replyBody = ref('');
  const replyTo = ref<Comment | null>(null);
  const editing = ref<Comment | null>(null);
  const editBody = ref('');
  let active = true;
  let contextGeneration = 0;
  let threadRequest = 0;
  let commentRequest = 0;
  const busy = computed(
    () => actionsBlocked.value || loadingThreads.value || loadingComments.value,
  );
  const canReply = computed(
    () => props.canComment && !!selected.value && selected.value.status !== 'DELETED',
  );
  const canModerate = computed(() =>
    Boolean(selected.value && (props.canManage || selected.value.author_id === props.actor)),
  );
  const canEdit = (comment: Comment) => comment.body !== '' && comment.author_id === props.actor;
  const canDelete = (comment: Comment) =>
    comment.body !== '' && (props.canManage || comment.author_id === props.actor);

  function clearPreviews() {
    previewGeneration++;
    previewQueue.length = 0;
    previews.value = {};
  }
  function loadPreview(thread: CommentThread) {
    if (!active || thread.status === 'DELETED' || previews.value[thread.id]) return;
    previews.value[thread.id] = { body: '', author: thread.author_id, loading: true };
    previewQueue.push({ thread, generation: previewGeneration, entry: previews.value[thread.id]! });
    void drainPreviews();
  }
  async function drainPreviews() {
    // 只为进入可视区域的卡片读取首条评论，沿用逐线程授权；最多两个请求，不预取整个历史列表。
    if (previewWorkers >= 2 || !active) return;
    const task = previewQueue.shift();
    if (!task) return;
    previewWorkers++;
    const current = () =>
      active &&
      task.generation === previewGeneration &&
      previews.value[task.thread.id] === task.entry;
    try {
      const page = await readComments(task.thread.id, null, 1);
      if (current()) {
        const first = page.items[0];
        previews.value[task.thread.id] = {
          body: first?.body ?? '',
          author: first?.author_id ?? task.thread.author_id,
        };
      }
    } catch (error) {
      if (current())
        previews.value[task.thread.id] = {
          body: '',
          author: task.thread.author_id,
          error: explain(error),
        };
    } finally {
      previewWorkers--;
      void drainPreviews();
    }
  }

  function clearComments() {
    commentRequest++;
    comments.value = [];
    commentCursor.value = null;
    replyTo.value = null;
    editing.value = null;
    editBody.value = '';
    replyBody.value = '';
    replyMentions.value = [];
    loadingComments.value = false;
  }

  async function fetchThreads(index = threadPage.index) {
    const generation = contextGeneration;
    const request = ++threadRequest;
    const cursor = pageCursor(threadPage, index);
    // 翻页只替换主评论列表；已选原讨论与回复草稿保持原上下文。
    clearPreviews();
    threads.value = [];
    loadingThreads.value = true;
    try {
      const history =
        (props.contextKind === 'MAIN' && props.browseMainHistory) ||
        (props.contextKind === 'BRANCH' && props.browseBranchHistory);
      const page = await readThreads(props.document, cursor, {
        kind: props.contextKind,
        ...(props.contextKind === 'MAIN' && props.browseMainHistory
          ? {}
          : { context: props.contextId }),
        ...(history ? {} : { version: props.version }),
      });
      if (!active || generation !== contextGeneration || request !== threadRequest) return;
      // 筛选由服务器先于分页执行；仍只接受原文档与上下文的结果。
      threads.value = page.items.filter(
        (thread) =>
          thread.document_id === props.document &&
          thread.context_kind === props.contextKind &&
          ((props.contextKind === 'MAIN' && props.browseMainHistory) ||
            (props.contextKind === 'BRANCH' &&
              props.browseBranchHistory &&
              thread.context_id === props.contextId) ||
            (thread.context_id === props.contextId &&
              thread.anchor.content_version_id === props.version)),
      );
      acceptPage(threadPage, index, cursor, page.next_cursor);
      threadCursor.value = page.next_cursor;
    } catch (error) {
      if (
        active &&
        generation === contextGeneration &&
        request === threadRequest &&
        error instanceof ApiError &&
        [401, 403, 404].includes(error.status)
      ) {
        threads.value = [];
        threadCursor.value = null;
        resetPage(threadPage);
        selected.value = null;
        clearComments();
      }
      throw error;
    } finally {
      if (request === threadRequest) loadingThreads.value = false;
    }
  }

  async function fetchComments(thread: string, append = false) {
    const generation = contextGeneration;
    const request = ++commentRequest;
    const cursor = append ? commentCursor.value : null;
    if (append && !cursor) return;
    if (!append) {
      comments.value = [];
      commentCursor.value = null;
    }
    loadingComments.value = true;
    try {
      const page = await readComments(thread, cursor);
      if (
        !active ||
        generation !== contextGeneration ||
        request !== commentRequest ||
        selected.value?.id !== thread
      )
        return;
      comments.value = append
        ? [
            ...comments.value,
            ...page.items.filter((comment) => !comments.value.some((old) => old.id === comment.id)),
          ]
        : page.items;
      commentCursor.value = page.next_cursor;
      if (!append) {
        const first = page.items[0];
        previews.value[thread] = {
          body: first?.body ?? '',
          author: first?.author_id ?? selected.value!.author_id,
        };
      }
    } catch (error) {
      if (
        active &&
        generation === contextGeneration &&
        request === commentRequest &&
        error instanceof ApiError &&
        [401, 403, 404].includes(error.status)
      ) {
        // 删除或撤权之后立即关闭旧线程，不能继续显示旧正文或用旧资格回复。
        selected.value = null;
        clearComments();
      }
      throw error;
    } finally {
      if (request === commentRequest) loadingComments.value = false;
    }
  }

  async function read(task: () => Promise<void>) {
    const generation = contextGeneration;
    readingError.value = '';
    try {
      await task();
    } catch (error) {
      if (active && generation === contextGeneration) readingError.value = explain(error);
    }
  }
  async function fetchExactThread() {
    const id = props.initialThread?.id;
    if (!id) throw new UserFacingError('缺少原讨论目标');
    const generation = contextGeneration;
    const request = ++threadRequest;
    selected.value = null;
    clearComments();
    loadingThreads.value = true;
    try {
      const thread = await readThread(id);
      if (!active || generation !== contextGeneration || request !== threadRequest) return;
      if (
        thread.document_id !== props.document ||
        thread.context_kind !== props.contextKind ||
        thread.context_id !== props.contextId ||
        thread.anchor.content_version_id !== props.version
      )
        throw new UserFacingError('讨论目标已变化，请重新打开原通知');
      selected.value = thread;
      if (thread.status !== 'DELETED') await fetchComments(thread.id);
    } finally {
      if (request === threadRequest) loadingThreads.value = false;
    }
  }
  async function fetchCurrent(index = threadPage.index) {
    if (!props.threadOnly) await fetchThreads(index);
    // 精确线程不依赖文档第一页出现，不把另一条讨论当成通知目标。
    if (props.initialThread) await fetchExactThread();
    else if (props.threadOnly) throw new UserFacingError('缺少原讨论目标');
  }
  // 显式刷新从首页建立新游标链；不能让停留超过五分钟的旧游标困住整个评论区。
  const refresh = () => read(() => fetchCurrent(0));
  const turnThreads = (index: number) => read(() => fetchThreads(index));
  const moreThreads = () => turnThreads(threadPage.index + 1);
  const moreComments = () =>
    selected.value ? read(() => fetchComments(selected.value!.id, true)) : Promise.resolve();
  async function open(thread: CommentThread) {
    if (busy.value) return;
    if (selected.value?.id === thread.id && !readingError.value) return;
    clearComments();
    selected.value = thread;
    if (thread.status !== 'DELETED') await read(() => fetchComments(thread.id));
  }

  function body(value: string) {
    if (!value.trim()) throw new UserFacingError('请输入批注内容');
    if (value.length > 8000) throw new UserFacingError('批注不能超过 8000 个字符');
    return value;
  }

  async function write(
    path: string,
    input: Record<string, unknown>,
    success: string,
    after: () => Promise<void>,
    method = 'POST',
  ) {
    if (busy.value) return;
    const generation = contextGeneration;
    readingError.value = '';
    // 闭包固定上下文、正文和期望修订；useAction保留原号，恢复不得改成新线程或新会话输入。
    await action.run(async () => {
      await action.mutate(path, input, method);
      if (active && generation === contextGeneration) await after();
    }, success);
  }
  async function create() {
    if (!props.canComment || !props.version || props.threadOnly || busy.value) return;
    try {
      if (props.anchor && props.anchor.content_version_id !== props.version)
        throw new UserFacingError('选区属于另一份固定正文，请重新选择或清除选区');
      const input: Omit<CommentThreadCreate, 'operation_id'> = {
        context_kind: props.contextKind,
        context_id: props.contextId,
        // 复制固定锚点，原号恢复期间不能因用户随后移动选区而改写原操作输入。
        anchor: props.anchor
          ? { ...props.anchor }
          : { schema_version: 'anchor-v1', content_version_id: props.version },
        body: body(newBody.value),
        mentions: [...new Set(newMentions.value)].filter((id) => id !== props.actor),
      };
      await write(threadsPath(props.document), input, '讨论已创建', async () => {
        newBody.value = '';
        newMentions.value = [];
        resetPage(threadPage);
        await fetchThreads(0);
      });
    } catch (error) {
      action.error.value = explain(error);
    }
  }
  async function reply() {
    if (!canReply.value || !selected.value || busy.value) return;
    try {
      const thread = selected.value.id;
      const input: Omit<CommentCreate, 'operation_id'> = {
        body: body(replyBody.value),
        mentions: [...new Set(replyMentions.value)].filter((id) => id !== props.actor),
        ...(replyTo.value ? { reply_to_id: replyTo.value.id } : {}),
      };
      await write(commentsPath(thread), input, '批注已发表', async () => {
        replyBody.value = '';
        replyMentions.value = [];
        replyTo.value = null;
        await fetchComments(thread);
      });
    } catch (error) {
      action.error.value = explain(error);
    }
  }
  function beginEdit(comment: Comment) {
    if (busy.value || !canEdit(comment)) return;
    editing.value = comment;
    editBody.value = comment.body;
  }
  async function saveEdit() {
    if (!editing.value || !canEdit(editing.value) || busy.value) return;
    try {
      const comment = editing.value;
      const input: Omit<CommentEdit, 'operation_id'> = {
        expected_revision: comment.revision,
        body: body(editBody.value),
      };
      await write(
        commentPath(comment.id),
        input,
        '批注已修改',
        async () => {
          editing.value = null;
          editBody.value = '';
          await fetchComments(comment.thread_id);
        },
        'PATCH',
      );
    } catch (error) {
      action.error.value = explain(error);
    }
  }
  async function remove(comment: Comment) {
    if (!canDelete(comment) || busy.value) return;
    const input: Omit<VersionedAction, 'operation_id'> = { expected_revision: comment.revision };
    await write(
      commentPath(comment.id),
      input,
      '批注已删除',
      async () => {
        if (replyTo.value?.id === comment.id) replyTo.value = null;
        if (editing.value?.id === comment.id) editing.value = null;
        await fetchComments(comment.thread_id);
      },
      'DELETE',
    );
  }
  async function setState(state: ThreadState['state']) {
    if (!selected.value || !canModerate.value || busy.value || selected.value.status === 'DELETED')
      return;
    const thread = selected.value;
    const input: Omit<ThreadState, 'operation_id'> = { expected_revision: thread.revision, state };
    await write(
      threadStatePath(thread.id),
      input,
      state === 'DELETED' ? '讨论已删除' : state === 'RESOLVED' ? '讨论已解决' : '讨论已重新打开',
      async () => {
        if (state === 'DELETED') {
          if (selected.value?.id === thread.id) {
            selected.value = null;
            clearComments();
          }
        } else {
          // 列表分页保留原选中讨论；成功写入后须单独重读新状态和revision，
          // 不能让下一次重开沿用旧revision，也不能为刷新状态清掉未提交回复。
          const current = await readThread(thread.id);
          if (selected.value?.id === thread.id) selected.value = current;
        }
        if (!props.threadOnly) await fetchThreads();
      },
    );
  }

  watch(
    () => [
      props.document,
      props.contextKind,
      // 历史浏览与创建位置分开：切换新讨论单元不能清空正在阅读/回复的原线程。
      props.contextKind === 'MAIN' && props.browseMainHistory ? null : props.contextId,
      props.contextKind === 'MAIN' && props.browseMainHistory ? null : props.version,
      props.browseMainHistory,
      props.browseBranchHistory,
      props.initialThread?.id,
      props.threadOnly,
      props.space,
      props.actor,
      transportEpoch.value,
    ],
    () => {
      contextGeneration++;
      threadRequest++;
      loadingThreads.value = false;
      threads.value = [];
      resetPage(threadPage);
      clearPreviews();
      threadCursor.value = null;
      selected.value = null;
      clearComments();
      newBody.value = '';
      newMentions.value = [];
      readingError.value = '';
      if (!action.pending.value && !action.busy.value) {
        action.error.value = '';
        action.notice.value = '';
      }
      if (props.document && props.contextId && props.version) void refresh();
    },
    { immediate: true },
  );
  onBeforeUnmount(() => {
    active = false;
    contextGeneration++;
    threadRequest++;
    commentRequest++;
    clearPreviews();
  });
  // 文档级历史浏览不切换已读线程，但新的创建上下文不能沿用上个单元的提及名单。
  watch(
    () => [props.contextId, props.version],
    () => {
      newMentions.value = [];
    },
  );

  return {
    action,
    threads,
    selected,
    comments,
    threadCursor,
    threadPage,
    turnThreads,
    commentCursor,
    loadingThreads,
    loadingComments,
    readingError,
    previews,
    loadPreview,
    newBody,
    newMentions,
    replyMentions,
    replyBody,
    replyTo,
    editing,
    editBody,
    busy,
    canReply,
    canModerate,
    canEdit,
    canDelete,
    refresh,
    moreThreads,
    moreComments,
    open,
    create,
    reply,
    beginEdit,
    saveEdit,
    remove,
    setState,
  };
}
