package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validPlan = `# 0.1.0 技术方案

## 需求摘要

F-0.1.0-001

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

// newFixture 生成最小合法仓库，含一个版本 0.1.0。
func newFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"Specs/requirements/需求模版.md":     "# 模版",
		"Specs/requirements/协议与数据.md":    "# 协议",
		"Specs/technical/技术讲解.md":        "## 目录结构\n\n## 接口清单\n\n## 测试策略\n\n## 已知问题与待优化项\n",
		"Specs/technical/技术方案模版.md":      "# 模版",
		".ai/ai-rules.md":                "## 文件权限规则\n\n## AI 工作流（版本开发）\n\n## 验证命令\n",
		".ai/memory.md":                  "## 已验证的事实\n\n## 踩过的坑\n\n## 待验证\n",
		"Makefile":                       "check:\n",
		"Specs/requirements/0.1.0/需求.md": "### F-0.1.0-001 x\n",
		"Specs/technical/0.1.0/技术方案.md":  validPlan,
	}
	for rel, content := range files {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, ".ai", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	links := map[string]string{
		"CLAUDE.md":      ".ai/ai-rules.md",
		"AGENTS.md":      ".ai/ai-rules.md",
		".claude/skills": "../.ai/skills",
		".agents/skills": "../.ai/skills",
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

func TestRun_ExitCodes(t *testing.T) {
	cases := []struct {
		name       string
		prepare    func(t *testing.T) string
		wantCode   int
		wantStdout []string
		wantStderr string
	}{
		{
			name:       "无问题",
			prepare:    newFixture,
			wantCode:   0,
			wantStdout: []string{"spec-check ok (1 version(s))"},
		},
		{
			name: "有问题",
			prepare: func(t *testing.T) string {
				root := newFixture(t)
				if err := os.Remove(filepath.Join(root, "Makefile")); err != nil {
					t.Fatal(err)
				}
				return root
			},
			wantCode:   1,
			wantStdout: []string{"Makefile: 缺少必需文件", "spec-check: 1 problem(s)"},
		},
		{
			name:       "根目录不存在",
			prepare:    func(t *testing.T) string { return filepath.Join(t.TempDir(), "missing") },
			wantCode:   2,
			wantStderr: "spec-check:",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := c.prepare(t)
			var stdout, stderr bytes.Buffer
			code := run([]string{"-root", root}, &stdout, &stderr)
			if code != c.wantCode {
				t.Fatalf("exit code = %d, want %d; stdout=%q stderr=%q", code, c.wantCode, stdout.String(), stderr.String())
			}
			for _, want := range c.wantStdout {
				if !strings.Contains(stdout.String(), want) {
					t.Fatalf("stdout = %q, want containing %q", stdout.String(), want)
				}
			}
			if c.wantStderr != "" && !strings.Contains(stderr.String(), c.wantStderr) {
				t.Fatalf("stderr = %q, want containing %q", stderr.String(), c.wantStderr)
			}
			if c.wantCode == 0 && stderr.Len() != 0 {
				t.Fatalf("stderr should be empty, got %q", stderr.String())
			}
		})
	}
}

func TestRun_BadFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-unknown"}, &stdout, &stderr); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestRun_ProblemsOneLineEach(t *testing.T) {
	root := newFixture(t)
	for _, rel := range []string{"Makefile", "AGENTS.md"} {
		if err := os.Remove(filepath.Join(root, rel)); err != nil {
			t.Fatal(err)
		}
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-root", root}, &stdout, &stderr); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	lines := strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n")
	if len(lines) != 3 || lines[2] != "spec-check: 2 problem(s)" {
		t.Fatalf("stdout lines = %q", lines)
	}
}

func TestRun_WrongFileTypeIsProblem(t *testing.T) {
	root := newFixture(t)
	if err := os.Remove(filepath.Join(root, "Makefile")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "Makefile"), 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-root", root}, &stdout, &stderr); code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Makefile: 不是普通文件") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}
