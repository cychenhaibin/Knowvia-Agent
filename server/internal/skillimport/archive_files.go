package skillimport

import (
	"archive/zip"
	"io"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

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

func readArchiveFile(file archiveFileRef, maxBytes int64) ([]byte, error) {
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

func pathBase(name string) string { return filepath.Base(name) }
func pathExt(name string) string  { return filepath.Ext(name) }
