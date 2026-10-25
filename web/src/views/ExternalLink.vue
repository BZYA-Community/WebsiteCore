<template>
  <div>
    <main-nav :title="t('externalLink.pageTitle')" />
    <main class="external-link-page main-content-wrap">
      <n-result
        status="warning"
        :title="t('externalLink.title')"
        :description="t('externalLink.warning')"
      >
        <template #footer>
          <div class="external-link-target">
            <span>{{ t('externalLink.destination') }}</span>
            <strong>{{ targetHost || t('externalLink.invalidUrl') }}</strong>
            <code v-if="targetUrl">{{ targetUrl }}</code>
          </div>
          <n-space justify="center">
            <n-button @click="cancel">{{ t('common.cancel') }}</n-button>
            <n-button type="warning" :disabled="!targetUrl" @click="confirm">
              {{ t('common.confirm') }}
            </n-button>
          </n-space>
        </template>
      </n-result>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRoute, useRouter } from 'vue-router';
import { normalizeExternalUrl } from '@/utils/external-link';

const route = useRoute();
const router = useRouter();
const { t } = useI18n();

const rawTarget = computed(() => {
  const value = route.query.url;
  return Array.isArray(value) ? value[0] || '' : value || '';
});
const targetUrl = computed(() => normalizeExternalUrl(rawTarget.value));
const targetHost = computed(() => {
  try {
    return targetUrl.value ? new URL(targetUrl.value).host : '';
  } catch {
    return '';
  }
});

function cancel() {
  if (window.history.length > 1) router.back();
  else void router.replace({ name: 'home' });
}

function confirm() {
  if (targetUrl.value) window.location.assign(targetUrl.value);
}
</script>

<style scoped lang="less">
.external-link-page {
  min-height: 520px;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 36px 24px;
  box-sizing: border-box;
}

.external-link-target {
  width: min(560px, 76vw);
  display: grid;
  gap: 8px;
  margin: 0 auto 24px;
  text-align: left;

  span { color: var(--n-text-color-3); }
  strong { font-size: 16px; overflow-wrap: anywhere; }
  code {
    max-height: 92px;
    overflow: auto;
    padding: 10px 12px;
    border-radius: 8px;
    background: rgba(128, 128, 128, 0.12);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
}
</style>
