<template>
    <div class="detail-item" @click="goPostDetail(post.id)">
        <n-thing>
            <template #avatar>
                <n-avatar round :size="30" :src="post.user.avatar" />
            </template>
            <template #header>
                <router-link
                    @click.stop
                    class="username-link"
                    :to="{
                        name: 'user',
                        query: { s: post.user.username },
                    }"
                >
                    {{ post.user.nickname }}
                </router-link>
                <span class="username-wrap"> @{{ post.user.username }} </span>
                <n-tag
                    v-if="post.is_top"
                    class="top-tag"
                    type="warning"
                    size="small"
                    round
                >
                    {{ t('post.status.pinned') }}
                </n-tag>
                <n-tag
                    v-if="post.visibility == VisibilityEnum.PRIVATE"
                    class="top-tag"
                    type="error"
                    size="small"
                    round
                >
                    {{ t('post.status.private') }}
                </n-tag>
            </template>
            <template #header-extra>
                <div class="options">
                    <n-dropdown
                        placement="bottom-end"
                        trigger="click"
                        size="small"
                        :options="adminOptions"
                        @select="handlePostAction"
                    >
                        <n-button quaternary circle>
                            <template #icon>
                                <n-icon>
                                    <more-horiz-filled />
                                </n-icon>
                            </template>
                        </n-button>
                    </n-dropdown>
                </div>

                <!-- 删除确认 -->
                <n-modal
                    v-model:show="showDelModal"
                    :mask-closable="false"
                    preset="dialog"
                    :title="t('post.dialog.tipTitle')"
                    :content="t('post.dialog.deleteContent')"
                    :positive-text="t('common.confirm')"
                    :negative-text="t('common.cancel')"
                    @positive-click="execDelAction"
                />
                <!-- 锁定确认 -->
                <n-modal
                    v-model:show="showLockModal"
                    :mask-closable="false"
                    preset="dialog"
                    :title="t('post.dialog.tipTitle')"
                    :content="t('post.dialog.lockContent', { action: post.is_lock ? t('post.action.unlock') : t('post.action.lock') })"
                    :positive-text="t('common.confirm')"
                    :negative-text="t('common.cancel')"
                    @positive-click="execLockAction"
                />
                <!-- 置顶确认 -->
                <n-modal
                    v-model:show="showStickModal"
                    :mask-closable="false"
                    preset="dialog"
                    :title="t('post.dialog.tipTitle')"
                    :content="t('post.dialog.stickContent', { action: post.is_top ? t('post.action.unstick') : t('post.action.stick') })"
                    :positive-text="t('common.confirm')"
                    :negative-text="t('common.cancel')"
                    @positive-click="execStickAction"
                />
                <!-- 亮点确认 -->
                <n-modal
                    v-model:show="showHighlightModal"
                    :mask-closable="false"
                    preset="dialog"
                    :title="t('post.dialog.tipTitle')"
                    :content="t('post.dialog.highlightContent', { action: post.is_essence ? t('post.action.unhighlight') : t('post.action.highlight') })"
                    :positive-text="t('common.confirm')"
                    :negative-text="t('common.cancel')"
                    @positive-click="execHighlightAction"
                />
                <!-- 修改可见度确认 -->
                <n-modal
                    v-model:show="showVisibilityModal"
                    :mask-closable="false"
                    preset="dialog"
                    :title="t('post.dialog.tipTitle')"
                    :content="t('post.dialog.visibilityContent', { visibility: getVisibilityName(tempVisibility) })"
                    :positive-text="t('common.confirm')"
                    :negative-text="t('common.cancel')"
                    @positive-click="execVisibilityAction"
                />
                  <!-- 审核拒绝原因 -->
                <n-modal
                    v-model:show="showAuditReject"
                    :mask-closable="false"
                    preset="dialog"
                    :title="t('post.dialog.rejectTitle')"
                    :positive-text="t('post.action.confirmReject')"
                    :negative-text="t('common.cancel')"
                    @positive-click="execAuditReject"
                >
                    <n-space vertical>
                        <div class="audit-reject-tip">
                            {{ t('post.dialog.rejectTip') }}
                        </div>
                        <n-input
                            v-model:value="auditRejectReason"
                            type="textarea"
                            :placeholder="t('post.dialog.rejectPlaceholder')"
                            :autosize="{ minRows: 2, maxRows: 4 }"
                            maxlength="255"
                            show-count
                        />
                    </n-space>
                </n-modal>
            </template>
            <div v-if="showAuditBar" class="audit-bar" @click.stop>
                <n-tag
                    :type="post.audit_status === 2 ? 'error' : 'warning'"
                    size="small"
                    round
                >
                    {{ post.audit_status === 2 ? t('post.status.auditRejected') : t('post.status.pendingAudit') }}
                </n-tag>
                <span class="audit-bar-tip">
                    {{ t('post.audit.barTip') }}
                </span>
                <n-space size="small">
                    <n-button
                        size="small"
                        type="success"
                        secondary
                        :loading="auditActing"
                        @click.stop="handleAuditApprove"
                    >
                        {{ t('post.action.approve') }}
                    </n-button>
                    <n-button
                        size="small"
                        type="warning"
                        secondary
                        :disabled="auditActing"
                        @click.stop="showAuditReject = true"
                    >
                        {{ t('post.action.reject') }}
                    </n-button>
                </n-space>
            </div>

            <div v-if="post.texts.length > 0">
                <span
                    v-for="content in post.texts"
                    :key="content.id"
                    class="post-text"
                    @click.stop="doClickText($event, post.id)"
                    v-html="parsePostTag(content.content).content"
                >
                </span>
            </div>

            <div
                v-if="post.markdowns.length > 0"
                class="post-markdown-wrap"
                @click.stop="handleMdClick($event, post.id)"
            >
                <md-preview
                    v-for="md in post.markdowns"
                    :key="md.id"
                    :model-value="prepareMdRender(md.content)"
                    :theme="editorTheme"
                    no-mermaid
                    no-katex
                />
            </div>

            <template #footer>
                <post-attachment :attachments="post.attachments" />
                <post-attachment :attachments="post.charge_attachments" />
                <post-image :imgs="post.imgs" />
                <post-video :videos="post.videos" :full="true" />
                <post-link :links="post.links" />
                <div class="timestamp">
                    {{ t('post.publishAt') }} {{ formatPrettyTime(post.created_on) }}
                    <span v-if="post.ip_loc">
                        <n-divider vertical />
                        {{ post.ip_loc }}
                    </span>
                    <span v-if="!collapsedLeft && post.created_on != post.latest_replied_on">
                        <n-divider vertical /> {{ t('post.lastReply') }}
                        {{ formatPrettyTime(post.latest_replied_on) }}
                    </span>
                </div>
            </template>
            <template #action>
                <div class="opts-wrap">
                    <n-space justify="space-between">
                        <div
                            class="opt-item hover"
                            @click.stop="handlePostStar"
                        >
                            <n-icon size="20" class="opt-item-icon">
                                <heart-outline v-if="!hasStarred" />
                                <heart v-if="hasStarred" color="red" />
                            </n-icon>
                            {{ post.upvote_count }}
                        </div>
                        <div class="opt-item">
                            <n-icon size="20" class="opt-item-icon">
                                <chatbox-outline />
                            </n-icon>
                            {{ post.comment_count }}
                        </div>
                        <div
                            class="opt-item hover"
                            @click.stop="handlePostCollection"
                        >
                            <n-icon size="20" class="opt-item-icon">
                                <bookmark-outline v-if="!hasCollected" />
                                <bookmark v-if="hasCollected" color="#ff7600" />
                            </n-icon>
                            {{ post.collection_count }}
                        </div>
                        <div
                            class="opt-item hover"
                            @click.stop="handlePostShare"
                        >
                            <n-icon size="20" class="opt-item-icon">
                                <share-social-outline />
                            </n-icon>
                            {{ post.share_count }}
                        </div>
                    </n-space>
                </div>
            </template>
        </n-thing>
    </div>
