package store

import (
	"context"
)

func (s *MemoryStore) DeleteSkill(_ context.Context, userID, skillID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	installation, ok := s.skillInstallations[skillID]
	if !ok || installation.UserID != userID {
		return ErrNotFound
	}
	delete(s.skillInstallations, skillID)
	if installation.IsDefault {
		s.promoteDefaultSkillInstallation(userID, installation.DefinitionID, "")
	}
	return nil
}
