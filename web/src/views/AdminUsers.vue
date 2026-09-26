<template>
    <div>
        <main-nav :title="t('adminUsers.pageTitle')" />

        <n-card :title="t('adminUsers.pageTitle')" size="small" class="setting-card">
            <n-spin :show="loading">
                <div class="toolbar">
                    <n-input
                        v-model:value="keyword"
                        :placeholder="t('adminUsers.searchPlaceholder')"
                        clearable
                        class="keyword-input"
                        @keyup.enter="handleSearch"
                    />
                    <n-button
                        round
                        secondary
                        type="primary"
                        :loading="loading"
                        @click="handleSearch"
                    >
                        {{ t('common.search') }}
                    </n-button>
                </div>

                <n-data-table
                    remote
                    size="small"
                    :columns="userColumns"
                    :data="userItems"
                    :loading="loading"
                    :pagination="userPagination"
                    :scroll-x="1210"
                    :row-key="(row: Api.Admin.NetReq.UserItem) => row.id"
                    @update:page="handleUserPageChange"
                />
            </n-spin>
        </n-card>

        <n-drawer v-model:show="detailShow" :width="460" placement="right">
            <n-drawer-content :title="detailTitle" closable>
                <n-spin :show="detailLoading">
                    <n-space vertical size="large">
                        <n-descriptions
                            label-placement="left"
                            bordered
                            size="small"
                            :column="1"
                        >
                            <n-descriptions-item :label="t('adminUsers.drawer.labelUid')">
                                {{ detail.id }}
                            </n-descriptions-item>
                            <n-descriptions-item :label="t('adminUsers.drawer.labelNickname')">
                                {{ detail.nickname }}
                            </n-descriptions-item>
                            <n-descriptions-item :label="t('adminUsers.drawer.labelUsername')">
                                {{ detail.username }}
                            </n-descriptions-item>
                            <n-descriptions-item :label="t('adminUsers.drawer.labelPhone')">
                                {{ detail.phone }}
                            </n-descriptions-item>
                            <n-descriptions-item :label="t('adminUsers.drawer.labelIdentity')">
                                <n-tag
                                    round
                                    size="small"
                                    :type="identityTagType(detail.identity)"
                                >
                                    {{ identityLabel(detail.identity) }}
                                </n-tag>
                            </n-descriptions-item>
                            <n-descriptions-item :label="t('adminUsers.drawer.labelStatus')">
                                <n-tag
                                    round
                                    size="small"
                                    :type="detail.status === 1 ? 'success' : 'error'"
                                >
                                    {{ detail.status === 1 ? t('adminUsers.statusNormal') : t('adminUsers.statusBanned') }}
                                </n-tag>
                            </n-descriptions-item>
                            <n-descriptions-item :label="t('adminUsers.drawer.labelCreatedOn')">
                                {{ formatTime(detail.created_on) }}
                            </n-descriptions-item>
                        </n-descriptions>

                        <div>
                            <div class="section-title">{{ t('adminUsers.drawer.roleManagement') }}</div>
                            <div class="role-tip">
                                {{ t('adminUsers.drawer.roleTip') }}
                            </div>
                            <n-space vertical size="small" class="role-list">
                                <div
                                    v-for="role in manageableRoles(detail)"
                                    :key="role"
                                    class="role-row"
                                >
                                    <n-tag
                                        round
                                        size="small"
                                        :type="
                                            detail.roles?.includes(role)
                                                ? 'success'
                                                : 'default'
                                        "
                                    >
                                        {{ roleName(role) }}
                                    </n-tag>
                                    <span class="role-state">
                                        {{
                                            detail.roles?.includes(role)
                                                ? t('adminUsers.drawer.granted')
                                                : t('adminUsers.drawer.notGranted')
                                        }}
                                    </span>
                                    <n-button
                                        size="tiny"
                                        quaternary
                                        :type="
                                            detail.roles?.includes(role)
                                                ? 'error'
                                                : 'info'
                                        "
                                        :disabled="!canManageRole(detail, role)"
                                        :loading="roleChanging"
                                        @click="handleRoleChange(detail, role)"
                                    >
                                        {{
                                            detail.roles?.includes(role)
                                                ? t('adminUsers.actionRevoke')
                                                : t('adminUsers.actionGrant')
                                        }}
                                    </n-button>
                                </div>
                            </n-space>
                        </div>

                        <div>
                            <div class="section-title">{{ t('adminUsers.drawer.roleChangeLog') }}</div>
                            <n-data-table
                                remote
                                size="small"
                                :columns="roleLogColumns"
                                :data="roleLogItems"
                                :loading="roleLogLoading"
                                :pagination="roleLogPagination"
                                :scroll-x="720"
                                :row-key="
                                    (row: Api.Admin.NetReq.UserRoleLogItem) =>
                                        row.id
                                "
                                @update:page="handleRoleLogPageChange"
                            />
                        </div>
                    </n-space>
                </n-spin>
            </n-drawer-content>
        </n-drawer>
    </div>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue';
