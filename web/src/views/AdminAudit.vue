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
                    <div class="audit-cards">
                        <div class="empty-wrap audit-empty" v-if="!loading && postItems.length === 0">
                            <n-empty size="large" description="暂无数据" />
                        </div>
                        <div
                            v-for="row in postItems"
                            :key="row.id"
                            class="audit-card"
                            @click="openInNewTab(row.id)"
                        >
                            <div class="audit-card-head">
                                <span class="audit-card-id">#{{ row.id }}</span>
                                <n-tag size="small" round :type="statusTagType(row.audit_status)">
                                    {{ statusText(row.audit_status) }}
                                </n-tag>
                                <n-tag size="small" round :bordered="false">
                                    {{ visibilityText(row.visibility) }}
                                </n-tag>
                                <span class="audit-card-time">
                                    {{ formatTime(row.created_on) }}
                                </span>
                            </div>
                            <div class="audit-card-summary">{{ postSummary(row) }}</div>
                            <div class="audit-card-foot">
                                <span class="audit-card-author">
                                    {{ row.user?.nickname || '' }} @{{ row.user?.username || '' }}
                                </span>
                                <n-button
                                    size="tiny"
                                    quaternary
                                    type="info"
                                    @click.stop="openInNewTab(row.id)"
                                >
                                    查看帖子
                                </n-button>
                            </div>
                        </div>
                        <div
                            class="audit-card-pager"
                            v-if="postPagination.itemCount > postPagination.pageSize"
                        >
                            <n-pagination
                                :page="postPagination.page"
                                :page-size="postPagination.pageSize"
                                :item-count="postPagination.itemCount"
                                @update:page="handlePostPageChange"
                            />
                        </div>
                    </div>
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

    </div>
</template>

<script setup lang="ts">
import { h, onMounted, reactive, ref } from 'vue';
import { storeToRefs } from 'pinia';
import { useRouter } from 'vue-router';
import { NTag } from 'naive-ui';
import type { DataTableColumns } from 'naive-ui';
import { userInfo as fetchUserInfo } from '@/api/auth';
import { formatTime } from '@/utils/formatTime';
import { mdPlainText } from '@/utils/markdown';
import { useStoreMain } from '@/store/main';
import { TOKEN_KEY, useStoreUser } from '@/store/user';
import { Api } from '@/utils/request';

type AuditPostItem = Api.Admin.NetReq.AuditPostItem;
type AuditLogItem = Api.Admin.NetReq.AuditLogItem;

const storeMain = useStoreMain();
const storeUser = useStoreUser();
const { userInfo } = storeToRefs(storeUser);
const router = useRouter();

const loading = ref(false);
const activeTab = ref('pending');
const postItems = ref<AuditPostItem[]>([]);
const logItems = ref<AuditLogItem[]>([]);

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
        case 9:
            return 'Markdown长文';
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
        .filter((c) => c.type === 1 || c.type === 2 || c.type === 9)
        .map((c) => (c.type === 9 ? mdPlainText(c.content) : c.content))
        .join(' ')
        .trim();
    if (texts !== '') {
        return texts;
    }
    const media = contents.filter((c) => c.type !== 1 && c.type !== 2 && c.type !== 9);
    if (media.length > 0) {
        const kinds = Array.from(new Set(media.map((c) => contentTypeText(c.type))));
        return `(${media.length}个${kinds.join('/')}内容)`;
    }
    return '(无内容)';
};

// 新标签页打开帖子详情 审核员在帖子页内查看完整内容并直接审核
const openInNewTab = (postId: number) => {
    window.open(`/#/post?id=${postId}`, '_blank', 'noopener');
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
                    onClick: () => openInNewTab(row.post_id),
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

.audit-cards {
    margin-top: 12px;
    display: flex;
    flex-direction: column;
    gap: 10px;

    .audit-empty {
        padding: 24px 0;
    }

    .audit-card {
        border: 1px solid rgba(128, 128, 128, 0.18);
        border-radius: 6px;
        padding: 12px 14px;
        cursor: pointer;
        transition: border-color 0.2s, box-shadow 0.2s;

        &:hover {
            border-color: rgba(24, 160, 88, 0.4);
            box-shadow: 0 1px 6px rgba(24, 160, 88, 0.12);
        }

        .audit-card-head {
            display: flex;
            align-items: center;
            gap: 8px;

            .audit-card-id {
                font-weight: 600;
                color: #18a058;
            }

            .audit-card-time {
                margin-left: auto;
                font-size: 12px;
                opacity: 0.65;
            }
        }

        .audit-card-summary {
            margin: 8px 0;
            font-size: 14px;
            line-height: 1.6;
            opacity: 0.9;
            display: -webkit-box;
            -webkit-line-clamp: 3;
            -webkit-box-orient: vertical;
            overflow: hidden;
        }

        .audit-card-foot {
            display: flex;
            align-items: center;

            .audit-card-author {
                flex: 1;
                font-size: 13px;
                opacity: 0.7;
            }
        }
    }

    .audit-card-pager {
        display: flex;
        justify-content: center;
        margin-top: 6px;
    }
}

.post-id-link {
    color: #18a058;
    cursor: pointer;

    &:hover {
        text-decoration: underline;
    }
}
</style>
