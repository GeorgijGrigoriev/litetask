package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"litetask/internal/config"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

const (
	DefaultDBPath      = "tasks.db"
	DefaultProjectID   = 1
	DefaultProjectName = "Общий"
)

var (
	allowedStatuses = map[string]struct{}{
		"new":         {},
		"in_progress": {},
		"done":        {},
	}
	allowedRoles = map[string]struct{}{
		"admin":   {},
		"user":    {},
		"blocked": {},
	}
	StatusTitles = map[string]string{
		"new":         "Новая",
		"in_progress": "В работе",
		"done":        "Готова",
	}
	ErrInvalidStatus = errors.New("invalid status")
	ErrInvalidRole   = errors.New("invalid role")
	ErrLastAdmin     = errors.New("cannot remove last admin")
)

type Task struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	Comment   string    `json:"comment"`
	ProjectID int64     `json:"projectId"`
	CreatedAt time.Time `json:"createdAt"`
}

type Project struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

type User struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	Password  string    `json:"-"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
}

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	if err := setupSchema(db); err != nil {
		db.Close()
		return nil, err
	}

	if err := ensureDefaultProject(db); err != nil {
		log.Printf("warning: unable to ensure default project: %v", err)
	}
	if err := ensureAdminUser(db); err != nil {
		log.Printf("warning: unable to ensure admin user: %v", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) InsertTask(title, comment string, projectID int64) (Task, error) {
	var t Task
	ok, err := s.ProjectExists(projectID)
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

func (s *Store) SetTaskStatus(id int64, status string) (Task, error) {
	var t Task
	if _, ok := allowedStatuses[status]; !ok {
		return t, ErrInvalidStatus
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

func (s *Store) SetTaskComment(id int64, comment string) (Task, error) {
	var t Task
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

func (s *Store) DeleteTask(id int64) error {
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

func (s *Store) ProjectExists(id int64) (bool, error) {
	var exists bool
	err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM projects WHERE id = ?)`, id).Scan(&exists)
	return exists, err
}

func (s *Store) GetTask(id int64) (Task, error) {
	var t Task
	err := s.db.QueryRow(`SELECT id, title, status, comment, project_id, created_at FROM tasks WHERE id = ?`, id).
		Scan(&t.ID, &t.Title, &t.Status, &t.Comment, &t.ProjectID, &t.CreatedAt)
	if err != nil {
		return t, err
	}
	t.CreatedAt = t.CreatedAt.UTC()
	return t, nil
}

func (s *Store) CreateProject(name string) (Project, error) {
	var p Project
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

func (s *Store) ListProjects() ([]Project, error) {
	rows, err := s.db.Query(`SELECT id, name, created_at FROM projects ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	projects := make([]Project, 0)
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.CreatedAt); err != nil {
			return nil, err
		}
		p.CreatedAt = p.CreatedAt.UTC()
		projects = append(projects, p)
	}
	return projects, nil
}

func (s *Store) DeleteProject(id int64) error {
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

func (s *Store) CreateUser(email, password, role string) (User, error) {
	var u User
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
	if err := s.SetUserProjects(u.ID, []int64{DefaultProjectID}); err != nil {
		log.Printf("warning: failed to assign default project: %v", err)
	}
	return u, nil
}

func (s *Store) GetUserByEmail(email string) (User, error) {
	var u User
	err := s.db.QueryRow(`SELECT id, email, password_hash, role, created_at FROM users WHERE email = ?`, email).
		Scan(&u.ID, &u.Email, &u.Password, &u.Role, &u.CreatedAt)
	if err != nil {
		return u, err
	}
	u.CreatedAt = u.CreatedAt.UTC()
	return u, nil
}

func (s *Store) GetUserByID(id int64) (User, error) {
	var u User
	err := s.db.QueryRow(`SELECT id, email, password_hash, role, created_at FROM users WHERE id = ?`, id).
		Scan(&u.ID, &u.Email, &u.Password, &u.Role, &u.CreatedAt)
	if err != nil {
		return u, err
	}
	u.CreatedAt = u.CreatedAt.UTC()
	return u, nil
}

func (s *Store) ListUsers() ([]User, error) {
	rows, err := s.db.Query(`SELECT id, email, password_hash, role, created_at FROM users ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := make([]User, 0)
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.Password, &u.Role, &u.CreatedAt); err != nil {
			return nil, err
		}
		u.CreatedAt = u.CreatedAt.UTC()
		users = append(users, u)
	}
	return users, nil
}

func (s *Store) UpdateUserRole(id int64, role string) (User, error) {
	if _, ok := allowedRoles[role]; !ok {
		return User{}, ErrInvalidRole
	}

	var currentRole string
	if err := s.db.QueryRow(`SELECT role FROM users WHERE id = ?`, id).Scan(&currentRole); err != nil {
		return User{}, err
	}

	if currentRole == "admin" && role != "admin" {
		count, err := s.countAdmins()
		if err != nil {
			return User{}, err
		}
		if count <= 1 {
			return User{}, ErrLastAdmin
		}
	}

	res, err := s.db.Exec(`UPDATE users SET role = ? WHERE id = ?`, role, id)
	if err != nil {
		return User{}, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return User{}, sql.ErrNoRows
	}

	return s.GetUserByID(id)
}