</template>

<script setup lang="ts">
import { h, ref, onMounted, computed } from 'vue';
import type { Component } from 'vue';
import { NIcon, useDialog } from 'naive-ui';
import { useI18n } from 'vue-i18n';
import { useStoreMain } from '@/store/main';
import { useRouter } from 'vue-router';
import { formatPrettyTime } from '@/utils/formatTime';
import { parsePostTag } from '@/utils/content';
import { MdPreview } from 'md-editor-v3';
import { mdTheme, prepareMdRender } from '@/utils/markdown';
import {
  PaperPlaneOutline,
  Heart,
  HeartOutline,
  Bookmark,
  BookmarkOutline,
  ShareSocialOutline,
  ChatboxOutline,
  PushOutline,
  TrashOutline,
  LockClosedOutline,
  LockOpenOutline,
  EyeOutline,
  EyeOffOutline,
  BodyOutline,
  WalkOutline,
  PersonOutline,
  FlameOutline,
} from '@vicons/ionicons5';
import { MoreHorizFilled } from '@vicons/material';
import {
  getPostStar,
  postStar,
  getPostCollection,
  postCollection,
  deletePost,
  lockPost,
  stickPost,
  highlightPost,
  visibilityPost,
} from '@/api/post';
import type { DropdownOption } from 'naive-ui';
import { VisibilityEnum } from '@/utils/IEnum';
import copy from 'copy-to-clipboard';
import { storeToRefs } from 'pinia';
import { useStoreUser } from '@/store/user';
import { Api } from '@/utils/request';
import UserAction, { canWhisperUser, useChatJump } from '@/composables/useUserAction';
import { usePostContent } from '@/composables/usePostContent';

