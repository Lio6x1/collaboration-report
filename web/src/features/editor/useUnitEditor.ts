import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue';
import type { EditorView } from 'prosemirror-view';
import {
  absolutePositionToRelativePosition,
  relativePositionToAbsolutePosition,
  ySyncPluginKey,
} from 'y-prosemirror';
import { schema, toPM } from '../../../../packages/content/index.mjs';
import { DraftController } from './controller';
import { createDraftView } from './view';
import * as commands from './commands';
import type { FormatName, TableAction } from './commands';
import { bodyText } from './domain/recovery_body';
import type { MainView, Resource } from '../../../../contracts/http/generated';
import type { ResourceContext } from '../../lib/content/image';
import { api, transportGeneration, UserFacingError, explain } from '../../lib/http';
import { documentPath, editorPath } from '../workspace/navigation';
type EditorTarget = {
  actor: string;
  document: string;
  unit: string;
  structure: string;
  kind?: 'BRANCH' | 'MAIN';
};

// composable拥有视图生命周期与用户动作；收到新READY后重装视图，避免旧epoch继续写当前队列。
export function useUnitEditor(
  props: EditorTarget,
  flushed: (receipt: unknown) => void,
  receiveImage: (file: File, insert: (resource: Resource) => boolean) => boolean,
  editingBlocked: () => boolean = () => false,
) {
  const version = ref(0),
    host = ref<HTMLElement | null>(null),
    formula = ref('x^2'),
    markdown = ref(''),
    loss = ref(''),
    busy = ref(false),
    toolbar = ref(commands.toolbarState()),
    failure = ref('');
  let view: EditorView | undefined;
  const imageGeneration = transportGeneration();
  let mainRead: Promise<MainView> | null = null;
  const resourceSaved = new EventTarget();
  const controller = new DraftController(
    {
      actor: props.actor,
      document: props.document,
      unit: props.unit,
      kind: props.kind ?? 'BRANCH',
    },
    props.structure,
    () => {
      version.value++;
      refreshEditability();
    },
    () => void install(),
    () => {
      // SAVED之前开始的MAIN读取可能不含新资源；新一批重试必须取保存后的清单。
      mainRead = null;
      resourceSaved.dispatchEvent(new Event('saved'));
    },
    async () => {
      // 先让当前中文选词及DOM observer收尾，再排空可靠队列；暂停不能截掉用户正在输入的汉字。
      const selection = view?.state.selection.$from;
      let block: string | null = null;
      if (selection)
        for (let depth = selection.depth; depth > 0; depth--) {
          const node = selection.node(depth);
          if (typeof node.attrs.id === 'string') {
            block = node.attrs.id;
            break;
          }
        }
      view?.dom.blur();
      await new Promise((r) => setTimeout(r, 0));
      await new Promise((r) => setTimeout(r, 0));
      if (controller.composing) throw new UserFacingError('请先完成中文选词，目录调整已暂缓');
      return block;
    },
    (unit) => {
      // 新目录已实际安装才导航；新页面重新核验资格和READY，不复用旧房间的Y.Doc/待发更新。
      location.assign(
        unit
          ? editorPath(props.document, unit, props.kind ?? 'BRANCH')
          : documentPath(props.document),
      );
    },
  );
  const status = computed(() => {
    version.value;
    return {
      notice: controller.notice,
      problem: controller.problem,
      stale: controller.stale,
      retained: controller.retained,
      receipt: controller.receipt,
      loading: controller.loading,
      structurePaused: !!controller.structurePause,
    };
  });
  const discussionBoundary = computed(() => {
    version.value;
    return controller.discussionBoundary();
  });
  async function install() {
    await nextTick();
    if (!host.value || controller.stopped) return;
    view?.destroy();
    view = createDraftView(
      host.value,
      controller,
      (message) => {
        loss.value = message;
      },
      resourceContext,
      resourceSaved,
      pasteImage,
      editingBlocked,
      (state) => {
        toolbar.value = commands.toolbarState(state);
      },
    );
  }
  function refreshEditability() {
    view?.setProps({
      editable: () =>
        !controller.stopped &&
        !controller.stale &&
        !controller.loading &&
        (!controller.structurePause || controller.composing) &&
        !editingBlocked(),
    });
  }
  function pasteImage(file: File, originalView: EditorView) {
    const binding = ySyncPluginKey.getState(originalView.state)?.binding;
    if (!binding || !originalView.editable || busy.value) return;
    // 上传是异步的。用Yjs相对位置记住粘贴点，继续打字或收到远端修改都不能借用后来的光标。
    const position = absolutePositionToRelativePosition(
      originalView.state.selection.from,
      binding.type,
      binding.mapping,
    );
    const accepted = receiveImage(file, (resource) => {
      if (
        originalView !== view ||
        controller.stopped ||
        controller.loading ||
        controller.stale ||
        busy.value ||
        !originalView.editable ||
        imageGeneration !== transportGeneration() ||
        resource.state !== 'READY' ||
        ySyncPluginKey.getState(originalView.state)?.binding !== binding
      )
        return false;
      const at = relativePositionToAbsolutePosition(
        controller.doc,
        binding.type,
        position,
        binding.mapping,
      );
      if (at === null) return false;
      // 不抢走用户当前的选区；普通事务仍经过原Yjs可靠保存队列，上传成功不等于正文已保存。
      originalView.dispatch(
        originalView.state.tr.replaceRangeWith(
          at,
          at,
          schema.nodes.image.create({ resourceId: resource.id, alt: '正文图片' }),
        ),
      );
      loss.value = '';
      return true;
    });
    if (accepted) loss.value = '正在上传粘贴的图片，请留意上传进度和正文保存状态';
  }
  async function resourceContext(): Promise<ResourceContext | null> {
    if (
      controller.stopped ||
      controller.loading ||
      controller.stale ||
      imageGeneration !== transportGeneration()
    )
      return null;
    if (props.kind !== 'MAIN') {
      const branch = controller.resourceBranchID();
      return branch ? { document: props.document, kind: 'BRANCH', id: branch } : null;
    }
    // 同批图片共用一次新MAIN读取；不永久缓存manifest，否则远端新插图会一直查旧引用集合。
    mainRead ??= api<MainView>('/v1/documents/' + encodeURIComponent(props.document) + '/main');
    const pendingRead = mainRead;
    try {
      const current = await pendingRead;
      if (controller.stopped || controller.stale || imageGeneration !== transportGeneration())
        return null;
      return { document: props.document, kind: 'MAIN', id: current.manifest_id };
    } finally {
      if (mainRead === pendingRead) mainRead = null;
    }
  }
  function insertImage(resource: Resource) {
    failure.value = '';
    if (
      !view ||
      controller.stopped ||
      controller.loading ||
      controller.stale ||
      busy.value ||
      imageGeneration !== transportGeneration()
    ) {
      failure.value = '当前编辑器不可写，请重新打开后插入图片';
      return;
    }
    if (resource.state !== 'READY') return;
    commands.insertImage(view, resource.id);
  }
  const format = (name: FormatName) => commands.format(view, name);
  const insertFormula = () => commands.insertFormula(view, formula.value);
  const heading = () => commands.heading(view);
  const paragraph = () => commands.paragraph(view);
  const codeBlock = () => commands.codeBlock(view);
  const paragraphAlignment = (align: commands.ParagraphAlignment) =>
    commands.paragraphAlignment(view, align);
  const firstLineIndent = (enabled: boolean) => commands.firstLineIndent(view, enabled);
  const table = () => commands.table(view);
  const tableAction = (name: TableAction) =>
    commands.tableAction(view, commands.tableActions[name]);
  function insertMarkdown() {
    failure.value = '';
    try {
      if (!view || controller.stale || controller.loading)
        throw new UserFacingError('请先打开当前可恢复的草稿');
      commands.insertMarkdown(view, markdown.value, (message) => {
        loss.value = message;
      });
    } catch (e) {
      failure.value = explain(e, 'Markdown 导入失败，请检查文件内容后重试');
    }
  }
  async function flush() {
    try {
      await prepareSave();
    } catch {
      // 原确认保存按钮由相同边界函数展示错误；送审入口会将错误继续交给操作状态。
    }
  }
  async function prepareSubmission() {
    const receipt = await prepareSave();
    // MAIN只能确认可靠保存；送审资格仍由原BRANCH边界校验，不能因共用视图放宽。
    return controller.submissionBoundary(receipt);
  }
  async function preparePull() {
    const receipt = await prepareSave();
    return controller.pullBoundary(receipt);
  }
  async function prepareDraft() {
    const receipt = await prepareSave();
    const boundary = controller.pullBoundary(receipt);
    return {
      expected_branch_id: boundary.expected_branch_id,
      expected_structure_id: boundary.expected_structure_id,
    };
  }
  async function prepareSave() {
    busy.value = true;
    failure.value = '';
    try {
      // 点击按钮先让浏览器完成composition/DOM observer，再抓本地队列；只删除也必须进入依赖集合。
      view?.dom.blur();
      await new Promise((r) => setTimeout(r, 0));
      await new Promise((r) => setTimeout(r, 0));
      if (controller.composing) throw new UserFacingError('请完成中文选词后再保存');
      const receipt = await controller.flush();
      flushed(receipt);
      return receipt;
    } catch (e) {
      failure.value = explain(e, '当前修改尚未保存，请保留页面并稍后重试');
      throw e;
    } finally {
      busy.value = false;
      version.value++;
    }
  }
  function oldText() {
    try {
      const body = controller.staleBody();
      return body ? bodyText(body) : '';
    } catch {
      return '旧输入尚无法解析，可保留本机草稿等待恢复';
    }
  }
  async function recover() {
    failure.value = '';
    try {
      const body = controller.staleBody();
      if (!body || !view) return;
      await controller.beginRecovery();
      view.dispatch(
        view.state.tr.replaceWith(
          0,
          view.state.doc.content.size,
          schema.nodeFromJSON(toPM(body)).content,
        ),
      );
      view.setProps({ editable: () => true });
      view.focus();
    } catch (e) {
      failure.value = explain(e, '本机输入尚未恢复，请保留原内容并重新尝试');
    }
  }
  async function selectRetained(key: string) {
    failure.value = '';
    try {
      await controller.selectRetained(key);
      view?.setProps({ editable: () => false });
    } catch (e) {
      failure.value = explain(e, '本机输入尚未读取，请重新尝试');
    }
  }
  async function keepCurrent() {
    failure.value = '';
    try {
      await controller.beginRecovery();
      view?.setProps({ editable: () => true });
      view?.focus();
    } catch (e) {
      failure.value = explain(e, '本机输入尚未保留，请保留页面并复制当前内容备份');
    }
  }
  onMounted(() => {
    void controller.start().catch((e) => {
      failure.value = explain(e, '编辑器暂时无法打开，请检查浏览器存储权限后重试');
    });
    window.addEventListener('offline', controller.offline);
    window.addEventListener('beforeunload', protectLocalInput);
  });
  onUnmounted(() => {
    window.removeEventListener('offline', controller.offline);
    window.removeEventListener('beforeunload', protectLocalInput);
    view?.destroy();
    void controller.stop();
  });

  function protectLocalInput(event: BeforeUnloadEvent) {
    // 浏览器离页事件不能等待异步保存；只在输入尚未写入本机队列时提示，已落盘队列由刷新恢复。
    if (!controller.hasVolatileInput()) return;
    event.preventDefault();
    event.returnValue = '';
  }

  return {
    host,
    toolbar,
    formula,
    markdown,
    loss,
    busy,
    failure,
    status,
    discussionBoundary,
    format,
    heading,
    paragraph,
    codeBlock,
    paragraphAlignment,
    firstLineIndent,
    table,
    tableAction,
    insertFormula,
    insertImage,
    insertMarkdown,
    flush,
    prepareSubmission,
    preparePull,
    prepareDraft,
    prepareSave,
    oldText,
    recover,
    selectRetained,
    keepCurrent,
    refreshEditability,
  };
}
