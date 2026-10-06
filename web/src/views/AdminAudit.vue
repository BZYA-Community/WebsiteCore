<template>
    <div>
        <main-nav :title="t('adminAudit.pageTitle')" />

        <n-card :title="t('adminAudit.cardTitle')" size="small" class="setting-card">
            <n-alert type="info" style="margin-bottom: 12px">{{ t('adminAudit.task.assignmentHelp') }}</n-alert>
            <n-button size="small" :loading="loading" @click="loadActiveTab">{{ t('adminAudit.task.refresh') }}</n-button>
            <n-collapse style="margin: 12px 0" @item-header-click="loadStatistics">
                <n-collapse-item :title="t('adminAudit.task.statistics')" name="statistics">
                    <n-data-table :columns="statisticsColumns" :data="statistics" size="small" />
                </n-collapse-item>
            </n-collapse>
            <n-spin :show="loading">
                <n-tabs
                    :value="category"
                    type="segment"
                    @update:value="handleCategoryChange"
                >
                    <n-tab name="post">{{ t('adminAudit.tab.post') }}</n-tab>
                    <n-tab name="comment">{{ t('adminAudit.tab.comment') }}</n-tab>
                    <n-tab name="nickname">{{ t('adminAudit.tab.nickname') }}</n-tab>
                    <n-tab name="avatar">{{ t('adminAudit.tab.avatar') }}</n-tab>
                    <n-tab name="logs">{{ t('adminAudit.tab.logs') }}</n-tab>
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
                        <n-tab name="pending">{{ t('adminAudit.status.pending') }}</n-tab>
                        <n-tab name="approved">{{ t('adminAudit.status.approved') }}</n-tab>
                        <n-tab name="rejected">{{ t('adminAudit.status.rejected') }}</n-tab>
                    </n-tabs>

                    <!-- 帖子队列 -->
                    <div class="audit-cards" v-if="category === 'post'">
                        <div class="empty-wrap audit-empty" v-if="!loading && postItems.length === 0">
                            <n-empty size="large" :description="t('common.noData')" />
                        </div>
                        <div
                            v-for="row in postItems"
                            :key="row.id"
                            class="audit-card"
                            @click="openReview('post', row)"
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
                            <review-task-meta :task="row.review_task" />
                            <div class="audit-card-summary">{{ postSummary(row) }}</div>
                            <div class="audit-card-foot">
                                <span class="audit-card-author">
                                    {{ row.user?.nickname || '' }} @{{ row.user?.username || '' }}
                                </span>
                                <n-button
                                    size="tiny"
                                    quaternary
                                    type="info"
                                    @click.stop="openReview('post', row)"
                                >
                                    {{ t('adminAudit.action.viewPost') }}
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
                            <n-empty size="large" :description="t('common.noData')" />
                        </div>
                        <div
                            v-for="row in commentItems"
                            :key="`${row.comment_type}-${row.id}`"
                            class="audit-card"
                            @click="openReview('comment', row)"
                        >
                            <div class="audit-card-head">
                                <span class="audit-card-id">
                                    {{ commentTypeText(row.comment_type) }}#{{ row.id }}
                                </span>
                                <n-tag size="small" round :type="statusTagType(row.audit_status)">
                                    {{ statusText(row.audit_status) }}
                                </n-tag>
                                <span class="audit-card-time">
                                    {{ formatTime(row.created_on) }}
                                </span>
                            </div>
                            <review-task-meta :task="row.review_task" />
                            <div class="audit-card-summary">{{ row.content || t('adminAudit.noTextContent') }}</div>
                            <div class="audit-card-foot">
                                <span class="audit-card-author">
                                    {{ row.user?.nickname || '' }} @{{ row.user?.username || '' }}
                                </span>
                                <n-button
                                    size="tiny"
                                    quaternary
                                    type="info"
                                    @click.stop="openReview('comment', row)"
                                >
                                    {{ t('adminAudit.action.viewType', { type: commentTypeText(row.comment_type) }) }}
                                </n-button>
                                <n-button
                                    v-if="canReview(row)" size="tiny"
                                    quaternary
                                    type="success"
                                    :loading="acting"
                                    @click.stop="approveComment(row)"
                                >
                                    {{ t('adminAudit.action.approve') }}
                                </n-button>
                                <n-button
                                    v-if="canReview(row)" size="tiny"
                                    quaternary
                                    type="error"
                                    @click.stop="openReject('comment', row)"
                                >
                                    {{ t('adminAudit.action.reject') }}
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
                            <n-empty size="large" :description="t('common.noData')" />
                        </div>
                        <div
                            v-for="row in nicknameItems"
                            :key="row.user_id"
                            class="audit-card"
                            @click="openUserInNewTab(row.username)"
                        >
                            <div class="audit-card-head">
                                <span class="audit-card-id">@{{ row.username }}</span>
                                <n-tag size="small" round type="warning">{{ t('adminAudit.status.pending') }}</n-tag>
                                <span class="audit-card-time">
                                    {{ formatTime(row.created_on) }}
                                </span>
                            </div>
                            <review-task-meta :task="row.review_task" />
                            <div class="audit-card-summary nickname-change-wrap">
                                <span class="nickname-old">{{ row.nickname || t('adminAudit.emptyText') }}</span>
                                <span class="nickname-arrow">→</span>
                                <span class="nickname-new">{{ row.pending_nickname }}</span>
                            </div>
                            <div class="audit-card-foot">
                                <span class="audit-card-author">{{ t('adminAudit.nicknameChangeRequest') }}</span>
                                <n-button
                                    size="tiny"
                                    quaternary
                                    type="info"
                                    @click.stop="openUserInNewTab(row.username)"
                                >
                                    {{ t('adminAudit.action.viewHome') }}
                                </n-button>
                                <n-button
                                    v-if="canReview(row)" size="tiny"
                                    quaternary
                                    type="success"
                                    :loading="acting"
                                    @click.stop="approveNickname(row)"
                                >
                                    {{ t('adminAudit.action.approve') }}
                                </n-button>
                                <n-button
                                    v-if="canReview(row)" size="tiny"
                                    quaternary
                                    type="error"
                                    @click.stop="openReject('nickname', row)"
                                >
                                    {{ t('adminAudit.action.reject') }}
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

                <!-- 头像队列 -->
                <template v-else-if="category === 'avatar'">
                    <div class="audit-cards">
                        <div class="empty-wrap audit-empty" v-if="!loading && avatarItems.length === 0">
                            <n-empty size="large" :description="t('common.noData')" />
                        </div>
                        <div
                            v-for="row in avatarItems"
                            :key="row.user_id"
                            class="audit-card"
                            @click="openUserInNewTab(row.username)"
                        >
                            <div class="audit-card-head">
                                <span class="audit-card-id">@{{ row.username }}</span>
                                <n-tag size="small" round type="warning">{{ t('adminAudit.status.pending') }}</n-tag>
                                <span class="audit-card-time">
                                    {{ formatTime(row.created_on) }}
                                </span>
                            </div>
                            <review-task-meta :task="row.review_task" />
                            <div class="audit-card-summary avatar-change-wrap">
                                <n-avatar :size="64" :src="row.avatar" />
                                <span class="nickname-arrow">→</span>
                                <n-avatar :size="64" :src="row.pending_avatar" />
                            </div>
                            <div class="audit-card-foot">
                                <span class="audit-card-author">{{ t('adminAudit.avatarChangeRequest') }}</span>
                                <n-button
                                    size="tiny"
                                    quaternary
                                    type="info"
                                    @click.stop="openUserInNewTab(row.username)"
                                >
                                    {{ t('adminAudit.action.viewHome') }}
                                </n-button>
                                <n-button
                                    v-if="canReview(row)" size="tiny"
                                    quaternary
                                    type="success"
                                    :loading="acting"
                                    @click.stop="approveAvatar(row)"
                                >
                                    {{ t('adminAudit.action.approve') }}
                                </n-button>
                                <n-button
                                    v-if="canReview(row)" size="tiny"
                                    quaternary
                                    type="error"
                                    @click.stop="openReject('avatar', row)"
                                >
                                    {{ t('adminAudit.action.reject') }}
                                </n-button>
                            </div>
                        </div>
                        <div
                            class="audit-card-pager"
                            v-if="avatarPagination.itemCount > avatarPagination.pageSize"
                        >
                            <n-pagination
                                :page="avatarPagination.page"
                                :page-size="avatarPagination.pageSize"
                                :item-count="avatarPagination.itemCount"
                                @update:page="handleAvatarPageChange"
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

        <n-modal v-model:show="showReview" preset="card" :title="t('adminAudit.task.fullContent')" style="width: min(840px, calc(100vw - 32px)); max-height: calc(100dvh - 32px); overflow-y: auto">
            <template v-if="selectedReview">
                <review-task-meta :task="selectedReview.row.review_task" />
                <review-content :contents="reviewContents" />
                <n-space v-if="canReview(selectedReview.row)" justify="end" style="margin-top: 20px">
                    <n-button type="error" secondary :disabled="acting" @click="openReject(selectedReview.kind, selectedReview.row)">{{ t('adminAudit.action.reject') }}</n-button>
                    <n-button type="primary" :loading="acting" @click="approveReview">{{ t('adminAudit.action.approve') }}</n-button>
                </n-space>
            </template>
        </n-modal>

        <!-- 拒绝原因弹窗(评论/昵称共用) -->
        <n-modal
            v-model:show="showRejectModal"
            preset="dialog"
            :title="t('adminAudit.rejectTitle')"
            :positive-text="t('adminAudit.confirmReject')"
            :negative-text="t('common.cancel')"
            @positive-click="confirmReject"
        >
            <div class="reject-modal-body">
                <n-input
                    v-model:value="rejectReason"
                    type="textarea"
                    :placeholder="t('adminAudit.rejectPlaceholder')"
                    :autosize="{ minRows: 2, maxRows: 4 }"
                    maxlength="120"
                    show-count
                />
            </div>
        </n-modal>

    </div>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { NTag } from 'naive-ui';
