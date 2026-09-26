<template>
    <n-modal
        :show="show"
        preset="card"
        class="avatar-cropper-modal"
        style="width: 90vw; max-width: 520px"
        :title="t('setting.avatar.cropTitle')"
        :mask-closable="false"
        @update:show="handleClose"
    >
        <div class="cropper-wrap">
            <cropper
                ref="cropperRef"
                :src="src"
                :stencil-props="{ aspectRatio: 1 }"
                :canvas="{ maxWidth: 512, maxHeight: 512, imageSmoothingQuality: 'high' }"
                image-restriction="fitArea"
            />
        </div>
        <template #footer>
            <div class="cropper-footer">
                <n-button size="small" @click="handleClose(false)">
                    {{ t('setting.avatar.cropCancel') }}
                </n-button>
                <n-button size="small" type="primary" :loading="loading" @click="handleConfirm">
                    {{ t('setting.avatar.cropConfirm') }}
                </n-button>
            </div>
        </template>
    </n-modal>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { Cropper } from 'vue-advanced-cropper';
import 'vue-advanced-cropper/dist/style.css';

const { t } = useI18n();

defineProps<{
    show: boolean;
    src: string;
    loading?: boolean;
}>();

const emit = defineEmits<{
    (e: 'update:show', value: boolean): void;
    (e: 'confirm', blob: Blob): void;
}>();

const cropperRef = ref<InstanceType<typeof Cropper>>();

const handleClose = (value: boolean) => {
    emit('update:show', value);
};

const handleConfirm = () => {
    const result = cropperRef.value?.getResult();
    const canvas = result?.canvas;
    if (!canvas) {
        return;
    }
    // 输出统一为正方形 PNG(canvas 选项已限制最长边 512)
    canvas.toBlob(
        (blob) => {
            if (blob) {
                emit('confirm', blob);
            }
        },
        'image/png',
    );
};
</script>

<style lang="less" scoped>
.cropper-wrap {
    height: 400px;
    background: #f5f5f5;
}

.cropper-footer {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
}
</style>
