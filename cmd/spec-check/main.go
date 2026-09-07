// spec-check 核对 Specs 文档与仓库结构的一致性，是 make check 的一部分。
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"example.com/go-backend-template/internal/speccheck"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run 返回退出码：0 无问题，1 有规则违反，2 参数或 I/O 错误。
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("spec-check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "仓库根目录")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	problems, err := speccheck.Run(*root)
	if err != nil {
		fmt.Fprintf(stderr, "spec-check: %v\n", err)
		return 2
	}
	if len(problems) > 0 {
		for _, p := range problems {
			fmt.Fprintln(stdout, p)
		}
		fmt.Fprintf(stdout, "spec-check: %d problem(s)\n", len(problems))
		return 1
	}

	versions, err := speccheck.Versions(*root)
	if err != nil {
		fmt.Fprintf(stderr, "spec-check: %v\n", err)
		return 2
	}
	fmt.Fprintf(stdout, "spec-check ok (%d version(s))\n", len(versions))
	return 0
}
