// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// 该文件不带 migration 构建标签，因此 `go test ./internal/infra/migration/` 默认即可运行，
// 无需额外 tag。校验规则与背景见 scripts/migration/README.md。

package migration

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// migrationRoot 是相对本包目录的迁移脚本根目录
// (go test 的工作目录固定为包源码目录)。
const migrationRoot = "../../../scripts/migration"

// requiredDialects 是必须存在的方言目录。
// 只有 MySQL 与 PostgreSQL 接入了 gorm 与 golang-migrate(见 migration_embed.go)，
// SQLite 支持已在 2e0ac8bf 移除，scripts/migration/sqlite3 随之删除；
// 若未来恢复某个方言，只需在此登记(其中任意含 *.sql 的子目录也会被自动纳入校验)。
var requiredDialects = []string{"mysql", "postgres"}

// migrationFileRe 约束文件名形如 NNNN_name.{up,down}.sql，
// 与 golang-migrate iofs 的解析规则保持一致(4 位编号是本仓库的书写约定)。
var migrationFileRe = regexp.MustCompile(`^(\d{4})_([A-Za-z0-9_]+)\.(up|down)\.sql$`)

// dialectOnly 是"只存在于某一个方言"的迁移登记项。
// 新增此类迁移时必须在此登记并写明原因，否则跨方言一致性校验会失败。
type dialectOnly struct {
	dialect string
	name    string
	reason  string
}

// dialectOnlyMigrations 记录历史上有据可依的方言独有迁移，见 scripts/migration/README.md。
var dialectOnlyMigrations = []dialectOnly{
	{
		dialect: "mysql",
		name:    "optimize_idx",
		reason:  "MySQL 索引改名迁移；PostgreSQL 的 0001_initialize_schema 已按最终命名创建索引，无需对应迁移",
	},
}

// migrationEntry 描述同一方言内一个迁移的 up/down 情况。
type migrationEntry struct {
	version int
	name    string
	up      bool
	down    bool
}

// scanMigrations 校验 root 下的方言迁移目录，返回发现的问题列表(为空即通过)与解析结果。
// exceptions 为允许只存在于某个方言的迁移登记项。
func scanMigrations(root string, exceptions []dialectOnly) ([]string, map[string]map[int]*migrationEntry) {
	var issues []string

	entries, err := os.ReadDir(root)
	if err != nil {
		return []string{fmt.Sprintf("读取迁移目录 %s 失败: %v", root, err)}, nil
	}

	dialects := make(map[string]map[int]*migrationEntry)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		files, err := os.ReadDir(filepath.Join(root, e.Name()))
		if err != nil {
			issues = append(issues, fmt.Sprintf("读取方言目录 %s 失败: %v", e.Name(), err))
			continue
		}

		byVersion := make(map[int]*migrationEntry)
		byName := make(map[string]int)
		hasSQL := false
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".sql") {
				// 方言目录只校验 *.sql，其余文件(如说明文档)不参与。
				continue
			}
			hasSQL = true
			m := migrationFileRe.FindStringSubmatch(f.Name())
			if m == nil {
				issues = append(issues, fmt.Sprintf(
					"%s/%s: 文件名不符合 NNNN_name.{up,down}.sql 约定", e.Name(), f.Name()))
				continue
			}
			version, err := strconv.Atoi(m[1])
			if err != nil {
				issues = append(issues, fmt.Sprintf("%s/%s: 编号无法解析: %v", e.Name(), f.Name(), err))
				continue
			}
			name, kind := m[2], m[3]

			if prev, ok := byName[name]; ok && prev != version {
				issues = append(issues, fmt.Sprintf(
					"%s: 同名迁移 %s 同时出现在 %04d 与 %04d", e.Name(), name, prev, version))
				continue
			}
			byName[name] = version

			ent, ok := byVersion[version]
			if !ok {
				ent = &migrationEntry{version: version, name: name}
				byVersion[version] = ent
			} else if ent.name != name {
				issues = append(issues, fmt.Sprintf(
					"%s: 编号 %04d 被两个迁移占用: %s 与 %s", e.Name(), version, ent.name, name))
				continue
			}
			if kind == "up" {
				ent.up = true
			} else {
				ent.down = true
			}
		}

		if hasSQL || contains(requiredDialects, e.Name()) {
			dialects[e.Name()] = byVersion
		}
	}

	for _, req := range requiredDialects {
		if _, ok := dialects[req]; !ok {
			issues = append(issues, fmt.Sprintf("缺少必需的方言目录 %s/%s", root, req))
			continue
		}
		issues = append(issues, checkDialect(req, dialects[req])...)
	}
	// 自动发现的方言(如未来恢复的 sqlite3)同样校验编号与成对性。
	for dialect, byVersion := range dialects {
		if contains(requiredDialects, dialect) {
			continue
		}
		issues = append(issues, checkDialect(dialect, byVersion)...)
	}

	issues = append(issues, checkCrossDialect(dialects, exceptions)...)
	return issues, dialects
}

