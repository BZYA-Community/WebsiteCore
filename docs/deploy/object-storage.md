# Object Storage Guide (LocalOSS / AliOSS)

All uploaded files — avatars, tweet attachments, course videos — live in an object storage backend. Exactly one backend is active, selected by the `Features` list:

| Backend | Feature flag | When to use |
| --- | --- | --- |
| `LocalOSS` (default) | `"LocalOSS"` | Single-server deployments; files stored on local disk and served by the app itself |
| AliOSS (Alibaba Cloud OSS) | `"AliOSS"` | Cloud deployments, CDN in front, large video files, browser-direct uploads |

If neither flag is set, `LocalOSS` is used automatically. All settings below are also editable in the admin UI (storage group); config keys are the source of truth at startup.

## How uploads flow

- **Backend-proxied** (no browser CORS involved): avatars generated at registration, tweet image/file attachments (`POST /v1/attachment/upload`). The browser uploads to the app; the app calls `PutObject` server-side.
- **Browser-direct** (requires bucket CORS): course video upload. With `AliOSS` enabled the backend issues a signed PostObject policy and the browser uploads **straight to your bucket**; with `LocalOSS` it falls back to proxy mode through `POST /v1/admin/course/video`.

## LocalOSS

```yaml
LocalOSS:
  SavePath: custom/data/paopao-ce/oss   # relative to the working directory; back this up
  Secure: false                          # true = object URLs use https
  Bucket: paopao                         # path segment in URLs, not a real bucket
  Domain: 127.0.0.1:8008                 # host the app is reachable at (no scheme)
```

Object URLs are built as `http(s)://{Domain}/oss/{Bucket}/{objectKey}` and served by the app on the `/oss/*` route. Security behavior of that route:

- directory requests always 404 (no listing of uploaded files);
- path traversal outside `SavePath` is rejected;
- objects under `attachment/` require a valid, unexpired HMAC-signed URL (issued by the backend via `SignURL`, 60 s validity for downloads) — they are not publicly accessible.

Make sure `SavePath` exists and is writable by the service user; include it in your backups (see [database.md](database.md), "Also back up").

## AliOSS

### 1. Prepare on the Alibaba Cloud side

| Item | Requirement |
| --- | --- |
| Bucket | Read/write ACL "public read, private write" (avatars and images must be publicly readable; `attachment/` objects stay protected by signed URLs) |
| AccessKey | Create a **RAM sub-account** with OSS permissions scoped to this bucket only — never use the root account's AK |
| Endpoint | Region endpoint, e.g. `oss-cn-beijing.aliyuncs.com`. If the app runs on an ECS in the same region, use the **internal** endpoint (`oss-cn-beijing-internal.aliyuncs.com`) — free traffic, lower latency |
| Domain | Public object domain: either the bucket's external domain `{bucket}.oss-cn-{region}.aliyuncs.com`, or a custom/CDN domain bound to the bucket in the OSS console (CNAME resolved) |

### 2. Configure the app

```yaml
Features:
  Default: [..., "AliOSS"]     # replaces LocalOSS

AliOSS:
  Endpoint: oss-cn-beijing.aliyuncs.com
  AccessKeyID: LTAI5t...
  AccessKeySecret: <secret>
  Bucket: my-bucket
  Domain: my-bucket.oss-cn-beijing.aliyuncs.com   # no scheme; URLs are always https
```

Object URLs are built as `https://{Domain}/{objectKey}` (AliOSS is always https). Direct video uploads target `https://{Bucket}.{Endpoint}` regardless of `Domain` (which may be a CDN domain).

### 3. Configure bucket CORS (required for course video direct upload)

Course videos are uploaded by the **browser** directly to the bucket, so the bucket must allow cross-origin requests. Without this, the browser console shows `blocked by CORS policy: No 'Access-Control-Allow-Origin' header...` and the upload never happens.

OSS console → Bucket → **Data Security → Cross-Origin Settings (CORS)** → create rule:

