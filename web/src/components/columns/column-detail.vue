<template>
  <n-spin :show="loading">
    <n-alert v-if="error" type="error">{{ t('columns.loadFailed') }} <n-button @click="load">{{ t('columns.retry') }}</n-button></n-alert>
    <article v-else-if="item">
      <n-image v-if="item.kind === 'collection' && item.cover" :src="item.cover" width="200" />
      <h1>{{ item.title }}</h1>
      <div class="column-meta-row">
        <n-space class="column-author-info" align="center"><router-link :to="{ name: 'user', query: { s: item.user?.username } }">{{ item.user?.nickname || item.user?.username }}</router-link>
          <span>{{ t('columns.publishedAt') }} {{ formatPrettyTime(item.published_on) }}</span>
          <span v-if="item.updated_on > item.published_on">{{ t('columns.updatedAt') }} {{ formatPrettyTime(item.updated_on) }}</span>
          <n-tag v-if="item.official" type="warning" size="small">{{ t('columns.official') }}</n-tag>
        </n-space>
        <n-button class="column-share" size="small" type="primary" secondary @click="share">{{ t('columns.share') }}</n-button>
      </div>
      <p v-if="item.summary" class="column-summary">{{ item.summary }}</p>
      <n-space v-if="canEdit || canOrganize || manager || (owner && !item.curated)" class="column-actions">
        <n-button v-if="canEdit" size="small" @click="edit">{{ t('columns.edit') }}</n-button>
        <n-button v-if="canOrganize" size="small" @click="organize">{{ t('columns.organize') }}</n-button>
        <n-button v-if="item.kind === 'article' && (canEdit || manager)" size="small" @click="action(item.locked ? 'unlock' : 'lock')">{{ item.locked ? t('columns.unlock') : t('columns.lock') }}</n-button>
        <n-button v-if="item.kind === 'article' && manager" size="small" @click="action(item.official ? 'uninvite' : 'invite')">{{ item.official ? t('columns.uninvite') : t('columns.invite') }}</n-button>
        <n-popconfirm v-if="manager" @positive-click="remove('takedown')"><template #trigger><n-button size="small" type="warning">{{ t('columns.takedown') }}</n-button></template>{{ t('columns.confirmRemoval') }}</n-popconfirm>
        <column-delete-button v-if="manager || (owner && !item.curated)" :item="item" @deleted="afterDelete" />
      </n-space>
      <column-content v-if="item.kind === 'article'" :body="item.body" :attachments="item.attachments" :column-id="item.id" />
      <template v-else>
        <n-empty v-if="!entries.length" :description="t('common.noData')" />
        <ol class="collection-articles"><li v-for="entry in entries" :key="entry.id"><router-link :to="{ name: 'columns', query: { view: 'read', id: entry.id } }">{{ entry.title }}</router-link><span> · {{ entry.user?.nickname || entry.user?.username }}</span></li></ol>
      </template>
      <section v-if="item.kind === 'article'" class="column-discussion">
        <h2>{{ t('post.postDetail.commentTitle') }}</h2>
        <n-select v-model:value="sort" :options="sorts" style="width: 160px" @update:value="resetComments" />
        <compose-comment :post-id="0" :column-id="id" :lock="item.locked ? 1 : 0" @post-success="resetComments" />
        <n-alert v-if="commentError" type="error">{{ t('columns.loadFailed') }} <n-button @click="loadComments">{{ t('columns.retry') }}</n-button></n-alert>
        <comment-item v-for="comment in comments" :key="`${sort}-${comment.id}-${commentVersion}`" :comment="comment" :post-user-id="item.user_id" :column-id="id" :locked="item.locked" @reload="resetComments" />
        <n-empty v-if="!commentLoading && !commentError && !comments.length" :description="t('post.postDetail.emptyComments')" />
        <n-button v-if="commentLoading || comments.length < commentTotal" :loading="commentLoading" @click="loadComments">{{ t('columns.more') }}</n-button>
      </section>
    </article>
  </n-spin>
</template>
<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue';
import { useRouter, useRoute } from 'vue-router';
import { useI18n } from 'vue-i18n';
import { useStoreUser } from '@/store/user';
import {
  columnAction,
  columnEntries,
  getColumn,
  getColumnComments,
  type ColumnItem,
} from '@/api/columns';
import { formatPrettyTime } from '@/utils/formatTime';
import ColumnContent from './column-content.vue';
import ColumnDeleteButton from './column-delete-button.vue';
const props = defineProps<{ id: number }>();
const { t } = useI18n();
const user = useStoreUser(),
  router = useRouter(),
  route = useRoute();
