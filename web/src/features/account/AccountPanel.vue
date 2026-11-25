<script setup lang="ts">
import {
  UserFacingError,
  api,
  ApiError,
  useAction,
  actionsBlocked,
  passwordPolicyMessage as passwordRule,
} from '../../lib/http';
import { computed, onMounted, onUnmounted, ref, watch } from 'vue';
import type { Challenge, ChallengeCreate } from '../../../../contracts/http/generated';
import { user, prepareLogin, afterCredentialChange, validNewPassword } from './session';
import { accountNumber } from './identity';
import ActionStatus from '../../components/ActionStatus.vue';
import AccountClosurePanel from './AccountClosurePanel.vue';
const emit = defineEmits<{ succession: [kind: string, target: string] }>();
const copyingNumber = ref(false),
  numberNotice = ref('');
watch(
  () => user.value?.id,
  () => {
    numberNotice.value = '';
  },
);
async function copyAccountNumber() {
  if (!user.value || copyingNumber.value) return;
  const actor = user.value.id;
  copyingNumber.value = true;
  numberNotice.value = '';
  try {
    await navigator.clipboard.writeText(accountNumber(actor));
    if (user.value?.id === actor) numberNotice.value = '账号编号已复制';
  } catch {
    if (user.value?.id === actor) numberNotice.value = '暂时无法复制，请选中账号编号手动复制。';
  } finally {
    copyingNumber.value = false;
  }
}
const mode = ref<'login' | 'signup' | 'reset'>('login'),
  email = ref(''),
  password = ref(''),
  confirmPassword = ref(''),
  displayName = ref(''),
  code = ref(''),
  challenge = ref<Challenge | null>(null),
  challengeDelivery = ref(''),
  oldPassword = ref(''),
  newPassword = ref(''),
  newEmail = ref(''),
  emailCode = ref(''),
  emailChallenge = ref<Challenge | null>(null),
  emailChallengeDelivery = ref(''),
  sendingCode = ref(false),
  sendingEmailCode = ref(false),
  action = useAction();
const { busy, error, notice, pending, retry, write, mutate, publicMutation } = action;
type ChallengePurpose = ChallengeCreate['purpose'];
const cooldownStorageKey = 'collaboration-report.challenge-cooldowns.v1';
const now = ref(Date.now()),
  cooldowns = ref(readCooldowns()),
  codeCooldown = computed(() =>
    remainingCooldown(mode.value === 'signup' ? 'SIGNUP' : 'RESET_PASSWORD', email.value),
  ),
  emailCodeCooldown = computed(() => remainingCooldown('CHANGE_EMAIL', newEmail.value));
let cooldownTimer: ReturnType<typeof setInterval> | undefined;

function cooldownKey(purpose: ChallengePurpose, target: string) {
  // 与服务端的邮箱规范化一致；轻量散列只为避免缓存邮箱明文，不能代替服务端限流。
  let hash = 2166136261;
  for (const character of target.trim().toLowerCase()) {
    hash = Math.imul(hash ^ character.codePointAt(0)!, 16777619);
  }
  return purpose + ':' + (hash >>> 0).toString(16);
}
function readCooldowns(): Record<string, number> {
  const restored: Record<string, number> = {};
  try {
    const saved = JSON.parse(sessionStorage.getItem(cooldownStorageKey) ?? '{}');
    for (const [key, until] of Object.entries(saved)) {
      if (
        /^(SIGNUP|RESET_PASSWORD|CHANGE_EMAIL):[a-f0-9]+$/.test(key) &&
        typeof until === 'number' &&
        Number.isFinite(until) &&
        until > Date.now()
      )
        restored[key] = until;
    }
  } catch {
    // 禁用浏览器存储时仍可在当前页面倒计时，真实频率始终由Redis原子校验。
  }
  return restored;
}
function saveCooldowns() {
  try {
    // 刷新页面只恢复用途和截止时间；密码、验证码、邮箱明文都不进入持久缓存。
    sessionStorage.setItem(cooldownStorageKey, JSON.stringify(cooldowns.value));
  } catch {
    // 存储不可用不应把已受理的验证码误显示为发送失败。
  }
}
function remainingCooldown(purpose: ChallengePurpose, target: string) {
  return Math.max(
    0,
    Math.ceil(((cooldowns.value[cooldownKey(purpose, target)] ?? 0) - now.value) / 1000),
  );
}
function startCooldown(purpose: ChallengePurpose, target: string, seconds = 60) {
  now.value = Date.now();
  cooldowns.value[cooldownKey(purpose, target)] = now.value + seconds * 1000;
  saveCooldowns();
}
onMounted(() => {
  cooldownTimer = setInterval(() => {
    // 依据绝对截止时间重算，后台标签被浏览器节流后也不会多等几十秒。
    now.value = Date.now();
    let changed = false;
    for (const [key, until] of Object.entries(cooldowns.value)) {
      if (until <= now.value) {
        delete cooldowns.value[key];
        changed = true;
      }
    }
    if (changed) saveCooldowns();
  }, 1000);
});
onUnmounted(() => clearInterval(cooldownTimer));