import type { DataTableColumns } from 'naive-ui';
import { formatTime } from '@/utils/formatTime';
import { mdPlainText } from '@/utils/markdown';
import { Api, request } from '@/utils/request';
import ReviewTaskMeta from '@/components/review-task-meta.vue';
import ReviewContent from '@/components/review-content.vue';
import { useStoreUser } from '@/store/user';

type AuditPostItem = Api.Admin.NetReq.AuditPostItem;
type AuditCommentItem = Api.Admin.NetReq.AuditCommentItem;
type AuditNicknameItem = Api.Admin.NetReq.AuditNicknameItem;
type AuditAvatarItem = Api.Admin.NetReq.AuditAvatarItem;
type AuditLogItem = Api.Admin.NetReq.AuditLogItem;

const { t } = useI18n();
const storeUser = useStoreUser();
const canReview = (row: { review_task?: Api.Admin.NetReq.ReviewTask }) => {
    const task = row.review_task;
    return !!task && task.state === 'pending' && storeUser.hasPermission('content.review') && (task.assignee_id === storeUser.userInfo.id || storeUser.userInfo.is_operator);
};
const showReview = ref(false);
const selectedReview = ref<{ kind: 'post' | 'comment'; row: AuditPostItem | AuditCommentItem }>();
const reviewContents = computed(() => {
    const selected = selectedReview.value;
    if (!selected) return [];
    if (selected.kind === 'post') return (selected.row as AuditPostItem).contents || [];
    const row = selected.row as AuditCommentItem;
    return row.contents?.length ? row.contents : [{ id: row.id, content: row.content, type: 2, sort: 0 }];
});
function openReview(kind: 'post' | 'comment', row: AuditPostItem | AuditCommentItem) { selectedReview.value = { kind, row }; showReview.value = true; }
async function approveReview() {
    if (!selectedReview.value) return;
    if (selectedReview.value.kind === 'post') await approvePost(selectedReview.value.row as AuditPostItem);
    else await approveComment(selectedReview.value.row as AuditCommentItem);
}
const statistics = ref<{ user_id: number; timeout_count: number }[]>([]);
async function loadStatistics() {
    try { statistics.value = (await request<unknown, { items: typeof statistics.value }>({ method: 'get', url: '/v1/admin/audit/statistics' })).items || []; }
    catch { /* Request errors preserve the last successful statistics. */ }
}
const statisticsColumns = computed(() => [
    { title: t('adminAudit.task.assignee'), key: 'user_id' },
    { title: t('adminAudit.task.timeouts'), key: 'timeout_count' },
]);