const item = ref<ColumnItem>(),
  entries = ref<ColumnItem[]>([]),
  loading = ref(true),
  error = ref(false);
const owner = computed(() => item.value?.user_id === user.userInfo.id);
const manager = computed(() => user.hasPermission('content.manage'));
const canEdit = computed(
  () =>
    user.hasPermission('column.publish') &&
    (!item.value?.curated || manager.value) &&
    (owner.value || (manager.value && item.value?.kind === 'collection')),
);
const canOrganize = computed(
  () =>
    item.value?.kind === 'collection' &&
    (!item.value.curated || manager.value) &&
    (manager.value || (owner.value && user.hasPermission('column.publish'))),
);
const sort = ref('default');
const sorts = computed(() => [
  { label: t('post.postDetail.sortRecommended'), value: 'default' },
  { label: t('post.postDetail.sortHot'), value: 'hots' },
  { label: t('post.postDetail.sortNewest'), value: 'newest' },
]);
const comments = ref<Item.CommentProps[]>([]),
  commentTotal = ref(0),
  commentLoading = ref(false),
  commentError = ref(false),
  commentVersion = ref(0);
let page = 1,
  revision = 0,
  active = true;
async function load() {
  loading.value = true;
  error.value = false;
  try {
    const result = await getColumn(props.id);
    if (!active) return;
    item.value = result.item;
    if (result.item.kind === 'collection') {
      const response = await columnEntries(props.id);
      if (active) entries.value = response.items;
    } else await resetComments();
  } catch {
    if (active) error.value = true;
  } finally {
    if (active) loading.value = false;
  }
}
async function loadComments() {
  if (commentLoading.value) return;
  const current = revision;
  commentLoading.value = true;
  commentError.value = false;
  try {
    const result = await getColumnComments(props.id, sort.value, page);
    if (!active || current !== revision) return;
    comments.value.push(...result.items);
    commentTotal.value = result.total;
    page++;
    await nextTick();
    const target = route.query.comment_id;
    if (
      target &&
      !document.getElementById(`comment-${target}`) &&
      !document.getElementById(`reply-${target}`) &&
      comments.value.length < commentTotal.value
    ) {
      commentLoading.value = false;
      await loadComments();
    } else if (target)
      (
        document.getElementById(`comment-${target}`) ||
        document.getElementById(`reply-${target}`)
      )?.scrollIntoView({ block: 'center' });
  } catch {
    if (active && current === revision) commentError.value = true;
  } finally {
    if (active && current === revision) commentLoading.value = false;
  }
}
async function resetComments() {
  revision++;
  page = 1;
  comments.value = [];
  commentTotal.value = 0;
  commentLoading.value = false;
  commentVersion.value++;
  await loadComments();
}
async function action(value: string) {
  try {
    await columnAction(props.id, value);
    if (active) await load();
  } catch {
    /* Preserve the current detail after a failed action. */
  }
}
async function remove(value: string) {
  try {
    await columnAction(props.id, value);
    if (active) await router.push({ name: 'columns' });
  } catch {
    /* Do not navigate away after a failed removal. */
  }
}
function afterDelete() {
  if (active) router.push({ name: 'columns' });
}
function edit() {
  router.push({
    name: 'columns',
    query: { view: 'edit', id: props.id, kind: item.value?.kind },
  });
}
function organize() {
  router.push({ name: 'columns', query: { view: 'entries', id: props.id } });
}
async function share() {
  try {
    await navigator.clipboard.writeText(window.location.href);
    window.$message.success(t('columns.copied'));
  } catch {
    window.$message.info(window.location.href);
  }
}
onMounted(load);
onBeforeUnmount(() => {
  active = false;
  revision++;
});
</script>
<style scoped>
h1 { font-size: 24px; line-height: 1.5; overflow-wrap: anywhere; }
h2 { font-size: 18px; }
a { color: var(--primary-color); }
.column-meta-row { display: flex; align-items: flex-start; gap: 12px; }
.column-author-info { flex: 1; min-width: 0; min-height: 28px; }
.column-share { flex-shrink: 0; }
.column-summary { white-space: pre-wrap; opacity: .8; }
.column-actions { margin: 18px 0; }
.collection-articles { padding-left: 24px; }
.collection-articles li { padding: 12px 0; }
.column-discussion { border-top: 1px solid var(--n-border-color, #ddd); margin-top: 24px; padding-top: 12px; }
</style>
