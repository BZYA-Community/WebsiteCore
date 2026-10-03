# Email verification with Alibaba Mail

WebsiteCore can verify an account by sending a one-time code from a deployment-controlled mailbox. Email is an optional alternative verified contact method, not a mandatory identifier for every account. Existing phone-bound accounts remain active during migration, while new users can verify a deployment-approved email address instead of providing a phone number.

## Alibaba Mail preparation

1. In the Alibaba Mail domain administrator console, create an OpenAPI application.
2. Grant only the mail-send permission (`Mail.Send.All`).
3. Record its `client_id` and `client_secret` outside version control.
4. Confirm that the configured sender is an active mailbox allowed to send through that OpenAPI application.

This adapter uses Alibaba Mail OAuth2 client credentials and the Alibaba Mail API two-step flow: create a draft, then send that draft. It does not use Alibaba Cloud DirectMail and does not open an SMTP connection.

- Authorization: <https://mailhelp.aliyun.com/docs/api/alibaba-mail-api>
- Create and send a draft: <https://mailhelp.aliyun.com/docs/guides/examples/send-draft>

## Configuration

Enable the `Email` feature and configure the mailbox application:

```yaml
Features:
  Default: ["Web", "Email", "Postgres", "Redis"]

AliMail:
  BaseURL: https://alimail-cn.aliyuncs.com
  ClientID: "<application client_id>"
  ClientSecret: "<application client_secret>"
  SenderEmail: sender@example.edu
  SenderName: WebsiteCore
  AllowedRecipientDomains: ["example.edu"]

WebProfile:
  AllowEmailBind: true
```

Never commit `ClientSecret`. Supply production credentials through the deployment's protected configuration or secret-management mechanism.

`AllowedRecipientDomains` is required when email binding is enabled and fails closed when empty. It accepts exact domains and their subdomains. Add only domains approved by the school; this prevents disposable mailbox services from being used for account verification. If the community later accepts another institutional mail provider, add that domain explicitly after review.

## Security behavior

- A graphical captcha is required before sending mail.
- Each email address can request at most 10 codes per day.
- Only configured institutional email domains are accepted; an empty allowlist rejects all recipients.
- Codes are six digits, generated with `crypto/rand`, and expire after five minutes.
- Incorrect attempts are counted atomically; a successful code is immediately exhausted.
- The OAuth access token is cached until shortly before expiry.
- Missing credentials fail closed: email binding cannot silently accept arbitrary codes.
- The public profile never returns an email address; it is returned only to the account owner.

The database migration adds `email` columns to users and captcha records. Run the normal WebsiteCore migration before enabling the feature.
