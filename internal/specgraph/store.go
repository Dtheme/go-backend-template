package specgraph

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

const stagePlanning = "planning"

var (
	stages         = []string{"planning", "implementing", "reviewing", "fixing", "verifying", "ready_to_deliver"}
	evidenceKinds  = []string{"check", "test-integration", "test-race", "smoke", "api-verify", "review"}
	severities     = []string{"P0", "P1", "P2", "P3"}
	findingStates  = []string{"open", "accepted", "rejected", "fixed", "verified"}
	invalidatable  = []string{"reviewing", "verifying", "ready_to_deliver"}
	readyToDeliver = "ready_to_deliver"
)

// linkFile 供测试注入不支持硬链接的文件系统错误。
var linkFile = os.Link

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func (g Graph) now() string {
	t := time.Now()
	if g.Now != nil {
		t = g.Now()
	}
	return t.UTC().Format(time.RFC3339)
}

func (g Graph) readState() (State, error) {
	raw, err := os.ReadFile(g.abs(g.graphRel()))
	if errors.Is(err, fs.ErrNotExist) {
		return State{}, ErrNotInitialized
	}
	if err != nil {
		return State{}, err
	}
	if err := checkJSONShape(raw); err != nil {
		return State{}, fmt.Errorf("解析 graph.json: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var st State
	if err := dec.Decode(&st); err != nil {
		return State{}, fmt.Errorf("解析 graph.json: %w", err)
	}
	if err := validateState(st, g.Version); err != nil {
		return State{}, fmt.Errorf("graph.json 校验失败: %w", err)
	}
	return st, nil
}

// checkJSONShape 拒绝首个 JSON 值之后的多余内容以及同一对象内的重复键；这两类问题 encoding/json 都会静默接受。
func checkJSONShape(raw []byte) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	var walk func() error
	walk = func() error {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		d, ok := tok.(json.Delim)
		if !ok {
			return nil
		}
		keys := map[string]bool{}
		for dec.More() {
			if d == '{' {
				k, err := dec.Token()
				if err != nil {
					return err
				}
				if keys[k.(string)] {
					return fmt.Errorf("重复键 %q", k)
				}
				keys[k.(string)] = true
			}
			if err := walk(); err != nil {
				return err
			}
		}
		_, err = dec.Token()
		return err
	}
	if err := walk(); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return errors.New("首个 JSON 值之后存在多余内容")
	}
	return nil
}

func validateState(st State, version string) error {
	if st.Version != version {
		return fmt.Errorf("version %q 与 %q 不一致", st.Version, version)
	}
	if !contains(stages, st.Stage) {
		return fmt.Errorf("stage 非法: %q", st.Stage)
	}
	seen := map[string]bool{}
	for _, f := range st.Findings {
		if f.ID == "" || seen[f.ID] {
			return fmt.Errorf("finding id 为空或重复: %q", f.ID)
		}
		seen[f.ID] = true
		if !contains(severities, f.Severity) {
			return fmt.Errorf("finding %s severity 非法: %q", f.ID, f.Severity)
		}
		if !contains(findingStates, f.Status) {
			return fmt.Errorf("finding %s status 非法: %q", f.ID, f.Status)
		}
		if err := checkTime(f.UpdatedAt); err != nil {
			return fmt.Errorf("finding %s updated_at: %w", f.ID, err)
		}
	}
	for i, e := range st.Evidence {
		if !contains(evidenceKinds, e.Kind) {
			return fmt.Errorf("evidence[%d] kind 非法: %q", i, e.Kind)
		}
		if err := checkTime(e.RecordedAt); err != nil {
			return fmt.Errorf("evidence[%d] recorded_at: %w", i, err)
		}
	}
	ids := map[string]bool{}
	stage := stagePlanning
	for i, ev := range st.Events {
		if ev.ID == "" || ids[ev.ID] {
			return fmt.Errorf("events[%d] id 为空或重复: %q", i, ev.ID)
		}
		ids[ev.ID] = true
		tr, ok := transitions[ev.Type]
		if !ok {
			return fmt.Errorf("events[%d] type 未知: %q", i, ev.Type)
		}
		if !contains(tr.from, ev.From) || ev.To != tr.to {
			return fmt.Errorf("events[%d] %s 不允许 %s -> %s", i, ev.Type, ev.From, ev.To)
		}
		if err := checkTime(ev.At); err != nil {
			return fmt.Errorf("events[%d] at: %w", i, err)
		}
		if ev.From != stage {
			return fmt.Errorf("事件重放与 stage 不一致: events[%d] 从 %s 出发，但重放到此处为 %s", i, ev.From, stage)
		}
		stage = ev.To
	}
	if stage != st.Stage {
		return fmt.Errorf("事件重放与 stage 不一致: 重放结果 %s，记录为 %s", stage, st.Stage)
	}
	return nil
}