import type { Component } from 'vue';
import { storeToRefs } from 'pinia';
import { useRouter } from 'vue-router';
import { useI18n } from 'vue-i18n';
import { NButton, NSpace, NTag, useDialog } from 'naive-ui';
import type { DataTableColumns } from 'naive-ui';
import { userInfo as fetchUserInfo } from '@/api/auth';
import { formatTime } from '@/utils/formatTime';
import { identityTagType, identityLabel } from '@/utils/identity';
import { useStoreMain } from '@/store/main';
import { TOKEN_KEY, useStoreUser } from '@/store/user';
import { Api } from '@/utils/request';

type UserItem = Api.Admin.NetReq.UserItem;
type RoleLogItem = Api.Admin.NetReq.UserRoleLogItem;

const { t } = useI18n();
const storeMain = useStoreMain();
const storeUser = useStoreUser();
const { userInfo } = storeToRefs(storeUser);
const router = useRouter();
const dialog = useDialog();

const loading = ref(false);
const detailLoading = ref(false);
const roleChanging = ref(false);
const roleLogLoading = ref(false);
const keyword = ref('');
const userItems = ref<UserItem[]>([]);
const detailShow = ref(false);
const detail = ref<Partial<UserItem>>({});
const roleLogItems = ref<RoleLogItem[]>([]);

const allRoles = ['mentor', 'auditor', 'admin', 'operator'] as const;

const userPagination = reactive({
    page: 1,
    pageSize: 10,
    itemCount: 0,
    showSizePicker: false,
    prefix: computed(({ itemCount }: { itemCount: number }) =>
        t('adminUsers.pagination.userCount', { count: itemCount }),
    ),
});

const roleLogPagination = reactive({
    page: 1,
    pageSize: 5,
    itemCount: 0,
    showSizePicker: false,
    prefix: computed(({ itemCount }: { itemCount: number }) =>
        t('adminUsers.pagination.itemCount', { count: itemCount }),
    ),
});

const roleNameMap = computed<Record<string, string>>(() => ({
    mentor: t('adminUsers.role.mentor'),
    auditor: t('adminUsers.role.auditor'),
    admin: t('adminUsers.role.admin'),
    operator: t('adminUsers.role.operator'),
}));

const roleName = (role: string) => roleNameMap.value[role] ?? role;

const detailTitle = ref(t('adminUsers.detailTitle', { name: '' }));

// 运维账号在服务端享有全部角色管理权限，前端按同一规则禁用按钮
const selfIsOperator = () =>
    (userInfo.value.roles || []).includes('operator');

const manageableRoles = (row: Partial<UserItem>) => {
    // 非运维不展示运维角色入口
    if (!selfIsOperator()) {
        return allRoles.filter((role) => role !== 'operator');
    }
    return allRoles;
};

const canManageRole = (row: Partial<UserItem>, role: string) => {
    if (row.id === userInfo.value.id) {
        // 不允许改动自己的角色，避免误操作锁死管理入口
        return false;
    }
    if (selfIsOperator()) {
        return true;
    }
    return role !== 'operator' && !(row.roles || []).includes('operator');
};

const loadUsers = async () => {
    loading.value = true;
    try {
        const resp = await Api.v1.admin.get.user.list({
            keyword: keyword.value.trim(),
            page: userPagination.page,
            page_size: userPagination.pageSize,
        });
        userItems.value = resp.list || [];
        userPagination.itemCount = resp.pager?.total_rows || 0;
    } catch (_err) {
        // do nothing
    } finally {
        loading.value = false;
    }
};

const loadRoleLogs = async () => {
    roleLogLoading.value = true;
    try {
        const resp = await Api.v1.admin.get.user.role.logs({
            page: roleLogPagination.page,
            page_size: roleLogPagination.pageSize,
        });
        roleLogItems.value = resp.list || [];
        roleLogPagination.itemCount = resp.pager?.total_rows || 0;
    } catch (_err) {
        // do nothing
    } finally {
        roleLogLoading.value = false;
    }
};

const loadDetail = async (id: number) => {
    detailLoading.value = true;
    try {
        detail.value = await Api.v1.admin.get.user.detail({ id });
    } catch (_err) {
        // do nothing
    } finally {
        detailLoading.value = false;
    }
};

const openDetail = (row: UserItem) => {
    detailTitle.value = t('adminUsers.detailTitle', { name: row.nickname || row.username });
    detail.value = { ...row };
    detailShow.value = true;
    roleLogPagination.page = 1;
    loadDetail(row.id);
    loadRoleLogs();
};

