package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const plan = "# 0.1.0 技术方案\n\n## 交付状态\n\n```yaml\nstage: %s\nuser_acceptance: %s\nreview: %s\n```\n"

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeDelivery(t *testing.T, root, stage, ua, review string) {
	t.Helper()
	doc := plan
	for _, v := range []string{stage, ua, review} {
		doc = strings.Replace(doc, "%s", v, 1)
	}
	writeFile(t, root, "Specs/technical/0.1.0/技术方案.md", doc)
}

func newFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.com/fixture\n")
	writeFile(t, root, "Makefile", "check:\n")
	writeFile(t, root, "cmd/api/main.go", "package main\n")
	writeFile(t, root, "internal/x/x.go", "package x\n")
	writeFile(t, root, "Specs/requirements/0.1.0/需求.md", "# 需求\n")
	writeFile(t, root, "Specs/requirements/协议与数据.md", "# 协议\n")
	writeDelivery(t, root, "planning", "pending", "not_required")
	return root
}

type result struct {
	code   int
	stdout string
	stderr string
}

func exec(t *testing.T, root string, args ...string) result {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(append([]string{"--root", root}, args...), &out, &errb)
	return result{code, out.String(), errb.String()}
}

func mustOK(t *testing.T, root string, args ...string) result {
	t.Helper()
	r := exec(t, root, args...)
	if r.code != 0 {
		t.Fatalf("%v: code=%d stderr=%s", args, r.code, r.stderr)
	}
	return r
}

func TestRun_ExitCodes(t *testing.T) {
	cases := []struct {
		name   string
		setup  func(t *testing.T, root string)
		args   []string
		code   int
		stdout string
		stderr string
	}{
		{"无参数", nil, nil, 2, "", "用法"},
		{"未知子命令", nil, []string{"frobnicate", "0.1.0"}, 2, "", "未知子命令"},
		{"非法 version", nil, []string{"init", "v0.1.0"}, 2, "", "version"},
		{"多余参数", nil, []string{"init", "0.1.0", "extra"}, 2, "", "多余参数"},
		{"未初始化 status", nil, []string{"status", "0.1.0"}, 1, "", "init"},
		{"未初始化 check", nil, []string{"check", "0.1.0"}, 1, "", "init"},
		{"init 成功", nil, []string{"init", "0.1.0"}, 0, "stage=planning revision=0\n", ""},
		{"重复 init 冲突", func(t *testing.T, root string) { mustOK(t, root, "init", "0.1.0") }, []string{"init", "0.1.0"}, 3, "", "已存在"},
		{"record 未知 kind", func(t *testing.T, root string) { mustOK(t, root, "init", "0.1.0") },
			[]string{"record", "0.1.0", "--kind", "lint", "--exit", "0"}, 2, "", "kind"},
		{"record 缺 --exit", func(t *testing.T, root string) { mustOK(t, root, "init", "0.1.0") },
			[]string{"record", "0.1.0", "--kind", "check"}, 2, "", "--exit"},
		{"record 未知 flag", func(t *testing.T, root string) { mustOK(t, root, "init", "0.1.0") },
			[]string{"record", "0.1.0", "--kind", "check", "--exit", "0", "--bogus"}, 2, "", "bogus"},
		{"record 成功", func(t *testing.T, root string) { mustOK(t, root, "init", "0.1.0") },
			[]string{"record", "0.1.0", "--kind", "check", "--exit", "0"}, 0, "stage=planning revision=1\n", ""},
		{"record 日志不存在", func(t *testing.T, root string) { mustOK(t, root, "init", "0.1.0") },
			[]string{"record", "0.1.0", "--kind", "check", "--exit", "0", "--log", "/nonexistent/x.log"}, 2, "", "x.log"},
		{"expect-revision 冲突", func(t *testing.T, root string) { mustOK(t, root, "init", "0.1.0") },
			[]string{"record", "0.1.0", "--kind", "check", "--exit", "0", "--expect-revision", "9"}, 3, "", "revision"},
		{"finding 成功", func(t *testing.T, root string) { mustOK(t, root, "init", "0.1.0") },
			[]string{"finding", "0.1.0", "--id", "R1", "--severity", "P1", "--status", "open", "--note", "说明"}, 0, "stage=planning revision=1\n", ""},
		{"finding 非法转换", func(t *testing.T, root string) { mustOK(t, root, "init", "0.1.0") },
			[]string{"finding", "0.1.0", "--id", "R1", "--severity", "P1", "--status", "fixed"}, 3, "", "open"},
		{"finding 缺参数", func(t *testing.T, root string) { mustOK(t, root, "init", "0.1.0") },
			[]string{"finding", "0.1.0", "--id", "R1"}, 2, "", "--severity"},
		{"event 守卫拒绝", func(t *testing.T, root string) { mustOK(t, root, "init", "0.1.0") },
			[]string{"event", "0.1.0", "plan_confirmed"}, 3, "", "planning"},
		{"event 缺类型", func(t *testing.T, root string) { mustOK(t, root, "init", "0.1.0") },
			[]string{"event", "0.1.0"}, 2, "", "类型"},
		{"event 未知类型", func(t *testing.T, root string) { mustOK(t, root, "init", "0.1.0") },
			[]string{"event", "0.1.0", "warp"}, 2, "", "warp"},
		{"event 成功带 --at 与 --id", func(t *testing.T, root string) {
			mustOK(t, root, "init", "0.1.0")
			writeDelivery(t, root, "implementing", "pending", "not_required")
		}, []string{"event", "0.1.0", "plan_confirmed", "--id", "e1", "--at", "2026-03-04T05:06:07+08:00"}, 0, "stage=implementing revision=1\n", ""},
		{"event 非法 --at", func(t *testing.T, root string) { mustOK(t, root, "init", "0.1.0") },
			[]string{"event", "0.1.0", "plan_confirmed", "--at", "yesterday"}, 2, "", "--at"},
		{"check 无问题", func(t *testing.T, root string) { mustOK(t, root, "init", "0.1.0") },
			[]string{"check", "0.1.0"}, 0, "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := newFixture(t)
			if c.setup != nil {
				c.setup(t, root)
			}
			r := exec(t, root, c.args...)
			if r.code != c.code {
				t.Fatalf("code = %d, want %d (stdout=%q stderr=%q)", r.code, c.code, r.stdout, r.stderr)
			}
			if c.stdout != "" && r.stdout != c.stdout {
				t.Fatalf("stdout = %q, want %q", r.stdout, c.stdout)
			}
			if c.stderr != "" && !strings.Contains(r.stderr, c.stderr) {
				t.Fatalf("stderr = %q, want containing %q", r.stderr, c.stderr)
			}
		})
	}
}

