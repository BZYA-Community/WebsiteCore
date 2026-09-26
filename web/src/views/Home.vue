<template>
    <div>
        <main-nav :title="title" />

        <n-list class="main-content-wrap" bordered>
            <n-list-item>
                <!-- 发布器 -->
                <compose @post-success="onPostSuccess" />
            </n-list-item>

            <n-list-item v-if="showTrendsBar" >
            <SlideBar :key="slideBarKey" v-model="slideBarList" :wheel-blocks="wheelBlocks" :init-blocks="initBlocks" @click="handleBarClick" tag="div" sub-tag="div">
                <template #default="data">
                    <div class="slide-bar-item">
                        <n-badge value="1" :offset="[-4, 48]" dot :show="data.slotData.show">
                            <n-avatar
                                round
                                :size="48"
                                :src="data.slotData.avatar"
                                class="slide-bar-item-avatar"
                            />
                        </n-badge>
                        <div class="slide-bar-item-title slide-bar-user-link">
                            <n-ellipsis :line-clamp="2">
                                {{ data.slotData.title }}
                            </n-ellipsis>
                        </div>
                    </div>
                </template>
            </SlideBar>
            </n-list-item>
            <div  class="style-wrap" v-else-if="showTrendsTag">
            <n-space >
                <n-button
                    v-for="btn in filterButtons"
                    :key="btn.key"
                    size="small"
                    :type="newestTweetsStyle === btn.key ? 'success' : undefined"
                    :bordered="false"
                    @click="onFilterClick(btn.key, btn.index)"
                    class="style-item"
                    secondary
                    round
                >
                    {{ btn.label }}
                </n-button>
            </n-space>
            </div>
            <div v-if="loading && list.length === 0" class="skeleton-wrap">
                <post-skeleton :num="pageSize" />
            </div>

            <div>
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
            <InfiniteLoading class="load-more" :slots="{ complete: t('post.home.noMoreBubbles'), error: t('post.home.loadError') }" @infinite="handleNextPage">
                <template #spinner>
                    <div class="load-more-wrap">
                        <n-spin :size="14" v-if="!noMore" />
                        <span class="load-more-spinner">{{ noMore ? t('post.home.noMoreBubbles') : t('post.home.loadMore') }}</span>
                    </div>
                </template>
            </InfiniteLoading>
        </n-space>
    </div>
</template>

<script setup lang="ts">
import { ref, onMounted, reactive, computed, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useStoreMain } from '@/store/main';
import { useRoute, useRouter } from 'vue-router';
import { useDialog } from 'naive-ui';
import InfiniteLoading from 'v3-infinite-loading';
import { getPosts, getIndexTrends } from '@/api/post';
import SlideBar from '@opentiny/vue-slide-bar';
import allTweets from '@/assets/img/fresh-tweets.png';
import discoverTweets from '@/assets/img/discover-tweets.jpeg';
import followingTweets from '@/assets/img/following-tweets.jpeg';
import { useStoreUser } from '@/store/user';
import { useStoreProfile } from '@/store/profile';
import { storeToRefs } from 'pinia';
import { Api } from '@/utils/request';
import { usePagination } from '@/composables/usePagination';
import { useChatJump } from '@/composables/useUserAction';

const storeMain = useStoreMain();
const storeUser = useStoreUser();
const storeProfile = useStoreProfile();
const { desktopModelShow, refresh } = storeToRefs(storeMain);
const { userInfo } = storeToRefs(storeUser);
const { profile } = storeToRefs(storeProfile);

const route = useRoute();
const router = useRouter();
const dialog = useDialog();
const { t } = useI18n();

const newestTweetsStyle = ref<'newest' | 'hots' | 'following'>('newest');

// 筛选按钮配置
const filterButtons = computed(() => [
  { key: 'newest' as const, label: t('common.all'), index: 0 },
  { key: 'hots' as const, label: t('post.home.filterHot'), index: 1 },
  { key: 'following' as const, label: t('post.home.filterFollowing'), index: 2 },
]);

const onFilterClick = (key: 'newest' | 'hots' | 'following', index: number) => {
  newestTweetsStyle.value = key;
  handleBarClick(slideBarList.value[index], index);
};

// 保留原有的方法以确保兼容性
const onNewestTweets = () => {
  newestTweetsStyle.value = 'newest';
  handleBarClick(slideBarList.value[0], 0);
};
const onHotTweets = () => {
  newestTweetsStyle.value = 'hots';
  handleBarClick(slideBarList.value[1], 1);
};
const onFollowingTweets = () => {
  newestTweetsStyle.value = 'following';
  handleBarClick(slideBarList.value[2], 2);
};

