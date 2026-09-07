// spec-graph 是版本生命周期状态图的薄 CLI，所有逻辑在 internal/specgraph。
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"example.com/go-backend-template/internal/specgraph"
)

const usageText = `用法: spec-graph [--root <dir>] <子命令> <version> [参数]
  init <version>
  status <version> [--json]
  record <version> --kind <k> --exit <n> [--log <path>] [--expect-revision <n>]
  finding <version> --id <id> --severity <P0..P3> --status <s> [--note <text>] [--expect-revision <n>]
  event <version> <type> [--id <id>] [--expect-revision <n>] [--at <RFC3339>]
  check <version>
摘要规则: inputs / candidate 中每个文件的值为其内容的 sha256；唯一例外是 技术方案.md 按剔除「## 交付状态」块后的内容摘要。
写入规则: record / finding / event 在 graph.json.lock 独占锁内执行，锁被占用时以退出码 3 拒绝（进程被中断后残留的锁由 check 报告，确认无进程运行后手工删除）；
  finding 同 id 相同 --status 为幂等成功，不改 severity / note；合法转换不带 --note 时保留原 note。
`

var versionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	root, rest, err := splitRoot(args)
	if err != nil {
		return fail(stderr, fmt.Errorf("%w: %v", specgraph.ErrUsage, err))
	}
	if len(rest) < 2 {
		fmt.Fprint(stderr, usageText)
		return 2
	}
	cmd, version := rest[0], rest[1]
	if !versionPattern.MatchString(version) {
		return fail(stderr, fmt.Errorf("%w: version %q 必须形如 x.y.z", specgraph.ErrUsage, version))
	}
	g := specgraph.Graph{Root: root, Version: version}
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(stderr)

	var st specgraph.State
	switch cmd {
	case "init":
		if err = parse(fs, rest[2:]); err == nil {
			st, err = g.Init()
		}
	case "status":
		asJSON := fs.Bool("json", false, "以 JSON 输出")
		if err = parse(fs, rest[2:]); err != nil {
			break
		}
		var s specgraph.Status
		if s, err = g.Status(); err != nil {
			break
		}
		return printStatus(stdout, stderr, s, *asJSON)
	case "record":
		kind := fs.String("kind", "", "证据类型")
		exit := fs.Int("exit", -1, "命令退出码")
		logPath := fs.String("log", "", "日志文件")
		expect := fs.Int("expect-revision", -1, "期望的当前 revision")
		if err = parse(fs, rest[2:]); err != nil {
			break
		}
		if err = required(fs, "kind", "exit"); err != nil {
			break
		}
		st, err = g.Record(*kind, *exit, *logPath, optional(fs, "expect-revision", expect))
	case "finding":
		id := fs.String("id", "", "finding id")
		severity := fs.String("severity", "", "P0..P3")
		status := fs.String("status", "", "finding 状态")
		note := fs.String("note", "", "说明")
		expect := fs.Int("expect-revision", -1, "期望的当前 revision")
		if err = parse(fs, rest[2:]); err != nil {
			break
		}
		if err = required(fs, "id", "severity", "status"); err != nil {
			break
		}
		st, err = g.Finding(*id, *severity, *status, *note, optional(fs, "expect-revision", expect))
	case "event":
		if len(rest) < 3 {
			err = fmt.Errorf("%w: event 需要事件类型", specgraph.ErrUsage)
			break
		}
		id := fs.String("id", "", "事件 id")
		expect := fs.Int("expect-revision", -1, "期望的当前 revision")
		at := fs.String("at", "", "事件时间 RFC3339")
		if err = parse(fs, rest[3:]); err != nil {
			break
		}
		if *at != "" {
			var ts time.Time
			if ts, err = time.Parse(time.RFC3339, *at); err != nil {
				err = fmt.Errorf("%w: --at 必须是 RFC3339: %v", specgraph.ErrUsage, err)
				break
			}
			g.Now = func() time.Time { return ts }
		}
		st, err = g.Apply(rest[2], *id, optional(fs, "expect-revision", expect))
	case "check":
		if err = parse(fs, rest[2:]); err != nil {
			break
		}
		var problems []specgraph.Problem
		if problems, err = g.Check(); err != nil {
			break
		}
		for _, p := range problems {
			fmt.Fprintf(stdout, "%s: %s\n", p.Path, p.Message)
		}
		if len(problems) > 0 {
			return 1
		}
		return 0
	default:
		err = fmt.Errorf("%w: 未知子命令 %q", specgraph.ErrUsage, cmd)
	}
	if err != nil {
		return fail(stderr, err)
	}
	fmt.Fprintf(stdout, "stage=%s revision=%d\n", st.Stage, st.Revision)
	return 0
}

// splitRoot 只解析子命令之前的 --root <dir> 或 --root=<dir>，其余参数原样返回，避免吞掉子命令 flag 的取值。
func splitRoot(args []string) (string, []string, error) {
	root := "."
	for len(args) > 0 {
		switch {
		case args[0] == "--root":
			if len(args) < 2 {
				return "", nil, errors.New("--root 缺少目录")
			}
			root, args = args[1], args[2:]
		case strings.HasPrefix(args[0], "--root="):
			root, args = strings.TrimPrefix(args[0], "--root="), args[1:]
		default:
			return root, args, nil
		}
		if root == "" {
			return "", nil, errors.New("--root 缺少目录")
		}
	}
	return root, args, nil
}

func parse(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w: %v", specgraph.ErrUsage, err)
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("%w: 多余参数 %v", specgraph.ErrUsage, fs.Args())
	}
	return nil
}

func required(fs *flag.FlagSet, names ...string) error {
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	for _, n := range names {
		if !set[n] {
			return fmt.Errorf("%w: 缺少 --%s", specgraph.ErrUsage, n)
		}
	}
	return nil
}

func optional(fs *flag.FlagSet, name string, v *int) *int {
	var set bool
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			set = true
		}
	})
	if !set {
		return nil
	}
	return v
}

func printStatus(stdout, stderr io.Writer, s specgraph.Status, asJSON bool) int {
	if asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		if err := enc.Encode(s); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	fmt.Fprintf(stdout, "version: %s\nstage: %s\nrevision: %d\ninvalidated: %t\ndrifted_inputs: %t\ndrifted_candidate: %t\n",
		s.Version, s.Stage, s.Revision, s.Invalidated, s.DriftedInputs, s.DriftedCandidate)
	fmt.Fprintf(stdout, "delivery: stage=%s user_acceptance=%s review=%s\n", s.Delivery.Stage, s.Delivery.UserAcceptance, s.Delivery.Review)
	fmt.Fprintln(stdout, "findings:")
	for _, k := range sortedKeys(s.FindingCounts) {
		fmt.Fprintf(stdout, "  %s: %d\n", k, s.FindingCounts[k])
	}
	fmt.Fprintln(stdout, "latest_evidence:")
	for _, k := range sortedKeys(s.LatestEvidence) {
		e := s.LatestEvidence[k]
		fmt.Fprintf(stdout, "  %s: exit_code=%d recorded_at=%s candidate=%.12s inputs=%.12s\n", k, e.ExitCode, e.RecordedAt, e.Candidate, e.Inputs)
	}
	return 0
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func fail(stderr io.Writer, err error) int {
	fmt.Fprintln(stderr, err)
	switch {
	case errors.Is(err, specgraph.ErrUsage):
		return 2
	case errors.Is(err, specgraph.ErrGuard), errors.Is(err, specgraph.ErrConflict):
		return 3
	default:
		return 1
	}
}