const { t } = useI18n();
const storeMain = useStoreMain();
const storeUser = useStoreUser();
const { collapsedLeft, theme } = storeToRefs(storeMain);
const { userInfo } = storeToRefs(storeUser);
const editorTheme = computed(() => mdTheme(theme.value));

const router = useRouter();
const dialog = useDialog();
const hasStarred = ref(false);
const hasCollected = ref(false);
const props = withDefaults(
  defineProps<{
    post: Item.PostProps;
  }>(),
  {},
);
const showDelModal = ref(false);
const showLockModal = ref(false);
const showStickModal = ref(false);
const showHighlightModal = ref(false);
const showVisibilityModal = ref(false);
const loading = ref(false);
const tempVisibility = ref<VisibilityEnum>(VisibilityEnum.PUBLIC);
// 私信入口: 跳转消息页会话(原 whisper 弹窗已移除)
const { goWhisper: onSendWhisper } = useChatJump();
// 审核操作(审核员/管理员在详情页直接审核)
const showAuditReject = ref(false);
const auditRejectReason = ref('');
const auditActing = ref(false);
const isAuditor = computed(
  () =>
    userInfo.value.id > 0 &&
    (userInfo.value.is_admin || (userInfo.value.roles || []).includes('auditor')),
);
const showAuditBar = computed(
  () =>
    isAuditor.value &&
    post.value.audit_status !== undefined &&
    post.value.audit_status !== 1,
);

const emit = defineEmits<{
  (e: 'reload', post_id: number): void;
}>();

// 使用 usePostContent composable (包含额外字段)
const post = usePostContent(props.post, true);

const renderIcon = (icon: Component) => {
  return () => {
    return h(NIcon, null, {
      default: () => h(icon),
    });
  };
};

const getVisibilityName = (v: number) => {
  switch (v) {
    case 0: return t('post.status.public');
    case 1: return t('post.status.private');
    case 2: return t('post.status.friendVisible');
    default: return t('post.status.followingVisible');
  }
};

