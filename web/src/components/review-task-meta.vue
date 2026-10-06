<template>
  <div v-if="task" class="review-task" @click.stop>
    <span>{{ t('adminAudit.task.assignee') }}: {{ task.assignee_id ? `#${task.assignee_id}` : t('adminAudit.task.unassigned') }}</span>
    <span v-if="task.deadline_on">{{ t('adminAudit.task.deadline') }}: {{ formatTime(task.deadline_on) }}</span>
    <span>{{ t('adminAudit.task.revision', { revision: task.revision }) }}</span>
    <n-button text size="tiny" @click="showHistory">{{ t('adminAudit.task.history') }}</n-button>
  </div>
  <n-modal v-model:show="show" preset="card" :title="t('adminAudit.task.history')" style="width: min(820px, calc(100vw - 32px)); max-height: calc(100dvh - 32px); overflow-y: auto">
    <n-data-table :columns="columns" :data="history" :loading="loading" :scroll-x="680" size="small" />
  </n-modal>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import type { DataTableColumns } from 'naive-ui';
import { request } from '@/utils/request';
import { formatTime } from '@/utils/formatTime';
interface ReviewEvent { id: number; event: string; actor_id: number; from_assignee_id: number; to_assignee_id: number; reason: string; revision: number; created_on: number }
const props = defineProps<{ task?: Api.Admin.NetReq.ReviewTask }>();
const { t } = useI18n();
const show = ref(false);
const loading = ref(false);
const history = ref<ReviewEvent[]>([]);
async function showHistory() {
  if (!props.task) return;
  show.value = true; loading.value = true;
  try { history.value = (await request<unknown, { items: ReviewEvent[] }>({ method: 'get', url: '/v1/admin/audit/history', params: { task_id: props.task.id } })).items || []; }
  catch { /* Request errors retain the last loaded history. */ }
  finally { loading.value = false; }
}
const columns = computed<DataTableColumns<ReviewEvent>>(() => [
  { title: t('adminAudit.task.event'), key: 'event', width: 130 },
  { title: t('identity.actor'), key: 'actor_id', width: 90 },
  { title: t('adminAudit.task.transfer'), key: 'transfer', width: 130, render: (row) => `${row.from_assignee_id || '—'} → ${row.to_assignee_id || '—'}` },
  { title: t('adminAudit.task.reason'), key: 'reason' },
  { title: t('adminUsers.table.time'), key: 'created_on', width: 160, render: (row) => formatTime(row.created_on) },
]);
</script>

<style scoped>
.review-task { display: flex; flex-wrap: wrap; gap: 6px 14px; font-size: 12px; margin: 10px 0; opacity: .8; }
</style>
