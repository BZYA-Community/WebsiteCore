<template>
    <div>
        <main-nav title="审核队列" />

        <n-card title="内容审核" size="small" class="setting-card">
            <n-spin :show="loading">
                <n-tabs
                    :value="category"
                    type="segment"
                    @update:value="handleCategoryChange"
                >
                    <n-tab name="post">帖子</n-tab>
                    <n-tab name="comment">评论</n-tab>
                    <n-tab name="nickname">昵称</n-tab>
                    <n-tab name="logs">审核日志</n-tab>
                </n-tabs>

                <!-- 帖子/评论: 状态子标签 -->
                <template v-if="category === 'post' || category === 'comment'">
                    <n-tabs
                        :value="activeTab"
                        type="line"
                        size="small"
                        class="status-tabs"
                        @update:value="handleTabChange"
                    >
                        <n-tab name="pending">待审核</n-tab>
                        <n-tab name="approved">已通过</n-tab>
                        <n-tab name="rejected">未通过</n-tab>
                    </n-tabs>

                    <!-- 帖子队列 -->
                    <div class="audit-cards" v-if="category === 'post'">
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

                    <!-- 评论队列 -->
                    <div class="audit-cards" v-else>
                        <div class="empty-wrap audit-empty" v-if="!loading && commentItems.length === 0">
                            <n-empty size="large" description="暂无数据" />
                        </div>
                        <div
                            v-for="row in commentItems"
                            :key="`${row.comment_type}-${row.id}`"
                            class="audit-card"
                            @click="openCommentInNewTab(row)"
                        >
                            <div class="audit-card-head">
                                <span class="audit-card-id">
                                    {{ row.comment_type === 1 ? '回复' : '评论' }}#{{ row.id }}
                                </span>
                                <n-tag size="small" round :type="statusTagType(row.audit_status)">
                                    {{ statusText(row.audit_status) }}
                                </n-tag>
                                <span class="audit-card-time">
                                    {{ formatTime(row.created_on) }}
                                </span>
                            </div>
                            <div class="audit-card-summary">{{ row.content || '(无文字内容)' }}</div>
                            <div class="audit-card-foot">
                                <span class="audit-card-author">
                                    {{ row.user?.nickname || '' }} @{{ row.user?.username || '' }}
                                </span>
                                <n-button
                                    size="tiny"
                                    quaternary
                                    type="info"
                                    @click.stop="openCommentInNewTab(row)"
                                >
                                    查看{{ row.comment_type === 1 ? '回复' : '评论' }}
                                </n-button>
                                <n-button
                                    size="tiny"
                                    quaternary
                                    type="success"
                                    :loading="acting"
                                    @click.stop="approveComment(row)"
                                >
                                    通过
                                </n-button>
                                <n-button
                                    size="tiny"
                                    quaternary
                                    type="error"
                                    @click.stop="openReject('comment', row)"
                                >
                                    拒绝
                                </n-button>
                            </div>
                        </div>
                        <div
                            class="audit-card-pager"
                            v-if="commentPagination.itemCount > commentPagination.pageSize"
                        >
                            <n-pagination
                                :page="commentPagination.page"
                                :page-size="commentPagination.pageSize"
                                :item-count="commentPagination.itemCount"
                                @update:page="handleCommentPageChange"
                            />
                        </div>
                    </div>
                </template>

                <!-- 昵称队列 -->
                <template v-else-if="category === 'nickname'">
                    <div class="audit-cards">
                        <div class="empty-wrap audit-empty" v-if="!loading && nicknameItems.length === 0">
                            <n-empty size="large" description="暂无数据" />
                        </div>
                        <div
                            v-for="row in nicknameItems"
                            :key="row.user_id"
                            class="audit-card"
                            @click="openUserInNewTab(row.username)"
                        >
                            <div class="audit-card-head">
                                <span class="audit-card-id">@{{ row.username }}</span>
                                <n-tag size="small" round type="warning">待审核</n-tag>
                                <span class="audit-card-time">
                                    {{ formatTime(row.created_on) }}
                                </span>
                            </div>
                            <div class="audit-card-summary nickname-change-wrap">
                                <span class="nickname-old">{{ row.nickname || '(空)' }}</span>
                                <span class="nickname-arrow">→</span>
                                <span class="nickname-new">{{ row.pending_nickname }}</span>
                            </div>
                            <div class="audit-card-foot">
                                <span class="audit-card-author">昵称变更申请</span>
                                <n-button
                                    size="tiny"
                                    quaternary
                                    type="info"
                                    @click.stop="openUserInNewTab(row.username)"
                                >
                                    查看主页
                                </n-button>
                                <n-button
                                    size="tiny"
                                    quaternary
                                    type="success"
                                    :loading="acting"
                                    @click.stop="approveNickname(row)"
                                >
                                    通过
                                </n-button>
                                <n-button
                                    size="tiny"
                                    quaternary
                                    type="error"
                                    @click.stop="openReject('nickname', row)"
                                >
                                    拒绝
                                </n-button>
                            </div>
                        </div>
                        <div
                            class="audit-card-pager"
                            v-if="nicknamePagination.itemCount > nicknamePagination.pageSize"
                        >
                            <n-pagination
                                :page="nicknamePagination.page"
                                :page-size="nicknamePagination.pageSize"
                                :item-count="nicknamePagination.itemCount"
                                @update:page="handleNicknamePageChange"
                            />
                        </div>
                    </div>
                </template>

                <!-- 审核日志 -->
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

        <!-- 拒绝原因弹窗(评论/昵称共用) -->
        <n-modal
            v-model:show="showRejectModal"
            preset="dialog"
            title="拒绝原因"
            positive-text="确认拒绝"
            negative-text="取消"
            @positive-click="confirmReject"
        >
            <div class="reject-modal-body">
                <n-input
                    v-model:value="rejectReason"
                    type="textarea"
                    placeholder="请填写拒绝原因(必填)"
                    :autosize="{ minRows: 2, maxRows: 4 }"
                    maxlength="120"
                    show-count
                />
            </div>
        </n-modal>

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
type AuditCommentItem = Api.Admin.NetReq.AuditCommentItem;
type AuditNicknameItem = Api.Admin.NetReq.AuditNicknameItem;
type AuditLogItem = Api.Admin.NetReq.AuditLogItem;

