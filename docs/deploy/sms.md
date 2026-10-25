# 阿里云短信配置

WebsiteCore 使用阿里云国内短信服务，为已登录用户发送手机号绑定验证码。当前不提供手机号注册或短信登录。手机号支持中国大陆 11 位号码，也接受 `+86` 前缀；同一号码不能绑定多个账户。

公开部署应在隐私政策中说明收集手机号的目的，只收集必要信息；不要将验证码模板用于营销，导出或共享用户手机号须经维护者批准。

## 开通阿里云服务

1. 在阿里云开通短信服务，确认账户余额或短信套餐充足。
2. 在短信控制台申请短信签名和验证码模板，完成审核及控制台要求的运营商报备。保存签名名称和形如 `SMS_123456789` 的模板 Code。
3. 模板只使用一个验证码变量 `${code}`，例如 `您的验证码为 ${code}，请勿向他人泄露。`。程序固定发送 JSON 字符串 `{"code":"012345"}`，保留验证码前导零；不提供自定义模板变量或过期分钟变量。若模板正文写明有效期，应与 `account_verify.code_ttl_seconds` 保持一致。
4. 创建专用 RAM 用户及 AccessKey，授予下述最小发送权限。不要使用阿里云主账号 AccessKey。

```json
{
  "Version": "1",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": "dysms:SendSms",
      "Resource": "*"
    }
  ]
}
```

