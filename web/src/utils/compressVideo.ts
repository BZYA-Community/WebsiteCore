export const MAX_VIDEO_INPUT = 30 * 1024 ** 2;
export const MAX_ATTACHMENT_SIZE = 15 * 1024 ** 2;

/** Re-encode locally; unsupported browsers must never silently upload the source. */
export async function compressVideo(file: File, signal: AbortSignal, progress: (value: number) => void, maxOutputBytes = MAX_ATTACHMENT_SIZE): Promise<File> {
  if (!file.size || file.size > MAX_VIDEO_INPUT) throw new Error('videoSizeError');
  const video = document.createElement('video') as HTMLVideoElement & { captureStream?: () => MediaStream };
  if (typeof MediaRecorder === 'undefined' || !video.captureStream) throw new Error('compressionUnsupported');
  maxOutputBytes = Math.min(maxOutputBytes, MAX_ATTACHMENT_SIZE);
  const mime = ['video/mp4;codecs=avc1.42E01E,mp4a.40.2', 'video/mp4', 'video/webm;codecs=vp8,opus', 'video/webm;codecs=vp9,opus'].find((type) => MediaRecorder.isTypeSupported(type));
  if (!mime) throw new Error('compressionUnsupported');
  const source = URL.createObjectURL(file);
  let stream: MediaStream | undefined;
  let recorder: MediaRecorder | undefined;
  let timer: ReturnType<typeof setTimeout> | undefined;
  let onAbort: (() => void) | undefined;
  video.muted = true; video.playsInline = true; video.preload = 'auto';
  try {
    await new Promise<void>((resolve, reject) => {
      const fail = () => reject(new Error('compressionFailed'));
      onAbort = () => reject(new DOMException('Cancelled', 'AbortError'));
      signal.addEventListener('abort', onAbort, { once: true });
      if (signal.aborted) return onAbort();
      timer = setTimeout(fail, 15000);
      video.onloadeddata = () => resolve(); video.onerror = fail;
      video.src = source;
    });
    clearTimeout(timer); signal.removeEventListener('abort', onAbort!);
    // Browser-recorded WebM often omits duration; seeking reads its final timestamp.
    if (video.duration === Infinity) {
      const seek = async (time: number) => {
        await new Promise<void>((resolve, reject) => {
          onAbort = () => reject(new DOMException('Cancelled', 'AbortError'));
          signal.addEventListener('abort', onAbort, { once: true });
          if (signal.aborted) return onAbort();
          timer = setTimeout(() => reject(new Error('compressionFailed')), 15000);
          video.onseeked = () => resolve();
          video.currentTime = time;
        });
        clearTimeout(timer); signal.removeEventListener('abort', onAbort!);
      };
      await seek(Number.MAX_VALUE);
      await seek(0);
    }
    if (!Number.isFinite(video.duration) || video.duration <= 0 || !video.videoWidth) throw new Error('compressionFailed');
    stream = video.captureStream();
    if (!stream.getVideoTracks().length) throw new Error('compressionUnsupported');
    const targetBits = Math.min(file.size * 0.75, maxOutputBytes * 0.93) * 8 / video.duration;
    recorder = new MediaRecorder(stream, { mimeType: mime, videoBitsPerSecond: Math.max(32000, Math.min(2000000, targetBits - 64000)), audioBitsPerSecond: 64000 });
    const compressed = await new Promise<Blob>((resolve, reject) => {
      const chunks: Blob[] = [];
      let bytes = 0;
      const fail = () => reject(new Error('compressionFailed'));
      onAbort = () => reject(new DOMException('Cancelled', 'AbortError'));
      signal.addEventListener('abort', onAbort, { once: true });
      if (signal.aborted) return onAbort();
      timer = setTimeout(fail, video.duration * 1500 + 15000);
      recorder!.ondataavailable = (event) => {
        bytes += event.data.size;
        if (bytes > maxOutputBytes) reject(new Error('compressedTooLarge'));
        else if (event.data.size) chunks.push(event.data);
      };
      recorder!.onerror = fail;
      recorder!.onstop = () => resolve(new Blob(chunks, { type: recorder!.mimeType }));
      video.onerror = fail;
      video.ontimeupdate = () => progress(Math.min(99, Math.round(video.currentTime * 100 / video.duration)));
      video.onended = () => { if (recorder?.state !== 'inactive') recorder?.stop(); };
      recorder!.start(1000);
      void video.play().catch(fail);
    });
    if (!compressed.size || compressed.size > maxOutputBytes) throw new Error('compressedTooLarge');
    progress(100);
    const extension = compressed.type.startsWith('video/mp4') ? 'mp4' : 'webm';
    return new File([compressed], file.name.replace(/\.[^.]+$/, '') + '.' + extension, { type: compressed.type.split(';')[0] });
  } finally {
    clearTimeout(timer);
    if (onAbort) signal.removeEventListener('abort', onAbort);
    video.onloadeddata = null; video.onseeked = null; video.onerror = null; video.ontimeupdate = null; video.onended = null;
    video.pause(); video.removeAttribute('src'); video.load();
    if (recorder && recorder.state !== 'inactive') recorder.stop();
    stream?.getTracks().forEach((track) => track.stop());
    URL.revokeObjectURL(source);
  }
}
