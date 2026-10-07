<template>
  <div>
    <main-nav :title="t('adminUsers.pageTitle')" />
    <n-card v-if="storeUser.hasPermission('user.manage')" :title="t('adminUsers.pageTitle')" size="small" class="setting-card">
      <div class="toolbar">
        <n-input v-model:value="keyword" :placeholder="t('adminUsers.searchPlaceholder')" clearable class="keyword-input" @keyup.enter="searchUsers" />
        <n-date-picker v-model:value="registeredRange" type="daterange" clearable class="registered-range" :start-placeholder="t('adminUsers.registeredFrom')" :end-placeholder="t('adminUsers.registeredTo')" :aria-label="t('adminUsers.registeredRange')" />
        <n-button type="primary" secondary :loading="loading" @click="searchUsers">{{ t('common.search') }}</n-button>
      </div>
      <div v-if="storeUser.hasPermission('identity.manage')" class="batch-toolbar">
        <span>{{ t('adminUsers.selectedCount', { count: checkedUsers.length }) }}</span>
        <n-button secondary :disabled="!checkedUsers.length || loading" @click="openBatchMembership">{{ t('adminUsers.batchIdentity') }}</n-button>
      </div>
      <n-data-table v-model:checked-row-keys="checkedRowKeys" remote size="small" :columns="columns" :data="users" :loading="loading" :pagination="pagination" :scroll-x="1100" :row-key="(row: UserItem) => row.id" @update:page="changePage" @update:page-size="changePageSize" />
    </n-card>

    <identity-groups v-if="storeUser.hasPermission('identity.manage')" :groups="groups" @changed="refreshGroups" />

    <n-modal v-model:show="batchOpen" preset="card" :title="t('adminUsers.batchIdentity')" style="width: min(520px, calc(100vw - 32px))" :closable="!batchSaving" :mask-closable="!batchSaving" :close-on-esc="!batchSaving">
      <p>{{ t('adminUsers.batchTargets', { count: batchUsers.length }) }}</p>
      <n-space class="batch-users">
        <n-tag v-for="user in batchUsers" :key="user.id" size="small">{{ user.nickname || user.username }} · {{ user.id }}</n-tag>
      </n-space>
      <n-alert type="info" :show-icon="false" class="batch-help">{{ t('adminUsers.batchHelp') }}</n-alert>
      <n-form label-placement="top" @submit.prevent="saveBatchMembership">
        <n-form-item :label="t('identity.membership')">
          <n-select v-model:value="batchGroups" multiple :options="groupOptions" :disabled="batchSaving" :placeholder="t('identity.selectGroups')" :aria-label="t('identity.membership')" />
        </n-form-item>
        <n-button attr-type="submit" type="primary" :loading="batchSaving" :disabled="!batchUsers.length">{{ t('adminUsers.applyBatch') }}</n-button>
      </n-form>
    </n-modal>

    <n-drawer v-model:show="showDetail" width="min(460px, 100vw)" placement="right">
      <n-drawer-content :title="t('adminUsers.detailTitle', { name: detail?.nickname || detail?.username || '' })" closable>
        <n-spin :show="detailLoading">
          <template v-if="detail">
            <n-descriptions label-placement="left" bordered size="small" :column="1">
              <n-descriptions-item :label="t('adminUsers.drawer.labelUid')">{{ detail.id }}</n-descriptions-item>
              <n-descriptions-item :label="t('adminUsers.drawer.labelNickname')">{{ detail.nickname }}</n-descriptions-item>
              <n-descriptions-item :label="t('adminUsers.drawer.labelUsername')">{{ detail.username }}</n-descriptions-item>
              <n-descriptions-item :label="t('adminUsers.drawer.labelPhone')">{{ detail.phone || '—' }}</n-descriptions-item>
              <n-descriptions-item :label="t('adminUsers.drawer.labelIdentity')">{{ identityNames(detail) }}</n-descriptions-item>
              <n-descriptions-item :label="t('adminUsers.drawer.labelStatus')">{{ detail.status === 1 ? t('adminUsers.statusNormal') : t('adminUsers.statusBanned') }}</n-descriptions-item>
              <n-descriptions-item :label="t('adminUsers.drawer.labelCreatedOn')">{{ formatTime(detail.created_on) }}</n-descriptions-item>
            </n-descriptions>
            <section v-if="storeUser.hasPermission('identity.manage')" class="membership">
              <h3>{{ t('identity.membership') }}</h3>
              <p v-if="detail.is_operator">{{ t('identity.operatorSeparate') }}</p>
              <template v-else>
                <p>{{ t('identity.membershipHelp') }}</p>
                <n-select v-model:value="selectedGroups" multiple :options="groupOptions" :disabled="detail.id === userInfo.id || membershipSaving" :placeholder="t('identity.selectGroups')" :aria-label="t('identity.membership')" />
                <p v-if="detail.id === userInfo.id">{{ t('identity.selfReadonly') }}</p>
                <n-button type="primary" class="membership-save" :disabled="detail.id === userInfo.id" :loading="membershipSaving" @click="saveMembership">{{ t('common.save') }}</n-button>
              </template>
            </section>
          </template>
        </n-spin>
      </n-drawer-content>
    </n-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue';
