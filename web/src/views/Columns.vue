<template>
  <div>
    <main-nav :title="t('nav.columns')" :back="!!view" />
    <n-card size="small" class="main-content-wrap columns-page">
      <n-button v-if="view" quaternary @click="back">{{ t('columns.back') }}</n-button>
      <column-editor v-if="view === 'edit' && user.userLogined" :id="id" :key="screenKey" :kind="kind" :draft-id="draftId" />
      <collection-entries v-else-if="view === 'entries' && user.userLogined" :id="id" :key="screenKey" />
      <column-detail v-else-if="view === 'read'" :id="id" :key="screenKey" />
      <template v-else>
        <n-tabs :value="tab" @update:value="changeTab">
          <n-tab name="articles">{{ t('columns.articles') }}</n-tab><n-tab name="collections">{{ t('columns.collections') }}</n-tab>
          <n-tab v-if="user.userLogined" name="mine">{{ t('columns.mine') }}</n-tab>
          <n-tab v-if="user.hasPermission('content.manage')" name="manage">{{ t('columns.manage') }}</n-tab>
        </n-tabs>
        <n-space v-if="tab === 'mine' || tab === 'manage'" class="list-controls" align="center">
          <n-select :value="kind" :options="kinds" style="width: 150px" @update:value="changeKind" />
          <n-button v-if="user.hasPermission('column.publish')" type="primary" @click="create">{{ kind === 'article' ? t('columns.newArticle') : t('columns.newCollection') }}</n-button>
        </n-space>
        <template v-if="tab === 'mine' && drafts.length">
          <h2>{{ t('columns.localDrafts') }}</h2>
          <ul><li v-for="draft in drafts" :key="draft.key"><n-button text @click="openDraft(draft)">{{ draft.title || t('columns.untitled') }}</n-button><n-popconfirm @positive-click="deleteDraft(draft.key)"><template #trigger><n-button size="tiny" quaternary>{{ t('columns.delete') }}</n-button></template>{{ t('columns.confirmDraftDelete') }}</n-popconfirm></li></ul>
        </template>
        <form class="list-controls" @submit.prevent="search">
          <n-input-group v-if="kind === 'article'"><n-input v-model:value="keyword" :placeholder="t('columns.search')" clearable /><n-button attr-type="submit">{{ t('columns.search') }}</n-button></n-input-group>
          <n-checkbox v-if="kind === 'article'" v-model:checked="official" @update:checked="search">{{ t('columns.officialOnly') }}</n-checkbox>
        </form>
        <n-alert v-if="error" type="error">{{ t('columns.loadFailed') }} <n-button @click="load">{{ t('columns.retry') }}</n-button></n-alert>
        <n-spin :show="loading">
          <n-empty v-if="!loading && !error && !items.length" :description="t('common.noData')" />
          <div v-for="item in items" :key="item.id" class="column-row">
            <n-image v-if="item.kind === 'collection' && item.cover" :src="item.cover" width="100" />
            <h2><router-link v-if="item.published_on && !item.blocked" :to="{ name: 'columns', query: { view: 'read', id: item.id } }">{{ item.title }}</router-link><span v-else>{{ item.submission?.title || item.title }}</span></h2>
            <n-space align="center"><span>{{ item.user?.nickname || item.user?.username }}</span><span v-if="item.published_on">{{ formatPrettyTime(item.published_on) }}</span><n-tag v-if="item.official" type="warning" size="small">{{ t('columns.official') }}</n-tag><n-tag v-if="item.blocked" type="error" size="small">{{ t('columns.blocked') }}</n-tag></n-space>
            <p>{{ item.summary || item.submission?.summary }}</p>
            <n-space v-if="tab === 'mine' || tab === 'manage'">
              <n-tag v-if="item.submission" size="small">{{ statusText(item.submission.audit_status) }}</n-tag>
              <n-button v-if="canEdit(item)" size="small" @click="edit(item)">{{ t('columns.edit') }}</n-button>
              <n-button v-if="item.kind === 'collection' && canOrganize(item)" size="small" @click="organize(item)">{{ t('columns.organize') }}</n-button>
              <n-button v-if="item.blocked && user.hasPermission('content.manage')" size="small" @click="act(item, 'restore')">{{ t('columns.restore') }}</n-button>
              <n-popconfirm v-if="!item.curated || user.hasPermission('content.manage')" @positive-click="act(item, 'delete')"><template #trigger><n-button size="small" type="error">{{ t('columns.delete') }}</n-button></template>{{ t('columns.confirmRemoval') }}</n-popconfirm>
            </n-space>
          </div>
        </n-spin>
        <n-pagination v-if="total > 20" v-model:page="page" :page-size="20" :item-count="total" @update:page="load" />
      </template>
    </n-card>
  </div>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { useI18n } from 'vue-i18n';
import { useStoreUser } from '@/store/user';
import { listColumns, columnAction, type ColumnItem } from '@/api/columns';
import { formatPrettyTime } from '@/utils/formatTime';
import ColumnDetail from '@/components/columns/column-detail.vue';
import ColumnEditor from '@/components/columns/column-editor.vue';
import CollectionEntries from '@/components/columns/collection-entries.vue';
const { t } = useI18n();
const user = useStoreUser(),
  route = useRoute(),
  router = useRouter();
