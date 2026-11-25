import { computed, ref } from 'vue';
import type {
  MainView,
  StructureChange,
  StructurePreview,
  StructurePreviewCreate,
} from '../../../../contracts/http/generated';
import { useAction } from '../../lib/http';
import { structureChangePath, structurePreviewPath } from './api';

type RangeType = 'SPLIT_LEVEL2' | 'JOIN_CHAPTER' | 'REMOVE';
type PreviewInput = Omit<StructurePreviewCreate, 'operation_id'>;
type FixedPreview = {
  response: StructurePreview;
  input: PreviewInput;
  title: string;
  type: RangeType;
};
export const rangeNames: Record<RangeType, string> = {
  SPLIT_LEVEL2: '按二级标题拆分',
  JOIN_CHAPTER: '合为整章单元',
  REMOVE: '删除整章范围',
};

export function useStructurePreview(
  current: () => MainView,
  reload: () => Promise<void>,
  prepare: () => Promise<unknown>,
  completed: () => void = () => {},
) {
  const action = useAction();
  const chapters = computed(() =>
    current().structure.entries.filter((entry) => entry.kind === 'CHAPTER'),
  );
  const chapter = ref(chapters.value[0]?.chapter_id ?? '');
  const type = ref<RangeType>('SPLIT_LEVEL2');
  const fixed = ref<FixedPreview | null>(null);
  const acknowledged = ref(false);

  async function preview() {
    const view = current();
    const selectedType = type.value;
    const selectedTitle =
      chapters.value.find((entry) => entry.chapter_id === chapter.value)?.title ?? '';
    const input: PreviewInput = {
      expected_structure_id: view.structure.id,
      expected_control_revision: view.document.control_revision,
      changes: [{ type: selectedType, chapter_id: chapter.value }],
    };
    let prepared = false;
    acknowledged.value = false;
    await action.run(async () => {
      if (!prepared) {
        await prepare();
        prepared = true;
      }
      // 预览POST未知时仍重取首次原号的完整固定预览，不生成另一个候选。
      const response = await action.write<StructurePreview>(
        structurePreviewPath(view.document.id),
        input,
      );
      fixed.value = { response, input, title: selectedTitle, type: selectedType };
    }, '预览已固定，请核对范围后明确确认');
  }

  async function confirm() {
    const original = fixed.value;
    if (!original || !acknowledged.value || original.response.blocking_units.length) return;
    const document = current().document.id;
    const input: Omit<StructureChange, 'operation_id'> = {
      ...original.input,
      changes: original.input.changes.map((change) => ({
        ...change,
        preview_cut_ref: original.response.preview_cut_ref,
      })),
    };
    let prepared = false;
    await action.run(async () => {
      // 预览后的本机输入先可靠保存，服务端明确CHANGED；不能静默重采预览覆盖它。
      if (!prepared) {
        await prepare();
        prepared = true;
      }
      await action.mutate(structureChangePath(document), input);
      await reload();
      completed();
    }, '目录范围已调整，共同支线已安装');
  }

  function discardPreview() {
    // 只关闭本机展示，原预览和历史仍不可变；再次生成是用户明确的新动作。
    fixed.value = null;
    acknowledged.value = false;
  }

  return { action, chapters, chapter, type, fixed, acknowledged, preview, confirm, discardPreview };
}
