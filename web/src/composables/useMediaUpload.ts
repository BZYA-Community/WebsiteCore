import { onBeforeUnmount, ref, type Ref } from 'vue';
import { useI18n } from 'vue-i18n';
import type { UploadFileInfo } from 'naive-ui';
import { compressVideo, MAX_ATTACHMENT_SIZE, MAX_VIDEO_INPUT } from '@/utils/compressVideo';
import { isZipFile } from '@/utils/isZipFile';
import { useStoreProfile } from '@/store/profile';

export function useMediaUpload(uploadType: Ref<string>) {
  const { t } = useI18n();
  const storeProfile = useStoreProfile();
  const processingVideo = ref(false);
  const videoProgress = ref(0);
  let controller: AbortController | undefined;
  const cancelVideo = () => controller?.abort();
  onBeforeUnmount(cancelVideo);
  async function beforeUpload(data: { file: UploadFileInfo }) {
    const file = data.file.file;
    if (!file) return false;
    const type = uploadType.value;
    const limits = storeProfile.profile.uploadLimits;
    const outputLimit = Math.min(limits.attachment_max_bytes, MAX_ATTACHMENT_SIZE);
    const inputLimit = Math.min(limits.video_input_max_bytes, MAX_VIDEO_INPUT);
    try {
      if (!file.size) throw new Error('emptyFileError');
      if (type === 'public/video') {
        if (processingVideo.value) throw new Error('compressionBusy');
        if (file.size > inputLimit) throw new Error('videoSizeError');
        if (!['video/mp4', 'video/quicktime', 'video/webm'].includes(file.type)) throw new Error('videoFormatError');
        processingVideo.value = true; videoProgress.value = 0; controller = new AbortController();
        try {
          const result = await compressVideo(file, controller.signal, (value) => { videoProgress.value = value; }, outputLimit);
          data.file.file = result; data.file.name = result.name; data.file.type = result.type;
        } finally { processingVideo.value = false; controller = undefined; }
      } else {
        if (file.size > outputLimit) throw new Error('attachmentSizeError');
        if (type === 'public/image' && !['image/webp', 'image/png', 'image/jpeg', 'image/gif'].includes(file.type)) throw new Error('imageFormatError');
        if (type === 'attachment' && !(await isZipFile(file))) throw new Error('attachmentFormatError');
      }
      return true;
    } catch (error) {
      const messages = {
        emptyFileError: t('compose.emptyFileError'), compressionBusy: t('compose.compressionBusy'),
        videoSizeError: t('compose.videoSizeError', { limit: inputLimit / 1024 ** 2 }), videoFormatError: t('compose.videoFormatError'),
        attachmentSizeError: t('compose.attachmentSizeError', { limit: outputLimit / 1024 ** 2 }), imageFormatError: t('compose.imageFormatError'),
        attachmentFormatError: t('compose.attachmentFormatError'), compressionUnsupported: t('compose.compressionUnsupported'),
        compressionFailed: t('compose.compressionFailed'), compressedTooLarge: t('compose.compressedTooLarge', { limit: outputLimit / 1024 ** 2 }),
      };
      if (!(error instanceof DOMException && error.name === 'AbortError')) window.$message.error(messages[error instanceof Error ? error.message as keyof typeof messages : 'compressionFailed'] || messages.compressionFailed);
      return false;
    }
  }
  return { beforeUpload, processingVideo, videoProgress, cancelVideo };
}
