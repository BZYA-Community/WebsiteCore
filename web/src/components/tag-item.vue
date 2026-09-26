<template>
    <div v-if="(!checkFollowing && !checkPin) || (checkFollowing && tag.is_following === 1) || (checkPin && tag.is_following === 1 && tag.is_pin === 1)" class="tag-item">
        <n-thing>
            <template #header>
                <n-tag
                        type="success"
                        size="large"
                        round
                        :key="tag.id"
                    >
                        <router-link
                            class="hash-link"
                            :to="{
                                name: 'home',
                                query: {
                                    q: tag.tag,
                                    t: 'tag',
                                },
                            }"
                        >
                            #{{ tag.tag }}
                        </router-link>
                        <span v-if="!showAction" class="tag-quote">({{ tag.quote_num }})</span>
                        <span v-if="showAction" class="tag-quote tag-follow">({{ tag.quote_num }})</span>
                        <template #avatar>
                            <n-avatar :src="tagUserAvatar" />
                        </template>
                    </n-tag>
            </template>
            <template #header-extra>
                <div 
                    v-if="showAction" 
                    class="options">
                    <n-dropdown
                        placement="bottom-end"
                        trigger="click"
                        size="small"
                        :options="tagOptions"
                        @select="handleTagAction"
                    >
                        <n-button type="success" quaternary circle block>
                            <template #icon>
                                <n-icon>
                                    <more-vert-outlined />
                                </n-icon>
                            </template>
                        </n-button>
                    </n-dropdown>
                </div>
            </template>
        </n-thing>
    </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import { MoreVertOutlined } from '@vicons/material';
import type { DropdownOption } from 'naive-ui';
import { pinTopic, stickTopic, followTopic, unfollowTopic } from '@/api/post';
import defaultUserAvatar from '@/assets/img/logo.png';

const { t } = useI18n();

const props = withDefaults(
  defineProps<{
    tag: Item.TagProps;
    showAction: boolean;
    checkFollowing: boolean;
    checkPin: boolean;
  }>(),
  {},
);

// 关注/钉住/置顶状态由父组件持有, 通过 update 事件回写
const emit = defineEmits<{
  (e: 'update', patch: Partial<Pick<Item.TagProps, 'is_following' | 'is_pin' | 'is_top'>>): void;
}>();

const tagUserAvatar = computed(() => {
  if (props.tag.user) {
    return props.tag.user.avatar;
  } else {
    return defaultUserAvatar;
  }
});

const tagOptions = computed(() => {
  let options: DropdownOption[] = [];
  if (props.tag.is_following === 0) {
    options.push({
      label: t('post.action.follow'),
      key: 'follow',
    });
  } else {
    if (props.tag.is_pin === 0) {
      options.push({
        label: t('post.action.pin'),
        key: 'pin',
      });
    } else {
      options.push({
        label: t('post.action.unpin'),
        key: 'unpin',
      });
    }
    if (props.tag.is_top === 0) {
      options.push({
        label: t('post.action.stick'),
        key: 'stick',
      });
    } else {
      options.push({
        label: t('post.action.unstick'),
        key: 'unstick',
      });
    }
    options.push({
      label: t('post.action.unfollow'),
      key: 'unfollow',
    });
  }
  return options;
});

const handleTagAction = (
  item: 'follow' | 'unfollow' | 'pin' | 'unpin' | 'stick' | 'unstick',
) => {
  switch (item) {
    case 'follow':
      followTopic({
        topic_id: props.tag.id,
      })
        .then((_res) => {
          emit('update', { is_following: 1 });
          window.$message.success(t('post.msg.followSuccess'));
        })
        .catch((err) => {
          console.log(err);
        });
      break;
    case 'unfollow':
      unfollowTopic({
        topic_id: props.tag.id,
      })
        .then((_res) => {
          emit('update', { is_following: 0 });
          window.$message.success(t('post.msg.unfollowSuccess'));
        })
        .catch((err) => {
          console.log(err);
        });
      break;
    case 'pin':
      pinTopic({
        topic_id: props.tag.id,
      })
        .then((_res) => {
          emit('update', { is_pin: 1 });
          window.$message.success(t('post.msg.pinSuccess'));
        })
        .catch((err) => {
          console.log(err);
        });
      break;
    case 'unpin':
      pinTopic({
        topic_id: props.tag.id,
      })
        .then((_res) => {
          emit('update', { is_pin: 0 });
          window.$message.success(t('post.msg.unpinSuccess'));
        })
        .catch((err) => {
          console.log(err);
        });
      break;
    case 'stick':
      stickTopic({
        topic_id: props.tag.id,
      })
        .then((res) => {
          emit('update', { is_top: res.top_status });
          window.$message.success(t('post.msg.stickSuccess'));
        })
        .catch((err) => {
          console.log(err);
        });
      break;
    case 'unstick':
      stickTopic({
        topic_id: props.tag.id,
      })
        .then((res) => {
          emit('update', { is_top: res.top_status });
          window.$message.success(t('post.msg.unstickSuccess'));
        })
        .catch((err) => {
          console.log(err);
        });
      break;
    default:
      break;
  }
};
</script>

<style lang="less">
.tag-item {
    .tag-quote {
        margin-left: 12px;
        font-size: 14px;
        opacity: 0.75;
    }
    .tag-follow {
        margin-right: 22px;
    }
    .options {
        margin-left: -32px;
        margin-bottom: 4px;
        opacity: 0.55;
    }
    .n-thing {
        .n-thing-header {
            margin-bottom: 0px;
        }
        .n-thing-avatar-header-wrapper {
            align-items: center;
        }
    }
}
</style>