// checkTime 只接受 now() 写出的规范形式（UTC、秒精度、Z 后缀）。
func checkTime(s string) error {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil || s != t.UTC().Format(time.RFC3339) {
		return fmt.Errorf("时间必须是 UTC RFC3339（如 2026-01-02T03:04:05Z）: %q", s)
	}
	return nil
}

// lock 在 graph.json 同目录独占创建 graph.json.lock，覆盖 load→commit 全程；--expect-revision 只是读后比较，
// 不能防止丢写。锁已存在即另一写入者持有，返回 ErrConflict。进程被 SIGINT / SIGKILL 中断时 defer 不会执行，
// 锁会残留：错误信息与 Check 都给出锁文件创建时间，确认无进程运行后由人工删除。
func (g Graph) lock() (unlock func(), err error) {
	path := g.lockPath()
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	switch {
	case errors.Is(err, fs.ErrExist):
		return nil, fmt.Errorf("%w: 另一个 spec-graph 正在写入（%s.lock %s；确认无进程运行后可删除）", ErrConflict, g.graphRel(), lockDetail(path))
	case errors.Is(err, fs.ErrNotExist):
		return nil, ErrNotInitialized
	case err != nil:
		return nil, fmt.Errorf("创建锁文件: %w", err)
	}
	f.Close()
	return func() { os.Remove(path) }, nil
}

func (g Graph) lockPath() string { return g.abs(g.graphRel()) + ".lock" }

func lockDetail(path string) string {
	info, err := os.Lstat(path)
	if err != nil {
		return "创建时间未知"
	}
	return "创建于 " + info.ModTime().UTC().Format(time.RFC3339)
}

// marshalJSON 关闭 HTML 转义（graph.json 供人阅读，note 常含 < > &），输出末尾带换行。
func marshalJSON(v any, indent string) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", indent)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// linkUnsupported 判断文件系统不支持硬链接（ExFAT / FAT 返回 ENOTSUP 或 EPERM，部分 SMB / bind mount 返回 EOPNOTSUPP）。
func linkUnsupported(err error) bool {
	return errors.Is(err, syscall.ENOTSUP) || errors.Is(err, syscall.EOPNOTSUPP) || errors.Is(err, syscall.EPERM)
}

// writeState 校验后先写同目录临时文件（0644）并 fsync；create 时用 link 独占创建，目标已存在返回 ErrConflict，
// 文件系统不支持硬链接时退回 rename（Init 已先 Lstat 判存在，并发窗口极小）；否则 rename 覆盖。失败时删除临时文件。
func (g Graph) writeState(st State, create bool) error {
	if st.Findings == nil {
		st.Findings = []Finding{}
	}
	if st.Evidence == nil {
		st.Evidence = []Evidence{}
	}
	if st.Events == nil {
		st.Events = []Event{}
	}
	if err := validateState(st, g.Version); err != nil {
		return fmt.Errorf("拒绝写入非法状态: %w", err)
	}
	data, err := marshalJSON(st, "  ")
	if err != nil {
		return err
	}

	target := g.abs(g.graphRel())
	tmp, err := os.CreateTemp(filepath.Dir(target), "graph.json.tmp-*")
	if err != nil {
		return fmt.Errorf("创建临时文件: %w", err)
	}
	if err := writeSync(tmp, data); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if create {
		err := linkFile(tmp.Name(), target)
		switch {
		case err == nil:
			os.Remove(tmp.Name())
			return nil
		case errors.Is(err, fs.ErrExist):
			os.Remove(tmp.Name())
			return fmt.Errorf("%w: %s 已存在", ErrConflict, g.graphRel())
		case !linkUnsupported(err):
			os.Remove(tmp.Name())
			return fmt.Errorf("创建 graph.json: %w", err)
		}
	}
	if err := os.Rename(tmp.Name(), target); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("替换 graph.json: %w", err)
	}
	return nil
}

func writeSync(f *os.File, data []byte) error {
	defer f.Close()
	if err := f.Chmod(0o644); err != nil {
		return fmt.Errorf("设置临时文件权限: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("写入临时文件: %w", err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("fsync: %w", err)
	}
	return nil
}
