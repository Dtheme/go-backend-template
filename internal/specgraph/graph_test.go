package specgraph

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"example.com/go-backend-template/internal/specdoc"
)

var fixedNow = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

const (
	version   = "0.1.0"
	graphRel  = "Specs/technical/0.1.0/graph.json"
	planRel   = "Specs/technical/0.1.0/技术方案.md"
	reqRel    = "Specs/requirements/0.1.0/需求.md"
	protoRel  = "Specs/requirements/协议与数据.md"
	codeRel   = "internal/x/x.go"
	deliveryT = "# 0.1.0 技术方案\n\n## 交付状态\n\n```yaml\nstage: %s\nuser_acceptance: %s\nreview: %s\n```\n\n## 变更记录\n"
)

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
	writeFile(t, root, planRel, sprintf(deliveryT, stage, ua, review))
}

func sprintf(format string, args ...string) string {
	out := format
	for _, a := range args {
		out = strings.Replace(out, "%s", a, 1)
	}
	return out
}

// newFixture 建最小工程；交付状态默认 planning / pending / not_required。
func newFixture(t *testing.T) (Graph, string) {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.com/fixture\n\ngo 1.25.0\n")
	writeFile(t, root, "Makefile", "check:\n\tgo test ./...\n")
	writeFile(t, root, "cmd/api/main.go", "package main\n\nfunc main() {}\n")
	writeFile(t, root, codeRel, "package x\n")
	writeFile(t, root, reqRel, "# 0.1.0 需求\n\n## F-0.1.0-001\n")
	writeFile(t, root, protoRel, "# 协议与数据\n")
	writeDelivery(t, root, "planning", "pending", "not_required")
	g := Graph{Root: root, Version: version, Now: func() time.Time { return fixedNow }}
	return g, root
}

func mustInit(t *testing.T, g Graph) State {
	t.Helper()
	st, err := g.Init()
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	return st
}

func mustApply(t *testing.T, g Graph, typ string) State {
	t.Helper()
	st, err := g.Apply(typ, "", nil)
	if err != nil {
		t.Fatalf("apply %s: %v", typ, err)
	}
	return st
}

func mustRecord(t *testing.T, g Graph, kind string, exit int) State {
	t.Helper()
	st, err := g.Record(kind, exit, "", nil)
	if err != nil {
		t.Fatalf("record %s: %v", kind, err)
	}
	return st
}

func mustFinding(t *testing.T, g Graph, id, status string) State {
	t.Helper()
	st, err := g.Finding(id, "P1", status, "n", nil)
	if err != nil {
		t.Fatalf("finding %s %s: %v", id, status, err)
	}
	return st
}

// toStage 沿正向路径把已 init 的图推进到目标阶段，并同步交付状态。
func toStage(t *testing.T, g Graph, root, target string) State {
	t.Helper()
	st, err := g.Load()
	if err != nil {
		t.Fatal(err)
	}
	steps := []struct {
		at    string
		setup func()
		event string
	}{
		{"planning", func() { writeDelivery(t, root, "implementing", "pending", "not_required") }, "plan_confirmed"},
		{"implementing", func() { mustRecord(t, g, "check", 0) }, "implementation_done"},
		{"reviewing", func() { mustRecord(t, g, "review", 0) }, "review_passed"},
		{"verifying", func() {
			// 与 SKILL Step D 一致：先记录证据，再由用户确认验收；交付状态块不计入 inputs 摘要。
			for _, k := range []string{"check", "test-integration", "smoke", "api-verify"} {
				mustRecord(t, g, k, 0)
			}
			writeDelivery(t, root, "verifying", "confirmed", "pass")
		}, "verified"},
	}
	for _, s := range steps {
		if st.Stage == target {
			return st
		}
		if st.Stage != s.at {
			t.Fatalf("toStage: 当前 %s 无法到 %s", st.Stage, target)
		}
		s.setup()
		st = mustApply(t, g, s.event)
	}
	if st.Stage != target {
		t.Fatalf("toStage: 到达 %s 而非 %s", st.Stage, target)
	}
	return st
}

func wantErr(t *testing.T, err, sentinel error, contains string) {
	t.Helper()
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want %v", err, sentinel)
	}
	if contains != "" && !strings.Contains(err.Error(), contains) {
		t.Fatalf("err = %v, want containing %q", err, contains)
	}
}

func readGraphJSON(t *testing.T, root string) map[string]json.RawMessage {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(graphRel)))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func sha(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func TestInit(t *testing.T) {
	t.Run("成功", func(t *testing.T) {
		g, root := newFixture(t)
		writeFile(t, root, "internal/.hidden/h.go", "package h\n")
		writeFile(t, root, "internal/x/.DS_Store", "junk")
		writeFile(t, root, "scripts/smoke.sh", "echo ok\n")
		writeFile(t, root, "docs/readme.md", "not candidate\n")
		if err := os.Symlink(filepath.Join(root, "go.mod"), filepath.Join(root, "internal", "x", "link.go")); err != nil {
			t.Fatal(err)
		}
		st := mustInit(t, g)
		if st.Version != version || st.Revision != 0 || st.Stage != "planning" {
			t.Fatalf("state = %+v", st)
		}
		wantCandidate := []string{"Makefile", "cmd/api/main.go", "go.mod", "internal/x/x.go", "scripts/smoke.sh"}
		var got []string
		for p := range st.Candidate.Files {
			got = append(got, p)
		}
		sort.Strings(got)
		if strings.Join(got, ",") != strings.Join(wantCandidate, ",") {
			t.Fatalf("candidate files = %v, want %v", got, wantCandidate)
		}
		if len(st.Inputs.Files) != 3 || st.Inputs.Files[reqRel] != sha("# 0.1.0 需求\n\n## F-0.1.0-001\n") {
			t.Fatalf("inputs = %+v", st.Inputs)
		}
		var buf strings.Builder
		for _, p := range wantCandidate {
			buf.WriteString(p + "\n" + st.Candidate.Files[p] + "\n")
		}
		if st.Candidate.Digest != sha(buf.String()) {
			t.Fatalf("digest 不符合 排序(路径\\nsha\\n) 拼接规则")
		}
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(graphRel)))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(string(raw), "{\n  \"version\": \"0.1.0\",\n  \"revision\": 0,\n  \"stage\": \"planning\",\n  \"inputs\":") {
			t.Fatalf("json 键顺序或缩进不符:\n%s", raw)
		}
		if !strings.Contains(string(raw), "\"findings\": []") {
			t.Fatalf("空数组应写成 []:\n%s", raw)
		}
		st2, err := g.Load()
		if err != nil {
			t.Fatal(err)
		}
		if st2.Candidate.Digest != st.Candidate.Digest || st2.Stage != "planning" {
			t.Fatalf("load = %+v", st2)
		}
	})
	t.Run("重复 init 冲突", func(t *testing.T) {
		g, _ := newFixture(t)
		mustInit(t, g)
		_, err := g.Init()
		wantErr(t, err, ErrConflict, "")
	})
	t.Run("缺少输入文件", func(t *testing.T) {
		g, root := newFixture(t)
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(reqRel))); err != nil {
			t.Fatal(err)
		}
		if _, err := g.Init(); err == nil || !strings.Contains(err.Error(), "需求.md") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("未初始化 Load", func(t *testing.T) {
		g, _ := newFixture(t)
		_, err := g.Load()
		wantErr(t, err, ErrNotInitialized, "")
		_, err = g.Status()
		wantErr(t, err, ErrNotInitialized, "")
	})
}