`SendSms` 不支持资源级授权，因此这里使用 `Resource: "*"`。本服务只调用发送接口，不需要模板管理、短信查询或全部短信管理权限。官方参考：[SendSms 参数及授权信息](https://help.aliyun.com/zh/sms/developer-reference/api-dysmsapi-2017-05-25-sendsms)、[自定义权限策略](https://help.aliyun.com/zh/sms/custom-permission-policy-reference)、[首次调用 API 的准备工作](https://help.aliyun.com/zh/sms/developer-reference/how-to-use-api-quickly)。

## 最小配置

在实际使用的 `custom/config.yaml` 或 `config.yaml` 中配置短信。下面的示例同时将 `Sms` 加入现有功能列表，以显示后台短信设置分区；保留原有的其他功能：

```yaml
Features:
  Default: ["Web", "Frontend:EmbedWeb", "Meili", "LocalOSS", "Postgres", "BigCacheIndex", "LoggerFile", "Sms"]

App:
  RunMode: release

account_verify:
  mode: phone
  code_ttl_seconds: 300
  cooldown_seconds: 60
  max_attempts: 5
  max_daily_sends: 10
  dev_skip_code: false

SmsAliyun:
  AccessKeyID: "<RAM 用户 AccessKey ID>"
  AccessKeySecret: "<RAM 用户 AccessKey Secret>"
  SignName: "<审核通过的签名名称>"
  TemplateCode: "<审核通过的短信模板 Code>"
```

正常验证模式下，四项短信设置缺少任意一项，手机号验证均不可用；不会跳过验证码检查。`account_verify.mode: email` 使用邮件通道，阿里云短信设置不会自动切换验证方式。对外能力由 `/v1/site/profile` 的 `account_verify_mode` 和 `contact_verification_available` 提供，前端据此显示手机号验证入口。

`Sms` 功能标记控制后台短信设置分区的显示；通过 YAML 配置完整四项后，`phone` 模式的发送不依赖此标记。

保存后重启后端服务。YAML 中的密钥不会被自动加密，应保存在受限配置文件中并排除版本控制。后台已保存的设置在启动时覆盖同名 YAML 值，配置优先级见 [configuration.md](configuration.md)。

## 后台管理

启用 `Sms` 功能后，可在 `/#/admin/settings` 的 **notifications → sms_aliyun** 配置以下字段：

| 后台字段 | YAML 字段 | 含义 |
| --- | --- | --- |
| `sms_aliyun.access_key_id` | `SmsAliyun.AccessKeyID` | RAM 用户 AccessKey ID，密钥字段 |
| `sms_aliyun.access_key_secret` | `SmsAliyun.AccessKeySecret` | RAM 用户 AccessKey Secret，密钥字段 |
| `sms_aliyun.sign_name` | `SmsAliyun.SignName` | 短信签名名称 |
| `sms_aliyun.template_code` | `SmsAliyun.TemplateCode` | 验证码模板 Code |

AccessKey ID 和 Secret 均使用已有的 `AdminSettings.EncryptionKey` 加密，以 AES-GCM 密文（`enc:v1:` 前缀）存入数据库。先配置并妥善备份该加密密钥；更换它会使原有密文无法解密，届时需重新填写密钥字段。后台四项均为 `restart_required`：保存只写入数据库，**重启后端后才生效**。

## 验证和发送行为

用户登录后请求发送验证码，服务端校验当前验证通道和手机号，执行账户、手机号及 IP 的发送限额，再调用阿里云。验证码记录保存在 PostgreSQL；数据库只保存验证码摘要，并处理过期、尝试次数、最新记录及一次性消费。绑定成功后更新账户手机号。

| 设置 | 默认值 | 说明 |
| --- | --- | --- |
| `account_verify.code_ttl_seconds` | 300 秒 | 本地验证码有效期，范围 60–1800 秒 |
| `account_verify.cooldown_seconds` | 60 秒 | 同一账户或号码再次申请的冷却时间，范围 30–3600 秒 |
| `account_verify.max_attempts` | 5 次 | 单个验证码最大校验次数，范围 1–10 次 |
| `account_verify.max_daily_sends` | 10 次 | 账户或目标号码最近 24 小时的发送上限，范围 1–100 次；IP 上限为此值的 10 倍 |

服务固定通过 HTTPS 请求 `dysmsapi.aliyuncs.com` 的 `SendSms`（API 版本 `2017-05-25`），使用 ACS3-HMAC-SHA256 签名，发送超时为 10 秒，不自动重试。阿里云自身的流控仍会额外生效。

接口返回 `Code: OK` 只代表阿里云已受理请求，不能证明手机已收到短信。实际送达情况请到短信控制台的发送记录查看运营商回执，见 [短信发送 FAQ](https://help.aliyun.com/zh/sms/user-guide/message-delivery-faq)。

**超时或报错后不要连续点击重发。** 请求可能已经被阿里云受理；本地失败记录仍计入冷却和发送限额，且该记录的验证码不可用于绑定。先检查控制台发送记录，等待冷却结束后再按需申请，并使用最新一次验证码。

## 从旧短信方案升级

Juhe（聚合数据）适配已移除。旧 `SmsJuhe` YAML 配置、`sms_juhe.*` 后台值、旧 API Key 和模板设置不会迁移或沿用，也不会作为失败时的备用通道。

升级时将 `account_verify.mode` 设为 `phone`，填写上述四项阿里云配置并重启后端；使用后台管理时同时启用 `Sms`。再使用自有测试号码验证实际收件和绑定。未完成配置时服务声明手机号验证不可用，不会接受任意验证码。

仅本地开发可同时设置 `App.RunMode: debug` 和 `account_verify.dev_skip_code: true`，显式跳过邮件/短信发送并接受任意六位验证码；仍需先申请验证码以建立本地记录。非 `debug` 模式配置此开关会拒绝启动。生产环境必须保持 `dev_skip_code: false`。

## 常见问题

| 现象 | 检查项 |
| --- | --- |
| 后台没有短信分区 | 当前功能列表是否启用了 `Sms` |
| 手机号验证不可用 | 是否为 `phone` 模式，四项阿里云配置是否完整，保存后是否已重启 |
| 阿里云拒绝请求 | RAM 权限、AccessKey 是否有效、账户余额、签名和模板状态、`${code}` 是否匹配；同时检查系统时钟是否准确 |
| 显示已发送但手机未收到 | 在阿里云控制台查看发送记录和运营商回执，检查号码与终端拦截；不要立即重复发送 |
| 后台设置修改后无效 | 四项均需重启；同时确认后台值是否覆盖了 YAML |
| 提示请求过于频繁 | 等待本地冷却或最近 24 小时限额恢复，并检查阿里云额外流控 |
