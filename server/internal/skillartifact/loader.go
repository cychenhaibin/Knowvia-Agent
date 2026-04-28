package skillartifact

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"path"
	"strings"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/store"
)

var (
	ErrArtifactFilePathRequired = errors.New("skill artifact file path is required")
	ErrArtifactFileNotFound     = errors.New("skill artifact file not found")
)

func LoadFile(ctx context.Context, repo store.Store, userID, artifactID, filePath string) (domain.SkillArtifactFile, []byte, error) {
	normalizedPath := normalizePath(filePath)
	if normalizedPath == "" {
		return domain.SkillArtifactFile{}, nil, ErrArtifactFilePathRequired
	}

	artifact, err := repo.GetSkillArtifact(ctx, userID, artifactID)
	if err != nil {
		return domain.SkillArtifactFile{}, nil, err
	}
	files, err := repo.ListSkillArtifactFiles(ctx, userID, artifactID)
	if err != nil {
		return domain.SkillArtifactFile{}, nil, err
	}

	var metadata *domain.SkillArtifactFile
	for i := range files {
		if normalizePath(files[i].Path) == normalizedPath {
			metadata = &files[i]
			break
		}
	}
	if metadata == nil {
		return domain.SkillArtifactFile{}, nil, ErrArtifactFileNotFound
	}

	reader, err := zip.NewReader(bytes.NewReader(artifact.ArchiveBytes), int64(len(artifact.ArchiveBytes)))
	if err != nil {
		return domain.SkillArtifactFile{}, nil, err
	}
	for _, item := range reader.File {
		if item.FileInfo().IsDir() {
			continue
		}
		if normalizeArchiveEntryPath(item.Name) != normalizedPath {
			continue
		}
		raw, err := readArchiveEntry(item)
		if err != nil {
			return domain.SkillArtifactFile{}, nil, err
		}
		return *metadata, raw, nil
	}
	return domain.SkillArtifactFile{}, nil, ErrArtifactFileNotFound
}

func normalizeArchiveEntryPath(filePath string) string {
	normalized := normalizePath(filePath)
	if normalized == "" {
		return ""
	}
	parts := strings.Split(normalized, "/")
	if len(parts) <= 1 {
		return normalized
	}
	return strings.Join(parts[1:], "/")
}

func normalizePath(filePath string) string {
	filePath = strings.ReplaceAll(strings.TrimSpace(filePath), "\\", "/")
	if filePath == "" {
		return ""
	}
	cleaned := strings.TrimPrefix(path.Clean("/"+filePath), "/")
	if cleaned == "." {
		return ""
	}
	return cleaned
}

func readArchiveEntry(item *zip.File) ([]byte, error) {
	reader, err := item.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}
