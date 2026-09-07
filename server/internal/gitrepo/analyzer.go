package gitrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	DefaultMaxFileBytes = 200_000
	defaultMaxKeyFiles  = 48
	defaultExcerptBytes = 8_000
)

type ScanOptions struct {
	MaxFileBytes int
	MaxKeyFiles  int
}

type AnalyzeRequest struct {
	RepoURL string
	Goal    string
}

type AnalyzeResult struct {
	Scan     WorkspaceScan
	Markdown string
}

type Analyzer struct {
	WorkDir   string
	generator TextGenerator
}

type TextGenerator interface {
	Complete(ctx context.Context, systemPrompt string, userPrompt string) (string, error)
}

type WorkspaceScan struct {
	RepoURL      string
	TreeMarkdown string
	KeyFiles     []FileSummary
	Languages    []string
	RunHints     []string
	SkippedFiles []string
}

type FileSummary struct {
	Path      string
	Language  string
	Excerpt   string
	SizeBytes int64
}

func NewAnalyzer(workDir string) *Analyzer {
	return &Analyzer{WorkDir: strings.TrimSpace(workDir)}
}

func NewAnalyzerWithGenerator(workDir string, generator TextGenerator) *Analyzer {
	return &Analyzer{
		WorkDir:   strings.TrimSpace(workDir),
		generator: generator,
	}
}

func (a *Analyzer) Analyze(ctx context.Context, req AnalyzeRequest) (AnalyzeResult, error) {
	root, cleanup, err := a.clone(ctx, req.RepoURL)
	if err != nil {
		return AnalyzeResult{}, err
	}
	defer cleanup()

	scan, err := ScanWorkspace(root, ScanOptions{})
	if err != nil {
		return AnalyzeResult{}, err
	}
	scan.RepoURL = strings.TrimSpace(req.RepoURL)
	markdown, err := a.GenerateCodeWiki(ctx, req.Goal, scan)
	if err != nil {
		markdown = BuildFallbackCodeWiki(req.Goal, scan)
	}
	return AnalyzeResult{
		Scan:     scan,
		Markdown: markdown,
	}, nil
}

func (a *Analyzer) GenerateCodeWiki(ctx context.Context, goal string, scan WorkspaceScan) (string, error) {
	if a.generator == nil {
		return BuildFallbackCodeWiki(goal, scan), nil
	}
	systemPrompt, userPrompt := BuildCodeWikiPrompts(goal, scan)
	markdown, err := a.generator.Complete(ctx, systemPrompt, userPrompt)
	if err != nil || strings.TrimSpace(markdown) == "" {
		return "", err
	}
	return strings.TrimSpace(markdown), nil
}

func BuildCodeWikiPrompts(goal string, scan WorkspaceScan) (string, string) {
	systemPrompt := strings.Join([]string{
		"You are Knowvia's GitHub repository analysis agent.",
		"Write a structured Code Wiki in Markdown based only on the repository tree and selected file excerpts provided by the system.",
		"Do not claim files were analyzed if they are not present in the supplied context.",
		"Include architecture, module responsibilities, key classes/functions, dependencies, run/test commands, extension points, and limitations.",
	}, " ")
	parts := []string{
		"用户目标： " + strings.TrimSpace(goal),
		"仓库地址： " + strings.TrimSpace(scan.RepoURL),
		"语言： " + strings.Join(scan.Languages, "、"),
		"运行线索： " + strings.Join(scan.RunHints, "；"),
		"",
		"目录树：",
		scan.TreeMarkdown,
		"",
		"关键文件摘录：",
	}
	for _, file := range scan.KeyFiles {
		parts = append(parts,
			fmt.Sprintf("### %s (%s, %d bytes)", file.Path, file.Language, file.SizeBytes),
			file.Excerpt,
			"",
		)
	}
	if len(scan.SkippedFiles) > 0 {
		parts = append(parts, "跳过文件：", strings.Join(scan.SkippedFiles, "\n"))
	}
	return systemPrompt, strings.Join(parts, "\n")
}

