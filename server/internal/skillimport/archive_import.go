package skillimport

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

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
		strings.TrimSuffix(filepathBase(fileName), filepathExt(fileName)),
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
		raw, err := readArchiveFile(item.File, s.maxArchiveBytes)
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

// Small wrappers keep archive_import.go free from direct filepath imports.
func filepathBase(name string) string { return pathBase(name) }
func filepathExt(name string) string  { return pathExt(name) }

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
