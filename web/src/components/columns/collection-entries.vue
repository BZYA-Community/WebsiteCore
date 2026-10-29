<template>
  <n-spin :show="loading">
    <n-alert v-if="error" type="error">{{ t('columns.loadFailed') }} <n-button @click="load">{{ t('columns.retry') }}</n-button></n-alert>
    <template v-else-if="collection">
      <h2>{{ t('columns.organize') }} · {{ collection.title || collection.submission?.title }}</h2>
      <p>{{ t('columns.orderHint') }}</p>
      <ol class="ordered-entries"><li v-for="(item, index) in entries" :key="item.id">
        <span>{{ item.title }}</span>
        <n-space><n-button size="tiny" :disabled="!index || saving" @click="move(index, -1)">{{ t('columns.moveUp') }}</n-button>
          <n-button size="tiny" :disabled="index === entries.length - 1 || saving" @click="move(index, 1)">{{ t('columns.moveDown') }}</n-button>
          <n-button size="tiny" :disabled="saving" @click="entries.splice(index, 1)">{{ t('columns.remove') }}</n-button></n-space>
      </li></ol>
      <n-button type="primary" :loading="saving" :disabled="!allowed" @click="save">{{ t('columns.saveOrder') }}</n-button>
      <n-divider />
      <n-input-group><n-input v-model:value="keyword" :placeholder="t('columns.search')" @keyup.enter="search" /><n-button @click="search">{{ t('columns.search') }}</n-button></n-input-group>
      <n-spin :show="searching"><ul class="candidates"><li v-for="item in candidates" :key="item.id"><span>{{ item.title }} · {{ item.user?.nickname || item.user?.username }}</span><n-button size="small" :disabled="!allowed || entries.some(entry => entry.id === item.id)" @click="entries.push(item)">{{ t('columns.add') }}</n-button></li></ul></n-spin>
      <n-pagination v-if="total > 20" v-model:page="page" :page-size="20" :item-count="total" @update:page="loadCandidates" />
    </template>
  </n-spin>
</template>
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { useStoreUser } from '@/store/user';
import {
  getColumn,
  columnEntries,
  setColumnEntries,
  listColumns,
  type ColumnItem,
} from '@/api/columns';
const props = defineProps<{ id: number }>();
const { t } = useI18n();
const user = useStoreUser();
const collection = ref<ColumnItem>(),
  entries = ref<ColumnItem[]>([]),
  candidates = ref<ColumnItem[]>([]);
const loading = ref(true),
  error = ref(false),
  saving = ref(false),
  searching = ref(false),
  keyword = ref(''),
  page = ref(1),
  total = ref(0);
const allowed = computed(
  () =>
    !collection.value?.blocked &&
    (!collection.value?.curated || user.hasPermission('content.manage')) &&
    (user.hasPermission('content.manage') ||
      (collection.value?.user_id === user.userInfo.id &&
        user.hasPermission('column.publish'))),
);
async function load() {
  loading.value = true;
  error.value = false;
  try {
    collection.value = (await getColumn(props.id, true)).item;
    entries.value = (await columnEntries(props.id, true)).items;
    await loadCandidates();
  } catch {
    error.value = true;
  } finally {
    loading.value = false;
  }
}
async function loadCandidates() {
  searching.value = true;
  try {
    const result = await listColumns({
      kind: 'article',
      author_id: collection.value?.curated
        ? undefined
        : collection.value?.user_id,
      keyword: keyword.value,
      page: page.value,
      page_size: 20,
    });
    candidates.value = result.items;
    total.value = result.total;
  } catch {
    error.value = true;
  } finally {
    searching.value = false;
  }
}
function search() {
  page.value = 1;
  return loadCandidates();
}
function move(index: number, delta: number) {
  const target = index + delta;
  [entries.value[index], entries.value[target]] = [
    entries.value[target],
    entries.value[index],
  ];
}
async function save() {
  if (!allowed.value || saving.value) return;
  saving.value = true;
  try {
    await setColumnEntries(
      props.id,
      entries.value.map((item) => item.id),
    );
    window.$message.success(t('columns.saved'));
  } catch {
    // Keep the current selection and order for retry.
  } finally {
    saving.value = false;
  }
}
onMounted(load);
</script>
<style scoped>
.ordered-entries { padding-left: 24px; }
.ordered-entries li { margin: 16px 0; overflow-wrap: anywhere; }
.candidates { list-style: none; padding: 0; }
.candidates li { display: flex; gap: 12px; justify-content: space-between; padding: 12px 0; border-bottom: 1px solid var(--n-border-color, #ddd); }
</style>
