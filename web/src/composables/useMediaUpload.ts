import { nextTick, onBeforeUnmount, ref } from 'vue';
import type { MessageReactive, UploadFileInfo } from 'naive-ui';
import { checkSourceSize, compressImage, compressVideo } from '@/utils/media-upload';
import { isZipFile } from '@/utils/isZipFile';
import i18n from '@/locales';

// Naive UI awaits onBeforeUpload before placing the File into FormData.
export const useMediaUpload = () => {
  const processing = ref(0);
  const controller = new AbortController();
  onBeforeUnmount(() => controller.abort());
  const prepareUpload = async (data: { file: UploadFileInfo }, kind: string): Promise<boolean> => {
    const file = data.file.file;
    if (!file) return false;
    processing.value++;
    let notice: MessageReactive | undefined;
    try {
      checkSourceSize(file, kind);
      notice = window.$message.loading(i18n.global.t("media.processing"), { duration: 0 });
      await nextTick();
      let output: File;
      if (kind === 'attachment') {
        if (!(await isZipFile(file))) throw new Error(i18n.global.t('compose.attachmentFormatError'));
        output = file;
      } else if (kind === 'public/video') {
        // This path is at most 30 MiB and uses the in-memory target.
        output = (await compressVideo(file, kind, { signal: controller.signal })).file;
      } else {
        output = await compressImage(file, kind, { signal: controller.signal });
      }
      controller.signal.throwIfAborted();
      data.file.file = output;
      data.file.name = output.name;
      data.file.type = output.type;
      return true;
    } catch (error) {
      if (!controller.signal.aborted) window.$message.error(error instanceof Error ? error.message : i18n.global.t('media.failed'));
      return false;
    } finally {
      notice?.destroy();
      processing.value--;
    }
  };
  return { processing, prepareUpload };
};
