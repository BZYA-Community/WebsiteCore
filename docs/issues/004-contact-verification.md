# Exclusive account-contact verification

Status: implemented and locally verified with synthetic senders. Real provider delivery remains a deployment check.

`account_verify.mode` selects exactly `email` (default) or `phone`. Verifying that channel gives automatic Member identity; the other channel does not grant membership. Registration remains possible before verification. Operators remain independent of membership.

Authenticated `POST /v1/user/contact/code` accepts `{mode,address}`. `POST /v1/user/contact/verify` accepts `{mode,address,code}`. These account-bootstrap operations require a current active login, so editing Guest permissions cannot accidentally prevent verification. The former anonymous SMS-send endpoint is disabled. The old phone-bind endpoint delegates to the same secure verifier and is denied in email mode.

Codes use cryptographic randomness, expire, allow a bounded number of guesses, and can be consumed once. Only keyed hashes are stored. PostgreSQL serializes verification and throttles sends by account, address and IP fingerprint; failures retain cooldowns. A code is unusable until the provider confirms delivery. Binding and the operation log commit together. Unique indexes prevent binding the same active contact to two accounts.

The default provider follows the requested [Alibaba Mail OAuth API](https://help.aliyun.com/en/document_detail/2855420.html) and [draft/send API](https://help.aliyun.com/zh/document_detail/2856076.html). Configure the sender mailbox, app client ID and client secret in `account_verify.aliyun`; give the app only the required mail-draft/send scopes. Token caching, HTTPS, timeouts and redirect rejection are built in. Provider errors never return credentials, token or response bodies to clients. No test sends real mail.

Phone mode retains the existing Juhe provider and its mainland-China number support. Email mode supports international users. `site/profile` exposes only mode, availability and upload/module settings; secrets stay on the server.

## Verification

- Provider contract test uses an in-memory HTTP transport: OAuth form, cached token, draft payload, sending and provider failure.
- PostgreSQL tests cover concurrent single use, wrong-attempt exhaustion, expiry, resend throttling, duplicate contacts and inactive-channel denial.
- Live browser checks confirmed that the real email-mode profile displays email verification and hides phone binding. The settings page fit 375, 821, 1000, 1200, 1366, 1600, and 1920 pixel viewports.
- Separate browser response fixtures exercised both email and phone forms: the selected mode/address/code payload, resend countdown, binding submission, and refreshed verified session state. These fixtures intercepted delivery and verification responses; they sent no email or SMS and are not evidence of provider delivery.
- Live Aliyun/Juhe delivery requires deployment-owned test credentials and has not been exercised locally.

Apply migration 0031 after 0026. Duplicate development phone bindings must be resolved explicitly before its unique index can be created; no data is deleted to force a migration through. Rotating the JWT secret also invalidates outstanding verification-code hashes. Rolling back 0031 removes verified email data and verification history: export them before a deliberate downgrade.
