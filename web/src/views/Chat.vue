<template>
    <div>
        <main-nav title="消息" />
        <div class="chat-page">
            <!--
                vue-advanced-chat 为 Web Component:
                1. 对象/数组型 props 必须 JSON.stringify, 否则组件内部 JSON.parse("[object Object]") 报错
                2. 事件为 CustomEvent, 负载在 $event.detail[0]
            -->
            <vue-advanced-chat
                height="100%"
                :current-user-id="myId"
                :rooms="JSON.stringify(rooms)"
                rooms-order="desc"
                :rooms-loaded="roomsLoaded"
                :loading-rooms="loadingRooms"
                :room-id="activeRoomId"
                :messages="JSON.stringify(messages)"
                :messages-loaded="messagesLoaded"
                :show-add-room="false"
                :show-files="false"
                :show-audio="false"
                :show-video="false"
                :show-reaction-emojis="false"
                :show-new-messages-divider="false"
                :show-footer="canSend"
                :message-actions="JSON.stringify([])"
                :menu-actions="JSON.stringify([])"
                :room-actions="JSON.stringify([])"
                :text-formatting="JSON.stringify({ disabled: true })"
                :link-options="JSON.stringify({ disabled: false, target: '_blank' })"
                :text-messages="JSON.stringify(textMessages)"
                :styles="JSON.stringify(chatStyles)"
                :theme="storeMain.theme === 'dark' ? 'dark' : 'light'"
                :responsive-breakpoint="821"
                @fetch-messages="onFetchMessages($event.detail[0])"
                @send-message="onSendMessage($event.detail[0])"
            />
            <n-alert
                v-if="activeRoomId && !canSend"
                class="chat-tip"
                type="warning"
                :bordered="false"
            >
                {{ canSendTip }}
            </n-alert>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted, watch } from 'vue';
import { useRoute } from 'vue-router';
import { useStoreMain } from '@/store/main';
import { useChatRooms } from '@/composables/useChatRooms';

const storeMain = useStoreMain();
const route = useRoute();

const {
    myId,
    rooms,
    messages,
    roomsLoaded,
    messagesLoaded,
    loadingRooms,
    activeRoomId,
    canSend,
    canSendTip,
    loadContacts,
    onFetchMessages,
    onSendMessage,
    openPeerRoom,
    startPolling,
} = useChatRooms();

// 组件文案中文化
const textMessages = {
    ROOMS_EMPTY: '暂无会话',
    ROOM_EMPTY: '选择会话开始聊天',
    NEW_MESSAGES: '新消息',
    MESSAGES_EMPTY: '暂无消息',
    CONVERSATION_STARTED: '会话开始于:',
    TYPE_MESSAGE: '输入私信内容…',
    SEARCH: '搜索会话',
    IS_ONLINE: '在线',
    IS_TYPING: '正在输入…',
};

// 配色对齐站点(亮/暗两套由 theme prop 切换, 这里覆盖容器背景与气泡主色)
const chatStyles = computed(() => {
    const dark = storeMain.theme === 'dark';
    return {
        general: {
            backgroundInput: dark ? '#2c2c32' : '#ffffff',
        },
        footer: {
            background: dark ? '#18181c' : '#f8f8f8',
        },
    };
});

// ?to=<user_id> 进入: 直接打开与某人的会话(无私信入口传 nickname/avatar 种子信息)
const openFromRoute = () => {
    const to = Number(route.query.to);
    if (to > 0) {
        openPeerRoom(to, {
            username: (route.query.u as string) || '',
            nickname: (route.query.n as string) || '',
            avatar: (route.query.a as string) || '',
        });
    }
};

watch(() => route.query.to, openFromRoute);

onMounted(async () => {
    loadingRooms.value = true;
    await loadContacts();
    loadingRooms.value = false;
    openFromRoute();
    startPolling();
});
</script>

<style lang="less" scoped>
.chat-page {
    position: relative;
    // 100vh 减去顶栏; 聊天窗铺满剩余高度
    height: calc(100vh - 62px);

    .chat-tip {
        position: absolute;
        left: 0;
        right: 0;
        bottom: 0;
        z-index: 10;
        justify-content: center;
    }
}

@media screen and (max-width: 821px) {
    .chat-page {
        // 移动端顶栏更矮
        height: calc(100vh - 54px);
    }
}
</style>
