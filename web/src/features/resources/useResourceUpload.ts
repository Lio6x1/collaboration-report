import {
  UserFacingError,
  actionsBlocked,
  ApiError,
  explain,
  transportEpoch,
  transportGeneration,
  useAction,
} from '../../lib/http';
import { computed, onBeforeUnmount, ref, shallowRef, watch } from 'vue';
import type {
  Quota,
  Resource,
  ResourceUpload,
  UploadComplete,
  UploadHandle,
} from '../../../../contracts/http/generated';
import {
  imageChecksum,
  readQuota,
  resourceCompletePath,
  resourceUploadPath,
  uploadResourceBytes,
} from './api';

export function useResourceUpload(
  props: { document: string; space: string; disabled?: boolean },
  ready: (resource: Resource) => void,
) {
  const action = useAction();
  const quota = shallowRef<Quota | null>(null);
  const quotaError = ref('');
  const quotaBusy = ref(false);
  const selected = shallowRef<File | null>(null);
  const stage = ref('');
  const completed = shallowRef<Resource | null>(null);
  let pasteInsert: ((resource: Resource) => boolean) | null = null;
  let active = true;
  let quotaRequest = 0;
  let job: {
    file: File;
    document: string;
    space: string;
    generation: number;
    upload: ResourceUpload;
    complete: UploadComplete;
    bytesConfirmed: boolean;
  } | null = null;

  const locked = computed(() => Boolean(props.disabled || actionsBlocked.value));
  const hasJob = ref(false);

  async function refreshQuota() {
    const request = ++quotaRequest;
    quota.value = null;
    quotaError.value = '';
    if (!props.space) return;
    quotaBusy.value = true;
    try {
      const value = await readQuota(props.space);
      if (active && request === quotaRequest) quota.value = value;
    } catch (error) {
      if (active && request === quotaRequest) quotaError.value = explain(error);
    } finally {
      if (request === quotaRequest) quotaBusy.value = false;
    }
  }

  function current() {
    return (
      active &&
      job?.document === props.document &&
      job?.space === props.space &&
      job.generation === transportGeneration()
    );
  }

  function insert() {
    if (!completed.value || props.disabled || !current()) return;
    if (pasteInsert) {
      const insertAtPastePosition = pasteInsert;
      pasteInsert = null;
      if (!insertAtPastePosition(completed.value)) {
        stage.value = '原粘贴位置已不可用；图片已上传，请确认当前编辑器后手动插入正文';
        return;
      }
    } else ready(completed.value);
    completed.value = null;
    selected.value = null;
    job = null;
    hasJob.value = false;
    stage.value = '图片已交给正文编辑器；请留意正文保存状态';
  }

  async function upload() {
    await uploadAttempt(true);
  }

  async function uploadAttempt(retryQuotaConflict: boolean) {
    if (locked.value || !selected.value) return;
    const file = selected.value;
    const document = props.document;
    const space = props.space;
    const generation = transportGeneration();
    let quotaChanged = false;
    await action.run(async () => {
      if (!job) {
        // 正文可靠保存也会改变空间用量；新上传不能沿用打开编辑器时读到的旧配额版本。
        await refreshQuota();
        if (!quota.value) throw new UserFacingError('暂时无法读取空间配额，请刷新配额后重试');
        const revision = quota.value.revision;
        stage.value = '正在计算图片校验值…';
        const checksum = await imageChecksum(file);
        if (
          !active ||
          document !== props.document ||
          space !== props.space ||
          generation !== transportGeneration()
        )
          throw new ApiError('SESSION_CHANGED', 409);
        // 原号、配额版本与校验值绑定同一文件。未知结果恢复不能换文件或另发新号。
        job = {
          file,
          document,
          space,
          generation,
          upload: {
            operation_id: crypto.randomUUID(),
            expected_quota_revision: revision,
            byte_size: String(file.size),
            media_type: file.type,
            checksum,
          },
          complete: { operation_id: crypto.randomUUID(), byte_size: String(file.size), checksum },
          bytesConfirmed: false,
        };
        hasJob.value = true;
      }
      const fixed = job;
      if (fixed.generation !== transportGeneration()) throw new ApiError('SESSION_CHANGED', 409);
      stage.value = '正在预留图片存储配额…';
      let handle: UploadHandle;
      try {
        handle = await action.write<UploadHandle>(resourceUploadPath(fixed.document), fixed.upload);
      } catch (error) {
        if (
          error instanceof ApiError &&
          error.status === 409 &&
          error.code === 'QUOTA_REVISION_CHANGED' &&
          !fixed.bytesConfirmed &&
          current()
        ) {
          // 此409明确证明原预留被拒绝，没有资源/额度占用；旧版本和旧编号不能永远重发。
          // 文件和粘贴位置继续保留，下一次尝试重新读取配额并建立新请求。
          // 网络超时/5xx属于未知结果，绝不能走这里换号，仍由原action续接。
          job = null;
          hasJob.value = false;
          quotaChanged = true;
          stage.value = '空间用量已更新，正在重新准备原图片上传…';
        }
        throw error;
      }
      if (!fixed.bytesConfirmed) {
        stage.value = '正在上传图片…';
        // URL只在这次请求的内存中使用，绝不写入正文或本地存储。
        const uploaded = await uploadResourceBytes(handle.upload_url, fixed.file);
        if (uploaded.id !== handle.resource_id || !['UPLOADING', 'READY'].includes(uploaded.state))
          throw new ApiError('RESOURCE_UPLOAD_RESPONSE_INVALID', 409);
        fixed.bytesConfirmed = true;
      }
      stage.value = '正在确认图片完整性…';
      const resource = await action.write<Resource>(
        resourceCompletePath(handle.resource_id),
        fixed.complete,
      );
      if (
        resource.id !== handle.resource_id ||
        resource.state !== 'READY' ||
        resource.byte_size !== fixed.complete.byte_size ||
        resource.checksum !== fixed.complete.checksum ||
        resource.media_type !== fixed.upload.media_type
      )
        throw new ApiError('RESOURCE_NOT_READY', 409);
      // 离开原文档或切换会话后，恢复只确认原操作，不把旧结果插入新的编辑器。
      if (current()) {
        completed.value = resource;
        stage.value = '图片上传完成，可插入正文';
        // 粘贴只自动交付给原编辑器/原位置；失败重试仍保留原文件与两个幂等编号。
        if (pasteInsert) insert();
        await refreshQuota();
      }
    }, '图片上传已确认完成');
    if (
      !quotaChanged ||
      !active ||
      selected.value !== file ||
      document !== props.document ||
      space !== props.space ||
      generation !== transportGeneration()
    )
      return;
    // 只自动重试一次已知的版本冲突；持续编辑/多人上传时不循环争抢配额。
    if (retryQuotaConflict) await uploadAttempt(false);
    else {
      await refreshQuota();
      stage.value = '空间用量仍在变化，原图片已保留；请待正文保存后点击“上传图片”';
    }
  }

  function paste(file: File, insertAtPastePosition: (resource: Resource) => boolean) {
    // 不覆盖用户已选择的文件或尚未确定结果的上传，避免一次Ctrl+V丢掉原操作。
    if (locked.value || selected.value || job || completed.value) {
      action.error.value = '请先完成或清除当前图片上传，再粘贴下一张图片';
      return false;
    }
    selected.value = file;
    pasteInsert = insertAtPastePosition;
    void upload();
    return true;
  }

  function clear() {
    if (actionsBlocked.value) return;
    selected.value = null;
    completed.value = null;
    pasteInsert = null;
    job = null;
    hasJob.value = false;
    stage.value = '';
    action.error.value = '';
    action.notice.value = '';
    void refreshQuota();
  }

  watch(
    () => [props.document, props.space, transportEpoch.value],
    () => {
      selected.value = null;
      completed.value = null;
      pasteInsert = null;
      stage.value = '';
      // 未知操作由共享恢复入口持有，不在组件切换时取消或生成替代写入。
      if (!action.busy.value && !action.pending.value) {
        job = null;
        hasJob.value = false;
      }
      void refreshQuota();
    },
    { immediate: true },
  );
  onBeforeUnmount(() => {
    active = false;
    quotaRequest++;
  });

  return {
    action,
    quota,
    quotaError,
    quotaBusy,
    selected,
    stage,
    completed,
    locked,
    hasJob,
    refreshQuota,
    upload,
    insert,
    clear,
    paste,
  };
}
