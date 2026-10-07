<template>
  <n-card :title="title" size="small" class="contact-card">
    <n-space align="center" justify="space-between">
      <span>{{ currentAddress || t('setting.contact.notBound') }}</span>
      <n-tag :type="storeUser.userInfo.contact_verified ? 'success' : 'warning'" size="small">{{ storeUser.userInfo.contact_verified ? t('setting.contact.verified') : t('setting.contact.unverified') }}</n-tag>
    </n-space>
    <p>{{ mode === 'email' ? t('setting.contact.emailHelp') : t('setting.contact.phoneHelp') }}</p>
    <n-alert v-if="!profile.contactVerificationAvailable" type="warning">{{ t('setting.contact.unavailable') }}</n-alert>
    <n-form v-else label-placement="top" @submit.prevent="verify">
      <n-form-item :label="title" required>
        <n-input v-model:value="address" :input-props="{ 'aria-label': title, type: mode === 'email' ? 'email' : 'tel', autocomplete: mode === 'email' ? 'email' : 'tel' }" :maxlength="mode === 'email' ? 254 : 32" :disabled="sending || verifying" :placeholder="title" />
      </n-form-item>
      <n-form-item :label="t('setting.contact.code')" required>
        <div class="code-row">
          <n-input v-model:value="code" :input-props="{ 'aria-label': t('setting.contact.code'), inputmode: 'numeric', autocomplete: 'one-time-code' }" :maxlength="8" :disabled="verifying" />
          <n-button :loading="sending" :disabled="verifying || countdown > 0 || !address.trim()" @click="sendCode">{{ countdown ? t('setting.contact.resend', { seconds: countdown }) : t('setting.contact.send') }}</n-button>
        </div>
      </n-form-item>
      <n-alert v-if="error" type="error" class="contact-message">{{ error }}</n-alert>
      <n-button attr-type="submit" type="primary" :loading="verifying" :disabled="sending || !address.trim() || !code.trim()">{{ t('setting.contact.verify') }}</n-button>
    </n-form>
  </n-card>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { useStoreUser } from '@/store/user';
import { useStoreProfile } from '@/store/profile';
import { request } from '@/utils/request';

const { t } = useI18n();
const storeUser = useStoreUser();
const storeProfile = useStoreProfile();
const profile = computed(() => storeProfile.profile);
const mode = computed(() => profile.value.accountVerifyMode);
const title = computed(() => mode.value === 'email' ? t('setting.contact.email') : t('setting.contact.phone'));
const currentAddress = computed(() => mode.value === 'email' ? storeUser.userInfo.email : storeUser.userInfo.phone);
const address = ref('');
const code = ref('');
const error = ref('');
const sending = ref(false);
const verifying = ref(false);
const countdown = ref(0);
let timer: ReturnType<typeof setInterval> | undefined;
async function sendCode() {
  if (!address.value.trim() || sending.value || countdown.value) return;
  sending.value = true; error.value = '';
  try {
    await request({ method: 'post', url: '/v1/user/contact/code', data: { mode: mode.value, address: address.value.trim() } });
    window.$message.success(t('setting.contact.sent'));
    const deadline = Date.now() + 60000;
    countdown.value = 60;
    timer = setInterval(() => { countdown.value = Math.max(0, Math.ceil((deadline - Date.now()) / 1000)); if (!countdown.value) clearInterval(timer); }, 1000);
  } catch { error.value = t('setting.contact.sendFailed'); }
  finally { sending.value = false; }
}
async function verify() {
  if (!address.value.trim() || !code.value.trim() || verifying.value) return;
  verifying.value = true; error.value = '';
  try {
    await request({ method: 'post', url: '/v1/user/contact/verify', data: { mode: mode.value, address: address.value.trim(), code: code.value.trim() } });
    await storeUser.loadSession(true);
    code.value = ''; address.value = '';
    window.$message.success(t('setting.contact.success'));
  } catch { error.value = t('setting.contact.verifyFailed'); }
  finally { verifying.value = false; }
}
onBeforeUnmount(() => clearInterval(timer));
</script>

<style scoped>
.contact-card { margin-top: -1px; border-radius: 0; }
.code-row { display: flex; gap: 8px; width: 100%; }
.contact-message { margin-bottom: 12px; }
p { line-height: 1.6; }
@media (max-width: 420px) { .code-row { flex-wrap: wrap; } }
</style>
