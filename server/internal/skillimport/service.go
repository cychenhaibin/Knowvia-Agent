package skillimport

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

const defaultMaxArchiveBytes int64 = 10 << 20

var (
	ErrUnsupportedRepoURL    = errors.New("unsupported github repository url")
	ErrArchiveTooLarge       = errors.New("skill archive exceeds size limit")
	ErrInvalidArchive        = errors.New("invalid skill archive")
	ErrSkillPackageNotFound  = errors.New("skill package must contain skill.json/skill.yaml or SKILL.md")
	ErrInvalidManifest       = errors.New("invalid skill manifest")
	ErrIncompleteSkillPrompt = errors.New("skill package must provide prompt instructions")
)

type Service struct {
	client          *http.Client
	maxArchiveBytes int64
}

type GitHubImportRequest struct {
	RepoURL string
	Ref     string
	Path    string
}

type ParsedPackage struct {
	Source           domain.SkillSource
	RepoURL          string
	SourceURL        string
	FileName         string
	MediaType        string
	EntryPath        string
	ManifestPath     string
	InstructionsPath string
	ArchiveBytes     []byte
	SHA256           string
	SizeBytes        int64
	Slug             string
	Kind             domain.SkillKind
	Title            string
	Description      string
	Prompt           string
	Mode             string
	PlannerPolicy    map[string]string
	ToolAllowlist    []string
	Files            []ParsedPackageFile
}

type ParsedPackageFile struct {
	Path           string
	MediaType      string
	SizeBytes      int64
	SHA256         string
	IsManifest     bool
	IsInstructions bool
}

type manifestDocument struct {
	Slug          string
	Kind          string
	Title         string
	Description   string
	Prompt        string
	Mode          string
	PlannerPolicy map[string]string
	ToolAllowlist []string
}

type archiveFile struct {
	RawName string
	RelName string
	File    *zip.File
}

func NewService(client *http.Client) *Service {
	if client == nil {
		client = &http.Client{Timeout: 45 * 1e9}
	}
	return &Service{
		client:          client,
		maxArchiveBytes: defaultMaxArchiveBytes,
	}
}

func (s *Service) ImportGitHubArchive(ctx context.Context, req GitHubImportRequest) (ParsedPackage, error) {
	repoURL, archiveCandidates, fileName, err := buildGitHubArchiveCandidates(req.RepoURL, req.Ref)
	if err != nil {
		return ParsedPackage{}, err
	}
	var lastErr error
	for _, archiveURL := range archiveCandidates {
		raw, mediaType, err := s.downloadArchive(ctx, archiveURL)
		if err != nil {
			lastErr = err
			continue
		}
		pkg, err := s.ImportUploadedArchive(fileName, mediaType, req.Path, raw)
		if err != nil {
			return ParsedPackage{}, err
		}
		pkg.Source = domain.SkillSourceGithub
		pkg.RepoURL = repoURL
		pkg.SourceURL = archiveURL
		return pkg, nil
	}
	if lastErr == nil {
		lastErr = ErrUnsupportedRepoURL
	}
	return ParsedPackage{}, lastErr
}

