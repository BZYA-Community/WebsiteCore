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
                        :scroll-x="1080"
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
                        :scroll-x="960"
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
                <div class="reject-tip">
                    拒绝后该帖子将转为私密并标记「未通过」（作者可见），作者可将可见性重新设为非私密后再次提交审核。
                </div>
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

        <n-modal
            v-model:show="detailShow"
            preset="card"
            class="post-detail-modal"
            :title="detailTitle"
            :bordered="false"
            size="huge"
        >
            <n-spin :show="detailLoading">
                <div v-if="detailPost" class="post-detail-body">
                    <div class="post-detail-meta">
                        <span>作者：{{ detailPost.user?.nickname || '' }} @{{ detailPost.user?.username || '' }}</span>
                        <span>发布时间：{{ formatTime(detailPost.created_on) }}</span>
                        <span>
                            状态：<n-tag size="small" round :type="statusTagType(detailPost.audit_status)">{{ statusText(detailPost.audit_status) }}</n-tag>
                        </span>
                    </div>
                    <div class="post-detail-contents">
                        <template v-for="c in detailPost.contents" :key="c.id">
                            <div v-if="c.type === 1 || c.type === 2" class="detail-text">{{ c.content }}</div>
                            <n-image
                                v-else-if="c.type === 3"
                                :src="c.content"
                                width="200"
                                class="detail-image"
                            />
                            <div v-else class="detail-other">
                                <n-tag size="small" :bordered="false">{{ contentTypeText(c.type) }}</n-tag>
                                <span class="detail-other-content">{{ c.content }}</span>
                            </div>
                        </template>
                    </div>
                </div>
            </n-spin>
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
import { getPost } from '@/api/post';
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
const detailShow = ref(false);
const detailLoading = ref(false);
const detailPostId = ref(0);
const detailPost = ref<AuditPostItem | null>(null);

const detailTitle = () => `帖子详情 #${detailPostId.value}`;

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

const contentTypeText = (type: number) => {
    switch (type) {
        case 1:
            return '标题';
        case 2:
            return '文字';
        case 3:
            return '图片';
        case 4:
            return '视频';
        case 5:
            return '音频';
        case 6:
            return '链接';
        case 7:
            return '附件';
        case 8:
            return '收费附件';
        default:
            return `类型${type}`;
    }
};

const visibilityText = (visibility: number) => {
    switch (visibility) {
        case 0:
            return '私密';
        case 50:
            return '好友可见';
        case 60:
            return '关注可见';
        case 90:
            return '公开';
        default:
            return String(visibility);
    }
};

const postSummary = (row: AuditPostItem) => {
    const contents = row.contents || [];
    const texts = contents
        .filter((c) => c.type === 1 || c.type === 2)
        .map((c) => c.content)
        .join(' ')
        .trim();
    if (texts !== '') {
        return texts;
    }
    const media = contents.filter((c) => c.type !== 1 && c.type !== 2);
    if (media.length > 0) {
        const kinds = Array.from(new Set(media.map((c) => contentTypeText(c.type))));
        return `(${media.length}个${kinds.join('/')}内容)`;
    }
    return '(无内容)';
};

const openDetail = (row: AuditPostItem) => {
    detailPostId.value = row.id;
    detailPost.value = row;
    detailShow.value = true;
    detailLoading.value = false;
};

const openDetailById = async (postId: number) => {
    detailPostId.value = postId;
    detailPost.value = null;
    detailShow.value = true;
    detailLoading.value = true;
    try {
        const post = await getPost({ id: postId });
        detailPost.value = {
            id: post.id,
            user: {
                id: post.user?.id || 0,
                nickname: post.user?.nickname || '',
                username: post.user?.username || '',
            },
            contents: (post.contents || []).map((c, i) => ({
                id: i,
                content: c.content,
                type: c.type,
                sort: c.sort,
            })),
            visibility: post.visibility,
            created_on: post.created_on,
            audit_status: (post.audit_status ?? 1) as 0 | 1 | 2,
        };
    } catch (_err) {
        window.$message.warning('帖子详情获取失败(可能已被删除)');
        detailShow.value = false;
    } finally {
        detailLoading.value = false;
    }
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
    action: 'approve' | 'reject',
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

const postColumns: DataTableColumns<AuditPostItem> = [
    {
        title: 'ID',
        key: 'id',
        width: 90,
        render: (row) =>
            h(
                'span',
                {
                    class: 'post-id-link',
                    onClick: () => openDetail(row),
                },
                { default: () => `#${row.id}` }
            ),
    },
    {
        title: '内容',
        key: 'content',
        minWidth: 220,
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
        width: 150,
        ellipsis: { tooltip: true },
        render: (row) => `${row.user?.nickname || ''} @${row.user?.username || ''}`,
    },
    {
        title: '可见性',
        key: 'visibility',
        width: 90,
        render: (row) => visibilityText(row.visibility),
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
        width: 160,
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
        width: 90,
        render: (row) =>
            h(
                'span',
                {
                    class: 'post-id-link',
                    onClick: () => openDetailById(row.post_id),
                },
                { default: () => `#${row.post_id}` }
            ),
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

.post-id-link {
    color: #18a058;
    cursor: pointer;

    &:hover {
        text-decoration: underline;
    }
}

.reject-tip {
    font-size: 12px;
    opacity: 0.65;
}
</style>

<style lang="less">
.post-detail-modal {
    width: 640px;
    max-width: 92vw;

    .post-detail-meta {
        display: flex;
        flex-wrap: wrap;
        gap: 16px;
        font-size: 13px;
        opacity: 0.75;
        margin-bottom: 12px;
    }

    .post-detail-contents {
        display: flex;
        flex-direction: column;
        gap: 10px;
        max-height: 55vh;
        overflow-y: auto;

        .detail-text {
            white-space: pre-wrap;
            word-break: break-word;
            line-height: 1.7;
        }

        .detail-image {
            border-radius: 4px;
        }

        .detail-other {
            display: flex;
            align-items: center;
            gap: 8px;
            word-break: break-all;

            .detail-other-content {
                font-size: 13px;
                opacity: 0.8;
            }
        }
    }
}
</style>
