import { computed, onBeforeUnmount, ref } from 'vue';
import { Api } from '@/utils/request';
import { useStoreUser } from '@/store/user';
import { useStoreProfile } from '@/store/profile';
import { formatRelativeTime } from '@/utils/formatTime';
import moment from 'moment';

/**
 * 私信会话数据流管理(B站式: 左会话列表 + 右聊天窗)
 * 数据适配 vue-advanced-chat Web Component:
 * 注意其对象型 props 必须 JSON.stringify 后传入, 事件为 CustomEvent(detail[0] 为负载)
 */

export interface VACRoomUser {
  _id: string;
  username: string;
  avatar?: string;
}

export interface VACRoom {
  roomId: string;
  roomName: string;
  avatar: string;
  users: VACRoomUser[];
  unreadCount?: number;
  index?: number;
  lastMessage?: {
    content: string;
    senderId: string;
    timestamp: string;
  };
}

export interface VACMessage {
  _id: string;
  senderId: string;
  content: string;
  username?: string;
  avatar?: string;
  date?: string;
  timestamp?: string;
  seen?: boolean;
  system?: boolean;
  disableActions?: boolean;
  disableReactions?: boolean;
}

const SYSTEM_ROOM_ID = '0';
const PAGE_SIZE = 20;

const dayLabel = (ts: number) => moment.unix(ts).format('YYYY年M月D日');
const timeLabel = (ts: number) => moment.unix(ts).format('HH:mm');