func TestApply_PlanConfirmed(t *testing.T) {
	g, root := newFixture(t)
	mustInit(t, g)
	_, err := g.Apply("plan_confirmed", "", nil)
	wantErr(t, err, ErrGuard, "planning")
	writeDelivery(t, root, "implementing", "pending", "not_required")
	st := mustApply(t, g, "plan_confirmed")
	if st.Stage != "implementing" || st.Revision != 1 || len(st.Events) != 1 {
		t.Fatalf("state = %+v", st)
	}
	ev := st.Events[0]
	if ev.ID != "evt-1" || ev.Type != "plan_confirmed" || ev.From != "planning" || ev.To != "implementing" || ev.Revision != 1 || ev.At != "2026-01-02T03:04:05Z" {
		t.Fatalf("event = %+v", ev)
	}
}

func TestApply_ImplementationDone(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, g Graph, root string)
		want  string
	}{
		{"无 check 证据", func(t *testing.T, g Graph, root string) {}, "check"},
		{"check 失败", func(t *testing.T, g Graph, root string) { mustRecord(t, g, "check", 1) }, "exit_code"},
		{"check 后代码变化", func(t *testing.T, g Graph, root string) {
			mustRecord(t, g, "check", 0)
			writeFile(t, root, codeRel, "package x\n\nvar changed = 1\n")
		}, "candidate"},
		{"check 后需求变化", func(t *testing.T, g Graph, root string) {
			mustRecord(t, g, "check", 0)
			writeFile(t, root, reqRel, "# 变了\n")
		}, "inputs"},
		{"旧成功被新失败覆盖", func(t *testing.T, g Graph, root string) {
			mustRecord(t, g, "check", 0)
			mustRecord(t, g, "check", 1)
		}, "exit_code"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g, root := newFixture(t)
			mustInit(t, g)
			toStage(t, g, root, "implementing")
			c.setup(t, g, root)
			_, err := g.Apply("implementation_done", "", nil)
			wantErr(t, err, ErrGuard, c.want)
		})
	}
	t.Run("成功", func(t *testing.T) {
		g, root := newFixture(t)
		mustInit(t, g)
		toStage(t, g, root, "implementing")
		mustRecord(t, g, "check", 1)
		mustRecord(t, g, "check", 0)
		st := mustApply(t, g, "implementation_done")
		if st.Stage != "reviewing" {
			t.Fatalf("stage = %s", st.Stage)
		}
	})
}

func TestApply_ReviewFailed(t *testing.T) {
	g, root := newFixture(t)
	mustInit(t, g)
	toStage(t, g, root, "reviewing")
	_, err := g.Apply("review_failed", "", nil)
	wantErr(t, err, ErrGuard, "finding")
	mustFinding(t, g, "R1", "open")
	mustFinding(t, g, "R1", "rejected")
	_, err = g.Apply("review_failed", "", nil)
	wantErr(t, err, ErrGuard, "finding")
	mustFinding(t, g, "R2", "open")
	mustFinding(t, g, "R2", "accepted")
	if st := mustApply(t, g, "review_failed"); st.Stage != "fixing" {
		t.Fatalf("stage = %s", st.Stage)
	}
}

func TestApply_ReviewPassed(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, g Graph, root string)
		want  string
	}{
		{"无 review 证据", func(t *testing.T, g Graph, root string) {}, "review"},
		{"review 失败", func(t *testing.T, g Graph, root string) { mustRecord(t, g, "review", 1) }, "exit_code"},
		{"review 后代码变化", func(t *testing.T, g Graph, root string) {
			mustRecord(t, g, "review", 0)
			writeFile(t, root, codeRel, "package x\n// changed\n")
		}, "candidate"},
		{"存在 fixed finding", func(t *testing.T, g Graph, root string) {
			mustFinding(t, g, "R1", "open")
			mustFinding(t, g, "R1", "accepted")
			mustFinding(t, g, "R1", "fixed")
			mustRecord(t, g, "review", 0)
		}, "finding"},
		{"存在 open finding", func(t *testing.T, g Graph, root string) {
			mustFinding(t, g, "R1", "open")
			mustRecord(t, g, "review", 0)
		}, "finding"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g, root := newFixture(t)
			mustInit(t, g)
			toStage(t, g, root, "reviewing")
			c.setup(t, g, root)
			_, err := g.Apply("review_passed", "", nil)
			wantErr(t, err, ErrGuard, c.want)
		})
	}
	t.Run("成功且不要求 inputs 绑定", func(t *testing.T) {
		g, root := newFixture(t)
		mustInit(t, g)
		toStage(t, g, root, "reviewing")
		mustFinding(t, g, "R1", "open")
		mustFinding(t, g, "R1", "rejected")
		mustRecord(t, g, "review", 0)
		if st := mustApply(t, g, "review_passed"); st.Stage != "verifying" {
			t.Fatalf("stage = %s", st.Stage)
		}
	})
}

func TestApply_FixDone(t *testing.T) {
	setup := func(t *testing.T) (Graph, string) {
		g, root := newFixture(t)
		mustInit(t, g)
		toStage(t, g, root, "reviewing")
		mustFinding(t, g, "R1", "open")
		mustApply(t, g, "review_failed")
		return g, root
	}
	t.Run("open finding", func(t *testing.T) {
		g, _ := setup(t)
		mustRecord(t, g, "check", 0)
		_, err := g.Apply("fix_done", "", nil)
		wantErr(t, err, ErrGuard, "open")
	})
	t.Run("check 未绑定当前候选", func(t *testing.T) {
		g, root := setup(t)
		mustFinding(t, g, "R1", "accepted")
		mustRecord(t, g, "check", 0)
		writeFile(t, root, codeRel, "package x\n// fix\n")
		_, err := g.Apply("fix_done", "", nil)
		wantErr(t, err, ErrGuard, "candidate")
	})
	t.Run("成功", func(t *testing.T) {
		g, root := setup(t)
		mustFinding(t, g, "R1", "accepted")
		writeFile(t, root, codeRel, "package x\n// fix\n")
		mustFinding(t, g, "R1", "fixed")
		mustRecord(t, g, "check", 0)
		if st := mustApply(t, g, "fix_done"); st.Stage != "reviewing" {
			t.Fatalf("stage = %s", st.Stage)
		}
	})
}

