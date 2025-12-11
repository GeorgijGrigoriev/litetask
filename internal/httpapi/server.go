package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"litetask/internal/store"

	"golang.org/x/crypto/bcrypt"
)

const authExpiry = 30 * 24 * time.Hour

type ctxKey string

const ctxUser ctxKey = "user"

type authUser struct {
	user         store.User
	allowed      map[int64]struct{}
	isRestricted bool
}

type Server struct {
	store             *store.Store
	authSecret        []byte
	allowRegistration bool
	staticDir         string
}

func New(s *store.Store, secret []byte, allowRegistration bool, staticDir string) *Server {
	return &Server{
		store:             s,
		authSecret:        secret,
		allowRegistration: allowRegistration,
		staticDir:         staticDir,
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/api/auth/", s.cors(http.HandlerFunc(s.handleAuthRoutes)))
	mux.Handle("/api/tasks", s.cors(s.requireUser(http.HandlerFunc(s.handleTasks))))
	mux.Handle("/api/tasks/", s.cors(s.requireUser(http.HandlerFunc(s.handleTaskActions))))
	mux.Handle("/api/projects", s.cors(s.requireUser(http.HandlerFunc(s.handleProjects))))
	mux.Handle("/api/projects/", s.cors(s.requireUser(http.HandlerFunc(s.handleProjectActions))))
	mux.Handle("/api/users", s.cors(s.requireAdmin(http.HandlerFunc(s.handleUsers))))
	mux.Handle("/api/users/", s.cors(s.requireAdmin(http.HandlerFunc(s.handleUserActions))))
	mux.Handle("/", s.staticHandler())
	return mux
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, err := s.authenticate(r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if u.Role == "blocked" {
			http.Error(w, "account blocked", http.StatusForbidden)
			return
		}
		auth := authUser{user: u}
		if u.Role != "admin" {
			allowed, err := s.store.GetUserProjects(u.ID)
			if err != nil {
				http.Error(w, "server error", http.StatusInternalServerError)
				return
			}
			auth.isRestricted = true
			auth.allowed = make(map[int64]struct{}, len(allowed))
			for _, pid := range allowed {
				auth.allowed[pid] = struct{}{}
			}
		}
		ctx := context.WithValue(r.Context(), ctxUser, auth)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, err := s.authenticate(r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if u.Role != "admin" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		ctx := context.WithValue(r.Context(), ctxUser, authUser{user: u})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listTasks(w, r)
	case http.MethodPost:
		s.createTask(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleTaskActions(w http.ResponseWriter, r *http.Request) {
	trimmed := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	parts := strings.Split(strings.TrimSuffix(trimmed, "/"), "/")
	if len(parts) < 1 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}

	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		http.Error(w, "invalid task id", http.StatusBadRequest)
		return
	}

	if len(parts) == 2 && parts[1] == "status" {
		if r.Method != http.MethodPatch {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.updateStatus(w, r, id)
		return
	}

	if len(parts) == 1 && r.Method == http.MethodPatch {
		s.updateComment(w, r, id)
		return
	}

	if len(parts) == 1 && r.Method == http.MethodDelete {
		s.deleteTaskHandler(w, r, id)
		return
	}

	http.NotFound(w, r)
}

func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listProjects(w, r)
	case http.MethodPost:
		s.createProjectHandler(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleAuthRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/auth")
	switch {
	case strings.HasPrefix(path, "/login") && r.Method == http.MethodPost:
		s.handleLogin(w, r)
	case strings.HasPrefix(path, "/register") && r.Method == http.MethodPost:
		s.handleRegister(w, r)
	case strings.HasPrefix(path, "/me") && r.Method == http.MethodGet:
		s.handleMe(w, r)
	case strings.HasPrefix(path, "/logout") && r.Method == http.MethodPost:
		s.handleLogout(w, r)
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		users, err := s.store.ListUsers()
		if err != nil {
			http.Error(w, "failed to load users", http.StatusInternalServerError)
			return
		}
		trimmed := make([]struct {
			ID         int64   `json:"id"`
			Email      string  `json:"email"`
			Role       string  `json:"role"`
			ProjectIDs []int64 `json:"projectIds"`
		}, len(users))
		for i, u := range users {
			projects, _ := s.store.GetUserProjects(u.ID)
			trimmed[i] = struct {
				ID         int64   `json:"id"`
				Email      string  `json:"email"`
				Role       string  `json:"role"`
				ProjectIDs []int64 `json:"projectIds"`
			}{ID: u.ID, Email: u.Email, Role: u.Role, ProjectIDs: projects}
		}
		writeJSON(w, trimmed)
	case http.MethodPost:
		var payload struct {
			Email    string `json:"email"`
			Password string `json:"password"`
			Role     string `json:"role"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		payload.Email = strings.ToLower(strings.TrimSpace(payload.Email))
		payload.Password = strings.TrimSpace(payload.Password)
		payload.Role = strings.TrimSpace(strings.ToLower(payload.Role))
		if payload.Email == "" || payload.Password == "" {
			http.Error(w, "email and password required", http.StatusBadRequest)
			return
		}
		if len(payload.Password) < 6 {
			http.Error(w, "password too short", http.StatusBadRequest)
			return
		}
		if payload.Role == "" {
			payload.Role = "user"
		}
		if payload.Role != "user" && payload.Role != "admin" && payload.Role != "blocked" {
			http.Error(w, "invalid role", http.StatusBadRequest)
			return
		}
		u, err := s.store.CreateUser(payload.Email, payload.Password, payload.Role)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "unique") {
				http.Error(w, "email already registered", http.StatusBadRequest)
				return
			}
			http.Error(w, "failed to create user", http.StatusInternalServerError)
			return
		}
		projects, _ := s.store.GetUserProjects(u.ID)
		writeJSON(w, struct {
			ID         int64   `json:"id"`
			Email      string  `json:"email"`
			Role       string  `json:"role"`
			ProjectIDs []int64 `json:"projectIds"`
		}{ID: u.ID, Email: u.Email, Role: u.Role, ProjectIDs: projects})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleUserActions(w http.ResponseWriter, r *http.Request) {
	idStr := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/users/"), "/")
	if idStr == "" {
		http.NotFound(w, r)
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid user id", http.StatusBadRequest)
		return
	}
	if r.Method != http.MethodPatch {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var payload struct {
		Role       string  `json:"role"`
		Password   string  `json:"password"`
		ProjectIDs []int64 `json:"projectIds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	payload.Role = strings.TrimSpace(strings.ToLower(payload.Role))
	password := strings.TrimSpace(payload.Password)
	if payload.Role == "" && password == "" && payload.ProjectIDs == nil {
		http.Error(w, "nothing to update", http.StatusBadRequest)
		return
	}

	var updated store.User
	if payload.Role != "" {
		updated, err = s.store.UpdateUserRole(id, payload.Role)
		if errors.Is(err, store.ErrInvalidRole) {
			http.Error(w, "invalid role", http.StatusBadRequest)
			return
		}
		if errors.Is(err, store.ErrLastAdmin) {
			http.Error(w, "cannot remove last admin", http.StatusBadRequest)
			return
		}
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "failed to update user", http.StatusInternalServerError)
			return
		}
	}
	if password != "" {
		updated, err = s.store.UpdateUserPassword(id, password)
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "failed to update password", http.StatusBadRequest)
			return
		}
	}
	if payload.ProjectIDs != nil {
		if err := s.store.SetUserProjects(id, payload.ProjectIDs); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				http.Error(w, "user not found", http.StatusNotFound)
				return
			}
			http.Error(w, "failed to update projects", http.StatusBadRequest)
			return
		}
	}
	if updated.ID == 0 {
		updated, err = s.store.GetUserByID(id)
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
	}
	projects, _ := s.store.GetUserProjects(id)
	writeJSON(w, struct {
		ID         int64   `json:"id"`
		Email      string  `json:"email"`
		Role       string  `json:"role"`
		ProjectIDs []int64 `json:"projectIds"`
	}{ID: updated.ID, Email: updated.Email, Role: updated.Role, ProjectIDs: projects})
}