// checkDialect 校验单个方言内部的编号连续性与 up/down 成对性。
func checkDialect(dialect string, byVersion map[int]*migrationEntry) []string {
	var issues []string
	if len(byVersion) == 0 {
		return []string{fmt.Sprintf("%s: 方言目录没有任何迁移文件", dialect)}
	}

	versions := make([]int, 0, len(byVersion))
	for v := range byVersion {
		versions = append(versions, v)
	}
	sort.Ints(versions)
	for i, v := range versions {
		want := i + 1
		if v != want {
			issues = append(issues, fmt.Sprintf(
				"%s: 编号不连续，期望 %04d，实际 %04d(%s)", dialect, want, v, byVersion[v].name))
		}
	}
	for _, v := range versions {
		ent := byVersion[v]
		switch {
		case ent.up && !ent.down:
			issues = append(issues, fmt.Sprintf(
				"%s: 迁移 %04d_%s 缺少 down 文件", dialect, ent.version, ent.name))
		case ent.down && !ent.up:
			issues = append(issues, fmt.Sprintf(
				"%s: 迁移 %04d_%s 缺少 up 文件", dialect, ent.version, ent.name))
		}
	}
	return issues
}

// checkCrossDialect 校验各方言的"迁移名集合"(忽略编号)一致，
// 并检查 dialectOnly 登记项是否仍然准确。
func checkCrossDialect(dialects map[string]map[int]*migrationEntry, exceptions []dialectOnly) []string {
	var issues []string
	if len(dialects) == 0 {
		return issues
	}

	names := make(map[string]map[string]bool)
	for dialect, byVersion := range dialects {
		for _, ent := range byVersion {
			if names[ent.name] == nil {
				names[ent.name] = make(map[string]bool)
			}
			names[ent.name][dialect] = true
		}
	}

	allDialects := make([]string, 0, len(dialects))
	for dialect := range dialects {
		allDialects = append(allDialects, dialect)
	}
	sort.Strings(allDialects)

	allowed := make(map[string]bool) // 已登记为"方言独有"的迁移名
	for _, ex := range exceptions {
		allowed[ex.name] = true
	}

	sortedNames := make([]string, 0, len(names))
	for name := range names {
		sortedNames = append(sortedNames, name)
	}
	sort.Strings(sortedNames)

	for _, name := range sortedNames {
		// 已登记为方言独有时跳过缺失检查，登记项是否仍准确由下方"过期登记"检查兜底。
		exclusive := allowed[name]
		for _, dialect := range allDialects {
			if names[name][dialect] || exclusive {
				continue
			}
			issues = append(issues, fmt.Sprintf(
				"迁移 %s 缺少方言 %s 的对应脚本(若属方言独有，请在 dialectOnlyMigrations 登记原因)", name, dialect))
		}
	}

	for _, ex := range exceptions {
		byVersion, ok := dialects[ex.dialect]
		if !ok {
			issues = append(issues, fmt.Sprintf(
				"dialectOnly 登记项 %s/%s 已过期: 方言目录不存在，请从 dialectOnlyMigrations 移除", ex.dialect, ex.name))
			continue
		}
		stale := true
		for _, ent := range byVersion {
			if ent.name == ex.name {
				stale = false
				break
			}
		}
		if stale {
			issues = append(issues, fmt.Sprintf(
				"dialectOnly 登记项 %s/%s 已过期: 该迁移不存在，请从 dialectOnlyMigrations 移除", ex.dialect, ex.name))
			continue
		}
		for _, dialect := range allDialects {
			if dialect == ex.dialect {
				continue
			}
			for _, ent := range dialects[dialect] {
				if ent.name == ex.name {
					issues = append(issues, fmt.Sprintf(
						"dialectOnly 登记项 %s/%s 已过期: %s 也存在同名迁移，请从 dialectOnlyMigrations 移除",
						ex.dialect, ex.name, dialect))
				}
			}
		}
	}
	return issues
}

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

// TestMigrationDialectConsistency 是 scripts/migration 的一致性校验，
// 失败说明有方言漏写迁移、编号错位或文件命名不合规。
func TestMigrationDialectConsistency(t *testing.T) {
	issues, dialects := scanMigrations(migrationRoot, dialectOnlyMigrations)
	for _, issue := range issues {
		t.Error(issue)
	}
	if t.Failed() {
		t.Log("修复方法见 scripts/migration/README.md")
		return
	}
	logMapping(t, dialects)
}

