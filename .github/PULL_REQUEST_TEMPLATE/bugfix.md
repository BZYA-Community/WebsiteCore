<!-- Bugfix PR. Pick the template that matches your change. -->

## Linked issue

<!-- closes #xx -->

## Root cause

<!-- What was actually broken and why. One or two sentences; reviewers check the fix against this. -->

## The fix

<!-- What you changed and why that is the correct fix rather than a workaround.
     Call out separately: generated code (auto/), migrations, config keys, docs. -->

## BVT — backend (mandatory, see CONTRIBUTING.md)

- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean
- [ ] `golangci-lint run ./...` clean
- [ ] `go test ./...` all green
- [ ] Touched `mirc/`: ran `make gen-mir`, no hand edits in `auto/`
- [ ] Touched schema: migration pair in **both** dialects, verified with a `migration`-tagged build
- [ ] Touched config: `internal/conf/config.yaml` and `config.yaml.sample` updated together
- [ ] No build artifacts committed

## BVT — frontend (mandatory if `web/` changed)

- [ ] `npm run lint` — 0 errors
- [ ] `npm run build` — succeeds
- [ ] No overlapping or overflowing content at the 7 standard viewports (`verify_sidebar_830.py` / `measure_width.py` / screenshots attached)

## Regression verification

<!-- How did you prove the bug is gone AND nothing else broke? -->

- Reproduced the bug before the fix: <!-- how (script / curl / UI steps) -->
- Verified the fix: <!-- paste the output or attach screenshots -->
- Relevant `scripts/test_*.py` results: PASS=__ FAIL=__ (delete lines that do not apply)

## Impact

<!-- Does any deployment need action: config change, migration, Redis flush? Any behavior change beyond the bug? -->
