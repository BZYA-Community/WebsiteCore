# SMS Platform Configuration

## What SMS is used for

WebsiteCore sends SMS verification codes for **phone number binding only**: a logged-in user proves ownership of a phone number, which is then stored on their account. There is currently no phone-number login; authentication is username/password based.

The feature is optional. If it is not enabled, the phone-binding UI still appears but any verification code is accepted — so on a public deployment either configure SMS properly or disable phone binding (`WebProfile.AllowPhoneBind`, bootstrap-only) entirely.

## Provider

The only built-in provider is **Juhe SMS (聚合数据)**, a Chinese SMS aggregator. The provider is selected by the `sms` configuration value and implemented in `internal/dao/security/phone_verify_juhe.go`; the factory falls back to Juhe for any unknown provider name.

If you need another provider (Aliyun, Tencent Cloud, ...), implement the `core.PhoneVerifyService` interface alongside the Juhe implementation and register it in `internal/dao/security/security.go`.

## Enabling SMS

1. Add the `Sms` feature to your suite:

   ```yaml
   Features:
     Default: ["Web", "Frontend:EmbedWeb", "Meili", "LocalOSS", "Postgres", "BigCacheIndex", "LoggerFile", "Sms"]
   ```

2. Configure the `SmsJuhe` section:

   ```yaml
   SmsJuhe:
     Gateway: https://v.juhe.cn/sms/send   # Juhe send endpoint; defaults to this value
     Key: "<your Juhe API key>"
     TplID: "<approved template ID>"
     TplVal: "#code#=%s&#m#=%d"            # template variable format: code, then expiry minutes
   ```

3. Register at [juhe.cn](https://www.juhe.cn/), apply for an SMS template, and wait for the template to be approved. `TplID` is the numeric ID of the approved template; `TplVal` must match the template's variables (`#code#` receives the 6-digit code, `#m#` the expiry in minutes).

The `Key` is a secret. Keep it out of version control; if you edit it through the admin UI it is stored encrypted (see below).

## Managing SMS settings from the admin UI

All four values are editable at `/#/admin/settings` under **notifications → sms_juhe** (`sms_juhe.gateway`, `sms_juhe.key`, `sms_juhe.tpl_id`, `sms_juhe.tpl_val`). The section only appears when the `Sms` feature is enabled.

- `sms_juhe.key` is a secret field: it is written to the database as AES-GCM ciphertext (`enc:v1:` prefix), encrypted with `AdminSettings.EncryptionKey`.
- Apply mode is `restart_required`: changes are persisted immediately but take effect after the next process restart.

## How a verification round works

```text
user requests captcha image      GET  captcha (6-digit PNG, stored in Redis, 5 min TTL)
        |
user submits phone + captcha     POST send phone captcha
        |
  image captcha checked -> per-phone daily counter checked (max 10/day, Redis)
        |
  Juhe API called (mobile, tpl_id, tpl_value, key) -> counter incremented
        |
user submits code                phone bind (authenticated)
        |
  code checked against latest Redis record (value / expiry / use count,
  max uses = App.MaxCaptchaTimes, default 2) -> phone stored on the account
```

Relevant knobs:

| Setting | Default | Where |
| --- | --- | --- |
| Daily SMS cap per phone number | 10 | constant in `internal/servants/web/pub.go` |
| Code validity | 5 minutes | Redis TTL |
| Max verification attempts per code | `App.MaxCaptchaTimes` (2) | admin UI: app → limits (`restart_required`) |
| Phone binding enabled | `WebProfile.AllowPhoneBind` | YAML only (bootstrap) |

A phone number can only be bound to one account; binding a number already used by another account is rejected.

## Compliance notes

This platform targets a youth community in mainland China. When enabling phone binding:

- Only collect phone numbers with a stated purpose and a privacy policy (data minimization is required by [../governance.md](../governance.md), section 6).
- SMS templates must be approved by the provider; do not send marketing content through verification templates.
- Keep the daily cap in place — it limits both cost and SMS-bombing abuse.
- Exporting or sharing user phone numbers requires maintainer approval.

## Troubleshooting

| Symptom | Cause / fix |
| --- | --- |
| Any code is accepted | The `Sms` feature is not enabled — verification is a no-op by design. |
| Juhe returns an error code | Check `Key` balance/validity and that `TplID` is approved and matches `TplVal`'s variables. |
| "Too many requests" for a phone | Daily cap (10) reached; it resets after 24 h. |
| Settings changed in admin UI have no effect | `sms_juhe.*` is `restart_required` — restart the process. |
| Admin UI shows no SMS section | `Sms` missing from `Features`. |
