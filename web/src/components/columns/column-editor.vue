<template>
  <n-spin :show="loading">
    <n-alert v-if="error" type="error">{{ t('columns.loadFailed') }} <n-button @click="load">{{ t('columns.retry') }}</n-button></n-alert>
    <template v-else>
      <n-alert v-if="!canPublish" type="info">{{ t('identity.accessDenied') }}</n-alert>
      <n-alert v-if="pending" type="warning">{{ t('columns.pendingHint') }}</n-alert>
      <n-alert v-if="row?.submission?.reason" type="info">{{ row.submission.reason }}</n-alert>
      <n-alert v-if="draftConflict" type="warning">{{ t('columns.draftConflict') }}
        <n-space><n-button @click="restoreConflict">{{ t('columns.restoreLocal') }}</n-button><n-button @click="discard">{{ t('columns.discardLocal') }}</n-button></n-space>
      </n-alert>
      <n-alert v-if="storageError" type="warning">{{ t('columns.storageFailed') }}</n-alert>
      <n-alert v-if="sourceId" type="info">{{ t('columns.recreateHint') }}</n-alert>
      <n-form label-placement="top" class="column-form" :disabled="readonly || saving">
        <n-form-item :label="t('columns.titleLabel')" required><n-input v-model:value="draft.title" :maxlength="200" show-count /></n-form-item>
        <n-form-item v-if="kind === 'collection'" :label="t('columns.introduction')"><n-input v-model:value="draft.summary" type="textarea" :maxlength="2000" show-count /></n-form-item>
        <template v-if="kind === 'article'">
          <label class="import-label"><span>{{ t('columns.import') }}</span><input type="file" accept=".md,text/markdown,text/plain" :disabled="readonly || saving" @change="importFile"></label>
          <md-editor v-model="draft.body" :theme="mdTheme(main.theme)" :language="locale === 'en' ? 'en-US' : 'zh-CN'"
            :disabled="readonly || saving" :toolbars-exclude="excluded" no-mermaid no-html />
          <p :class="{ 'length-error': length > MD_MAX_LENGTH }">{{ length }} / {{ MD_MAX_LENGTH }}</p>
          <column-media v-model="draft.attachments" :disabled="readonly || saving" @busy="uploading = $event" />
        </template>
        <template v-else>
          <p>{{ t('columns.cover') }}</p>
          <column-media v-model="coverMedia" image-only :disabled="readonly || saving" @busy="uploading = $event" />
        </template>
      </n-form>
      <p class="draft-tip">{{ t('columns.localDraftHint') }}</p>
      <div class="editor-actions">
        <n-button v-if="pending && canPublish" key="withdraw" :loading="saving" @click="withdraw">{{ t('columns.withdraw') }}</n-button>
        <n-button v-if="!pending && canPublish && !row?.blocked" key="submit" type="primary" :loading="saving" :disabled="loading || uploading || !!draftConflict || !valid" @click="submit">{{ t('columns.submit') }}</n-button>
        <n-button v-if="!pending" key="discard" @click="discard">{{ t('columns.discardLocal') }}</n-button>
      </div>
    </template>
  </n-spin>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { useI18n } from 'vue-i18n';
import { MdEditor, type ToolbarNames } from 'md-editor-v3';
import 'md-editor-v3/lib/style.css';
import { MD_MAX_LENGTH, mdTheme } from '@/utils/markdown';
import { useStoreUser } from '@/store/user';
import { useStoreMain } from '@/store/main';
import {
  columnAction,
  getColumn,
  getReturnedColumn,
  submitColumn,
  type ColumnItem,
  type ColumnDraft,
  type ColumnMedia,
} from '@/api/columns';
import ColumnMediaInput from './column-media.vue';

const ColumnMedia = ColumnMediaInput;
const props = defineProps<{
  id: number;
  kind: 'article' | 'collection';
  draftId: string;
  sourceId?: number;
}>();
const { t, locale } = useI18n();
const user = useStoreUser();
const main = useStoreMain();
const router = useRouter();
const route = useRoute();
const draft = ref<ColumnDraft>({
  title: '',
  summary: '',
  body: '',
  cover: '',
  attachments: [],
});
const row = ref<ColumnItem>();
const draftConflict = ref<ColumnDraft>();
const loading = ref(true),
  error = ref(false),
  saving = ref(false),
  uploading = ref(false),
  storageError = ref(false);
