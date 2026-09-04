package skillimport

import (
	"errors"
	"io"
	"net/http"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

const (
	defaultMaxArchiveBytes     int64 = 10 << 20
	defaultMaxArchiveFiles           = 1024
	defaultMaxCompressionRatio       = 100
)

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
	File    archiveFileRef
}

type archiveFileRef interface {
	Open() (io.ReadCloser, error)
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
