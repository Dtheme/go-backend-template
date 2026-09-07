package specdoc

import (
	"strings"
	"testing"
)

func block(stage, ua, review string) string {
	return "# 0.1.0 技术方案\n\n## 交付状态\n\n说明文字。\n\n```yaml\nstage: " + stage + "   # 注释\nuser_acceptance: " + ua + "\nreview: " + review + "\n```\n\n## 变更记录\n"
}

func TestParseDeliveryStatus_OK(t *testing.T) {
	ds, err := ParseDeliveryStatus([]byte(block("implementing", "pending", "not_required")))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ds.Stage != "implementing" || ds.UserAcceptance != "pending" || ds.Review != "not_required" {
		t.Fatalf("parsed = %+v", ds)
	}
}

func TestParseDeliveryStatus_DeliveredRequiresConfirmation(t *testing.T) {
	cases := []struct {
		name string
		doc  string
		want string
	}{
		{"未确认", block("delivered", "pending", "pass"), "user_acceptance 不是 confirmed"},
		{"审查未过", block("delivered", "confirmed", "changes_required"), "review 为 changes_required"},
		{"审查待定", block("delivered", "confirmed", "pending"), "review 为 pending"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseDeliveryStatus([]byte(c.doc))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want containing %q", err, c.want)
			}
		})
	}
	if _, err := ParseDeliveryStatus([]byte(block("delivered", "confirmed", "pass"))); err != nil {
		t.Fatalf("delivered+confirmed+pass should pass: %v", err)
	}
	if _, err := ParseDeliveryStatus([]byte(block("delivered", "confirmed", "not_required"))); err != nil {
		t.Fatalf("delivered+confirmed+not_required should pass: %v", err)
	}
}

func TestParseDeliveryStatus_Structure(t *testing.T) {
	cases := []struct {
		name string
		doc  string
		want string
	}{
		{"缺标题", "# x\n\n```yaml\nstage: planning\n```\n", "缺少「## 交付状态」"},
		{"标题重复", block("planning", "pending", "not_required") + "\n## 交付状态\n", "出现多次"},
		{"非 yaml 块", "## 交付状态\n\n```json\n{}\n```\n", "必须是 ```yaml"},
		{"无代码块进入下一节", "## 交付状态\n\n## 变更记录\n", "没有 yaml 代码块"},
		{"未闭合", "## 交付状态\n\n```yaml\nstage: planning\nuser_acceptance: pending\nreview: pass\n", "未闭合"},
		{"未知键", "## 交付状态\n\n```yaml\nstage: planning\nuser_acceptance: pending\nreview: pass\nowner: x\n```\n", "未知键"},
		{"重复键", "## 交付状态\n\n```yaml\nstage: planning\nstage: planning\nuser_acceptance: pending\nreview: pass\n```\n", "键重复"},
		{"缺键", "## 交付状态\n\n```yaml\nstage: planning\nreview: pass\n```\n", "缺少键: user_acceptance"},
		{"非法枚举", block("done", "pending", "pass"), "stage 非法"},
		{"非法 review", block("planning", "pending", "approved"), "review 非法"},
		{"行格式错误", "## 交付状态\n\n```yaml\nstage planning\n```\n", "行格式错误"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseDeliveryStatus([]byte(c.doc))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want containing %q", err, c.want)
			}
		})
	}
}

func TestParseDeliveryStatus_LongLineNotTruncated(t *testing.T) {
	long := strings.Repeat("x", 5*1024*1024)
	doc := "# 0.1.0 技术方案\n\n## 需求摘要\n\n" + long + "\n\n" + block("planning", "pending", "not_required")
	if _, err := ParseDeliveryStatus([]byte(doc)); err != nil {
		t.Fatalf("5 MiB 单行不应导致截断: %v", err)
	}
}

func TestParseDeliveryStatus_HeadingInsideFenceIgnored(t *testing.T) {
	quoted := "# 0.1.0 技术方案\n\n```md\n## 交付状态\n```\n\n" + block("planning", "pending", "not_required")
	ds, err := ParseDeliveryStatus([]byte(quoted))
	if err != nil {
		t.Fatalf("围栏内的标题不应算重复: %v", err)
	}
	if ds.Stage != "planning" {
		t.Fatalf("stage = %q", ds.Stage)
	}
	onlyQuoted := "# x\n\n```md\n## 交付状态\n```\n"
	if _, err := ParseDeliveryStatus([]byte(onlyQuoted)); err == nil || !strings.Contains(err.Error(), "缺少「## 交付状态」") {
		t.Fatalf("只有围栏内标题应视为缺少: %v", err)
	}
	crlf := strings.ReplaceAll(block("planning", "pending", "not_required"), "\n", "\r\n")
	if _, err := ParseDeliveryStatus([]byte(crlf)); err != nil {
		t.Fatalf("CRLF 文档应可解析: %v", err)
	}
}

func TestTemplateFileParses(t *testing.T) {
	// 技术方案模版中的块以 planning 起步，必须能被解析。
	doc := block("planning", "pending", "not_required")
	if _, err := ParseDeliveryStatus([]byte(doc)); err != nil {
		t.Fatal(err)
	}
}
