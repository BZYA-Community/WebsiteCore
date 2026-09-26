<template>
    <div>
        <main-nav :title="t('user.collection.title')" />

        <n-list class="main-content-wrap" bordered>
            <div v-if="loading && list.length === 0" class="skeleton-wrap">
                <post-skeleton :num="pageSize" />
            </div>
            <div v-else>
                <div class="empty-wrap" v-if="list.length === 0">
                    <n-empty size="large" :description="t('common.noData')" />
                </div>

                <n-list-item v-for="post in list" :key="post.id">
                    <post-item :post="post"
                        :isOwner="userInfo.id == post.user_id"
                        :isMobile="!desktopModelShow"
                        addFollowAction
                        @send-whisper="onSendWhisper"
                        @post-follow-action="postFollowAction" />
                </n-list-item>
            </div>
        </n-list>
        <n-space v-if="totalPage > 0" justify="center">
            <InfiniteLoading class="load-more" :slots="{ complete: t('user.collection.noMore'), error: t('user.collection.loadError') }" @infinite="nextPage">
                <template #spinner>
                    <div class="load-more-wrap">
                        <n-spin :size="14" v-if="!noMore" />
                        <span class="load-more-spinner">{{ noMore ? t('user.collection.noMore') : t('user.collection.loadMore') }}</span>
                    </div>
                </template>
            </InfiniteLoading>
        </n-space>
    </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useI18n } from 'vue-i18n';
import { useStoreMain } from '@/store/main';
import { useStoreUser } from '@/store/user';
import { useRoute } from 'vue-router';
import { useDialog } from 'naive-ui';
import InfiniteLoading from 'v3-infinite-loading';
import { storeToRefs } from 'pinia';
import { Api } from '@/utils/request';
import { useChatJump } from '@/composables/useUserAction';

const { t } = useI18n();
const storeMain = useStoreMain();
const storeUser = useStoreUser();
const { collapsedRight, desktopModelShow } = storeToRefs(storeMain);
const { userInfo } = storeToRefs(storeUser);

const route = useRoute();
const dialog = useDialog();

const loading = ref(false);
const noMore = ref(false);
const list = ref<any[]>([]);
const page = ref(+(route.query.p as any) || 1);
const pageSize = ref(20);
const totalPage = ref(0);

// 私信入口: 跳转消息页会话(原 whisper 弹窗已移除)
const { goWhisper: onSendWhisper } = useChatJump();

function postFollowAction(userId: number, isFollowing: boolean) {
  for (let index in list.value) {
    if (list.value[index].user_id == userId) {
      list.value[index].user.is_following = isFollowing;
    }
  }
}

const loadPosts = () => {
  loading.value = true;
  Api.v1.user.get.collections({
    page: page.value,
    page_size: pageSize.value,
  })
    .then((res) => {
      loading.value = false;
      if (res.list.length === 0) {
        noMore.value = true;
      }
      if (page.value > 1) {
        list.value = list.value.concat(res.list);
      } else {
        list.value = res.list;
        window.scrollTo(0, 0);
      }
      totalPage.value = Math.ceil(res.pager.total_rows / pageSize.value);
    })
    .catch((_err) => {
      loading.value = false;
      if (page.value > 1) {
        page.value--;
      }
    });
};
const nextPage = () => {
  if (page.value < totalPage.value || totalPage.value == 0) {
    noMore.value = false;
    page.value++;
    loadPosts();
  } else {
    noMore.value = true;
  }
};
onMounted(() => {
  loadPosts();
});
</script>

<style lang="less" scoped>
.load-more {
    margin: 20px;

    .load-more-wrap {
        display: flex;
        flex-direction: row;
        justify-content: center;
        align-items: center;
        gap: 14px;

        .load-more-spinner {
            font-size: 14px;
            opacity: 0.65;
        }
    }
}
.dark {
    .main-content-wrap, .empty-wrap, .skeleton-wrap {
        background-color: rgba(16, 16, 20, 0.75);
    }
}
</style>