const storeMain = useStoreMain();
const storeUser = useStoreUser();
const { userInfo } = storeToRefs(storeUser);
const router = useRouter();

const loading = ref(false);
const acting = ref(false);
const category = ref<'post' | 'comment' | 'nickname' | 'logs'>('post');
const activeTab = ref('pending');
const postItems = ref<AuditPostItem[]>([]);
const commentItems = ref<AuditCommentItem[]>([]);
const nicknameItems = ref<AuditNicknameItem[]>([]);
const logItems = ref<AuditLogItem[]>([]);

// 拒绝原因弹窗
const showRejectModal = ref(false);
const rejectReason = ref('');
const rejectTarget = ref<{ kind: 'comment' | 'nickname'; row: AuditCommentItem | AuditNicknameItem } | null>(null);

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

const commentPagination = reactive({
    page: 1,
    pageSize: 10,
    itemCount: 0,
    showSizePicker: false,
    prefix: ({ itemCount }: { itemCount: number }) => `共 ${itemCount} 条`,
});

const nicknamePagination = reactive({
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
        case 'comment_approve':
            return '评论通过';
        case 'comment_reject':
            return '评论拒绝';
        case 'reply_approve':
            return '回复通过';
        case 'reply_reject':
            return '回复拒绝';
        case 'nickname_approve':
            return '昵称通过';
        case 'nickname_reject':
            return '昵称拒绝';
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

// 评论/回复审核条目: 跳转对应帖子并定位高亮该评论(回复定位到所属评论下的回复)
const openCommentInNewTab = (row: AuditCommentItem) => {
    const commentId = row.comment_type === 1 ? row.comment_id : row.id;
    let url = `/#/post?id=${row.post_id}&comment_id=${commentId}`;
    if (row.comment_type === 1) {
        url += `&reply_id=${row.id}`;
    }
    window.open(url, '_blank', 'noopener');
};

// 昵称审核条目: 跳转对应用户主页
const openUserInNewTab = (username: string) => {
    window.open(`/#/u?s=${encodeURIComponent(username)}`, '_blank', 'noopener');
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

const loadComments = async () => {
    loading.value = true;
    try {
        const resp = await Api.v1.admin.get.audit.comments({
            status: tabStatusMap[activeTab.value] ?? 0,
            page: commentPagination.page,
            page_size: commentPagination.pageSize,
        });
        commentItems.value = resp.list || [];
        commentPagination.itemCount = resp.pager?.total_rows || 0;
    } catch (_err) {
        // do nothing
    } finally {
        loading.value = false;
    }
};

const loadNicknames = async () => {
    loading.value = true;
    try {
        const resp = await Api.v1.admin.get.audit.nicknames({
            page: nicknamePagination.page,
            page_size: nicknamePagination.pageSize,
        });
        nicknameItems.value = resp.list || [];
        nicknamePagination.itemCount = resp.pager?.total_rows || 0;
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
    switch (category.value) {
        case 'logs':
            loadLogs();
            break;
        case 'nickname':
            loadNicknames();
            break;
        case 'comment':
            loadComments();
            break;
        default:
            loadPosts();
            break;
    }
};

const handleCategoryChange = (tab: string) => {
    category.value = tab as typeof category.value;
    if (tab === 'logs') {
        logPagination.page = 1;
    } else if (tab === 'nickname') {
        nicknamePagination.page = 1;
    } else if (tab === 'comment') {
        commentPagination.page = 1;
    } else {
        postPagination.page = 1;
    }
    loadActiveTab();
};

const handleTabChange = (tab: string) => {
    activeTab.value = tab;
    if (category.value === 'comment') {
        commentPagination.page = 1;
        loadComments();
    } else {
        postPagination.page = 1;
        loadPosts();
    }
};

const handlePostPageChange = (page: number) => {
    postPagination.page = page;
    loadPosts();
};

const handleCommentPageChange = (page: number) => {
    commentPagination.page = page;
    loadComments();
};

const handleNicknamePageChange = (page: number) => {
    nicknamePagination.page = page;
    loadNicknames();
};

const handleLogPageChange = (page: number) => {
    logPagination.page = page;
    loadLogs();
};

const approveComment = async (row: AuditCommentItem) => {
    acting.value = true;
    try {
        await Api.v1.admin.post.audit.comment({
            id: row.id,
            comment_type: row.comment_type,
            action: 'approve',
        });
        window.$message.success('已通过审核');
        loadComments();
    } catch (_err) {
        // 错误提示由请求拦截器统一处理
    } finally {
        acting.value = false;
    }
};

const approveNickname = async (row: AuditNicknameItem) => {
    acting.value = true;
    try {
        await Api.v1.admin.post.audit.nickname({
            user_id: row.user_id,
            action: 'approve',
        });
        window.$message.success('已通过昵称变更');
        loadNicknames();
    } catch (_err) {
        // 错误提示由请求拦截器统一处理
    } finally {
        acting.value = false;
    }
};

const openReject = (
    kind: 'comment' | 'nickname',
    row: AuditCommentItem | AuditNicknameItem
) => {
    rejectTarget.value = { kind, row };
    rejectReason.value = '';
    showRejectModal.value = true;
};

const confirmReject = async () => {
    const target = rejectTarget.value;
    if (!target) {
        return true;
    }
    const reason = rejectReason.value.trim();
    if (!reason) {
        window.$message.warning('请填写拒绝原因');
        return false;
    }
    acting.value = true;
    try {
        if (target.kind === 'comment') {
            const row = target.row as AuditCommentItem;
            await Api.v1.admin.post.audit.comment({
                id: row.id,
                comment_type: row.comment_type,
                action: 'reject',
                reason,
            });
            loadComments();
        } else {
            const row = target.row as AuditNicknameItem;
            await Api.v1.admin.post.audit.nickname({
                user_id: row.user_id,
                action: 'reject',
                reason,
            });
            loadNicknames();
        }
        window.$message.success('已拒绝');
        showRejectModal.value = false;
    } catch (_err) {
        // 错误提示由请求拦截器统一处理
    } finally {
        acting.value = false;
    }
    return true;
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
            row.post_id > 0
                ? h(
                      'span',
                      {
                          class: 'post-id-link',
                          onClick: () => openInNewTab(row.post_id),
                      },
                      { default: () => `#${row.post_id}` }
                  )
                : '-',
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
        width: 100,
        render: (row) =>
            h(
                NTag,
                {
                    round: true,
                    size: 'small',
                    type: row.action.endsWith('approve')
                        ? 'success'
                        : row.action.endsWith('reject')
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

.status-tabs {
    margin-top: 8px;
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

        .nickname-change-wrap {
            display: flex;
            align-items: center;
            gap: 8px;

            .nickname-old {
                opacity: 0.6;
                text-decoration: line-through;
            }

            .nickname-arrow {
                opacity: 0.5;
            }

            .nickname-new {
                color: #18a058;
                font-weight: 600;
            }
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

.reject-modal-body {
    padding: 8px 0;
}

.post-id-link {
    color: #18a058;
    cursor: pointer;

    &:hover {
        text-decoration: underline;
    }
}
</style>
