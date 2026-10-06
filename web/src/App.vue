<template>
    <n-config-provider :theme="iTheme" :locale="naiveLocale" :date-locale="naiveDateLocale">
        <n-message-provider>
            <n-dialog-provider>
                <div
                    class="app-container"
                    :class="{ dark: iTheme?.name === 'dark', mobile: !desktopModelShow, 'course-shell': isCourseRoute }"
                    :style="isCourseRoute ? courseThemeVars : undefined"
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
                        <rightbar v-if="route.name !== 'course'" />
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
import { computed } from 'vue';
import { useStoreMain } from '@/store/main';
import { darkTheme, lightTheme, zhCN, enUS, dateZhCN, dateEnUS } from 'naive-ui';
import { storeToRefs } from 'pinia';
import { useI18n } from 'vue-i18n';
import { useRoute } from 'vue-router';

const storeMain = useStoreMain();
const { theme, desktopModelShow } = storeToRefs(storeMain);
const { locale } = useI18n();
const route = useRoute();
const isCourseRoute = computed(() => ['courses', 'course'].includes(String(route.name)));

const iTheme = computed(() => (theme.value === 'dark' ? darkTheme : null));
const courseThemeVars = computed(() => {
    const tokens = (iTheme.value || lightTheme).common!;
    return {
        '--course-bg': tokens.bodyColor,
        '--course-surface': tokens.cardColor,
        '--course-text': tokens.textColor2,
        '--course-muted': tokens.textColor3,
        '--course-accent': tokens.primaryColor,
        '--course-line': tokens.dividerColor,
        '--course-hover': tokens.hoverColor,
        '--course-radius': tokens.borderRadius,
        '--course-tint': 'color-mix(in srgb, var(--course-accent) 10%, transparent)',
    };
});
// naive-ui 组件内置文案/日期本地化, 随 vue-i18n 语言切换联动
const naiveLocale = computed(() => (locale.value === 'en' ? enUS : zhCN));
const naiveDateLocale = computed(() => (locale.value === 'en' ? dateEnUS : dateZhCN));

</script>

<style lang="less">
.course-shell {
    a:focus-visible, button:focus-visible { outline: 2px solid var(--course-accent); outline-offset: 3px; }
}
</style>
