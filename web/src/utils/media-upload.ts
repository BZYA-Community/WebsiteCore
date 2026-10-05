import i18n from '@/locales';
import imageWorkerURL from 'browser-image-compression/dist/browser-image-compression.js?url';
import { cancelOnVideoOutputLimit } from './video-output-limit';

export const IMAGE_SOURCE_LIMIT = 5 * 1024 * 1024;
export const VIDEO_SOURCE_LIMIT = 30 * 1024 * 1024;
export const COURSE_IMAGE_SOURCE_LIMIT = 60 * 1024 * 1024;
export const COURSE_VIDEO_SOURCE_LIMIT = 1024 * 1024 * 1024;
export const ATTACHMENT_LIMIT = 5 * 1024 * 1024;
export const IMAGE_MIME_TYPES = ['image/png', 'image/jpeg', 'image/jpg'];
export type UploadKind = 'public/image' | 'public/avatar' | 'public/course-image' | 'public/video' | 'course/video' | 'attachment';
export type ProcessingOptions = { signal?: AbortSignal; onProgress?: (percent: number) => void };
export type PreparedMedia = { file: File; dispose: () => Promise<void> };

const t = i18n.global.t;
export const uploadLimit = (kind: string): number => {
  if (kind === 'course/video') return COURSE_VIDEO_SOURCE_LIMIT;
  if (kind === 'public/course-image') return COURSE_IMAGE_SOURCE_LIMIT;
  if (kind === 'public/video') return VIDEO_SOURCE_LIMIT;
  return ATTACHMENT_LIMIT;
};
export const uploadAccept = (kind: string): string => {
  if (kind === 'public/video' || kind === 'course/video') return '.mp4,video/mp4';
  if (kind === 'attachment') return '.zip,application/zip';
  return '.png,.jpg,.jpeg,image/png,image/jpeg';
};
export const checkSourceSize = (file: File, kind: string): void => {
  const limit = uploadLimit(kind);
  if (!file.size || file.size > limit) {
    throw new Error(t('media.sourceSize', { limit: limit / 1024 / 1024 }));
  }
};
const checkOutputSize = (file: Blob, kind: string): void => {
  if (!file.size || file.size > uploadLimit(kind)) {
    throw new Error(t('media.outputSize', { limit: uploadLimit(kind) / 1024 / 1024 }));
  }
};
const outputName = (file: File, ext: string) => file.name.replace(/\.[^.]*$/, '') + ext;

// One image at a time avoids simultaneously decoding nine large camera photos.
let imageQueue: Promise<unknown> = Promise.resolve();
export const compressImage = (file: File, kind: string, options: ProcessingOptions = {}): Promise<File> => {
  // Original bytes are checked before importing a codec or decoding a pixel.
  checkSourceSize(file, kind);
  if (!IMAGE_MIME_TYPES.includes(file.type) || !/\.(png|jpe?g)$/i.test(file.name)) {
    throw new Error(t('media.imageFormat'));
  }
  const run = imageQueue.then(async () => {
    options.signal?.throwIfAborted();
    const header = new Uint8Array(await file.slice(0, 8).arrayBuffer());
    const png = [137, 80, 78, 71, 13, 10, 26, 10].every((byte, i) => header[i] === byte);
    const jpeg = header[0] === 255 && header[1] === 216 && header[2] === 255;
    if (!png && !jpeg) throw new Error(t('media.imageFormat'));
    const { default: compress } = await import('browser-image-compression');
    const result = await compress(file, {
      maxWidthOrHeight: 1920,
      maxSizeMB: uploadLimit(kind) / 1024 / 1024,
      fileType: 'image/webp',
      initialQuality: 0.95,
      maxIteration: 1,
      alwaysKeepResolution: true,
      useWebWorker: true,
      libURL: new URL(imageWorkerURL, location.href).href,
      preserveExif: false,
      signal: options.signal,
      onProgress: options.onProgress,
    });
    options.signal?.throwIfAborted();
    if (result.type !== 'image/webp') throw new Error(t('media.unsupportedImage'));
    checkOutputSize(result, kind);
    return new File([result], outputName(file, '.webp'), { type: 'image/webp' });
  });
  imageQueue = run.catch(() => {});
  return run;
};

