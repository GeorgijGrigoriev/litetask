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
	store             store.Storer
	authSecret        []byte
	allowRegistration bool
	staticDir         string
}

type taskResponse struct {
	ID          int64               `json:"id"`
	Title       string              `json:"title"`
	Status      string              `json:"status"`
	Description string              `json:"description"`
	ProjectID   int64               `json:"projectId"`
	CreatedAt   time.Time           `json:"createdAt"`
	CreatedBy   int64               `json:"createdBy"`
	AuthorEmail string              `json:"authorEmail"`
	AuthorFirst string              `json:"authorFirstName,omitempty"`
	AuthorLast  string              `json:"authorLastName,omitempty"`
	Comments    []store.TaskComment `json:"comments"`
}

type userResponse struct {
	ID         int64   `json:"id"`
	Email      string  `json:"email"`
	Username   string  `json:"username"`
	Role       string  `json:"role"`
	FirstName  string  `json:"firstName"`
	LastName   string  `json:"lastName"`
	ProjectIDs []int64 `json:"projectIds"`
}

type meResponse struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Telegram  string `json:"telegram"`
}

func userToResponse(u store.User, projectIDs []int64) userResponse {
	return userResponse{
		ID:         u.ID,
		Email:      u.Email,
		Username:   u.Username,
		Role:       u.Role,
		FirstName:  u.FirstName,
		LastName:   u.LastName,
		ProjectIDs: projectIDs,
	}
}

func userToMe(u store.User) meResponse {
	return meResponse{
		ID:        u.ID,
		Email:     u.Email,
		Username:  u.Username,
		Role:      u.Role,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		Telegram:  u.Telegram,
	}
}

func New(s store.Storer, secret []byte, allowRegistration bool, staticDir string) *Server {
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
	mux.Handle("/api/profile", s.cors(s.requireUser(http.HandlerFunc(s.handleProfile))))
	mux.Handle("/", s.staticHandler())
	return mux
}