func (s *Service) ImportUploadedArchive(fileName, mediaType, skillPath string, archiveBytes []byte) (ParsedPackage, error) {
	if len(archiveBytes) == 0 {
		return ParsedPackage{}, ErrInvalidArchive
	}
	if int64(len(archiveBytes)) > s.maxArchiveBytes {
		return ParsedPackage{}, ErrArchiveTooLarge
	}
	reader, err := zip.NewReader(bytes.NewReader(archiveBytes), int64(len(archiveBytes)))
	if err != nil {
		return ParsedPackage{}, fmt.Errorf("%w: %v", ErrInvalidArchive, err)
	}

	files := collectArchiveFiles(reader.File)
	parsedFiles, fileContents, err := s.buildParsedFiles(files)
	if err != nil {
		return ParsedPackage{}, err
	}
	manifestFile, instructionsFile := locateSkillFiles(files, skillPath)
	if manifestFile == nil && instructionsFile == nil {
		return ParsedPackage{}, ErrSkillPackageNotFound
	}

	var manifest manifestDocument
	if manifestFile != nil {
		manifestBytes := fileContents[manifestFile.RelName]
		manifest, err = parseManifest(manifestFile.RelName, manifestBytes)
		if err != nil {
			return ParsedPackage{}, err
		}
	}

	instructions := ""
	if instructionsFile != nil {
		instructions = strings.TrimSpace(string(fileContents[instructionsFile.RelName]))
	}

	prompt := strings.TrimSpace(instructions)
	if prompt == "" {
		prompt = strings.TrimSpace(manifest.Prompt)
	}
	if prompt == "" {
		return ParsedPackage{}, ErrIncompleteSkillPrompt
	}

	entryPath := deriveEntryPath(manifestFile, instructionsFile)
	title := firstNonEmpty(
		strings.TrimSpace(manifest.Title),
		friendlyNameFromEntry(entryPath),
		strings.TrimSuffix(filepath.Base(fileName), filepath.Ext(fileName)),
		"Imported Skill",
	)
	slug := normalizeSlug(firstNonEmpty(strings.TrimSpace(manifest.Slug), title))
	if slug == "" {
		return ParsedPackage{}, fmt.Errorf("%w: empty skill slug", ErrInvalidManifest)
	}

	kind := domain.SkillKindChatProfile
	if parsedKind := normalizeKind(manifest.Kind); parsedKind != "" {
		kind = parsedKind
	}
	mode := normalizeMode(manifest.Mode)
	if mode == "" {
		mode = "answer"
	}
	sum := sha256.Sum256(archiveBytes)

	return ParsedPackage{
		Source:           domain.SkillSourceUpload,
		FileName:         fileName,
		MediaType:        mediaType,
		EntryPath:        entryPath,
		ManifestPath:     relName(manifestFile),
		InstructionsPath: relName(instructionsFile),
		ArchiveBytes:     append([]byte(nil), archiveBytes...),
		SHA256:           hex.EncodeToString(sum[:]),
		SizeBytes:        int64(len(archiveBytes)),
		Slug:             slug,
		Kind:             kind,
		Title:            title,
		Description:      firstNonEmpty(strings.TrimSpace(manifest.Description), title),
		Prompt:           prompt,
		Mode:             mode,
		PlannerPolicy:    cloneStringMap(manifest.PlannerPolicy),
		ToolAllowlist:    append([]string(nil), manifest.ToolAllowlist...),
		Files:            markCoreFiles(parsedFiles, relName(manifestFile), relName(instructionsFile)),
	}, nil
}

func (s *Service) buildParsedFiles(files []archiveFile) ([]ParsedPackageFile, map[string][]byte, error) {
	items := make([]ParsedPackageFile, 0, len(files))
	contents := make(map[string][]byte, len(files))
	for _, item := range files {
		raw, err := readZipFile(item.File, s.maxArchiveBytes)
		if err != nil {
			return nil, nil, err
		}
		sum := sha256.Sum256(raw)
		items = append(items, ParsedPackageFile{
			Path:      item.RelName,
			MediaType: detectMediaType(item.RelName),
			SizeBytes: int64(len(raw)),
			SHA256:    hex.EncodeToString(sum[:]),
		})
		contents[item.RelName] = raw
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Path < items[j].Path
	})
	return items, contents, nil
}

func (s *Service) downloadArchive(ctx context.Context, archiveURL string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, archiveURL, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("github archive download failed: %s", resp.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, s.maxArchiveBytes+1))
	if err != nil {
		return nil, "", err
	}
	if int64(len(raw)) > s.maxArchiveBytes {
		return nil, "", ErrArchiveTooLarge
	}
	return raw, resp.Header.Get("Content-Type"), nil
}

