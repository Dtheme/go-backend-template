package speccheck

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const fixtureRequirement = `# 0.1.0 需求

## 功能列表

### F-0.1.0-001 注册

### F-0.1.0-002 登录
`

const fixtureTechnicalPlan = `# 0.1.0 技术方案

## 需求摘要

| F-0.1.0-001 | F-0.1.0-002 |

## API 契约

## 模块设计

## 测试计划

## 风险与回滚

## 文件清单

## 交付状态

` + "```yaml" + `
stage: planning
user_acceptance: pending
review: not_required
` + "```" + `

## 变更记录
`

const fixtureRules = `# 规则

## 文件权限规则

## AI 工作流（版本开发）

## 验证命令
`

const fixtureOverview = `# 技术讲解

## 目录结构

## 接口清单

## 测试策略

## 已知问题与待优化项
`

const fixturePostman = `{"info":{"name":"demo"},"item":[]}`

// newFixture 生成一份完整合法的模板仓库副本，含一个版本 0.1.0。
func newFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"Specs/requirements/需求模版.md":               "# 模版",
		"Specs/requirements/协议与数据.md":              "# 协议",
		"Specs/technical/技术讲解.md":                  fixtureOverview,
		"Specs/technical/技术方案模版.md":                "# 模版",
		".ai/ai-rules.md":                          fixtureRules,
		"Makefile":                                 "check:\n",
		"Specs/requirements/0.1.0/需求.md":           fixtureRequirement,
		"Specs/technical/0.1.0/技术方案.md":            fixtureTechnicalPlan,
		"api/postman/demo.postman_collection.json": fixturePostman,
	}
	for rel, content := range files {
		writeFile(t, root, rel, content)
	}
	links := map[string]string{
		"CLAUDE.md":      ".ai/ai-rules.md",
		"AGENTS.md":      ".ai/ai-rules.md",
		".claude/skills": "../.ai/skills",
		".agents/skills": "../.ai/skills",
	}
	if err := os.MkdirAll(filepath.Join(root, ".ai", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	for rel, target := range links {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(root, rel)); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func remove(t *testing.T, root, rel string) {
	t.Helper()
	if err := os.RemoveAll(filepath.Join(root, rel)); err != nil {
		t.Fatal(err)
	}
}

func mustRun(t *testing.T, root string) []Problem {
	t.Helper()
	problems, err := Run(root)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	return problems
}

func assertProblem(t *testing.T, problems []Problem, path, msg string) {
	t.Helper()
	for _, p := range problems {
		if p.Path == path && strings.Contains(p.Message, msg) {
			return
		}
	}
	t.Fatalf("want problem path=%q message containing %q, got %v", path, msg, problems)
}

func TestRun_ValidFixture(t *testing.T) {
	root := newFixture(t)
	if problems := mustRun(t, root); len(problems) != 0 {
		t.Fatalf("want 0 problems, got %v", problems)
	}
}

func TestRun_RootUnreadable(t *testing.T) {
	if _, err := Run(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("want error for missing root")
	}
}

func TestProblem_String(t *testing.T) {
	p := Problem{Path: "Makefile", Message: "缺少必需文件"}
	if got := p.String(); got != "Makefile: 缺少必需文件" {
		t.Fatalf("String() = %q", got)
	}
}

func TestRun_RequiredFiles(t *testing.T) {
	for _, rel := range []string{
		"Specs/requirements/需求模版.md",
		"Specs/requirements/协议与数据.md",
		"Specs/technical/技术讲解.md",
		"Specs/technical/技术方案模版.md",
		".ai/ai-rules.md",
		"Makefile",
	} {
		t.Run(rel, func(t *testing.T) {
			root := newFixture(t)
			remove(t, root, rel)
			assertProblem(t, mustRun(t, root), rel, "缺少必需文件")
		})
	}
}

func TestRun_RulesHeadings(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{"缺少文件权限规则", "## AI 工作流（版本开发）\n\n## 验证命令\n", "缺少章节「## 文件权限规则」"},
		{"缺少工作流", "## 文件权限规则\n\n## 验证命令\n", "缺少章节「## AI 工作流（版本开发）」"},
		{"缺少验证命令", "## 文件权限规则\n\n## AI 工作流（版本开发）\n", "缺少章节「## 验证命令」"},
		{"重复", fixtureRules + "\n## 验证命令\n", "章节「## 验证命令」重复"},
		{"三级标题不算", "### 文件权限规则\n\n## AI 工作流（版本开发）\n\n## 验证命令\n", "缺少章节「## 文件权限规则」"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := newFixture(t)
			writeFile(t, root, ".ai/ai-rules.md", c.content)
			assertProblem(t, mustRun(t, root), ".ai/ai-rules.md", c.want)
		})
	}
}

