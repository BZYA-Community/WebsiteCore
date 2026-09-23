<template>
    <div>
        <main-nav title="审核队列" />

        <n-card title="内容审核" size="small" class="setting-card">
            <n-spin :show="loading">
                <n-tabs
                    :value="activeTab"
                    type="line"
                    @update:value="handleTabChange"
                >
                    <n-tab name="pending">待审核</n-tab>
                    <n-tab name="approved">已通过</n-tab>
                    <n-tab name="rejected">未通过</n-tab>
                    <n-tab name="logs">审核日志</n-tab>
                </n-tabs>

                <template v-if="activeTab !== 'logs'">
                    <n-data-table
                        remote
                        size="small"
                        class="audit-table"
                        :columns="postColumns"
                        :data="postItems"
                        :loading="loading"
                        :pagination="postPagination"
                        :row-key="(row: Api.Admin.NetReq.AuditPostItem) => row.id"
                        @update:page="handlePostPageChange"
                    />
                </template>
                <template v-else>
                    <n-data-table
                        remote
                        size="small"
                        class="audit-table"
                        :columns="logColumns"
                        :data="logItems"
                        :loading="loading"
                        :pagination="logPagination"
                        :row-key="(row: Api.Admin.NetReq.AuditLogItem) => row.id"
                        @update:page="handleLogPageChange"
                    />
                </template>
            </n-spin>
        </n-card>

        <n-modal
            v-model:show="rejectShow"
            preset="dialog"
            title="拒绝原因"
            positive-text="确定拒绝"
            negative-text="取消"
            @positive-click="handleRejectConfirm"
        >
            <n-space vertical>
                <div class="reject-tip">拒绝后该帖子不会公开展示，作者仍可见。</div>
                <n-input
                    v-model:value="rejectReason"
                    type="textarea"
                    placeholder="请填写拒绝原因（必填）"
                    :autosize="{ minRows: 2, maxRows: 4 }"
                    maxlength="255"
                    show-count
                />
            </n-space>
        </n-modal>
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
import { useStoreMain } from '@/store/main';
import { TOKEN_KEY, useStoreUser } from '@/store/user';
import { Api } from '@/utils/request';

type AuditPostItem = Api.Admin.NetReq.AuditPostItem;
type AuditLogItem = Api.Admin.NetReq.AuditLogItem;

const storeMain = useStoreMain();
const storeUser = useStoreUser();
const { userInfo } = storeToRefs(storeUser);
const router = useRouter();
const dialog = useDialog();

const loading = ref(false);
const acting = ref(false);
const activeTab = ref('pending');
const postItems = ref<AuditPostItem[]>([]);
const logItems = ref<AuditLogItem[]>([]);
const rejectShow = ref(false);
const rejectReason = ref('');
const rejectPostId = ref(0);

const tabStatusMap: Record<string, number> = {
    pending: 0,
    approved: 1,
    rejected: 2,
};

const postPagination = reactive({
    page: 1,
    pageSize: 10,
    itemCount: 0,
    showSizePicker: false,
    prefix: ({ itemCount }: { itemCount: number }) => `共 ${itemCount} 条`,
});

const logPagination = reactive({
    page: 1,
    pageSize: 10,
    itemCount: 0,
    showSizePicker: false,
    prefix: ({ itemCount }: { itemCount: number }) => `共 ${itemCount} 条`,
});

const statusTagType = (
    status: number
): 'warning' | 'success' | 'error' | 'default' => {
    switch (status) {
        case 0:
            return 'warning';
        case 1:
            return 'success';
        case 2:
            return 'error';
        default:
            return 'default';
    }
};

const statusText = (status: number) => {
    switch (status) {
        case 0:
            return '待审核';
        case 1:
            return '已通过';
        case 2:
            return '未通过';
        default:
            return String(status);
    }
};

const actionText = (action: string) => {
    switch (action) {
        case 'approve':
            return '通过';
        case 'reject':
            return '拒绝';
        case 'delete':
            return '删除';
        default:
            return action;
    }
};

const postSummary = (row: AuditPostItem) => {
    const text = (row.texts || []).map((t) => t.content).join(' ');
    return text.trim() || '(无文字内容)';
};

const loadPosts = async () => {
    loading.value = true;
    try {
        const resp = await Api.v1.admin.get.audit.posts({
            status: tabStatusMap[activeTab.value] ?? 0,
            page: postPagination.page,
            page_size: postPagination.pageSize,
        });
        postItems.value = resp.list || [];
        postPagination.itemCount = resp.pager?.total_rows || 0;
    } catch (_err) {
        // do nothing
    } finally {
        loading.value = false;
    }
};

const loadLogs = async () => {
    loading.value = true;
    try {
        const resp = await Api.v1.admin.get.audit.logs({
            page: logPagination.page,
            page_size: logPagination.pageSize,
        });
        logItems.value = resp.list || [];
        logPagination.itemCount = resp.pager?.total_rows || 0;
    } catch (_err) {
        // do nothing
    } finally {
        loading.value = false;
    }
};

const loadActiveTab = () => {
    if (activeTab.value === 'logs') {
        loadLogs();
    } else {
        loadPosts();
    }
};

const handleTabChange = (tab: string) => {
    activeTab.value = tab;
    if (tab === 'logs') {
        logPagination.page = 1;
    } else {
        postPagination.page = 1;
    }
    loadActiveTab();
};

const handlePostPageChange = (page: number) => {
    postPagination.page = page;
    loadPosts();
};

const handleLogPageChange = (page: number) => {
    logPagination.page = page;
    loadLogs();
};

