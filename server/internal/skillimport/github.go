package skillimport

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

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