func TestRun_OverviewHeadings(t *testing.T) {
	for _, h := range []string{"## 目录结构", "## 接口清单", "## 测试策略", "## 已知问题与待优化项"} {
		t.Run(h, func(t *testing.T) {
			root := newFixture(t)
			writeFile(t, root, "Specs/technical/技术讲解.md", strings.Replace(fixtureOverview, h, "## 其他", 1))
			assertProblem(t, mustRun(t, root), "Specs/technical/技术讲解.md", "缺少章节「"+h+"」")
		})
	}
}

func TestRun_Symlinks(t *testing.T) {
	links := []string{"CLAUDE.md", "AGENTS.md", ".claude/skills", ".agents/skills"}
	for _, rel := range links {
		t.Run(rel+"/缺失", func(t *testing.T) {
			root := newFixture(t)
			remove(t, root, rel)
			assertProblem(t, mustRun(t, root), rel, "缺少符号链接")
		})
		t.Run(rel+"/普通文件", func(t *testing.T) {
			root := newFixture(t)
			remove(t, root, rel)
			writeFile(t, root, rel, "x")
			assertProblem(t, mustRun(t, root), rel, "不是符号链接")
		})
	}
	t.Run("目标不同", func(t *testing.T) {
		root := newFixture(t)
		remove(t, root, "CLAUDE.md")
		if err := os.Symlink("./.ai/ai-rules.md", filepath.Join(root, "CLAUDE.md")); err != nil {
			t.Fatal(err)
		}
		assertProblem(t, mustRun(t, root), "CLAUDE.md", "符号链接目标应为 .ai/ai-rules.md")
	})
	t.Run("目标不存在", func(t *testing.T) {
		root := newFixture(t)
		remove(t, root, ".ai/skills")
		assertProblem(t, mustRun(t, root), ".claude/skills", "符号链接目标不存在")
	})
}

func TestRun_VersionDirs(t *testing.T) {
	t.Run("目录名不是语义版本", func(t *testing.T) {
		root := newFixture(t)
		writeFile(t, root, "Specs/requirements/v0.2.0/需求.md", "x")
		writeFile(t, root, "Specs/technical/0.2/技术方案.md", "x")
		problems := mustRun(t, root)
		assertProblem(t, problems, "Specs/requirements/v0.2.0", "版本目录名")
		assertProblem(t, problems, "Specs/technical/0.2", "版本目录名")
	})
	t.Run("需求目录缺少需求.md", func(t *testing.T) {
		root := newFixture(t)
		remove(t, root, "Specs/requirements/0.1.0/需求.md")
		writeFile(t, root, "Specs/requirements/0.1.0/.keep", "")
		assertProblem(t, mustRun(t, root), "Specs/requirements/0.1.0/需求.md", "缺少")
	})
	t.Run("技术方案目录缺少技术方案.md", func(t *testing.T) {
		root := newFixture(t)
		remove(t, root, "Specs/technical/0.1.0/技术方案.md")
		writeFile(t, root, "Specs/technical/0.1.0/.keep", "")
		assertProblem(t, mustRun(t, root), "Specs/technical/0.1.0/技术方案.md", "缺少")
	})
	t.Run("只有需求没有技术方案", func(t *testing.T) {
		root := newFixture(t)
		remove(t, root, "Specs/technical/0.1.0")
		assertProblem(t, mustRun(t, root), "Specs/technical/0.1.0/技术方案.md", "缺少技术方案，运行 make spec-init VERSION=0.1.0")
	})
	t.Run("只有技术方案没有需求", func(t *testing.T) {
		root := newFixture(t)
		remove(t, root, "Specs/requirements/0.1.0")
		assertProblem(t, mustRun(t, root), "Specs/technical/0.1.0", "没有对应人工需求")
	})
	t.Run("忽略点目录与 .DS_Store", func(t *testing.T) {
		root := newFixture(t)
		writeFile(t, root, "Specs/requirements/.DS_Store", "")
		writeFile(t, root, "Specs/technical/.cache/x", "")
		writeFile(t, root, "Specs/technical/0.1.0/.DS_Store", "")
		if problems := mustRun(t, root); len(problems) != 0 {
			t.Fatalf("want 0 problems, got %v", problems)
		}
	})
}

