package skillimport

import (
	"archive/zip"
	"bytes"
	"testing"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

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