func TestApply_Verified(t *testing.T) {
	all := []string{"check", "test-integration", "smoke", "api-verify"}
	recordAll := func(t *testing.T, g Graph) {
		for _, k := range all {
			mustRecord(t, g, k, 0)
		}
	}
	cases := []struct {
		name  string
		setup func(t *testing.T, g Graph, root string)
		want  string
	}{
		{"缺 smoke", func(t *testing.T, g Graph, root string) {
			writeDelivery(t, root, "verifying", "confirmed", "pass")
			for _, k := range []string{"check", "test-integration", "api-verify"} {
				mustRecord(t, g, k, 0)
			}
		}, "smoke"},
		{"test-race 失败", func(t *testing.T, g Graph, root string) {
			writeDelivery(t, root, "verifying", "confirmed", "pass")
			recordAll(t, g)
			mustRecord(t, g, "test-race", 1)
		}, "test-race"},
		{"证据后需求变化", func(t *testing.T, g Graph, root string) {
			writeDelivery(t, root, "verifying", "confirmed", "pass")
			recordAll(t, g)
			writeFile(t, root, reqRel, "# 变化\n")
		}, "inputs"},
		{"证据后技术方案正文变化", func(t *testing.T, g Graph, root string) {
			writeDelivery(t, root, "verifying", "confirmed", "pass")
			recordAll(t, g)
			writeFile(t, root, planRel, sprintf(deliveryT, "verifying", "confirmed", "pass")+"\n## 新增一节\n")
		}, "inputs"},
		{"finding 未关闭", func(t *testing.T, g Graph, root string) {
			writeDelivery(t, root, "verifying", "confirmed", "pass")
			recordAll(t, g)
			mustFinding(t, g, "R9", "open")
			mustFinding(t, g, "R9", "accepted")
		}, "finding"},
		{"用户未确认", func(t *testing.T, g Graph, root string) {
			writeDelivery(t, root, "verifying", "pending", "pass")
			recordAll(t, g)
		}, "user_acceptance"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g, root := newFixture(t)
			mustInit(t, g)
			toStage(t, g, root, "verifying")
			c.setup(t, g, root)
			_, err := g.Apply("verified", "", nil)
			wantErr(t, err, ErrGuard, c.want)
		})
	}
	t.Run("证据后再确认验收不破坏 inputs 绑定", func(t *testing.T) {
		g, root := newFixture(t)
		mustInit(t, g)
		toStage(t, g, root, "verifying")
		recordAll(t, g)
		writeDelivery(t, root, "verifying", "confirmed", "pass")
		if st := mustApply(t, g, "verified"); st.Stage != "ready_to_deliver" {
			t.Fatalf("stage = %s", st.Stage)
		}
	})
	t.Run("成功含 test-race", func(t *testing.T) {
		g, root := newFixture(t)
		mustInit(t, g)
		toStage(t, g, root, "verifying")
		writeDelivery(t, root, "verifying", "confirmed", "pass")
		recordAll(t, g)
		mustRecord(t, g, "test-race", 0)
		mustFinding(t, g, "R1", "open")
		mustFinding(t, g, "R1", "rejected")
		if st := mustApply(t, g, "verified"); st.Stage != "ready_to_deliver" {
			t.Fatalf("stage = %s", st.Stage)
		}
	})
}

func TestApply_ReadinessInvalidated(t *testing.T) {
	for _, from := range []string{"verifying", "ready_to_deliver"} {
		t.Run(from, func(t *testing.T) {
			g, root := newFixture(t)
			mustInit(t, g)
			toStage(t, g, root, from)
			_, err := g.Apply("readiness_invalidated", "", nil)
			wantErr(t, err, ErrGuard, "失效")
			writeFile(t, root, codeRel, "package x\n// drift\n")
			st := mustApply(t, g, "readiness_invalidated")
			if st.Stage != "fixing" {
				t.Fatalf("stage = %s", st.Stage)
			}
			s, err := g.Status()
			if err != nil {
				t.Fatal(err)
			}
			if s.Invalidated || s.DriftedCandidate {
				t.Fatalf("事件后应刷新摘要: %+v", s)
			}
		})
	}
}

func TestApply_Illegal(t *testing.T) {
	g, root := newFixture(t)
	mustInit(t, g)
	_, err := g.Apply("implementation_done", "", nil)
	wantErr(t, err, ErrGuard, "非法转换")
	_, err = g.Apply("readiness_invalidated", "", nil)
	wantErr(t, err, ErrGuard, "非法转换")
	_, err = g.Apply("teleport", "", nil)
	wantErr(t, err, ErrUsage, "")
	toStage(t, g, root, "implementing")
	_, err = g.Apply("plan_confirmed", "", nil)
	wantErr(t, err, ErrGuard, "非法转换")
}

func TestFinding(t *testing.T) {
	type step struct {
		status string
		err    error
	}
	cases := []struct {
		name  string
		steps []step
	}{
		{"新建必须 open", []step{{"accepted", ErrGuard}}},
		{"open->accepted->fixed->verified", []step{{"open", nil}, {"accepted", nil}, {"fixed", nil}, {"verified", nil}}},
		{"open->rejected 终态", []step{{"open", nil}, {"rejected", nil}, {"accepted", ErrGuard}, {"open", ErrGuard}}},
		{"open->fixed 非法", []step{{"open", nil}, {"fixed", ErrGuard}}},
		{"open->verified 非法", []step{{"open", nil}, {"verified", ErrGuard}}},
		{"accepted->rejected 非法", []step{{"open", nil}, {"accepted", nil}, {"rejected", ErrGuard}}},
		{"accepted->verified 非法", []step{{"open", nil}, {"accepted", nil}, {"verified", ErrGuard}}},
		{"fixed->open 非法", []step{{"open", nil}, {"accepted", nil}, {"fixed", nil}, {"open", ErrGuard}}},
		{"verified 终态", []step{{"open", nil}, {"accepted", nil}, {"fixed", nil}, {"verified", nil}, {"fixed", ErrGuard}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g, _ := newFixture(t)
			mustInit(t, g)
			for _, s := range c.steps {
				_, err := g.Finding("R1", "P2", s.status, "note", nil)
				if s.err == nil && err != nil {
					t.Fatalf("status %s: %v", s.status, err)
				}
				if s.err != nil {
					wantErr(t, err, s.err, "")
				}
			}
		})
	}
	t.Run("相同值幂等", func(t *testing.T) {
		g, _ := newFixture(t)
		mustInit(t, g)
		st1 := mustFinding(t, g, "R1", "open")
		st2, err := g.Finding("R1", "P1", "open", "n", nil)
		if err != nil {
			t.Fatal(err)
		}
		if st2.Revision != st1.Revision || len(st2.Findings) != 1 {
			t.Fatalf("幂等不应改变状态: %+v", st2)
		}
	})
	t.Run("同状态不同 severity 与 note 仍幂等且不写盘", func(t *testing.T) {
		g, _ := newFixture(t)
		mustInit(t, g)
		st1 := mustFinding(t, g, "R1", "open")
		st2, err := g.Finding("R1", "P0", "open", "升级", nil)
		if err != nil {
			t.Fatal(err)
		}
		f := st2.Findings[0]
		if st2.Revision != st1.Revision || len(st2.Findings) != 1 || f.Severity != "P1" || f.Note != "n" || f.Status != "open" {
			t.Fatalf("相同 status 应为幂等成功，不改字段不递增 revision: %+v rev=%d", f, st2.Revision)
		}
		loaded, err := g.Load()
		if err != nil || loaded.Revision != st1.Revision || loaded.Findings[0].Severity != "P1" || loaded.Findings[0].Note != "n" {
			t.Fatalf("落盘 = %+v rev=%d err=%v", loaded.Findings, loaded.Revision, err)
		}
		mustFinding(t, g, "R1", "rejected")
		st3, err := g.Finding("R1", "P0", "rejected", "终态改 note", nil)
		if err != nil || st3.Findings[0].Note != "n" || st3.Revision != st1.Revision+1 {
			t.Fatalf("终态相同 status 也应幂等: %+v rev=%d err=%v", st3.Findings[0], st3.Revision, err)
		}
	})
	t.Run("转换时空 note 保留原 note", func(t *testing.T) {
		g, _ := newFixture(t)
		mustInit(t, g)
		if _, err := g.Finding("R1", "P1", "open", "审查原因", nil); err != nil {
			t.Fatal(err)
		}
		st, err := g.Finding("R1", "P1", "accepted", "", nil)
		if err != nil || st.Findings[0].Note != "审查原因" || st.Findings[0].Status != "accepted" {
			t.Fatalf("不带 note 的转换清空了原 note: %+v err=%v", st.Findings, err)
		}
		st, err = g.Finding("R1", "P1", "fixed", "已修", nil)
		if err != nil || st.Findings[0].Note != "已修" {
			t.Fatalf("带 note 的转换应更新 note: %+v err=%v", st.Findings, err)
		}
		loaded, err := g.Load()
		if err != nil || loaded.Findings[0].Note != "已修" {
			t.Fatalf("落盘 = %+v err=%v", loaded.Findings, err)
		}
	})
	t.Run("字段与 revision", func(t *testing.T) {
		g, _ := newFixture(t)
		mustInit(t, g)
		st, err := g.Finding("R1", "P0", "open", "描述", nil)
		if err != nil {
			t.Fatal(err)
		}
		f := st.Findings[0]
		if st.Revision != 1 || f.ID != "R1" || f.Severity != "P0" || f.Status != "open" || f.Note != "描述" || f.UpdatedAt != "2026-01-02T03:04:05Z" {
			t.Fatalf("finding = %+v rev=%d", f, st.Revision)
		}
	})
	t.Run("非法枚举", func(t *testing.T) {
		g, _ := newFixture(t)
		mustInit(t, g)
		_, err := g.Finding("R1", "P4", "open", "", nil)
		wantErr(t, err, ErrUsage, "severity")
		_, err = g.Finding("R1", "P1", "closed", "", nil)
		wantErr(t, err, ErrUsage, "status")
		_, err = g.Finding("", "P1", "open", "", nil)
		wantErr(t, err, ErrUsage, "id")
	})
}