func TestRun_FeatureIDs(t *testing.T) {
	const req = "Specs/requirements/0.1.0/需求.md"
	const plan = "Specs/technical/0.1.0/技术方案.md"
	cases := []struct {
		name string
		req  string
		plan string
		path string
		want string
	}{
		{"需求没有 Feature ID", "# 需求\n", fixtureTechnicalPlan, req, "至少一个 Feature ID"},
		{"需求 ID 版本不一致", fixtureRequirement + "\n### F-0.2.0-003 x\n", fixtureTechnicalPlan, req, "F-0.2.0-003 的版本与目录 0.1.0 不一致"},
		{"技术方案缺少 ID", fixtureRequirement, strings.Replace(fixtureTechnicalPlan, "F-0.1.0-002", "", 1), plan, "未引用 F-0.1.0-002"},
		{"技术方案 ID 版本不一致", fixtureRequirement, strings.Replace(fixtureTechnicalPlan, "| F-0.1.0-001 |", "| F-0.1.0-001 | F-0.3.0-001 |", 1), plan, "F-0.3.0-001 的版本与目录 0.1.0 不一致"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := newFixture(t)
			writeFile(t, root, req, c.req)
			writeFile(t, root, plan, c.plan)
			assertProblem(t, mustRun(t, root), c.path, c.want)
		})
	}
}

func TestRun_PlanHeadings(t *testing.T) {
	headings := []string{"## 需求摘要", "## API 契约", "## 模块设计", "## 测试计划", "## 风险与回滚", "## 文件清单", "## 交付状态", "## 变更记录"}
	for _, h := range headings {
		t.Run("缺少"+h, func(t *testing.T) {
			root := newFixture(t)
			writeFile(t, root, "Specs/technical/0.1.0/技术方案.md", strings.Replace(fixtureTechnicalPlan, h, "## 其他", 1))
			assertProblem(t, mustRun(t, root), "Specs/technical/0.1.0/技术方案.md", "缺少章节「"+h+"」")
		})
	}
	t.Run("重复", func(t *testing.T) {
		root := newFixture(t)
		writeFile(t, root, "Specs/technical/0.1.0/技术方案.md", fixtureTechnicalPlan+"\n## 模块设计\n")
		assertProblem(t, mustRun(t, root), "Specs/technical/0.1.0/技术方案.md", "章节「## 模块设计」重复")
	})
}

func TestRun_DeliveryStatus(t *testing.T) {
	cases := []struct {
		name string
		plan string
		want string
	}{
		{"delivered 未确认", strings.Replace(fixtureTechnicalPlan, "stage: planning", "stage: delivered", 1), "user_acceptance 不是 confirmed"},
		{"非法枚举", strings.Replace(fixtureTechnicalPlan, "stage: planning", "stage: done", 1), "stage 非法"},
		{"缺少代码块", strings.Replace(fixtureTechnicalPlan, "```yaml", "```json", 1), "必须是 ```yaml"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := newFixture(t)
			writeFile(t, root, "Specs/technical/0.1.0/技术方案.md", c.plan)
			assertProblem(t, mustRun(t, root), "Specs/technical/0.1.0/技术方案.md", c.want)
		})
	}
}

