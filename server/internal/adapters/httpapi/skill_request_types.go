package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	skillsvc "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skill"
)

type createSkillRequest struct {
	DefinitionID      string            `json:"definitionId"`
	CurrentRevisionID string            `json:"currentRevisionId"`
	Name              string            `json:"name"`
	IsDefault         *bool             `json:"isDefault"`
	Slug              string            `json:"slug"`
	Kind              string            `json:"kind"`
	Title             string            `json:"title"`
	Description       string            `json:"description"`
	Prompt            string            `json:"prompt"`
	Mode              string            `json:"mode"`
	Source            string            `json:"source"`
	Enabled           *bool             `json:"enabled"`
	RepoURL           string            `json:"repoUrl"`
	PlannerPolicy     map[string]string `json:"plannerPolicy"`
	ToolAllowlist     []string          `json:"toolAllowlist"`
}

type updateSkillRequest struct {
	CurrentRevisionID *string            `json:"currentRevisionId"`
	Name              *string            `json:"name"`
	IsDefault         *bool              `json:"isDefault"`
	Slug              *string            `json:"slug"`
	Kind              *string            `json:"kind"`
	Title             *string            `json:"title"`
	Description       *string            `json:"description"`
	Prompt            *string            `json:"prompt"`
	Mode              *string            `json:"mode"`
	Source            *string            `json:"source"`
	Enabled           *bool              `json:"enabled"`
	RepoURL           *string            `json:"repoUrl"`
	PlannerPolicy     *map[string]string `json:"plannerPolicy"`
	ToolAllowlist     *[]string          `json:"toolAllowlist"`
}

type importSkillGitHubRequest struct {
	RepoURL          string `json:"repoUrl"`
	Ref              string `json:"ref"`
	Path             string `json:"path"`
	Install          *bool  `json:"install"`
	InstallationName string `json:"installationName"`
	IsDefault        *bool  `json:"isDefault"`
	Enabled          *bool  `json:"enabled"`
}

func decodeCreateSkillRequest(r *http.Request) (createSkillRequest, error) {
	var req createSkillRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return createSkillRequest{}, errors.New("invalid request body")
	}
	return req, nil
}

func decodeUpdateSkillRequest(r *http.Request) (updateSkillRequest, error) {
	var req updateSkillRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return updateSkillRequest{}, errors.New("invalid request body")
	}
	return req, nil
}

func createSkillInputFromRequest(req createSkillRequest) skillsvc.CreateInput {
	return skillsvc.CreateInput{
		DefinitionID:      req.DefinitionID,
		CurrentRevisionID: req.CurrentRevisionID,
		Name:              req.Name,
		IsDefault:         req.IsDefault,
		Slug:              req.Slug,
		Kind:              req.Kind,
		Title:             req.Title,
		Description:       req.Description,
		Prompt:            req.Prompt,
		Mode:              req.Mode,
		Source:            req.Source,
		Enabled:           req.Enabled,
		RepoURL:           req.RepoURL,
		PlannerPolicy:     req.PlannerPolicy,
		ToolAllowlist:     req.ToolAllowlist,
	}
}

func updateSkillInputFromRequest(req updateSkillRequest) skillsvc.UpdateInput {
	return skillsvc.UpdateInput{
		CurrentRevisionID: req.CurrentRevisionID,
		Name:              req.Name,
		IsDefault:         req.IsDefault,
		Slug:              req.Slug,
		Kind:              req.Kind,
		Title:             req.Title,
		Description:       req.Description,
		Prompt:            req.Prompt,
		Mode:              req.Mode,
		Source:            req.Source,
		Enabled:           req.Enabled,
		RepoURL:           req.RepoURL,
		PlannerPolicy:     req.PlannerPolicy,
		ToolAllowlist:     req.ToolAllowlist,
	}
}

func parseBoolWithDefault(raw string, fallback bool) (bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	return strconv.ParseBool(raw)
}

func parseOptionalBool(raw string) (*bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return nil, err
	}
	return &value, nil
}
