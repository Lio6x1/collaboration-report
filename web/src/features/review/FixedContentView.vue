<script setup lang="ts">
import type { Anchor, FixedContent, StyleRevision } from '../../../../contracts/http/generated';
import ContentView from '../../lib/content/ContentView.vue';
import type { ResourceContext } from '../../lib/content/image';
defineProps<{
  content: FixedContent;
  style: StyleRevision;
  resourceContext: ResourceContext;
  selectable?: boolean;
  anchor?: Anchor | null;
}>();
const emit = defineEmits<{ selection: [anchor: Anchor | null] }>();
</script>
<template>
  <ContentView
    class="paper fixed-review-body"
    :content="content"
    :style="style"
    :resource-context="resourceContext"
    label="固定送审正文"
    :selectable="selectable"
    :anchor="anchor"
    @selection="emit('selection', $event)"
  />
</template>