func TestRecord(t *testing.T) {
	g, root := newFixture(t)
	mustInit(t, g)
	_, err := g.Record("lint", 0, "", nil)
	wantErr(t, err, ErrUsage, "kind")
	_, err = g.Record("check", 0, filepath.Join(root, "missing.log"), nil)
	wantErr(t, err, ErrUsage, "")
	logPath := filepath.Join(root, "check.log")
	if err := os.WriteFile(logPath, []byte("ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := g.Record("check", 0, logPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	ev := st.Evidence[0]
	if st.Revision != 1 || ev.Kind != "check" || ev.ExitCode != 0 || ev.LogSHA256 != sha("ok\n") || ev.RecordedAt != "2026-01-02T03:04:05Z" {
		t.Fatalf("evidence = %+v rev=%d", ev, st.Revision)
	}
	if ev.Candidate != st.Candidate.Digest || ev.Inputs != st.Inputs.Digest {
		t.Fatalf("证据未绑定当前摘要: %+v", ev)
	}
	st, err = g.Record("smoke", 2, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if st.Evidence[1].LogSHA256 != "" || st.Evidence[1].ExitCode != 2 || st.Revision != 2 {
		t.Fatalf("evidence = %+v", st.Evidence[1])
	}
}

func TestExpectRevision(t *testing.T) {
	g, root := newFixture(t)
	mustInit(t, g)
	wrong, right := 5, 0
	_, err := g.Record("check", 0, "", &wrong)
	wantErr(t, err, ErrConflict, "revision")
	_, err = g.Finding("R1", "P1", "open", "", &wrong)
	wantErr(t, err, ErrConflict, "revision")
	writeDelivery(t, root, "implementing", "pending", "not_required")
	_, err = g.Apply("plan_confirmed", "", &wrong)
	wantErr(t, err, ErrConflict, "revision")
	st, err := g.Apply("plan_confirmed", "", &right)
	if err != nil || st.Revision != 1 {
		t.Fatalf("st=%+v err=%v", st, err)
	}
}

func TestEventID(t *testing.T) {
	g, root := newFixture(t)
	mustInit(t, g)
	writeDelivery(t, root, "implementing", "pending", "not_required")
	st, err := g.Apply("plan_confirmed", "custom-1", nil)
	if err != nil || st.Events[0].ID != "custom-1" {
		t.Fatalf("st=%+v err=%v", st, err)
	}
	again, err := g.Apply("plan_confirmed", "custom-1", nil)
	if err != nil {
		t.Fatalf("幂等应成功: %v", err)
	}
	if again.Revision != st.Revision || len(again.Events) != 1 || again.Stage != "implementing" {
		t.Fatalf("幂等不应改变状态: %+v", again)
	}
	_, err = g.Apply("implementation_done", "custom-1", nil)
	wantErr(t, err, ErrConflict, "custom-1")
	mustRecord(t, g, "check", 0)
	st = mustApply(t, g, "implementation_done")
	if st.Events[1].ID != "evt-3" || st.Revision != 3 {
		t.Fatalf("auto id = %s rev=%d", st.Events[1].ID, st.Revision)
	}
}

func TestEventID_AutoConflictsWithCustom(t *testing.T) {
	g, root := newFixture(t)
	mustInit(t, g)
	writeDelivery(t, root, "implementing", "pending", "not_required")
	if _, err := g.Apply("plan_confirmed", "evt-3", nil); err != nil {
		t.Fatal(err)
	}
	mustRecord(t, g, "check", 0)
	_, err := g.Apply("implementation_done", "", nil)
	wantErr(t, err, ErrConflict, "evt-3")
	st, err := g.Load()
	if err != nil {
		t.Fatalf("冲突不应写坏 graph.json: %v", err)
	}
	if st.Stage != "implementing" || st.Revision != 2 {
		t.Fatalf("state = %+v", st)
	}
	if st, err = g.Apply("implementation_done", "done", nil); err != nil || st.Stage != "reviewing" {
		t.Fatalf("显式 id 应成功: st=%+v err=%v", st, err)
	}
}

// 借助 Now 钩子在首次摘要与落盘之间改动代码：落盘的摘要必须就是证据或守卫所依据的那一次。
func TestSnapshotComputedOnce(t *testing.T) {
	t.Run("record", func(t *testing.T) {
		g, root := newFixture(t)
		mustInit(t, g)
		g.Now = func() time.Time {
			writeFile(t, root, codeRel, "package x\n// between\n")
			return fixedNow
		}
		st, err := g.Record("check", 0, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		if st.Evidence[0].Candidate != st.Candidate.Digest {
			t.Fatalf("证据 candidate %s 与落盘 %s 不一致", st.Evidence[0].Candidate, st.Candidate.Digest)
		}
	})
	t.Run("event", func(t *testing.T) {
		g, root := newFixture(t)
		before := mustInit(t, g)
		writeDelivery(t, root, "implementing", "pending", "not_required")
		g.Now = func() time.Time {
			writeFile(t, root, codeRel, "package x\n// between\n")
			return fixedNow
		}
		st := mustApply(t, g, "plan_confirmed")
		if st.Candidate.Digest != before.Candidate.Digest {
			t.Fatalf("落盘摘要应为守卫判定时的摘要")
		}
	})
}

func TestInit_Concurrent(t *testing.T) {
	g, _ := newFixture(t)
	const n = 8
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = g.Init()
		}(i)
	}
	wg.Wait()
	ok := 0
	for _, err := range errs {
		if err == nil {
			ok++
		} else if !errors.Is(err, ErrConflict) {
			t.Fatalf("err = %v", err)
		}
	}
	if ok != 1 {
		t.Fatalf("成功 %d 次，应恰好 1 次", ok)
	}
	if _, err := g.Load(); err != nil {
		t.Fatal(err)
	}
}

func TestInputs_IgnoreDeliveryBlock(t *testing.T) {
	g, root := newFixture(t)
	st := mustInit(t, g)
	writeDelivery(t, root, "delivered", "confirmed", "pass")
	s, err := g.Status()
	if err != nil {
		t.Fatal(err)
	}
	if s.DriftedInputs {
		t.Fatalf("交付状态块变化不应算 inputs 漂移: %+v", s)
	}
	writeFile(t, root, planRel, sprintf(deliveryT, "delivered", "confirmed", "pass")+"\n正文变化\n")
	if s, _ = g.Status(); !s.DriftedInputs {
		t.Fatalf("正文变化应算 inputs 漂移")
	}
	planDoc := sprintf(deliveryT, "planning", "pending", "not_required")
	if st.Inputs.Files[planRel] == sha(planDoc) {
		t.Fatalf("技术方案摘要不应包含交付状态块")
	}
	if st.Inputs.Files[planRel] != sha(string(stripDeliveryBlock([]byte(planDoc)))) || st.Inputs.Files[protoRel] != sha("# 协议与数据\n") {
		t.Fatalf("技术方案应为剔除交付状态块后的 sha256，其余输入为整文件 sha256: %+v", st.Inputs.Files)
	}
	t.Run("无交付状态块时整文件参与摘要", func(t *testing.T) {
		if stripDeliveryBlock([]byte("# 无块\n")) == nil || string(stripDeliveryBlock([]byte("# 无块\n"))) != "# 无块\n" {
			t.Fatal("应原样返回")
		}
		doc := "# T\n\n## 交付状态\n\n```yaml\nstage: planning\n```\n\n## 后文\n"
		if got := string(stripDeliveryBlock([]byte(doc))); got != "# T\n\n\n## 后文\n" {
			t.Fatalf("strip = %q", got)
		}
	})
}

// 候选根为符号链接时按「不存在」跳过，不跟随也不报错；Makefile 不是「存在才算」项，缺失或为符号链接必须报错。
func TestCandidate_SymlinkRootSkipped(t *testing.T) {
	g, root := newFixture(t)
	writeFile(t, root, "tools/run.sh", "echo\n")
	if err := os.Symlink(filepath.Join(root, "tools"), filepath.Join(root, "scripts")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "go.mod"), filepath.Join(root, "go.sum")); err != nil {
		t.Fatal(err)
	}
	st := mustInit(t, g)
	for p := range st.Candidate.Files {
		if strings.HasPrefix(p, "scripts/") || p == "go.sum" {
			t.Fatalf("符号链接根不应纳入摘要: %s", p)
		}
	}
	if _, ok := st.Candidate.Files["go.mod"]; !ok {
		t.Fatalf("candidate = %v", st.Candidate.Files)
	}
}

func TestCandidate_MakefileRequired(t *testing.T) {
	t.Run("缺失", func(t *testing.T) {
		g, root := newFixture(t)
		if err := os.Remove(filepath.Join(root, "Makefile")); err != nil {
			t.Fatal(err)
		}
		if _, err := g.Init(); err == nil || !strings.Contains(err.Error(), "Makefile") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("符号链接", func(t *testing.T) {
		g, root := newFixture(t)
		if err := os.Rename(filepath.Join(root, "Makefile"), filepath.Join(root, "Makefile.real")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(root, "Makefile.real"), filepath.Join(root, "Makefile")); err != nil {
			t.Fatal(err)
		}
		if _, err := g.Init(); err == nil || !strings.Contains(err.Error(), "Makefile") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("go.mod 与 go.sum 存在才算", func(t *testing.T) {
		g, root := newFixture(t)
		if err := os.Remove(filepath.Join(root, "go.mod")); err != nil {
			t.Fatal(err)
		}
		st := mustInit(t, g)
		if _, ok := st.Candidate.Files["go.mod"]; ok {
			t.Fatalf("candidate = %v", st.Candidate.Files)
		}
	})
}

// 「当前证据」按追加顺序而非 recorded_at：后追加但时间戳更早的失败证据必须覆盖先前的成功证据。
func TestLatestEvidence_AppendOrder(t *testing.T) {
	g, root := newFixture(t)
	mustInit(t, g)
	toStage(t, g, root, "implementing")
	mustRecord(t, g, "check", 0)
	g.Now = func() time.Time { return fixedNow.Add(-time.Hour) }
	mustRecord(t, g, "check", 1)
	st, err := g.Load()
	if err != nil {
		t.Fatal(err)
	}
	if ev, ok := latestEvidence(st, "check"); !ok || ev.ExitCode != 1 || ev.RecordedAt != "2026-01-02T02:04:05Z" {
		t.Fatalf("latest = %+v ok=%v", ev, ok)
	}
	if _, err := g.Apply("implementation_done", "", nil); !errors.Is(err, ErrGuard) || !strings.Contains(err.Error(), "exit_code") {
		t.Fatalf("时钟回拨不应让旧成功证据放行: %v", err)
	}
	s, err := g.Status()
	if err != nil || s.LatestEvidence["check"].ExitCode != 1 {
		t.Fatalf("status latest = %+v err=%v", s.LatestEvidence, err)
	}
}

// 借助 Now 钩子在 load 与 commit 之间发起第二次写入：它必须以 ErrConflict 被拒绝，而不是被外层 commit 覆盖丢失。
func TestWrite_ExclusiveDuringCommit(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, g Graph, root string)
		write func(g Graph) (State, error)
	}{
		{"record", nil, func(g Graph) (State, error) { return g.Record("check", 0, "", nil) }},
		{"finding", nil, func(g Graph) (State, error) { return g.Finding("R1", "P1", "open", "", nil) }},
		{"event", func(t *testing.T, g Graph, root string) {
			writeDelivery(t, root, "implementing", "pending", "not_required")
		}, func(g Graph) (State, error) { return g.Apply("plan_confirmed", "", nil) }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g, root := newFixture(t)
			mustInit(t, g)
			if c.setup != nil {
				c.setup(t, g, root)
			}
			var innerErr error
			g.Now = func() time.Time {
				inner := g
				inner.Now = func() time.Time { return fixedNow }
				_, innerErr = inner.Record("smoke", 1, "", nil)
				return fixedNow
			}
			st, err := c.write(g)
			if err != nil {
				t.Fatal(err)
			}
			wantErr(t, innerErr, ErrConflict, "正在写入")
			loaded, err := g.Load()
			if err != nil {
				t.Fatal(err)
			}
			if loaded.Revision != 1 || loaded.Revision != st.Revision {
				t.Fatalf("revision = %d (returned %d)", loaded.Revision, st.Revision)
			}
			for _, e := range loaded.Evidence {
				if e.Kind == "smoke" {
					t.Fatalf("被拒绝的写入不应落盘: %+v", loaded.Evidence)
				}
			}
			noTempLeftover(t, root)
			if entries, _ := os.ReadDir(filepath.Join(root, "Specs", "technical", "0.1.0")); len(entries) != 2 {
				t.Fatalf("锁文件应已释放: %v", entries)
			}
		})
	}
	t.Run("失败后锁释放", func(t *testing.T) {
		g, _ := newFixture(t)
		mustInit(t, g)
		wrong := 9
		_, err := g.Record("check", 0, "", &wrong)
		wantErr(t, err, ErrConflict, "revision")
		if _, err := g.Record("check", 0, "", nil); err != nil {
			t.Fatalf("上一次失败不应留下锁: %v", err)
		}
	})
	t.Run("残留锁文件报冲突并给出创建时间", func(t *testing.T) {
		g, root := newFixture(t)
		mustInit(t, g)
		writeFile(t, root, graphRel+".lock", "")
		info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(graphRel)+".lock"))
		if err != nil {
			t.Fatal(err)
		}
		_, err = g.Record("check", 0, "", nil)
		wantErr(t, err, ErrConflict, "graph.json.lock")
		wantErr(t, err, ErrConflict, "创建于 "+info.ModTime().UTC().Format(time.RFC3339))
	})
}

