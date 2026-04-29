package chat

import (
	"context"
	"errors"
	"strings"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/persistence"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillresolver"
)

var ErrKnowledgeForwardUnavailable = errors.New("chat: knowledge forward unavailable")
var ErrSkillDefinitionNotFound = errors.New("chat: skill definition not found")
var ErrSkillInstallationNotFound = errors.New("chat: skill installation not found")
var ErrSkillDefinitionMismatch = errors.New("chat: skill definition mismatch")
var ErrSkillInstallationDisabled = errors.New("chat: skill installation disabled")
var ErrNoEnabledSkillInstallation = errors.New("chat: no enabled skill installation available for definition")

type PrepareStreamInput struct {
	UserID             string
	Message            string
	SessionID          string
	ConnectionIDs      []string
	RequestedChatModel string
	Skill              Skill
	SkillID            string
	SkillDefinitionID  string
	CustomPrompt       string
	UseKnowledge       bool
	EnableSearch       bool
	Temperature        *float64
}

type PreparedStream struct {
	Model   domain.UserChatModel
	Skill   Skill
	Request StreamConversationRequest
}

func (s *Service) PrepareStreamConversation(
	ctx context.Context,
	input PrepareStreamInput,
) (PreparedStream, error) {
	if s.prepareStore == nil {
		return PreparedStream{}, errors.New("chat preparation store unavailable")
	}
	if input.UseKnowledge && !s.CanForward(true) {
		return PreparedStream{}, ErrKnowledgeForwardUnavailable
	}
	if err := s.EnsureModelDefaults(ctx, input.UserID); err != nil {
		return PreparedStream{}, err
	}

	model, err := s.ResolveModel(ctx, input.UserID, input.UseKnowledge, strings.TrimSpace(input.RequestedChatModel))
	if err != nil {
		return PreparedStream{}, err
	}

	skill := input.Skill
	skillID := strings.TrimSpace(input.SkillID)
	customPrompt := strings.TrimSpace(input.CustomPrompt)
	var selectedSkill *domain.Skill

	if skillID != "" || strings.TrimSpace(input.SkillDefinitionID) != "" {
		resolvedSkillID, storedSkill, err := s.ResolveChatSelection(ctx, input.UserID, skillID, input.SkillDefinitionID)
		if err != nil {
			return PreparedStream{}, err
		}
		skillID = resolvedSkillID
		if mode := NormalizeSkill(storedSkill.Mode); mode != "" {
			skill = mode
		}
		if strings.TrimSpace(storedSkill.Prompt) != "" {
			customPrompt = strings.TrimSpace(storedSkill.Prompt)
		}
		selectedSkill = &storedSkill
	}

	request := StreamConversationRequest{
		UserID:        input.UserID,
		SessionID:     strings.TrimSpace(input.SessionID),
		Message:       strings.TrimSpace(input.Message),
		ConnectionIDs: append([]string(nil), input.ConnectionIDs...),
		Runtime:       model.RuntimeConfig(),
		SkillID:       skillID,
		Skill:         skill,
		CustomPrompt:  customPrompt,
		UseKnowledge:  input.UseKnowledge,
		EnableSearch:  input.EnableSearch,
		SelectedSkill: selectedSkill,
	}
	request.Runtime.EnableSearch = input.EnableSearch
	if input.Temperature != nil {
		request.Runtime.Temperature = *input.Temperature
		request.Runtime.TemperatureSet = true
	}

	return PreparedStream{
		Model:   model,
		Skill:   skill,
		Request: request,
	}, nil
}

func (s *Service) ResolveChatSelection(
	ctx context.Context,
	userID, installationID, definitionID string,
) (string, domain.Skill, error) {
	if s.prepareStore == nil {
		return "", domain.Skill{}, errors.New("chat preparation store unavailable")
	}
	record, err := skillresolver.ResolveInstallationRecord(ctx, s.prepareStore, userID, installationID, definitionID)
	if err != nil {
		return "", domain.Skill{}, mapChatSkillSelectionError(installationID, definitionID, err)
	}
	storedSkill, err := s.prepareStore.GetSkill(ctx, userID, record.Installation.ID)
	if err != nil {
		if errors.Is(err, persistence.ErrNotFound) {
			return "", domain.Skill{}, ErrSkillInstallationNotFound
		}
		return "", domain.Skill{}, err
	}
	return record.Installation.ID, storedSkill, nil
}

func mapChatSkillSelectionError(installationID, definitionID string, err error) error {
	definitionOnly := strings.TrimSpace(definitionID) != "" && strings.TrimSpace(installationID) == ""
	switch {
	case errors.Is(err, persistence.ErrNotFound) && definitionOnly:
		return ErrSkillDefinitionNotFound
	case errors.Is(err, persistence.ErrNotFound):
		return ErrSkillInstallationNotFound
	case errors.Is(err, skillresolver.ErrDefinitionMismatch):
		return ErrSkillDefinitionMismatch
	case errors.Is(err, skillresolver.ErrDisabled) && definitionOnly:
		return ErrNoEnabledSkillInstallation
	case errors.Is(err, skillresolver.ErrDisabled):
		return ErrSkillInstallationDisabled
	default:
		return err
	}
}