export function useChatRooms() {
  const storeUser = useStoreUser();
  const storeProfile = useStoreProfile();

  const myId = computed(() => String(storeUser.userInfo.id || 0));
  const myName = computed(() => storeUser.userInfo.nickname || '');

  const rooms = ref<VACRoom[]>([]);
  const messages = ref<VACMessage[]>([]);
  const roomsLoaded = ref(false);
  const messagesLoaded = ref(false);
  const loadingRooms = ref(false);
  const activeRoomId = ref('');
  const canSend = ref(true);
  const canSendTip = ref('');
  const sending = ref(false);

  // 会话对端信息缓存(roomId -> contact), 临时会话(?to=直接进入)也放这里
  const peerMap = new Map<string, Item.ChatContactItem>();
  // 每会话分页状态
  const pageState = new Map<string, { page: number; total: number }>();
  // 是否为待落库的临时会话(还没发过消息)
  const tempRooms = new Set<string>();

  let contactsTimer: ReturnType<typeof setInterval> | null = null;
  let historyTimer: ReturnType<typeof setInterval> | null = null;

  const toRoom = (c: Item.ChatContactItem): VACRoom => ({
    roomId: String(c.user_id),
    roomName: c.nickname,
    avatar: c.avatar,
    users: [{ _id: String(c.user_id), username: c.nickname, avatar: c.avatar }],
    unreadCount: c.unread > 0 ? c.unread : undefined,
    index: c.last_time,
    lastMessage: c.last_content
      ? {
          content: (c.last_from_me ? '我: ' : '') + c.last_content,
          senderId: c.last_from_me ? myId.value : String(c.user_id),
          timestamp: formatRelativeTime(c.last_time),
        }
      : undefined,
  });

  /** 系统会话消息 → 组件气泡内容(通知文本 + 跳转链接) */
  const composeSystemContent = (m: Item.ChatHistoryItem): string => {
    const base = `${location.origin}${location.pathname}`;
    const lines: string[] = [];
    if (m.brief) lines.push(m.brief);
    if (m.content) lines.push(m.content);
    if (m.type >= 1 && m.type <= 3 && m.post_id) {
      lines.push(`查看详情: ${base}#/post?id=${m.post_id}`);
    } else if (m.type === 99 && m.sender_id > 0 && m.sender_username) {
      lines.push(`查看主页: ${base}#/u?s=${encodeURIComponent(m.sender_username)}`);
    }
    return lines.join('\n');
  };

  const toMessage = (m: Item.ChatHistoryItem, isSystem: boolean): VACMessage => ({
    _id: String(m.id),
    senderId: String(m.sender_id),
    username: isSystem ? m.sender_name || '系统' : undefined,
    content: isSystem ? composeSystemContent(m) : m.content,
    date: dayLabel(m.timestamp),
    timestamp: timeLabel(m.timestamp),
    seen: m.seen,
    disableActions: true,
    disableReactions: true,
  });

  /** 拉取会话列表并映射为 rooms(系统联系人固定首位) */
  const loadContacts = async () => {
    try {
      const res = await Api.v1.user.get.chat.contacts({});
      const list: VACRoom[] = [
        {
          roomId: SYSTEM_ROOM_ID,
          roomName: '系统通知',
          avatar: res.system.avatar || '/logo.png',
          users: [{ _id: SYSTEM_ROOM_ID, username: '系统通知' }],
          unreadCount: res.system.unread > 0 ? res.system.unread : undefined,
          index: res.system.last_time || Number.MAX_SAFE_INTEGER / 1000,
          lastMessage: res.system.last_content
            ? {
                content: res.system.last_content,
                senderId: SYSTEM_ROOM_ID,
                timestamp: formatRelativeTime(res.system.last_time),
              }
            : undefined,
        },
      ];
      for (const c of res.contacts || []) {
        peerMap.set(String(c.user_id), c);
        list.push(toRoom(c));
      }
      // 保留还未落库的临时会话(对方尚未回复、列表里还没有)
      for (const id of tempRooms) {
        if (!list.find((r) => r.roomId === id)) {
          const peer = peerMap.get(id);
          if (peer) list.push(toRoom(peer));
        }
      }
      rooms.value = list;
      roomsLoaded.value = true;
    } catch (err) {
      console.log('load chat contacts error:', err);
    }
  };

  /**
   * 拉取会话历史
   * @param mergeNew true 时为轮询模式: 仅更新已读态并追加新消息, 不打扰滚动位置
   */
  const loadHistory = async (roomId: string, page: number, mergeNew = false) => {
    const isSystem = roomId === SYSTEM_ROOM_ID;
    const res = await Api.v1.user.get.chat.history({
      user_id: Number(roomId),
      page,
      page_size: PAGE_SIZE,
    });
    pageState.set(roomId, { page, total: res.total_rows });

    if (!isSystem && res.peer) {
      peerMap.set(roomId, res.peer);
      // 回填临时会话的标题/头像
      const idx = rooms.value.findIndex((r) => r.roomId === roomId);
      if (idx >= 0) {
        const r = rooms.value[idx];
        rooms.value[idx] = {
          ...r,
          roomName: res.peer.nickname,
          avatar: res.peer.avatar,
          users: [
            { _id: roomId, username: res.peer.nickname, avatar: res.peer.avatar },
          ],
        };
        rooms.value = [...rooms.value];
      }
    }
    canSend.value = res.can_send;
    canSendTip.value = res.can_send_tip || '';

    const items = res.messages.map((m) => toMessage(m, isSystem));
    if (page <= 1) {
      if (mergeNew) {
        // 轮询合并: 更新已有消息(已读态), 追加新消息
        const known = new Map(messages.value.map((m) => [m._id, m]));
        const fresh: VACMessage[] = [];
        for (const item of items) {
          if (known.has(item._id)) {
            known.set(item._id, { ...known.get(item._id)!, seen: item.seen });
          } else {
            fresh.push(item);
          }
        }
        messages.value = [...known.values(), ...fresh];
      } else {
        messages.value = items;
      }
      // 打开即读(服务端已标记), 本地角标清零
      const idx = rooms.value.findIndex((r) => r.roomId === roomId);
      if (idx >= 0 && rooms.value[idx].unreadCount) {
        rooms.value[idx] = { ...rooms.value[idx], unreadCount: undefined };
        rooms.value = [...rooms.value];
      }
    } else {
      // 向上翻页: 前置拼接更早的消息
      messages.value = [...items, ...messages.value];
    }
    const st = pageState.get(roomId)!;
    messagesLoaded.value = st.page * PAGE_SIZE >= st.total;
  };

  /** fetch-messages: 打开会话(reset)或滚动到顶加载更多 */
  const onFetchMessages = async (detail: { room: VACRoom; options?: { reset?: boolean } }) => {
    const roomId = detail.room.roomId;
    activeRoomId.value = roomId;
    const reset = detail.options?.reset;
    if (reset) {
      messages.value = [];
      messagesLoaded.value = false;
      canSendTip.value = '';
      try {
        await loadHistory(roomId, 1);
      } catch (err) {
        console.log('load chat history error:', err);
      }
      return;
    }
    const st = pageState.get(roomId);
    if (!st || st.page * PAGE_SIZE >= st.total) {
      messagesLoaded.value = true;
      return;
    }
    try {
      await loadHistory(roomId, st.page + 1);
    } catch (err) {
      console.log('load chat history more error:', err);
    }
  };

  /** 发送私信 */
  const onSendMessage = async (detail: { roomId: string; content: string }) => {
    const roomId = detail.roomId;
    const content = (detail.content || '').trim();
    if (!content || sending.value || roomId === SYSTEM_ROOM_ID) return;
    sending.value = true;
    try {
      const res = await Api.v1.user.post.chat.send({
        user_id: Number(roomId),
        content,
      });
      // 本地回显, 轮询会 reconcile 已读态
      messages.value = [
        ...messages.value,
        {
          _id: String(res.message_id),
          senderId: myId.value,
          content,
          date: dayLabel(Date.now() / 1000),
          timestamp: timeLabel(Date.now() / 1000),
          seen: false,
          disableActions: true,
          disableReactions: true,
        },
      ];
      // 首条消息后临时会话转为正式: 刷新列表
      if (tempRooms.has(roomId)) {
        tempRooms.delete(roomId);
      }
      loadContacts();
    } catch (err) {
      // 错误提示由 axios 拦截器统一 toast(含后端权限文案)
      console.log('send chat message error:', err);
    } finally {
      sending.value = false;
    }
  };

  /** 打开与指定用户的会话(入口跳转用); 无历史会话时先造临时 room */
  const openPeerRoom = async (userId: number, seed?: Partial<Item.ChatContactItem>) => {
    if (!userId || userId <= 0) return;
    const roomId = String(userId);
    if (!rooms.value.find((r) => r.roomId === roomId)) {
      const peer: Item.ChatContactItem = {
        user_id: userId,
        username: seed?.username || '',
        nickname: seed?.nickname || '加载中…',
        avatar: seed?.avatar || '',
        roles: seed?.roles || [],
        identity: seed?.identity || '',
        last_content: '',
        last_time: Math.floor(Date.now() / 1000),
        last_from_me: false,
        unread: 0,
      };
      peerMap.set(roomId, peer);
      tempRooms.add(roomId);
      rooms.value = [...rooms.value, toRoom(peer)];
    }
    activeRoomId.value = roomId;
    // 组件 room-id 变化会触发 fetch-messages(reset) 加载历史
  };

  /** 轮询: 会话列表慢轮询 + 活跃会话快轮询 */
  const startPolling = () => {
    const slow = Math.max(storeProfile.profile.defaultMsgLoopInterval || 5000, 3000);
    contactsTimer = setInterval(() => {
      if (!document.hidden) loadContacts();
    }, slow);
    historyTimer = setInterval(() => {
      if (!document.hidden && activeRoomId.value) {
        loadHistory(activeRoomId.value, 1, true).catch((err) =>
          console.log('poll chat history error:', err),
        );
      }
    }, 3000);
  };

  const stopPolling = () => {
    if (contactsTimer) clearInterval(contactsTimer);
    if (historyTimer) clearInterval(historyTimer);
    contactsTimer = null;
    historyTimer = null;
  };

  onBeforeUnmount(stopPolling);

  return {
    myId,
    myName,
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
    stopPolling,
  };
}
