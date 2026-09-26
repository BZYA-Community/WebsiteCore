<template>
    <n-popover trigger="click" placement="top-end">
        <template #trigger>
            <n-button quaternary circle :size="size">
                <template #icon>
                    <n-icon :size="iconSize">
                        <language-outline />
                    </n-icon>
                </template>
            </n-button>
        </template>
        <div class="lang-pop">
            <div class="lang-pop-title">{{ t('sidebar.language') }}</div>
            <n-button
                v-for="opt in localeOptions"
                :key="opt.value"
                quaternary
                size="small"
                block
                :type="locale === opt.value ? 'primary' : 'default'"
                @click="handleSetLocale(opt.value)"
            >
                {{ opt.label }}
            </n-button>
        </div>
    </n-popover>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { LanguageOutline } from '@vicons/ionicons5';
import { localeOptions, setLocale, type SupportedLocale } from '@/locales';

withDefaults(
    defineProps<{
        size?: 'tiny' | 'small' | 'medium' | 'large';
        iconSize?: number;
    }>(),
    {
        size: 'tiny',
        iconSize: undefined,
    },
);

const { t, locale } = useI18n();

function handleSetLocale(loc: SupportedLocale) {
    setLocale(loc);
}
</script>

<style lang="less">
.lang-pop {
    min-width: 120px;

    .lang-pop-title {
        font-size: 12px;
        opacity: 0.6;
        margin-bottom: 4px;
    }
}
</style>
