package specgraph

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"example.com/go-backend-template/internal/specdoc"
)

type transition struct {
	from  []string
	to    string
	guard func(c guardContext) error
}

// guardContext 汇总守卫需要的三类输入：状态、当前重新计算的摘要、技术方案交付状态。
type guardContext struct {
	g         Graph
	st        State
	inputs    Snapshot
	candidate Snapshot
}

var transitions = map[string]transition{
	"plan_confirmed": {from: []string{"planning"}, to: "implementing", guard: func(c guardContext) error {
		ds, err := c.delivery()
		if err != nil {
			return err
		}
		if ds.Stage == "planning" {
			return errors.New("技术方案「交付状态」stage 仍为 planning")
		}
		return nil
	}},
	"implementation_done": {from: []string{"implementing"}, to: "reviewing", guard: func(c guardContext) error {
		return c.boundEvidence("check", true)
	}},
	"review_failed": {from: []string{"reviewing"}, to: "fixing", guard: func(c guardContext) error {
		if len(c.findingsIn("open", "accepted")) == 0 {
			return errors.New("没有状态为 open 或 accepted 的 finding")
		}
		return nil
	}},
	"review_passed": {from: []string{"reviewing"}, to: "verifying", guard: func(c guardContext) error {
		if ids := c.findingsIn("open", "accepted", "fixed"); len(ids) > 0 {
			return fmt.Errorf("finding 仍为 open/accepted/fixed: %s", strings.Join(ids, ", "))
		}
		return c.boundEvidence("review", false)
	}},
	"fix_done": {from: []string{"fixing"}, to: "reviewing", guard: func(c guardContext) error {
		if ids := c.findingsIn("open"); len(ids) > 0 {
			return fmt.Errorf("finding 仍为 open: %s", strings.Join(ids, ", "))
		}
		return c.boundEvidence("check", true)
	}},
	"verified": {from: []string{"verifying"}, to: "ready_to_deliver", guard: func(c guardContext) error {
		for _, kind := range []string{"check", "test-integration", "smoke", "api-verify"} {
			if err := c.boundEvidence(kind, true); err != nil {
				return err
			}
		}
		if ev, ok := latestEvidence(c.st, "test-race"); ok && ev.ExitCode != 0 {
			return fmt.Errorf("test-race 证据 exit_code=%d", ev.ExitCode)
		}
		var bad []string
		for _, f := range c.st.Findings {
			if f.Status != "verified" && f.Status != "rejected" {
				bad = append(bad, f.ID+"("+f.Status+")")
			}
		}
		if len(bad) > 0 {
			return fmt.Errorf("finding 未到 verified/rejected: %s", strings.Join(bad, ", "))
		}
		ds, err := c.delivery()
		if err != nil {
			return err
		}
		if ds.UserAcceptance != "confirmed" {
			return fmt.Errorf("技术方案「交付状态」user_acceptance 为 %s，需要 confirmed", ds.UserAcceptance)
		}
		return nil
	}},
	"readiness_invalidated": {from: []string{"verifying", "ready_to_deliver"}, to: "fixing", guard: func(c guardContext) error {
		if !invalidated(c.st, c.inputs, c.candidate) {
			return errors.New("当前 readiness 未失效")
		}
		return nil
	}},
}

func (c guardContext) delivery() (specdoc.DeliveryStatus, error) {
	return c.g.readDelivery()
}

func (g Graph) readDelivery() (specdoc.DeliveryStatus, error) {
	doc, err := os.ReadFile(g.abs(g.planRel()))
	if errors.Is(err, fs.ErrNotExist) {
		return specdoc.DeliveryStatus{}, fmt.Errorf("缺少 %s", g.planRel())
	}
	if err != nil {
		return specdoc.DeliveryStatus{}, err
	}
	ds, err := specdoc.ParseDeliveryStatus(doc)
	if err != nil {
		return specdoc.DeliveryStatus{}, fmt.Errorf("技术方案交付状态: %w", err)
	}
	return ds, nil
}

func (c guardContext) findingsIn(statuses ...string) []string {
	var ids []string
	for _, f := range c.st.Findings {
		if contains(statuses, f.Status) {
			ids = append(ids, f.ID)
		}
	}
	return ids
}