func (s *Store) countAdmins() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users WHERE role = 'admin'`).Scan(&count)
	return count, err
}

func (s *Store) UpdateUserPassword(id int64, password string) (User, error) {
	if len(password) < 6 {
		return User{}, errors.New("password too short")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, err
	}
	res, err := s.db.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, string(hash), id)
	if err != nil {
		return User{}, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return User{}, sql.ErrNoRows
	}
	return s.GetUserByID(id)
}

func (s *Store) SetUserProjects(userID int64, projectIDs []int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, pid := range projectIDs {
		ok, err := s.projectExistsTx(tx, pid)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("project not found")
		}
	}

	if _, err := tx.Exec(`DELETE FROM user_projects WHERE user_id = ?`, userID); err != nil {
		return err
	}
	for _, pid := range projectIDs {
		if _, err := tx.Exec(`INSERT INTO user_projects (user_id, project_id) VALUES (?, ?)`, userID, pid); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) GetUserProjects(userID int64) ([]int64, error) {
	rows, err := s.db.Query(`SELECT project_id FROM user_projects WHERE user_id = ? ORDER BY project_id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (s *Store) projectExistsTx(tx *sql.Tx, id int64) (bool, error) {
	var exists bool
	err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM projects WHERE id = ?)`, id).Scan(&exists)
	return exists, err
}

func (s *Store) FetchTasks(projectID int64, status string, allowed map[int64]struct{}) ([]Task, error) {
	query := `SELECT id, title, status, comment, project_id, created_at FROM tasks`
	conds := make([]string, 0)
	args := make([]any, 0)

	if projectID > 0 {
		conds = append(conds, "project_id = ?")
		args = append(args, projectID)
	}
	if len(allowed) > 0 {
		placeholders := make([]string, 0, len(allowed))
		for pid := range allowed {
			placeholders = append(placeholders, "?")
			args = append(args, pid)
		}
		conds = append(conds, "project_id IN ("+strings.Join(placeholders, ",")+")")
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

	tasks := make([]Task, 0)
	for rows.Next() {
		var t Task
		var created time.Time
		if err := rows.Scan(&t.ID, &t.Title, &t.Status, &t.Comment, &t.ProjectID, &created); err != nil {
			return nil, err
		}
		t.CreatedAt = created.UTC()
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func (s *Store) ProjectNameMap() map[int64]string {
	projects, err := s.ListProjects()
	result := make(map[int64]string, len(projects))
	if err != nil {
		return result
	}
	for _, p := range projects {
		result[p.ID] = p.Name
	}
	return result
}

func (s *Store) LookupProjectName(id int64) string {
	names := s.ProjectNameMap()
	if name, ok := names[id]; ok {
		return name
	}
	if id == DefaultProjectID {
		return DefaultProjectName
	}
	return fmt.Sprintf("Проект %d", id)
}

func setupSchema(db *sql.DB) error {
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
CREATE TABLE IF NOT EXISTS user_projects (
	user_id INTEGER NOT NULL,
	project_id INTEGER NOT NULL,
	PRIMARY KEY (user_id, project_id),
	FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
	FOREIGN KEY(project_id) REFERENCES projects(id) ON DELETE CASCADE
);
`
	if _, err := db.Exec(schema); err != nil {
		return err
	}

	if _, err := db.Exec(`ALTER TABLE tasks ADD COLUMN comment TEXT DEFAULT ''`); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			log.Printf("warning: unable to add comment column: %v", err)
		}
	}

	if _, err := db.Exec(`ALTER TABLE tasks ADD COLUMN project_id INTEGER NOT NULL DEFAULT 1`); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			log.Printf("warning: unable to add project_id column: %v", err)
		}
	}
	if _, err := db.Exec(`UPDATE tasks SET project_id = ? WHERE project_id IS NULL OR project_id = 0`, DefaultProjectID); err != nil {
		log.Printf("warning: unable to backfill project_id: %v", err)
	}

	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_tasks_project ON tasks(project_id)`); err != nil {
		log.Printf("warning: unable to ensure idx_tasks_project: %v", err)
	}

	return nil
}

func ensureDefaultProject(db *sql.DB) error {
	if _, err := db.Exec(`INSERT OR IGNORE INTO projects (id, name) VALUES (?, ?)`, DefaultProjectID, DefaultProjectName); err != nil {
		return err
	}
	_, err := db.Exec(`UPDATE projects SET name = ? WHERE id = ? AND name != ?`, DefaultProjectName, DefaultProjectID, DefaultProjectName)
	return err
}

func ensureAdminUser(db *sql.DB) error {
	adminEmail := config.EnvOrDefault("ADMIN_EMAIL", "admin@example.com")
	adminPassword := os.Getenv("ADMIN_PASSWORD")

	var existing User
	err := db.QueryRow(`SELECT id, email FROM users WHERE role = 'admin' ORDER BY id LIMIT 1`).Scan(&existing.ID, &existing.Email)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}

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
	if _, err := db.Exec(`INSERT OR IGNORE INTO user_projects (user_id, project_id) VALUES ((SELECT id FROM users WHERE email = ?), ?)`, adminEmail, DefaultProjectID); err != nil {
		log.Printf("warning: failed to assign default project to admin: %v", err)
	}
	return nil
}

func randomPassword() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "changeme123"
	}
	return base64.RawStdEncoding.EncodeToString(b)
}