import { storeToRefs } from 'pinia';
import { useI18n } from 'vue-i18n';
import { NButton, NSpace, NTag, useDialog } from 'naive-ui';
import type { DataTableColumns } from 'naive-ui';
import { formatTime } from '@/utils/formatTime';
import { useStoreUser } from '@/store/user';
import { Api } from '@/utils/request';
import { getIdentityGroups, setUserIdentity, setUsersIdentity, type IdentityGroup } from '@/api/identity';
import IdentityGroups from '@/components/identity-groups.vue';
import { isAssignableIdentityGroup as assignable } from '@/utils/permissions';

type UserItem = Api.Admin.NetReq.UserItem;
const { t } = useI18n();
const storeUser = useStoreUser();
const { userInfo } = storeToRefs(storeUser);
const dialog = useDialog();
const keyword = ref('');
const registeredRange = ref<[number, number] | null>(null);
const loading = ref(false);
const users = ref<UserItem[]>([]);
const groups = ref<IdentityGroup[]>([]);
const detail = ref<UserItem>();
const detailLoading = ref(false);
const showDetail = ref(false);
const membershipSaving = ref(false);
const selectedGroups = ref<number[]>([]);
const checkedRowKeys = ref<number[]>([]);
const checkedUsers = computed(() => users.value.filter(user => checkedRowKeys.value.includes(user.id)));
const batchOpen = ref(false);
const batchSaving = ref(false);
const batchUsers = ref<UserItem[]>([]);
const batchGroups = ref<number[]>([]);
const pagination = reactive({ page: 1, pageSize: 10, itemCount: 0, showSizePicker: true, pageSizes: [10, 25, 50, 100] });
const groupOptions = computed(() => groups.value.filter(assignable).map((group) => ({ label: group.name, value: group.id, disabled: group.permissions.some((permission) => !storeUser.hasPermission(permission)) })));
const identityNames = (user: UserItem) => user.is_operator ? t('user.identity.operator') : (user.identity_groups || []).map((group) => group.name).join(', ');
const canModify = (user: UserItem) => user.id !== userInfo.value.id && (!user.is_operator || userInfo.value.is_operator);
const canAssign = (user: UserItem) => user.id !== userInfo.value.id && !user.is_operator &&
  (user.identity_groups || []).filter(assignable).every(group => group.permissions.every(permission => storeUser.hasPermission(permission)));
let listRevision = 0;

