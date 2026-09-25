<template>
    <div class="reply-compose-wrap">
        <div class="reply-switch">
            <span class="time-item">
                {{ formatPrettyTime(comment.created_on) }}
            </span>
            <div class="actions">
                <span class="show reply-btn" v-if="userLogined && !showReply" @click="switchReply(true)">
                    回复
                </span>
                <span class="hide reply-btn" v-if="userLogined && showReply" @click="switchReply(false)">
                    取消
                </span>
            </div>
        </div>

        <div class="reply-input-wrap" v-if="showReply">
            <n-input-group>
                <n-input ref="inputInstRef" size="small" :placeholder="
                    props.atUsername
                        ? '@' + props.atUsername
                        : '请输入回复内容..'
                " :maxlength="defaultReplyMaxLength" v-model:value="replyContent" show-count clearable />
                <n-button type="primary" size="small" ghost :loading="submitting" @click="submitReply">
                    回复
                </n-button>
            </n-input-group>
        </div>
    </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { useStoreUser } from '@/store/user';
import { formatPrettyTime } from '@/utils/formatTime';
import { createCourseCommentReply, type CourseComment } from '@/api/course';
import { InputInst } from 'naive-ui';
import { storeToRefs } from 'pinia';

const props = withDefaults(
  defineProps<{
    comment: CourseComment;
    atUserid: number;
    atUsername: string;
  }>(),
  {
    atUserid: 0,
    atUsername: '',
  },
);

const storeUser = useStoreUser();
const { userLogined } = storeToRefs(storeUser);

const emit = defineEmits<{
  (e: 'reload'): void;
  (e: 'reset'): void;
}>();
const inputInstRef = ref<InputInst>();
const showReply = ref(false);
const replyContent = ref('');
const submitting = ref(false);

const defaultReplyMaxLength = Number(
  import.meta.env.VITE_DEFAULT_REPLY_MAX_LENGTH,
);

const switchReply = (status: boolean) => {
  showReply.value = status;

  if (status) {
    setTimeout(() => {
      inputInstRef.value?.focus();
    }, 10);
  } else {
    submitting.value = false;
    replyContent.value = '';
    emit('reset');
  }
};
const submitReply = () => {
  if (replyContent.value.trim().length === 0) {
    window.$message.warning('请输入回复内容');
    return;
  }
  submitting.value = true;
  createCourseCommentReply({
    comment_id: props.comment.id,
    at_user_id: props.atUserid,
    content: replyContent.value,
  })
    .then((res) => {
      switchReply(false);
      if (res.audit_status === 0) {
        window.$message.success('回复已提交，审核通过后对外可见');
      } else {
        window.$message.success('评论成功');
      }
      emit('reload');
    })
    .catch(() => {
      submitting.value = false;
    });
};
defineExpose({ switchReply });
</script>

<style lang="less" scoped>
.reply-compose-wrap {
    .reply-switch {
        display: flex;
        align-items: center;
        justify-content: space-between;
        text-align: right;
        font-size: 12px;

        .actions {
            display: flex;
            align-items: center;
            text-align: right;
            font-size: 12px;
            margin: 10px 0;
        }

        .time-item {
            font-size: 12px;
            opacity: 0.65;
            margin-right: 18px;
        }

        .reply-btn {
            margin-left: 18px;
        }

        .show {
            color: #18a058;
            cursor: pointer;
            opacity: 0.75;
        }

        .hide {
            opacity: 0.75;
            cursor: pointer;
        }
    }
}

.dark {
    .reply-compose-wrap {
        background-color: rgba(16, 16, 20, 0.75);
    }
}
</style>
