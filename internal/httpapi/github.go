package httpapi

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"litetask/internal/github"
)

func githubAuthKey(secret []byte) []byte {
	sum := sha256.Sum256(secret)
	return sum[:]
}

// handleGitHubCallback handles the OAuth redirect from GitHub. Not behind requireUser —
// the user identity is embedded and verified in the state parameter.
func (s *Server) handleGitHubCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	stateParam := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	if stateParam == "" || code == "" {
		writeError(w, "missing state or code", http.StatusBadRequest)
		return
	}

	parts := strings.SplitN(stateParam, ".", 2)
	if len(parts) != 2 {
		writeError(w, "invalid state", http.StatusBadRequest)
		return
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || !verify(s.authSecret, string(payloadBytes), parts[1]) {
		writeError(w, "invalid state signature", http.StatusBadRequest)
		return
	}
	userID, err := strconv.ParseInt(string(payloadBytes), 10, 64)
	if err != nil || userID <= 0 {
		writeError(w, "invalid state payload", http.StatusBadRequest)
		return
	}

	token, err := s.exchangeGitHubCode(r.Context(), code)
	if err != nil {
		writeError(w, "failed to exchange code", http.StatusBadGateway)
		return
	}

	encrypted, err := github.EncryptToken(token, githubAuthKey(s.authSecret))
	if err != nil {
		writeError(w, "failed to encrypt token", http.StatusInternalServerError)
		return
	}

	if err := s.store.StoreGitHubToken(r.Context(), userID, encrypted); err != nil {
		writeError(w, "failed to store token", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/settings?github=connected", http.StatusFound)
}

// handleGitHub dispatches authenticated GitHub API endpoints.
func (s *Server) handleGitHub(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/github")
	parts := strings.Split(strings.Trim(path, "/"), "/")

	switch {
	case len(parts) == 1 && parts[0] == "auth" && r.Method == http.MethodGet:
		s.handleGitHubAuth(w, r)
	case len(parts) == 1 && parts[0] == "repos" && r.Method == http.MethodGet:
		s.handleGitHubRepos(w, r)
	case len(parts) == 1 && parts[0] == "integrations" && r.Method == http.MethodGet:
		s.handleListGitHubIntegrations(w, r)
	case len(parts) == 1 && parts[0] == "connect" && r.Method == http.MethodPost:
		s.handleGitHubConnect(w, r)
	case len(parts) == 2 && parts[0] == "integrations" && r.Method == http.MethodDelete:
		id, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			writeError(w, "invalid id", http.StatusBadRequest)
			return
		}
		s.handleGitHubDeleteIntegration(w, r, id)
	case len(parts) == 2 && parts[0] == "sync" && r.Method == http.MethodPost:
		id, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			writeError(w, "invalid id", http.StatusBadRequest)
			return
		}
		s.handleGitHubSyncIntegration(w, r, id)
	default:
		writeError(w, "not found", http.StatusNotFound)
	}
}

func (s *Server) handleGitHubAuth(w http.ResponseWriter, r *http.Request) {
	if s.githubClientID == "" {
		writeError(w, "github integration not configured", http.StatusNotImplemented)
		return
	}
	auth := getAuth(r)
	payload := strconv.FormatInt(auth.user.ID, 10)
	sig := sign(s.authSecret, payload)
	state := base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + sig
	redirectURL := fmt.Sprintf("https://github.com/login/oauth/authorize?client_id=%s&state=%s&scope=repo",
		url.QueryEscape(s.githubClientID), url.QueryEscape(state))
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (s *Server) handleGitHubRepos(w http.ResponseWriter, r *http.Request) {
	auth := getAuth(r)
	encToken, err := s.store.GetGitHubToken(r.Context(), auth.user.ID)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, "github not connected", http.StatusUnauthorized)
		return
	}
	if err != nil {
		writeError(w, "server error", http.StatusInternalServerError)
		return
	}
	token, err := github.DecryptToken(encToken, githubAuthKey(s.authSecret))
	if err != nil {
		writeError(w, "invalid token", http.StatusInternalServerError)
		return
	}
	repos, err := github.NewClient(token).ListRepos(r.Context())
	if err != nil {
		writeError(w, "failed to list repos", http.StatusBadGateway)
		return
	}
	if repos == nil {
		repos = []string{}
	}
	writeJSON(w, repos)
}