func (s *Server) cors(next http.Handler) http.Handler {
	allowedOrigin := os.Getenv("CORS_ORIGIN")
	if allowedOrigin == "" {
		allowedOrigin = "*"
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
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
			writeError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if u.Role == "blocked" {
			writeError(w, "account blocked", http.StatusForbidden)
			return
		}
		auth := authUser{user: u}
		if u.Role != "admin" {
			allowed, err := s.store.GetUserProjects(r.Context(), u.ID)
			if err != nil {
				writeError(w, "server error", http.StatusInternalServerError)
				return
			}
			auth.isRestricted = true
			auth.allowed = make(map[int64]struct{}, len(allowed)+1)
			for _, pid := range allowed {
				auth.allowed[pid] = struct{}{}
			}
			// Personal inbox is always accessible even if not in user_projects
			if inboxID := s.store.GetUserInboxID(r.Context(), u.ID); inboxID > 0 {
				auth.allowed[inboxID] = struct{}{}
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
			writeError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if u.Role != "admin" {
			writeError(w, "forbidden", http.StatusForbidden)
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
		writeError(w, "method not allowed", http.StatusMethodNotAllowed)
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
		writeError(w, "invalid task id", http.StatusBadRequest)
		return
	}

	if len(parts) == 2 && parts[1] == "status" {
		if r.Method != http.MethodPatch {
			writeError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.updateStatus(w, r, id)
		return
	}

	if len(parts) == 2 && parts[1] == "comments" {
		switch r.Method {
		case http.MethodGet:
			s.listTaskComments(w, r, id)
		case http.MethodPost:
			s.addTaskComment(w, r, id)
		default:
			writeError(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	if len(parts) == 3 && parts[1] == "comments" && r.Method == http.MethodDelete {
		commentID, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			writeError(w, "invalid comment id", http.StatusBadRequest)
			return
		}
		s.deleteTaskComment(w, r, id, commentID)
		return
	}

	if len(parts) == 1 && r.Method == http.MethodPatch {
		s.updateDescription(w, r, id)
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
		writeError(w, "method not allowed", http.StatusMethodNotAllowed)
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
		writeError(w, "not found", http.StatusNotFound)
	}
}

func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		users, err := s.store.ListUsers(r.Context())
		if err != nil {
			writeError(w, "failed to load users", http.StatusInternalServerError)
			return
		}
		result := make([]userResponse, len(users))
		for i, u := range users {
			projects, _ := s.store.GetUserProjects(r.Context(), u.ID)
			result[i] = userToResponse(u, projects)
		}
		writeJSON(w, result)
	case http.MethodPost:
		var payload struct {
			Email     string `json:"email"`
			Username  string `json:"username"`
			Password  string `json:"password"`
			Role      string `json:"role"`
			FirstName string `json:"firstName"`
			LastName  string `json:"lastName"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(w, "invalid request body", http.StatusBadRequest)
			return
		}
		payload.Email = strings.ToLower(strings.TrimSpace(payload.Email))
		payload.Username = strings.TrimSpace(strings.ToLower(payload.Username))
		payload.Password = strings.TrimSpace(payload.Password)
		payload.Role = strings.TrimSpace(strings.ToLower(payload.Role))
		if payload.Email == "" || payload.Password == "" {
			writeError(w, "email and password required", http.StatusBadRequest)
			return
		}
		if len(payload.Password) < 6 {
			writeError(w, "password too short", http.StatusBadRequest)
			return
		}
		if payload.Role == "" {
			payload.Role = "user"
		}
		if payload.Role != "user" && payload.Role != "admin" && payload.Role != "blocked" {
			writeError(w, "invalid role", http.StatusBadRequest)
			return
		}
		u, err := s.store.CreateUser(r.Context(), payload.Email, payload.Username, payload.Password, payload.Role, payload.FirstName, payload.LastName)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "unique") {
				lower := strings.ToLower(err.Error())
				if strings.Contains(lower, "users.username") || strings.Contains(lower, "idx_users_username") {
					writeError(w, "юзернейм уже занят", http.StatusBadRequest)
					return
				}
				writeError(w, "email already registered", http.StatusBadRequest)
				return
			}
			if strings.Contains(strings.ToLower(err.Error()), "username") {
				writeError(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeError(w, "failed to create user", http.StatusInternalServerError)
			return
		}
		projects, _ := s.store.GetUserProjects(r.Context(), u.ID)
		writeJSON(w, userToResponse(u, projects))
	default:
		writeError(w, "method not allowed", http.StatusMethodNotAllowed)
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
		writeError(w, "invalid user id", http.StatusBadRequest)
		return
	}
	if r.Method != http.MethodPatch {
		writeError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var payload struct {
		Role       string  `json:"role"`
		Password   string  `json:"password"`
		ProjectIDs []int64 `json:"projectIds"`
		FirstName  *string `json:"firstName"`
		LastName   *string `json:"lastName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	payload.Role = strings.TrimSpace(strings.ToLower(payload.Role))
	password := strings.TrimSpace(payload.Password)
	if payload.Role == "" && password == "" && payload.ProjectIDs == nil && payload.FirstName == nil && payload.LastName == nil {
		writeError(w, "nothing to update", http.StatusBadRequest)
		return
	}

	var updated store.User
	if payload.Role != "" {
		updated, err = s.store.UpdateUserRole(r.Context(), id, payload.Role)
		if errors.Is(err, store.ErrInvalidRole) {
			writeError(w, "invalid role", http.StatusBadRequest)
			return
		}
		if errors.Is(err, store.ErrLastAdmin) {
			writeError(w, "cannot remove last admin", http.StatusBadRequest)
			return
		}
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, "user not found", http.StatusNotFound)
			return
		}
		if err != nil {
			writeError(w, "failed to update user", http.StatusInternalServerError)
			return
		}
	}
	if password != "" {
		updated, err = s.store.UpdateUserPassword(r.Context(), id, password)
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, "user not found", http.StatusNotFound)
			return
		}
		if err != nil {
			writeError(w, "failed to update password", http.StatusBadRequest)
			return
		}
	}
	if payload.ProjectIDs != nil {
		if err := s.store.SetUserProjects(r.Context(), id, payload.ProjectIDs); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, "user not found", http.StatusNotFound)
				return
			}
			writeError(w, "failed to update projects", http.StatusBadRequest)
			return
		}
	}
	if payload.FirstName != nil || payload.LastName != nil {
		updated, err = s.store.UpdateUserProfile(r.Context(), id, nil, nil, payload.FirstName, payload.LastName)
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, "user not found", http.StatusNotFound)
			return
		}
		if err != nil {
			writeError(w, "failed to update user", http.StatusInternalServerError)
			return
		}
	}
	if updated.ID == 0 {
		updated, err = s.store.GetUserByID(r.Context(), id)
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, "user not found", http.StatusNotFound)
			return
		}
		if err != nil {
			writeError(w, "server error", http.StatusInternalServerError)
			return
		}
	}
	projects, _ := s.store.GetUserProjects(r.Context(), id)
	writeJSON(w, userToResponse(updated, projects))
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
		writeError(w, "invalid project id", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodDelete:
		s.deleteProjectHandler(w, r, id)
	default:
		writeError(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) listProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := s.store.ListProjects(r.Context())
	if err != nil {
		writeError(w, "failed to load projects", http.StatusInternalServerError)
		return
	}
	auth := getAuth(r)
	filtered := make([]store.Project, 0, len(projects))
	for _, p := range projects {
		// Never show another user's personal project
		if p.OwnerID != 0 && p.OwnerID != auth.user.ID {
			continue
		}
		if auth.isRestricted {
			if _, ok := auth.allowed[p.ID]; !ok {
				continue
			}
		}
		filtered = append(filtered, p)
	}
	writeJSON(w, filtered)
}

