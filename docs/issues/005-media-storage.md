# Bounded media uploads and verified course resources

Status: implemented and locally verified, including a complete 2 GiB LocalOSS HTTP upload. Real AliOSS deployment checks remain separate.

Configured limits can be lower than the product ceilings: normal attachments 15 MiB, course attachments 50 MiB, course resources 2 GiB, and ordinary video input 30 MiB. The browser compresses ordinary video before uploading, caps output at the normal attachment limit, and reports unsupported browser codecs instead of silently sending the source file. The backend checks size, declared MIME, filename extension and file signature; it does not transcode or execute uploaded files.

Trusted course uploaders use `POST /v1/admin/course/upload/init` with `{name,size,mime_type,kind}`. The response contains an attachment ID and a short-lived upload capability: AliOSS form POST or LocalOSS streaming PUT. Exact object key, MIME and size are bound to the capability. Uploaders finish with `POST /v1/admin/course/upload/complete`; only verified IDs can be attached to lessons. Ownership and current permissions are rechecked after upload.

Resources upload into an inaccessible staging prefix. Completion inspects the actual stored bytes and publishes a separate private object, so an unexpired upload capability cannot replace a verified lesson resource. Metadata and operation logs commit together; staging is kept after a failed database transaction for retry. Expiry maintenance removes abandoned uploads and failed promotions. Local uploads use bounded streaming and publish completed files atomically. Large AliOSS promotion uses server-side copying rather than buffering resources in application memory.

Supported types include JPEG/PNG/GIF/WebP, MP4/QuickTime/WebM, MP3/WAV/Ogg, PDF, ZIP, OpenXML documents, and plain text/CSV. Package other course data formats in ZIP. Header validation is not malware scanning; downloaded archives/documents remain attachments, with `nosniff` response headers. Resource URLs require short-lived signatures.

## Deployment

Set AliOSS CORS for the actual frontend origin and the required upload headers/form fields. Keep private resources private even if public images use the same bucket. See official [PostObject conditions](https://www.alibabacloud.com/help/en/oss/developer-reference/postobject) and [conditional copy](https://www.alibabacloud.com/help/en/oss/developer-reference/copyobject).

For LocalOSS, put storage on a filesystem supporting atomic hard links (for example NTFS or ext4), provision disk capacity, and allow the reverse proxy to stream up to the configured course resource limit. The signed LocalOSS route extends its own HTTP deadline to the capability expiry; normal routes retain normal timeouts. Do not expose the storage directory through a separate static-file server that bypasses signature checks.

Actual AliOSS provider traffic requires deployment credentials and is not implied by local/unit test results. Local streaming and upload-boundary checks are recorded during integration verification.

See [the complete verification record](../community-verification.md) and `scripts/test_course_uploads.py` for the HTTP boundary checks and reproduction commands.

## Frontend verification

- A real headless Edge run encoded a synthetic 3-second MP4 from 253,760 to 102,433 bytes. The output loaded, sought, and played to completion. A browser-recorded WebM with missing duration metadata also re-encoded successfully; active cancellation, unsupported-platform rejection, the 30 MiB input boundary, and a reduced output ceiling were checked. No dependency or server transcoder was added.
- The live browser/API workflow compressed a video, uploaded the resulting MP4 through actual header/MIME validation, created a pending post, displayed its full text and video in the review dialog, and approved it using the current task ID/revision. The completed task left the pending queue. Review dialogs fit all seven viewports from 375 to 1920 pixels.
- Live course-editor checks completed two signed LocalOSS uploads and bound only their verified attachment IDs. Browser capability-response fixtures separately reduced course attachment/resource and ordinary-video input limits; oversized files were rejected before initiating upload. Those fixtures validate frontend configuration handling, not a changed server configuration.
- `npm run test:permissions`, `npm run test:media`, and `npm run i18n:check` passed. Locale validation reported 775 keys, 624 references, no missing keys, and matching Chinese/English catalogs. Production Vite build passed; full ESLint reported 0 errors and 136 existing warnings. Browser evidence is stored locally under the ignored `web/node_modules/.cache/identity-qa/` directory.
- Local browser and LocalOSS checks do not verify real AliOSS CORS/provider credentials, email/SMS delivery, production load, or every browser's media codecs. Unsupported recording platforms fail explicitly before upload.