const view = computed(() => String(route.query.view || ''));
const id = computed(() => Number(route.query.id || 0));
const tab = computed(() => String(route.query.tab || 'articles'));
const kind = computed<'article' | 'collection'>(() =>
  route.query.kind === 'collection' || tab.value === 'collections'
    ? 'collection'
    : 'article',
);
const draftId = computed(() => String(route.query.draft || 'new'));
const screenKey = computed(
  () =>
    `${user.userInfo.id}:${view.value}:${id.value}:${kind.value}:${draftId.value}`,
);
const kinds = computed(() => [
  { label: t('columns.articles'), value: 'article' },
  { label: t('columns.collections'), value: 'collection' },
]);
const items = ref<ColumnItem[]>([]),
  total = ref(0),
  page = ref(1),
  keyword = ref(''),
  official = ref(false),
  loading = ref(false),
  error = ref(false);
interface LocalDraft {
  key: string;
  id: number;
  kind: string;
  draftId: string;
  title: string;
}
const drafts = ref<LocalDraft[]>([]);
let requestId = 0;
function loadDrafts() {
  drafts.value = [];
  try {
    const prefix = `columns:draft:${user.userInfo.id}:${kind.value}:`;
    for (let i = 0; i < localStorage.length; i++) {
      const key = localStorage.key(i);
      if (!key?.startsWith(prefix)) continue;
      try {
        const value = JSON.parse(localStorage.getItem(key) || '{}');
        if (value.draft)
          drafts.value.push({
            key,
            id: value.id,
            kind: value.kind,
            draftId: value.draftId,
            title: value.draft.title,
          });
      } catch {
        /* Ignore a damaged local entry without affecting other drafts. */
      }
    }
  } catch {
    /* Browsing remains available when browser storage is disabled. */
  }
}
async function load() {
  if (view.value) return;
  const current = ++requestId;
  loading.value = true;
  error.value = false;
  loadDrafts();
  try {
    const result = await listColumns({
      kind: kind.value,
      keyword: keyword.value,
      official: official.value,
      mine: tab.value === 'mine',
      manage: tab.value === 'manage',
      page: page.value,
      page_size: 20,
    });
    if (current === requestId) {
      items.value = result.items;
      total.value = result.total;
    }
  } catch {
    if (current === requestId) error.value = true;
  } finally {
    if (current === requestId) loading.value = false;
  }
}
function search() {
  page.value = 1;
  return load();
}
function changeTab(value: string) {
  keyword.value = '';
  official.value = false;
  return router.push({ name: 'columns', query: { tab: value } });
}
function changeKind(value: string) {
  return router.push({
    name: 'columns',
    query: { tab: tab.value, kind: value },
  });
}
function back() {
  return router.push({
    name: 'columns',
    query: { tab: user.userLogined ? 'mine' : 'articles', kind: kind.value },
  });
}
function create() {
  return router.push({
    name: 'columns',
    query: { view: 'edit', kind: kind.value, draft: crypto.randomUUID() },
  });
}
function edit(item: ColumnItem) {
  return router.push({
    name: 'columns',
    query: { view: 'edit', kind: item.kind, id: item.id },
  });
}
function organize(item: ColumnItem) {
  return router.push({
    name: 'columns',
    query: { view: 'entries', kind: 'collection', id: item.id },
  });
}
function openDraft(draft: LocalDraft) {
  return router.push({
    name: 'columns',
    query: {
      view: 'edit',
      kind: draft.kind,
      id: draft.id || undefined,
      draft: draft.draftId,
    },
  });
}
function deleteDraft(key: string) {
  localStorage.removeItem(key);
  loadDrafts();
}
function canEdit(item: ColumnItem) {
  return (
    !item.blocked &&
    (!item.curated || user.hasPermission('content.manage')) &&
    user.hasPermission('column.publish') &&
    (item.user_id === user.userInfo.id ||
      (item.kind === 'collection' && user.hasPermission('content.manage')))
  );
}
function canOrganize(item: ColumnItem) {
  return (
    !item.blocked &&
    (!item.curated || user.hasPermission('content.manage')) &&
    (user.hasPermission('content.manage') ||
      (item.user_id === user.userInfo.id &&
        user.hasPermission('column.publish')))
  );
}
async function act(item: ColumnItem, action: string) {
  try {
    await columnAction(item.id, action);
    await load();
  } catch {
    /* Keep the list available; the request layer reports the failure. */
  }
}
function statusText(status: number) {
  return status === 0
    ? t('columns.pending')
    : status === 1
      ? t('columns.approved')
      : status === 2
        ? t('columns.rejected')
        : t('columns.withdrawn');
}
watch(
  () => [route.fullPath, user.userInfo.id],
  () => {
    requestId++;
    page.value = 1;
    void load();
  },
  { immediate: true },
);
onBeforeUnmount(() => {
  requestId++;
});
</script>
<style scoped>
.columns-page { box-sizing: border-box; }
.list-controls { margin: 16px 0; }
.column-row { padding: 20px 0; border-bottom: 1px solid var(--n-border-color, #ddd); }
h2 { font-size: 18px; line-height: 1.5; overflow-wrap: anywhere; }
a { color: var(--primary-color); text-decoration: none; }
p { white-space: pre-wrap; overflow-wrap: anywhere; }
</style>