const handleRoleChange = (row: Partial<UserItem>, role: string) => {
    if (!row.id) {
        return;
    }
    const has = (row.roles || []).includes(role);
    const actionText = has ? t('adminUsers.actionRevoke') : t('adminUsers.actionGrant');
    dialog.warning({
        title: t('adminUsers.dialog.changeRoleTitle'),
        content: t('adminUsers.dialog.changeRoleContent', {
            username: row.nickname || row.username,
            action: actionText,
            role: roleName(role),
        }),
        positiveText: t('common.confirm'),
        negativeText: t('common.cancel'),
        onPositiveClick: async () => {
            roleChanging.value = true;
            try {
                await Api.v1.admin.post.user.role({
                    user_id: row.id as number,
                    role: role as Api.Admin.NetParams.UserRoleChangeReq['role'],
                    action: has ? 'remove' : 'add',
                });
                window.$message.success(t('adminUsers.dialog.roleUpdated'));
                await loadDetail(row.id as number);
                loadUsers();
                loadRoleLogs();
            } catch (_err) {
                // do nothing
            } finally {
                roleChanging.value = false;
            }
        },
    });
};

const handleSearch = () => {
    userPagination.page = 1;
    loadUsers();
};

// 禁言/解封(复用既有 admin/user/status API)
const handleStatusChange = (row: UserItem) => {
    const banning = row.status === 1;
    dialog.warning({
        title: banning ? t('adminUsers.dialog.banTitle') : t('adminUsers.dialog.unbanTitle'),
        content: t('adminUsers.dialog.banContent', {
            username: row.nickname || row.username,
            action: banning ? t('adminUsers.actionBan') : t('adminUsers.actionUnban'),
            warning: banning ? t('adminUsers.dialog.banWarning') : '',
        }),
        positiveText: t('common.confirm'),
        negativeText: t('common.cancel'),
        onPositiveClick: async () => {
            try {
                await Api.v1.admin.post.user.status({
                    id: row.id,
                    status: banning ? 2 : 1,
                });
                window.$message.success(banning ? t('adminUsers.dialog.banned') : t('adminUsers.dialog.unbanned'));
                loadUsers();
                // 详情抽屉打开时同步刷新
                if (detailShow.value && detail.value.id === row.id) {
                    loadDetail(row.id);
                }
            } catch (_err) {
                // 错误提示由请求拦截器统一处理
            }
        },
    });
};

// 软删除(is_del=1: 无法登录/从前台与列表消失, 数据保留可恢复)
const handleUserDelete = (row: UserItem) => {
    dialog.warning({
        title: t('adminUsers.dialog.deleteTitle'),
        content: t('adminUsers.dialog.deleteContent', {
            username: row.nickname || row.username,
        }),
        positiveText: t('common.delete'),
        negativeText: t('common.cancel'),
        onPositiveClick: async () => {
            try {
                await Api.v1.admin.post.user.delete({ id: row.id });
                window.$message.success(t('adminUsers.dialog.deleted'));
                if (detailShow.value && detail.value.id === row.id) {
                    detailShow.value = false;
                }
                // 删除后可能不足整页, 当前页超出时回退一页
                const lastPage = Math.max(
                    1,
                    Math.ceil((userPagination.itemCount - 1) / userPagination.pageSize),
                );
                if (userPagination.page > lastPage) {
                    userPagination.page = lastPage;
                }
                loadUsers();
            } catch (_err) {
                // 错误提示由请求拦截器统一处理
            }
        },
    });
};

const handleUserPageChange = (page: number) => {
    userPagination.page = page;
    loadUsers();
};

const handleRoleLogPageChange = (page: number) => {
    roleLogPagination.page = page;
    loadRoleLogs();
};

