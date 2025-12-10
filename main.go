package main

import (
	"context"
	"database/sql"
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

	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

const (
	defaultAddr        = ":8080"
	defaultDBPath      = "tasks.db"
	defaultAuthExpiry  = 30 * 24 * time.Hour
	defaultProjectID   = 1
	defaultProjectName = "Общий"
)

var allowedStatuses = map[string]struct{}{
	"new":         {},
	"in_progress": {},
	"done":        {},
}

var statusTitles = map[string]string{
	"new":         "Новая",
	"in_progress": "В работе",
	"done":        "Готова",
}

var errInvalidStatus = errors.New("invalid status")

func loadSecret() ([]byte, error) {
	if val := os.Getenv("AUTH_SECRET"); val != "" {
		decoded, err := base64.StdEncoding.DecodeString(val)
		if err == nil && len(decoded) >= 32 {
			return decoded, nil
		}
		if len(val) >= 32 {
			return []byte(val), nil
		}
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	log.Printf("generated random auth secret; set AUTH_SECRET to persist sessions")
	return secret, nil
}

func randomPassword() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "changeme123"
	}
	return base64.RawStdEncoding.EncodeToString(b)
}

type task struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	Comment   string    `json:"comment"`
	ProjectID int64     `json:"projectId"`
	CreatedAt time.Time `json:"createdAt"`
}

type server struct {
	db                *sql.DB
	authSecret        []byte
	allowRegistration bool
}

type project struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

type user struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	Password  string    `json:"-"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
}

type ctxKey string

const ctxUser ctxKey = "user"

func (s *server) insertTask(title, comment string, projectID int64) (task, error) {
	var t task
	ok, err := s.projectExists(projectID)
	if err != nil {
		return t, err
	}
	if !ok {
		return t, fmt.Errorf("project not found")
	}

	res, err := s.db.Exec(`INSERT INTO tasks (title, status, comment, project_id) VALUES (?, 'new', ?, ?)`, title, comment, projectID)
	if err != nil {
		return t, err
	}
	id, _ := res.LastInsertId()
	err = s.db.QueryRow(`SELECT id, title, status, comment, project_id, created_at FROM tasks WHERE id = ?`, id).
		Scan(&t.ID, &t.Title, &t.Status, &t.Comment, &t.ProjectID, &t.CreatedAt)
	if err != nil {
		return t, err
	}
	t.CreatedAt = t.CreatedAt.UTC()
	return t, nil
}

func (s *server) setTaskStatus(id int64, status string) (task, error) {
	var t task
	if _, ok := allowedStatuses[status]; !ok {
		return t, errInvalidStatus
	}

	res, err := s.db.Exec(`UPDATE tasks SET status = ? WHERE id = ?`, status, id)
	if err != nil {
		return t, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return t, sql.ErrNoRows
	}

	err = s.db.QueryRow(`SELECT id, title, status, comment, project_id, created_at FROM tasks WHERE id = ?`, id).
		Scan(&t.ID, &t.Title, &t.Status, &t.Comment, &t.ProjectID, &t.CreatedAt)
	if err != nil {
		return t, err
	}
	t.CreatedAt = t.CreatedAt.UTC()
	return t, nil
}

func (s *server) setTaskComment(id int64, comment string) (task, error) {
	var t task
	res, err := s.db.Exec(`UPDATE tasks SET comment = ? WHERE id = ?`, comment, id)
	if err != nil {
		return t, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return t, sql.ErrNoRows
	}
	err = s.db.QueryRow(`SELECT id, title, status, comment, project_id, created_at FROM tasks WHERE id = ?`, id).
		Scan(&t.ID, &t.Title, &t.Status, &t.Comment, &t.ProjectID, &t.CreatedAt)
	if err != nil {
		return t, err
	}
	t.CreatedAt = t.CreatedAt.UTC()
	return t, nil
}

func (s *server) deleteTask(id int64) error {
	res, err := s.db.Exec(`DELETE FROM tasks WHERE id = ?`, id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *server) projectExists(id int64) (bool, error) {
	var exists bool
	err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM projects WHERE id = ?)`, id).Scan(&exists)
	return exists, err
}