func buildGitHubArchiveCandidates(repoURL, ref string) (string, []string, string, error) {
	parsed, err := url.Parse(strings.TrimSpace(repoURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", nil, "", ErrUnsupportedRepoURL
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 {
		return "", nil, "", ErrUnsupportedRepoURL
	}
	owner := strings.TrimSpace(parts[0])
	repo := strings.TrimSuffix(strings.TrimSpace(parts[1]), ".git")
	if owner == "" || repo == "" {
		return "", nil, "", ErrUnsupportedRepoURL
	}
	baseRepoURL := fmt.Sprintf("%s://%s/%s/%s", parsed.Scheme, parsed.Host, owner, repo)
	refs := []string{}
	if trimmedRef := strings.TrimSpace(ref); trimmedRef != "" {
		refs = append(refs, trimmedRef)
	} else {
		refs = append(refs, "main", "master")
	}
	candidates := make([]string, 0, len(refs))
	for _, item := range refs {
		candidates = append(candidates, fmt.Sprintf("%s/archive/refs/heads/%s.zip", baseRepoURL, item))
	}
	return baseRepoURL, candidates, repo + ".zip", nil
}

func collectArchiveFiles(files []*zip.File) []archiveFile {
	items := make([]archiveFile, 0, len(files))
	for _, file := range files {
		if file.FileInfo().IsDir() {
			continue
		}
		rel := stripArchiveRoot(file.Name)
		if rel == "" {
			continue
		}
		items = append(items, archiveFile{
			RawName: file.Name,
			RelName: rel,
			File:    file,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if len(items[i].RelName) == len(items[j].RelName) {
			return items[i].RelName < items[j].RelName
		}
		return len(items[i].RelName) < len(items[j].RelName)
	})
	return items
}

func locateSkillFiles(files []archiveFile, skillPath string) (*archiveFile, *archiveFile) {
	normalizedPath := strings.Trim(strings.TrimSpace(skillPath), "/")
	manifestNames := map[string]bool{
		"skill.json": true,
		"skill.yaml": true,
		"skill.yml":  true,
	}
	var manifestFile *archiveFile
	var instructionsFile *archiveFile

	for i := range files {
		item := files[i]
		rel := item.RelName
		if normalizedPath != "" {
			if rel != normalizedPath && !strings.HasPrefix(rel, normalizedPath+"/") {
				continue
			}
		}
		base := strings.ToLower(path.Base(rel))
		switch {
		case manifestNames[base] && manifestFile == nil:
			copyItem := item
			manifestFile = &copyItem
		case base == "skill.md" && instructionsFile == nil:
			copyItem := item
			instructionsFile = &copyItem
		}
		if manifestFile != nil && instructionsFile != nil {
			return manifestFile, instructionsFile
		}
	}
	return manifestFile, instructionsFile
}

func parseManifest(fileName string, raw []byte) (manifestDocument, error) {
	switch strings.ToLower(filepath.Ext(fileName)) {
	case ".json":
		var doc manifestDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			return manifestDocument{}, fmt.Errorf("%w: %v", ErrInvalidManifest, err)
		}
		return doc, nil
	case ".yaml", ".yml":
		return parseSimpleYAMLManifest(raw)
	default:
		return manifestDocument{}, fmt.Errorf("%w: unsupported manifest file %s", ErrInvalidManifest, fileName)
	}
}

func parseSimpleYAMLManifest(raw []byte) (manifestDocument, error) {
	lines := strings.Split(string(raw), "\n")
	doc := manifestDocument{
		PlannerPolicy: map[string]string{},
		ToolAllowlist: []string{},
	}
	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasSuffix(trimmed, ":") {
			key := strings.TrimSuffix(trimmed, ":")
			switch normalizeYAMLKey(key) {
			case "plannerpolicy":
				for i+1 < len(lines) {
					next := strings.TrimRight(lines[i+1], "\r")
					if strings.TrimSpace(next) == "" {
						i++
						continue
					}
					if leadingIndent(next) <= leadingIndent(line) {
						break
					}
					part := strings.TrimSpace(next)
					kv := strings.SplitN(part, ":", 2)
					if len(kv) == 2 {
						doc.PlannerPolicy[strings.TrimSpace(kv[0])] = unquoteYAMLValue(kv[1])
					}
					i++
				}
			case "toolallowlist":
				for i+1 < len(lines) {
					next := strings.TrimRight(lines[i+1], "\r")
					if strings.TrimSpace(next) == "" {
						i++
						continue
					}
					if leadingIndent(next) <= leadingIndent(line) {
						break
					}
					part := strings.TrimSpace(next)
					if strings.HasPrefix(part, "-") {
						value := strings.TrimSpace(strings.TrimPrefix(part, "-"))
						if value != "" {
							doc.ToolAllowlist = append(doc.ToolAllowlist, unquoteYAMLValue(value))
						}
					}
					i++
				}
			}
			continue
		}
		kv := strings.SplitN(trimmed, ":", 2)
		if len(kv) != 2 {
			continue
		}
		key := normalizeYAMLKey(kv[0])
		value := unquoteYAMLValue(kv[1])
		switch key {
		case "slug":
			doc.Slug = value
		case "kind":
			doc.Kind = value
		case "title":
			doc.Title = value
		case "description":
			doc.Description = value
		case "prompt":
			doc.Prompt = value
		case "mode":
			doc.Mode = value
		}
	}
	return doc, nil
}

func normalizeYAMLKey(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	key = strings.ReplaceAll(key, "_", "")
	key = strings.ReplaceAll(key, "-", "")
	return key
}

func unquoteYAMLValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"'`)
	return strings.TrimSpace(value)
}