func (s *Server) handleProjectActions(w http.ResponseWriter, r *http.Request) {
	trimmed := strings.TrimPrefix(r.URL.Path, "/api/projects/")
	idStr := strings.Trim(strings.TrimSuffix(trimmed, "/"), " ")
	if idStr == "" {
		http.NotFound(w, r)
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid project id", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodDelete:
		s.deleteProjectHandler(w, r, id)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) listProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := s.store.ListProjects()
	if err != nil {
		http.Error(w, "failed to load projects", http.StatusInternalServerError)
		return
	}
	authVal := r.Context().Value(ctxUser)
	if authVal != nil {
		if auth, ok := authVal.(authUser); ok && auth.isRestricted {
			filtered := make([]store.Project, 0, len(auth.allowed))
			for _, p := range projects {
				if _, ok := auth.allowed[p.ID]; ok {
					filtered = append(filtered, p)
				}
			}
			projects = filtered
		}
	}
	writeJSON(w, projects)
}

func (s *Server) createProjectHandler(w http.ResponseWriter, r *http.Request) {
	auth := getAuth(r)
	var payload struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	payload.Name = strings.TrimSpace(payload.Name)
	if payload.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	p, err := s.store.CreateProject(payload.Name)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			http.Error(w, "project name already exists", http.StatusBadRequest)
			return
		}
		http.Error(w, "failed to create project", http.StatusInternalServerError)
		return
	}
	if auth.isRestricted {
		current, err := s.store.GetUserProjects(auth.user.ID)
		if err != nil {
			log.Printf("failed to load user projects after create: %v", err)
		} else {
			next := append(current, p.ID)
			if err := s.store.SetUserProjects(auth.user.ID, next); err != nil {
				log.Printf("failed to assign project to user: %v", err)
			}
		}
	}
	writeJSON(w, p)
}

