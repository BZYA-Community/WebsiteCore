<template>
    <n-config-provider :theme="iTheme" :locale="naiveLocale" :date-locale="naiveDateLocale">
        <n-message-provider>
            <n-dialog-provider>
                <div
                    class="app-container"
                    :class="{ dark: iTheme?.name === 'dark', mobile: !desktopModelShow }"
                >
                    <div has-sider class="main-wrap" position="static" >
                        <!-- 侧边栏 -->
                        <div v-if="desktopModelShow">
                            <sidebar />
                        </div>

                        <div class="content-wrap">
                            <router-view
                                class="app-wrap"
                                v-slot="{ Component }"
                            >
                                <keep-alive>
                                    <component
                                        v-if="$route.meta.keepAlive"
                                        :is="Component"
                                    />
                                </keep-alive>
                                <component
                                    v-if="!$route.meta.keepAlive"
                                    :is="Component"
                                />
                            </router-view>
                        </div>

                        <!-- 右侧 -->
                        <rightbar />
                    </div>
                    <!-- 登录/注册公共组件 -->
                    <auth />
                </div>
            </n-dialog-provider>
        </n-message-provider>
        <n-global-style />
    </n-config-provider>
</template>

<script setup lang="ts">
import { onMounted, computed } from 'vue';
import { useStoreMain } from '@/store/main';
import { darkTheme, zhCN, enUS, dateZhCN, dateEnUS } from 'naive-ui';
import { getSiteProfile } from '@/api/site';
import { useStoreProfile } from '@/store/profile';
import { storeToRefs } from 'pinia';
import { useI18n } from 'vue-i18n';

const storeMain = useStoreMain();
const storeProfile = useStoreProfile();
const { theme, desktopModelShow } = storeToRefs(storeMain);
const { locale } = useI18n();

const iTheme = computed(() => (theme.value === 'dark' ? darkTheme : null));
// naive-ui 组件内置文案/日期本地化, 随 vue-i18n 语言切换联动
const naiveLocale = computed(() => (locale.value === 'en' ? enUS : zhCN));
const naiveDateLocale = computed(() => (locale.value === 'en' ? dateEnUS : dateZhCN));

function loadSiteProfile() {
    storeProfile.loadDefaultSiteProfile();
    if (import.meta.env.VITE_USE_WEB_PROFILE.toLowerCase() === 'true') {
        getSiteProfile()
            .then((res) => {
                storeProfile.updateSiteProfile(res);
            }).catch((err) => {
                console.log(err);
            });
    }
}

onMounted(() => {
  loadSiteProfile();
});
</script>
