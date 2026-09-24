<template>
    <div class="post-item" @click="goPostDetail(post.id)">
        <n-thing content-indented>
            <template #avatar>
                <n-avatar round :size="30" :src="post.user.avatar" />
            </template>
            <template #header>
                    <span class="nickname-wrap">
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
                    </span>
                    <span class="username-wrap"> @{{ post.user.username }} </span>
                    <n-tag
                        v-if="post.is_top"
                        class="top-tag"
                        type="warning"
                        size="small"
                        round
                    >
                        置顶
                    </n-tag>
                    <n-tag
                        v-if="post.audit_status === 0"
                        class="top-tag"
                        type="warning"
                        size="small"
                        round
                    >
                        待审核
                    </n-tag>
                    <n-tag
                        v-if="post.audit_status === 2"
                        class="top-tag"
                        type="error"
                        size="small"
                        round
                    >
                        未通过
                    </n-tag>
                    <n-tag
                        v-if="post.visibility == 1"
                        class="top-tag"
                        type="error"
                        size="small"
                        round
                    >
                        私密
                    </n-tag>
                    <n-tag
                        v-if="post.visibility == 2"
                        class="top-tag"
                        type="info"
                        size="small"
                        round
                    >
                        好友可见
                    </n-tag>
                    <div v-if="isMobile">
                        <span class="timestamp-mobile">
                            {{ formatPrettyDate(post.created_on) }} {{ post.ip_loc }}
                        </span>
                    </div>
            </template>
            <template #header-extra>
                <div class="item-header-extra">
                    <span v-if="!isMobile" class="timestamp">
                        {{ post.ip_loc ? post.ip_loc + ' · ' : post.ip_loc }}
                        {{ formatPrettyDate(post.created_on) }}
                    </span>
                    <n-dropdown
                        placement="bottom-end"
                        :trigger="isMobile ? 'click' : 'hover'"
                        size="small"
                        :options="tweetOptions"
                        @select="handleTweetAction"
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
            </template>
            <template #description v-if="post.texts.length > 0 || post.markdowns.length > 0">
                <template v-if="post.texts.length > 0">
                    <div v-if="isMobile" @click="goPostDetail(post.id)">
                        <span v-for="content in post.texts"
                            :key="content.id"
                            class="post-text"
                            @click.stop="doClickText($event, post.id)"
                            v-html="preparePost(content.content, '展开', '收起', profile.tweetMobileEllipsisSize, inFoldStyle)"
                        ></span>
                    </div>
                    <template v-else>
                        <span
                            v-for="content in post.texts"
                            :key="content.id"
                            class="post-text hover"
                            @click.stop="doClickText($event, post.id)"
                            v-html="preparePost(content.content, '展开', '收起', profile.tweetWebEllipsisSize, inFoldStyle)"
                        ></span>
                    </template>
                </template>
                <div
                    v-for="md in post.markdowns"
                    :key="md.id"
                    class="post-markdown"
                    @click.stop="handleMdClick($event, post.id)"
                >
                    <md-preview
                        :model-value="mdExcerptText(md.content)"
                        :theme="editorTheme"
                        no-mermaid
                        no-katex
                    />
                    <span
                        v-if="mdExcerptMore(md.content)"
                        class="hash-link read-full-link"
                        @click.stop="goPostDetail(post.id)"
                    >
                        阅读全文
                    </span>
                </div>
            </template>

            <template #footer>
                <post-attachment 
                    v-if="post.attachments.length > 0"
                    :attachments="post.attachments" />
                <post-attachment
                    v-if="post.charge_attachments.length > 0"
                    :attachments="post.charge_attachments"
                    :price="post.attachment_price"
                />
                <post-image
                    v-if="post.imgs.length > 0"
                    :imgs="post.imgs" />
                <post-video
                    v-if="post.videos.length > 0"
                    :videos="post.videos" />
                <post-link
                    v-if="post.links.length > 0"
                    :links="post.links" />
            </template>
            <template #action>
                <n-space justify="space-between">
                    <div class="opt-item hover" @click.stop="handlePostStar">
                        <n-icon size="18" class="opt-item-icon">
                            <heart-outline />
                        </n-icon>
                        {{ post.upvote_count }}
                    </div>
                    <div class="opt-item hover" @click.stop="goPostDetail(post.id)">
                        <n-icon size="18" class="opt-item-icon">
                            <chatbox-outline />
                        </n-icon>
                        {{ post.comment_count }}
                    </div>
                    <div class="opt-item hover" @click.stop="handlePostCollection">
                        <n-icon size="18" class="opt-item-icon">
                            <bookmark-outline />
                        </n-icon>
                        {{ post.collection_count }}
                    </div>
                </n-space>
            </template>
        </n-thing>
    </div>
