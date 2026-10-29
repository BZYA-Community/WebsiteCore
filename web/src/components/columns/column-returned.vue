<template>
  <n-spin :show="loading">
    <n-alert v-if="error" type="error">{{ t('columns.loadFailed') }} <n-button @click="load">{{ t('columns.retry') }}</n-button></n-alert>
    <article v-else-if="item" class="returned-column">
      <h1>{{ item.title }}</h1>
      <n-alert type="warning" :title="t('columns.returned')">
        <p>{{ t('columns.returnNotice') }}</p>
        <p class="return-reason">{{ t('columns.deleteReason') }}：{{ item.admin_delete_reason }}</p>
        <span>{{ t('columns.deletedAt') }} {{ formatPrettyTime(item.admin_deleted_on || 0) }}</span>
      </n-alert>
      <n-space class="return-actions">
        <n-button @click="copy">{{ t('columns.copyMarkdown') }}</n-button>
        <n-button @click="download">{{ t('columns.downloadMarkdown') }}</n-button>
        <n-button v-if="user.hasPermission('column.publish')" type="primary" @click="recreate">{{ t('columns.recreate') }}</n-button>
      </n-space>
      <column-content :body="item.body" :attachments="item.attachments" :column-id="item.id" returned />
    </article>
  </n-spin>
</template>
<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRouter } from 'vue-router';
import { useStoreUser } from '@/store/user';
import { getReturnedColumn, type ColumnItem } from '@/api/columns';
import { formatPrettyTime } from '@/utils/formatTime';
import ColumnContent from './column-content.vue';
const props = defineProps<{ id: number }>();
const { t } = useI18n(), router = useRouter(), user = useStoreUser();
const item = ref<ColumnItem>(), loading = ref(true), error = ref(false);
let requestId = 0;
async function load() {
  const current = ++requestId;
  loading.value = true;
  error.value = false;
  try {
    const result = await getReturnedColumn(props.id);
    if (current === requestId) item.value = result.item;
  } catch {
    if (current === requestId) error.value = true;
  } finally {
    if (current === requestId) loading.value = false;
  }
}
async function copy() {
  try {
    await navigator.clipboard.writeText(item.value?.body || '');
    window.$message.success(t('columns.markdownCopied'));
  } catch {
    window.$message.info(t('columns.downloadMarkdown'));
  }
}
function download() {
  if (!item.value) return;
  const url = URL.createObjectURL(new Blob([item.value.body], { type: 'text/markdown;charset=utf-8' }));
  const link = document.createElement('a');
  link.href = url;
  link.download = (item.value.title.replace(/[\\/:*?"<>|]/g, '_') || 'article') + '.md';
  link.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
function recreate() {
  router.push({ name: 'columns', query: { view: 'edit', kind: 'article', source: props.id, draft: crypto.randomUUID() } });
}
onMounted(load);
onBeforeUnmount(() => { requestId++; });
</script>
<style scoped>
h1 { font-size: 24px; overflow-wrap: anywhere; }
.return-reason { white-space: pre-wrap; overflow-wrap: anywhere; }
.return-actions { margin: 18px 0; }
</style>
