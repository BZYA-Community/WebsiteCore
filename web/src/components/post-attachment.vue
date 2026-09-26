<template>
    <div class="attachment-wrap">
        <div
            class="attach-item"
            v-for="attachment in attachments"
            :key="attachment.id"
        >
            <n-button
                @click.stop="download(attachment)"
                type="primary"
                size="tiny"
                dashed
            >
                <template #icon>
                    <n-icon>
                        <cloud-download-outline />
                    </n-icon>
                </template>
                {{ t('post.attachment') }}
            </n-button>
        </div>

        <!-- 删除确认 -->
        <n-modal
            v-model:show="showDownloadModal"
            :mask-closable="false"
            preset="dialog"
            :title="t('post.downloadTipTitle')"
            :content="downloadTip"
            :positive-text="t('post.action.confirmDownload')"
            :negative-text="t('common.cancel')"
            icon-placement="top"
            @positive-click="execDownloadAction"
        />
    </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { CloudDownloadOutline } from '@vicons/ionicons5';
import { Api } from '@/utils/request';

const { t } = useI18n();

withDefaults(
  defineProps<{
    attachments: Item.PostItemProps[];
  }>(),
  {
    attachments: () => [],
  },
);
const showDownloadModal = ref(false);
const downloadTip = ref<string>('');
const attachmentID = ref(0);

const download = (attachment: Item.PostItemProps) => {
  showDownloadModal.value = true;
  attachmentID.value = attachment.id;
  downloadTip.value = t('post.downloadTipContent');
};
const execDownloadAction = () => {
  Api.v1.attachment.get._self({
    id: attachmentID.value,
  })
    .then((res) => {
      window.open(res.signed_url.replace('http://', 'https://'), '_blank');
    })
    .catch((err) => {
      console.log(err);
    });
};
</script>

<style lang="less" scoped>
.attach-item {
    margin: 10px 0;
}
</style>