func (s *Server) handleListGitHubIntegrations(w http.ResponseWriter, r *http.Request) {
	auth := getAuth(r)
	igs, err := s.store.ListGitHubIntegrations(r.Context(), auth.user.ID)
	if err != nil {
		writeError(w, "server error", http.StatusInternalServerError)
		return
	}
	type integrationResponse struct {
		ID           int64   `json:"id"`
		RepoFullName string  `json:"repoFullName"`
		ProjectID    int64   `json:"projectId"`
		LastSyncedAt *string `json:"lastSyncedAt"`
	}
	result := make([]integrationResponse, len(igs))
	for i, ig := range igs {
		r := integrationResponse{
			ID:           ig.ID,
			RepoFullName: ig.RepoFullName,
			ProjectID:    ig.ProjectID,
		}
		if ig.LastSyncedAt != nil {
			s := ig.LastSyncedAt.Format("2006-01-02T15:04:05Z")
			r.LastSyncedAt = &s
		}
		result[i] = r
	}
	writeJSON(w, result)
}

func (s *Server) handleGitHubConnect(w http.ResponseWriter, r *http.Request) {
	auth := getAuth(r)
	var payload struct {
		RepoFullName string `json:"repoFullName"`
		ProjectID    int64  `json:"projectId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	payload.RepoFullName = strings.TrimSpace(payload.RepoFullName)
	if payload.RepoFullName == "" || payload.ProjectID == 0 {
		writeError(w, "repoFullName and projectId required", http.StatusBadRequest)
		return
	}
	if !strings.Contains(payload.RepoFullName, "/") {
		writeError(w, "invalid repoFullName", http.StatusBadRequest)
		return
	}

	encToken, err := s.store.GetGitHubToken(r.Context(), auth.user.ID)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, "github not connected", http.StatusUnauthorized)
		return
	}
	if err != nil {
		writeError(w, "server error", http.StatusInternalServerError)
		return
	}

	ig, err := s.store.CreateGitHubIntegration(r.Context(), auth.user.ID, payload.RepoFullName, payload.ProjectID, encToken)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			writeError(w, "integration already exists for this project", http.StatusBadRequest)
			return
		}
		writeError(w, "failed to create integration", http.StatusInternalServerError)
		return
	}

	type integrationResponse struct {
		ID           int64   `json:"id"`
		RepoFullName string  `json:"repoFullName"`
		ProjectID    int64   `json:"projectId"`
		LastSyncedAt *string `json:"lastSyncedAt"`
	}
	writeJSON(w, integrationResponse{
		ID:           ig.ID,
		RepoFullName: ig.RepoFullName,
		ProjectID:    ig.ProjectID,
	})
}

func (s *Server) handleGitHubDeleteIntegration(w http.ResponseWriter, r *http.Request, id int64) {
	auth := getAuth(r)
	ig, err := s.store.GetGitHubIntegrationByID(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, "integration not found", http.StatusNotFound)
		return
	}
	if err != nil {
		writeError(w, "server error", http.StatusInternalServerError)
		return
	}
	if ig.UserID != auth.user.ID {
		writeError(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := s.store.DeleteGitHubIntegration(r.Context(), id); err != nil {
		writeError(w, "failed to delete integration", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGitHubSyncIntegration(w http.ResponseWriter, r *http.Request, id int64) {
	auth := getAuth(r)
	ig, err := s.store.GetGitHubIntegrationByID(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, "integration not found", http.StatusNotFound)
		return
	}
	if err != nil {
		writeError(w, "server error", http.StatusInternalServerError)
		return
	}
	if ig.UserID != auth.user.ID {
		writeError(w, "forbidden", http.StatusForbidden)
		return
	}

	syncer := github.NewSyncer(s.store, githubAuthKey(s.authSecret))
	result, err := syncer.Sync(r.Context(), ig)
	if err != nil {
		writeError(w, "sync failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, result)
}

func (s *Server) exchangeGitHubCode(ctx context.Context, code string) (string, error) {
	body := fmt.Sprintf(`{"client_id":%q,"client_secret":%q,"code":%q}`,
		s.githubClientID, s.githubClientSecret, code)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://github.com/login/oauth/access_token", strings.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close() //nolint:errcheck
	var result struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.Error != "" {
		return "", fmt.Errorf("github oauth: %s", result.Error)
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("github oauth: empty access token")
	}
	return result.AccessToken, nil
}