async function loadUsers() {
  if (!storeUser.hasPermission('user.manage')) return;
  const revision = ++listRevision;
  loading.value = true;
  checkedRowKeys.value = [];
  try {
    const end = registeredRange.value ? new Date(registeredRange.value[1]) : undefined;
    end?.setHours(23, 59, 59, 999);
    const response = await Api.v1.admin.get.user.list({
      keyword: keyword.value.trim(), page: pagination.page, page_size: pagination.pageSize,
      registered_from: registeredRange.value ? Math.floor(registeredRange.value[0] / 1000) : undefined,
      registered_to: end ? Math.floor(end.getTime() / 1000) : undefined,
    });
    if (revision !== listRevision) return;
    users.value = response.list || [];
    pagination.itemCount = response.pager?.total_rows || 0;
  } catch { /* Request interceptor reports the failure. */ }
  finally { if (revision === listRevision) loading.value = false; }
}
async function refreshGroups() {
  if (!storeUser.hasPermission('identity.manage')) return;
  try { groups.value = (await getIdentityGroups()).groups || []; }
  catch { /* Keep the last successful catalog. */ }
  await loadUsers();
}
function searchUsers() { pagination.page = 1; void loadUsers(); }
function changePage(page: number) { pagination.page = page; void loadUsers(); }
function changePageSize(size: number) { pagination.pageSize = size; searchUsers(); }
function openBatchMembership() {
  batchUsers.value = checkedUsers.value.filter(canAssign);
  batchGroups.value = [];
  batchOpen.value = batchUsers.value.length > 0;
}
async function saveBatchMembership() {
  if (batchSaving.value || !batchUsers.value.length) return;
  batchSaving.value = true;
  try {
    await setUsersIdentity(batchUsers.value.map(user => user.id), batchGroups.value);
    batchOpen.value = false;
    await loadUsers();
    window.$message.success(t('common.operationSuccess'));
  } catch { /* Retain the selection for retry; the server rolls back the whole batch. */ }
  finally { batchSaving.value = false; }
}
async function openDetail(user: UserItem) {
  detail.value = user;
  selectedGroups.value = (user.identity_groups || []).filter(assignable).map((group) => group.id);
  showDetail.value = true;
  detailLoading.value = true;
  try {
    detail.value = await Api.v1.admin.get.user.detail({ id: user.id });
    selectedGroups.value = (detail.value.identity_groups || []).filter(assignable).map((group) => group.id);
  } catch { /* Retain the list's user summary. */ }
  finally { detailLoading.value = false; }
}
async function saveMembership() {
  if (!detail.value || detail.value.is_operator || detail.value.id === userInfo.value.id) return;
  membershipSaving.value = true;
  try {
    await setUserIdentity(detail.value.id, selectedGroups.value);
    await openDetail(detail.value);
    await loadUsers();
    window.$message.success(t('common.operationSuccess'));
  } catch { /* Keep the selected groups for retry. */ }
  finally { membershipSaving.value = false; }
}
function changeStatus(user: UserItem) {
  const banning = user.status === 1;
  dialog.warning({
    title: banning ? t('adminUsers.dialog.banTitle') : t('adminUsers.dialog.unbanTitle'),
    content: t('adminUsers.dialog.banContent', { username: user.nickname || user.username, action: banning ? t('adminUsers.actionBan') : t('adminUsers.actionUnban'), warning: banning ? t('adminUsers.dialog.banWarning') : '' }),
    positiveText: t('common.confirm'), negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await Api.v1.admin.post.user.status({ id: user.id, status: banning ? 2 : 1 });
        await loadUsers();
        window.$message.success(t('common.operationSuccess'));
      } catch { return false; }
    },
  });
}
function deleteUser(user: UserItem) {
  dialog.warning({
    title: t('adminUsers.dialog.deleteTitle'),
    content: t('adminUsers.dialog.deleteContent', { username: user.nickname || user.username }),
    positiveText: t('common.delete'), negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await Api.v1.admin.post.user.delete({ id: user.id });
        pagination.page = Math.min(pagination.page, Math.max(1, Math.ceil((pagination.itemCount - 1) / pagination.pageSize)));
        await loadUsers();
        window.$message.success(t('adminUsers.dialog.deleted'));
      } catch { return false; }
    },
  });
}
const columns = computed<DataTableColumns<UserItem>>(() => [
  ...(storeUser.hasPermission('identity.manage') ? [{ type: 'selection' as const, disabled: (user: UserItem) => !canAssign(user) }] : []),
  { title: t('adminUsers.table.id'), key: 'id', width: 80 },
  { title: t('adminUsers.table.nickname'), key: 'nickname', width: 140, ellipsis: { tooltip: true } },
  { title: t('adminUsers.table.username'), key: 'username', width: 130, ellipsis: { tooltip: true } },
  { title: t('adminUsers.table.phone'), key: 'phone', width: 130 },
  { title: t('identity.membership'), key: 'identity_groups', width: 180, render: identityNames },
  { title: t('adminUsers.table.status'), key: 'status', width: 80, render: (user) => h(NTag, { size: 'small', type: user.status === 1 ? 'success' : 'error' }, { default: () => user.status === 1 ? t('adminUsers.statusNormal') : t('adminUsers.statusBanned') }) },
  { title: t('adminUsers.table.createdOn'), key: 'created_on', width: 150, render: (user) => formatTime(user.created_on) },
  { title: t('adminUsers.table.actions'), key: 'actions', width: 220, render: (user) => h(NSpace, { size: 'small' }, { default: () => [
    h(NButton, { size: 'small', onClick: () => openDetail(user) }, { default: () => t('adminUsers.actionDetail') }),
    ...(canModify(user) ? [
      h(NButton, { size: 'small', type: 'warning', secondary: true, onClick: () => changeStatus(user) }, { default: () => user.status === 1 ? t('adminUsers.actionBan') : t('adminUsers.actionUnban') }),
      h(NButton, { size: 'small', type: 'error', secondary: true, onClick: () => deleteUser(user) }, { default: () => t('common.delete') }),
    ] : []),
  ] }) },
]);
onMounted(async () => {
  await Promise.all([loadUsers(), storeUser.hasPermission('identity.manage') ? getIdentityGroups().then((response) => { groups.value = response.groups || []; }).catch(() => {}) : Promise.resolve()]);
});
</script>

<style lang="less" scoped>
.setting-card { margin-top: -1px; border-radius: 0; }
.toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-bottom: 12px;
  .keyword-input { flex: 1 1 240px; min-width: 0; }
  .registered-range { flex: 1 1 280px; min-width: 0; }
}
.batch-toolbar { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 12px; }
.batch-users { max-height: 160px; overflow-y: auto; }
.batch-help { margin: 16px 0; }
.membership { margin-top: 24px; p { line-height: 1.6; } }
.membership-save { margin-top: 12px; }
</style>
