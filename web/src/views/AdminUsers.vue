<template>
    <div>
        <main-nav title="用户管理" />

        <n-card title="用户管理" size="small" class="setting-card">
            <n-spin :show="loading">
                <div class="toolbar">
                    <n-input
                        v-model:value="keyword"
                        placeholder="搜索 ID / 用户名 / 昵称 / 手机号"
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
                        搜索
                    </n-button>
                </div>

                <n-data-table
                    remote
                    size="small"
                    :columns="userColumns"
                    :data="userItems"
                    :loading="loading"
                    :pagination="userPagination"
                    :scroll-x="1100"
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
                            <n-descriptions-item label="UID">
                                {{ detail.id }}
                            </n-descriptions-item>
                            <n-descriptions-item label="昵称">
                                {{ detail.nickname }}
                            </n-descriptions-item>
                            <n-descriptions-item label="用户名">
                                {{ detail.username }}
                            </n-descriptions-item>
                            <n-descriptions-item label="手机号">
                                {{ detail.phone }}
                            </n-descriptions-item>
                            <n-descriptions-item label="身份">
                                <n-tag
                                    round
                                    size="small"
                                    :type="identityTagType(detail.identity)"
                                >
                                    {{ detail.identity }}
                                </n-tag>
                            </n-descriptions-item>
                            <n-descriptions-item label="状态">
                                <n-tag
                                    round
                                    size="small"
                                    :type="detail.status === 1 ? 'success' : 'error'"
                                >
                                    {{ detail.status === 1 ? '正常' : '封禁' }}
                                </n-tag>
                            </n-descriptions-item>
                            <n-descriptions-item label="注册时间">
                                {{ formatTime(detail.created_on) }}
                            </n-descriptions-item>
                        </n-descriptions>

                        <div>
                            <div class="section-title">角色管理</div>
                            <div class="role-tip">
                                运维可管理所有角色；管理员可管理 导师/审核/管理员，
                                不可变更运维账号。
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
                                                ? '已授予'
                                                : '未授予'
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
                                                ? '移除'
                                                : '授予'
                                        }}
                                    </n-button>
                                </div>
                            </n-space>
                        </div>

                        <div>
                            <div class="section-title">角色变更记录</div>
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
import { h, onMounted, reactive, ref } from 'vue';
import { storeToRefs } from 'pinia';
import { useRouter } from 'vue-router';
import { NButton, NSpace, NTag, useDialog } from 'naive-ui';
import type { DataTableColumns } from 'naive-ui';
import { userInfo as fetchUserInfo } from '@/api/auth';
import { formatTime } from '@/utils/formatTime';
import { identityTagType } from '@/utils/identity';
import { useStoreMain } from '@/store/main';
import { TOKEN_KEY, useStoreUser } from '@/store/user';
import { Api } from '@/utils/request';

type UserItem = Api.Admin.NetReq.UserItem;
type RoleLogItem = Api.Admin.NetReq.UserRoleLogItem;

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
    prefix: ({ itemCount }: { itemCount: number }) => `共 ${itemCount} 人`,
});

const roleLogPagination = reactive({
    page: 1,
    pageSize: 5,
    itemCount: 0,
    showSizePicker: false,
    prefix: ({ itemCount }: { itemCount: number }) => `共 ${itemCount} 条`,
});

const roleNameMap: Record<string, string> = {
    mentor: '导师',
    auditor: '审核',
    admin: '管理员',
    operator: '运维',
};

const roleName = (role: string) => roleNameMap[role] ?? role;

const detailTitle = ref('用户详情');

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
    detailTitle.value = `用户详情 - ${row.nickname || row.username}`;
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
    const actionText = has ? '移除' : '授予';
    dialog.warning({
        title: '变更用户角色',
        content: `确定为 ${row.nickname || row.username} ${actionText}「${roleName(role)}」角色？`,
        positiveText: '确定',
        negativeText: '取消',
        onPositiveClick: async () => {
            roleChanging.value = true;
            try {
                await Api.v1.admin.post.user.role({
                    user_id: row.id as number,
                    role: role as Api.Admin.NetParams.UserRoleChangeReq['role'],
                    action: has ? 'remove' : 'add',
                });
                window.$message.success('角色已更新');
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

const handleUserPageChange = (page: number) => {
    userPagination.page = page;
    loadUsers();
};

const handleRoleLogPageChange = (page: number) => {
    roleLogPagination.page = page;
    loadRoleLogs();
};

const userColumns: DataTableColumns<UserItem> = [
    {
        title: 'ID',
        key: 'id',
        width: 80,
    },
    {
        title: '昵称',
        key: 'nickname',
        width: 140,
        ellipsis: { tooltip: true },
    },
    {
        title: '用户名',
        key: 'username',
        width: 130,
        ellipsis: { tooltip: true },
    },
    {
        title: '手机号',
        key: 'phone',
        width: 130,
        render: (row) => row.phone || '-',
    },
    {
        title: '身份',
        key: 'identity',
        width: 90,
        render: (row) =>
            h(
                NTag,
                { round: true, size: 'small', type: identityTagType(row.identity) },
                { default: () => row.identity }
            ),
    },
    {
        title: '角色',
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
        title: '状态',
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
                { default: () => (row.status === 1 ? '正常' : '封禁') }
            ),
    },
    {
        title: '注册时间',
        key: 'created_on',
        width: 150,
        render: (row) => formatTime(row.created_on),
    },
    {
        title: '操作',
        key: 'actions',
        width: 90,
        render: (row) =>
            h(
                NButton,
                {
                    size: 'small',
                    quaternary: true,
                    type: 'info',
                    onClick: () => openDetail(row),
                },
                { default: () => '详情' }
            ),
    },
];

const roleLogColumns: DataTableColumns<RoleLogItem> = [
    {
        title: '用户',
        key: 'username',
        width: 100,
        ellipsis: { tooltip: true },
        render: (row) => row.username || `#${row.user_id}`,
    },
    {
        title: '操作人',
        key: 'operator_name',
        width: 100,
        ellipsis: { tooltip: true },
        render: (row) => row.operator_name || `#${row.operator_id}`,
    },
    {
        title: '变更',
        key: 'roles',
        render: (row) =>
            `${row.old_roles || '无'} → ${row.new_roles || '无'}`,
    },
    {
        title: '动作',
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
                { default: () => (row.action === 'add' ? '授予' : '移除') }
            ),
    },
    {
        title: '时间',
        key: 'created_on',
        width: 140,
        render: (row) => formatTime(row.created_on),
    },
];

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
