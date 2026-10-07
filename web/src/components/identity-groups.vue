<template>
  <n-card :title="t('identity.title')" size="small" class="identity-card">
    <p class="identity-help">{{ t('identity.help') }}</p>
    <n-space align="center" justify="space-between">
      <span>{{ t('identity.groupCount', { count: groups.length }) }}</span>
      <n-button type="primary" secondary @click="editGroup()">{{ t('identity.create') }}</n-button>
    </n-space>
    <n-list>
      <n-list-item v-for="group in groups" :key="group.id">
        <div class="group-row">
          <div class="group-name">
            <strong>{{ group.name }}</strong>
            <n-tag v-if="group.builtin" size="small">{{ t('identity.builtin') }}</n-tag>
            <p>{{ t('identity.permissionCount', { count: group.permissions.length }) }}</p>
          </div>
          <n-space>
            <n-button size="small" :disabled="!canManageGroup(group)" @click="editGroup(group)">{{ t('common.edit') }}</n-button>
            <n-popconfirm v-if="!group.builtin" @positive-click="removeGroup(group)" :positive-text="t('common.delete')" :negative-text="t('common.cancel')">
              <template #trigger><n-button size="small" type="error" secondary :disabled="!canManageGroup(group)">{{ t('common.delete') }}</n-button></template>
              {{ t('identity.deleteConfirm', { name: group.name }) }}
            </n-popconfirm>
          </n-space>
        </div>
      </n-list-item>
    </n-list>
    <n-empty v-if="groups.length === 0" :description="t('common.noData')" />
    <n-collapse @item-header-click="loadLogs">
      <n-collapse-item :title="t('identity.logs')" name="logs">
        <n-data-table :columns="logColumns" :data="logs" :loading="logsLoading" :scroll-x="900" size="small" />
        <n-pagination v-model:page="logPage" :page-size="10" :item-count="logTotal" @update:page="loadLogs" />
      </n-collapse-item>
    </n-collapse>
  </n-card>

  <n-modal v-model:show="showEditor" preset="card" :title="draft.id ? t('identity.edit') : t('identity.create')" class="identity-editor" style="width: min(680px, calc(100vw - 32px)); max-height: calc(100dvh - 32px); overflow-y: auto" :mask-closable="!saving" :close-on-esc="!saving">
    <n-form label-placement="top" @submit.prevent="saveGroup">
      <n-form-item :label="t('identity.name')" required>
        <n-input v-model:value="draft.name" :input-props="{ 'aria-label': t('identity.name') }" :maxlength="100" :disabled="saving" />
      </n-form-item>
      <n-form-item :label="t('identity.key')" required>
        <n-input v-model:value="draft.key" :input-props="{ 'aria-label': t('identity.key') }" :disabled="!!draft.id || saving" :maxlength="64" :placeholder="t('identity.keyHint')" />
      </n-form-item>
      <n-form-item :label="t('identity.description')">
        <n-input v-model:value="draft.description" :input-props="{ 'aria-label': t('identity.description') }" type="textarea" :maxlength="500" :disabled="saving" />
      </n-form-item>
      <fieldset class="permission-fieldset" :disabled="saving || catalogLoading">
        <legend>{{ t('identity.permissions') }}</legend>
        <n-spin :show="catalogLoading">
          <n-checkbox-group v-model:value="draft.permissions" :disabled="saving || catalogLoading">
            <div class="permission-grid">
              <n-checkbox v-for="permission in catalog" :key="permission.key" :value="permission.key" :disabled="!storeUser.hasPermission(permission.key)">
                {{ permissionName(permission.key, permission.name) }}
              </n-checkbox>
            </div>
          </n-checkbox-group>
        </n-spin>
      </fieldset>
      <n-alert v-if="validationError" type="error" class="form-error">{{ validationError }}</n-alert>
      <n-space justify="end" class="editor-actions">
        <n-button :disabled="saving" @click="showEditor = false">{{ t('common.cancel') }}</n-button>
        <n-button attr-type="submit" type="primary" :loading="saving" :disabled="catalogLoading || !catalog.length">{{ t('common.save') }}</n-button>
      </n-space>
    </n-form>
  </n-modal>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import type { DataTableColumns } from 'naive-ui';
import { deleteIdentityGroup, getIdentityLogs, getPermissionCatalog, saveIdentityGroup, type IdentityGroup, type IdentityLog, type PermissionItem } from '@/api/identity';
import { formatTime } from '@/utils/formatTime';
import { useStoreUser } from '@/store/user';