// 不支持硬链接的文件系统（ExFAT、部分 SMB / bind mount）上 link 返回 ENOTSUP / EPERM，Init 必须退回 rename 而不是失败。
func TestInit_LinkUnsupportedFallsBackToRename(t *testing.T) {
	orig := linkFile
	linkFile = func(oldname, newname string) error {
		return &os.LinkError{Op: "link", Old: oldname, New: newname, Err: syscall.ENOTSUP}
	}
	t.Cleanup(func() { linkFile = orig })

	g, root := newFixture(t)
	st := mustInit(t, g)
	loaded, err := g.Load()
	if err != nil || loaded.Candidate.Digest != st.Candidate.Digest || loaded.Stage != "planning" {
		t.Fatalf("load = %+v err=%v", loaded, err)
	}
	noTempLeftover(t, root)
	if entries, _ := os.ReadDir(filepath.Join(root, "Specs", "technical", "0.1.0")); len(entries) != 2 {
		t.Fatalf("entries = %v", entries)
	}
	_, err = g.Init()
	wantErr(t, err, ErrConflict, "已存在")

	linkFile = func(oldname, newname string) error {
		return &os.LinkError{Op: "link", Old: oldname, New: newname, Err: syscall.EACCES}
	}
	g2, root2 := newFixture(t)
	if _, err := g2.Init(); err == nil || !strings.Contains(err.Error(), "创建 graph.json") {
		t.Fatalf("其他 link 错误不应退回 rename: %v", err)
	}
	noTempLeftover(t, root2)
}

