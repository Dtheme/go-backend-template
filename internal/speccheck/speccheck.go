// Package speccheck 核对模板仓库的 Spec 一致性：必需文件与章节、符号链接、版本目录、Feature ID 追溯、交付状态与 Postman 集合。
package speccheck

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
)

// Problem 是一条规则违反，Path 相对仓库根。
type Problem struct {
	Path    string
	Message string
}

func (p Problem) String() string {
	return p.Path + ": " + p.Message
}

var requiredFiles = []string{
	"Specs/requirements/需求模版.md",
	"Specs/requirements/协议与数据.md",
	"Specs/technical/技术讲解.md",
	"Specs/technical/技术方案模版.md",
	".ai/ai-rules.md",
	".ai/memory.md",
	"Makefile",
}

var rulesHeadings = []string{"## 文件权限规则", "## AI 工作流（版本开发）", "## 验证命令"}

var overviewHeadings = []string{"## 目录结构", "## 接口清单", "## 测试策略", "## 已知问题与待优化项"}

var memoryHeadings = []string{"## 已验证的事实", "## 踩过的坑", "## 待验证"}

var requiredSymlinks = map[string]string{
	"CLAUDE.md":      ".ai/ai-rules.md",
	"AGENTS.md":      ".ai/ai-rules.md",
	".claude/skills": "../.ai/skills",
	".agents/skills": "../.ai/skills",
}

// checker 汇总一次 Run 的问题；I/O 故障通过 err 提前终止。
type checker struct {
	root     string
	problems []Problem
}

func (c *checker) add(path, format string, args ...any) {
	c.problems = append(c.problems, Problem{Path: path, Message: fmt.Sprintf(format, args...)})
}

// missing 判断路径不存在，包括父路径是普通文件导致的 ENOTDIR。
func missing(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)
}

// read 返回文件内容；文件缺失或不是普通文件时记录问题并返回 nil，其他 I/O 错误向上返回。
func (c *checker) read(rel string) ([]byte, error) {
	full := filepath.Join(c.root, rel)
	info, err := os.Stat(full)
	if missing(err) {
		c.add(rel, "缺少必需文件")
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取 %s: %w", rel, err)
	}
	if !info.Mode().IsRegular() {
		c.add(rel, "不是普通文件")
		return nil, nil
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return nil, fmt.Errorf("读取 %s: %w", rel, err)
	}
	return data, nil
}

// Run 对 root 执行全部检查，返回按 Path、Message 排序的问题列表；error 只表示 I/O 故障。
func Run(root string) ([]Problem, error) {
	if _, err := os.Stat(root); err != nil {
		return nil, fmt.Errorf("读取仓库根: %w", err)
	}
	c := &checker{root: root}
	steps := []func() error{
		c.checkRequiredFiles,
		c.checkSymlinks,
		c.checkVersions,
		c.checkPostman,
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return nil, err
		}
	}
	sort.Slice(c.problems, func(i, j int) bool {
		if c.problems[i].Path != c.problems[j].Path {
			return c.problems[i].Path < c.problems[j].Path
		}
		return c.problems[i].Message < c.problems[j].Message
	})
	return c.problems, nil
}

func (c *checker) checkRequiredFiles() error {
	for _, rel := range requiredFiles {
		data, err := c.read(rel)
		if err != nil {
			return err
		}
		if data == nil {
			continue
		}
		switch rel {
		case ".ai/ai-rules.md":
			c.checkHeadings(rel, data, rulesHeadings, true)
		case "Specs/technical/技术讲解.md":
			c.checkHeadings(rel, data, overviewHeadings, false)
		case ".ai/memory.md":
			c.checkHeadings(rel, data, memoryHeadings, false)
		}
	}
	return nil
}

// checkHeadings 要求每个二级标题独占一行出现；exactlyOnce 时重复也算问题。围栏代码块内的行不计数。
func (c *checker) checkHeadings(rel string, doc []byte, headings []string, exactlyOnce bool) {
	counts := map[string]int{}
	fence := ""
	for _, line := range strings.Split(string(doc), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case fence == "" && (strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")):
			fence = trimmed[:3]
		case fence != "" && strings.HasPrefix(trimmed, fence):
			fence = ""
		case fence == "":
			counts[strings.TrimRight(line, " \t\r")]++
		}
	}
	for _, h := range headings {
		switch n := counts[h]; {
		case n == 0:
			c.add(rel, "缺少章节「%s」", h)
		case n > 1 && exactlyOnce:
			c.add(rel, "章节「%s」重复 %d 次", h, n)
		}
	}
}

func (c *checker) checkSymlinks() error {
	links := make([]string, 0, len(requiredSymlinks))
	for rel := range requiredSymlinks {
		links = append(links, rel)
	}
	sort.Strings(links)
	for _, rel := range links {
		want := requiredSymlinks[rel]
		full := filepath.Join(c.root, rel)
		info, err := os.Lstat(full)
		if errors.Is(err, fs.ErrNotExist) {
			c.add(rel, "缺少符号链接，应指向 %s", want)
			continue
		}
		if err != nil {
			return fmt.Errorf("读取 %s: %w", rel, err)
		}
		if info.Mode()&fs.ModeSymlink == 0 {
			c.add(rel, "不是符号链接，应指向 %s", want)
			continue
		}
		got, err := os.Readlink(full)
		if err != nil {
			return fmt.Errorf("读取符号链接 %s: %w", rel, err)
		}
		if got != want {
			c.add(rel, "符号链接目标应为 %s，实际为 %s", want, got)
			continue
		}
		if _, err := os.Stat(full); err != nil {
			c.add(rel, "符号链接目标不存在: %s", want)
		}
	}
	return nil
}

func (c *checker) checkPostman() error {
	const dir = "api/postman"
	entries, err := os.ReadDir(filepath.Join(c.root, dir))
	if missing(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("读取 %s: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".postman_collection.json") {
			continue
		}
		rel := dir + "/" + e.Name()
		data, err := c.read(rel)
		if err != nil {
			return err
		}
		if data == nil {
			continue
		}
		if msg := validatePostman(data); msg != "" {
			c.add(rel, "%s", msg)
		}
	}
	return nil
}

func validatePostman(data []byte) string {
	var col map[string]json.RawMessage
	if err := json.Unmarshal(data, &col); err != nil {
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) {
			return "顶层必须是 JSON 对象"
		}
		return "不是合法 JSON: " + err.Error()
	}
	// JSON null 能解码进 map 而不报错，需单独排除。
	if col == nil {
		return "顶层必须是 JSON 对象"
	}
	var info struct {
		Name json.RawMessage `json:"name"`
	}
	if raw := col["info"]; raw != nil && json.Unmarshal(raw, &info) != nil {
		return "info 必须是 JSON 对象"
	}
	var name string
	if info.Name == nil || json.Unmarshal(info.Name, &name) != nil || name == "" {
		return "info.name 必须是非空字符串"
	}
	var items []json.RawMessage
	if raw := col["item"]; raw == nil || json.Unmarshal(raw, &items) != nil || items == nil {
		return "item 必须是数组"
	}
	return ""
}