const userColumns = computed<DataTableColumns<UserItem>>(() => [
    {
        title: t('adminUsers.table.id'),
        key: 'id',
        width: 80,
    },
    {
        title: t('adminUsers.table.nickname'),
        key: 'nickname',
        width: 140,
        ellipsis: { tooltip: true },
    },
    {
        title: t('adminUsers.table.username'),
        key: 'username',
        width: 130,
        ellipsis: { tooltip: true },
    },
    {
        title: t('adminUsers.table.phone'),
        key: 'phone',
        width: 130,
        render: (row) => row.phone || '-',
    },
    {
        title: t('adminUsers.table.identity'),
        key: 'identity',
        width: 90,
        render: (row) =>
            h(
                NTag,
                { round: true, size: 'small', type: identityTagType(row.identity) },
                { default: () => identityLabel(row.identity) }
            ),
    },
    {
        title: t('adminUsers.table.role'),
        key: 'roles',
        render: (row) =>
            h(
                NSpace,
                { size: 'small' },
                {
                    default: () =>
                        (row.roles || []).map((role) =>
                            h(
                                NTag,
                                { round: true, size: 'small', key: role },
                                { default: () => roleName(role) }
                            )
                        ),
                }
            ),
    },
    {
        title: t('adminUsers.table.status'),
        key: 'status',
        width: 80,
        render: (row) =>
            h(
                NTag,
                {
                    round: true,
                    size: 'small',
                    type: row.status === 1 ? 'success' : 'error',
                },
                { default: () => (row.status === 1 ? t('adminUsers.statusNormal') : t('adminUsers.statusBanned')) }
            ),
    },
    {
        title: t('adminUsers.table.createdOn'),
        key: 'created_on',
        width: 150,
        render: (row) => formatTime(row.created_on),
    },
    {
        title: t('adminUsers.table.actions'),
        key: 'actions',
        width: 200,
        render: (row) => {
            // 不可操作自己; 运维账号仅运维可禁言/删除(与角色变更同规则)
            const operable =
                row.id !== userInfo.value.id &&
                (selfIsOperator() || !(row.roles || []).includes('operator'));
            const buttons: Component[] = [
                h(
                    NButton,
                    {
                        size: 'small',
                        quaternary: true,
                        type: 'info',
                        onClick: () => openDetail(row),
                    },
                    { default: () => t('adminUsers.actionDetail') }
                ),
            ];
            if (operable) {
                buttons.push(
                    h(
                        NButton,
                        {
                            size: 'small',
                            quaternary: true,
                            type: 'warning',
                            onClick: () => handleStatusChange(row),
                        },
                        { default: () => (row.status === 1 ? t('adminUsers.actionBan') : t('adminUsers.actionUnban')) }
                    ),
                    h(
                        NButton,
                        {
                            size: 'small',
                            quaternary: true,
                            type: 'error',
                            onClick: () => handleUserDelete(row),
                        },
                        { default: () => t('common.delete') }
                    ),
                );
            }
            return h(
                NSpace,
                { size: 'small' },
                { default: () => buttons }
            );
        },
    },
]);

const roleLogColumns = computed<DataTableColumns<RoleLogItem>>(() => [
    {
        title: t('adminUsers.table.user'),
        key: 'username',
        width: 100,
        ellipsis: { tooltip: true },
        render: (row) => row.username || t('adminUsers.userFallback', { id: row.user_id }),
    },
    {
        title: t('adminUsers.table.operator'),
        key: 'operator_name',
        width: 100,
        ellipsis: { tooltip: true },
        render: (row) => row.operator_name || t('adminUsers.operatorFallback', { id: row.operator_id }),
    },
    {
        title: t('adminUsers.table.change'),
        key: 'roles',
        render: (row) =>
            t('adminUsers.roleChangeArrow', {
                old: row.old_roles || t('adminUsers.noRoles'),
                new: row.new_roles || t('adminUsers.noRoles'),
            }),
    },
    {
        title: t('adminUsers.table.action'),
        key: 'action',
        width: 70,
        render: (row) =>
            h(
                NTag,
                {
                    round: true,
                    size: 'small',
                    type: row.action === 'add' ? 'success' : 'warning',
                },
                { default: () => (row.action === 'add' ? t('adminUsers.actionGrant') : t('adminUsers.actionRevoke')) }
            ),
    },
    {
        title: t('adminUsers.table.time'),
        key: 'created_on',
        width: 140,
        render: (row) => formatTime(row.created_on),
    },
]);

const ensureAdminAccess = async () => {
    if (!localStorage.getItem(TOKEN_KEY) && userInfo.value.id === 0) {
        storeMain.triggerAuth(true);
        storeMain.triggerAuthKey('signin');
        router.replace({
            name: 'home',
        });
        return false;
    }

    if (userInfo.value.id === 0) {
        try {
            const currentUser = await fetchUserInfo();
            storeUser.updateUserinfo(currentUser);
        } catch (_err) {
            storeUser.userLogout();
            router.replace({
                name: 'home',
            });
            return false;
        }
    }

    if (!userInfo.value.is_admin) {
        router.replace({
            name: '404',
        });
        return false;
    }

    return true;
};

onMounted(async () => {
    const allowed = await ensureAdminAccess();
    if (!allowed) {
        return;
    }
    loadUsers();
});
</script>

<style lang="less" scoped>
.setting-card {
    margin-top: -1px;
    border-radius: 0;
}

.toolbar {
    display: flex;
    gap: 12px;
    margin-bottom: 12px;

    .keyword-input {
        max-width: 320px;
    }
}

.section-title {
    font-size: 15px;
    font-weight: 600;
    margin-bottom: 8px;
}

.role-tip {
    font-size: 12px;
    opacity: 0.65;
    margin-bottom: 10px;
}

.role-list {
    .role-row {
        display: flex;
        align-items: center;
        gap: 10px;

        .role-state {
            flex: 1;
            font-size: 13px;
            opacity: 0.75;
        }
    }
}
</style>