func TestRun_Postman(t *testing.T) {
	const file = "api/postman/demo.postman_collection.json"
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{"非法 JSON", "{", "不是合法 JSON"},
		{"缺少 info.name", `{"info":{},"item":[]}`, "info.name"},
		{"info.name 不是字符串", `{"info":{"name":1},"item":[]}`, "info.name"},
		{"item 不是数组", `{"info":{"name":"x"},"item":{}}`, "item"},
		{"缺少 item", `{"info":{"name":"x"}}`, "item"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := newFixture(t)
			writeFile(t, root, file, c.content)
			assertProblem(t, mustRun(t, root), file, c.want)
		})
	}
	t.Run("目录不存在不是问题", func(t *testing.T) {
		root := newFixture(t)
		remove(t, root, "api")
		if problems := mustRun(t, root); len(problems) != 0 {
			t.Fatalf("want 0 problems, got %v", problems)
		}
	})
	t.Run("其他后缀忽略", func(t *testing.T) {
		root := newFixture(t)
		writeFile(t, root, "api/postman/env.json", "{")
		if problems := mustRun(t, root); len(problems) != 0 {
			t.Fatalf("want 0 problems, got %v", problems)
		}
	})
}

func TestRun_UnexpectedFiles(t *testing.T) {
	t.Run("需求目录", func(t *testing.T) {
		root := newFixture(t)
		writeFile(t, root, "Specs/requirements/0.1.0/notes.md", "x")
		assertProblem(t, mustRun(t, root), "Specs/requirements/0.1.0/notes.md", "意外文件")
	})
	t.Run("技术方案目录", func(t *testing.T) {
		root := newFixture(t)
		writeFile(t, root, "Specs/technical/0.1.0/draft.md", "x")
		assertProblem(t, mustRun(t, root), "Specs/technical/0.1.0/draft.md", "意外文件")
	})
	t.Run("允许 graph.json 与点文件", func(t *testing.T) {
		root := newFixture(t)
		writeFile(t, root, "Specs/technical/0.1.0/graph.json", "{}")
		writeFile(t, root, "Specs/technical/0.1.0/.keep", "")
		writeFile(t, root, "Specs/requirements/0.1.0/.keep", "")
		if problems := mustRun(t, root); len(problems) != 0 {
			t.Fatalf("want 0 problems, got %v", problems)
		}
	})
}

func TestRun_Sorted(t *testing.T) {
	root := newFixture(t)
	remove(t, root, "Makefile")
	remove(t, root, "AGENTS.md")
	writeFile(t, root, "Specs/technical/0.1.0/技术方案.md", strings.Replace(fixtureTechnicalPlan, "F-0.1.0-002", "F-0.2.0-002", 1))
	problems := mustRun(t, root)
	if len(problems) < 3 {
		t.Fatalf("want several problems, got %v", problems)
	}
	sorted := sort.SliceIsSorted(problems, func(i, j int) bool {
		if problems[i].Path != problems[j].Path {
			return problems[i].Path < problems[j].Path
		}
		return problems[i].Message < problems[j].Message
	})
	if !sorted {
		t.Fatalf("problems not sorted: %v", problems)
	}
}

func TestVersions(t *testing.T) {
	root := newFixture(t)
	writeFile(t, root, "Specs/technical/0.2.0/技术方案.md", "x")
	writeFile(t, root, "Specs/requirements/bad/需求.md", "x")
	got, err := Versions(root)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != "0.1.0,0.2.0" {
		t.Fatalf("Versions = %v", got)
	}
}

func TestRun_TemplateRepo(t *testing.T) {
	problems, err := Run(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Fatalf("template repo has problems: %v", problems)
	}
}

func mkdir(t *testing.T, root, rel string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, rel), 0o755); err != nil {
		t.Fatal(err)
	}
}

func symlink(t *testing.T, root, rel, target string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, rel)); err != nil {
		t.Fatal(err)
	}
}

func assertNoProblem(t *testing.T, problems []Problem, path, msg string) {
	t.Helper()
	for _, p := range problems {
		if p.Path == path && strings.Contains(p.Message, msg) {
			t.Fatalf("unexpected problem %v", p)
		}
	}
}

func assertNoProblems(t *testing.T, problems []Problem) {
	t.Helper()
	if len(problems) != 0 {
		t.Fatalf("want 0 problems, got %v", problems)
	}
}