func (s *Server) createProjectHandler(w http.ResponseWriter, r *http.Request) {
	auth := getAuth(r)
	var payload struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	payload.Name = strings.TrimSpace(payload.Name)
	if payload.Name == "" {
		writeError(w, "name is required", http.StatusBadRequest)
		return
	}
	if len(payload.Name) > 255 {
		writeError(w, "project name too long", http.StatusBadRequest)
		return
	}
	p, err := s.store.CreateProject(r.Context(), payload.Name)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			writeError(w, "project name already exists", http.StatusBadRequest)
			return
		}
		writeError(w, "failed to create project", http.StatusInternalServerError)
		return
	}
	if auth.isRestricted {
		current, err := s.store.GetUserProjects(r.Context(), auth.user.ID)
		if err != nil {
			log.Printf("failed to load user projects after create: %v", err)
		} else {
			next := append(current, p.ID)
			if err := s.store.SetUserProjects(r.Context(), auth.user.ID, next); err != nil {
				log.Printf("failed to assign project to user: %v", err)
			}
		}
	}
	writeJSON(w, p)
}

func (s *Server) deleteProjectHandler(w http.ResponseWriter, r *http.Request, id int64) {
	auth := getAuth(r)
	if auth.user.Role != "admin" {
		writeError(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := s.store.DeleteProject(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrProtectedProject) {
			writeError(w, "cannot delete default project", http.StatusBadRequest)
			return
		}
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, "project not found", http.StatusNotFound)
			return
		}
		writeError(w, "failed to delete project", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func toTaskResponse(t store.Task, comments []store.TaskComment) taskResponse {
	if comments == nil {
		comments = []store.TaskComment{}
	}
	return taskResponse{
		ID:          t.ID,
		Title:       t.Title,
		Status:      t.Status,
		Description: t.Description,
		ProjectID:   t.ProjectID,
		CreatedAt:   t.CreatedAt,
		CreatedBy:   t.CreatedBy,
		AuthorEmail: t.AuthorEmail,
		AuthorFirst: t.AuthorFirst,
		AuthorLast:  t.AuthorLast,
		Comments:    comments,
	}
}

func (s *Server) attachComments(ctx context.Context, tasks []store.Task) ([]taskResponse, error) {
	ids := make([]int64, 0, len(tasks))
	for _, t := range tasks {
		ids = append(ids, t.ID)
	}
	comments, err := s.store.ListCommentsByTaskIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]taskResponse, 0, len(tasks))
	for _, t := range tasks {
		result = append(result, toTaskResponse(t, comments[t.ID]))
	}
	return result, nil
}

func (s *Server) listTasks(w http.ResponseWriter, r *http.Request) {
	auth := getAuth(r)
	projectID := int64(0)
	if pid := r.URL.Query().Get("projectId"); pid != "" {
		val, err := strconv.ParseInt(pid, 10, 64)
		if err != nil {
			writeError(w, "invalid projectId", http.StatusBadRequest)
			return
		}
		projectID = val
	}

	if auth.isRestricted {
		if projectID == 0 || !auth.canAccess(projectID) {
			writeError(w, "forbidden", http.StatusForbidden)
			return
		}
	}

	tasks, err := s.store.FetchTasks(r.Context(), projectID, "", auth.allowed)
	if err != nil {
		writeError(w, "failed to load tasks", http.StatusInternalServerError)
		return
	}

	withComments, err := s.attachComments(r.Context(), tasks)
	if err != nil {
		writeError(w, "failed to load comments", http.StatusInternalServerError)
		return
	}
	writeJSON(w, withComments)
}