function switchMode(next: 'login' | 'signup' | 'reset') {
  mode.value = next;
  password.value = '';
  confirmPassword.value = '';
  code.value = '';
  challenge.value = null;
  challengeDelivery.value = '';
  error.value = '';
  notice.value = '';
}
function deliveryMessage(headers: Headers, purpose: string) {
  // 只解释服务端枚举；不能从收件地址是否@qq.com推断发信通道或实际送达。
  const delivery = headers.get('X-Collaboration-Mail-Delivery');
  if (delivery === 'DEVELOPMENT')
    return '当前使用本地开发收件箱，邮件不会发送到外部邮箱。请从开发收件箱获取验证码。';
  if (delivery === 'SMTP')
    return purpose === 'RESET_PASSWORD'
      ? '若该邮箱已注册，验证码会通过邮件服务器投递。请查看收件箱和垃圾邮件。'
      : '验证码已交给邮件服务器，请查看填写的邮箱及垃圾邮件。收件地址不限QQ邮箱。';
  return '验证码请求已受理，请按当前环境的邮件设置查看收件箱。';
}
async function requestChallenge(
  target: string,
  purpose: ChallengePurpose,
  delivery: typeof challengeDelivery,
) {
  try {
    const issued = await api<Challenge>(
      '/v1/account/challenges',
      'POST',
      { email: target, purpose },
      (headers) => (delivery.value = deliveryMessage(headers, purpose)),
    );
    startCooldown(purpose, target, issued.retry_after_seconds || 60);
    return issued;
  } catch (reason) {
    if (reason instanceof ApiError && reason.code === 'CHALLENGE_RATE_LIMITED') {
      // 其他页面可能已发送；429没有精确TTL，保守等待60秒且绝不能显示发送成功。
      startCooldown(purpose, target);
      throw new UserFacingError('验证码请求过于频繁，请等待倒计时结束后重试。');
    }
    if (reason instanceof ApiError && reason.code === 'SMTP_DELIVERY_FAILED') {
      // 服务端先占用频率额度再投递；失败仍需等待原额度到期，避免连续重试制造邮件压力。
      startCooldown(purpose, target);
      throw new UserFacingError('邮件服务器暂时未能接收验证码，请等待倒计时结束后重新发送。');
    }
    if (reason instanceof ApiError && reason.code === 'CHALLENGE_STORE_UNAVAILABLE')
      throw new UserFacingError('验证码服务暂时不可用，请稍后重试。');
    throw reason;
  }
}
async function sendCode() {
  if (actionsBlocked.value || codeCooldown.value > 0) return;
  sendingCode.value = true;
  try {
    await action.run(async () => {
      challenge.value = null;
      challengeDelivery.value = '';
      // 重发会绑定新的挑战编号，不能继续保留上一封邮件的验证码输入。
      code.value = '';
      const purpose = mode.value === 'signup' ? 'SIGNUP' : 'RESET_PASSWORD';
      challenge.value = await requestChallenge(email.value, purpose, challengeDelivery);
    }, '验证码请求已受理，请查看投递提示');
  } finally {
    sendingCode.value = false;
  }
}
// 注册成功后还要建立真实Cookie会话；写请求丢响应时用相同编号重取结果，再续接登录。
// 密码和验证码只留在本组件内存，不进入持久浏览器存储。
async function submit() {
  if (actionsBlocked.value) return;
  // 固定登录续接在run绑定身份代次之前创建；重试只补登录，不轮换槽或重发注册。
  const establishSession =
    mode.value === 'reset' ? null : prepareLogin(email.value, password.value);
  await action.run(
    async () => {
      if (mode.value !== 'login' && !validNewPassword(password.value))
        throw new UserFacingError(passwordRule);
      if (mode.value === 'login') {
        await establishSession!();
      } else if (mode.value === 'signup') {
        // 确认密码只在前端阻止误输，不增加后端DTO字段，也不进入幂等写入的固定输入。
        if (password.value !== confirmPassword.value)
          throw new UserFacingError('两次输入的密码不一致，请重新确认。');
        if (!challenge.value) throw new UserFacingError('请先获取注册验证码');
        await write(
          () => '/v1/account/signup',
          () => ({
            email: email.value,
            password: password.value,
            display_name: displayName.value,
            challenge_id: challenge.value!.challenge_id,
            code: code.value,
          }),
        );
        await establishSession!();
      } else {
        if (!challenge.value) throw new UserFacingError('请先获取找回验证码');
        await publicMutation(
          () => '/v1/account/password-resets',
          () => ({
            challenge_id: challenge.value!.challenge_id,
            code: code.value,
            new_password: password.value,
          }),
        );
        mode.value = 'login';
      }
      password.value = '';
      confirmPassword.value = '';
      code.value = '';
      challenge.value = null;
    },
    mode.value === 'reset' ? '密码已更新，请重新登录' : '已进入工作台',
  );
}
async function changePassword() {
  if (!user.value) return;
  await action.run(async () => {
    if (!validNewPassword(newPassword.value)) throw new UserFacingError(passwordRule);
    await mutate(
      () => '/v1/account/password',
      () => ({
        expected_credential_epoch: user.value!.credential_epoch,
        current_password: oldPassword.value,
        new_password: newPassword.value,
      }),
    );
    oldPassword.value = '';
    newPassword.value = '';
    afterCredentialChange();
  }, '密码已更新，旧会话已关闭，请重新登录');
}
async function sendEmailCode() {
  if (actionsBlocked.value || emailCodeCooldown.value > 0) return;
  sendingEmailCode.value = true;
  try {
    await action.run(async () => {
      emailChallenge.value = null;
      emailChallengeDelivery.value = '';
      emailCode.value = '';
      emailChallenge.value = await requestChallenge(
        newEmail.value,
        'CHANGE_EMAIL',
        emailChallengeDelivery,
      );
    }, '新邮箱验证码请求已受理，请查看投递提示');
  } finally {
    sendingEmailCode.value = false;
  }
}
async function changeEmail() {
  if (!user.value || !emailChallenge.value) return;
  await action.run(async () => {
    await mutate(
      () => '/v1/account/email',
      () => ({
        expected_credential_epoch: user.value!.credential_epoch,
        new_email: newEmail.value,
        challenge_id: emailChallenge.value!.challenge_id,
        code: emailCode.value,
      }),
    ).catch((reason) => {
      // 同一占用事实在注册时引导登录，在已登录的修改邮箱场景应引导选择其他邮箱。
      if (reason instanceof ApiError && reason.code === 'EMAIL_ALREADY_REGISTERED')
        throw new UserFacingError('该邮箱已被其他账号使用，请更换邮箱后重试。');
      throw reason;
    });
    emailCode.value = '';
    afterCredentialChange();
  }, '邮箱已更新，请重新登录');
}
</script>
<template>
  <AccountClosurePanel @succession="(kind, target) => emit('succession', kind, target)" />
  <section v-if="!user" class="auth-layout">
    <div class="auth-copy">
      <span class="eyebrow">共同完成一份有条理的报告</span>
      <h1>共同写作，<br /><span>每一步都有迹可循。</span></h1>
      <p>从知识库组织文档，用明确的权限分配协作范围。每一次管理变更都能追踪到原操作。</p>
      <!-- 纯装饰的纸页与批注构成产品识别，不伪装成用户的真实文档或实时协作状态。 -->
      <div class="auth-illustration" aria-hidden="true">
        <div class="illustration-back"></div>
        <div class="illustration-page">
          <span class="illustration-kicker">COLLABORATION / 共同创作</span>
          <strong>从一个想法<br />到一份完整报告</strong>
          <div class="illustration-lines"><i></i><i></i><i></i></div>
          <span class="illustration-note">讨论 · 修订 · 定稿</span>
          <span class="illustration-rule"></span>
          <span class="illustration-number">01 / 一起完成</span>
        </div>
        <div class="illustration-label"><span></span>清楚的权限，可靠的版本</div>
      </div>
    </div>
    <div class="panel auth-card">
      <div class="tabs" role="tablist">
        <button
          :disabled="actionsBlocked"
          role="tab"
          @click="switchMode('login')"
          :aria-selected="mode === 'login'"
          :class="{ selected: mode === 'login' }"
        >
          登录</button
        ><button
          :disabled="actionsBlocked"
          role="tab"
          @click="switchMode('signup')"
          :aria-selected="mode === 'signup'"
          :class="{ selected: mode === 'signup' }"
        >
          注册</button
        ><button
          :disabled="actionsBlocked"
          role="tab"
          @click="switchMode('reset')"
          :aria-selected="mode === 'reset'"
          :class="{ selected: mode === 'reset' }"
        >
          找回密码
        </button>
      </div>
      <h2>
        {{ mode === 'login' ? '欢迎回来' : mode === 'signup' ? '创建账号' : '验证邮箱并找回密码' }}
      </h2>
      <p v-if="mode === 'login'" class="hint">本标签页独立登录，其他标签页可以使用不同账号。</p>
      <form @submit.prevent="submit">
        <fieldset :disabled="actionsBlocked">
          <label
            >{{ mode === 'login' ? '登录邮箱' : '验证邮箱'
            }}<input
              v-model="email"
              type="email"
              required
              autocomplete="email"
              @input="challenge = null" /></label
          ><label v-if="mode === 'signup'"
            >显示名称<input v-model="displayName" required maxlength="160" autocomplete="nickname"
          /></label>
          <template v-if="mode !== 'login'"
            ><button
              type="button"
              :disabled="actionsBlocked || codeCooldown > 0"
              :aria-busy="sendingCode"
              @click="sendCode"
            >
              {{
                sendingCode
                  ? '发送中…'
                  : codeCooldown > 0
                    ? codeCooldown + ' 秒后重新发送'
                    : '发送验证码'
              }}</button
            ><label
              >邮箱验证码<input
                v-model="code"
                required
                inputmode="numeric"
                pattern="[0-9]{6}"
                autocomplete="one-time-code"
            /></label>
            <p v-if="challenge" class="hint">
              {{ challengeDelivery }}
            </p>
            <p v-if="challenge" class="hint">请填写本页面最近一次发送的邮件中的验证码。</p>
            <p v-if="challenge" class="hint">
              验证码有效至
              <!-- zh-CN只指定语言；显式固定时区，避免浏览器环境把截止时间显示成东京等时刻。 -->
              {{
                new Date(challenge.expires_at).toLocaleString('zh-CN', {
                  timeZone: 'Asia/Shanghai',
                  hour12: false,
                })
              }}（北京时间）；用途与邮箱必须一致。
            </p></template
          >
          <label
            >{{ mode === 'login' ? '登录密码' : '新密码'
            }}<input
              v-model="password"
              type="password"
              required
              :minlength="mode === 'login' ? 1 : 8"
              :autocomplete="mode === 'login' ? 'current-password' : 'new-password'"
          /></label>
          <label v-if="mode === 'signup'"
            >确认密码<input
              v-model="confirmPassword"
              type="password"
              required
              minlength="8"
              autocomplete="new-password"
          /></label>
          <p v-if="mode !== 'login'" class="hint">{{ passwordRule }}</p>
          <button class="primary" :disabled="actionsBlocked" type="submit">
            {{ mode === 'login' ? '登录' : mode === 'signup' ? '验证并注册' : '更新密码' }}
          </button>
        </fieldset>
      </form>
      <ActionStatus
        v-if="!pending"
        :busy="busy"
        :error="error"
        :notice="notice"
        :pending="pending"
        :retry="retry"
      />
    </div>
  </section>
  <section v-else class="account-grid">
    <div class="panel">
      <h2>账号信息</h2>
      <p>{{ user.display_name }}</p>
      <p>{{ user.email }}</p>
      <p>
        账号编号：<span class="account-number">{{ accountNumber(user.id) }}</span>
      </p>
      <button
        type="button"
        :disabled="copyingNumber"
        :aria-busy="copyingNumber"
        @click="copyAccountNumber"
      >
        {{ copyingNumber ? '复制中…' : '复制账号编号' }}
      </button>
      <p v-if="numberNotice" role="status">{{ numberNotice }}</p>
    </div>
    <div class="panel">
      <h2>修改密码</h2>
      <form @submit.prevent="changePassword">
        <fieldset :disabled="actionsBlocked">
          <label
            >当前密码<input
              v-model="oldPassword"
              type="password"
              required
              autocomplete="current-password" /></label
          ><label
            >修改后的密码<input
              v-model="newPassword"
              type="password"
              required
              minlength="8"
              autocomplete="new-password" /></label
          ><button :disabled="actionsBlocked">更新密码并关闭旧会话</button>
          <p class="hint">{{ passwordRule }}</p>
        </fieldset>
      </form>
    </div>
    <div class="panel">
      <h2>修改邮箱</h2>
      <form @submit.prevent="changeEmail">
        <fieldset :disabled="actionsBlocked">
          <label
            >新邮箱<input
              v-model="newEmail"
              type="email"
              required
              @input="emailChallenge = null" /></label
          ><button
            type="button"
            :disabled="actionsBlocked || emailCodeCooldown > 0"
            :aria-busy="sendingEmailCode"
            @click="sendEmailCode"
          >
            {{
              sendingEmailCode
                ? '发送中…'
                : emailCodeCooldown > 0
                  ? emailCodeCooldown + ' 秒后重新发送'
                  : '验证新邮箱'
            }}</button
          ><label
            >新邮箱验证码<input
              v-model="emailCode"
              inputmode="numeric"
              required
              pattern="[0-9]{6}" /></label
          ><button :disabled="busy || !emailChallenge">确认更新邮箱</button>
          <p v-if="emailChallenge" class="hint">{{ emailChallengeDelivery }}</p>
        </fieldset>
      </form>
    </div>
    <ActionStatus
      v-if="!pending"
      :busy="busy"
      :error="error"
      :notice="notice"
      :pending="pending"
      :retry="retry"
    />
  </section>
</template>
