package speccheck

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"example.com/go-backend-template/internal/specdoc"
)

const (
	requirementsDir = "Specs/requirements"
	technicalDir    = "Specs/technical"
	requirementFile = "需求.md"
	planFile        = "技术方案.md"
)

var (
	versionPattern   = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)
	featureIDPattern = regexp.MustCompile(`F-([0-9]+\.[0-9]+\.[0-9]+)-[0-9]{3}`)
)

var planHeadings = []string{
	"## 需求摘要", "## API 契约", "## 模块设计", "## 测试计划",
	"## 风险与回滚", "## 文件清单", specdoc.DeliveryHeading, "## 变更记录",
}

var allowedVersionFiles = map[string]map[string]bool{
	requirementsDir: {requirementFile: true},
	technicalDir:    {planFile: true, "graph.json": true},
}

// Versions 返回两个 Specs 目录下合法版本目录名的并集，已排序。
func Versions(root string) ([]string, error) {
	c := &checker{root: root}
	reqs, err := c.versionDirs(requirementsDir)
	if err != nil {
		return nil, err
	}
	techs, err := c.versionDirs(technicalDir)
	if err != nil {
		return nil, err
	}
	return unionVersions(reqs, techs), nil
}

// versionDirs 列出目录下的版本子目录（含指向目录的符号链接）；名字不合法的只记录问题，不参与后续检查。
func (c *checker) versionDirs(dir string) (map[string]bool, error) {
	entries, err := os.ReadDir(filepath.Join(c.root, dir))
	if missing(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取 %s: %w", dir, err)
	}
	versions := map[string]bool{}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		rel := dir + "/" + e.Name()
		isDir, err := c.entryIsDir(rel, e)
		if err != nil {
			return nil, err
		}
		switch {
		case !isDir && versionPattern.MatchString(e.Name()):
			c.add(rel, "版本条目必须是目录")
		case !isDir:
		case !versionPattern.MatchString(e.Name()):
			c.add(rel, "版本目录名必须是 x.y.z 形式")
		default:
			versions[e.Name()] = true
		}
	}
	return versions, nil
}

// entryIsDir 解析符号链接后判断目录；DirEntry.IsDir 对指向目录的符号链接恒为 false。
func (c *checker) entryIsDir(rel string, e fs.DirEntry) (bool, error) {
	if e.IsDir() {
		return true, nil
	}
	info, err := os.Stat(filepath.Join(c.root, rel))
	if missing(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("读取 %s: %w", rel, err)
	}
	return info.IsDir(), nil
}

func unionVersions(a, b map[string]bool) []string {
	set := map[string]bool{}
	for v := range a {
		set[v] = true
	}
	for v := range b {
		set[v] = true
	}
	out := make([]string, 0, len(set))
	for v := range set {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func (c *checker) checkVersions() error {
	reqs, err := c.versionDirs(requirementsDir)
	if err != nil {
		return err
	}
	techs, err := c.versionDirs(technicalDir)
	if err != nil {
		return err
	}
	for _, v := range unionVersions(reqs, techs) {
		if err := c.checkVersion(v, reqs[v], techs[v]); err != nil {
			return err
		}
	}
	return nil
}

func (c *checker) checkVersion(v string, hasReq, hasTech bool) error {
	reqPath := requirementsDir + "/" + v + "/" + requirementFile
	planPath := technicalDir + "/" + v + "/" + planFile

	switch {
	case !hasTech:
		c.add(planPath, "缺少技术方案，运行 make spec-init VERSION=%s", v)
	case !hasReq:
		c.add(technicalDir+"/"+v, "没有对应人工需求")
	}

	var req, plan []byte
	var err error
	if hasReq {
		if err = c.checkUnexpectedFiles(requirementsDir, v); err != nil {
			return err
		}
		if req, err = c.read(reqPath); err != nil {
			return err
		}
	}
	if hasTech {
		if err = c.checkUnexpectedFiles(technicalDir, v); err != nil {
			return err
		}
		if plan, err = c.read(planPath); err != nil {
			return err
		}
	}

	var reqIDs []string
	if req != nil {
		reqIDs = c.checkRequirementIDs(reqPath, v, req)
	}
	if plan != nil {
		c.checkPlan(planPath, v, plan, reqIDs)
	}
	return nil
}

func (c *checker) checkUnexpectedFiles(dir, v string) error {
	rel := dir + "/" + v
	entries, err := os.ReadDir(filepath.Join(c.root, rel))
	if err != nil {
		return fmt.Errorf("读取 %s: %w", rel, err)
	}
	allowed := allowedVersionFiles[dir]
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") || allowed[e.Name()] {
			continue
		}
		c.add(rel+"/"+e.Name(), "意外文件")
	}
	return nil
}

// checkRequirementIDs 返回需求中版本一致的 Feature ID，去重且有序。
// 版本不一致的 ID 只报一致性问题，不再追溯技术方案引用，避免同一根因报两条。
func (c *checker) checkRequirementIDs(rel, v string, doc []byte) []string {
	ids := featureIDs(doc)
	if len(ids) == 0 {
		c.add(rel, "至少一个 Feature ID（F-%s-NNN）", v)
		return nil
	}
	var matched []string
	for _, id := range ids {
		if featureVersion(id) != v {
			c.add(rel, "Feature ID %s 的版本与目录 %s 不一致", id, v)
			continue
		}
		matched = append(matched, id)
	}
	return matched
}

func (c *checker) checkPlan(rel, v string, doc []byte, reqIDs []string) {
	c.checkHeadings(rel, doc, planHeadings, true)
	if _, err := specdoc.ParseDeliveryStatus(doc); err != nil {
		c.add(rel, "%s", err.Error())
	}
	planIDs := map[string]bool{}
	for _, id := range featureIDs(doc) {
		planIDs[id] = true
		if featureVersion(id) != v {
			c.add(rel, "Feature ID %s 的版本与目录 %s 不一致", id, v)
		}
	}
	for _, id := range reqIDs {
		if !planIDs[id] {
			c.add(rel, "技术方案未引用 %s", id)
		}
	}
}

// featureIDs 收集文档中的 Feature ID；紧贴字母数字的前缀（REF-…）或超过三位的编号（…-0002）不算。
func featureIDs(doc []byte) []string {
	seen := map[string]bool{}
	var ids []string
	for _, loc := range featureIDPattern.FindAllIndex(doc, -1) {
		start, end := loc[0], loc[1]
		if start > 0 && isAlnum(doc[start-1]) || end < len(doc) && isDigit(doc[end]) {
			continue
		}
		id := string(doc[start:end])
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

func isAlnum(b byte) bool {
	return isDigit(b) || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

func featureVersion(id string) string {
	return featureIDPattern.FindStringSubmatch(id)[1]
}
