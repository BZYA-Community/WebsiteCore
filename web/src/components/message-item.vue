<template>
  <div class="message-item" :class="{ unread: isNotWhisperSender && message.is_read === 0 }" @click="handleReadMessage(message)">
    <n-thing content-indented>
      <template #avatar>
        <n-avatar round :size="30" :src="message.type == 4 && message.sender_user_id == userInfo.id
            ? message.receiver_user.avatar
            : (message.sender_user.id > 0
              ? message.sender_user.avatar
              : defaultavatar
            )
          " />
      </template>
      <template #header>
        <div class="sender-wrap">
          <span class="nickname" v-if="(message.type != 4 && message.sender_user.id > 0) || isWhisperReceiver">
            <router-link @click.stop class="username-link" :to="{
              name: 'user',
              query: {
                s: message.sender_user.username,
              },
            }">
              {{ message.sender_user.nickname }}
            </router-link>
            <span v-if="desktopModelShow" class="username">
              @{{ message.sender_user.username }}
            </span>
          </span>
          <span class="nickname" v-else-if="isWhisperSender">
            <router-link @click.stop class="username-link" :to="{
              name: 'user',
              query: {
                s: message.receiver_user.username,
              },
            }">
              {{ message.receiver_user.nickname }}
            </router-link>
            <span v-if="desktopModelShow" class="username">
              @{{ message.receiver_user.username }}
            </span>
          </span>
          <span class="nickname" v-else> {{ t('message.system') }} </span>
          <n-tag v-if="isWhisperSender" class="top-tag" type="info" size="small" round>
            {{ t('message.whisperSent') }}
            <template #icon>
              <n-icon :component="CheckmarkCircle" />
            </template>
          </n-tag>
          <n-tag v-if="message.type == 4 && message.receiver_user_id == userInfo.id" class="top-tag" type="warning" size="small" round>
            {{ t('message.whisperReceived') }}
            <template #icon>
              <n-icon :component="CheckmarkCircle" />
            </template>
          </n-tag>
        </div>
      </template>
      <template #header-extra>
        <span class="timestamp">
          <n-badge v-if="isNotWhisperSender && message.is_read === 0" dot processing />
          <span class="timestamp-txt">
            {{ formatRelativeTime(message.created_on) }}
          </span>
          <n-dropdown placement="bottom-end" trigger="click" size="small" :options="actionOpts" @select="handleAction">
            <n-button quaternary circle>
              <template #icon>
                <n-icon>
                  <more-horiz-filled />
                </n-icon>
              </template>
            </n-button>
          </n-dropdown>
        </span>
      </template>
      <template #description>
        <n-alert :show-icon="false" class="brief-wrap" :type="!isNotWhisperSender || message.is_read > 0 ? 'default' : 'success'">
          <div v-if="message.type != 4" class="brief-content">
            {{ message.brief }}
            <span v-if="message.type === 1 || message.type === 2 || message.type === 3" @click.stop="viewDetail(message)" class="hash-link view-link">
              <n-icon>
                <share-outline />
              </n-icon> {{ t('message.viewDetail') }}
            </span>
          </div>

          <div v-if="message.type === 99" class="whisper-content-wrap system-content-wrap">
            {{ message.content }}
          </div>

          <div v-if="message.type === 4" class="whisper-content-wrap">
            {{ message.content }}
          </div>

        </n-alert>
      </template>
    </n-thing>
  </div>
</template>

<script setup lang="ts">
import { h, computed } from 'vue';
import type { Component } from 'vue';
import { useI18n } from 'vue-i18n';
import { NIcon, useDialog, DropdownOption } from 'naive-ui';
import { useStoreMain } from '@/store/main';
import { useStoreUser } from '@/store/user';
import { useRouter } from 'vue-router';
import {
  ShareOutline,
  PaperPlaneOutline,
  CheckmarkCircle,
  BodyOutline,
  WalkOutline,
} from '@vicons/ionicons5';
import { formatRelativeTime } from '@/utils/formatTime';
import { MoreHorizFilled } from '@vicons/material';
import { storeToRefs } from 'pinia';
import { Api } from '@/utils/request';
import UserAction, { canWhisperUser } from '@/composables/useUserAction';

const { t } = useI18n();

const defaultavatar =
  'https://paopao-demo.vercel.app/avatar/default/admin.png';

const router = useRouter();

const storeMain = useStoreMain();
const storeUser = useStoreUser();
const { desktopModelShow } = storeToRefs(storeMain);
const { userInfo } = storeToRefs(storeUser);

const dialog = useDialog();
const props = withDefaults(
  defineProps<{
    message: Item.MessageProps;
  }>(),
  {},
);