func (a *Analyzer) clone(ctx context.Context, repoURL string) (string, func(), error) {
	baseDir := strings.TrimSpace(a.WorkDir)
	if baseDir == "" {
		baseDir = os.TempDir()
	}
	tmpDir, err := os.MkdirTemp(baseDir, "quickque-github-*")
	if err != nil {
		return "", func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(tmpDir) }
	target := filepath.Join(tmpDir, "repo")
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", strings.TrimSpace(repoURL), target)
	output, err := cmd.CombinedOutput()
	if err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("clone github repo: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return target, cleanup, nil
}

func ScanWorkspace(root string, options ScanOptions) (WorkspaceScan, error) {
	maxFileBytes := options.MaxFileBytes
	if maxFileBytes <= 0 {
		maxFileBytes = DefaultMaxFileBytes
	}
	maxKeyFiles := options.MaxKeyFiles
	if maxKeyFiles <= 0 {
		maxKeyFiles = defaultMaxKeyFiles
	}

	entries := []string{}
	candidates := []FileSummary{}
	skipped := []string{}
	languageSet := map[string]bool{}
	runHintSet := map[string]bool{}

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if shouldSkipDir(entry.Name()) {
				return filepath.SkipDir
			}
			entries = append(entries, rel+"/")
			return nil
		}
		if shouldSkipFile(rel) {
			skipped = append(skipped, rel)
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		if info.Size() > int64(maxFileBytes) {
			skipped = append(skipped, rel)
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if isBinary(content) {
			skipped = append(skipped, rel)
			return nil
		}
		language := languageForPath(rel)
		if language != "" {
			languageSet[language] = true
		}
		excerpt := strings.TrimSpace(string(content))
		if len(excerpt) > defaultExcerptBytes {
			excerpt = excerpt[:defaultExcerptBytes]
		}
		summary := FileSummary{
			Path:      rel,
			Language:  language,
			Excerpt:   excerpt,
			SizeBytes: info.Size(),
		}
		candidates = append(candidates, summary)
		entries = append(entries, rel)
		for _, hint := range runHintsForFile(rel, content) {
			runHintSet[hint] = true
		}
		return nil
	})
	if err != nil {
		return WorkspaceScan{}, err
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		left := fileScore(candidates[i].Path)
		right := fileScore(candidates[j].Path)
		if left != right {
			return left > right
		}
		return candidates[i].Path < candidates[j].Path
	})
	if len(candidates) > maxKeyFiles {
		candidates = candidates[:maxKeyFiles]
	}

	return WorkspaceScan{
		TreeMarkdown: buildTreeMarkdown(entries),
		KeyFiles:     candidates,
		Languages:    sortedKeys(languageSet),
		RunHints:     sortedKeys(runHintSet),
		SkippedFiles: skipped,
	}, nil
}

func BuildFallbackCodeWiki(goal string, scan WorkspaceScan) string {
	repoLabel := strings.TrimSpace(scan.RepoURL)
	if repoLabel == "" {
		repoLabel = "当前仓库"
	}
	languages := "未识别"
	if len(scan.Languages) > 0 {
		languages = strings.Join(scan.Languages, "、")
	}
	runHints := "- 未从 manifest 中识别到明确运行命令。"
	if len(scan.RunHints) > 0 {
		lines := make([]string, 0, len(scan.RunHints))
		for _, hint := range scan.RunHints {
			lines = append(lines, "- `"+hint+"`")
		}
		runHints = strings.Join(lines, "\n")
	}

	moduleLines := []string{}
	functionLines := []string{}
	for _, file := range scan.KeyFiles {
		moduleLines = append(moduleLines, fmt.Sprintf("- `%s`：%s 文件，建议作为理解仓库职责的关键入口。", file.Path, firstNonEmpty(file.Language, "text")))
		functionLines = append(functionLines, functionHints(file)...)
	}
	if len(moduleLines) == 0 {
		moduleLines = []string{"- 当前扫描没有选出可读的关键源码文件。"}
	}
	if len(functionLines) == 0 {
		functionLines = []string{"- 未在关键文件摘录中识别到显式类或函数定义，建议继续打开关键文件人工核查。"}
	}

	skipped := "- 未跳过重要文本文件。"
	if len(scan.SkippedFiles) > 0 {
		limit := len(scan.SkippedFiles)
		if limit > 12 {
			limit = 12
		}
		lines := make([]string, 0, limit)
		for _, item := range scan.SkippedFiles[:limit] {
			lines = append(lines, "- `"+item+"`")
		}
		if len(scan.SkippedFiles) > limit {
			lines = append(lines, fmt.Sprintf("- 另有 %d 个文件因目录、体积或二进制格式被跳过。", len(scan.SkippedFiles)-limit))
		}
		skipped = strings.Join(lines, "\n")
	}

	return strings.Join([]string{
		"# Code Wiki",
		"",
		"## 项目简介",
		fmt.Sprintf("仓库：%s", repoLabel),
		fmt.Sprintf("用户目标：%s", strings.TrimSpace(goal)),
		fmt.Sprintf("主要语言：%s", languages),
		"",
		"## 快速开始",
		runHints,
		"",
		"## 项目整体架构",
		"该仓库的结构可先从根目录 manifest、README 和主要源码目录理解。下面的目录树来自后台只读扫描，未执行仓库代码。",
		"",
		scan.TreeMarkdown,
		"",
		"## 主要模块职责",
		strings.Join(moduleLines, "\n"),
		"",
		"## 关键类与函数",
		strings.Join(functionLines, "\n"),
		"",
		"## 依赖关系",
		"- 依赖关系优先从 `package.json`、`go.mod`、`pyproject.toml`、`requirements.txt` 等 manifest 文件推断。",
		"- 模块调用关系基于关键文件路径和摘录推断；大型仓库需要进一步建立代码索引才能覆盖所有调用链。",
		"",
		"## 项目运行方式",
		runHints,
		"",
		"## 测试方式",
		testHints(runHints),
		"",
		"## 扩展点和二次开发建议",
		"- 从入口文件和服务层开始阅读，先确认启动命令、配置来源和核心调用链。",
		"- 对未覆盖的大文件或生成目录建立二次索引后，可继续补充 API、类图和调用链章节。",
		"",
		"## 限制与未覆盖内容",
		skipped,
	}, "\n")
}

