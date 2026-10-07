import { defineStore } from 'pinia';
import { computed, ref } from 'vue';

export const useStoreMain = defineStore('main', () => {
  const refresh = ref(Date.now());
  const refreshTopicFollow = ref(Date.now());
  const theme = ref(localStorage.getItem('PAOPAO_THEME'));
  const collapsedLeft = ref(window.innerWidth <= 821);
  // 右侧栏较宽(240px), 窄于1140px时布局放不下会右侧被裁切, 提前隐藏
  const collapsedRight = ref(window.innerWidth <= 1140);
  const drawerModelShow = computed(() => collapsedLeft.value);
  const desktopModelShow = computed(() => !collapsedLeft.value);
  const authModalShow = ref(false);
  const authModelTab = ref('signin');
  const unreadMsgCount = ref(0);

  function doRefresh(val?: number) {
    refresh.value = val || Date.now();
  }

  function doRefreshTopicFollow() {
    refreshTopicFollow.value = Date.now();
  }

  function updateUnreadMsgCount(count: number) {
    unreadMsgCount.value = count;
  }

  function triggerTheme(t: string) {
    theme.value = t;
  }

  function triggerAuth(status: boolean) {
    authModalShow.value = status;
  }

  function triggerAuthKey(key: string) {
    authModelTab.value = key;
  }

  function triggerCollapsedLeft(status: boolean) {
    collapsedLeft.value = status;
  }

  function triggerCollapsedRight(status: boolean) {
    collapsedRight.value = status;
  }

  return {
    refresh,
    refreshTopicFollow,
    theme,
    collapsedLeft,
    collapsedRight,
    drawerModelShow,
    desktopModelShow,
    authModalShow,
    authModelTab,
    unreadMsgCount,
    doRefresh,
    doRefreshTopicFollow,
    updateUnreadMsgCount,
    triggerTheme,
    triggerAuth,
    triggerAuthKey,
    triggerCollapsedLeft,
    triggerCollapsedRight,
  };
});
