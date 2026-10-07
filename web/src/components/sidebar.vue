<template>
    <div class="sidebar-wrap">
        <div class="logo-wrap">
            <n-image class="logo-img" width="36" :src="LOGO" :preview-disabled="true" @click="goHome" />
        </div>
        <n-menu :accordion="true" :icon-size="24" :options="menuOptions" :render-label="renderMenuLabel"
            :render-icon="renderMenuIcon" :value="selectedPath" @update:value="goRouter" />

        <div class="user-wrap" v-if="userInfo.id > 0">
            <n-avatar class="user-avatar" round :size="34" :src="userInfo.avatar" />

            <div class="user-info">
                <div class="nickname">
                    <span class="nickname-txt">
                        {{ userInfo.nickname }}
                    </span>
                    <span class="lang-btn">
                        <lang-switcher />
                    </span>
                    <n-button class="logout" quaternary circle size="tiny" :title="t('sidebar.logout')" @click="handleLogout">
                        <template #icon>
                            <n-icon>
                                <log-out-outline />
                            </n-icon>
                        </template>
                    </n-button>
                </div>
                <div class="username">@{{ userInfo.username }}</div>
            </div>

        </div>
        <div class="user-wrap guest-wrap" v-else>
            <n-button class="guest-auth" strong secondary round size="small" type="primary" @click="triggerAuth('signin')">
                {{ t('sidebar.login') }}
            </n-button>
            <n-button v-if="profile.allowUserRegister" class="guest-auth" quaternary size="small" @click="triggerAuth('signup')">
                {{ t('sidebar.signup') }}
            </n-button>
            <div class="guest-lang">
                <lang-switcher size="small" :icon-size="20" />
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { h, watch, computed } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { routePermissions } from '@/router';
import { useI18n } from 'vue-i18n';
import { useStoreMain } from '@/store/main';
import { NIcon, NBadge, useMessage } from 'naive-ui';
import {
  HomeOutline,
  BookmarksOutline,
  ChatbubblesOutline,
  LeafOutline,
  PeopleCircleOutline,
  ShieldCheckmarkOutline,
  SettingsOutline,
  ConstructOutline,
  LogOutOutline,
  VideocamOutline,
} from '@vicons/ionicons5';
import { Hash } from '@vicons/tabler';
import LOGO from '@/assets/img/logo.png';
import { useStoreUser } from '@/store/user';
import { useStoreProfile } from '@/store/profile';
import { storeToRefs } from 'pinia';
import { Api } from '@/utils/request';

const { t } = useI18n();
const storeMain = useStoreMain();
const storeUser = useStoreUser();
const storeProfile = useStoreProfile();
const { unreadMsgCount } = storeToRefs(storeMain);
const { userInfo } = storeToRefs(storeUser);
const { profile } = storeToRefs(storeProfile);

const route = useRoute();
const router = useRouter();
const hasUnreadMsg = computed(() => unreadMsgCount.value > 0);
const selectedPath = computed(() => route.name === 'course' ? 'courses' : route.name || '');

watch(
  () => [userInfo.value.id, storeUser.hasPermission('profile.edit'), profile.value.defaultMsgLoopInterval] as const,
  ([id, allowed, interval], _previous, onCleanup) => {
    storeMain.updateUnreadMsgCount(0);
    if (id <= 0 || !allowed) return;

    let active = true;
    let pending = false;
    const refreshUnread = async () => {
      if (pending) return;
      pending = true;
      try {
        const res = await Api.v1.user.get.msgcount.unread({});
        if (active) storeMain.updateUnreadMsgCount(res.count);
      } catch (err) {
        if (active) console.warn('Failed to load unread message count', err);
      } finally {
        pending = false;
      }
    };
    const timer = setInterval(refreshUnread, Math.max(1000, interval || 5000));
    onCleanup(() => {
      active = false;
      clearInterval(timer);
    });
    void refreshUnread();
  },
  { immediate: true, flush: 'sync' },
);
const menuOptions = computed(() => [
  { label: t('nav.home'), key: 'home', icon: () => h(HomeOutline), href: '/' },
  { label: t('nav.topic'), key: 'topic', icon: () => h(Hash), href: '/topic' },
  { label: t('nav.courses'), key: 'courses', icon: () => h(VideocamOutline), href: '/courses' },
  { label: t('nav.profile'), key: 'profile', icon: () => h(LeafOutline), href: '/profile', login: true },
  { label: t('nav.messages'), key: 'messages', icon: () => h(ChatbubblesOutline), href: '/messages', login: true },
  { label: t('nav.collection'), key: 'collection', icon: () => h(BookmarksOutline), href: '/collection', login: true },
  { label: t('nav.setting'), key: 'setting', icon: () => h(SettingsOutline), href: '/setting', login: true },
  { label: t('nav.adminSettings'), key: 'admin-settings', icon: () => h(ConstructOutline), href: '/admin/settings', login: true },
  { label: t('nav.adminUsers'), key: 'admin-users', icon: () => h(PeopleCircleOutline), href: '/admin/users', login: true },
  { label: t('nav.adminAudit'), key: 'admin-audit', icon: () => h(ShieldCheckmarkOutline), href: '/admin/audit', login: true },
].filter((option) => (!option.login || storeUser.userLogined) &&
  (option.key !== 'courses' || profile.value.coursesEnabled) &&
  (!routePermissions[option.key] || storeUser.hasAnyPermission(routePermissions[option.key]))));

