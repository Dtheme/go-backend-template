// Package specdoc 解析技术方案中机器可读的「交付状态」块，供 spec-check 与 spec-graph 共用。
package specdoc

import (
	"errors"
	"fmt"
	"strings"
)

const DeliveryHeading = "## 交付状态"

type DeliveryStatus struct {
	Stage          string
	UserAcceptance string
	Review         string
}

var (
	StageValues          = []string{"planning", "implementing", "verifying", "delivered"}
	UserAcceptanceValues = []string{"pending", "confirmed"}
	ReviewValues         = []string{"not_required", "pending", "changes_required", "pass"}
)

var ErrNotFound = errors.New("缺少「## 交付状态」一节")

// ParseDeliveryStatus 要求文档中恰好有一个「## 交付状态」标题，其后第一个代码块必须是 yaml，
// 且只包含 stage、user_acceptance、review 三个键。
func ParseDeliveryStatus(doc []byte) (DeliveryStatus, error) {
	var ds DeliveryStatus
	lines := splitLines(doc)

	// 围栏代码块内的「## 交付状态」是引用，不算标题。
	headingIdx := -1
	inFence := false
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if strings.TrimRight(l, " \t") == DeliveryHeading {
			if headingIdx >= 0 {
				return ds, errors.New("「## 交付状态」出现多次")
			}
			headingIdx = i
		}
	}
	if headingIdx < 0 {
		return ds, ErrNotFound
	}

	start := -1
	for i := headingIdx + 1; i < len(lines); i++ {
		l := strings.TrimSpace(lines[i])
		if strings.HasPrefix(l, "#") {
			return ds, errors.New("「## 交付状态」之后没有 yaml 代码块就进入了下一节")
		}
		if strings.HasPrefix(l, "```") {
			if l != "```yaml" {
				return ds, fmt.Errorf("「## 交付状态」的代码块必须是 ```yaml，实际为 %q", l)
			}
			start = i + 1
			break
		}
	}
	if start < 0 {
		return ds, errors.New("「## 交付状态」之后缺少 yaml 代码块")
	}

	seen := map[string]bool{}
	closed := false
	for i := start; i < len(lines); i++ {
		l := lines[i]
		if strings.TrimSpace(l) == "```" {
			closed = true
			break
		}
		if idx := strings.Index(l, "#"); idx >= 0 {
			l = l[:idx]
		}
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		key, val, ok := strings.Cut(l, ":")
		if !ok {
			return ds, fmt.Errorf("交付状态行格式错误: %q", l)
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if seen[key] {
			return ds, fmt.Errorf("交付状态键重复: %s", key)
		}
		seen[key] = true
		switch key {
		case "stage":
			ds.Stage = val
		case "user_acceptance":
			ds.UserAcceptance = val
		case "review":
			ds.Review = val
		default:
			return ds, fmt.Errorf("交付状态出现未知键: %s", key)
		}
	}
	if !closed {
		return ds, errors.New("交付状态 yaml 代码块未闭合")
	}
	for _, k := range []string{"stage", "user_acceptance", "review"} {
		if !seen[k] {
			return ds, fmt.Errorf("交付状态缺少键: %s", k)
		}
	}
	if err := ds.Validate(); err != nil {
		return ds, err
	}
	return ds, nil
}

// Validate 校验枚举值与交付门禁：delivered 要求用户已确认且 review 为 pass 或 not_required。
func (d DeliveryStatus) Validate() error {
	if !contains(StageValues, d.Stage) {
		return fmt.Errorf("stage 非法: %q（允许 %s）", d.Stage, strings.Join(StageValues, " | "))
	}
	if !contains(UserAcceptanceValues, d.UserAcceptance) {
		return fmt.Errorf("user_acceptance 非法: %q（允许 %s）", d.UserAcceptance, strings.Join(UserAcceptanceValues, " | "))
	}
	if !contains(ReviewValues, d.Review) {
		return fmt.Errorf("review 非法: %q（允许 %s）", d.Review, strings.Join(ReviewValues, " | "))
	}
	if d.Stage == "delivered" {
		if d.UserAcceptance != "confirmed" {
			return errors.New("stage 为 delivered 但 user_acceptance 不是 confirmed")
		}
		if d.Review != "pass" && d.Review != "not_required" {
			return fmt.Errorf("stage 为 delivered 但 review 为 %s", d.Review)
		}
	}
	return nil
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// splitLines 不用 bufio.Scanner：超长行会让 Scan 静默失败并截断文档。
func splitLines(doc []byte) []string {
	lines := strings.Split(string(doc), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, "\r")
	}
	return lines
}