func (s *Server) deleteProjectHandler(w http.ResponseWriter, r *http.Request, id int64) {
	if id == store.DefaultProjectID {
		http.Error(w, "cannot delete default project", http.StatusBadRequest)
		return
	}
	auth := getAuth(r)
	if auth.user.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := s.store.DeleteProject(id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "project not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to delete project", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listTasks(w http.ResponseWriter, r *http.Request) {
	auth := getAuth(r)
	projectID := int64(0)
	if pid := r.URL.Query().Get("projectId"); pid != "" {
		val, err := strconv.ParseInt(pid, 10, 64)
		if err != nil {
			http.Error(w, "invalid projectId", http.StatusBadRequest)
			return
		}
		projectID = val
	}

	if auth.isRestricted {
		if projectID == 0 || !auth.canAccess(projectID) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}

	tasks, err := s.store.FetchTasks(projectID, "", auth.allowed)
	if err != nil {
		http.Error(w, "failed to load tasks", http.StatusInternalServerError)
		return
	}

	writeJSON(w, tasks)
}

func (s *Server) createTask(w http.ResponseWriter, r *http.Request) {
	auth := getAuth(r)
	var payload struct {
		Title     string `json:"title"`
		Comment   string `json:"comment"`
		ProjectID int64  `json:"projectId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	payload.Title = strings.TrimSpace(payload.Title)
	payload.Comment = strings.TrimSpace(payload.Comment)
	if payload.ProjectID == 0 {
		payload.ProjectID = store.DefaultProjectID
	}
	if auth.isRestricted && !auth.canAccess(payload.ProjectID) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if payload.Title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}

	created, err := s.store.InsertTask(payload.Title, payload.Comment, payload.ProjectID)
	if err != nil {
		if strings.Contains(err.Error(), "project not found") {
			http.Error(w, "project not found", http.StatusBadRequest)
			return
		}
		http.Error(w, "failed to create task", http.StatusInternalServerError)
		return
	}

	writeJSON(w, created)
}

func (s *Server) updateStatus(w http.ResponseWriter, r *http.Request, id int64) {
	auth := getAuth(r)
	existing, err := s.store.GetTask(id)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "failed to load task", http.StatusInternalServerError)
		return
	}
	if auth.isRestricted && !auth.canAccess(existing.ProjectID) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var payload struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	payload.Status = strings.TrimSpace(payload.Status)

	updated, err := s.store.SetTaskStatus(id, payload.Status)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}
	if errors.Is(err, store.ErrInvalidStatus) {
		http.Error(w, "invalid status", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, "failed to update task", http.StatusInternalServerError)
		return
	}

	writeJSON(w, updated)
}

func (s *Server) updateComment(w http.ResponseWriter, r *http.Request, id int64) {
	auth := getAuth(r)
	existing, err := s.store.GetTask(id)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "failed to load task", http.StatusInternalServerError)
		return
	}
	if auth.isRestricted && !auth.canAccess(existing.ProjectID) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var payload struct {
		Comment string `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	payload.Comment = strings.TrimSpace(payload.Comment)

	updated, err := s.store.SetTaskComment(id, payload.Comment)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "failed to update comment", http.StatusInternalServerError)
		return
	}

	writeJSON(w, updated)
}

func (s *Server) deleteTaskHandler(w http.ResponseWriter, r *http.Request, id int64) {
	auth := getAuth(r)
	existing, err := s.store.GetTask(id)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "failed to load task", http.StatusInternalServerError)
		return
	}
	if auth.isRestricted && !auth.canAccess(existing.ProjectID) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := s.store.DeleteTask(id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "task not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to delete task", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	payload.Email = strings.TrimSpace(strings.ToLower(payload.Email))
	if payload.Email == "" || payload.Password == "" {
		http.Error(w, "email and password required", http.StatusBadRequest)
		return
	}
	u, err := s.store.GetUserByEmail(payload.Email)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(payload.Password)); err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	if u.Role == "blocked" {
		http.Error(w, "account blocked", http.StatusForbidden)
		return
	}
	token := createToken(u, s.authSecret)
	setAuthCookie(w, token)
	writeJSON(w, struct {
		ID    int64  `json:"id"`
		Email string `json:"email"`
		Role  string `json:"role"`
	}{ID: u.ID, Email: u.Email, Role: u.Role})
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if !s.allowRegistration {
		http.Error(w, "registration disabled", http.StatusForbidden)
		return
	}
	var payload struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	payload.Email = strings.TrimSpace(strings.ToLower(payload.Email))
	if payload.Email == "" || payload.Password == "" {
		http.Error(w, "email and password required", http.StatusBadRequest)
		return
	}
	if len(payload.Password) < 6 {
		http.Error(w, "password too short", http.StatusBadRequest)
		return
	}
	u, err := s.store.CreateUser(payload.Email, payload.Password, "user")
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			http.Error(w, "email already registered", http.StatusBadRequest)
			return
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	token := createToken(u, s.authSecret)
	setAuthCookie(w, token)
	writeJSON(w, struct {
		ID    int64  `json:"id"`
		Email string `json:"email"`
		Role  string `json:"role"`
	}{ID: u.ID, Email: u.Email, Role: u.Role})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	u, err := s.authenticate(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	writeJSON(w, struct {
		ID    int64  `json:"id"`
		Email string `json:"email"`
		Role  string `json:"role"`
	}{ID: u.ID, Email: u.Email, Role: u.Role})
}

