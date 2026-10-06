<template>
  <div class="review-content">
    <template v-for="item in ordered" :key="item.id">
      <p v-if="[1, 2, 9].includes(item.type)">{{ item.content }}</p>
      <n-image v-else-if="item.type === 3 && safeUrl(item.content)" :src="safeUrl(item.content)" :alt="t('adminAudit.contentType.image')" width="240" />
      <video v-else-if="item.type === 4 && safeUrl(item.content)" :src="safeUrl(item.content)" controls preload="metadata" />
      <audio v-else-if="item.type === 5 && safeUrl(item.content)" :src="safeUrl(item.content)" controls preload="metadata" />
      <a v-else-if="safeUrl(item.content)" :href="safeUrl(item.content)" target="_blank" rel="noopener noreferrer">{{ item.content }}</a>
      <p v-else>{{ item.content }}</p>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
const props = defineProps<{ contents: Api.Admin.NetReq.AuditPostItem['contents'] }>();
const { t } = useI18n();
const ordered = computed(() => [...props.contents].sort((a, b) => a.sort - b.sort));
function safeUrl(value: string) {
  try { const url = new URL(value, window.location.origin); return ['http:', 'https:'].includes(url.protocol) ? url.href : ''; }
  catch { return ''; }
}
</script>

<style scoped>
.review-content { display: grid; gap: 16px; }
p { white-space: pre-wrap; overflow-wrap: anywhere; line-height: 1.7; margin: 0; }
video, audio { max-width: 100%; }
a { overflow-wrap: anywhere; }
</style>
