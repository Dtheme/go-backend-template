// Package specgraph 维护版本生命周期状态图 Specs/technical/<version>/graph.json：
// 阶段、绑定 inputs 与 candidate 摘要的证据、finding 与事件历史。它只做确定性判定，不运行任何命令。
package specgraph

import (
	"bytes"
	"errors"
	"time"

	"example.com/go-backend-template/internal/specdoc"
)

var (
	ErrNotInitialized = errors.New("graph.json 不存在，先执行 init")
	ErrConflict       = errors.New("冲突")
	ErrGuard          = errors.New("守卫不满足")
	ErrUsage          = errors.New("用法错误")
)

type Graph struct {
	Root    string
	Version string
	Now     func() time.Time
}

type State struct {
	Version   string     `json:"version"`
	Revision  int        `json:"revision"`
	Stage     string     `json:"stage"`
	Inputs    Snapshot   `json:"inputs"`
	Candidate Snapshot   `json:"candidate"`
	Findings  []Finding  `json:"findings"`
	Evidence  []Evidence `json:"evidence"`
	Events    []Event    `json:"events"`
}

// Snapshot 的 Files 为 路径 → 内容 sha256 hex，Digest 为按路径排序后拼接 "路径\n摘要\n" 的 sha256。
// 唯一例外：inputs 中 Specs/technical/<version>/技术方案.md 的值是剔除「## 交付状态」块后的 sha256，
// 因为该块记录用户决定并由守卫直接读取，其变化不算需求或方案漂移；用 shasum 核对该文件时需先剔除此块。
type Snapshot struct {
	Files  map[string]string `json:"files"`
	Digest string            `json:"digest"`
}

type Finding struct {
	ID        string `json:"id"`
	Severity  string `json:"severity"`
	Status    string `json:"status"`
	Note      string `json:"note"`
	UpdatedAt string `json:"updated_at"`
}

type Evidence struct {
	Kind       string `json:"kind"`
	Candidate  string `json:"candidate"`
	Inputs     string `json:"inputs"`
	ExitCode   int    `json:"exit_code"`
	LogSHA256  string `json:"log_sha256"`
	RecordedAt string `json:"recorded_at"`
}

type Event struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	From     string `json:"from"`
	To       string `json:"to"`
	Revision int    `json:"revision"`
	At       string `json:"at"`
}

type Status struct {
	Version          string                 `json:"version"`
	Stage            string                 `json:"stage"`
	Revision         int                    `json:"revision"`
	Invalidated      bool                   `json:"invalidated"`
	DriftedInputs    bool                   `json:"drifted_inputs"`
	DriftedCandidate bool                   `json:"drifted_candidate"`
	FindingCounts    map[string]int         `json:"finding_counts"`
	LatestEvidence   map[string]Evidence    `json:"latest_evidence"`
	Delivery         specdoc.DeliveryStatus `json:"delivery"`
}

// MarshalJSON 把 Delivery 输出为 snake_case 三键（specdoc.DeliveryStatus 无 json tag），其余字段照常；不做 HTML 转义。
func (s Status) MarshalJSON() ([]byte, error) {
	type plain Status
	data, err := marshalJSON(struct {
		plain
		Delivery deliveryJSON `json:"delivery"`
	}{plain(s), deliveryJSON(s.Delivery)}, "")
	return bytes.TrimSuffix(data, []byte("\n")), err
}

type deliveryJSON struct {
	Stage          string `json:"stage"`
	UserAcceptance string `json:"user_acceptance"`
	Review         string `json:"review"`
}

type Problem struct {
	Path    string
	Message string
}
