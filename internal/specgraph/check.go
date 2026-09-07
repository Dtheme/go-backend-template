package specgraph

import (
	"errors"
	"io/fs"
	"os"
)

func (g Graph) Status() (Status, error) {
	st, err := g.readState()
	if err != nil {
		return Status{}, err
	}
	inputs, candidate, err := g.snapshots()
	if err != nil {
		return Status{}, err
	}
	ds, err := g.readDelivery()
	if err != nil {
		return Status{}, err
	}
	s := Status{
		Version:          st.Version,
		Stage:            st.Stage,
		Revision:         st.Revision,
		Invalidated:      invalidated(st, inputs, candidate),
		DriftedInputs:    inputs.Digest != st.Inputs.Digest,
		DriftedCandidate: candidate.Digest != st.Candidate.Digest,
		FindingCounts:    map[string]int{},
		LatestEvidence:   map[string]Evidence{},
		Delivery:         ds,
	}
	for _, f := range st.Findings {
		s.FindingCounts[f.Status]++
	}
	for _, kind := range evidenceKinds {
		if ev, ok := latestEvidence(st, kind); ok {
			s.LatestEvidence[kind] = ev
		}
	}
	return s, nil
}

// Check 只读核对 graph.json 与技术方案交付状态的一致性；文件不存在返回 ErrNotInitialized，其余校验失败都作为 Problem。
func (g Graph) Check() ([]Problem, error) {
	var problems []Problem
	st, err := g.readState()
	if errors.Is(err, ErrNotInitialized) {
		return nil, err
	}
	if err != nil {
		return []Problem{{Path: g.graphRel(), Message: err.Error()}}, nil
	}
	if _, err := os.Lstat(g.lockPath()); err == nil {
		problems = append(problems, Problem{Path: g.graphRel() + ".lock", Message: "存在锁文件（" + lockDetail(g.lockPath()) +
			"），若无 spec-graph 正在运行则为中断残留，删除前 record / finding / event 都会被拒绝"})
	}

	planExists := true
	if _, err := os.Stat(g.abs(g.planRel())); errors.Is(err, fs.ErrNotExist) {
		planExists = false
		problems = append(problems, Problem{Path: g.planRel(), Message: "存在 graph.json 但缺少 技术方案.md"})
	}
	inputs, candidate, err := g.snapshots()
	if err != nil {
		if planExists {
			problems = append(problems, Problem{Path: g.graphRel(), Message: "无法计算摘要: " + err.Error()})
		}
		return problems, nil
	}
	inv := invalidated(st, inputs, candidate)
	if st.Stage == readyToDeliver && inv {
		problems = append(problems, Problem{Path: g.graphRel(), Message: "readiness 已失效，执行 event readiness_invalidated"})
	}
	ds, err := g.readDelivery()
	if err != nil {
		return append(problems, Problem{Path: g.planRel(), Message: err.Error()}), nil
	}
	if ds.Stage == "delivered" && (st.Stage != readyToDeliver || inv) {
		problems = append(problems, Problem{Path: g.planRel(), Message: "交付状态为 delivered，但 graph 阶段为 " + st.Stage + " 或已失效"})
	}
	if st.Stage == readyToDeliver && ds.UserAcceptance != "confirmed" {
		problems = append(problems, Problem{Path: g.planRel(), Message: "graph 已 ready_to_deliver，但交付状态 user_acceptance 为 " + ds.UserAcceptance})
	}
	return problems, nil
}