func (s *server) createProject(name string) (project, error) {
	var p project
	res, err := s.db.Exec(`INSERT INTO projects (name) VALUES (?)`, name)
	if err != nil {
		return p, err
	}
	id, _ := res.LastInsertId()
	err = s.db.QueryRow(`SELECT id, name, created_at FROM projects WHERE id = ?`, id).
		Scan(&p.ID, &p.Name, &p.CreatedAt)
	if err != nil {
		return p, err
	}
	p.CreatedAt = p.CreatedAt.UTC()
	return p, nil
}

func (s *server) listProjectsDB() ([]project, error) {
	rows, err := s.db.Query(`SELECT id, name, created_at FROM projects ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	projects := make([]project, 0)
	for rows.Next() {
		var p project
		if err := rows.Scan(&p.ID, &p.Name, &p.CreatedAt); err != nil {
			return nil, err
		}
		p.CreatedAt = p.CreatedAt.UTC()
		projects = append(projects, p)
	}
	return projects, nil
}

func (s *server) deleteProject(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM tasks WHERE project_id = ?`, id); err != nil {
		return err
	}

	res, err := tx.Exec(`DELETE FROM projects WHERE id = ?`, id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}

	return tx.Commit()
}

func (s *server) createUser(email, password, role string) (user, error) {
	var u user
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return u, err
	}
	res, err := s.db.Exec(`INSERT INTO users (email, password_hash, role) VALUES (?, ?, ?)`, email, string(hash), role)
	if err != nil {
		return u, err
	}
	id, _ := res.LastInsertId()
	err = s.db.QueryRow(`SELECT id, email, password_hash, role, created_at FROM users WHERE id = ?`, id).
		Scan(&u.ID, &u.Email, &u.Password, &u.Role, &u.CreatedAt)
	if err != nil {
		return u, err
	}
	u.CreatedAt = u.CreatedAt.UTC()
	return u, nil
}

func (s *server) getUserByEmail(email string) (user, error) {
	var u user
	err := s.db.QueryRow(`SELECT id, email, password_hash, role, created_at FROM users WHERE email = ?`, email).
		Scan(&u.ID, &u.Email, &u.Password, &u.Role, &u.CreatedAt)
	if err != nil {
		return u, err
	}
	u.CreatedAt = u.CreatedAt.UTC()
	return u, nil
}

func (s *server) getUserByID(id int64) (user, error) {
	var u user
	err := s.db.QueryRow(`SELECT id, email, password_hash, role, created_at FROM users WHERE id = ?`, id).
		Scan(&u.ID, &u.Email, &u.Password, &u.Role, &u.CreatedAt)
	if err != nil {
		return u, err
	}
	u.CreatedAt = u.CreatedAt.UTC()
	return u, nil
}

func (s *server) authenticate(r *http.Request) (user, error) {
	cookie, err := r.Cookie("auth")
	if err != nil {
		return user{}, err
	}
	claims, err := parseToken(cookie.Value, s.authSecret)
	if err != nil {
		return user{}, err
	}
	u, err := s.getUserByID(claims.UserID)
	if err != nil {
		return user{}, err
	}
	if u.Role == "blocked" {
		return user{}, errors.New("blocked")
	}
	return u, nil
}

type tokenClaims struct {
	UserID int64
	Role   string
	Exp    time.Time
}