const loading = ref(false);
const acting = ref(false);
const category = ref<'post' | 'comment' | 'nickname' | 'avatar' | 'logs'>('post');
const activeTab = ref('pending');
const postItems = ref<AuditPostItem[]>([]);
const commentItems = ref<AuditCommentItem[]>([]);
const nicknameItems = ref<AuditNicknameItem[]>([]);
const avatarItems = ref<AuditAvatarItem[]>([]);
const logItems = ref<AuditLogItem[]>([]);

// 拒绝原因弹窗
const showRejectModal = ref(false);
const rejectReason = ref('');
const rejectTarget = ref<{ kind: 'post' | 'comment' | 'nickname' | 'avatar'; row: AuditPostItem | AuditCommentItem | AuditNicknameItem | AuditAvatarItem } | null>(null);

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
    prefix: ({ itemCount }: { itemCount: number }) => t('adminAudit.paginationPrefix', { count: itemCount }),
});

const commentPagination = reactive({
    page: 1,
    pageSize: 10,
    itemCount: 0,
    showSizePicker: false,
    prefix: ({ itemCount }: { itemCount: number }) => t('adminAudit.paginationPrefix', { count: itemCount }),
});

const nicknamePagination = reactive({
    page: 1,
    pageSize: 10,
    itemCount: 0,
    showSizePicker: false,
    prefix: ({ itemCount }: { itemCount: number }) => t('adminAudit.paginationPrefix', { count: itemCount }),
});