const initBlocks = ref(9);
const wheelBlocks = ref(8);
const slideBarKey = ref(0);
const slideBarList = ref<Item.SlideBarItem[]>([
  { title: t('post.home.latest'), style: 1, username: '', avatar: allTweets, show: true },
  {
    title: t('post.home.filterHot'),
    style: 2,
    username: '',
    avatar: discoverTweets,
    show: false,
  },
  {
    title: t('post.home.filterFollowing'),
    style: 3,
    username: '',
    avatar: followingTweets,
    show: false,
  },
]);
const title = ref<string>(t('post.home.squareTitle'));
const targetStyle = ref<number>(1);
const targetUsername = ref<string>('');
const list = ref<any[]>([]);
// 使用 usePagination composable
const { loading, noMore, page, pageSize, totalPage, reset, nextPage } = usePagination(20);

// 私信入口: 跳转消息页会话(原 whisper 弹窗已移除)
const { goWhisper: onSendWhisper } = useChatJump();

function postFollowAction(userId: number, isFollowing: boolean) {
  for (let index in list.value) {
    if (list.value[index].user_id == userId) {
      list.value[index].user.is_following = isFollowing;
    }
  }

  // 如果是在【正在关注】tab，且是取消关注操作（isFollowing 为 false），则刷新列表
  if (targetStyle.value === 3 && !isFollowing) {
    resetAll();
    loadPosts('following');
  }
}

const updateTitle = () => {
  title.value = t('post.home.squareTitle');
  if (route.query && route.query.q) {
    if (route.query.t && route.query.t === 'tag') {
      title.value = '#' + decodeURIComponent(route.query.q as string);
    } else {
      title.value = t('post.home.searchPrefix') + decodeURIComponent(route.query.q as string);
    }
  }
};

const showTrendsTag = computed(() => {
  return (
    userInfo.value.id > 0 &&
    !profile.value.enableTrendsBar &&
    desktopModelShow.value
  );
});
const showTrendsBar = computed(() => {
  return (
    profile.value.useFriendship &&
    profile.value.enableTrendsBar &&
    desktopModelShow.value &&
    userInfo.value.id > 0
  );
});

// 重写 reset 方法以包含 list 的重置
const resetAll = () => {
  reset();
  list.value = [];
};

const handleBarClick = (data: Item.SlideBarItem, index: number) => {
  resetAll();
  targetStyle.value = data.style;
  if (route.query.q) {
    route.query.q = null;
    updateTitle();
  }
  switch (data.style) {
    case 1:
      loadPosts('newest');
      break;
    case 2:
      loadPosts('hots');
      break;
    case 3:
      route.query.q = null;
      loadPosts('following');
      break;
    case 21:
      targetUsername.value = data.username;
      loadUserPosts();
      break;
    default:
      break;
  }
  slideBarList.value[index].show = false;
};

const loadContacts = () => {
  slideBarList.value = slideBarList.value.slice(0, 3);
  if (
    !profile.value.useFriendship ||
    !profile.value.enableTrendsBar ||
    userInfo.value.id === 0
  ) {
    return;
  }
  getIndexTrends({
    page: 1,
    page_size: 50,
  })
    .then((res) => {
      var i = 0;
      const list = res.list || [];
      let barItems: Item.SlideBarItem[] = [];
      for (; i < list.length; i++) {
        let item: Item.IndexTrendsItem = list[i];
        barItems.push({
          title: item.nickname,
          style: 21,
          username: item.username,
          avatar: item.avatar,
          show: item.is_fresh,
        });
      }
      if (barItems.length > 0) {
        slideBarList.value = slideBarList.value.concat(barItems);
        slideBarKey.value++;
      }
    })
    .catch((err) => {
      console.log(err);
    });
};

const loadPosts = (style: 'newest' | 'hots' | 'following' | 'search') => {
  loading.value = true;
  getPosts({
    query: route.query.q ? decodeURIComponent(route.query.q as string) : null,
    type: route.query.t as string,
    style: style,
    page: page.value,
    page_size: pageSize.value,
  })
    .then((rsp) => {
      loading.value = false;
      if (rsp.list.length === 0) {
        noMore.value = true;
      }

      if (page.value > 1) {
        list.value = list.value.concat(rsp.list);
      } else {
        list.value = rsp.list;
        window.scrollTo(0, 0);
      }

      totalPage.value = Math.ceil(rsp.pager.total_rows / pageSize.value);
    })
    .catch((err) => {
      loading.value = false;
      if (page.value > 1) {
        page.value--;
      }
    });
};