func (s *Server) handleLogout(w http.ResponseWriter, _ *http.Request) {
	clearAuthCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) authenticate(r *http.Request) (store.User, error) {
	cookie, err := r.Cookie("auth")
	if err != nil {
		return store.User{}, err
	}
	claims, err := parseToken(cookie.Value, s.authSecret)
	if err != nil {
		return store.User{}, err
	}
	u, err := s.store.GetUserByID(claims.UserID)
	if err != nil {
		return store.User{}, err
	}
	if u.Role == "blocked" {
		return store.User{}, errors.New("blocked")
	}
	return u, nil
}

type tokenClaims struct {
	UserID int64
	Role   string
	Exp    time.Time
}

func createToken(u store.User, secret []byte) string {
	exp := time.Now().Add(authExpiry).Unix()
	payload := fmt.Sprintf("%d:%s:%d", u.ID, u.Role, exp)
	sig := sign(secret, payload)
	return base64.RawStdEncoding.EncodeToString([]byte(payload)) + "." + sig
}

func parseToken(token string, secret []byte) (tokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return tokenClaims{}, errors.New("invalid token")
	}
	payloadBytes, err := base64.RawStdEncoding.DecodeString(parts[0])
	if err != nil {
		return tokenClaims{}, err
	}
	payload := string(payloadBytes)
	if !verify(secret, payload, parts[1]) {
		return tokenClaims{}, errors.New("invalid signature")
	}
	items := strings.Split(payload, ":")
	if len(items) != 3 {
		return tokenClaims{}, errors.New("invalid payload")
	}
	id, err := strconv.ParseInt(items[0], 10, 64)
	if err != nil {
		return tokenClaims{}, err
	}
	role := items[1]
	expUnix, err := strconv.ParseInt(items[2], 10, 64)
	if err != nil {
		return tokenClaims{}, err
	}
	if time.Now().Unix() > expUnix {
		return tokenClaims{}, errors.New("token expired")
	}
	return tokenClaims{UserID: id, Role: role, Exp: time.Unix(expUnix, 0)}, nil
}

func sign(secret []byte, payload string) string {
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(payload))
	return base64.RawStdEncoding.EncodeToString(h.Sum(nil))
}

func verify(secret []byte, payload, sig string) bool {
	expected := sign(secret, payload)
	return hmac.Equal([]byte(expected), []byte(sig))
}

func getAuth(r *http.Request) authUser {
	val := r.Context().Value(ctxUser)
	if val == nil {
		return authUser{}
	}
	if auth, ok := val.(authUser); ok {
		return auth
	}
	return authUser{}
}

func (a authUser) canAccess(projectID int64) bool {
	if !a.isRestricted {
		return true
	}
	_, ok := a.allowed[projectID]
	return ok
}

func setAuthCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "auth",
		Value:    token,
		Path:     "/",
		MaxAge:   int(authExpiry.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearAuthCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "auth",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	if err := enc.Encode(v); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

func (s *Server) staticHandler() http.Handler {
	abs, err := filepath.Abs(s.staticDir)
	if err != nil {
		log.Printf("static path error: %v", err)
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "static assets not available", http.StatusInternalServerError)
		})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		requestPath := r.URL.Path
		if requestPath == "/" {
			requestPath = "/index.html"
		}
		full := filepath.Join(abs, filepath.Clean(requestPath))
		if !strings.HasPrefix(full, abs) {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}

		if _, err := os.Stat(full); errors.Is(err, os.ErrNotExist) {
			http.ServeFile(w, r, filepath.Join(abs, "index.html"))
			return
		}

		http.ServeFile(w, r, full)
	})
}
