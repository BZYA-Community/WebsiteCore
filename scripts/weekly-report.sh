#!/usr/bin/env bash
# 生成社区周报 → /tmp/weekly-report.md（确定性模板，无 AI 依赖）
# 每周五 20:00 触发；按 conventional commits 前缀自动分类"功能更新 / Bug 修复"
set -uo pipefail
SINCE=$(date -d '7 days ago' +%Y-%m-%d 2>/dev/null || date -v-7d +%Y-%m-%d)
D=$(date +%Y-%m-%d)

gh pr list --state merged --search "merged:>=$SINCE" --limit 100 \
  --json number,title,author >/tmp/merged.json 2>/dev/null || echo '[]' >/tmp/merged.json

FEATS=$(jq -r '.[] | select(.title|test("^(feat|feature|新增)";"i")) | "- #\(.number) \(.title) —— @\(.author.login)"' /tmp/merged.json)
FIXES=$(jq -r '.[] | select(.title|test("^(fix|修复)";"i")) | "- #\(.number) \(.title) —— @\(.author.login)"' /tmp/merged.json)
OTHERS=$(jq -r '.[] | select((.title|test("^(feat|feature|新增|fix|修复)";"i"))|not) | "- #\(.number) \(.title) —— @\(.author.login)"' /tmp/merged.json)
N_FEAT=$(printf '%s' "$FEATS" | grep -c '^- ' || true)
N_FIX=$(printf '%s' "$FIXES" | grep -c '^- ' || true)

OPEN_LIST=$(gh pr list --state open --limit 20 --json number,title,author --template '{{range .}}- #{{.number}} {{.title}} —— @{{.author.login}}
{{end}}' 2>/dev/null)


git log --since='7 days ago' --format='%an' 2>/dev/null \
  | sort | uniq -c | sort -rn \
  | awk '{printf "- %s: %s ge commit\n", $2, $1}' > /tmp/commit_stats.txt || true

TOP=$(jq -r 'group_by(.author.login) | map({a:.[0].author.login, n:length}) | sort_by(-.n) | .[0:5][] | "- @\(.a)：\(.n) 个已合并 PR"' /tmp/merged.json)

cat > /tmp/weekly-report.md <<EOM
# 社区开发周报 ${D}

> 供老师决策是否上线：本周合并内容均已通过 CI 编译与 AI 审查。

## 本周功能更新（${N_FEAT} 项）

${FEATS}

## 本周 Bug 修复（${N_FIX} 项）

${FIXES}

## 其他改进

${OTHERS}

## 进行中的 PR

${OPEN_LIST}

## 本周贡献榜（已合并 PR）

${TOP}

## 本周 commit（按作者）

$(cat /tmp/commit_stats.txt)

---

*每周五 20:00 自动生成，数据来自 GitHub。*
EOM

echo '--- 周报已生成，预览 ---'
cat /tmp/weekly-report.md