</template>

<script setup lang="ts">
import { h, ref, computed } from 'vue';
import { useStoreMain } from '@/store/main';
import { useRouter } from 'vue-router';
import { NIcon, useDialog } from 'naive-ui';
import type { Component } from 'vue';
import type { DropdownOption } from 'naive-ui';
import { formatPrettyDate } from '@/utils/formatTime';
import { preparePost } from '@/utils/content';
import { MdPreview } from 'md-editor-v3';
import { mdTheme, mdExcerpt, prepareMdRender } from '@/utils/markdown';
import { postStar, postCollection } from '@/api/post';
import {
  PaperPlaneOutline,
  HeartOutline,
  BookmarkOutline,
  ChatboxOutline,
  ShareSocialOutline,
  PersonAddOutline,
  PersonRemoveOutline,
  BodyOutline,
  WalkOutline,
} from '@vicons/ionicons5';
import { MoreHorizFilled } from '@vicons/material';
import copy from 'copy-to-clipboard';
import { useStoreProfile } from '@/store/profile';
import { storeToRefs } from 'pinia';
import { Api } from '@/utils/request';
import UserAction, { canWhisperUser } from '@/composables/useUserAction';
import { usePostContent } from '@/composables/usePostContent';

const router = useRouter();

const storeMain = useStoreMain();
const storeProfile = useStoreProfile();
const { theme } = storeToRefs(storeMain);
const { profile } = storeToRefs(storeProfile);
const editorTheme = computed(() => mdTheme(theme.value));

const dialog = useDialog();

const inFoldStyle = ref<boolean>(true);
const props = withDefaults(defineProps<{
    post: Item.PostProps;
    isOwner: boolean;
    addFriendAction?: boolean;
    addFollowAction?: boolean;
    isMobile?: boolean;
}>(), {
	addFollowAction: false,
	addFriendAction: false,
    isMobile: false,
});

const emit = defineEmits<{
  (e: 'send-whisper', user: Item.UserInfo): void;
  (e: 'handle-follow-action', user: Item.PostProps): void;
  (e: 'handle-friend-action', user: Item.PostProps): void;
  (e: 'post-follow-action', user_id: number, is_following: boolean): void;
}>();

const renderIcon = (icon: Component) => {
  return () => {
    return h(NIcon, null, {
      default: () => h(icon),
    });
  };
};

const tweetOptions = computed(() => {
  let options: DropdownOption[] = [];
  // 私信入口: 道友仅对高级身份可见(后端仍强制校验)
  if (!props.isOwner && canWhisperUser(props.post.user)) {
    options.push({
      label: '私信 @' + props.post.user.username,
      key: 'whisper',
      icon: renderIcon(PaperPlaneOutline),
    });
  }
  if (!props.isOwner && props.addFollowAction) {
    if (props.post.user.is_following) {
      options.push({
        label: '取消关注 @' + props.post.user.username,
        key: 'unfollow',
        icon: renderIcon(WalkOutline),
      });
    } else {
      options.push({
        label: '关注 @' + props.post.user.username,
        key: 'follow',
        icon: renderIcon(BodyOutline),
      });
    }
  }
  if (!props.isOwner && props.addFriendAction) {
    if (props.post.user.is_friend) {
      options.push({
        label: '删除好友 @' + props.post.user.username,
        key: 'delete',
        icon: renderIcon(PersonRemoveOutline),
      });
    } else {
      options.push({
        label: '添加朋友 @' + props.post.user.username,
        key: 'requesting',
        icon: renderIcon(PersonAddOutline),
      });
    }
  }
  options.push({
    label: '复制链接',
    key: 'copyTweetLink',
    icon: renderIcon(ShareSocialOutline),
  });
  return options;
});