// logMapping 打印当前各方言的编号 -> 迁移名映射表，便于人工核对与更新文档。
func logMapping(t *testing.T, dialects map[string]map[int]*migrationEntry) {
	t.Helper()
	dialectNames := make([]string, 0, len(dialects))
	maxVersion := 0
	for dialect, byVersion := range dialects {
		dialectNames = append(dialectNames, dialect)
		for v := range byVersion {
			if v > maxVersion {
				maxVersion = v
			}
		}
	}
	sort.Strings(dialectNames)

	var b strings.Builder
	b.WriteString("迁移编号映射(编号: " + strings.Join(dialectNames, " / ") + ")\n")
	for v := 1; v <= maxVersion; v++ {
		b.WriteString(fmt.Sprintf("  %04d:", v))
		for _, dialect := range dialectNames {
			name := "-"
			if ent, ok := dialects[dialect][v]; ok {
				name = ent.name
			}
			b.WriteString(fmt.Sprintf(" %-28s", name))
		}
		b.WriteString("\n")
	}
	t.Log("\n" + b.String())
}

// TestMigrationCheckerDetectsProblems 用构造数据反向验证校验器确实能发现问题，
// 避免一致性校验长期"绿灯"却从未生效。
func TestMigrationCheckerDetectsProblems(t *testing.T) {
	cases := []struct {
		name     string
		files    map[string][]string
		wantSubs []string
	}{
		{
			name: "方言漏写迁移",
			files: map[string][]string{
				"mysql":    {"0001_initialize_schema.up.sql", "0001_initialize_schema.down.sql", "0002_share_count.up.sql", "0002_share_count.down.sql"},
				"postgres": {"0001_initialize_schema.up.sql", "0001_initialize_schema.down.sql"},
			},
			wantSubs: []string{"share_count", "postgres"},
		},
		{
			name: "编号跳号",
			files: map[string][]string{
				"mysql":    {"0001_initialize_schema.up.sql", "0001_initialize_schema.down.sql"},
				"postgres": {"0001_initialize_schema.up.sql", "0001_initialize_schema.down.sql", "0003_share_count.up.sql", "0003_share_count.down.sql"},
			},
			wantSubs: []string{"编号不连续", "0002"},
		},
		{
			name: "缺少 down 文件",
			files: map[string][]string{
				"mysql":    {"0001_initialize_schema.up.sql", "0001_initialize_schema.down.sql"},
				"postgres": {"0001_initialize_schema.up.sql", "0001_initialize_schema.down.sql", "0002_share_count.up.sql"},
			},
			wantSubs: []string{"缺少 down 文件"},
		},
		{
			name: "文件命名不合规",
			files: map[string][]string{
				"mysql":    {"0001_initialize_schema.up.sql", "0001_initialize_schema.down.sql", "share_count.sql"},
				"postgres": {"0001_initialize_schema.up.sql", "0001_initialize_schema.down.sql"},
			},
			wantSubs: []string{"NNNN_name"},
		},
		{
			name: "缺少方言目录",
			files: map[string][]string{
				"mysql": {"0001_initialize_schema.up.sql", "0001_initialize_schema.down.sql"},
			},
			wantSubs: []string{"缺少必需的方言目录", "postgres"},
		},
		{
			name: "过期的方言独有登记",
			files: map[string][]string{
				"mysql":    {"0001_initialize_schema.up.sql", "0001_initialize_schema.down.sql"},
				"postgres": {"0001_initialize_schema.up.sql", "0001_initialize_schema.down.sql"},
			},
			wantSubs: []string{"已过期"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			for dialect, files := range tc.files {
				writeFiles(t, root, dialect, files...)
			}
			issues, _ := scanMigrations(root, dialectOnlyMigrations)
			if len(issues) == 0 {
				t.Fatalf("期望报告问题，实际校验通过")
			}
			joined := strings.Join(issues, "\n")
			for _, want := range tc.wantSubs {
				if !strings.Contains(joined, want) {
					t.Errorf("问题列表缺少预期内容 %q，实际输出:\n%s", want, joined)
				}
			}
		})
	}
}

// TestMigrationCheckerAcceptsHealthyTree 确认结构正确的一组迁移能通过校验。
func TestMigrationCheckerAcceptsHealthyTree(t *testing.T) {
	root := t.TempDir()
	files := []string{
		"0001_initialize_schema.up.sql", "0001_initialize_schema.down.sql",
		"0002_share_count.up.sql", "0002_share_count.down.sql",
	}
	writeFiles(t, root, "mysql", files...)
	writeFiles(t, root, "postgres", files...)

	issues, dialects := scanMigrations(root, nil)
	for _, issue := range issues {
		t.Errorf("健康目录不应报错: %s", issue)
	}
	if len(dialects) != 2 {
		t.Errorf("期望解析 2 个方言，实际 %d", len(dialects))
	}
}

func writeFiles(t *testing.T, root, dialect string, files ...string) {
	t.Helper()
	dir := filepath.Join(root, dialect)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("创建目录 %s 失败: %v", dir, err)
	}
	for _, name := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("-- test only\n"), 0o644); err != nil {
			t.Fatalf("写文件 %s 失败: %v", name, err)
		}
	}
}
