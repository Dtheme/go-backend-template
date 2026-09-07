package specgraph

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"example.com/go-backend-template/internal/specdoc"
)

// 候选目录与 go.mod、go.sum 存在才算；Makefile 必须存在。
var (
	candidateDirs  = []string{"cmd", "internal", "scripts", "api", "migrations"}
	candidateFiles = []struct {
		name     string
		required bool
	}{{"go.mod", false}, {"go.sum", false}, {"Makefile", true}}
)

func (g Graph) abs(rel string) string {
	return filepath.Join(g.Root, filepath.FromSlash(rel))
}

func (g Graph) technicalDir() string { return "Specs/technical/" + g.Version }
func (g Graph) planRel() string      { return g.technicalDir() + "/技术方案.md" }
func (g Graph) graphRel() string     { return g.technicalDir() + "/graph.json" }

func (g Graph) inputRels() []string {
	return []string{
		"Specs/requirements/" + g.Version + "/需求.md",
		"Specs/requirements/协议与数据.md",
		g.planRel(),
	}
}

// inputsSnapshot 对技术方案剔除「交付状态」块后再摘要（规则见 Snapshot 文档与 CLI 用法）；若按整文件摘要，
// Step D 记证据后再确认验收、Step E 改为 delivered 都会让 inputs 漂移，verified 守卫与 check 的 delivered 规则无法满足。
func (g Graph) inputsSnapshot() (Snapshot, error) {
	files := map[string]string{}
	for _, rel := range g.inputRels() {
		raw, err := os.ReadFile(g.abs(rel))
		if err != nil {
			return Snapshot{}, fmt.Errorf("读取输入文件 %s: %w", rel, err)
		}
		if rel == g.planRel() {
			raw = stripDeliveryBlock(raw)
		}
		sum := sha256.Sum256(raw)
		files[rel] = hex.EncodeToString(sum[:])
	}
	return newSnapshot(files), nil
}

// stripDeliveryBlock 去掉「## 交付状态」标题到其后第一个代码块结束的内容；找不到完整块时原样返回。
func stripDeliveryBlock(doc []byte) []byte {
	lines := strings.SplitAfter(string(doc), "\n")
	start := -1
	for i, l := range lines {
		if strings.TrimRight(l, " \t\r\n") == specdoc.DeliveryHeading {
			start = i
			break
		}
	}
	if start < 0 {
		return doc
	}
	fences := 0
	for i := start + 1; i < len(lines); i++ {
		l := strings.TrimSpace(lines[i])
		if fences == 0 && strings.HasPrefix(l, "#") {
			return doc
		}
		if strings.HasPrefix(l, "```") {
			if fences++; fences == 2 {
				return []byte(strings.Join(lines[:start], "") + strings.Join(lines[i+1:], ""))
			}
		}
	}
	return doc
}

// candidateSnapshot 只收集普通文件；符号链接不跟随：候选根是符号链接时视为不存在，其内部的符号链接与以 . 开头的目录 / 文件一律跳过。
func (g Graph) candidateSnapshot() (Snapshot, error) {
	files := map[string]string{}
	add := func(path string) error {
		rel, err := filepath.Rel(g.Root, path)
		if err != nil {
			return err
		}
		sum, err := fileSHA256(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = sum
		return nil
	}
	lstat := func(name string) (fs.FileInfo, error) {
		info, err := os.Lstat(g.abs(name))
		if err == nil && info.Mode()&fs.ModeSymlink != 0 {
			return nil, fmt.Errorf("候选路径 %s 是符号链接: %w", name, fs.ErrNotExist)
		}
		return info, err
	}
	for _, dir := range candidateDirs {
		root := g.abs(dir)
		info, err := lstat(dir)
		if errors.Is(err, fs.ErrNotExist) || (err == nil && !info.IsDir()) {
			continue
		}
		if err != nil {
			return Snapshot{}, err
		}
		err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if path != root && strings.HasPrefix(d.Name(), ".") {
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if !d.Type().IsRegular() {
				return nil
			}
			return add(path)
		})
		if err != nil {
			return Snapshot{}, fmt.Errorf("扫描 %s: %w", dir, err)
		}
	}
	for _, f := range candidateFiles {
		info, err := lstat(f.name)
		if errors.Is(err, fs.ErrNotExist) || (err == nil && !info.Mode().IsRegular()) {
			if f.required {
				return Snapshot{}, fmt.Errorf("候选文件 %s 必须是普通文件: %v", f.name, err)
			}
			continue
		}
		if err != nil {
			return Snapshot{}, err
		}
		if err := add(g.abs(f.name)); err != nil {
			return Snapshot{}, err
		}
	}
	return newSnapshot(files), nil
}

func newSnapshot(files map[string]string) Snapshot {
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, p := range paths {
		io.WriteString(h, p+"\n"+files[p]+"\n")
	}
	return Snapshot{Files: files, Digest: hex.EncodeToString(h.Sum(nil))}
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