func readZipFile(file *zip.File, maxBytes int64) ([]byte, error) {
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	raw, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > maxBytes {
		return nil, ErrArchiveTooLarge
	}
	return raw, nil
}

func stripArchiveRoot(fileName string) string {
	cleaned := strings.Trim(strings.TrimSpace(fileName), "/")
	if cleaned == "" {
		return ""
	}
	parts := strings.Split(cleaned, "/")
	if len(parts) <= 1 {
		return cleaned
	}
	return strings.Join(parts[1:], "/")
}

func deriveEntryPath(manifestFile, instructionsFile *archiveFile) string {
	switch {
	case manifestFile != nil:
		return path.Dir(manifestFile.RelName)
	case instructionsFile != nil:
		return path.Dir(instructionsFile.RelName)
	default:
		return ""
	}
}

func friendlyNameFromEntry(entryPath string) string {
	entryPath = strings.Trim(entryPath, "/.")
	if entryPath == "" {
		return ""
	}
	name := filepath.Base(entryPath)
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")
	return strings.TrimSpace(strings.Title(name))
}

func normalizeKind(raw string) domain.SkillKind {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", string(domain.SkillKindChatProfile):
		return domain.SkillKindChatProfile
	case string(domain.SkillKindAgentWorkflow):
		return domain.SkillKindAgentWorkflow
	default:
		return ""
	}
}

func normalizeMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "answer":
		return "answer"
	case "summary":
		return "summary"
	case "actions":
		return "actions"
	default:
		return ""
	}
}

func normalizeSlug(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return ""
	}
	var builder strings.Builder
	lastDash := false
	for _, r := range raw {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			builder.WriteRune(r)
			lastDash = false
		case r == '-' || r == '_' || unicode.IsSpace(r) || r == '/':
			if !lastDash && builder.Len() > 0 {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(builder.String(), "-")
}

func relName(item *archiveFile) string {
	if item == nil {
		return ""
	}
	return item.RelName
}

func markCoreFiles(files []ParsedPackageFile, manifestPath, instructionsPath string) []ParsedPackageFile {
	items := make([]ParsedPackageFile, 0, len(files))
	for _, item := range files {
		item.IsManifest = item.Path == manifestPath && manifestPath != ""
		item.IsInstructions = item.Path == instructionsPath && instructionsPath != ""
		items = append(items, item)
	}
	return items
}

func detectMediaType(filePath string) string {
	switch strings.ToLower(filepath.Ext(filePath)) {
	case ".md":
		return "text/markdown"
	case ".json":
		return "application/json"
	case ".yaml", ".yml":
		return "application/yaml"
	case ".txt":
		return "text/plain"
	case ".js":
		return "application/javascript"
	case ".ts":
		return "application/typescript"
	case ".sh":
		return "application/x-sh"
	default:
		return "application/octet-stream"
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func leadingIndent(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}

func cloneStringMap(source map[string]string) map[string]string {
	if len(source) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}
