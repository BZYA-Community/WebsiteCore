<template>
    <div class="reply-item" :id="`reply-${props.reply.id}`" :class="{ 'reply-pending': props.reply.audit_status === AuditStatusEnum.PENDING }">
        <div class="header-wrap">
            <div class="username">
                <router-link class="user-link" :to="{
                    name: 'user',
                    query: { s: props.reply.user.username },
                }">
                    {{ props.reply.user.username }}
                </router-link>
                <span class="reply-name">
                    {{ props.reply.at_user_id > 0 ? '回复' : ':' }}
                </span>

                <router-link class="user-link" :to="{
                    name: 'user',
                    query: { s: props.reply.at_user.username },
                }" v-if="props.reply.at_user_id > 0">
                    {{ props.reply.at_user.username }}
                </router-link>
                <n-tag
                    v-if="props.reply.audit_status === AuditStatusEnum.PENDING"
                    class="audit-tag"
                    type="warning"
                    size="small"
                    round
                >
                    审核中
                </n-tag>
                <n-tag
                    v-else-if="props.reply.audit_status === AuditStatusEnum.REJECTED"
                    class="audit-tag"
                    type="error"
                    size="small"
                    round
                >
                    未通过审核
                </n-tag>
            </div>
            <div class="timestamp">
                {{ props.reply.ip_loc }}
                <n-popconfirm v-if="
                    userInfo.is_admin ||
                    userInfo.id === props.reply.user.id
                " negative-text="取消" positive-text="确认" @positive-click="execDelAction">
                    <template #trigger>
                        <n-button quaternary circle size="tiny" class="del-btn">
                            <template #icon>
                                <n-icon>
                                    <trash />
                                </n-icon>
                            </template>
                        </n-button>
                    </template>
                    是否删除这条回复？
                </n-popconfirm>
            </div>
        </div>

        <div class="base-wrap">
            <div class="content">
                <n-ellipsis expand-trigger="click" line-clamp="5" :tooltip="false">
                    {{ props.reply.content }}
                </n-ellipsis>
            </div>
            <div class="reply-switch">
                <span class="time-item">
                    {{ formatPrettyTime(props.reply.created_on) }}
                </span>

                <div class="actions">
                    <span v-if="userLogined" class="show opacity-item reply-btn" @click="focusReply"> 回复 </span>
                </div>
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { Trash } from '@vicons/tabler';
import { formatPrettyTime } from '@/utils/formatTime';
import { deleteCourseCommentReply, type CourseReply } from '@/api/course';
import { AuditStatusEnum } from '@/utils/IEnum';
import { useStoreUser } from '@/store/user';
import { storeToRefs } from 'pinia';

const props = withDefaults(
  defineProps<{
    reply: CourseReply;
  }>(),
  {},
);

const storeUser = useStoreUser();
const { userInfo, userLogined } = storeToRefs(storeUser);

const emit = defineEmits<{
  (e: 'focusReply', reply: CourseReply): void;
  (e: 'reload'): void;
}>();

const focusReply = () => {
  emit('focusReply', props.reply);
};
const execDelAction = () => {
  deleteCourseCommentReply({
    id: props.reply.id,
  })
    .then(() => {
      window.$message.success('删除成功');

      setTimeout(() => {
        emit('reload');
      }, 50);
    })
    .catch((err) => {
      console.log(err);
    });
};
</script>


<style lang="less" scoped>
.reply-item {
    display: flex;
    flex-direction: column;
    font-size: 12px;
    padding: 8px;
    border-bottom: 1px solid #f3f3f3;

    &.reply-pending {
        opacity: 0.75;
    }

    .header-wrap {
        display: flex;
        align-items: center;
        justify-content: space-between;

        .username {
            max-width: 50%;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;

            .reply-name {
                margin: 0 3px;
                opacity: 0.75;
            }

            .audit-tag {
                transform: scale(0.75);
                margin-left: 2px;
            }
        }

        .timestamp {
            opacity: 0.75;
            text-align: right;
            max-width: 50%;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
        }
    }

    .base-wrap {
        display: block;

        .content {
            width: calc(100%);
            margin-top: 4px;
            font-size: 12px;
            text-align: justify;
            line-height: 2;
        }

        .reply-switch {
            display: flex;
            align-items: center;
            justify-content: space-between;
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
                opacity: 0.75;
                margin-right: 18px;
            }

            .opacity-item {
                opacity: 0.75;
            }

            .reply-btn {
                margin-left: 18px;
             }

            .show {
                color: #18a058;
                cursor: pointer;
            }
        }
    }
}

.dark {
    .reply-item {
        border-bottom: 1px solid #262628;
        background-color: rgba(16, 16, 20, 0.75);
    }
}
</style>