const doAction = async (
    postId: number,
    action: 'approve' | 'reject' | 'delete',
    reason?: string
) => {
    acting.value = true;
    try {
        await Api.v1.admin.post.audit.post({
            post_id: postId,
            action,
            reason,
        });
        window.$message.success('操作成功');
        loadActiveTab();
    } catch (_err) {
        // do nothing
    } finally {
        acting.value = false;
    }
};

const handleApprove = (row: AuditPostItem) => {
    dialog.success({
        title: '通过审核',
        content: `确定通过该帖子（ID: ${row.id}）？通过后将公开出现在广场与搜索中。`,
        positiveText: '通过',
        negativeText: '取消',
        onPositiveClick: () => doAction(row.id, 'approve'),
    });
};

const handleReject = (row: AuditPostItem) => {
    rejectPostId.value = row.id;
    rejectReason.value = '';
    rejectShow.value = true;
};

const handleRejectConfirm = () => {
    const reason = rejectReason.value.trim();
    if (!reason) {
        window.$message.warning('请填写拒绝原因');
        return false;
    }
    doAction(rejectPostId.value, 'reject', reason);
    return true;
};

const handleDelete = (row: AuditPostItem) => {
    dialog.error({
        title: '删除帖子',
        content: `确定删除该帖子（ID: ${row.id}）？删除后作者不可再见，且无法恢复。`,
        positiveText: '删除',
        negativeText: '取消',
        onPositiveClick: () => doAction(row.id, 'delete'),
    });
};

const postColumns: DataTableColumns<AuditPostItem> = [
    {
        title: 'ID',
        key: 'id',
        width: 80,
    },
    {
        title: '内容',
        key: 'content',
        ellipsis: { tooltip: true },
        render: (row) =>
            h(
                'span',
                { class: 'post-summary' },
                { default: () => postSummary(row) }
            ),
    },
    {
        title: '作者',
        key: 'user',
        width: 140,
        ellipsis: { tooltip: true },
        render: (row) => `${row.user?.nickname || ''} @${row.user?.username || ''}`,
    },
    {
        title: '发布时间',
        key: 'created_on',
        width: 150,
        render: (row) => formatTime(row.created_on),
    },
    {
        title: '状态',
        key: 'audit_status',
        width: 90,
        render: (row) =>
            h(
                NTag,
                {
                    round: true,
                    size: 'small',
                    type: statusTagType(row.audit_status),
                },
                { default: () => statusText(row.audit_status) }
            ),
    },
    {
        title: '操作',
        key: 'actions',
        width: 200,
        render: (row) =>
            h(
                NSpace,
                { size: 'small' },
                {
                    default: () => {
                        const buttons: Array<ReturnType<typeof h>> = [];
                        if (row.audit_status !== 1) {
                            buttons.push(
                                h(
                                    NButton,
                                    {
                                        size: 'tiny',
                                        type: 'success',
                                        secondary: true,
                                        loading: acting.value,
                                        onClick: () => handleApprove(row),
                                    },
                                    { default: () => '通过' }
                                )
                            );
                        }
                        if (row.audit_status !== 2) {
                            buttons.push(
                                h(
                                    NButton,
                                    {
                                        size: 'tiny',
                                        type: 'warning',
                                        secondary: true,
                                        onClick: () => handleReject(row),
                                    },
                                    { default: () => '拒绝' }
                                )
                            );
                        }
                        buttons.push(
                            h(
                                NButton,
                                {
                                    size: 'tiny',
                                    type: 'error',
                                    secondary: true,
                                    onClick: () => handleDelete(row),
                                },
                                { default: () => '删除' }
                            )
                        );
                        return buttons;
                    },
                }
            ),
    },
];

const logColumns: DataTableColumns<AuditLogItem> = [
    {
        title: 'ID',
        key: 'id',
        width: 70,
    },
    {
        title: '帖子ID',
        key: 'post_id',
        width: 80,
    },
    {
        title: '操作人',
        key: 'operator_name',
        width: 120,
        ellipsis: { tooltip: true },
        render: (row) => row.operator_name || `#${row.operator_id}`,
    },
    {
        title: '动作',
        key: 'action',
        width: 80,
        render: (row) =>
            h(
                NTag,
                {
                    round: true,
                    size: 'small',
                    type:
                        row.action === 'approve'
                            ? 'success'
                            : row.action === 'reject'
                              ? 'warning'
                              : 'error',
                },
                { default: () => actionText(row.action) }
            ),
    },
    {
        title: '状态变更',
        key: 'status',
        width: 130,
        render: (row) => `${statusText(row.old_status)} → ${statusText(row.new_status)}`,
    },
    {
        title: '原因',
        key: 'reason',
        ellipsis: { tooltip: true },
        render: (row) => row.reason || '-',
    },
    {
        title: '时间',
        key: 'created_on',
        width: 150,
        render: (row) => formatTime(row.created_on),
    },
];

const ensureAuditAccess = async () => {
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

    if (!userInfo.value.is_admin && !userInfo.value.roles?.includes('auditor')) {
        router.replace({
            name: '404',
        });
        return false;
    }

    return true;
};

onMounted(async () => {
    const allowed = await ensureAuditAccess();
    if (!allowed) {
        return;
    }
    loadActiveTab();
});
</script>

<style lang="less" scoped>
.setting-card {
    margin-top: -1px;
    border-radius: 0;
}

.audit-table {
    margin-top: 12px;
}

.post-summary {
    opacity: 0.85;
}

.reject-tip {
    font-size: 12px;
    opacity: 0.65;
}
</style>