const adminOptions = computed(() => {
  let options: DropdownOption[] = [];
  if (
    !userInfo.value.is_admin &&
    userInfo.value.id != props.post.user.id
  ) {
    // 私信入口: 道友仅对高级身份可见(后端仍强制校验)
    if (canWhisperUser(props.post.user)) {
      options.push({
        label: t('post.menu.whisper', { user: props.post.user.username }),
        key: 'whisper',
        icon: renderIcon(PaperPlaneOutline),
      });
    }
    if (props.post.user.is_following) {
      options.push({
        label: t('post.menu.unfollowUser', { user: props.post.user.username }),
        key: 'unfollow',
        icon: renderIcon(WalkOutline),
      });
    } else {
      options.push({
        label: t('post.menu.followUser', { user: props.post.user.username }),
        key: 'follow',
        icon: renderIcon(BodyOutline),
      });
    }
    return options;
  }
  options.push({
    label: t('post.action.delete'),
    key: 'delete',
    icon: renderIcon(TrashOutline),
  });
  if (post.value.is_lock === 0) {
    options.push({
      label: t('post.action.lock'),
      key: 'lock',
      icon: renderIcon(LockClosedOutline),
    });
  } else {
    options.push({
      label: t('post.action.unlock'),
      key: 'unlock',
      icon: renderIcon(LockOpenOutline),
    });
  }
  if (userInfo.value.is_admin) {
    if (post.value.is_top === 0) {
      options.push({
        label: t('post.action.stick'),
        key: 'stick',
        icon: renderIcon(PushOutline),
      });
    } else {
      options.push({
        label: t('post.action.unstick'),
        key: 'unstick',
        icon: renderIcon(PushOutline),
      });
    }
  }
  if (post.value.is_essence === 0) {
    options.push({
      label: t('post.action.highlight'),
      key: 'highlight',
      icon: renderIcon(FlameOutline),
    });
  } else {
    options.push({
      label: t('post.action.unhighlight'),
      key: 'unhighlight',
      icon: renderIcon(FlameOutline),
    });
  }
  let visitMenu: DropdownOption;
  if (post.value.visibility === VisibilityEnum.PUBLIC) {
    visitMenu = {
      label: t('post.status.public'),
      key: 'vpublic',
      icon: renderIcon(EyeOutline),
      children: [
        { label: t('post.status.private'), key: 'vprivate', icon: renderIcon(EyeOffOutline) },
        { label: t('post.status.followingVisible'), key: 'vfollowing', icon: renderIcon(BodyOutline) },
      ],
    };
  } else if (post.value.visibility === VisibilityEnum.PRIVATE) {
    visitMenu = {
      label: t('post.status.private'),
      key: 'vprivate',
      icon: renderIcon(EyeOffOutline),
      children: [
        { label: t('post.status.public'), key: 'vpublic', icon: renderIcon(EyeOutline) },
        { label: t('post.status.followingVisible'), key: 'vfollowing', icon: renderIcon(BodyOutline) },
      ],
    };
  } else {
    visitMenu = {
      label: t('post.status.followingVisible'),
      key: 'vfollowing',
      icon: renderIcon(BodyOutline),
      children: [
        { label: t('post.status.public'), key: 'vpublic', icon: renderIcon(EyeOutline) },
        { label: t('post.status.private'), key: 'vprivate', icon: renderIcon(EyeOffOutline) },
      ],
    };
  }
  options.push(visitMenu);
  return options;
});

const onHandleFollowAction = (post: Item.PostProps) => {
	UserAction.followAction(dialog, post.user.id, post.user.username, post.user.is_following)
		.then(_action => {
			post.user.is_following = _action;
		})
};