// 必需路径存在但类型不对属于规则违反，Run 必须返回 Problem 而不是 I/O 错误。
func TestRun_WrongFileType(t *testing.T) {
	t.Run(".ai 为普通文件", func(t *testing.T) {
		root := newFixture(t)
		remove(t, root, ".ai")
		writeFile(t, root, ".ai", "x")
		assertProblem(t, mustRun(t, root), ".ai/ai-rules.md", "缺少必需文件")
	})
	t.Run("Makefile 为目录", func(t *testing.T) {
		root := newFixture(t)
		remove(t, root, "Makefile")
		mkdir(t, root, "Makefile")
		assertProblem(t, mustRun(t, root), "Makefile", "不是普通文件")
	})
	t.Run("需求.md 为目录", func(t *testing.T) {
		root := newFixture(t)
		remove(t, root, "Specs/requirements/0.1.0/需求.md")
		mkdir(t, root, "Specs/requirements/0.1.0/需求.md")
		assertProblem(t, mustRun(t, root), "Specs/requirements/0.1.0/需求.md", "不是普通文件")
	})
	t.Run("技术方案.md 为目录", func(t *testing.T) {
		root := newFixture(t)
		remove(t, root, "Specs/technical/0.1.0/技术方案.md")
		mkdir(t, root, "Specs/technical/0.1.0/技术方案.md")
		assertProblem(t, mustRun(t, root), "Specs/technical/0.1.0/技术方案.md", "不是普通文件")
	})
	t.Run("Specs/requirements 为普通文件", func(t *testing.T) {
		root := newFixture(t)
		remove(t, root, "Specs/requirements")
		writeFile(t, root, "Specs/requirements", "x")
		problems := mustRun(t, root)
		assertProblem(t, problems, "Specs/requirements/需求模版.md", "缺少必需文件")
		assertProblem(t, problems, "Specs/technical/0.1.0", "没有对应人工需求")
	})
	t.Run("postman 条目为目录符号链接", func(t *testing.T) {
		root := newFixture(t)
		mkdir(t, root, "api/target")
		symlink(t, root, "api/postman/x.postman_collection.json", "../target")
		assertProblem(t, mustRun(t, root), "api/postman/x.postman_collection.json", "不是普通文件")
	})
}

func TestRun_SymlinkVersionDir(t *testing.T) {
	plan020 := strings.ReplaceAll(fixtureTechnicalPlan, "0.1.0", "0.2.0")
	t.Run("需求目录为符号链接", func(t *testing.T) {
		root := newFixture(t)
		writeFile(t, root, "_ext/0.2.0/需求.md", "### F-0.2.0-001 x\n")
		symlink(t, root, "Specs/requirements/0.2.0", "../../_ext/0.2.0")
		writeFile(t, root, "Specs/technical/0.2.0/技术方案.md", plan020)
		assertNoProblems(t, mustRun(t, root))
	})
	t.Run("符号链接目录内容同样被检查", func(t *testing.T) {
		root := newFixture(t)
		writeFile(t, root, "_ext/0.2.0/需求.md", "# 无 ID\n")
		symlink(t, root, "Specs/requirements/0.2.0", "../../_ext/0.2.0")
		writeFile(t, root, "Specs/technical/0.2.0/技术方案.md", plan020)
		assertProblem(t, mustRun(t, root), "Specs/requirements/0.2.0/需求.md", "至少一个 Feature ID")
	})
	t.Run("技术方案目录为符号链接", func(t *testing.T) {
		root := newFixture(t)
		writeFile(t, root, "Specs/requirements/0.2.0/需求.md", "### F-0.2.0-001 x\n")
		writeFile(t, root, "_ext/tech/技术方案.md", plan020)
		symlink(t, root, "Specs/technical/0.2.0", "../../_ext/tech")
		assertNoProblems(t, mustRun(t, root))
	})
	t.Run("版本同名普通文件", func(t *testing.T) {
		root := newFixture(t)
		writeFile(t, root, "Specs/requirements/0.3.0", "x")
		assertProblem(t, mustRun(t, root), "Specs/requirements/0.3.0", "版本条目必须是目录")
	})
	t.Run("Versions 包含符号链接版本", func(t *testing.T) {
		root := newFixture(t)
		writeFile(t, root, "_ext/0.2.0/需求.md", "x")
		symlink(t, root, "Specs/requirements/0.2.0", "../../_ext/0.2.0")
		got, err := Versions(root)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Join(got, ",") != "0.1.0,0.2.0" {
			t.Fatalf("Versions = %v", got)
		}
	})
}