| Setting | Value |
| --- | --- |
| Allowed Origin | your site origin, one per line, exact scheme+host+port (e.g. `https://forum.example.com`; add `http://ip:8008` for a test deployment) |
| Allowed Method | GET, POST, PUT, DELETE, HEAD |
| Allowed Header | `*` |
| Exposed Header | `ETag` |
| Cache Time | 600 s |

Equivalent CLI (`aliyun oss cors --method put oss://my-bucket cors.xml`) with:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<CORSConfiguration>
  <CORSRule>
    <AllowedOrigin>https://forum.example.com</AllowedOrigin>
    <AllowedMethod>GET</AllowedMethod>
    <AllowedMethod>POST</AllowedMethod>
    <AllowedMethod>PUT</AllowedMethod>
    <AllowedMethod>DELETE</AllowedMethod>
    <AllowedMethod>HEAD</AllowedMethod>
    <AllowedHeader>*</AllowedHeader>
    <ExposeHeader>ETag</ExposeHeader>
    <MaxAgeSeconds>600</MaxAgeSeconds>
  </CORSRule>
</CORSConfiguration>
```

Origins must match **exactly** (scheme, host, port). `*` works for debugging but must not stay in production.

## Upload staging modes (sub-features)

Shared settings:

```yaml
ObjectStorage:
  RetainInDays: 2   # temp object expiry (Retention mode)
  TempDir: tmp      # temp object directory name (TempDir mode)
```

| Sub-feature | Behavior | Cleanup |
| --- | --- | --- |
| *(none — `OSS:Direct`, default)* | Every object is written to its final key immediately | none needed |
| `"OSS:TempDir"` | Unconfirmed objects are written under `{TempDir}/`; confirming moves (copy + delete) them to the final key | add an OSS **lifecycle rule** deleting prefix `{TempDir}/` after N days |
| `"OSS:Retention"` | Non-persistent objects get an `Expires` header `RetainInDays` out; persisting rewrites it to 2049 | add a lifecycle rule honoring expiry for the temp prefix |

The startup log tells you which mode is active (`use OSS:Direct feature` / `OSS:TempDir` / `OSS:Retention`).

## Troubleshooting

| Symptom | Cause / fix |
| --- | --- |
| Console: `blocked by CORS policy ... No 'Access-Control-Allow-Origin'` | Bucket has no CORS rule — see section 3. Verify with a preflight: `curl -i -X OPTIONS "https://{bucket}.{endpoint}/" -H "Origin: {your-origin}" -H "Access-Control-Request-Method: POST"` |
| Video upload returns `400` with `x-oss-ec: 0002-00000703` | PostObject form is missing/misplacing the signature. Required fields: `key`, `policy`, `OSSAccessKeyId`, `Signature` (V1 signature, capital S), `success_action_status`, and `file` **last** |
| Video upload returns `403 InvalidAccessKeyId` / `SignatureDoesNotMatch` | Wrong AK in config, or `AccessKeySecret` mismatched with the ID; RAM user lacks OSS write permission on the bucket |
| Uploaded images 403 when opened | Bucket ACL is private-read; set "public read, private write" or bind a CDN with proper auth |
| Avatar generation fails at registration (500, `avatar.Generate err` in logs) | Storage backend unreachable or not writable: LocalOSS `SavePath` missing/read-only, or AliOSS credentials/bucket wrong |
| Object URLs point at `127.0.0.1` in production | `LocalOSS.Domain` still the dev value — set it to the public domain (and `Secure: true` for https) |
| Direct upload offered but you expected proxy | Course video direct mode activates only with the `AliOSS` feature; `LocalOSS` deployments always proxy through the backend |

## Related

- [configuration.md](configuration.md) — full config reference
- [production.md](production.md) — production layout (LocalOSS behind Nginx)
- [public-launch.md](public-launch.md) — pre-launch checklist (https object URLs, backups)