// boundEvidence 要求该 kind 的当前证据 exit_code=0 且绑定当前 candidate（needInputs 时还要绑定 inputs）。
func (c guardContext) boundEvidence(kind string, needInputs bool) error {
	ev, ok := latestEvidence(c.st, kind)
	if !ok {
		return fmt.Errorf("缺少 %s 证据", kind)
	}
	if ev.ExitCode != 0 {
		return fmt.Errorf("%s 证据 exit_code=%d", kind, ev.ExitCode)
	}
	if ev.Candidate != c.candidate.Digest {
		return fmt.Errorf("%s 证据绑定的 candidate 与当前不一致", kind)
	}
	if needInputs && ev.Inputs != c.inputs.Digest {
		return fmt.Errorf("%s 证据绑定的 inputs 与当前不一致", kind)
	}
	return nil
}

// latestEvidence 取同 kind 中最后追加的一条；evidence 只追加，recorded_at 仅作记录，不依赖系统时钟单调。
func latestEvidence(st State, kind string) (Evidence, bool) {
	for i := len(st.Evidence) - 1; i >= 0; i-- {
		if st.Evidence[i].Kind == kind {
			return st.Evidence[i], true
		}
	}
	return Evidence{}, false
}

func invalidated(st State, inputs, candidate Snapshot) bool {
	return contains(invalidatable, st.Stage) && (inputs.Digest != st.Inputs.Digest || candidate.Digest != st.Candidate.Digest)
}

var findingFlow = map[string][]string{
	"open":     {"accepted", "rejected"},
	"accepted": {"fixed"},
	"fixed":    {"verified"},
}

func (g Graph) Init() (State, error) {
	if _, err := os.Lstat(g.abs(g.graphRel())); err == nil {
		return State{}, fmt.Errorf("%w: %s 已存在", ErrConflict, g.graphRel())
	} else if !errors.Is(err, fs.ErrNotExist) {
		return State{}, err
	}
	inputs, candidate, err := g.snapshots()
	if err != nil {
		return State{}, err
	}
	st := State{Version: g.Version, Stage: stagePlanning, Inputs: inputs, Candidate: candidate}
	if err := g.writeState(st, true); err != nil {
		return State{}, err
	}
	return st, nil
}

func (g Graph) Load() (State, error) { return g.readState() }

func (g Graph) Record(kind string, exitCode int, logPath string, expectRevision *int) (State, error) {
	if !contains(evidenceKinds, kind) {
		return State{}, fmt.Errorf("%w: 未知证据 kind %q（允许 %s）", ErrUsage, kind, strings.Join(evidenceKinds, " | "))
	}
	logSum := ""
	if logPath != "" {
		sum, err := fileSHA256(logPath)
		if err != nil {
			return State{}, fmt.Errorf("%w: 读取日志 %s: %v", ErrUsage, logPath, err)
		}
		logSum = sum
	}
	unlock, err := g.lock()
	if err != nil {
		return State{}, err
	}
	defer unlock()
	st, err := g.loadExpecting(expectRevision)
	if err != nil {
		return State{}, err
	}
	inputs, candidate, err := g.snapshots()
	if err != nil {
		return State{}, err
	}
	st.Evidence = append(st.Evidence, Evidence{
		Kind: kind, Candidate: candidate.Digest, Inputs: inputs.Digest,
		ExitCode: exitCode, LogSHA256: logSum, RecordedAt: g.now(),
	})
	if err := g.commit(&st, inputs, candidate); err != nil {
		return State{}, err
	}
	return st, nil
}

