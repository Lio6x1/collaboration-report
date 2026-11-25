<script setup lang="ts">
import { ref } from 'vue';
import { useAction, actionsBlocked } from '../../lib/http';
import { accountIDFromInput } from '../account/identity';
import ActionStatus from '../../components/ActionStatus.vue';
import AppDialog from '../../components/AppDialog.vue';

const props = defineProps<{ document: string; revision: string }>();
const target = ref('');
const visible = ref(false);
const action = useAction();
async function propose() {
  const id = props.document,
    revision = props.revision,
    receiver = accountIDFromInput(target.value);
  if (!receiver) return;
  await action.run(async () => {
    await action.mutate(`/v1/documents/${id}/personal-transfers`, {
      expected_revision: revision,
      target_user_id: receiver,
    });
    visible.value = false;
  }, '转让申请已发送，接受前文档仍归你所有');
}
</script>

<template>
  <section class="management">
    <h3>转交文档</h3>
    <p>转入对方个人空间需要接收人确认；确认前仍可继续使用当前文档。</p>
    <div class="button-row">
      <button :disabled="actionsBlocked" @click="visible = true">转入他人个人空间</button>
      <a href="/transfers" @click="actionsBlocked && $event.preventDefault()">查看个人转让申请</a>
    </div>
    <ActionStatus
      v-if="!visible && !action.pending.value"
      :busy="action.busy.value"
      :error="action.error.value"
      :notice="action.notice.value"
      :pending="action.pending.value"
      :retry="action.retry.value"
    />
    <AppDialog v-model="visible" title="转入他人个人空间" :blocked="actionsBlocked">
      <p class="hint">
        填写接收人的账号编号，可在对方的账号信息中查看。对方接受且容量足够后才迁移归属，接受前可在转让中心取消。
      </p>
      <form class="inline-form" @submit.prevent="propose">
        <label
          >个人转让接收人编号<input
            v-model="target"
            inputmode="numeric"
            pattern="[0-9]{1,20}"
            placeholder="例如 00007764"
            required
            :disabled="actionsBlocked" /></label
        ><button type="submit" :disabled="actionsBlocked || !accountIDFromInput(target)">
          发送个人转让申请
        </button>
      </form>
      <ActionStatus
        v-if="!action.pending.value"
        :busy="action.busy.value"
        :error="action.error.value"
        :notice="action.notice.value"
        :pending="action.pending.value"
        :retry="action.retry.value"
      />
    </AppDialog>
  </section>
</template>
