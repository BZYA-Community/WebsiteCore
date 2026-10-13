<template>
    <article class="system-notification" :class="{ dark }">
        <header>
            <a v-if="notice.profilePath" :href="notice.profilePath">{{ senderName }}</a>
            <span v-else>{{ senderName }}</span>
            <time :datetime="isoTime">{{ date }} {{ time }}</time>
        </header>
        <strong v-if="notice.brief">{{ notice.brief }}</strong>
        <p v-if="notice.content">{{ notice.content }}</p>
        <nav v-if="notice.profilePath || notice.postPath" :aria-label="t('message.chat.notificationActions')">
            <a v-if="notice.profilePath" :href="notice.profilePath">{{ t('message.chat.viewProfile') }}</a>
            <a v-if="notice.postPath" :href="notice.postPath">{{ t('message.chat.viewPost') }}</a>
        </nav>
    </article>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import type { SystemNotification } from '@/utils/systemNotification';

const props = defineProps<{
    notice: SystemNotification;
    date?: string;
    time?: string;
    dark: boolean;
}>();
const { t } = useI18n();
const senderName = computed(() => props.notice.senderName || t('message.chat.systemUser'));
const isoTime = computed(() => new Date(props.notice.timestamp * 1000).toISOString());
</script>

<style lang="less" scoped>
.system-notification {
    box-sizing: border-box;
    width: 100%;
    min-width: 0;
    padding: 16px;
    border: 1px solid #e0e0e6;
    border-radius: 8px;
    background: #fff;
    color: #333639;
    text-align: left;
    font-size: 14px;
    line-height: 1.6;
    overflow-wrap: anywhere;

    header {
        display: flex;
        flex-wrap: wrap;
        justify-content: space-between;
        gap: 4px 16px;
        margin-bottom: 8px;
    }

    time {
        color: #666;
        font-size: 12px;
    }

    p {
        margin: 8px 0;
        white-space: pre-wrap;
    }

    nav {
        display: flex;
        flex-wrap: wrap;
        gap: 8px 16px;
        margin-top: 12px;
    }

    a {
        color: #087f5b;
        text-decoration: underline;
        text-underline-offset: 3px;

        &:focus-visible {
            outline: 2px solid currentColor;
            outline-offset: 3px;
        }
    }

    &.dark {
        background: #242428;
        border-color: #48484e;
        color: #eee;

        time { color: #aaa; }
        a { color: #63e2b7; }
    }
}
</style>