const renderMenuLabel = (option: AnyObject) => {
  if ('href' in option) {
    return h('div', {}, option.label);
  }
  return option.label;
};
const renderMenuIcon = (option: AnyObject) => {
  if (option.key === 'messages') {
    return h(
      NBadge,
      {
        dot: true,
        show: hasUnreadMsg.value,
        processing: true,
      },
      {
        default: () =>
          h(
            NIcon,
            {
              color:
                option.key === selectedPath.value
                  ? 'var(--n-item-icon-color-active)'
                  : 'var(--n-item-icon-color)',
            },
            { default: option.icon },
          ),
      },
    );
  }
  return h(NIcon, null, { default: option.icon });
};

const goRouter = (name: string) => {
  router.push({
    name,
    query: {
      t: new Date().getTime(),
    },
  });
};
const goHome = () => {
  if (route.path === '/') {
    storeMain.doRefresh();
  }
  goRouter('home');
};
const triggerAuth = (key: string) => {
  storeMain.triggerAuth(true);
  storeMain.triggerAuthKey(key);
};
const handleLogout = async () => {
  storeUser.userLogout();
  await storeUser.loadSession();
  storeMain.doRefresh();
  goHome();
};
window.$message = useMessage();
</script>

<style lang="less">
.sidebar-wrap::-webkit-scrollbar,
.sidebar-wrap .n-menu::-webkit-scrollbar {
    width: 0;
    /* 隐藏滚动条的宽度 */
    height: 0;
    /* 隐藏滚动条的高度 */
}

.sidebar-wrap {
    z-index: 99;
    width: 200px;
    height: 100vh;
    position: fixed;
    right: calc(50% + var(--content-main) / 2 + 10px);
    padding: 12px 0;
    box-sizing: border-box;
    /* 纵向flex布局: 菜单区自适应滚动, 底部用户卡片固定位置不被菜单压住 */
    display: flex;
    flex-direction: column;
    overflow: hidden;

    .n-menu {
        flex: 1 1 0;
        min-height: 0;
        overflow-y: auto;
    }

    .n-menu .n-menu-item-content::before {
        border-radius: 21px;
    }


    .logo-wrap {
        display: flex;
        justify-content: flex-start;
        margin-bottom: 12px;

        .logo-img {
            margin-left: 24px;

            &:hover {
                cursor: pointer;
            }
        }
    }

    .user-wrap {
        display: flex;
        align-items: center;
        flex-shrink: 0;
        padding: 12px 12px 0;

        &.guest-wrap {
            gap: 6px;
            padding-inline: 8px;

            .guest-auth {
                flex: 1;
                min-width: 0;
                padding: 0 8px;
            }
        }

        .user-avatar {
            margin-right: 8px;
            flex-shrink: 0;
        }

        .user-info {
            flex: 1;
            min-width: 0;
            display: flex;
            flex-direction: column;

            .nickname {
                font-size: 16px;
                font-weight: bold;
                line-height: 16px;
                height: 16px;
                margin-bottom: 2px;
                display: flex;
                align-items: center;

                .nickname-txt {
                    flex: 1;
                    min-width: 0;
                    text-overflow: ellipsis;
                    overflow: hidden;
                    white-space: nowrap;
                }

                .lang-btn {
                    flex-shrink: 0;
                    display: inline-flex;
                    align-items: center;
                    margin-left: 6px;
                }

                .logout {
                    flex-shrink: 0;
                    margin-left: 6px;
                }
            }

            .username {
                font-size: 14px;
                line-height: 16px;
                height: 16px;
                width: 100%;
                text-overflow: ellipsis;
                overflow: hidden;
                white-space: nowrap;
                opacity: 0.75;
            }
        }

        .guest-lang {
            display: flex;
            align-items: center;
            flex-shrink: 0;
        }
    }
}

.auth-card {
    .n-card-header {
        z-index: 999;
    }
}

@media screen and (max-width: 821px) {
    .sidebar-wrap {
        width: 200px;
        right: calc(100% - 200px);
    }

    .logo-wrap {
        .logo-img {
            margin-left: 12px !important;
        }
    }

    .user-wrap {

        .user-avatar,
        .user-info {
            margin-bottom: 32px;
        }

        &.guest-wrap {
            margin-bottom: 32px;
        }

    }
}</style>