func shouldSkipDir(name string) bool {
	switch strings.ToLower(name) {
	case ".git", "node_modules", "dist", "build", ".next", ".expo", "coverage", "target", "vendor", "__pycache__":
		return true
	default:
		return false
	}
}

func shouldSkipFile(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	if strings.HasPrefix(name, ".env") || strings.HasSuffix(name, ".pem") || strings.HasSuffix(name, ".key") {
		return true
	}
	switch filepath.Ext(name) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico", ".pdf", ".zip", ".gz", ".jar", ".lockb":
		return true
	default:
		return false
	}
}

func isBinary(content []byte) bool {
	if len(content) == 0 {
		return false
	}
	if !utf8.Valid(content) {
		return true
	}
	for _, b := range content {
		if b == 0 {
			return true
		}
	}
	return false
}

func languageForPath(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return "Go"
	case ".ts", ".tsx":
		return "TypeScript"
	case ".js", ".jsx", ".mjs", ".cjs":
		return "JavaScript"
	case ".py":
		return "Python"
	case ".md", ".mdx":
		return "Markdown"
	case ".json":
		return "JSON"
	case ".yaml", ".yml":
		return "YAML"
	case ".rs":
		return "Rust"
	case ".java":
		return "Java"
	case ".kt":
		return "Kotlin"
	case ".swift":
		return "Swift"
	default:
		return "Text"
	}
}

func fileScore(path string) int {
	lower := strings.ToLower(path)
	score := 0
	switch filepath.Base(lower) {
	case "readme.md", "package.json", "go.mod", "pyproject.toml", "requirements.txt", "cargo.toml", "pom.xml", "build.gradle", "makefile", "dockerfile":
		score += 100
	}
	if strings.Contains(lower, "/cmd/") || strings.Contains(lower, "/src/") || strings.Contains(lower, "/internal/") || strings.Contains(lower, "/app/") {
		score += 30
	}
	if strings.Contains(lower, "main.") || strings.Contains(lower, "server.") || strings.Contains(lower, "app.") || strings.Contains(lower, "index.") {
		score += 20
	}
	return score
}

func runHintsForFile(path string, content []byte) []string {
	switch strings.ToLower(filepath.Base(path)) {
	case "package.json":
		var payload struct {
			Scripts map[string]string `json:"scripts"`
		}
		if err := json.Unmarshal(content, &payload); err != nil {
			return nil
		}
		hints := []string{}
		for _, name := range []string{"dev", "start", "build", "test"} {
			if _, ok := payload.Scripts[name]; ok {
				hints = append(hints, "npm run "+name)
			}
		}
		return hints
	case "go.mod":
		return []string{"go test ./...", "go run ./..."}
	case "pyproject.toml":
		return []string{"python -m pytest"}
	case "requirements.txt":
		return []string{"pip install -r requirements.txt"}
	default:
		return nil
	}
}

func buildTreeMarkdown(entries []string) string {
	sort.Strings(entries)
	if len(entries) == 0 {
		return "- 当前仓库没有可展示的文本文件树。"
	}
	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		depth := strings.Count(strings.TrimSuffix(entry, "/"), "/")
		indent := strings.Repeat("  ", depth)
		lines = append(lines, fmt.Sprintf("%s- %s", indent, filepath.Base(strings.TrimSuffix(entry, "/"))+dirSuffix(entry)))
	}
	return strings.Join(lines, "\n")
}

func dirSuffix(entry string) string {
	if strings.HasSuffix(entry, "/") {
		return "/"
	}
	return ""
}

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func functionHints(file FileSummary) []string {
	lines := []string{}
	for _, line := range strings.Split(file.Excerpt, "\n") {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "func ") ||
			strings.HasPrefix(lower, "function ") ||
			strings.HasPrefix(lower, "def ") ||
			strings.HasPrefix(lower, "class ") ||
			strings.Contains(lower, " class ") ||
			strings.Contains(lower, " function ") {
			lines = append(lines, fmt.Sprintf("- `%s`：`%s`", file.Path, trimmed))
		}
		if len(lines) >= 16 {
			break
		}
	}
	return lines
}

func testHints(runHints string) string {
	lines := []string{}
	for _, line := range strings.Split(runHints, "\n") {
		if strings.Contains(line, "test") || strings.Contains(line, "pytest") {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		return "- 未识别到明确测试命令。"
	}
	return strings.Join(lines, "\n")
}
