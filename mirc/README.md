# mirc — RESTful API Definitions for WebsiteCore

This directory is the single source of truth for all RESTful API routes. Files here are [go-mir](https://github.com/alimy/mir) interface definitions; the actual routing code is generated into `../auto/` and must never be hand-edited.

## Services

| Service | Directory | API series | URL prefix | Purpose |
| --- | --- | --- | --- | --- |
| Web | `web/` | `/` | `/` | Main site API (posts, comments, messaging, users, courses) |
| Admin | `admin/` | `m` | `/m/` | Admin backend operations |
| SpaceX | `space/` | `x` | `/x/` | SpaceX service (legacy from upstream) |
| NativeOBS | `localoss/` | `s` | `/s/` | Direct object-storage uploads |
| Bot | `bot/` | `r` | `/r/` | Bot service (legacy from upstream) |

## Workflow

1. Declare or modify an endpoint signature in the matching service directory (e.g. `web/v1/`).
2. Regenerate:

   ```sh
   make gen-mir        # runs `go generate mirc/gen.go`, then gofumpt over auto/api
   ```

3. Implement the handler in `../internal/servants/<service>/`.

Generation is driven by `gen.go` in this directory. After regenerating, `auto/` changes belong in the same commit as the `mirc/` change — reviewers check that `auto/` contains no manual edits.

See [../docs/development.md](../docs/development.md) for the full backend development flow.
