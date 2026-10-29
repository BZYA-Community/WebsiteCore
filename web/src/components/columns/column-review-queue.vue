<template>
  <section class="column-review">
    <n-button :loading="loading" @click="load">{{ t('adminAudit.task.refresh') }}</n-button>
    <n-alert v-if="error" type="error">{{ t('columns.loadFailed') }}</n-alert>
    <n-spin :show="loading">
      <n-empty v-if="!loading && !error && !items.length" :description="t('common.noData')" />
      <n-card v-for="item in items" :key="`${item.task.id}:${item.task.revision}`" size="small" class="review-item" :title="item.submission?.title || item.column.title">
        <p>{{ t('columns.reviewTask', { id: item.task.id, author: item.task.author_id, reviewer: item.task.assignee_id || '—' }) }}</p>
        <template v-if="item.submission">
          <h3>{{ t('columns.reviewSubmission') }}</h3>
          <p>{{ item.submission.summary }}</p>
          <n-image v-if="item.submission.cover" :src="item.submission.cover" width="160" />
          <column-content :body="item.submission.body" :attachments="item.submission.attachments" :column-id="item.column.id" :submission-id="item.submission.id" />
          <n-collapse v-if="item.column.published_on"><n-collapse-item :title="t('columns.reviewCurrent')">
            <h3>{{ item.column.title }}</h3><p>{{ item.column.summary }}</p>
            <n-image v-if="item.column.cover" :src="item.column.cover" width="160" />
            <column-content :body="item.column.body" :attachments="item.column.attachments" :column-id="item.column.id" />
          </n-collapse-item></n-collapse>
        </template>
        <template v-if="item.comment">
          <h3>{{ t('columns.reviewComment') }} · {{ item.comment.user?.username }}</h3>
          <p class="comment-text">{{ item.comment.content }}</p>
          <n-image v-for="image in item.comment.contents?.filter(content => content.type === 3)" :key="image.content" :src="image.content" width="160" />
        </template>
        <n-input v-model:value="reasons[item.task.id]" :placeholder="t('columns.reviewReason')" :maxlength="255" />
        <n-space class="decisions">
          <n-button type="success" :loading="busy === item.task.id" :disabled="!canDecide(item) || !!busy" @click="decide(item, 'approve')">{{ t('columns.approve') }}</n-button>
          <n-button type="error" :disabled="!canDecide(item) || !!busy || !reasons[item.task.id]?.trim()" @click="decide(item, 'reject')">{{ t('columns.reject') }}</n-button>
        </n-space>
      </n-card>
    </n-spin>
    <n-pagination v-if="total > 20" v-model:page="page" :page-size="20" :item-count="total" @update:page="load" />
  </section>
</template>
<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { useStoreUser } from '@/store/user';
import {
  listColumnReviews,
  decideColumnReview,
  type ColumnReview,
} from '@/api/columns';
import ColumnContent from './column-content.vue';
const { t } = useI18n();
const user = useStoreUser();
const items = ref<ColumnReview[]>([]),
  total = ref(0),
  page = ref(1),
  loading = ref(false),
  error = ref(false),
  busy = ref(0);
const reasons = ref<Record<number, string>>({});
async function load() {
  loading.value = true;
  error.value = false;
  try {
    const result = await listColumnReviews(page.value);
    items.value = result.items;
    total.value = result.total;
  } catch {
    error.value = true;
  } finally {
    loading.value = false;
  }
}
function canDecide(item: ColumnReview) {
  return (
    user.hasPermission('content.review') &&
    item.task.author_id !== user.userInfo.id &&
    (item.task.assignee_id === user.userInfo.id || user.userInfo.is_operator)
  );
}
async function decide(item: ColumnReview, action: string) {
  if (busy.value || !canDecide(item)) return;
  busy.value = item.task.id;
  try {
    await decideColumnReview(item, action, reasons.value[item.task.id] || '');
    window.$message.success(t('columns.reviewDone'));
    await load();
  } catch {
    // Retain the queue and entered reason; stale tasks can be refreshed.
  } finally {
    busy.value = 0;
  }
}
onMounted(load);
defineExpose({ load });
</script>
<style scoped>
.column-review { margin-top: 16px; }
.review-item { margin: 16px 0; }
.decisions { margin-top: 12px; }
.comment-text { white-space: pre-wrap; overflow-wrap: anywhere; }
</style>