let active = true;
const key = computed(
  () =>
    `columns:draft:${user.userInfo.id}:${props.kind}:${props.id || props.draftId}`,
);
const pending = computed(() => row.value?.submission?.audit_status === 0);
const canPublish = computed(
  () =>
    user.userLogined &&
    user.hasPermission('column.publish') &&
    (!row.value?.curated || user.hasPermission('content.manage')),
);
const readonly = computed(
  () => !canPublish.value || pending.value || !!row.value?.blocked,
);
const length = computed(() => [...draft.value.body].length);
const valid = computed(
  () =>
    draft.value.title.trim() &&
    (props.kind === 'collection' || draft.value.body.trim()) &&
    length.value <= MD_MAX_LENGTH,
);
const excluded: ToolbarNames[] = [
  'mermaid',
  'katex',
  'image',
  'github',
  'htmlPreview',
  'fullscreen',
];
const coverMedia = computed<ColumnMedia[]>({
  get: () =>
    draft.value.cover ? [{ content: draft.value.cover, type: 3, sort: 1 }] : [],
  set: (items) => {
    draft.value.cover = items[0]?.content || '';
  },
});
function serverDraft(item?: ColumnItem): ColumnDraft {
  const source = item?.submission || item;
  return {
    title: source?.title || '',
    summary: source?.summary || '',
    body: source?.body || '',
    cover: source?.cover || '',
    attachments: source?.attachments || [],
  };
}
async function load() {
  loading.value = true;
  error.value = false;
  try {
    row.value = props.id ? (await getColumn(props.id, true)).item : undefined;
    if (row.value && row.value.kind !== props.kind) {
      await router.replace({ name: 'columns', query: { ...route.query, kind: row.value.kind } });
      return;
    }
    const source = !props.id && props.sourceId ? (await getReturnedColumn(props.sourceId)).item : row.value;
    draft.value = serverDraft(source);
    if (!active) return;
    try {
      const local = localStorage.getItem(key.value);
      if (local && !pending.value) {
        const saved = JSON.parse(local);
        if (saved.draft && typeof saved.draft.body === 'string') {
          if (saved.expectedSubmission === (row.value?.submission_id || 0))
            draft.value = saved.draft;
          else draftConflict.value = saved.draft;
        }
      }
    } catch {
      storageError.value = true;
    }
  } catch {
    error.value = true;
  } finally {
    loading.value = false;
  }
}
watch(
  draft,
  () => {
    if (loading.value || readonly.value || error.value || draftConflict.value)
      return;
    try {
      localStorage.setItem(
        key.value,
        JSON.stringify({
          id: props.id,
          kind: props.kind,
          draftId: props.draftId,
          sourceId: props.sourceId || 0,
          expectedSubmission: row.value?.submission_id || 0,
          draft: draft.value,
        }),
      );
      storageError.value = false;
    } catch {
      storageError.value = true;
    }
  },
  { deep: true },
);
async function importFile(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  if (!file) return;
  try {
    if (
      !file.name.toLowerCase().endsWith('.md') ||
      file.size > 4 * MD_MAX_LENGTH
    )
      throw new Error('size');
    const body = await file.text();
    if ([...body].length > MD_MAX_LENGTH) throw new Error('size');
    draft.value.body = body;
    if (!draft.value.title.trim())
      draft.value.title = file.name.slice(0, -3).slice(0, 200);
  } catch {
    window.$message.warning(t('columns.importFailed'));
  } finally {
    input.value = '';
  }
}
async function submit() {
  if (
    saving.value ||
    uploading.value ||
    readonly.value ||
    !!draftConflict.value ||
    !valid.value
  )
    return;
  saving.value = true;
  const submittedKey = key.value;
  try {
    const payload = {
      ...draft.value,
      summary: props.kind === 'article' ? '' : draft.value.summary.trim(),
      id: props.id,
      kind: props.kind,
      expected_submission: row.value?.submission_id || 0,
      recreated_from_id: !props.id ? props.sourceId || 0 : 0,
    };
    const result = await submitColumn(payload);
    try {
      localStorage.removeItem(submittedKey);
    } catch {
      /* Publication already succeeded; storage cannot undo it. */
    }
    if (!active) return;
    window.$message.success(
      result.item.submission?.audit_status === 0
        ? t('columns.submitted')
        : t('columns.published'),
    );
    await router.push({
      name: 'columns',
      query: { tab: 'mine', kind: props.kind },
    });
  } catch {
    // Keep the editor and local changes available for retry.
  } finally {
    saving.value = false;
  }
}
async function withdraw() {
  if (saving.value || !canPublish.value) return;
  saving.value = true;
  try {
    await columnAction(props.id, 'withdraw');
    await load();
  } catch {
    // Keep the editor and local changes available for retry.
  } finally {
    saving.value = false;
  }
}
function restoreConflict() {
  if (!draftConflict.value || readonly.value) return;
  draft.value = draftConflict.value;
  draftConflict.value = undefined;
}
function discard() {
  draftConflict.value = undefined;
  try {
    localStorage.removeItem(key.value);
  } catch {
    storageError.value = true;
  }
  loading.value = true;
  if (!props.id && props.sourceId) {
    void load();
    return;
  }
  draft.value = serverDraft(row.value);
  queueMicrotask(() => {
    loading.value = false;
  });
}
onMounted(load);
onBeforeUnmount(() => {
  active = false;
});
</script>

<style scoped>
.editor-actions { display: flex; flex-wrap: wrap; gap: 8px; }
.column-form { margin-top: 18px; }
.draft-tip { color: var(--n-text-color, #888); font-size: 12px; }
.import-label { display: flex; flex-wrap: wrap; gap: 12px; margin: 12px 0; }
.length-error { color: #d03050; }
</style>
