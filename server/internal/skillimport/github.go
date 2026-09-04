package skillimport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

var githubPathSegmentPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

func (s *Service) downloadArchive(ctx context.Context, archiveURL string) ([]byte, string, error) {
	parsedArchiveURL, err := url.Parse(archiveURL)
	if err != nil || !isAllowedGitHubDownloadURL(parsedArchiveURL) {
		return nil, "", ErrUnsupportedRepoURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, archiveURL, nil)
	if err != nil {
		return nil, "", err
	}
	client := *s.client
	previousCheckRedirect := client.CheckRedirect
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if !isAllowedGitHubDownloadURL(req.URL) {
			return ErrUnsupportedRepoURL
		}
		if previousCheckRedirect != nil {
			return previousCheckRedirect(req, via)
		}
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}
	resp, err := client.Do(req)
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
	if err != nil || !isCanonicalGitHubRepositoryURL(parsed) {
		return "", nil, "", ErrUnsupportedRepoURL
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 2 {
		return "", nil, "", ErrUnsupportedRepoURL
	}
	owner := strings.TrimSpace(parts[0])
	repo := strings.TrimSuffix(strings.TrimSpace(parts[1]), ".git")
	if !githubPathSegmentPattern.MatchString(owner) || !githubPathSegmentPattern.MatchString(repo) || owner == "." || owner == ".." || repo == "." || repo == ".." {
		return "", nil, "", ErrUnsupportedRepoURL
	}
	baseRepoURL := fmt.Sprintf("https://github.com/%s/%s", owner, repo)
	refs := []string{}
	if trimmedRef := strings.TrimSpace(ref); trimmedRef != "" {
		refs = append(refs, trimmedRef)
	} else {
		refs = append(refs, "main", "master")
	}
	candidates := make([]string, 0, len(refs))
	for _, item := range refs {
		if item == "." || item == ".." || strings.ContainsAny(item, "\r\n?#") {
			return "", nil, "", ErrUnsupportedRepoURL
		}
		candidates = append(candidates, fmt.Sprintf("%s/archive/refs/heads/%s.zip", baseRepoURL, item))
	}
	return baseRepoURL, candidates, repo + ".zip", nil
}

func isCanonicalGitHubRepositoryURL(parsed *url.URL) bool {
	return parsed != nil && parsed.Scheme == "https" && strings.EqualFold(parsed.Hostname(), "github.com") && parsed.Port() == "" && parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}

func isAllowedGitHubDownloadURL(parsed *url.URL) bool {
	if parsed == nil || parsed.Scheme != "https" || parsed.Port() != "" || parsed.User != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "github.com" || host == "codeload.github.com"
}