const loadUserPosts = () => {
  loading.value = true;
  Api.v1.user.get.posts({
    username: targetUsername.value,
    style: 'post',
    page: page.value,
    page_size: pageSize.value,
  })
    .then((rsp) => {
      loading.value = false;
      if (rsp.list.length === 0) {
        noMore.value = true;
      }
      if (page.value > 1) {
        list.value = list.value.concat(rsp.list);
      } else {
        list.value = rsp.list || [];
        window.scrollTo(0, 0);
      }
      totalPage.value = Math.ceil(rsp.pager.total_rows / pageSize.value);
    })
    .catch((err) => {
      list.value = [];
      if (page.value > 1) {
        page.value--;
      }
      loading.value = false;
    });
};

const onPostSuccess = (post: Item.PostProps) => {
  // 暂时统统跳到详情页面，后续再精细化分场景优化
  router.push({
    name: 'post',
    query: {
      id: post.id,
    },
  });
  // // 如果不在第一页，需要跳转到详情页面
  // if (targetStyle.value != 1) {
  //     router.push({
  //         name: 'post',
  //         query: {
  //             id: post.id,
  //         },
  //     });
  //     return;
  // }

  // // 如果是在第一页，就地插入新推文到文章列表中
  // let items = [];
  // let length = list.value.length;
  // if (length == pageSize.value) {
  //     length--;
  // }
  // var i = 0;
  // for (; i < length; i++) {
  //     let item: Item.PostProps = list.value[i];
  //     if (!item.is_top) {
  //         break;
  //     }
  //     items.push(item);
  // }
  // items.push(post);
  // for (; i < length; i++) {
  //     items.push(list.value[i]);
  // }
  // list.value = items;
};

const loadMorePosts = () => {
  switch (targetStyle.value) {
    case 1:
      loadPosts('newest');
      break;
    case 2:
      loadPosts('hots');
      break;
    case 3:
      loadPosts('following');
      break;
    case 21:
      if (route.query.q) {
        loadPosts('search');
      } else {
        loadUserPosts();
      }
      break;
    default:
      break;
  }
};

const handleNextPage = () => {
  nextPage(loadMorePosts);
};

onMounted(() => {
  resetAll();
  loadContacts();
  loadPosts('newest');
});

watch(
  () => ({
    path: route.path,
    query: route.query,
    refresh: refresh.value,
  }),
  (to, from) => {
    updateTitle();
    if (to.refresh !== from.refresh) {
      resetAll();
      setTimeout(() => {
        loadContacts();
        loadMorePosts();
      }, 0);
      return;
    }
    if (from.path !== '/post' && to.path === '/') {
      resetAll();
      setTimeout(() => {
        loadContacts();
        loadMorePosts();
      }, 0);
    }
  },
);
</script>

<style lang="less" scoped>

.tiny-slide-bar .tiny-slide-bar__list > 
div.tiny-slide-bar__select .slide-bar-item .slide-bar-item-title {
    color: #18a058;
    opacity: 0.8;
}

.tiny-slide-bar .tiny-slide-bar__list > 
div:hover .slide-bar-item {
    cursor: pointer;
    .slide-bar-item-avatar {
        color: #18a058;
        opacity: 0.8;
    }
    .slide-bar-item-title {
        color: #18a058;
        opacity: 0.8;
    }
}
.style-wrap {
    margin-top: 10px;
    margin-left: 16px;
    margin-bottom: 4px;
    opacity: 0.80;  
    .style-item {
        &.hover {
                cursor: pointer;
        }
    }
}
.tiny-slide-bar {
    margin-top: -30px;
    margin-bottom: -30px;
    .slide-bar-item {
        min-height: 170px;
        width: 64px;
        display: flex;
        flex-direction:column;
        justify-content: center;
        align-items: center;
        margin-top: 8px;
        .slide-bar-item-title {
            justify-content: center;
            font-size: 12px;
            margin-top: 4px;
            height: 40px;
        }
    }
}

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
    .main-content-wrap,
    .pagination-wrap,
    .empty-wrap,
    .skeleton-wrap {
        background-color: rgba(16, 16, 20, 0.75);
    }
    .tiny-slide-bar .tiny-slide-bar__list > 
    div.tiny-slide-bar__select .slide-bar-item .slide-bar-item-title {
        color: #63e2b7;
        opacity: 0.8;
    }

    .tiny-slide-bar .tiny-slide-bar__list > 
    div:hover .slide-bar-item .slide-bar-item-title {
        color: #63e2b7;
        opacity: 0.8;
    }

    .tiny-slide-bar {
        --ti-slider-progress-box-arrow-hover-text-color: #f2f2f2;
        --ti-slider-progress-box-arrow-normal-text-color: #808080;
    }
}
</style>