const renderIcon = (icon: Component) => {
  return () => {
    return h(NIcon, null, {
      default: () => h(icon),
    });
  };
};

const actionOpts = computed(() => {
  let user =
    props.message.type == 4 &&
      props.message.sender_user_id == userInfo.value.id
      ? props.message.receiver_user
      : props.message.sender_user;
  let options: DropdownOption[] = [];
  // 私信入口: 道友仅对高级身份可见(后端仍强制校验)
  if (canWhisperUser(user)) {
    options.push({
      label: t('message.whisperAt', { username: user.username }),
      key: 'whisper',
      icon: renderIcon(PaperPlaneOutline),
    });
  }
  if (userInfo.value.id != user.id) {
    if (user.is_following) {
      options.push({
        label: t('message.unfollowAt', { username: user.username }),
        key: 'unfollow',
        icon: renderIcon(WalkOutline),
      });
    } else {
      options.push({
        label: t('message.followAt', { username: user.username }),
        key: 'follow',
        icon: renderIcon(BodyOutline),
      });
    }
  }
  return options;
});

const emit = defineEmits<{
  (e: 'send-whisper', user: Item.UserInfo): void;
  (e: 'reload'): void;
}>();

const onHandleFollowAction = (message: Item.MessageProps) => {
  let user =
    message.type == 4 && message.sender_user_id == userInfo.value.id
      ? message.receiver_user
      : message.sender_user;
  UserAction.followAction(dialog, user.id, user.username, user.is_following)
    .then(_action => {
      user.is_following = _action;
      // TODO: 这里暴力处理，简单重新加载，更好的做法是遍历所有message，如果是对应user就更新到新状态
      setTimeout(() => {
        emit('reload');
      }, 50);
    })
    .catch(err => {
      console.log(err);
    });
};

const handleAction = (item: 'whisper' | 'follow' | 'unfollow') => {
  switch (item) {
    case 'whisper':
      const message = props.message;
      if (message.type != 99) {
        let user =
          message.type == 4 && message.sender_user_id == userInfo.value.id
            ? message.receiver_user
            : message.sender_user;
        emit('send-whisper', user);
      }
      break;
    case 'follow':
    case 'unfollow':
      onHandleFollowAction(props.message);
      break;
    default:
      break;
  }
};

const isNotWhisperSender = computed(() => {
  return (
    props.message.type !== 4 ||
    props.message.sender_user_id !== userInfo.value.id
  );
});

const isWhisperReceiver = computed(() => {
  return (
    props.message.type == 4 &&
    props.message.receiver_user_id == userInfo.value.id
  );
});

const isWhisperSender = computed(() => {
  return (
    props.message.type == 4 &&
    props.message.sender_user_id == userInfo.value.id
  );
});

const viewDetail = (message: Item.MessageProps) => {
  handleReadMessage(message);
  if (message.type === 1 || message.type === 2 || message.type === 3) {
    if (message.post && message.post.id > 0) {
      router.push({
        name: 'post',
        query: {
          id: message.post_id,
        },
      });
    } else {
      window.$message.error(t('message.postDeleted'));
    }
  }
};

const handleReadMessage = (message: Item.MessageProps) => {
  if (props.message.receiver_user_id != userInfo.value.id) {
    return;
  }
  if (message.is_read === 0) {
    Api.v1.user.message.post.read({
      id: message.id,
    })
      .then((_res) => {
        message.is_read = 1;
      })
      .catch((err) => {
        console.log(err);
      });
  }
};
</script>

<style lang="less" scoped>
.message-item {
  padding: 16px;

  &.unread {
    background: #fcfffc;
  }

  .sender-wrap {
    display: flex;
    align-items: center;

    .top-tag {
      transform: scale(0.75);
    }

    .username {
      opacity: 0.75;
      font-size: 14px;
    }
  }

  .timestamp {
    opacity: 0.75;
    font-size: 12px;
    display: flex;
    align-items: center;

    .timestamp-txt {
      margin-left: 6px;
    }
  }

  .brief-wrap {
    margin-top: 10px;

    .brief-content {
      display: flex;
      width: 100%;
    }

    .whisper-content-wrap {
      display: flex;
      width: 100%;
    }

  }

  .view-link {
    margin-left: 8px;
    display: flex;
    align-items: center;
  }

  .status-info {
    margin-left: 8px;
    align-items: center;
  }
}

.dark {
  .message-item {
    &.unread {
      background: #0f180b;
    }

    .brief-wrap {
      background-color: #18181c;
    }

    background-color: rgba(16, 16, 20, 0.75);
  }
}
</style>