func TestRun_CheckProblems(t *testing.T) {
	root := newFixture(t)
	mustOK(t, root, "init", "0.1.0")
	writeDelivery(t, root, "delivered", "confirmed", "pass")
	r := exec(t, root, "check", "0.1.0")
	if r.code != 1 || !strings.Contains(r.stdout, "Specs/technical/0.1.0/技术方案.md: ") || !strings.Contains(r.stdout, "delivered") {
		t.Fatalf("code=%d stdout=%q stderr=%q", r.code, r.stdout, r.stderr)
	}
}

func TestRun_EventAt(t *testing.T) {
	root := newFixture(t)
	mustOK(t, root, "init", "0.1.0")
	writeDelivery(t, root, "implementing", "pending", "not_required")
	mustOK(t, root, "event", "0.1.0", "plan_confirmed", "--at", "2026-03-04T05:06:07+08:00")
	raw, err := os.ReadFile(filepath.Join(root, "Specs", "technical", "0.1.0", "graph.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"at": "2026-03-03T21:06:07Z"`) {
		t.Fatalf("--at 未转为 UTC 写入:\n%s", raw)
	}
}

func TestRun_Status(t *testing.T) {
	root := newFixture(t)
	mustOK(t, root, "init", "0.1.0")
	mustOK(t, root, "record", "0.1.0", "--kind", "check", "--exit", "1")
	writeFile(t, root, "internal/x/x.go", "package x\n// drift\n")

	r := mustOK(t, root, "status", "0.1.0")
	for _, want := range []string{"version: 0.1.0", "stage: planning", "revision: 1", "invalidated: false", "drifted_candidate: true", "check: exit_code=1", "delivery: stage=planning"} {
		if !strings.Contains(r.stdout, want) {
			t.Fatalf("stdout 缺少 %q:\n%s", want, r.stdout)
		}
	}

	r = mustOK(t, root, "status", "0.1.0", "--json")
	var s struct {
		Version          string `json:"version"`
		Stage            string `json:"stage"`
		Revision         int    `json:"revision"`
		DriftedCandidate bool   `json:"drifted_candidate"`
		LatestEvidence   map[string]struct {
			ExitCode int `json:"exit_code"`
		} `json:"latest_evidence"`
		Delivery struct {
			Stage string `json:"stage"`
		} `json:"delivery"`
	}
	if err := json.Unmarshal([]byte(r.stdout), &s); err != nil {
		t.Fatalf("json: %v\n%s", err, r.stdout)
	}
	if s.Version != "0.1.0" || s.Stage != "planning" || s.Revision != 1 || !s.DriftedCandidate || s.LatestEvidence["check"].ExitCode != 1 || s.Delivery.Stage != "planning" {
		t.Fatalf("status = %+v", s)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(r.stdout), &m); err != nil {
		t.Fatal(err)
	}
	var delivery map[string]string
	if err := json.Unmarshal(m["delivery"], &delivery); err != nil {
		t.Fatal(err)
	}
	if delivery["stage"] != "planning" || delivery["user_acceptance"] != "pending" || delivery["review"] != "not_required" || len(delivery) != 3 {
		t.Fatalf("delivery 键应为 snake_case: %s", m["delivery"])
	}
}

func TestRun_RootEqualsForm(t *testing.T) {
	root := newFixture(t)
	var out, errb bytes.Buffer
	if code := run([]string{"--root=" + root, "init", "0.1.0"}, &out, &errb); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, errb.String())
	}
	if code := run([]string{"--root"}, &out, &errb); code != 2 {
		t.Fatalf("--root 缺值应为用法错误, code=%d", code)
	}
	if code := run([]string{"--root="}, &out, &errb); code != 2 {
		t.Fatalf("--root= 空值应为用法错误, code=%d", code)
	}
}

func TestRun_RootOnlyBeforeSubcommand(t *testing.T) {
	root := newFixture(t)
	mustOK(t, root, "init", "0.1.0")
	mustOK(t, root, "finding", "0.1.0", "--id", "R1", "--severity", "P1", "--status", "open", "--note", "--root")
	raw, err := os.ReadFile(filepath.Join(root, "Specs", "technical", "0.1.0", "graph.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"note": "--root"`) {
		t.Fatalf("--note 的取值不应被当作 --root:\n%s", raw)
	}
	var out, errb bytes.Buffer
	if code := run([]string{"status", "0.1.0", "--root", root}, &out, &errb); code != 2 {
		t.Fatalf("子命令之后的 --root 应为用法错误, code=%d stderr=%s", code, errb.String())
	}
}
