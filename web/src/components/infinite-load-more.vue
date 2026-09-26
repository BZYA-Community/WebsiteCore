<template>
    <n-space v-if="totalPage > 0" justify="center">
        <InfiniteLoading
            class="load-more"
            :slots="{ complete: displayCompleteText, error: loadErrorText }"
            @infinite="handleInfinite"
        >
            <template #spinner>
                <div class="load-more-wrap">
                    <n-spin :size="14" v-if="!noMore" />
                    <span class="load-more-spinner">{{ noMore ? displayCompleteText : loadMoreText }}</span>
                </div>
            </template>
        </InfiniteLoading>
    </n-space>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import InfiniteLoading from 'v3-infinite-loading';

const props = withDefaults(defineProps<{
    totalPage: number;
    noMore: boolean;
    completeText?: string;
}>(), {
    completeText: '',
});

const { t } = useI18n();

const displayCompleteText = computed(() => props.completeText || t('common.noMore'));
const loadMoreText = computed(() => t('message.loadMore'));
const loadErrorText = computed(() => t('message.loadError'));

const emit = defineEmits<{
    (e: 'load-more'): void;
}>();

const handleInfinite = () => {
    emit('load-more');
};
</script>

<style lang="less" scoped>
.load-more {
    margin: 20px;

    .load-more-wrap {
        display: flex;
        flex-direction: row;
        justify-content: center;
        align-items: center;
        gap: 14px;

        .load-more-spinner {
            font-size: 14px;
            opacity: 0.65;
        }
    }
}
</style>
