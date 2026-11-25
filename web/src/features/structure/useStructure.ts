import { computed, ref, watch } from 'vue';
import type {
  ChapterCreate,
  MainView,
  StructureChange,
} from '../../../../contracts/http/generated';
import { useAction } from '../../lib/http';
import { chapterPath, structureChangePath } from './api';

export function useStructure(
  current: () => MainView,
  reload: () => Promise<void>,
  prepare: () => Promise<unknown>,
  completed: () => void = () => {},
) {
  const action = useAction();
  const chapters = computed(() =>
    current().structure.entries.filter((entry) => entry.kind === 'CHAPTER'),
  );
  const newTitle = ref('');
  const addAfter = ref('');
  const chapter = ref(chapters.value[0]?.chapter_id ?? '');
  const title = ref(chapters.value[0]?.title ?? '');
  const moveAfter = ref('');
  watch(chapter, (id) => {
    title.value = chapters.value.find((entry) => entry.chapter_id === id)?.title ?? '';
    moveAfter.value = '';
  });
  watch(chapters, (entries) => {
    if (!entries.some((entry) => entry.chapter_id === chapter.value))
      chapter.value = entries[0]?.chapter_id ?? '';
    if (addAfter.value && !entries.some((entry) => entry.chapter_id === addAfter.value))
      addAfter.value = '';
    if (moveAfter.value && !entries.some((entry) => entry.chapter_id === moveAfter.value))
      moveAfter.value = '';
  });

  async function execute(path: string, input: Record<string, unknown>, notice: string) {
    let prepared = false;
    await action.run(async () => {
      // 目录关闭可能退役编辑器；先完成当前MAIN或BRANCH的IME和本机可靠保存。
      // 后续未知结果只沿原号及固定输入续接，不再次采尾或改用新的目录期待值。
      if (!prepared) {
        await prepare();
        prepared = true;
      }
      await action.mutate(path, input);
      await reload();
      completed();
    }, notice);
  }

  async function addChapter() {
    const view = current();
    const input: Omit<ChapterCreate, 'operation_id'> = {
      expected_structure_id: view.structure.id,
      title: newTitle.value.trim(),
      ...(addAfter.value ? { after_chapter_id: addAfter.value } : {}),
    };
    await execute(chapterPath(view.document.id), input, '章节已新增，共同支线就绪');
  }

  async function changeChapter(type: 'RENAME' | 'MOVE') {
    const view = current();
    const input: Omit<StructureChange, 'operation_id'> = {
      expected_structure_id: view.structure.id,
      expected_control_revision: view.document.control_revision,
      changes: [
        {
          type,
          chapter_id: chapter.value,
          ...(type === 'RENAME'
            ? { title: title.value.trim() }
            : moveAfter.value
              ? { after_chapter_id: moveAfter.value }
              : {}),
        },
      ],
    };
    await execute(
      structureChangePath(view.document.id),
      input,
      type === 'RENAME' ? '章节标题已更新' : '章节顺序已更新',
    );
  }

  return {
    action,
    chapters,
    newTitle,
    addAfter,
    chapter,
    title,
    moveAfter,
    addChapter,
    changeChapter,
  };
}
