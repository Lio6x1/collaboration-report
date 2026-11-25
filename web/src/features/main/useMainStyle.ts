import { ref, watch } from 'vue';
import type { MainView, StyleChange } from '../../../../contracts/http/generated';
import { useAction } from '../../lib/http';
import { mainStylePath } from './api';

export function useMainStyle(
  current: () => MainView,
  reload: () => Promise<void>,
  prepare: () => Promise<unknown>,
) {
  const action = useAction();
  const font = ref<StyleChange['style']['body_font']>(current().style.style.body_font);
  watch(
    () => current().style.id,
    () => {
      font.value = current().style.style.body_font;
    },
  );
  async function applyStyle() {
    const view = current();
    const style: StyleChange['style'] = { ...view.style.style, body_font: font.value };
    let prepared = false;
    await action.run(async () => {
      if (!prepared) {
        await prepare();
        prepared = true;
      }
      // 只修改用户选择的字段，其余冻结排版及原样式/control期待值完整保留。
      // 重试由useAction沿首次固定参数续接，不以新视图悄悄登记另一个样式动作。
      await action.mutate(mainStylePath(view.document.id), {
        expected_style_id: view.style.id,
        expected_control_revision: view.document.control_revision,
        style,
      });
      await reload();
    }, '正文样式已更新，固定送审轮次保留原样式');
  }
  return { action, font, applyStyle };
}