// graph.json 是人读的状态台账，note 里的 < > & 不应被转义成 \u003c 等。
func TestWrite_NoteKeepsHTMLChars(t *testing.T) {
	g, root := newFixture(t)
	mustInit(t, g)
	if _, err := g.Finding("R1", "P1", "open", "a<b & c", nil); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(graphRel)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"note": "a<b & c"`) {
		t.Fatalf("note 被转义:\n%s", raw)
	}
	st, err := g.Load()
	if err != nil || st.Findings[0].Note != "a<b & c" {
		t.Fatalf("load = %+v err=%v", st.Findings, err)
	}
}

func TestRecord_Concurrent(t *testing.T) {
	g, root := newFixture(t)
	mustInit(t, g)
	const n = 16
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = g.Record("check", 0, "", nil)
		}(i)
	}
	wg.Wait()
	ok := 0
	for _, err := range errs {
		if err == nil {
			ok++
		} else if !errors.Is(err, ErrConflict) {
			t.Fatalf("err = %v", err)
		}
	}
	st, err := g.Load()
	if err != nil {
		t.Fatal(err)
	}
	if ok == 0 || len(st.Evidence) != ok || st.Revision != ok {
		t.Fatalf("成功 %d 次，但 evidence=%d revision=%d", ok, len(st.Evidence), st.Revision)
	}
	noTempLeftover(t, root)
	if entries, _ := os.ReadDir(filepath.Join(root, "Specs", "technical", "0.1.0")); len(entries) != 2 {
		t.Fatalf("锁文件应已释放: %v", entries)
	}
}

func TestWrite_FileMode(t *testing.T) {
	g, root := newFixture(t)
	mustInit(t, g)
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(graphRel)))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("mode = %o, want 644", info.Mode().Perm())
	}
}

func TestLoad_RawTampering(t *testing.T) {
	cases := []struct {
		name string
		fn   func(raw string) string
		want string
	}{
		{"尾随垃圾", func(raw string) string { return raw + "GARBAGE\n" }, "多余内容"},
		{"尾随第二个 JSON", func(raw string) string { return raw + "{\"x\":1}\n" }, "多余内容"},
		{"重复键", func(raw string) string {
			return strings.Replace(raw, "\"stage\": \"planning\"", "\"stage\": \"planning\",\n  \"stage\": \"planning\"", 1)
		}, "重复键"},
		{"嵌套重复键", func(raw string) string {
			return strings.Replace(raw, "\"digest\":", "\"files\": {},\n    \"digest\":", 1)
		}, "重复键"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g, root := newFixture(t)
			mustInit(t, g)
			p := filepath.Join(root, filepath.FromSlash(graphRel))
			raw, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(c.fn(string(raw))), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err = g.Load()
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want containing %q", err, c.want)
			}
		})
	}
}

func TestStatus_Drift(t *testing.T) {
	t.Run("planning 阶段漂移不算失效", func(t *testing.T) {
		g, root := newFixture(t)
		mustInit(t, g)
		writeFile(t, root, codeRel, "package x\n// a\n")
		s, err := g.Status()
		if err != nil {
			t.Fatal(err)
		}
		if !s.DriftedCandidate || s.DriftedInputs || s.Invalidated {
			t.Fatalf("status = %+v", s)
		}
	})
	t.Run("reviewing 代码漂移", func(t *testing.T) {
		g, root := newFixture(t)
		mustInit(t, g)
		toStage(t, g, root, "reviewing")
		s, _ := g.Status()
		if s.Invalidated {
			t.Fatalf("无改动不应失效")
		}
		writeFile(t, root, codeRel, "package x\n// a\n")
		s, err := g.Status()
		if err != nil {
			t.Fatal(err)
		}
		if !s.Invalidated || !s.DriftedCandidate || s.DriftedInputs {
			t.Fatalf("status = %+v", s)
		}
	})
	t.Run("verifying 需求漂移", func(t *testing.T) {
		g, root := newFixture(t)
		mustInit(t, g)
		toStage(t, g, root, "verifying")
		writeFile(t, root, reqRel, "# 需求变化\n")
		s, err := g.Status()
		if err != nil {
			t.Fatal(err)
		}
		if !s.Invalidated || !s.DriftedInputs || s.DriftedCandidate {
			t.Fatalf("status = %+v", s)
		}
	})
	t.Run("汇总字段", func(t *testing.T) {
		g, root := newFixture(t)
		mustInit(t, g)
		toStage(t, g, root, "reviewing")
		mustFinding(t, g, "R1", "open")
		mustFinding(t, g, "R2", "open")
		mustFinding(t, g, "R2", "rejected")
		mustRecord(t, g, "review", 1)
		s, err := g.Status()
		if err != nil {
			t.Fatal(err)
		}
		if s.Version != version || s.Stage != "reviewing" || s.Revision != 7 {
			t.Fatalf("status = %+v", s)
		}
		if s.FindingCounts["open"] != 1 || s.FindingCounts["rejected"] != 1 {
			t.Fatalf("counts = %v", s.FindingCounts)
		}
		if s.LatestEvidence["check"].ExitCode != 0 || s.LatestEvidence["review"].ExitCode != 1 {
			t.Fatalf("latest = %v", s.LatestEvidence)
		}
		if s.Delivery.Stage != "implementing" {
			t.Fatalf("delivery = %+v", s.Delivery)
		}
	})
}

// Status.Delivery 按合同为 specdoc.DeliveryStatus，JSON 输出仍为 snake_case 三键。
func TestStatus_DeliveryType(t *testing.T) {
	g, _ := newFixture(t)
	mustInit(t, g)
	s, err := g.Status()
	if err != nil {
		t.Fatal(err)
	}
	var ds specdoc.DeliveryStatus = s.Delivery
	if ds.Stage != "planning" || ds.UserAcceptance != "pending" || ds.Review != "not_required" {
		t.Fatalf("delivery = %+v", ds)
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"version", "stage", "revision", "invalidated", "drifted_inputs", "drifted_candidate", "finding_counts", "latest_evidence", "delivery"} {
		if _, ok := m[k]; !ok {
			t.Fatalf("缺少键 %s: %s", k, data)
		}
	}
	if len(m) != 9 {
		t.Fatalf("键数 = %d: %s", len(m), data)
	}
	var d map[string]string
	if err := json.Unmarshal(m["delivery"], &d); err != nil {
		t.Fatal(err)
	}
	if len(d) != 3 || d["stage"] != "planning" || d["user_acceptance"] != "pending" || d["review"] != "not_required" {
		t.Fatalf("delivery 键应为 snake_case: %s", m["delivery"])
	}
}

func TestWrite_NoTempLeftover(t *testing.T) {
	g, root := newFixture(t)
	mustInit(t, g)
	mustRecord(t, g, "check", 0)
	mustFinding(t, g, "R1", "open")
	entries, err := os.ReadDir(filepath.Join(root, "Specs", "technical", "0.1.0"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp") {
			t.Fatalf("残留临时文件 %s", e.Name())
		}
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %d", len(entries))
	}
}

func noTempLeftover(t *testing.T, root string) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, "Specs", "technical", "0.1.0"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp") {
			t.Fatalf("残留临时文件 %s", e.Name())
		}
	}
}

func TestWrite_FailureRemovesTemp(t *testing.T) {
	t.Run("rename 失败", func(t *testing.T) {
		g, root := newFixture(t)
		st := mustInit(t, g)
		p := filepath.Join(root, filepath.FromSlash(graphRel))
		if err := os.Remove(p); err != nil {
			t.Fatal(err)
		}
		writeFile(t, root, graphRel+"/inner.txt", "x")
		if err := g.writeState(st, false); err == nil || !strings.Contains(err.Error(), "替换 graph.json") {
			t.Fatalf("err = %v", err)
		}
		noTempLeftover(t, root)
	})
	t.Run("独占创建遇到已存在", func(t *testing.T) {
		g, root := newFixture(t)
		st := mustInit(t, g)
		err := g.writeState(st, true)
		wantErr(t, err, ErrConflict, "已存在")
		noTempLeftover(t, root)
	})
	t.Run("拒绝写入非法状态", func(t *testing.T) {
		g, root := newFixture(t)
		st := mustInit(t, g)
		st.Stage = "done"
		if err := g.writeState(st, false); err == nil || !strings.Contains(err.Error(), "非法状态") {
			t.Fatalf("err = %v", err)
		}
		noTempLeftover(t, root)
		if _, err := g.Load(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestLoad_Validation(t *testing.T) {
	mutate := func(t *testing.T, root string, fn func(m map[string]any)) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(graphRel))
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		fn(m)
		out, _ := json.Marshal(m)
		if err := os.WriteFile(p, out, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct {
		name string
		fn   func(m map[string]any)
		want string
	}{
		{"未知字段", func(m map[string]any) { m["extra"] = 1 }, "extra"},
		{"篡改 stage", func(m map[string]any) { m["stage"] = "reviewing" }, "事件重放与 stage 不一致"},
		{"非法 stage", func(m map[string]any) { m["stage"] = "done" }, "stage"},
		{"非法证据 kind", func(m map[string]any) {
			m["evidence"].([]any)[0].(map[string]any)["kind"] = "lint"
		}, "kind"},
		{"非法事件 from/to", func(m map[string]any) {
			m["events"].([]any)[0].(map[string]any)["to"] = "reviewing"
		}, "plan_confirmed"},
		{"非法 finding status", func(m map[string]any) {
			m["findings"].([]any)[0].(map[string]any)["status"] = "wontfix"
		}, "status"},
		{"版本不匹配", func(m map[string]any) { m["version"] = "0.2.0" }, "version"},
		{"非 UTC 时间", func(m map[string]any) {
			m["evidence"].([]any)[0].(map[string]any)["recorded_at"] = "2026-01-02T11:04:05+08:00"
		}, "UTC"},
		{"带小数秒时间", func(m map[string]any) {
			m["events"].([]any)[0].(map[string]any)["at"] = "2026-01-02T03:04:05.000Z"
		}, "UTC"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g, root := newFixture(t)
			mustInit(t, g)
			toStage(t, g, root, "implementing")
			mustRecord(t, g, "check", 0)
			mustFinding(t, g, "R1", "open")
			mutate(t, root, c.fn)
			_, err := g.Load()
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want containing %q", err, c.want)
			}
			if errors.Is(err, ErrNotInitialized) {
				t.Fatalf("校验失败不应报未初始化")
			}
		})
	}
}

func TestCheck(t *testing.T) {
	problems := func(t *testing.T, g Graph) []Problem {
		t.Helper()
		ps, err := g.Check()
		if err != nil {
			t.Fatal(err)
		}
		return ps
	}
	hasProblem := func(ps []Problem, msg string) bool {
		for _, p := range ps {
			if strings.Contains(p.Message, msg) {
				return true
			}
		}
		return false
	}
	t.Run("未初始化", func(t *testing.T) {
		g, _ := newFixture(t)
		_, err := g.Check()
		wantErr(t, err, ErrNotInitialized, "")
	})
	t.Run("健康", func(t *testing.T) {
		g, root := newFixture(t)
		mustInit(t, g)
		toStage(t, g, root, "reviewing")
		if ps := problems(t, g); len(ps) != 0 {
			t.Fatalf("problems = %+v", ps)
		}
	})
	t.Run("不可加载作为 Problem", func(t *testing.T) {
		g, root := newFixture(t)
		mustInit(t, g)
		writeFile(t, root, graphRel, `{"version":"0.1.0","revision":0,"stage":"planning","inputs":{"files":{},"digest":""},"candidate":{"files":{},"digest":""},"findings":[],"evidence":[{"kind":"lint","candidate":"","inputs":"","exit_code":0,"log_sha256":"","recorded_at":"2026-01-02T03:04:05Z"}],"events":[]}`)
		ps := problems(t, g)
		if !hasProblem(ps, "kind") || ps[0].Path != graphRel {
			t.Fatalf("problems = %+v", ps)
		}
	})
	t.Run("ready_to_deliver 已失效", func(t *testing.T) {
		g, root := newFixture(t)
		mustInit(t, g)
		toStage(t, g, root, "ready_to_deliver")
		if ps := problems(t, g); len(ps) != 0 {
			t.Fatalf("problems = %+v", ps)
		}
		writeFile(t, root, codeRel, "package x\n// drift\n")
		if ps := problems(t, g); !hasProblem(ps, "readiness 已失效，执行 event readiness_invalidated") {
			t.Fatalf("problems = %+v", ps)
		}
	})
	t.Run("delivered 但 graph 未就绪", func(t *testing.T) {
		g, root := newFixture(t)
		mustInit(t, g)
		toStage(t, g, root, "verifying")
		writeDelivery(t, root, "delivered", "confirmed", "pass")
		if ps := problems(t, g); !hasProblem(ps, "delivered") {
			t.Fatalf("problems = %+v", ps)
		}
	})
	t.Run("delivered 且就绪", func(t *testing.T) {
		g, root := newFixture(t)
		mustInit(t, g)
		toStage(t, g, root, "ready_to_deliver")
		writeDelivery(t, root, "delivered", "confirmed", "pass")
		if ps := problems(t, g); len(ps) != 0 {
			t.Fatalf("problems = %+v", ps)
		}
		s, err := g.Status()
		if err != nil || s.Invalidated || s.DriftedInputs {
			t.Fatalf("status = %+v err=%v", s, err)
		}
	})
	t.Run("ready_to_deliver 但用户未确认", func(t *testing.T) {
		g, root := newFixture(t)
		mustInit(t, g)
		toStage(t, g, root, "ready_to_deliver")
		writeDelivery(t, root, "verifying", "pending", "pass")
		if ps := problems(t, g); !hasProblem(ps, "user_acceptance") {
			t.Fatalf("problems = %+v", ps)
		}
	})
	t.Run("缺技术方案", func(t *testing.T) {
		g, root := newFixture(t)
		mustInit(t, g)
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(planRel))); err != nil {
			t.Fatal(err)
		}
		ps := problems(t, g)
		if !hasProblem(ps, "技术方案.md") {
			t.Fatalf("problems = %+v", ps)
		}
	})
	// 进程被 SIGINT / SIGKILL 中断后锁文件残留，record / finding / event 全部被拒绝；check 必须把它报告出来。
	t.Run("残留锁文件", func(t *testing.T) {
		g, root := newFixture(t)
		mustInit(t, g)
		toStage(t, g, root, "reviewing")
		if ps := problems(t, g); len(ps) != 0 {
			t.Fatalf("problems = %+v", ps)
		}
		writeFile(t, root, graphRel+".lock", "")
		ps := problems(t, g)
		if len(ps) != 1 || ps[0].Path != graphRel+".lock" || !hasProblem(ps, "残留") || !hasProblem(ps, "创建于 ") {
			t.Fatalf("problems = %+v", ps)
		}
	})
	t.Run("只读", func(t *testing.T) {
		g, root := newFixture(t)
		mustInit(t, g)
		toStage(t, g, root, "ready_to_deliver")
		writeFile(t, root, codeRel, "package x\n// drift\n")
		before := readGraphJSON(t, root)
		problems(t, g)
		after := readGraphJSON(t, root)
		if string(before["revision"]) != string(after["revision"]) || string(before["candidate"]) != string(after["candidate"]) {
			t.Fatalf("Check 修改了 graph.json")
		}
	})
}

func TestFullPath(t *testing.T) {
	g, root := newFixture(t)
	mustInit(t, g)
	writeDelivery(t, root, "implementing", "pending", "not_required")
	mustApply(t, g, "plan_confirmed")
	writeFile(t, root, codeRel, "package x\n\nfunc F() {}\n")
	mustRecord(t, g, "check", 0)
	mustApply(t, g, "implementation_done")
	mustFinding(t, g, "R1", "open")
	mustFinding(t, g, "R1", "accepted")
	mustRecord(t, g, "review", 1)
	mustApply(t, g, "review_failed")
	writeFile(t, root, codeRel, "package x\n\nfunc F() int { return 1 }\n")
	mustFinding(t, g, "R1", "fixed")
	mustRecord(t, g, "check", 0)
	mustApply(t, g, "fix_done")
	mustFinding(t, g, "R1", "verified")
	mustRecord(t, g, "review", 0)
	mustApply(t, g, "review_passed")
	writeDelivery(t, root, "verifying", "confirmed", "pass")
	for _, k := range []string{"check", "test-integration", "test-race", "smoke", "api-verify"} {
		mustRecord(t, g, k, 0)
	}
	st := mustApply(t, g, "verified")
	if st.Stage != "ready_to_deliver" {
		t.Fatalf("stage = %s", st.Stage)
	}
	types := []string{}
	for _, e := range st.Events {
		types = append(types, e.Type)
	}
	want := "plan_confirmed,implementation_done,review_failed,fix_done,review_passed,verified"
	if strings.Join(types, ",") != want {
		t.Fatalf("events = %v", types)
	}
	if st.Revision != len(st.Events)+len(st.Evidence)+4 {
		t.Fatalf("revision = %d", st.Revision)
	}
	ps, err := g.Check()
	if err != nil || len(ps) != 0 {
		t.Fatalf("check = %+v, %v", ps, err)
	}
	writeDelivery(t, root, "delivered", "confirmed", "pass")
	if ps, err = g.Check(); err != nil || len(ps) != 0 {
		t.Fatalf("delivered 后 check = %+v, %v", ps, err)
	}
	if _, err := g.Load(); err != nil {
		t.Fatalf("重放失败: %v", err)
	}
}