func (g Graph) Finding(id, severity, status, note string, expectRevision *int) (State, error) {
	if id == "" {
		return State{}, fmt.Errorf("%w: finding id 不能为空", ErrUsage)
	}
	if !contains(severities, severity) {
		return State{}, fmt.Errorf("%w: severity 非法 %q（允许 P0..P3）", ErrUsage, severity)
	}
	if !contains(findingStates, status) {
		return State{}, fmt.Errorf("%w: status 非法 %q（允许 %s）", ErrUsage, status, strings.Join(findingStates, " | "))
	}
	unlock, err := g.lock()
	if err != nil {
		return State{}, err
	}
	defer unlock()
	st, err := g.loadExpecting(expectRevision)
	if err != nil {
		return State{}, err
	}
	idx := -1
	for i, f := range st.Findings {
		if f.ID == id {
			idx = i
		}
	}
	// 同 id 再次给出相同 status 为幂等成功：不改 severity / note，不写盘，revision 不变。
	// 合法转换时 --note 可选，note 为空保留原 note，避免 open 时登记的审查原因被 accepted 清空。
	switch {
	case idx < 0 && status != "open":
		return State{}, fmt.Errorf("%w: 新建 finding %s 必须为 open", ErrGuard, id)
	case idx < 0:
		st.Findings = append(st.Findings, Finding{ID: id, Severity: severity, Status: status, Note: note, UpdatedAt: g.now()})
	case st.Findings[idx].Status == status:
		return st, nil
	case !contains(findingFlow[st.Findings[idx].Status], status):
		return State{}, fmt.Errorf("%w: finding %s 不允许 %s -> %s", ErrGuard, id, st.Findings[idx].Status, status)
	default:
		f := &st.Findings[idx]
		if note != "" {
			f.Note = note
		}
		f.Severity, f.Status, f.UpdatedAt = severity, status, g.now()
	}
	inputs, candidate, err := g.snapshots()
	if err != nil {
		return State{}, err
	}
	if err := g.commit(&st, inputs, candidate); err != nil {
		return State{}, err
	}
	return st, nil
}

func (g Graph) Apply(eventType, eventID string, expectRevision *int) (State, error) {
	tr, ok := transitions[eventType]
	if !ok {
		return State{}, fmt.Errorf("%w: 未知事件 %q", ErrUsage, eventType)
	}
	unlock, err := g.lock()
	if err != nil {
		return State{}, err
	}
	defer unlock()
	st, err := g.loadExpecting(expectRevision)
	if err != nil {
		return State{}, err
	}
	if eventID != "" {
		for _, ev := range st.Events {
			if ev.ID != eventID {
				continue
			}
			if ev.Type == eventType {
				return st, nil
			}
			return State{}, fmt.Errorf("%w: 事件 %s 已存在且类型为 %s", ErrConflict, eventID, ev.Type)
		}
	}
	if !contains(tr.from, st.Stage) {
		return State{}, fmt.Errorf("%w: 非法转换，阶段 %s 不接受事件 %s", ErrGuard, st.Stage, eventType)
	}
	inputs, candidate, err := g.snapshots()
	if err != nil {
		return State{}, err
	}
	if err := tr.guard(guardContext{g: g, st: st, inputs: inputs, candidate: candidate}); err != nil {
		return State{}, fmt.Errorf("%w: %s: %v", ErrGuard, eventType, err)
	}
	rev := st.Revision + 1
	if eventID == "" {
		eventID = fmt.Sprintf("evt-%d", rev)
		for _, ev := range st.Events {
			if ev.ID == eventID {
				return State{}, fmt.Errorf("%w: 自动事件 id %s 已被占用，请显式指定 --id", ErrConflict, eventID)
			}
		}
	}
	st.Events = append(st.Events, Event{ID: eventID, Type: eventType, From: st.Stage, To: tr.to, Revision: rev, At: g.now()})
	st.Stage = tr.to
	if err := g.commit(&st, inputs, candidate); err != nil {
		return State{}, err
	}
	return st, nil
}

func (g Graph) loadExpecting(expectRevision *int) (State, error) {
	st, err := g.readState()
	if err != nil {
		return State{}, err
	}
	if expectRevision != nil && *expectRevision != st.Revision {
		return State{}, fmt.Errorf("%w: 期望 revision %d，当前为 %d", ErrConflict, *expectRevision, st.Revision)
	}
	return st, nil
}

func (g Graph) snapshots() (inputs, candidate Snapshot, err error) {
	if inputs, err = g.inputsSnapshot(); err != nil {
		return
	}
	candidate, err = g.candidateSnapshot()
	return
}

// commit 绑定调用方判定或记录时所用的同一份摘要，递增 revision 并落盘；不重新扫描。
func (g Graph) commit(st *State, inputs, candidate Snapshot) error {
	st.Inputs, st.Candidate = inputs, candidate
	st.Revision++
	return g.writeState(*st, false)
}