func createToken(u user, secret []byte) string {
	exp := time.Now().Add(defaultAuthExpiry).Unix()
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

func setAuthCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "auth",
		Value:    token,
		Path:     "/",
		MaxAge:   int(defaultAuthExpiry.Seconds()),
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

func main() {
	dbPath := envOrDefault("DB_PATH", defaultDBPath)
	db, err := initDB(dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	secret, err := loadSecret()
	if err != nil {
		log.Fatalf("failed to load auth secret: %v", err)
	}

	s := &server{
		db:                db,
		authSecret:        secret,
		allowRegistration: envOrDefault("ALLOW_REGISTRATION", "true") != "false",
	}
	go s.startTelegramBot(ctx)

	log.Printf("listening on %s", defaultAddr)
	if err := http.ListenAndServe(defaultAddr, s.routes()); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func initDB(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	schema := `
CREATE TABLE IF NOT EXISTS projects (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL UNIQUE,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS users (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	email TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	role TEXT NOT NULL DEFAULT 'user',
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS tasks (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	title TEXT NOT NULL,
	status TEXT NOT NULL,
	comment TEXT DEFAULT '',
	project_id INTEGER NOT NULL DEFAULT 1,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}

	// Add comment column for existing databases created before the field was introduced.
	if _, err := db.Exec(`ALTER TABLE tasks ADD COLUMN comment TEXT DEFAULT ''`); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			log.Printf("warning: unable to add comment column: %v", err)
		}
	}

	// Ensure project_id column and index exist for older databases.
	if _, err := db.Exec(`ALTER TABLE tasks ADD COLUMN project_id INTEGER NOT NULL DEFAULT 1`); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			log.Printf("warning: unable to add project_id column: %v", err)
		}
	}
	if _, err := db.Exec(`UPDATE tasks SET project_id = ? WHERE project_id IS NULL OR project_id = 0`, defaultProjectID); err != nil {
		log.Printf("warning: unable to backfill project_id: %v", err)
	}

	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_tasks_project ON tasks(project_id)`); err != nil {
		log.Printf("warning: unable to ensure idx_tasks_project: %v", err)
	}

	if err := ensureDefaultProject(db); err != nil {
		log.Printf("warning: unable to ensure default project: %v", err)
	}
	if err := ensureAdminUser(db); err != nil {
		log.Printf("warning: unable to ensure admin user: %v", err)
	}
	return db, nil
}

func ensureDefaultProject(db *sql.DB) error {
	if _, err := db.Exec(`INSERT OR IGNORE INTO projects (id, name) VALUES (?, ?)`, defaultProjectID, defaultProjectName); err != nil {
		return err
	}
	_, err := db.Exec(`UPDATE projects SET name = ? WHERE id = ? AND name != ?`, defaultProjectName, defaultProjectID, defaultProjectName)
	return err
}

func ensureAdminUser(db *sql.DB) error {
	adminEmail := envOrDefault("ADMIN_EMAIL", "admin@example.com")
	adminPassword := os.Getenv("ADMIN_PASSWORD")

	var existing user
	err := db.QueryRow(`SELECT id, email FROM users WHERE role = 'admin' ORDER BY id LIMIT 1`).Scan(&existing.ID, &existing.Email)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	// If admin exists, allow updating email/password via env.
	if existing.ID != 0 {
		if adminEmail != "" && adminEmail != existing.Email {
			if _, err := db.Exec(`UPDATE users SET email = ? WHERE id = ?`, adminEmail, existing.ID); err != nil {
				return err
			}
			log.Printf("updated admin email to %s from ADMIN_EMAIL", adminEmail)
		}
		if adminPassword != "" {
			hash, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
			if err != nil {
				return err
			}
			if _, err := db.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, string(hash), existing.ID); err != nil {
				return err
			}
			log.Printf("updated admin password from ADMIN_PASSWORD")
		}
		return nil
	}

	// Create admin if none exists.
	password := adminPassword
	if password == "" {
		password = randomPassword()
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if _, err := db.Exec(`INSERT INTO users (email, password_hash, role) VALUES (?, ?, 'admin')`, adminEmail, string(hash)); err != nil {
		return err
	}
	if adminPassword == "" {
		log.Printf("created default admin: %s / %s", adminEmail, password)
	} else {
		log.Printf("created admin from env: %s", adminEmail)
	}
	return nil
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/api/auth/", s.cors(http.HandlerFunc(s.handleAuthRoutes)))
	mux.Handle("/api/tasks", s.cors(s.requireUser(http.HandlerFunc(s.handleTasks))))
	mux.Handle("/api/tasks/", s.cors(s.requireUser(http.HandlerFunc(s.handleTaskActions))))
	mux.Handle("/api/projects", s.cors(s.requireUser(http.HandlerFunc(s.handleProjects))))
	mux.Handle("/api/projects/", s.cors(s.requireUser(http.HandlerFunc(s.handleProjectActions))))
	mux.Handle("/", s.staticHandler("web/dist"))
	return mux
}

func (s *server) cors(next http.Handler) http.Handler {
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

func (s *server) requireUser(next http.Handler) http.Handler {
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
		ctx := context.WithValue(r.Context(), ctxUser, u)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *server) requireAdmin(next http.Handler) http.Handler {
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
		ctx := context.WithValue(r.Context(), ctxUser, u)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *server) handleTasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listTasks(w, r)
	case http.MethodPost:
		s.createTask(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *server) handleTaskActions(w http.ResponseWriter, r *http.Request) {
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

func (s *server) handleProjects(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listProjects(w, r)
	case http.MethodPost:
		s.createProjectHandler(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *server) handleAuthRoutes(w http.ResponseWriter, r *http.Request) {
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

func (s *server) handleProjectActions(w http.ResponseWriter, r *http.Request) {
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

func (s *server) listProjects(w http.ResponseWriter, _ *http.Request) {
	projects, err := s.listProjectsDB()
	if err != nil {
		http.Error(w, "failed to load projects", http.StatusInternalServerError)
		return
	}
	writeJSON(w, projects)
}

func (s *server) createProjectHandler(w http.ResponseWriter, r *http.Request) {
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
	p, err := s.createProject(payload.Name)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			http.Error(w, "project name already exists", http.StatusBadRequest)
			return
		}
		http.Error(w, "failed to create project", http.StatusInternalServerError)
		return
	}
	writeJSON(w, p)
}

func (s *server) deleteProjectHandler(w http.ResponseWriter, r *http.Request, id int64) {
	if id == defaultProjectID {
		http.Error(w, "cannot delete default project", http.StatusBadRequest)
		return
	}
	u, ok := r.Context().Value(ctxUser).(user)
	if !ok || u.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := s.deleteProject(id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "project not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to delete project", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) listTasks(w http.ResponseWriter, r *http.Request) {
	projectID := int64(0)
	if pid := r.URL.Query().Get("projectId"); pid != "" {
		val, err := strconv.ParseInt(pid, 10, 64)
		if err != nil {
			http.Error(w, "invalid projectId", http.StatusBadRequest)
			return
		}
		projectID = val
	}

	tasks, err := s.fetchTasks(projectID, "")
	if err != nil {
		http.Error(w, "failed to load tasks", http.StatusInternalServerError)
		return
	}

	writeJSON(w, tasks)
}

func (s *server) fetchTasks(projectID int64, status string) ([]task, error) {
	query := `SELECT id, title, status, comment, project_id, created_at FROM tasks`
	conds := make([]string, 0)
	args := make([]any, 0)

	if projectID > 0 {
		conds = append(conds, "project_id = ?")
		args = append(args, projectID)
	}
	if status != "" {
		conds = append(conds, "status = ?")
		args = append(args, status)
	}
	if len(conds) > 0 {
		query += " WHERE " + strings.Join(conds, " AND ")
	}
	query += " ORDER BY created_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := make([]task, 0)
	for rows.Next() {
		var t task
		var created time.Time
		if err := rows.Scan(&t.ID, &t.Title, &t.Status, &t.Comment, &t.ProjectID, &created); err != nil {
			return nil, err
		}
		t.CreatedAt = created.UTC()
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func (s *server) createTask(w http.ResponseWriter, r *http.Request) {
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
		payload.ProjectID = defaultProjectID
	}
	if payload.Title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}

	created, err := s.insertTask(payload.Title, payload.Comment, payload.ProjectID)
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

func (s *server) updateStatus(w http.ResponseWriter, r *http.Request, id int64) {
	var payload struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	payload.Status = strings.TrimSpace(payload.Status)

	updated, err := s.setTaskStatus(id, payload.Status)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}
	if errors.Is(err, errInvalidStatus) {
		http.Error(w, "invalid status", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, "failed to update task", http.StatusInternalServerError)
		return
	}

	writeJSON(w, updated)
}

func (s *server) updateComment(w http.ResponseWriter, r *http.Request, id int64) {
	var payload struct {
		Comment string `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	payload.Comment = strings.TrimSpace(payload.Comment)

	updated, err := s.setTaskComment(id, payload.Comment)
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

func (s *server) deleteTaskHandler(w http.ResponseWriter, _ *http.Request, id int64) {
	if err := s.deleteTask(id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "task not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to delete task", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleLogin(w http.ResponseWriter, r *http.Request) {
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
	u, err := s.getUserByEmail(payload.Email)
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

func (s *server) handleRegister(w http.ResponseWriter, r *http.Request) {
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
	u, err := s.createUser(payload.Email, payload.Password, "user")
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

func (s *server) handleMe(w http.ResponseWriter, r *http.Request) {
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

func (s *server) handleLogout(w http.ResponseWriter, _ *http.Request) {
	clearAuthCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) startTelegramBot(ctx context.Context) {
	token := strings.TrimSpace(os.Getenv("BOT_TOKEN"))
	chatIDStr := strings.TrimSpace(os.Getenv("BOT_CHAT_ID"))
	if token == "" || chatIDStr == "" {
		log.Printf("telegram bot is disabled: BOT_TOKEN or BOT_CHAT_ID not set")
		return
	}

	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		log.Printf("telegram bot disabled: invalid BOT_CHAT_ID: %v", err)
		return
	}

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Printf("telegram bot disabled: %v", err)
		return
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30

	updates := bot.GetUpdatesChan(u)
	log.Printf("telegram bot started for chat %d", chatID)

	for {
		select {
		case <-ctx.Done():
			bot.StopReceivingUpdates()
			return
		case update, ok := <-updates:
			if !ok {
				return
			}
			if update.Message == nil || update.Message.Chat == nil {
				continue
			}
			if update.Message.Chat.ID != chatID {
				continue
			}
			s.handleBotMessage(bot, chatID, update.Message)
		}
	}
}

func (s *server) handleBotMessage(bot *tgbotapi.BotAPI, chatID int64, msg *tgbotapi.Message) {
	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return
	}

	cmd, rest := splitCommand(text)
	switch cmd {
	case "/start", "/help":
		reply := "LiteTask бот\n\n" +
			"Команды:\n" +
			"/new [projectId] <название> |комментарий — создать задачу в проекте (по умолчанию Общий)\n" +
			"/status <id> <new|in_progress|done> — сменить статус\n" +
			"/list [projectId] [all] — показать задачи (по умолчанию новые задачи в Общем, all — все статусы, projectId=all — все проекты)\n" +
			"/projects — список проектов\n" +
			"/project <название> — создать проект"
		sendBotMessage(bot, chatID, reply)
	case "/new", "/add":
		projectID := int64(defaultProjectID)
		content := rest
		fields := strings.Fields(rest)
		if len(fields) > 0 {
			if val, err := strconv.ParseInt(fields[0], 10, 64); err == nil {
				projectID = val
				content = strings.TrimSpace(strings.TrimPrefix(rest, fields[0]))
			}
		}
		if content == "" {
			sendBotMessage(bot, chatID, "Используй: /new [projectId] <название> |комментарий (комментарий необязателен)")
			return
		}
		title, comment := parseTitleAndComment(content)
		if title == "" {
			sendBotMessage(bot, chatID, "Название задачи не может быть пустым")
			return
		}
		if ok, _ := s.projectExists(projectID); !ok {
			sendBotMessage(bot, chatID, "Проект не найден")
			return
		}

		t, err := s.insertTask(title, comment, projectID)
		if err != nil {
			log.Printf("bot: failed to insert task: %v", err)
			sendBotMessage(bot, chatID, "Не удалось создать задачу")
			return
		}
		projectName := s.lookupProjectName(projectID)
		sendBotMessage(bot, chatID, fmt.Sprintf("Создана #%d (%s) [%s]: %s", t.ID, projectName, statusTitles[t.Status], t.Title))
	case "/status", "/move":
		parts := strings.Fields(rest)
		if len(parts) < 2 {
			sendBotMessage(bot, chatID, "Используй: /status <id> <new|in_progress|done>")
			return
		}
		taskID, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			sendBotMessage(bot, chatID, "ID задачи должен быть числом")
			return
		}
		status := strings.ToLower(strings.TrimSpace(parts[1]))
		t, err := s.setTaskStatus(taskID, status)
		if errors.Is(err, errInvalidStatus) {
			sendBotMessage(bot, chatID, "Недопустимый статус. Используй new, in_progress или done.")
			return
		}
		if errors.Is(err, sql.ErrNoRows) {
			sendBotMessage(bot, chatID, "Задача не найдена")
			return
		}
		if err != nil {
			log.Printf("bot: failed to update status: %v", err)
			sendBotMessage(bot, chatID, "Не удалось обновить статус")
			return
		}
		projectName := s.lookupProjectName(t.ProjectID)
		sendBotMessage(bot, chatID, fmt.Sprintf("Статус задачи #%d (%s) теперь [%s]", t.ID, projectName, statusTitles[t.Status]))
	case "/list":
		projectID := int64(defaultProjectID)
		statusFilter := "new"
		fields := strings.Fields(rest)
		if len(fields) > 0 {
			if strings.ToLower(fields[0]) == "all" {
				projectID = 0
				statusFilter = ""
			} else if val, err := strconv.ParseInt(fields[0], 10, 64); err == nil {
				projectID = val
				if len(fields) > 1 && strings.ToLower(fields[1]) == "all" {
					statusFilter = ""
				}
			}
		}

		tasks, err := s.fetchTasks(projectID, statusFilter)
		if err != nil {
			log.Printf("bot: failed to fetch tasks: %v", err)
			sendBotMessage(bot, chatID, "Не удалось получить список задач")
			return
		}
		if len(tasks) == 0 {
			sendBotMessage(bot, chatID, "Задач пока нет")
			return
		}
		var b strings.Builder
		title := "Задачи:"
		if statusFilter == "new" {
			title = "Новые задачи:"
		}
		if projectID == 0 {
			title += " (все проекты)"
		} else {
			title += fmt.Sprintf(" (проект %s)", s.lookupProjectName(projectID))
		}
		if statusFilter == "" {
			title += " (все статусы)"
		}
		b.WriteString(title + "\n")
		projNames := s.projectNameMap()
		for _, t := range tasks {
			name := projNames[t.ProjectID]
			fmt.Fprintf(&b, "#%d (%s) [%s] %s\n", t.ID, name, statusTitles[t.Status], t.Title)
		}
		sendBotMessage(bot, chatID, b.String())
	case "/projects":
		projects, err := s.listProjectsDB()
		if err != nil {
			log.Printf("bot: failed to list projects: %v", err)
			sendBotMessage(bot, chatID, "Не удалось получить проекты")
			return
		}
		if len(projects) == 0 {
			sendBotMessage(bot, chatID, "Проектов пока нет")
			return
		}
		var b strings.Builder
		b.WriteString("Проекты:\n")
		for _, p := range projects {
			fmt.Fprintf(&b, "%d — %s\n", p.ID, p.Name)
		}
		sendBotMessage(bot, chatID, b.String())
	case "/project":
		if rest == "" {
			sendBotMessage(bot, chatID, "Используй: /project <название>")
			return
		}
		p, err := s.createProject(strings.TrimSpace(rest))
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "unique") {
				sendBotMessage(bot, chatID, "Проект с таким названием уже существует")
				return
			}
			log.Printf("bot: failed to create project: %v", err)
			sendBotMessage(bot, chatID, "Не удалось создать проект")
			return
		}
		sendBotMessage(bot, chatID, fmt.Sprintf("Проект создан: #%d %s", p.ID, p.Name))
	default:
		sendBotMessage(bot, chatID, "Неизвестная команда. Отправь /help для подсказки.")
	}
}

func splitCommand(text string) (string, string) {
	parts := strings.SplitN(text, " ", 2)
	cmd := strings.ToLower(parts[0])
	if len(parts) == 1 {
		return cmd, ""
	}
	return cmd, strings.TrimSpace(parts[1])
}

func parseTitleAndComment(input string) (string, string) {
	parts := strings.SplitN(input, "|", 2)
	title := strings.TrimSpace(parts[0])
	if len(parts) == 2 {
		return title, strings.TrimSpace(parts[1])
	}
	return title, ""
}

func sendBotMessage(bot *tgbotapi.BotAPI, chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := bot.Send(msg); err != nil {
		log.Printf("failed to send bot message: %v", err)
	}
}

func (s *server) projectNameMap() map[int64]string {
	projects, err := s.listProjectsDB()
	result := make(map[int64]string, len(projects))
	if err != nil {
		return result
	}
	for _, p := range projects {
		result[p.ID] = p.Name
	}
	return result
}

func (s *server) lookupProjectName(id int64) string {
	names := s.projectNameMap()
	if name, ok := names[id]; ok {
		return name
	}
	if id == defaultProjectID {
		return defaultProjectName
	}
	return fmt.Sprintf("Проект %d", id)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	if err := enc.Encode(v); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

func (s *server) staticHandler(distPath string) http.Handler {
	abs, err := filepath.Abs(distPath)
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