defineProps<{ groups: IdentityGroup[] }>();
const emit = defineEmits<{ changed: [] }>();
const { t, te } = useI18n();
const storeUser = useStoreUser();
const canManageGroup = (group: IdentityGroup) => group.permissions.every((permission) => storeUser.hasPermission(permission));
const catalog = ref<PermissionItem[]>([]);
const catalogLoading = ref(false);
const saving = ref(false);
const showEditor = ref(false);
const validationError = ref('');
const draft = reactive({ id: undefined as number | undefined, key: '', name: '', description: '', permissions: [] as string[] });
const permissionName = (key: string, fallback: string) => {
  const label = `identity.permission.${key.replaceAll('.', '_')}`;
  return te(label) ? t(label) : fallback;
};

function editGroup(group?: IdentityGroup) {
  Object.assign(draft, { id: group?.id, key: group?.key || '', name: group?.name || '', description: group?.description || '', permissions: [...(group?.permissions || [])] });
  validationError.value = '';
  showEditor.value = true;
}

async function saveGroup() {
  validationError.value = '';
  if (!draft.name.trim() || !/^[a-z][a-z0-9_]{0,63}$/.test(draft.key)) {
    validationError.value = t('identity.invalidGroup');
    return;
  }
  saving.value = true;
  try {
    await saveIdentityGroup({ ...draft, name: draft.name.trim(), description: draft.description.trim() });
    showEditor.value = false;
    await storeUser.loadSession(true);
    emit('changed');
    window.$message.success(t('common.operationSuccess'));
    await loadLogs();
  } catch {
    validationError.value = t('identity.saveFailed');
  } finally {
    saving.value = false;
  }
}

async function removeGroup(group: IdentityGroup) {
  try {
    await deleteIdentityGroup(group.id);
    emit('changed');
    window.$message.success(t('common.operationSuccess'));
    await loadLogs();
  } catch { /* The API displays the error and keeps the group. */ }
}

const logs = ref<IdentityLog[]>([]);
const logsLoading = ref(false);
const logPage = ref(1);
const logTotal = ref(0);
const logActions = computed<Record<string, string>>(() => ({
  'group.save': t('identity.actions.groupSave'),
  'group.delete': t('identity.actions.groupDelete'),
  'user.groups': t('identity.actions.membership'),
  'user.status': t('identity.actions.status'),
  'user.delete': t('identity.actions.userDelete'),
}));
function snapshot(value: string): string {
  try {
    const parsed = JSON.parse(value);
    if (parsed === null) return '—';
    return (Array.isArray(parsed) ? parsed : [parsed]).map((item) => {
      if (!item || typeof item !== 'object') return String(item);
      const name = item.name || item.key || '';
      const permissions = Array.isArray(item.permissions)
        ? item.permissions.filter((key: unknown) => typeof key === 'string').map((key: string) => permissionName(key, key)).join(', ')
        : '';
      return permissions ? `${name}: ${permissions}` : name;
    }).join('; ') || '—';
  } catch { return value; }
}
async function loadLogs() {
  if (!storeUser.hasPermission('identity.manage')) return;
  logsLoading.value = true;
  try {
    const response = await getIdentityLogs(logPage.value);
    logs.value = response.list || [];
    logTotal.value = response.pager?.total_rows || 0;
  } catch {
    // Preserve the last successful page; the request interceptor reports errors.
  } finally { logsLoading.value = false; }
}
const logColumns = computed<DataTableColumns<IdentityLog>>(() => [
  { title: t('identity.actor'), key: 'actor_id', width: 80 },
  { title: t('identity.action'), key: 'action', width: 150, render: (row) => logActions.value[row.action] || row.action },
  { title: t('identity.target'), key: 'target', width: 100, render: (row) => row.user_id ? `#${row.user_id}` : `#${row.group_id}` },
  { title: t('identity.before'), key: 'before', ellipsis: { tooltip: true }, render: (row) => snapshot(row.before) },
  { title: t('identity.after'), key: 'after', ellipsis: { tooltip: true }, render: (row) => snapshot(row.after) },
  { title: t('adminUsers.table.time'), key: 'created_on', width: 150, render: (row) => formatTime(row.created_on) },
]);
onMounted(async () => {
  catalogLoading.value = true;
  try { catalog.value = (await getPermissionCatalog()).permissions || []; }
  catch { window.$message.error(t('identity.saveFailed')); }
  finally { catalogLoading.value = false; }
});
</script>

<style scoped lang="less">
.identity-card { margin-top: 16px; }
.identity-help { margin: 0 0 16px; line-height: 1.6; }
.group-row { display: flex; align-items: center; justify-content: space-between; gap: 16px; width: 100%; }
.group-name { min-width: 0; overflow-wrap: anywhere; strong { margin-right: 8px; } p { margin: 4px 0 0; opacity: .7; } }
.permission-fieldset { border: 1px solid var(--n-border-color); margin: 0; padding: 12px; border-radius: 6px; }
.permission-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px 20px; }
.editor-actions, .form-error { margin-top: 16px; }
@media (max-width: 540px) { .permission-grid { grid-template-columns: 1fr; } .group-row { flex-wrap: wrap; } }
</style>
