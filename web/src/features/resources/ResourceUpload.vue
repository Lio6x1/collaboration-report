<script setup lang="ts">
import { computed } from 'vue';
import type { Resource } from '../../../../contracts/http/generated';
import ActionStatus from '../../components/ActionStatus.vue';
import { useResourceUpload } from './useResourceUpload';

const props = defineProps<{ document: string; space: string; disabled?: boolean }>();
const emit = defineEmits<{ ready: [resource: Resource] }>();
const {
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
} = useResourceUpload(props, (resource) => emit('ready', resource));
defineExpose({ paste });
const available = computed(() => {
  if (!quota.value) return '0';
  const remaining =
    BigInt(quota.value.limit_bytes) -
    BigInt(quota.value.used_bytes) -
    BigInt(quota.value.reserved_bytes);
  return (remaining > 0n ? remaining : 0n).toString();
});

function select(event: Event) {
  const input = event.target as HTMLInputElement;
  selected.value = input.files?.[0] ?? null;
  input.value = '';
}
function bytes(value: string) {
  const count = BigInt(value);
  const tenths = (count * 10n) / 1048576n;
  return count < 1048576n ? count.toString() + ' 字节' : `${tenths / 10n}.${tenths % 10n} MiB`;
}
</script>

<template>
  <section class="resource-upload" aria-label="上传正文图片">
    <p class="hint">
      可在下方正文中直接按 Ctrl+V 粘贴图片，或选择文件上传。支持 PNG、JPEG、GIF、WebP，单张不超过 32
      MiB。
    </p>
    <p v-if="quota" class="hint">
      空间容量：可用 {{ bytes(available) }}，已用 {{ bytes(quota.used_bytes) }}，上传中占用
      {{ bytes(quota.reserved_bytes) }}，总容量 {{ bytes(quota.limit_bytes) }}。
    </p>
    <p v-if="quotaError" class="error" role="alert">配额读取失败：{{ quotaError }}</p>
    <button type="button" :disabled="quotaBusy || locked" @click="refreshQuota">
      {{ quotaBusy ? '正在读取配额…' : '刷新配额' }}
    </button>
    <label>
      选择正文图片
      <input
        type="file"
        accept="image/png,image/jpeg,image/gif,image/webp"
        :disabled="locked || hasJob"
        @change="select"
      />
    </label>
    <p v-if="selected">已选：{{ selected.name }}（{{ bytes(String(selected.size)) }}）</p>
    <button type="button" :disabled="locked || !selected || !quota || !!completed" @click="upload">
      {{ hasJob ? '重试原图片上传' : '上传图片' }}
    </button>
    <button v-if="selected || hasJob" type="button" :disabled="locked" @click="clear">
      清除当前选择
    </button>
    <button v-if="completed" type="button" :disabled="locked" @click="insert">
      将已上传图片插入正文
    </button>
    <p v-if="stage" role="status" aria-live="polite">{{ stage }}</p>
    <ActionStatus
      :busy="action.busy.value"
      :error="action.error.value"
      :notice="action.notice.value"
      :pending="action.pending.value"
      :retry="action.retry.value"
    />
  </section>
</template>