const avatarPagination = reactive({
    page: 1,
    pageSize: 10,
    itemCount: 0,
    showSizePicker: false,
    prefix: ({ itemCount }: { itemCount: number }) => t('adminAudit.paginationPrefix', { count: itemCount }),
});

const logPagination = reactive({
    page: 1,
    pageSize: 10,
    itemCount: 0,
    showSizePicker: false,
    prefix: ({ itemCount }: { itemCount: number }) => t('adminAudit.paginationPrefix', { count: itemCount }),
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
            return t('adminAudit.status.pending');
        case 1:
            return t('adminAudit.status.approved');
        case 2:
            return t('adminAudit.status.rejected');
        default:
            return String(status);
    }
};

// comment_type: 0帖子评论 1帖子回复 2课程评论 3课程回复
const commentTypeText = (type: number) => {
    switch (type) {
        case 1:
            return t('adminAudit.commentType.reply');
        case 2:
            return t('adminAudit.commentType.courseComment');
        case 3:
            return t('adminAudit.commentType.courseReply');
        default:
            return t('adminAudit.commentType.comment');
    }
};

const actionText = (action: string) => {
    switch (action) {
        case 'approve':
            return t('adminAudit.action.approveAction');
        case 'reject':
            return t('adminAudit.action.rejectAction');
        case 'delete':
            return t('adminAudit.action.deleteAction');
        case 'comment_approve':
            return t('adminAudit.action.commentApprove');
        case 'comment_reject':
            return t('adminAudit.action.commentReject');
        case 'reply_approve':
            return t('adminAudit.action.replyApprove');
        case 'reply_reject':
            return t('adminAudit.action.replyReject');
        case 'nickname_approve':
            return t('adminAudit.action.nicknameApprove');
        case 'nickname_reject':
            return t('adminAudit.action.nicknameReject');
        case 'avatar_approve':
            return t('adminAudit.action.avatarApprove');
        case 'avatar_reject':
            return t('adminAudit.action.avatarReject');
        case 'course_comment_approve':
            return t('adminAudit.action.courseCommentApprove');
        case 'course_comment_reject':
            return t('adminAudit.action.courseCommentReject');
        case 'course_reply_approve':
            return t('adminAudit.action.courseReplyApprove');
        case 'course_reply_reject':
            return t('adminAudit.action.courseReplyReject');
        case 'course_delete':
            return t('adminAudit.action.courseDelete');
        default:
            return action;
    }
};