const goPostDetail = (id: number) => {
  router.push({
    name: 'post',
    query: {
      id,
    },
  });
};
const doClickText = (e: MouseEvent, id: number) => {
  if ((e.target as any).dataset.detail) {
    const d = (e.target as any).dataset.detail.split(':');
    if (d.length === 2) {
      storeMain.doRefresh();
      if (d[0] === 'tag') {
        router.push({
          name: 'home',
          query: {
            q: d[1],
            t: 'tag',
          },
        });
      } else {
        router.push({
          name: 'user',
          query: {
            s: d[1],
          },
        });
      }
      return;
    }
  }
  goPostDetail(id);
};
// Markdown渲染区点击委托: 话题/站外链接拦截
const handleMdClick = (e: MouseEvent, _id: number) => {
  const anchor = (e.target as HTMLElement).closest('a');
  if (!anchor) {
    return;
  }
  const href = anchor.getAttribute('href') || '';
  const text = anchor.textContent || '';
  if (href === '#' && (text.startsWith('#') || text.startsWith('＃'))) {
    e.preventDefault();
    const tag = text.replace(/^[#＃]/, '').replace(/[#＃]$/, '');
    if (tag) {
      storeMain.doRefresh();
      router.push({
        name: 'home',
        query: {
          q: tag,
          t: 'tag',
        },
      });
    }
    return;
  }
  if (href.startsWith('http://') || href.startsWith('https://')) {
    e.preventDefault();
    window.open(href, '_blank', 'noopener,noreferrer');
  }
};
const handleAuditApprove = () => {
  dialog.success({
    title: t('post.dialog.auditApproveTitle'),
    content: t('post.dialog.auditApproveContent'),
    positiveText: t('post.action.approve'),
    negativeText: t('common.cancel'),
    onPositiveClick: () => doAuditAction('approve'),
  });
};
const execAuditReject = () => {
  const reason = auditRejectReason.value.trim();
  if (!reason) {
    window.$message.warning(t('post.audit.rejectReasonRequired'));
    return false;
  }
  doAuditAction('reject', reason);
  return true;
};
const doAuditAction = async (action: 'approve' | 'reject', reason?: string) => {
  auditActing.value = true;
  try {
    await Api.v1.admin.post.audit.post({
      post_id: post.value.id,
      action,
      reason,
    });
    window.$message.success(action === 'approve' ? t('post.msg.auditApproved') : t('post.msg.auditRejected'));
    emit('reload', post.value.id);
  } catch (_err) {
    // 错误提示由请求拦截器统一处理
  } finally {
    auditActing.value = false;
  }
};
const handlePostAction = (
  item:
    | 'whisper'
    | 'follow'
    | 'unfollow'
    | 'delete'
    | 'lock'
    | 'unlock'
    | 'stick'
    | 'unstick'
    | 'highlight'
    | 'unhighlight'
    | 'vpublic'
    | 'vprivate'
    | 'vfollowing',
) => {
  switch (item) {
    case 'whisper':
      onSendWhisper(props.post.user);
      break;
    case 'follow':
    case 'unfollow':
      onHandleFollowAction(props.post);
      break;
    case 'delete':
      showDelModal.value = true;
      break;
    case 'lock':
    case 'unlock':
      showLockModal.value = true;
      break;
    case 'stick':
    case 'unstick':
      showStickModal.value = true;
      break;
    case 'highlight':
    case 'unhighlight':
      showHighlightModal.value = true;
      break;
    case 'vpublic':
      tempVisibility.value = 0;
      showVisibilityModal.value = true;
      break;
    case 'vprivate':
      tempVisibility.value = 1;
      showVisibilityModal.value = true;
      break;
    case 'vfollowing':
      tempVisibility.value = 3;
      showVisibilityModal.value = true;
      break;
    default:
      break;
  }
};
const execDelAction = () => {
  deletePost({
    id: post.value.id,
  })
    .then((_res) => {
      window.$message.success(t('post.msg.deleteSuccess'));
      router.replace('/');

      setTimeout(() => {
        storeMain.doRefresh();
      }, 50);
    })
    .catch((_err) => {
      loading.value = false;
    });
};
const execLockAction = () => {
  lockPost({
    id: post.value.id,
  })
    .then((res) => {
      emit('reload', post.value.id);
      if (res.lock_status === 1) {
        window.$message.success(t('post.msg.lockSuccess'));
      } else {
        window.$message.success(t('post.msg.unlockSuccess'));
      }
    })
    .catch((_err) => {
      loading.value = false;
    });
};
const execStickAction = () => {
  stickPost({
    id: post.value.id,
  })
    .then((res) => {
      emit('reload', post.value.id);
      if (res.top_status === 1) {
        window.$message.success(t('post.msg.stickSuccess'));
      } else {
        window.$message.success(t('post.msg.unstickSuccess'));
      }
    })
    .catch((_err) => {
      loading.value = false;
    });
};
const execHighlightAction = () => {
  highlightPost({
    id: post.value.id,
  })
    .then((res) => {
      post.value = {
        ...post.value,
        is_essence: res.highlight_status,
      };
      if (res.highlight_status === 1) {
        window.$message.success(t('post.msg.highlightSuccess'));
      } else {
        window.$message.success(t('post.msg.unhighlightSuccess'));
      }
    })
    .catch((_err) => {
      loading.value = false;
    });
};
const execVisibilityAction = () => {
  visibilityPost({
    id: post.value.id,
    visibility: tempVisibility.value,
  })
    .then((_res) => {
      emit('reload', post.value.id);
      window.$message.success(t('post.msg.visibilitySuccess'));
    })
    .catch((_err) => {
      loading.value = false;
    });
};
const handlePostStar = () => {
  postStar({
    id: post.value.id,
  })
    .then((res) => {
      hasStarred.value = res.status;
      if (res.status) {
        post.value = {
          ...post.value,
          upvote_count: post.value.upvote_count + 1,
        };
      } else {
        post.value = {
          ...post.value,
          upvote_count: post.value.upvote_count - 1,
        };
      }
    })
    .catch((err) => {
      console.log(err);
    });
};
const handlePostCollection = () => {
  postCollection({
    id: post.value.id,
  })
    .then((res) => {
      hasCollected.value = res.status;
      if (res.status) {
        post.value = {
          ...post.value,
          collection_count: post.value.collection_count + 1,
        };
      } else {
        post.value = {
          ...post.value,
          collection_count: post.value.collection_count - 1,
        };
      }
    })
    .catch((err) => {
      console.log(err);
    });
};
const handlePostShare = () => {
  copy(
    `${window.location.origin}/#/post?id=${post.value.id}&share=copy_link&t=${new Date().getTime()}`,
  );
  window.$message.success(t('post.msg.linkCopied'));
};

onMounted(() => {
  if (userInfo.value.id > 0) {
    getPostStar({
      id: post.value.id,
    })
      .then((res) => {
        hasStarred.value = res.status;
      })
      .catch((err) => {
        console.log(err);
      });

    getPostCollection({
      id: post.value.id,
    })
      .then((res) => {
        hasCollected.value = res.status;
      })
      .catch((err) => {
        console.log(err);
      });
  }
});
</script>

<style lang="less">
.detail-item {
    width: 100%;
    padding: 16px;
    box-sizing: border-box;

    background: #f7f9f9;
    .nickname-wrap {
        font-size: 14px;
    }
    .username-wrap {
        font-size: 14px;
        opacity: 0.75;
    }
    .top-tag {
        transform: scale(0.75);
    }
    .options {
        opacity: 0.75;
    }
    .post-text {
        font-size: 16px;
        text-align: justify;
        overflow: hidden;
        white-space: pre-wrap;
        word-break: break-all;
    }
    .post-markdown-wrap {
        // 困住md-editor内部浮层的z-index(最高100001), 防止代码块头部等盖住站内弹窗
        isolation: isolate;

        // 背景透明: 跟随帖子卡片背景(深色模式一致)
        .md-editor.md-editor {
            --md-bk-color: transparent;
        }
        .md-editor-preview-wrapper {
            padding: 0;
        }
        .md-editor-preview {
            font-size: 15px;
        }
    }
    .audit-bar {
        display: flex;
        align-items: center;
        gap: 10px;
        padding: 8px 12px;
        margin-bottom: 10px;
        border-radius: 4px;
        background: rgba(240, 160, 32, 0.08);

        .audit-bar-tip {
            flex: 1;
            font-size: 13px;
            opacity: 0.75;
        }
    }
    .audit-reject-tip {
        font-size: 12px;
        opacity: 0.65;
    }
    .opts-wrap {
        margin-top: 20px;
        .opt-item {
            display: flex;
            align-items: center;
            opacity: 0.7;
            .opt-item-icon {
                margin-right: 10px;
            }
            &.hover {
                cursor: pointer;
            }
        }
    }
    .n-thing {
        .n-thing-avatar-header-wrapper {
            align-items: center;
        }
    }
    .timestamp {
        opacity: 0.75;
        font-size: 12px;
        margin-top: 10px;
    }
}
.dark {
    .detail-item {
        background: #18181c;
    }
}
</style>