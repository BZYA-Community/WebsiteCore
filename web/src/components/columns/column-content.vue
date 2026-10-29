<template>
  <div class="column-content">
    <md-preview v-if="body" :model-value="body" :theme="mdTheme(main.theme)" :language="locale === 'en' ? 'en-US' : 'zh-CN'" no-mermaid no-html />
    <n-image-group><n-space><n-image v-for="item in images" :key="item.content" :src="item.content" width="160" /></n-space></n-image-group>
    <video-player v-for="item in videos" :key="item.content" :src="item.content" />
    <n-space v-if="files.length" class="column-files">
      <n-button v-for="item in files" :key="item.content" size="small" @click="download(item.index)">{{ t('post.attachment') }} {{ item.index + 1 }}</n-button>
    </n-space>
  </div>
</template>
<script setup lang="ts">
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import { MdPreview } from 'md-editor-v3';
import { mdTheme } from '@/utils/markdown';
import { useStoreMain } from '@/store/main';
import { request } from '@/utils/request';
import type { ColumnMedia } from '@/api/columns';
import VideoPlayer from '@/components/video-player.vue';
const props = withDefaults(
  defineProps<{
    body: string;
    attachments?: ColumnMedia[];
    columnId?: number;
    submissionId?: number;
    returned?: boolean;
  }>(),
  { attachments: () => [], columnId: 0, submissionId: 0, returned: false },
);
const { t, locale } = useI18n();
const main = useStoreMain();
const images = computed(() =>
  props.attachments.filter((item) => item.type === 3),
);
const videos = computed(() =>
  props.attachments.filter((item) => item.type === 4),
);
const files = computed(() =>
  props.attachments
    .map((item, index) => ({ ...item, index }))
    .filter((item) => item.type === 7),
);
async function download(index: number) {
  const result = await request<unknown, { signed_url: string }>({
    method: 'get',
    url: props.returned ? '/v1/column/returned/attachment' : '/v1/column/attachment',
    params: { id: props.columnId, index, submission_id: props.submissionId },
  });
  window.open(result.signed_url, '_blank', 'noopener,noreferrer');
}
</script>
<style scoped>
.column-content { overflow-wrap: anywhere; min-width: 0; }
.column-files { margin: 16px 0; }
</style>