const contentTypeText = (type: number) => {
    switch (type) {
        case 1:
            return t('adminAudit.contentType.title');
        case 2:
            return t('adminAudit.contentType.text');
        case 3:
            return t('adminAudit.contentType.image');
        case 4:
            return t('adminAudit.contentType.video');
        case 5:
            return t('adminAudit.contentType.audio');
        case 6:
            return t('adminAudit.contentType.link');
        case 7:
            return t('adminAudit.contentType.attachment');
        case 8:
            return t('adminAudit.contentType.paidAttachment');
        case 9:
            return t('adminAudit.contentType.markdownLong');
        default:
            return t('adminAudit.contentType.typeN', { n: type });
    }
};

const visibilityText = (visibility: number) => {
    switch (visibility) {
        case 0:
            return t('adminAudit.visibility.private');
        case 50:
            return t('adminAudit.visibility.friendsOnly');
        case 60:
            return t('adminAudit.visibility.followOnly');
        case 90:
            return t('adminAudit.visibility.public');
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
        return t('adminAudit.mediaCount', { count: media.length, kinds: kinds.join('/') });
    }
    return t('adminAudit.noContent');
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
    } catch {
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
    } catch {
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
    } catch {
        // do nothing
    } finally {
        loading.value = false;
    }
};

const loadAvatars = async () => {
    loading.value = true;
    try {
        const resp = await Api.v1.admin.get.audit.avatars({
            page: avatarPagination.page,
            page_size: avatarPagination.pageSize,
        });
        avatarItems.value = resp.list || [];
        avatarPagination.itemCount = resp.pager?.total_rows || 0;
    } catch {
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
    } catch {
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
        case 'avatar':
            loadAvatars();
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
    } else if (tab === 'avatar') {
        avatarPagination.page = 1;
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

const handleAvatarPageChange = (page: number) => {
    avatarPagination.page = page;
    loadAvatars();
};

const handleLogPageChange = (page: number) => {
    logPagination.page = page;
    loadLogs();
};

const approvePost = async (row: AuditPostItem) => {
    if (!canReview(row)) return;
    acting.value = true;
    try {
        await Api.v1.admin.post.audit.post({ post_id: row.id, task_id: row.review_task!.id, revision: row.review_task!.revision, action: 'approve' });
        showReview.value = false;
        window.$message.success(t('adminAudit.msg.approved'));
        loadPosts();
    } catch { /* The request interceptor displays stale or reassigned tasks. */ }
    finally { acting.value = false; }
};

const approveComment = async (row: AuditCommentItem) => {
    if (!canReview(row)) return;
    acting.value = true;
    try {
        await Api.v1.admin.post.audit.comment({
            task_id: row.review_task!.id, revision: row.review_task!.revision,
            id: row.id,
            comment_type: row.comment_type,
            action: 'approve',
        });
        showReview.value = false;
        window.$message.success(t('adminAudit.msg.approved'));
        loadComments();
    } catch {
        // 错误提示由请求拦截器统一处理
    } finally {
        acting.value = false;
    }
};

const approveNickname = async (row: AuditNicknameItem) => {
    if (!canReview(row)) return;
    acting.value = true;
    try {
        await Api.v1.admin.post.audit.nickname({
            task_id: row.review_task!.id, revision: row.review_task!.revision,
            user_id: row.user_id,
            action: 'approve',
        });
        window.$message.success(t('adminAudit.msg.nicknameApproved'));
        loadNicknames();
    } catch {
        // 错误提示由请求拦截器统一处理
    } finally {
        acting.value = false;
    }
};

const approveAvatar = async (row: AuditAvatarItem) => {
    if (!canReview(row)) return;
    acting.value = true;
    try {
        await Api.v1.admin.post.audit.avatar({
            task_id: row.review_task!.id, revision: row.review_task!.revision,
            user_id: row.user_id,
            action: 'approve',
        });
        window.$message.success(t('adminAudit.msg.avatarApproved'));
        loadAvatars();
    } catch {
        // 错误提示由请求拦截器统一处理
    } finally {
        acting.value = false;
    }
};

const openReject = (
    kind: 'post' | 'comment' | 'nickname' | 'avatar',
    row: AuditPostItem | AuditCommentItem | AuditNicknameItem | AuditAvatarItem
) => {
    rejectTarget.value = { kind, row };
    rejectReason.value = '';
    showRejectModal.value = true;
};

const confirmReject = async () => {
    const target = rejectTarget.value;
    if (!target || !canReview(target.row)) return false;
    const reason = rejectReason.value.trim();
    if (!reason) {
        window.$message.warning(t('adminAudit.msg.fillRejectReason'));
        return false;
    }
    acting.value = true;
    try {
        if (target.kind === 'post') {
            const row = target.row as AuditPostItem;
            await Api.v1.admin.post.audit.post({ post_id: row.id, task_id: row.review_task!.id, revision: row.review_task!.revision, action: 'reject', reason });
            loadPosts();
        } else if (target.kind === 'comment') {
            const row = target.row as AuditCommentItem;
            await Api.v1.admin.post.audit.comment({
            task_id: row.review_task!.id, revision: row.review_task!.revision,
                id: row.id,
                comment_type: row.comment_type,
                action: 'reject',
                reason,
            });
            loadComments();
        } else if (target.kind === 'nickname') {
            const row = target.row as AuditNicknameItem;
            await Api.v1.admin.post.audit.nickname({
            task_id: row.review_task!.id, revision: row.review_task!.revision,
                user_id: row.user_id,
                action: 'reject',
                reason,
            });
            loadNicknames();
        } else {
            const row = target.row as AuditAvatarItem;
            await Api.v1.admin.post.audit.avatar({
            task_id: row.review_task!.id, revision: row.review_task!.revision,
                user_id: row.user_id,
                action: 'reject',
                reason,
            });
            loadAvatars();
        }
        window.$message.success(t('adminAudit.msg.rejected'));
        showRejectModal.value = false;
        showReview.value = false;
    } catch {
        return false;
    } finally {
        acting.value = false;
    }
    return true;
};

const logColumns = computed<DataTableColumns<AuditLogItem>>(() => [
    {
        title: 'ID',
        key: 'id',
        width: 70,
    },
    {
        title: t('adminAudit.table.postId'),
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
        title: t('adminAudit.table.operator'),
        key: 'operator_name',
        width: 120,
        ellipsis: { tooltip: true },
        render: (row) => row.operator_name || `#${row.operator_id}`,
    },
    {
        title: t('adminAudit.table.action'),
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
        title: t('adminAudit.table.statusChange'),
        key: 'status',
        width: 130,
        render: (row) => `${statusText(row.old_status)} → ${statusText(row.new_status)}`,
    },
    {
        title: t('adminAudit.table.reason'),
        key: 'reason',
        ellipsis: { tooltip: true },
        render: (row) => row.reason || '-',
    },
    {
        title: t('adminAudit.table.time'),
        key: 'created_on',
        width: 150,
        render: (row) => formatTime(row.created_on),
    },
]);

onMounted(async () => {
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

        .avatar-change-wrap {
            display: flex;
            align-items: center;
            gap: 12px;

            .nickname-arrow {
                opacity: 0.5;
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