func (s *Server) createTask(w http.ResponseWriter, r *http.Request) {
	auth := getAuth(r)
	var payload struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		ProjectID   int64  `json:"projectId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	payload.Title = strings.TrimSpace(payload.Title)
	payload.Description = strings.TrimSpace(payload.Description)
	if payload.ProjectID == 0 {
		payload.ProjectID = store.DefaultProjectID
	}
	if auth.isRestricted && !auth.canAccess(payload.ProjectID) {
		writeError(w, "forbidden", http.StatusForbidden)
		return
	}
	if payload.Title == "" {
		writeError(w, "title is required", http.StatusBadRequest)
		return
	}
	if len(payload.Title) > 500 {
		writeError(w, "title too long", http.StatusBadRequest)
		return
	}

	created, err := s.store.InsertTask(r.Context(), payload.Title, payload.Description, "", payload.ProjectID, auth.user.ID)
	if err != nil {
		if strings.Contains(err.Error(), "project not found") {
			writeError(w, "project not found", http.StatusBadRequest)
			return
		}
		writeError(w, "failed to create task", http.StatusInternalServerError)
		return
	}

	writeJSON(w, toTaskResponse(created, []store.TaskComment{}))
}

func (s *Server) updateStatus(w http.ResponseWriter, r *http.Request, id int64) {
	auth := getAuth(r)
	existing, err := s.store.GetTask(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, "task not found", http.StatusNotFound)
		return
	}
	if err != nil {
		writeError(w, "failed to load task", http.StatusInternalServerError)
		return
	}
	if auth.isRestricted && !auth.canAccess(existing.ProjectID) {
		writeError(w, "forbidden", http.StatusForbidden)
		return
	}
	var payload struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	payload.Status = strings.TrimSpace(payload.Status)

	updated, err := s.store.SetTaskStatus(r.Context(), id, payload.Status)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, "task not found", http.StatusNotFound)
		return
	}
	if errors.Is(err, store.ErrInvalidStatus) {
		writeError(w, "invalid status", http.StatusBadRequest)
		return
	}
	if err != nil {
		writeError(w, "failed to update task", http.StatusInternalServerError)
		return
	}

	comments, err := s.store.ListTaskComments(r.Context(), id)
	if err != nil {
		writeError(w, "failed to load comments", http.StatusInternalServerError)
		return
	}

	writeJSON(w, toTaskResponse(updated, comments))
}

func (s *Server) updateDescription(w http.ResponseWriter, r *http.Request, id int64) {
	auth := getAuth(r)
	existing, err := s.store.GetTask(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, "task not found", http.StatusNotFound)
		return
	}
	if err != nil {
		writeError(w, "failed to load task", http.StatusInternalServerError)
		return
	}
	if auth.isRestricted && !auth.canAccess(existing.ProjectID) {
		writeError(w, "forbidden", http.StatusForbidden)
		return
	}
	var payload struct {
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	payload.Description = strings.TrimSpace(payload.Description)

	updated, err := s.store.SetTaskDescription(r.Context(), id, payload.Description)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, "task not found", http.StatusNotFound)
		return
	}
	if err != nil {
		writeError(w, "failed to update description", http.StatusInternalServerError)
		return
	}

	comments, err := s.store.ListTaskComments(r.Context(), id)
	if err != nil {
		writeError(w, "failed to load comments", http.StatusInternalServerError)
		return
	}

	writeJSON(w, toTaskResponse(updated, comments))
}

func (s *Server) listTaskComments(w http.ResponseWriter, r *http.Request, taskID int64) {
	auth := getAuth(r)
	existing, err := s.store.GetTask(r.Context(), taskID)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, "task not found", http.StatusNotFound)
		return
	}
	if err != nil {
		writeError(w, "failed to load task", http.StatusInternalServerError)
		return
	}
	if auth.isRestricted && !auth.canAccess(existing.ProjectID) {
		writeError(w, "forbidden", http.StatusForbidden)
		return
	}
	comments, err := s.store.ListTaskComments(r.Context(), taskID)
	if err != nil {
		writeError(w, "failed to load comments", http.StatusInternalServerError)
		return
	}
	if comments == nil {
		comments = []store.TaskComment{}
	}
	writeJSON(w, comments)
}

func (s *Server) addTaskComment(w http.ResponseWriter, r *http.Request, taskID int64) {
	auth := getAuth(r)
	existing, err := s.store.GetTask(r.Context(), taskID)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, "task not found", http.StatusNotFound)
		return
	}
	if err != nil {
		writeError(w, "failed to load task", http.StatusInternalServerError)
		return
	}
	if auth.isRestricted && !auth.canAccess(existing.ProjectID) {
		writeError(w, "forbidden", http.StatusForbidden)
		return
	}
	var payload struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	payload.Body = strings.TrimSpace(payload.Body)
	if payload.Body == "" {
		writeError(w, "comment cannot be empty", http.StatusBadRequest)
		return
	}
	if len(payload.Body) > 5000 {
		writeError(w, "comment too long", http.StatusBadRequest)
		return
	}
	comment, err := s.store.AddTaskComment(r.Context(), taskID, payload.Body, auth.user.ID)
	if err != nil {
		writeError(w, "failed to add comment", http.StatusInternalServerError)
		return
	}
	writeJSON(w, comment)
}

func (s *Server) deleteTaskComment(w http.ResponseWriter, r *http.Request, taskID, commentID int64) {
	auth := getAuth(r)
	task, err := s.store.GetTask(r.Context(), taskID)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, "task not found", http.StatusNotFound)
		return
	}
	if err != nil {
		writeError(w, "failed to load task", http.StatusInternalServerError)
		return
	}
	if auth.isRestricted && !auth.canAccess(task.ProjectID) {
		writeError(w, "forbidden", http.StatusForbidden)
		return
	}
	comment, err := s.store.GetTaskComment(r.Context(), commentID)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, "comment not found", http.StatusNotFound)
		return
	}
	if err != nil {
		writeError(w, "failed to load comment", http.StatusInternalServerError)
		return
	}
	if comment.TaskID != taskID {
		writeError(w, "comment does not belong to task", http.StatusBadRequest)
		return
	}
	if comment.AuthorID != auth.user.ID {
		writeError(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := s.store.DeleteTaskComment(r.Context(), commentID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, "comment not found", http.StatusNotFound)
			return
		}
		writeError(w, "failed to delete comment", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deleteTaskHandler(w http.ResponseWriter, r *http.Request, id int64) {
	auth := getAuth(r)
	existing, err := s.store.GetTask(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, "task not found", http.StatusNotFound)
		return
	}
	if err != nil {
		writeError(w, "failed to load task", http.StatusInternalServerError)
		return
	}
	if auth.isRestricted && !auth.canAccess(existing.ProjectID) {
		writeError(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := s.store.DeleteTask(r.Context(), id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, "task not found", http.StatusNotFound)
			return
		}
		writeError(w, "failed to delete task", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Login    string `json:"login"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	login := strings.TrimSpace(strings.ToLower(payload.Login))
	if login == "" {
		login = strings.TrimSpace(strings.ToLower(payload.Email))
	}
	if login == "" || payload.Password == "" {
		writeError(w, "email/юзернейм и пароль обязательны", http.StatusBadRequest)
		return
	}
	u, err := s.store.GetUserByEmailOrUsername(r.Context(), login)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	if err != nil {
		writeError(w, "server error", http.StatusInternalServerError)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(payload.Password)); err != nil {
		writeError(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	if u.Role == "blocked" {
		writeError(w, "account blocked", http.StatusForbidden)
		return
	}
	token := createToken(u, s.authSecret)
	setAuthCookie(w, token)
	writeJSON(w, userToMe(u))
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if !s.allowRegistration {
		writeError(w, "registration disabled", http.StatusForbidden)
		return
	}
	var payload struct {
		Email     string `json:"email"`
		Username  string `json:"username"`
		Password  string `json:"password"`
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	payload.Email = strings.TrimSpace(strings.ToLower(payload.Email))
	payload.Username = strings.TrimSpace(strings.ToLower(payload.Username))
	if payload.Email == "" || payload.Password == "" {
		writeError(w, "email and password required", http.StatusBadRequest)
		return
	}
	if len(payload.Password) < 6 {
		writeError(w, "password too short", http.StatusBadRequest)
		return
	}
	u, err := s.store.CreateUser(r.Context(), payload.Email, payload.Username, payload.Password, "user", payload.FirstName, payload.LastName)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			lower := strings.ToLower(err.Error())
			if strings.Contains(lower, "users.username") || strings.Contains(lower, "idx_users_username") {
				writeError(w, "юзернейм уже занят", http.StatusBadRequest)
				return
			}
			writeError(w, "email already registered", http.StatusBadRequest)
			return
		}
		if strings.Contains(err.Error(), "username") {
			writeError(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeError(w, "server error", http.StatusInternalServerError)
		return
	}
	token := createToken(u, s.authSecret)
	setAuthCookie(w, token)
	writeJSON(w, userToMe(u))
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	u, err := s.authenticate(r)
	if err != nil {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if u.Role == "blocked" {
		writeError(w, "account blocked", http.StatusForbidden)
		return
	}
	writeJSON(w, userToMe(u))
}

func (s *Server) handleLogout(w http.ResponseWriter, _ *http.Request) {
	clearAuthCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	u, err := s.authenticate(r)
	if err != nil {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if u.Role == "blocked" {
		writeError(w, "account blocked", http.StatusForbidden)
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, userToMe(u))
	case http.MethodPatch:
		var payload struct {
			Password  *string `json:"password"`
			Telegram  *string `json:"telegram"`
			FirstName *string `json:"firstName"`
			LastName  *string `json:"lastName"`
			Username  *string `json:"username"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if payload.Password == nil && payload.Telegram == nil && payload.FirstName == nil && payload.LastName == nil && payload.Username == nil {
			writeError(w, "nothing to update", http.StatusBadRequest)
			return
		}
		if payload.Telegram != nil {
			trimmed := strings.TrimSpace(*payload.Telegram)
			payload.Telegram = &trimmed
		}
		if payload.FirstName != nil {
			trimmed := strings.TrimSpace(*payload.FirstName)
			payload.FirstName = &trimmed
		}
		if payload.LastName != nil {
			trimmed := strings.TrimSpace(*payload.LastName)
			payload.LastName = &trimmed
		}
		if payload.Username != nil {
			trimmed := strings.TrimSpace(strings.ToLower(*payload.Username))
			payload.Username = &trimmed
		}

		if payload.Username != nil && *payload.Username != "" {
			if u.Username != "" && u.Username != *payload.Username {
				writeError(w, "юзернейм уже установлен", http.StatusBadRequest)
				return
			}
			if u.Username == "" {
				updated, err := s.store.SetUsernameOnce(r.Context(), u.ID, *payload.Username)
				if err != nil {
					if errors.Is(err, store.ErrUsernameSet) {
						writeError(w, "юзернейм уже установлен", http.StatusBadRequest)
						return
					}
					if strings.Contains(strings.ToLower(err.Error()), "unique") {
						writeError(w, "юзернейм уже занят", http.StatusBadRequest)
						return
					}
					writeError(w, err.Error(), http.StatusBadRequest)
					return
				}
				u = updated
			}
		}

		updated, err := s.store.UpdateUserProfile(r.Context(), u.ID, payload.Password, payload.Telegram, payload.FirstName, payload.LastName)
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, "user not found", http.StatusNotFound)
			return
		}
		if err != nil {
			if err.Error() == "password too short" {
				writeError(w, "password too short", http.StatusBadRequest)
				return
			}
			writeError(w, "failed to update profile", http.StatusInternalServerError)
			return
		}
		writeJSON(w, userToMe(updated))
	default:
		writeError(w, "method not allowed", http.StatusMethodNotAllowed)
	}
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
	return s.store.GetUserByID(r.Context(), claims.UserID)
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

func writeError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(struct {
		Error string `json:"error"`
	}{Error: msg})
}

func (s *Server) staticHandler() http.Handler {
	abs, err := filepath.Abs(s.staticDir)
	if err != nil {
		log.Printf("static path error: %v", err)
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			writeError(w, "static assets not available", http.StatusInternalServerError)
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
			writeError(w, "invalid path", http.StatusBadRequest)
			return
		}

		if _, err := os.Stat(full); errors.Is(err, os.ErrNotExist) {
			http.ServeFile(w, r, filepath.Join(abs, "index.html"))
			return
		}

		http.ServeFile(w, r, full)
	})
}
