<template>
    <div class="comment-item" :id="`comment-${comment.id}`" :class="{ 'comment-pending': comment.audit_status === AuditStatusEnum.PENDING }">
        <n-thing content-indented>
            <template #avatar>
                <n-avatar round :size="30" :src="comment.user.avatar" />
            </template>
            <template #header>
                <span class="nickname-wrap">
                    <router-link
                        @click.stop
                        class="username-link"
                        :to="{
                            name: 'user',
                            query: { s: comment.user.username },
                        }"
                    >
                        {{ comment.user.nickname }}
                    </router-link>
                </span>
                <span class="username-wrap">
                    @{{ comment.user.username }}
                </span>
                <n-tag
                    v-if="comment.audit_status === AuditStatusEnum.PENDING"
                    class="top-tag"
                    type="warning"
                    size="small"
                    round
                >
                    {{ t('course.comment.auditing') }}
                </n-tag>
                <n-tag
                    v-else-if="comment.audit_status === AuditStatusEnum.REJECTED"
                    class="top-tag"
                    type="error"
                    size="small"
                    round
                >
                    {{ t('course.comment.auditRejected') }}
                </n-tag>
            </template>
            <template #header-extra>
                <div class="opt-wrap">
                    <span class="timestamp">
                        {{ comment.ip_loc }}
                    </span>
                    <n-popconfirm
                        v-if="
                            userInfo.is_admin ||
                            userInfo.id === comment.user.id
                        "
                        :negative-text="t('common.cancel')"
                        :positive-text="t('common.confirm')"
                        @positive-click="execDelAction"
                    >
                        <template #trigger>
                            <n-button
                                quaternary
                                circle
                                size="tiny"
                                class="action-btn"
                            >
                                <template #icon>
                                    <n-icon>
                                        <trash />
                                    </n-icon>
                                </template>
                            </n-button>
                        </template>
                        {{ t('course.comment.deleteConfirm') }}
                    </n-popconfirm>
                </div>
            </template>
            <template #description v-if="comment.texts.length > 0">
                <span
                    v-for="content in comment.texts"
                    :key="content.id"
                    class="comment-text"
                    @click.stop="doClickText($event)"
                    v-html="parsePostTag(content.content).content"
                ></span>
            </template>

            <template #footer>
                <post-image
                    v-if="comment.imgs.length > 0"
                    :imgs="comment.imgs" />
                  <!-- 回复编辑器 -->
                  <course-compose-reply
                    ref="replyComposeRef"
                    :comment="comment"
                    :at-userid="replyAtUserID"
                    :at-username="replyAtUsername"
                    @reload="reload"
                    @reset="resetReply"
                />
                <!-- 回复列表 -->
                <div class="reply-wrap">
                    <course-reply-item
                        v-for="reply in comment.replies"
                        :key="reply.id"
                        :reply="reply"
                        @focusReply="focusReply"
                        @reload="reload"
                    />
                </div>
            </template>
        </n-thing>
    </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';
import { useStoreUser } from '@/store/user';
import { useRouter } from 'vue-router';
import { parsePostTag } from '@/utils/content';
import { Trash } from '@vicons/tabler';
import { deleteCourseComment, type CourseComment, type CourseReply, type CourseCommentContent } from '@/api/course';
import { AuditStatusEnum } from '@/utils/IEnum';
import { storeToRefs } from 'pinia';
import { useI18n } from 'vue-i18n';

const { t } = useI18n();

const router = useRouter();
const replyAtUserID = ref(0);
const replyAtUsername = ref('');
const replyComposeRef = ref();

const storeUser = useStoreUser();
const { userInfo } = storeToRefs(storeUser);

const emit = defineEmits<{
  (e: 'reload'): void;
}>();
const props = withDefaults(
  defineProps<{
    comment: CourseComment;
  }>(),
  {},
);

type CommentView = CourseComment & {
  texts: CourseCommentContent[];
  imgs: { id: number; content: string }[];
};

const comment = computed<CommentView>(() => {
  let view = Object.assign(
    {
      texts: [],
      imgs: [],
    },
    props.comment,
  ) as CommentView;
  (props.comment.contents || []).map((content) => {
    if (+content.type === 1 || +content.type === 2) {
      view.texts.push(content);
    }
    if (+content.type === 3) {
      view.imgs.push({ id: content.id, content: content.content });
    }
  });
  return view;
});

const doClickText = (e: MouseEvent) => {
  let _target = e.target as any;
  if (_target.dataset.detail) {
    const d = _target.dataset.detail.split(':');
    if (d.length === 2) {
      if (d[0] === 'tag') {
        window.$message.warning(t('course.comment.invalidTag'));
      } else {
        router.push({
          name: 'user',
          query: {
            s: d[1],
          },
        });
      }
    }
  }
};

const focusReply = (reply: CourseReply) => {
  replyAtUserID.value = reply.user_id;
  replyAtUsername.value = reply.user?.username || '';
  replyComposeRef.value?.switchReply(true);
};
const reload = () => {
  emit('reload');
};
const resetReply = () => {
  replyAtUserID.value = 0;
  replyAtUsername.value = '';
};

const execDelAction = () => {
  deleteCourseComment({
    id: comment.value.id,
  })
    .then(() => {
      window.$message.success(t('course.deleteSuccess'));
      setTimeout(() => {
        reload();
      }, 50);
    })
    .catch(() => {});
};
</script>

<style lang="less" scoped>
.comment-item {
    width: 100%;
    padding: 16px;
    box-sizing: border-box;

    &.comment-pending {
        opacity: 0.75;
    }

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
    .opt-wrap {
        display: flex;
        align-items: center;
        .timestamp {
            opacity: 0.75;
            font-size: 12px;
        }
        .action-btn {
            margin-left: 4px;
        }
    }
    .comment-text {
        display: block;
        text-align: justify;
        overflow: hidden;
        white-space: pre-wrap;
        word-break: break-all;
    }
}

.reply-wrap {
    margin-top: 10px;
    border-radius: 5px;
    background: #fafafc;
}

.dark {
    .reply-wrap {
        background: #18181c;
    }
    .comment-item {
        background-color: rgba(16, 16, 20, 0.75);
    }
}
</style>
