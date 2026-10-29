<template>
  <div class="column-media">
    <n-space v-if="user.hasPermission('content.upload') && !disabled" align="center">
      <n-select v-model:value="uploadType" :options="types" :disabled="uploading" style="width: 150px" />
      <n-upload :action="gateway" :headers="headers" :data="{ type: uploadType }" :show-file-list="false"
        :disabled="uploading" :multiple="false" @before-upload="before" @finish="finish" @error="failed">
        <n-button :loading="uploading">{{ t('columns.upload') }}</n-button>
      </n-upload>
    </n-space>
    <n-alert v-if="processingVideo" type="info">{{ t('compose.compressionProgress') }} {{ videoProgress }}% <n-button @click="cancelVideo">{{ t('common.cancel') }}</n-button></n-alert>
    <ul v-if="modelValue.length" class="media-list">
      <li v-for="(item, index) in modelValue" :key="item.content">
        <a :href="item.content" target="_blank" rel="noopener noreferrer">{{ fileName(item.content) }}</a>
        <n-button v-if="!disabled" size="tiny" quaternary @click="remove(index)">{{ t('columns.remove') }}</n-button>
      </li>
    </ul>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { useStoreUser, TOKEN_KEY } from '@/store/user';
import { useMediaUpload } from '@/composables/useMediaUpload';
import type { UploadFileInfo } from 'naive-ui';
import type { ColumnMedia } from '@/api/columns';

const props = withDefaults(
  defineProps<{
    modelValue: ColumnMedia[];
    disabled?: boolean;
    imageOnly?: boolean;
  }>(),
  { disabled: false, imageOnly: false },
);
const emit = defineEmits<{
  (e: 'update:modelValue', items: ColumnMedia[]): void;
  (e: 'busy', busy: boolean): void;
}>();
const { t } = useI18n();
const user = useStoreUser();
const uploadType = ref('public/image');
const uploading = ref(false);
const types = computed(() =>
  props.imageOnly
    ? [{ label: t('columns.image'), value: 'public/image' }]
    : [
        { label: t('columns.image'), value: 'public/image' },
        { label: t('columns.video'), value: 'public/video' },
        { label: t('columns.attachment'), value: 'attachment' },
      ],
);
const headers = computed(() => ({
  Authorization: `Bearer ${localStorage.getItem(TOKEN_KEY) || ''}`,
}));
const gateway = import.meta.env.VITE_HOST + '/v1/attachment';
const { beforeUpload, processingVideo, videoProgress, cancelVideo } =
  useMediaUpload(uploadType);
function busy(value: boolean) {
  uploading.value = value;
  emit('busy', value);
}
async function before(data: {
  file: UploadFileInfo;
  fileList: UploadFileInfo[];
}) {
  if (props.disabled || uploading.value) return false;
  busy(true);
  try {
    const allowed = await beforeUpload(data);
    if (!allowed) busy(false);
    return allowed;
  } catch {
    busy(false);
    return false;
  }
}
function finish({ event }: { event?: ProgressEvent }) {
  try {
    const response = JSON.parse(
      (event?.target as XMLHttpRequest)?.responseText || '{}',
    );
    if (response.code !== 0 || !response.data?.content)
      throw new Error('upload');
    const item = {
      content: response.data.content,
      type:
        uploadType.value === 'public/image'
          ? 3
          : uploadType.value === 'public/video'
            ? 4
            : 7,
      sort: props.modelValue.length + 1,
    };
    emit(
      'update:modelValue',
      props.imageOnly ? [item] : [...props.modelValue, item],
    );
  } catch {
    window.$message.error(t('comment.upload.uploadFailed'));
  } finally {
    busy(false);
  }
}
function failed() {
  busy(false);
  window.$message.error(t('comment.upload.uploadFailed'));
}
function remove(index: number) {
  emit(
    'update:modelValue',
    props.modelValue.filter((_, i) => i !== index),
  );
}
function fileName(value: string) {
  try {
    return decodeURIComponent(value.split('/').pop() || value);
  } catch {
    return value;
  }
}
</script>

<style scoped>
.column-media { margin: 16px 0; }
.media-list { padding-left: 20px; overflow-wrap: anywhere; }
.media-list li { margin: 8px 0; }
.media-list a { color: var(--primary-color); }
</style>