export const compressVideo = async (file: File, kind: string, options: ProcessingOptions = {}): Promise<PreparedMedia> => {
  checkSourceSize(file, kind);
  if (!/\.mp4$/i.test(file.name)) throw new Error(t('media.videoFormat'));
  options.signal?.throwIfAborted();
  if (!('VideoEncoder' in globalThis) || !('VideoDecoder' in globalThis)) {
    throw new Error(t('media.unsupportedVideo'));
  }
  const { Input, BlobSource, MP4, Output, Mp4OutputFormat, BufferTarget, StreamTarget, Conversion, QUALITY_HIGH } = await import('mediabunny');
  const input = new Input({ source: new BlobSource(file), formats: [MP4] });
  let dispose = async () => {};
  let abortOutput = async () => {};
  let conversion: Awaited<ReturnType<typeof Conversion.init>> | undefined;
  let cancellation: Promise<void> | undefined;
  let outputError: () => Error | undefined = () => undefined;
  const cancel = () => { cancellation ??= conversion?.cancel().catch(() => {}); };
  options.signal?.addEventListener('abort', cancel, { once: true });
  try {
    const video = await input.getPrimaryVideoTrack();
    const audio = await input.getPrimaryAudioTrack();
    if (!video) throw new Error(t('media.videoFormat'));
    const width = await video.getDisplayWidth();
    const height = await video.getDisplayHeight();
    if (width < 2 || height < 2) throw new Error(t('media.videoFormat'));
    const scale = Math.min(1, 1920 / Math.max(width, height), 1080 / Math.min(width, height));
    const outWidth = Math.max(2, Math.floor(width * scale / 2) * 2);
    const outHeight = Math.max(2, Math.floor(height * scale / 2) * 2);
    // Ordinary videos are bounded to 30 MiB. Course sources write to
    // private browser storage, keeping RAM usage independent of the file size.
    let target: InstanceType<typeof BufferTarget> | InstanceType<typeof StreamTarget>;
    let readResult: () => Promise<Blob>;
    if (kind === 'course/video') {
      if (!navigator.storage?.getDirectory) throw new Error(t('media.storageUnavailable'));
      const directory = await navigator.storage.getDirectory();
      const name = 'websitecore-media-' + crypto.randomUUID() + '.mp4';
      const handle = await directory.getFileHandle(name, { create: true });
      dispose = async () => { await directory.removeEntry(name); };
      const writable = await handle.createWritable();
      abortOutput = async () => { await writable.abort().catch(() => {}); };
      target = new StreamTarget(new WritableStream({
        async write(chunk) {
          if (chunk.position + chunk.data.byteLength > uploadLimit(kind)) {
            throw new Error(t('media.outputSize', { limit: uploadLimit(kind) / 1024 / 1024 }));
          }
          await writable.write(chunk);
        },
        close: () => writable.close(),
        abort: () => abortOutput(),
      }), { chunked: true, chunkSize: 1024 * 1024 });
      readResult = () => handle.getFile();
    } else {
      const buffer = new BufferTarget();
      target = buffer;
      readResult = async () => new Blob([buffer.buffer!], { type: 'video/mp4' });
    }
    outputError = cancelOnVideoOutputLimit(target, uploadLimit(kind), cancel,
      () => new Error(t('media.outputSize', { limit: uploadLimit(kind) / 1024 / 1024 })));
    const output = new Output({ format: new Mp4OutputFormat({ fastStart: false }), target });
    conversion = await Conversion.init({
      input, output, tracks: 'primary', tags: {}, showWarnings: false,
      video: {
        codec: 'avc', width: outWidth, height: outHeight, fit: 'contain',
        frameRate: 30000 / 1001, quality: QUALITY_HIGH,
        forceTranscode: true, allowTransformationMetadata: false,
      },
    });
    if (!conversion.isValid || conversion.discardedTracks.some(({ track }) => track === video || track === audio)) {
      throw new Error(t('media.unsupportedVideo'));
    }
    options.signal?.throwIfAborted();
    conversion.onProgress = (value) => options.onProgress?.(Math.round(value * 100));
    await conversion.execute();
    if (outputError()) throw outputError();
    options.signal?.throwIfAborted();
    const result = await readResult();
    checkOutputSize(result, kind);
    return { file: new File([result], outputName(file, '.mp4'), { type: 'video/mp4' }), dispose };
  } catch (error) {
    cancel();
    await cancellation;
    await abortOutput();
    await dispose().catch(() => {});
    throw outputError() ?? error;
  } finally {
    options.signal?.removeEventListener('abort', cancel);
    input.dispose();
  }
};
