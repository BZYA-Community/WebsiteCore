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
import { computed, onMounted, onUnmounted, watch } from 'vue';
import { useStoreMain } from '@/store/main';
import { useStoreProfile } from '@/store/profile';
import { darkTheme, lightTheme, zhCN, enUS, dateZhCN, dateEnUS } from 'naive-ui';
import { storeToRefs } from 'pinia';
import { useI18n } from 'vue-i18n';
import { useRoute, useRouter } from 'vue-router';
import { resolveExternalWarningHref } from '@/utils/external-link';

const storeMain = useStoreMain();
const storeProfile = useStoreProfile();
const { theme, desktopModelShow } = storeToRefs(storeMain);
const { locale } = useI18n();
const route = useRoute();
const router = useRouter();
const isCourseRoute = computed(() => ['courses', 'course'].includes(String(route.name)));
const protectedAnchors = new WeakMap<HTMLAnchorElement, { original: string; protected: string }>();
let externalLinkObserver: MutationObserver | undefined;

function restoreAnchor(anchor: HTMLAnchorElement, saved?: { original: string; protected: string }) {
    if (saved && anchor.href === saved.protected) anchor.href = saved.original;
    protectedAnchors.delete(anchor);
}

function protectAnchor(anchor: HTMLAnchorElement) {
    const saved = protectedAnchors.get(anchor);
    if (anchor.hasAttribute('download')) {
        restoreAnchor(anchor, saved);
        return;
    }
    const original = saved && anchor.href === saved.protected ? saved.original : anchor.href;
    const warningHref = resolveExternalWarningHref(
        router,
        original,
        window.location.origin,
        storeProfile.profile.trustedExternalDomains,
    );
    if (!warningHref) {
        restoreAnchor(anchor, saved);
        return;
    }
    const protectedHref = new URL(warningHref, window.location.href).href;
    protectedAnchors.set(anchor, { original, protected: protectedHref });
    if (anchor.href !== protectedHref) anchor.href = warningHref;
}

function protectAnchors(root: ParentNode) {
    if (root instanceof HTMLAnchorElement) protectAnchor(root);
    root.querySelectorAll<HTMLAnchorElement>('a[href]').forEach(protectAnchor);
}

function protectEventAnchor(event: Event) {
    const anchor = event.composedPath().find(
        (target): target is HTMLAnchorElement => target instanceof HTMLAnchorElement,
    );
    if (anchor) protectAnchor(anchor);
}

function updateLayout() {
    storeMain.triggerCollapsedLeft(window.innerWidth <= 821);
    storeMain.triggerCollapsedRight(window.innerWidth <= 1140);
}
onMounted(() => {
    updateLayout();
    window.addEventListener('resize', updateLayout);
    protectAnchors(document);
    externalLinkObserver = new MutationObserver((records) => {
        for (const record of records) {
            if (record.type === 'attributes') {
                if (record.target instanceof HTMLAnchorElement) protectAnchor(record.target);
                continue;
            }
            record.addedNodes.forEach((node) => {
                if (node instanceof Element) protectAnchors(node);
            });
        }
    });
    externalLinkObserver.observe(document.body, {
        subtree: true,
        childList: true,
        attributes: true,
        attributeFilter: ['href'],
    });
    for (const event of ['pointerdown', 'focusin', 'contextmenu', 'dragstart', 'click', 'auxclick']) {
        document.addEventListener(event, protectEventAnchor, true);
    }
});
onUnmounted(() => {
    window.removeEventListener('resize', updateLayout);
    externalLinkObserver?.disconnect();
    for (const event of ['pointerdown', 'focusin', 'contextmenu', 'dragstart', 'click', 'auxclick']) {
        document.removeEventListener(event, protectEventAnchor, true);
    }
});
watch(() => storeProfile.profile.trustedExternalDomains, () => protectAnchors(document), { deep: true });

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
