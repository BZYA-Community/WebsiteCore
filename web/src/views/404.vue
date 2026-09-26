<template>
    <div>
        <main-nav title="404" />

        <n-list class="main-content-wrap wrap404" bordered>
            <n-result
                status="404"
                :title="t('post.notFound.resourceNotFound')"
                :description="t('post.notFound.description')"
            >
                <template #footer>
                    <n-button @click="goHome">{{ t('post.notFound.backHome') }}</n-button>
                </template>
            </n-result>
        </n-list>
    </div>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router';
import { useI18n } from 'vue-i18n';

const router = useRouter();
const { t } = useI18n();
const goHome = () => {
  router.push({
    path: '/',
  });
};
</script>

<style lang="less" scoped>
.wrap404 {
    min-height: 500px;
    display: flex;
    align-items: center;
    justify-content: center;
}
.dark {
    .main-content-wra {
        background-color: rgba(16, 16, 20, 0.75);
    }
}
</style>