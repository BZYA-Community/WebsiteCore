import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { test } from 'node:test';
import ts from 'typescript';
import { Input, BufferSource, MP4, Output, Mp4OutputFormat, BufferTarget, StreamTarget, Conversion } from 'mediabunny';

// Use the production listener and the real library, without a browser codec
// mock. Remuxing exercises the same target/cancellation/execute lifecycle.
const source = readFileSync(new URL('../src/utils/video-output-limit.ts', import.meta.url), 'utf8');
const js = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ESNext } }).outputText;
const { cancelOnVideoOutputLimit } = await import('data:text/javascript;base64,' + Buffer.from(js).toString('base64'));
const directory = mkdtempSync(join(tmpdir(), 'websitecore-video-limit-'));
let fixture;
try {
  const path = join(directory, 'source.mp4');
  execFileSync('ffmpeg', ['-hide_banner', '-loglevel', 'error', '-f', 'lavfi', '-i',
    'testsrc2=size=320x180:rate=30000/1001', '-t', '20', '-c:v', 'libx264', '-threads', '2', path]);
  fixture = readFileSync(path);
} finally {
  rmSync(directory, { recursive: true, force: true });
}

for (const storage of ['buffer', 'stream']) {
  test(storage + ': output overflow cancels before all frames are processed', async () => {
    const input = new Input({ source: new BufferSource(fixture), formats: [MP4] });
    const target = storage === 'buffer' ? new BufferTarget() : new StreamTarget(new WritableStream({
      write() {},
    }), { chunked: true, chunkSize: 1024 });
    const output = new Output({ format: new Mp4OutputFormat({ fastStart: false }), target });
    const conversion = await Conversion.init({ input, output, tracks: 'primary' });
    assert.equal(conversion.isValid, true);
    const limit = Math.floor(fixture.length / 4);
    const sizeError = new Error('output too large');
    let cancellations = 0;
    let cancellation;
    let progress = 0;
    const getError = cancelOnVideoOutputLimit(target, limit, () => {
      cancellations++;
      cancellation = conversion.cancel();
    }, () => sizeError);
    conversion.onProgress = value => { progress = value; };
    try {
      await assert.rejects(conversion.execute());
      await cancellation;
      assert.equal(getError(), sizeError);
      assert.equal(cancellations, 1);
      assert.equal(output.state, 'canceled');
      assert.ok(progress < 0.75, 'conversion consumed the whole source');
      if (storage === 'buffer') assert.equal(target.buffer, null, 'oversized output was finalized');
    } finally {
      await conversion.cancel();
      input.dispose();
    }
  });
}

test('exact output limit succeeds, including final metadata writes', async () => {
  const run = async limit => {
    const input = new Input({ source: new BufferSource(fixture), formats: [MP4] });
    const target = new BufferTarget();
    const output = new Output({ format: new Mp4OutputFormat({ fastStart: false }), target });
    const conversion = await Conversion.init({ input, output, tracks: 'primary', tags: {} });
    const error = cancelOnVideoOutputLimit(target, limit, () => {
      assert.fail('canceled a file at or below its limit');
    }, () => new Error('too large'));
    try {
      await conversion.execute();
      assert.equal(error(), undefined);
      return target.buffer.byteLength;
    } finally { input.dispose(); }
  };
  const size = await run(Infinity);
  assert.equal(await run(size), size);
});

test('a user cancellation does not become a size error', async () => {
  const input = new Input({ source: new BufferSource(fixture), formats: [MP4] });
  const target = new BufferTarget();
  const output = new Output({ format: new Mp4OutputFormat({ fastStart: false }), target });
  const conversion = await Conversion.init({ input, output, tracks: 'primary' });
  const error = cancelOnVideoOutputLimit(target, Infinity, () => {}, () => new Error('too large'));
  try {
    await conversion.cancel();
    await assert.rejects(conversion.execute());
    assert.equal(error(), undefined);
  } finally { input.dispose(); }
});
