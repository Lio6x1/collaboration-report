import { watch } from 'vue';
import type { Node } from 'prosemirror-model';
import type { EditorView, NodeView } from 'prosemirror-view';
import { NodeSelection } from 'prosemirror-state';
import {
  ApiError,
  transportEpoch,
  transportGeneration,
  UserFacingError,
  explain,
  sessionContentURL,
} from '../http';

export type ResourceContext = {
  document: string;
  kind: 'MAIN' | 'BRANCH' | 'REVIEW' | 'PUBLIC';
  id: string;
};
export type ResourceContextSource = () => ResourceContext | null | Promise<ResourceContext | null>;

export function resourceContentURL(resource: string, context: ResourceContext) {
  const query = new URLSearchParams({
    document_id: context.document,
    context_kind: context.kind,
    context_id: context.id,
  });
  return sessionContentURL(
    '/v1/resources/' + encodeURIComponent(resource) + '/content?' + query.toString(),
  );
}

// URL仅属于视图DOM；Yjs和业务正文始终只持有resourceId，不携带可复用下载凭据。
export function imageNodeView(
  source: ResourceContextSource,
  saved?: EventTarget,
  resizable = false,
) {
  return (initial: Node, view: EditorView, getPos: () => number | undefined): NodeView => {
    let node = initial;
    let disposed = false;
    let request = 0;
    let loaded = false;
    let selected = false;
    let retries = 0;
    let retryTimer: ReturnType<typeof setTimeout> | undefined;
    let drag: { x: number; width: number; ratio: number; preview: number } | null = null;
    const generation = transportGeneration();
    const dom = document.createElement('span');
    dom.className = 'resource-image';
    dom.contentEditable = 'false';
    const image = document.createElement('img');
    image.style.maxWidth = '100%';
    image.style.height = 'auto';
    const frame = document.createElement('span');
    frame.className = 'resource-image-frame';
    frame.append(image);
    const resize = document.createElement('button');
    resize.type = 'button';
    resize.className = 'image-resize-handle';
    resize.setAttribute('aria-label', '等比例调整图片大小');
    resize.title = '拖动等比例缩放；方向键微调';
    const controls = document.createElement('span');
    controls.className = 'image-size-controls';
    const smaller = document.createElement('button');
    smaller.type = 'button';
    smaller.textContent = '缩小图片';
    const larger = document.createElement('button');
    larger.type = 'button';
    larger.textContent = '放大图片';
    const dimensions = document.createElement('span');
    dimensions.setAttribute('role', 'status');
    controls.append(smaller, larger, dimensions);
    if (resizable) frame.append(resize);
    const message = document.createElement('span');
    message.setAttribute('role', 'status');
    const refresh = document.createElement('button');
    refresh.type = 'button';
    refresh.textContent = '重新加载图片';
    dom.append(frame, message, refresh);
    if (resizable) dom.append(controls);

    function canResize() {
      return (
        resizable && loaded && !disposed && view.editable && generation === transportGeneration()
      );
    }
    function resizeControls() {
      const visible = selected && canResize();
      resize.hidden = controls.hidden = !visible;
      dimensions.textContent = `${Math.round(image.getBoundingClientRect().width)} px · 等比例`;
    }
    function ratio() {
      return image.naturalWidth / image.naturalHeight || 1;
    }
    function constrainedWidth(width: number, proportion: number) {
      // 图片只能在正文宽度内缩放，且宽高都遵守共享正文规范的10000px上限。
      const max = Math.max(1, Math.min(view.dom.clientWidth, 10000, 10000 * proportion));
      return Math.round(Math.min(max, Math.max(Math.min(32, max), width)));
    }
    function saveSize(width: number, proportion = ratio()) {
      if (!canResize()) return;
      const pos = getPos();
      if (
        pos === undefined ||
        view.state.doc.nodeAt(pos)?.attrs.resourceId !== node.attrs.resourceId
      )
        return;
      const nextWidth = constrainedWidth(width, proportion);
      const height = Math.max(1, Math.min(10000, Math.round(nextWidth / proportion)));
      // 只提交一次普通正文事务，继续经过原Yjs队列；不把下载URL或拖动预览写进正文。
      const tr = view.state.tr.setNodeMarkup(pos, undefined, {
        ...node.attrs,
        width: nextWidth,
        height,
      });
      view.dispatch(tr.setSelection(NodeSelection.create(tr.doc, pos)));
      view.focus();
      resizeControls();
    }
    function cancelDrag() {
      drag = null;
      image.style.width = '';
      window.removeEventListener('pointermove', move);
      window.removeEventListener('pointerup', finish);
      window.removeEventListener('pointercancel', cancelDrag);
    }
    function move(event: PointerEvent) {
      if (!drag) return;
      if (!canResize()) {
        cancelDrag();
        return;
      }
      drag.preview = constrainedWidth(drag.width + event.clientX - drag.x, drag.ratio);
      image.style.width = drag.preview + 'px';
      dimensions.textContent = `${drag.preview} px · 等比例`;
    }
    function finish() {
      const current = drag;
      cancelDrag();
      if (current) saveSize(current.preview, current.ratio);
    }
    resize.onpointerdown = (event) => {
      if (event.button !== 0 || !canResize()) return;
      event.preventDefault();
      drag = {
        x: event.clientX,
        width: image.getBoundingClientRect().width,
        ratio: ratio(),
        preview: image.getBoundingClientRect().width,
      };
      window.addEventListener('pointermove', move);
      window.addEventListener('pointerup', finish);
      window.addEventListener('pointercancel', cancelDrag);
    };
    resize.onkeydown = (event) => {
      if (!['ArrowLeft', 'ArrowDown', 'ArrowRight', 'ArrowUp'].includes(event.key)) return;
      event.preventDefault();
      const change = ['ArrowLeft', 'ArrowDown'].includes(event.key) ? -8 : 8;
      saveSize(image.getBoundingClientRect().width + change);
      resize.focus();
    };
    smaller.onclick = () => saveSize(image.getBoundingClientRect().width * 0.9);
    larger.onclick = () => saveSize(image.getBoundingClientRect().width * 1.1);
    controls.onpointerdown = (event) => event.preventDefault();

    function format() {
      image.alt = String(node.attrs.alt ?? '正文图片');
      image.title = String(node.attrs.title ?? '');
      for (const key of ['width', 'height']) {
        if (node.attrs[key]) image.setAttribute(key, String(node.attrs[key]));
        else image.removeAttribute(key);
      }
      dom.style.fontWeight = node.attrs.atomStrong ? '700' : '';
      dom.style.fontStyle = node.attrs.atomEm ? 'italic' : '';
      dom.style.textDecoration = node.attrs.atomStrike ? 'line-through' : '';
      resizeControls();
    }
    function clearRetry() {
      clearTimeout(retryTimer);
      retryTimer = undefined;
    }
    function retryLater() {
      if (disposed || generation !== transportGeneration() || retries >= 2) return;
      retries++;
      message.textContent = `图片读取暂时失败，正在自动重试（${retries}/2）…`;
      // 只做短暂且有上限的只读重试；重新取得上下文并走服务端鉴权，不缓存文件绕过撤权。
      retryTimer = setTimeout(() => void load(true), retries === 1 ? 1000 : 3000);
    }
    async function load(automatic = false) {
      clearRetry();
      if (!automatic) retries = 0;
      const sequence = ++request;
      loaded = false;
      cancelDrag();
      resizeControls();
      image.hidden = true;
      image.removeAttribute('src');
      message.textContent = '正在读取图片…';
      refresh.disabled = true;
      try {
        if (generation !== transportGeneration())
          throw new UserFacingError('登录状态已变化，请重新打开正文');
        const context = await source();
        if (disposed || sequence !== request) return;
        if (generation !== transportGeneration())
          throw new UserFacingError('登录状态已变化，请重新打开正文');
        if (!context) throw new UserFacingError('此图片暂时无法读取，请确认正文已保存后重新加载');
        image.onload = () => {
          if (disposed || sequence !== request) return;
          clearRetry();
          retries = 0;
          loaded = true;
          image.hidden = false;
          message.textContent = '';
          refresh.hidden = true;
          resizeControls();
        };
        image.onerror = () => {
          if (disposed || sequence !== request) return;
          message.textContent = saved
            ? '图片暂不可用；正文保存确认后会自动重试，也可手动重新加载。'
            : '图片暂不可用，请重新加载或刷新正文。';
          refresh.disabled = false;
          refresh.hidden = false;
          retryLater();
        };
        image.src = resourceContentURL(String(node.attrs.resourceId), context);
      } catch (error) {
        if (disposed || sequence !== request) return;
        message.textContent = explain(error, '图片暂时无法读取，请重新加载或刷新正文');
        refresh.disabled = false;
        refresh.hidden = false;
        // 明确的权限、引用或会话错误留给用户处理；只重试网络中断及服务暂不可用。
        if (error instanceof ApiError && (error.status === 0 || error.status >= 500)) retryLater();
      }
    }
    refresh.onclick = () => void load();
    const retryMissing = () => {
      // 只重试失败或尚未完成的图片。已显示图片不随每次打字的SAVED重复下载。
      if (!loaded && !disposed && generation === transportGeneration()) void load();
    };
    saved?.addEventListener('saved', retryMissing);
    const stop = watch(transportEpoch, () => {
      clearRetry();
      request++;
      loaded = false;
      cancelDrag();
      resizeControls();
      image.removeAttribute('src');
      image.hidden = true;
      message.textContent = '会话已变化，请重新打开正文';
      refresh.hidden = true;
    });
    format();
    void load();
    return {
      dom,
      update(next) {
        if (next.type !== node.type) return false;
        const changed = next.attrs.resourceId !== node.attrs.resourceId;
        if (
          changed ||
          next.attrs.width !== node.attrs.width ||
          next.attrs.height !== node.attrs.height
        )
          cancelDrag();
        node = next;
        format();
        if (changed) void load();
        return true;
      },
      selectNode() {
        selected = true;
        dom.classList.add('ProseMirror-selectednode');
        resizeControls();
      },
      deselectNode() {
        selected = false;
        dom.classList.remove('ProseMirror-selectednode');
        cancelDrag();
        resizeControls();
      },
      stopEvent: (event) =>
        event.target === refresh ||
        event.target === resize ||
        controls.contains(event.target as globalThis.Node),
      ignoreMutation: () => true,
      destroy() {
        disposed = true;
        clearRetry();
        cancelDrag();
        request++;
        stop();
        saved?.removeEventListener('saved', retryMissing);
        image.onload = image.onerror = null;
        image.removeAttribute('src');
      },
    };
  };
}
