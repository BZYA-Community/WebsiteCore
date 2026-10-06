# Community framework verification

Local verification on 2026-10-07 covers the five phases in [the development baseline](community-framework.md). The runtime is Go/Gin with PostgreSQL, Vue/Naive UI, and the existing OSS SDK. No new runtime dependency was added for verification, email delivery, video compression, or storage promotion.

## Results

| Area | Executed checks |
| --- | --- |
| RBAC | 70 HTTP checks and 47 browser checks from Phase 1; current database tests also verify live grants, independent operators, identity mutation logging, and serialized private-message policy |
| Courses and settings UI | 48 real browser checks, including nested categories, independent lessons, two direct uploads, signed downloads, retained attachment references, keyboard focus, and seven viewport widths; another 9 real checks cover read-only review and mandatory course Q&A review |
| Review | 83 real HTTP checks: author exclusion, assignment scope, read-only global access, revision rejection, timeout transfer, missed-review statistics, operator override, and immediate revocation |
| Review and video UI | 20 real browser checks: native compression, real validated upload, pending publication, full review content, task history, and approval |
| Contact verification and configuration UI | PostgreSQL tests with fake senders cover email/phone exclusivity, one-time consumption under concurrency, expiry, attempt exhaustion, throttling, failed delivery, duplicate binding, and operation-log rollback; 14 browser response-fixture checks cover contact modes, capability handling and lowered upload limits |
| Upload HTTP | 31 checks with a complete 2,147,483,648-byte LocalOSS HTTP stream; a separate 34-check run includes exactly 15 MiB accepted, one byte above rejected, and a forged image signature rejected |
| Storage | Actual NTFS atomic writes/promotion, symlink and Windows alias denial, signed HEAD/range; OSS SDK requests exercised through an in-process mock transport, including 16-part 2 GiB copy and ambiguous completion outcomes |
| Module disabled | 17 HTTP checks with a separate configuration: course routes and direct-upload route return 404, profile advertises the disabled module, community routes remain available |
| Migrations | All PostgreSQL up migrations applied in an isolated transaction/schema; new migrations exercised down/up; original course/upload/user records preserved where documented |

Browser checks use desktop Edge/Chromium and widths 1920, 1600, 1366, 1200, 1000, 821 and 375. Video checks include MP4 and WebM input, missing WebM duration metadata, cancellation, unsupported codecs, lowered size limits, finite progress, and playable output. Other browsers should run the same checks before being added to a supported-browser promise.

## Reproduction

Backend build, migration-tagged build, vet, golangci-lint and the full test suite passed. Frontend production build and permission/media tests passed; ESLint reported 0 errors and 136 pre-existing style warnings. Translation parity passed with 775 keys and 624 references. In total, 77 real API/browser checks and 14 intercepted-response browser checks passed; these are separate from the HTTP script counts above.

Run the repository's backend and frontend BVT from [CONTRIBUTING.md](../CONTRIBUTING.md). Set `TEST_POSTGRES_DSN` to a disposable PostgreSQL instance before `go test ./...`; otherwise database integration tests explicitly skip. These tests create isolated random tables or schemas and clean them up.

```sh
go build ./...
go vet ./...
golangci-lint run ./...
go test ./... -count=1

cd web
npm run lint
npm run test:permissions
npm run test:media
npm run i18n:check
npm run build
```

For HTTP upload checks, run a disposable LocalOSS server with courses enabled. Provide `TEST_OPERATOR_USERNAME` and `TEST_OPERATOR_PASSWORD` through environment variables. `TEST_API_BASE` defaults to `http://127.0.0.1:18008`; the script refuses non-local hosts.

```sh
python scripts/test_course_uploads.py
python scripts/test_course_uploads.py --large
```

The large test streams the entire 2 GiB payload using a bounded buffer. It is distinct from the storage unit test's logically 2 GiB file. These scripts create synthetic uploads; use disposable storage, because published resources deliberately survive removal of course references. See [storage verification](storage-verification.md) for cleanup and provider contracts.

## Deployment gates

Local tests do not exercise a real Aliyun Mail tenant, Juhe account, AliOSS bucket, production CDN/proxy, or public traffic. Before launch, verify provider permissions and delivery, OSS CORS/private ACL/lifecycle rules, proxy streaming limits and trusted proxy addresses, backup restoration, and expected concurrent workload against the deployment configuration. The local upload duration is not a production throughput claim.

The disposable API, preview server, PostgreSQL and Redis containers were stopped after verification. Test evidence remains in ignored local verification directories. A final scan of the 168 changed/new repository paths found no credential-pattern matches or broken local documentation links; this bounded check is not a substitute for a full security audit.

Use the example configuration and [deployment configuration guide](deploy/configuration.md). Keep provider secrets outside source control. Contact mode changes require verification of the newly selected channel before automatic Member permissions apply. Courses can be disabled independently of community and review maintenance.

Shared published files are not physically removed by user-supplied references, profile changes, rejected reviews, or content deletion. Failed new uploads and abandoned course staging are cleaned separately. Reference-aware garbage collection is deferred; until introduced, budget storage for retained published objects.
