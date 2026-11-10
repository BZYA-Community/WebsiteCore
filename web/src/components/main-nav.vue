<template>
    <div v-if="drawerModelShow && !route.meta.hideSidebar">
        <n-drawer
            v-model:show="activeDrawerRef"
            :width=212
            :placement="placementRef"
            @after-leave="restoreDrawerFocus"
            resizable
        >
            <n-drawer-content>
                <sidebar />
            </n-drawer-content>
        </n-drawer>
    </div>
    <n-card size="small" :bordered="true" class="nav-title-card">
        <template #header>
            <div class="navbar">
                <n-button
                    class="drawer-btn"
                    ref="drawerTrigger"
                    v-if="drawerModelShow && !route.meta.hideSidebar && !back"
                    :aria-label="t('nav.openMenu')"
                    :aria-expanded="activeDrawerRef"
                    @click="activeDrawer"
                    quaternary
                    circle
                    size="medium"
                >
                    <template #icon>
                        <n-icon><dehaze-round /></n-icon>
                    </template>
                </n-button>
                <n-button
                    class="back-btn"
                    v-if="back"
                    @click="goBack"
                    quaternary
                    circle
                    size="small"
                >
                    <template #icon>
                        <n-icon><chevron-left-round /></n-icon>
                    </template>
                </n-button>

                {{ props.title }}

                <n-switch
                    v-if="props.theme"
                    :value="themeName === 'dark'"
                    @update:value="switchTheme"
                    size="small"
                    class="theme-switch-wrap"
                >
                    <template #checked-icon>
                        <n-icon :component="DarkModeOutlined" />
                    </template>
                    <template #unchecked-icon>
                        <n-icon :component="LightModeOutlined" />
                    </template>
                </n-switch>
            </div>
        </template>
    </n-card>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useStoreMain } from '@/store/main';
import { useRoute, useRouter } from 'vue-router';
import { useMessage, useOsTheme, DrawerPlacement } from 'naive-ui';
import type { ButtonInst } from 'naive-ui';
import {
  LightModeOutlined,
  DarkModeOutlined,
  ChevronLeftRound,
  DehazeRound,
} from '@vicons/material';
import { storeToRefs } from 'pinia';

const storeMain = useStoreMain();
const { desktopModelShow, drawerModelShow, theme: themeName } = storeToRefs(storeMain);

const router = useRouter();
const route = useRoute();
const { t } = useI18n();
const activeDrawerRef = ref(false);
const drawerTrigger = ref<ButtonInst | null>(null);
const placementRef = ref<DrawerPlacement>('left');

const removeNavigationHook = router.afterEach((_to, _from, failure) => {
  if (failure || !activeDrawerRef.value) return;
  activeDrawerRef.value = false;
  // Navigation may replace this entire header. Focus the destination header,
  // rather than the old drawer trigger that Naive UI would otherwise restore.
  void nextTick(() => {
    if (drawerModelShow.value) {
      document.querySelector<HTMLButtonElement>('.nav-title-card .drawer-btn')?.focus();
    }
  });
});
onBeforeUnmount(removeNavigationHook);
watch(drawerModelShow, (visible) => {
  if (!visible) activeDrawerRef.value = false;
});
function restoreDrawerFocus() {
  if (drawerModelShow.value) drawerTrigger.value?.focus();
}

const props = withDefaults(
  defineProps<{
    title: string;
    back?: boolean;
    theme?: boolean;
  }>(),
  {
    title: '',
    back: false,
    theme: true,
  },
);
const switchTheme = (theme: boolean) => {
  if (theme) {
    localStorage.setItem('PAOPAO_THEME', 'dark');
    storeMain.triggerTheme('dark');
  } else {
    localStorage.setItem('PAOPAO_THEME', 'light');
    storeMain.triggerTheme('light');
  }
};
const goBack = () => {
  if (window.history.length <= 1) {
    router.push({
      path: '/',
    });
  } else {
    router.go(-1);
  }
};
const activeDrawer = () => {
  activeDrawerRef.value = true;
};

onMounted(() => {
  if (!localStorage.getItem('PAOPAO_THEME')) {
    switchTheme((useOsTheme() as unknown as string) === 'dark');
  }
  // 移动端特殊处理
  if (!desktopModelShow.value) {
    window.$message = useMessage();
  }
});
</script>

<style lang="less">
.nav-title-card {
    z-index: 99;
    width: 100%;
    top: 0;
    position: sticky;
    border-radius: 0;
    border-bottom: 0;
    background-color: rgba(255, 255, 255, 0.75);
    backdrop-filter: blur(12px);

    .navbar {
        height: 30px;
        position: relative;
        display: flex;
        align-items: center;

        .drawer-btn, 
        .back-btn {
            margin-right: 8px;
        }

        .theme-switch-wrap {
            position: absolute;
            right: 0;
            top: calc(50% - 9px);
        }
    }
}
.dark {
    .nav-title-card {
        background-color: rgba(16, 16, 20, 0.75);
    }
}
</style>