func TestRun_PostmanTopLevel(t *testing.T) {
	const file = "api/postman/demo.postman_collection.json"
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{"顶层是数组", "[]", "顶层必须是 JSON 对象"},
		{"顶层是字符串", `"x"`, "顶层必须是 JSON 对象"},
		{"顶层是 null", "null", "顶层必须是 JSON 对象"},
		{"info 不是对象", `{"info":1,"item":[]}`, "info 必须是 JSON 对象"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := newFixture(t)
			writeFile(t, root, file, c.content)
			problems := mustRun(t, root)
			assertProblem(t, problems, file, c.want)
			assertNoProblem(t, problems, file, "Go value")
		})
	}
}

func TestFeatureIDs_Boundaries(t *testing.T) {
	doc := []byte("F-0.1.0-0002 REF-0.1.0-001 F-0.1.0-001 F-0.1.0-002,F-0.1.0-003 功能F-0.1.0-004。\nF-0.1.0-005")
	got := strings.Join(featureIDs(doc), ",")
	if got != "F-0.1.0-001,F-0.1.0-002,F-0.1.0-003,F-0.1.0-004,F-0.1.0-005" {
		t.Fatalf("featureIDs = %s", got)
	}
}

func TestRun_FeatureIDBoundaries(t *testing.T) {
	root := newFixture(t)
	writeFile(t, root, "Specs/requirements/0.1.0/需求.md", fixtureRequirement+"\n四位编号 F-0.1.0-0002 与 REF-0.1.0-009 都不是 Feature ID\n")
	problems := mustRun(t, root)
	assertNoProblem(t, problems, "Specs/technical/0.1.0/技术方案.md", "未引用")
	assertNoProblems(t, problems)
}

func TestRun_HeadingsInsideFence(t *testing.T) {
	const plan = "Specs/technical/0.1.0/技术方案.md"
	t.Run("围栏内的重复标题不计数", func(t *testing.T) {
		root := newFixture(t)
		writeFile(t, root, ".ai/ai-rules.md", fixtureRules+"\n```md\n## 验证命令\n```\n")
		assertNoProblems(t, mustRun(t, root))
	})
	t.Run("波浪线围栏", func(t *testing.T) {
		root := newFixture(t)
		writeFile(t, root, ".ai/ai-rules.md", fixtureRules+"\n~~~\n## 验证命令\n~~~\n")
		assertNoProblems(t, mustRun(t, root))
	})
	t.Run("围栏内的标题不算存在", func(t *testing.T) {
		root := newFixture(t)
		writeFile(t, root, ".ai/ai-rules.md", "## 文件权限规则\n\n## AI 工作流（版本开发）\n\n```\n## 验证命令\n```\n")
		assertProblem(t, mustRun(t, root), ".ai/ai-rules.md", "缺少章节「## 验证命令」")
	})
	t.Run("技术方案变更记录引用章节名", func(t *testing.T) {
		root := newFixture(t)
		writeFile(t, root, plan, fixtureTechnicalPlan+"\n```\n## 模块设计\n```\n")
		assertNoProblems(t, mustRun(t, root))
	})
}

// 版本不一致的需求 ID 只报一致性问题，不再追溯技术方案引用，避免同一根因报两条。
func TestRun_MismatchedRequirementIDNotTraced(t *testing.T) {
	const req = "Specs/requirements/0.1.0/需求.md"
	const plan = "Specs/technical/0.1.0/技术方案.md"
	root := newFixture(t)
	writeFile(t, root, req, fixtureRequirement+"\n### F-0.2.0-003 x\n")
	problems := mustRun(t, root)
	assertProblem(t, problems, req, "F-0.2.0-003 的版本与目录 0.1.0 不一致")
	assertNoProblem(t, problems, plan, "未引用 F-0.2.0-003")
	if len(problems) != 1 {
		t.Fatalf("want exactly 1 problem, got %v", problems)
	}
}
