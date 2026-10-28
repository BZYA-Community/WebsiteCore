<template>
  <section class="author-columns">
    <n-alert v-if="error" type="error">
      {{ t('columns.loadFailed') }}
      <n-button @click="load">{{ t('columns.retry') }}</n-button>
    </n-alert>
    <n-spin :show="loading">
      <n-empty v-if="!loading && !error && !items.length" :description="t('common.noData')" />
      <div v-for="item in items" :key="item.id" class="author-column-row">
        <n-image v-if="item.kind === 'collection' && item.cover" :src="item.cover" width="100" />
        <h2><router-link :to="{ name: 'columns', query: { view: 'read', id: item.id } }">{{ item.title }}</router-link></h2>
        <n-space align="center">
          <span>{{ formatPrettyTime(item.published_on) }}</span>
          <n-tag v-if="item.official" type="warning" size="small">{{ t('columns.official') }}</n-tag>
        </n-space>
        <p>{{ item.summary }}</p>
      </div>
    </n-spin>
    <n-pagination v-if="total > 20" v-model:page="page" :page-size="20" :item-count="total" :page-slot="5" @update:page="load" />
  </section>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { listColumns, type ColumnItem } from '@/api/columns';
import { formatPrettyTime } from '@/utils/formatTime';

const props = defineProps<{
  authorId: number;
  kind: 'article' | 'collection';
}>();
const { t } = useI18n();
const items = ref<ColumnItem[]>([]),
  total = ref(0),
  page = ref(1),
  loading = ref(false),
  error = ref(false);
let requestId = 0;

async function load() {
  const current = ++requestId;
  if (props.authorId <= 0) {
    items.value = [];
    total.value = 0;
    loading.value = false;
    return;
  }
  loading.value = true;
  error.value = false;
  try {
    const result = await listColumns({
      author_id: props.authorId,
      kind: props.kind,
      page: page.value,
      page_size: 20,
    });
    if (current !== requestId) return;
    items.value = result.items;
    total.value = result.total;
  } catch {
    if (current === requestId) error.value = true;
  } finally {
    if (current === requestId) loading.value = false;
  }
}

watch(
  () => [props.authorId, props.kind],
  () => {
    page.value = 1;
    items.value = [];
    total.value = 0;
    void load();
  },
  { immediate: true },
);
onBeforeUnmount(() => {
  requestId++;
});
</script>

<style scoped>
.author-columns { padding: 16px; min-width: 0; }
.author-column-row { padding: 20px 0; border-bottom: 1px solid var(--n-border-color, #ddd); }
h2 { font-size: 18px; line-height: 1.5; overflow-wrap: anywhere; }
a { color: var(--primary-color); text-decoration: none; }
p { white-space: pre-wrap; overflow-wrap: anywhere; }
.n-pagination { margin-top: 16px; }
</style>
