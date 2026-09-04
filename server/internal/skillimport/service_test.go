package skillimport

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

type importRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn importRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }

func TestBuildGitHubArchiveCandidatesRejectsUnsafeURLs(t *testing.T) {
	for _, raw := range []string{
		"http://github.com/acme/repo",
		"http://127.0.0.1/acme/repo",
		"https://github.com.evil/acme/repo",
		"https://user@github.com/acme/repo",
		"https://github.com:8443/acme/repo",
		"https://github.com/acme/repo/extra",
	} {
		t.Run(raw, func(t *testing.T) {
			if _, _, _, err := buildGitHubArchiveCandidates(raw, "main"); !errors.Is(err, ErrUnsupportedRepoURL) {
				t.Fatalf("expected unsafe URL rejection, got %v", err)
			}
		})
	}
}

func TestGitHubImportRejectsUnsafeRedirect(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: importRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return &http.Response{
				StatusCode: http.StatusFound,
				Status:     "302 Found",
				Header:     http.Header{"Location": []string{"http://127.0.0.1/private"}},
				Body:       io.NopCloser(strings.NewReader("")),
				Request:    req,
			}, nil
		}
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(buildTestArchive(t, map[string]string{"repo-main/SKILL.md": "prompt"}))), Request: req}, nil
	})}

	_, err := NewService(client).ImportGitHubArchive(context.Background(), GitHubImportRequest{RepoURL: "https://github.com/acme/repo", Ref: "main"})
	if !errors.Is(err, ErrUnsupportedRepoURL) || calls != 1 {
		t.Fatalf("expected redirect rejection after one request, calls=%d err=%v", calls, err)
	}
}

func TestImportUploadedArchiveEnforcesCumulativeExpansionLimit(t *testing.T) {
	archive := buildTestArchive(t, map[string]string{
		"repo/SKILL.md": "prompt" + strings.Repeat("a", 600),
		"repo/data.txt": strings.Repeat("b", 600),
	})
	service := NewService(nil)
	service.maxArchiveBytes = 1024
	if _, err := service.ImportUploadedArchive("large.zip", "application/zip", "", archive); !errors.Is(err, ErrArchiveTooLarge) {
		t.Fatalf("expected cumulative expansion rejection, got %v", err)
	}
}

func TestImportUploadedArchiveEnforcesCompressionRatioLimit(t *testing.T) {
	archive := buildTestArchive(t, map[string]string{
		"repo/SKILL.md": "prompt",
		"repo/data.txt": strings.Repeat("a", 1<<20),
	})
	if _, err := NewService(nil).ImportUploadedArchive("ratio.zip", "application/zip", "", archive); !errors.Is(err, ErrArchiveTooLarge) {
		t.Fatalf("expected compression ratio rejection, got %v", err)
	}
}

func TestImportUploadedArchiveEnforcesMemberLimit(t *testing.T) {
	files := map[string]string{"repo/SKILL.md": "prompt"}
	for i := 0; i < 1100; i++ {
		files["repo/files/"+strings.Repeat("x", i%8)+string(rune('a'+i%26))+fmt.Sprint(i)] = "x"
	}
	archive := buildTestArchive(t, files)
	if _, err := NewService(nil).ImportUploadedArchive("many.zip", "application/zip", "", archive); !errors.Is(err, ErrArchiveTooLarge) {
		t.Fatalf("expected member count rejection, got %v", err)
	}
}

func TestImportUploadedArchiveParsesSkillPackage(t *testing.T) {
	archive := buildTestArchive(t, map[string]string{
		"demo-skill-main/skill.yaml": "slug: demo-skill\nkind: agent_workflow\ntitle: Demo Skill\ndescription: Demo import\nmode: summary\nplannerPolicy:\n  preferred_mode: kb_only\ntoolAllowlist:\n  - report.write\n",
		"demo-skill-main/SKILL.md":   "Follow the packaged workflow strictly.",
	})

	service := NewService(nil)
	pkg, err := service.ImportUploadedArchive("demo-skill.zip", "application/zip", "", archive)
	if err != nil {
		t.Fatalf("import uploaded archive: %v", err)
	}

	if pkg.Source != domain.SkillSourceUpload {
		t.Fatalf("expected upload source, got %s", pkg.Source)
	}
	if pkg.Slug != "demo-skill" {
		t.Fatalf("expected slug demo-skill, got %s", pkg.Slug)
	}
	if pkg.Kind != domain.SkillKindAgentWorkflow {
		t.Fatalf("expected agent_workflow kind, got %s", pkg.Kind)
	}
	if pkg.Title != "Demo Skill" {
		t.Fatalf("expected title Demo Skill, got %s", pkg.Title)
	}
	if pkg.Mode != "summary" {
		t.Fatalf("expected summary mode, got %s", pkg.Mode)
	}
	if pkg.Prompt != "Follow the packaged workflow strictly." {
		t.Fatalf("expected prompt from SKILL.md, got %q", pkg.Prompt)
	}
	if pkg.PlannerPolicy["preferred_mode"] != "kb_only" {
		t.Fatalf("expected preferred_mode kb_only, got %#v", pkg.PlannerPolicy)
	}
	if len(pkg.ToolAllowlist) != 1 || pkg.ToolAllowlist[0] != "report.write" {
		t.Fatalf("expected tool allowlist [report.write], got %#v", pkg.ToolAllowlist)
	}
	if pkg.ManifestPath != "skill.yaml" {
		t.Fatalf("expected manifest path skill.yaml, got %s", pkg.ManifestPath)
	}
	if pkg.InstructionsPath != "SKILL.md" {
		t.Fatalf("expected instructions path SKILL.md, got %s", pkg.InstructionsPath)
	}
}

func buildTestArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for name, content := range files {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := file.Write([]byte(content)); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	return buf.Bytes()
}