const handleTweetAction = async (
  item:
    | 'copyTweetLink'
    | 'whisper'
    | 'follow'
    | 'unfollow'
    | 'delete'
    | 'requesting',
) => {
  switch (item) {
    case 'copyTweetLink':
      copy(
        `${window.location.origin}/#/post?id=${post.value.id}&share=copy_link&t=${new Date().getTime()}`,
      );
      window.$message.success('链接已复制到剪贴板');
      break;
    case 'whisper':
      emit('send-whisper', props.post.user);
      break;
    case 'delete':
    case 'requesting':
      emit('handle-friend-action', props.post);
      break;
    case 'follow':
    case 'unfollow':
      UserAction.followAction(dialog, props.post.user.id, props.post.user.username, props.post.user.is_following)
        .then(_action => {
          emit('post-follow-action', props.post.user.id, _action);
        })
		  emit('handle-follow-action', props.post);
      break;
    default:
    	break;
  }
};

// 使用 usePostContent composable
const post = usePostContent(props.post);
const handlePostStar = () => {
  postStar({
    id: post.value.id,
  })
    .then((res) => {
      if (res.status) {
        post.value = {
          ...post.value,
          upvote_count: post.value.upvote_count + 1,
        };
      } else {
        post.value = {
          ...post.value,
          upvote_count:
            post.value.upvote_count > 0 ? post.value.upvote_count - 1 : 0,
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
      if (res.status) {
        post.value = {
          ...post.value,
          collection_count: post.value.collection_count + 1,
        };
      } else {
        post.value = {
          ...post.value,
          collection_count:
            post.value.collection_count > 0
              ? post.value.collection_count - 1
              : 0,
        };
      }
    })
    .catch((err) => {
      console.log(err);
    });
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
  const detail = (e.target as any).dataset.detail;
  if (detail && detail !== 'post') {
    const d = detail.split(':');
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
    }
  } else if (detail && detail === 'post') {
    inFoldStyle.value = !inFoldStyle.value;
  } else {
    goPostDetail(id);
  }
};

// Markdown节选文本(前5行 话题链接化)
const mdExcerptText = (mdc: string) => prepareMdRender(mdExcerpt(mdc).text);
// 是否超出节选行数(需要展示阅读全文)
const mdExcerptMore = (mdc: string) => mdExcerpt(mdc).truncated;
// Markdown渲染区点击委托: 话题/站外链接拦截 其余跳详情
const handleMdClick = (e: MouseEvent, id: number) => {
  const anchor = (e.target as HTMLElement).closest('a');
  if (!anchor) {
    goPostDetail(id);
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
    return;
  }
  if (href === '#') {
    e.preventDefault();
  }
};
</script>

<style lang="less">
.post-item {
    width: 100%;
    padding: 16px;
    box-sizing: border-box;

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
    .timestamp-mobile {
        margin-top: 2px;
        opacity: 0.75;
        font-size: 11px;
    }
    .item-header-extra {
        display: flex;
        align-items: center;
        opacity: 0.75;
        .timestamp {
            font-size: 12px;
        }
    }
    .post-text {
        text-align: justify;
        overflow: hidden;
        white-space: pre-wrap;
        word-break: break-all;
    }

    .post-markdown {
        width: 100%;
        cursor: pointer;
        // 困住md-editor内部浮层的z-index(最高100001), 防止代码块头部等盖住站内弹窗
        isolation: isolate;

        .read-full-link {
            display: inline-block;
            margin-top: 4px;
            cursor: pointer;
        }

        // 背景透明: 跟随帖子卡片背景(鼠标悬浮加深/深色模式)
        .md-editor.md-editor {
            --md-bk-color: transparent;
        }

        .md-editor-preview-wrapper {
            padding: 0 4px 0 12px;
        }

        .md-editor-preview {
            font-size: 15px;
        }
    }

    .opt-item {
        display: flex;
        align-items: center;
        opacity: 0.7;
        .opt-item-icon {
            margin-right: 10px;
        }
    }
    
    &:hover {
        background: #f7f9f9;
    }
    
    &.hover {
        cursor: pointer;
    }

    .n-thing-avatar {
        margin-top: 0;
    }
    .n-thing-header {
        line-height: 16px;
        margin-bottom: 8px !important;
    }
}
.dark {
    .post-item {
        &:hover {
            background: #18181c;
        }
        background-color: rgba(16, 16, 20, 0.75);
    }
}
</style>