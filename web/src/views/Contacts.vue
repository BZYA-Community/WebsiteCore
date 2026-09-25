<template>
    <div>
        <main-nav title="通讯录" />

        <n-list class="main-content-wrap" bordered>
            <n-tabs type="line" animated v-model:value="tab">
                <n-tab-pane name="contact"><template #tab>好友</template></n-tab-pane>
                <n-tab-pane name="requesting"><template #tab>好友申请</template></n-tab-pane>
            </n-tabs>

            <!-- 好友列表 -->
            <template v-if="tab === 'contact'">
                <div v-if="loading && list.length === 0" class="skeleton-wrap">
                    <post-skeleton :num="pageSize" />
                </div>
                <div v-else>
                    <div class="empty-wrap" v-if="list.length === 0">
                        <n-empty size="large" description="暂无数据" />
                    </div>

                    <n-list-item class="list-item" v-for="contact in list" :key="contact.user_id">
                         <user-card type="contact" :contact="contact" @send-whisper="onSendWhisper" @delete-success="onDeleteFriend" />
                    </n-list-item>
                </div>

                <infinite-load-more
                    :total-page="totalPage"
                    :no-more="noMore"
                    complete-text="没有更多好友了"
                    @load-more="nextPage"
                />
            </template>

            <!-- 好友申请列表(同意/拒绝) -->
            <template v-else>
                <div v-if="reqLoading && reqList.length === 0" class="skeleton-wrap">
                    <message-skeleton :num="5" />
                </div>
                <div v-else>
                    <div class="empty-wrap" v-if="reqList.length === 0">
                        <n-empty size="large" description="暂无好友申请" />
                    </div>
                    <n-list-item v-for="m in reqList" :key="m.id">
                        <message-item :message="m" @send-whisper="onSendWhisper" @reload="reloadRequests" />
                    </n-list-item>
                </div>

                <infinite-load-more
                    :total-page="reqTotalPage"
                    :no-more="reqNoMore"
                    complete-text="没有更多申请了"
                    @load-more="nextReqPage"
                />
            </template>
        </n-list>
    </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useRoute } from 'vue-router';
import { Api } from '@/utils/request';
import { usePagination } from '@/composables/usePagination';
import { useChatJump } from '@/composables/useUserAction';
import InfiniteLoadMore from '@/components/infinite-load-more.vue';
import UserCard from '@/components/user-card.vue';

const route = useRoute();
const tab = ref<'contact' | 'requesting'>(
  route.query.t === 'requesting' ? 'requesting' : 'contact',
);

// 私信入口: 跳转消息页会话(原 whisper 弹窗已移除)
const { goWhisper: onSendWhisper } = useChatJump();

// ===== 好友 tab =====
const { loading, noMore, page, pageSize, totalPage } = usePagination(20);
const list = ref<Item.ContactItemProps[]>([]);

// 初始化页码
page.value = +(route.query.p as string) || 1;

// 删除好友后从列表移除
const onDeleteFriend = (userId: number) => {
  list.value = list.value.filter((c) => c.user_id !== userId);
};

const nextPage = () => {
  if (page.value < totalPage.value || totalPage.value == 0) {
    noMore.value = false;
    page.value++;
    loadContacts();
  } else {
    noMore.value = true;
  }
};

const loadContacts = (scrollToBottom: boolean = false) => {
  if (list.value.length === 0) {
    loading.value = true;
  }
  Api.v1.user.get.contacts({
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
        if (scrollToBottom) {
          setTimeout(() => {
            window.scrollTo(0, 99999);
          }, 50);
        }
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

// ===== 好友申请 tab =====
const reqLoading = ref(false);
const reqNoMore = ref(false);
const reqList = ref<Item.MessageProps[]>([]);
const reqPage = ref(1);
const reqTotalPage = ref(0);

const loadRequests = () => {
  if (reqList.value.length === 0) {
    reqLoading.value = true;
  }
  Api.v1.user.get.messages({
    style: 'requesting',
    page: reqPage.value,
    page_size: 20,
  })
    .then((res) => {
      reqLoading.value = false;
      if (res.list.length === 0) {
        reqNoMore.value = true;
      }
      if (reqPage.value > 1) {
        reqList.value = reqList.value.concat(res.list);
      } else {
        reqList.value = res.list;
      }
      reqTotalPage.value = Math.ceil(res.pager.total_rows / 20);
    })
    .catch((_err) => {
      reqLoading.value = false;
      if (reqPage.value > 1) {
        reqPage.value--;
      }
    });
};

const reloadRequests = () => {
  reqPage.value = 1;
  reqNoMore.value = false;
  loadRequests();
};

const nextReqPage = () => {
  if (reqPage.value < reqTotalPage.value || reqTotalPage.value == 0) {
    reqNoMore.value = false;
    reqPage.value++;
    loadRequests();
  } else {
    reqNoMore.value = true;
  }
};

onMounted(() => {
  loadContacts();
  loadRequests();
});
</script>

<style lang="less" scoped>
.dark {
    .main-content-wrap, .empty-wrap, .skeleton-wrap {
        background-color: rgba(16, 16, 20, 0.75);
    }
}
</style>
