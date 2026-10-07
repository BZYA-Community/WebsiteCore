import assert from 'node:assert/strict';
import { compressVideo, MAX_VIDEO_INPUT } from '../src/utils/compressVideo.ts';

await assert.rejects(compressVideo(new File([new Uint8Array(MAX_VIDEO_INPUT + 1)], 'large.mp4'), new AbortController().signal, () => {}), /videoSizeError/);
await assert.rejects(compressVideo(new File([], 'empty.mp4'), new AbortController().signal, () => {}), /videoSizeError/);

// Unsupported platforms must not hand the original File to an upload request.
globalThis.document = { createElement: () => ({}) };
await assert.rejects(compressVideo(new File(['video'], 'clip.mp4'), new AbortController().signal, () => {}), /compressionUnsupported/);

let captured = false;
globalThis.MediaRecorder = class { static isTypeSupported() { return true; } };
globalThis.document = { createElement: () => ({ captureStream() { captured = true; }, pause() {}, removeAttribute() {}, load() {} }) };
const controller = new AbortController();
controller.abort();
await assert.rejects(compressVideo(new File(['video'], 'clip.mp4'), controller.signal, () => {}), { name: 'AbortError' });
assert.equal(captured, false);
console.log('Video input limits, unsupported-browser rejection and cancellation checks passed.');
