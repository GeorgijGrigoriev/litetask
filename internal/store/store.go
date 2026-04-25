package store

import (
	"context"
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
	DefaultDBPath           = "tasks.db"
	DefaultProjectID        = 1
	DefaultProjectName      = "Общий"
	DefaultInboxProjectName = "Входящие"
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
	ErrInvalidStatus    = errors.New("invalid status")
	ErrInvalidPriority  = errors.New("invalid priority")
	ErrInvalidRole      = errors.New("invalid role")
	ErrLastAdmin        = errors.New("cannot remove last admin")
	ErrUsernameSet      = errors.New("username already set")
	ErrProtectedProject = errors.New("cannot delete protected project")
)

var allowedPriorities = map[string]struct{}{
	"high":   {},
	"medium": {},
	"low":    {},
}

type Task struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Status      string    `json:"status"`
	Priority    string    `json:"priority"`
	Description string    `json:"description"`
	ProjectID   int64     `json:"projectId"`
	CreatedAt   time.Time `json:"createdAt"`
	CreatedBy   int64     `json:"createdBy"`
	AuthorEmail string    `json:"authorEmail"`
	AuthorFirst string    `json:"authorFirstName,omitempty"`
	AuthorLast  string    `json:"authorLastName,omitempty"`
}

type TaskComment struct {
	ID          int64     `json:"id"`
	TaskID      int64     `json:"taskId"`
	Body        string    `json:"body"`
	AuthorID    int64     `json:"authorId,omitempty"`
	AuthorEmail string    `json:"authorEmail"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Project struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	IsInbox   bool      `json:"isInbox"`
	OwnerID   int64     `json:"-"`
	CreatedAt time.Time `json:"createdAt"`
}

type User struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	Username  string    `json:"username"`
	Password  string    `json:"-"`
	Role      string    `json:"role"`
	FirstName string    `json:"firstName"`
	LastName  string    `json:"lastName"`
	Telegram  string    `json:"telegram"`
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
		_ = db.Close()
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

func (s *Store) InsertTask(ctx context.Context, title, description, authorLabel string, projectID, createdBy int64, priority string) (Task, error) {
	var t Task
	ok, err := s.ProjectExists(ctx, projectID)
	if err != nil {
		return t, err
	}
	if !ok {
		return t, fmt.Errorf("project not found")
	}
	if _, ok := allowedPriorities[priority]; !ok {
		return t, ErrInvalidPriority
	}

	res, err := s.db.ExecContext(ctx,
		`INSERT INTO tasks (title, status, priority, description, author_label, project_id, created_by) VALUES (?, 'new', ?, ?, ?, ?, ?)`,
		title,
		priority,
		description,
		sql.NullString{String: authorLabel, Valid: authorLabel != ""},
		projectID,
		nullableInt64(createdBy),
	)
	if err != nil {
		return t, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return t, fmt.Errorf("lastInsertId: %w", err)
	}
	return s.scanTask(ctx, id)
}

func (s *Store) SetTaskStatus(ctx context.Context, id int64, status string) (Task, error) {
	var t Task
	if _, ok := allowedStatuses[status]; !ok {
		return t, ErrInvalidStatus
	}

	res, err := s.db.ExecContext(ctx, `UPDATE tasks SET status = ? WHERE id = ?`, status, id)
	if err != nil {
		return t, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return t, fmt.Errorf("rowsAffected: %w", err)
	}
	if affected == 0 {
		return t, sql.ErrNoRows
	}
	return s.scanTask(ctx, id)
}

func (s *Store) SetTaskPriority(ctx context.Context, id int64, priority string) (Task, error) {
	if _, ok := allowedPriorities[priority]; !ok {
		return Task{}, ErrInvalidPriority
	}
	res, err := s.db.ExecContext(ctx, `UPDATE tasks SET priority = ? WHERE id = ?`, priority, id)
	if err != nil {
		return Task{}, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return Task{}, fmt.Errorf("rowsAffected: %w", err)
	}
	if affected == 0 {
		return Task{}, sql.ErrNoRows
	}
	return s.scanTask(ctx, id)
}

func (s *Store) SetTaskDescription(ctx context.Context, id int64, description string) (Task, error) {
	var t Task
	res, err := s.db.ExecContext(ctx, `UPDATE tasks SET description = ? WHERE id = ?`, description, id)
	if err != nil {
		return t, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return t, fmt.Errorf("rowsAffected: %w", err)
	}
	if affected == 0 {
		return t, sql.ErrNoRows
	}
	return s.scanTask(ctx, id)
}

func (s *Store) MoveTask(ctx context.Context, taskID, newProjectID int64) (Task, error) {
	ok, err := s.ProjectExists(ctx, newProjectID)
	if err != nil {
		return Task{}, err
	}
	if !ok {
		return Task{}, fmt.Errorf("project not found")
	}
	res, err := s.db.ExecContext(ctx, `UPDATE tasks SET project_id = ? WHERE id = ?`, newProjectID, taskID)
	if err != nil {
		return Task{}, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return Task{}, fmt.Errorf("rowsAffected: %w", err)
	}
	if affected == 0 {
		return Task{}, sql.ErrNoRows
	}
	return s.scanTask(ctx, taskID)
}

func (s *Store) DeleteTask(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM tasks WHERE id = ?`, id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rowsAffected: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) ProjectExists(ctx context.Context, id int64) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM projects WHERE id = ?)`, id).Scan(&exists)
	return exists, err
}

func (s *Store) GetTask(ctx context.Context, id int64) (Task, error) {
	return s.scanTask(ctx, id)
}

// scanTask fetches a single task by ID including author info.
func (s *Store) scanTask(ctx context.Context, id int64) (Task, error) {
	var t Task
	var created sql.NullInt64
	var email, first, last sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT t.id, t.title, t.status, t.priority, COALESCE(t.description, t.comment, ''), t.project_id, t.created_at, t.created_by, COALESCE(t.author_label, u.email, ''), u.first_name, u.last_name
			FROM tasks t
			LEFT JOIN users u ON t.created_by = u.id
			WHERE t.id = ?`,
		id,
	).Scan(&t.ID, &t.Title, &t.Status, &t.Priority, &t.Description, &t.ProjectID, &t.CreatedAt, &created, &email, &first, &last)
	if err != nil {
		return t, err
	}
	t.CreatedAt = t.CreatedAt.UTC()
	if created.Valid {
		t.CreatedBy = created.Int64
	}
	if email.Valid {
		t.AuthorEmail = email.String
	}
	if first.Valid {
		t.AuthorFirst = first.String
	}
	if last.Valid {
		t.AuthorLast = last.String
	}
	return t, nil
}

func (s *Store) CreateProject(ctx context.Context, name string) (Project, error) {
	var p Project
	res, err := s.db.ExecContext(ctx, `INSERT INTO projects (name) VALUES (?)`, name)
	if err != nil {
		return p, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return p, fmt.Errorf("lastInsertId: %w", err)
	}
	err = s.db.QueryRowContext(ctx, `SELECT id, name, created_at FROM projects WHERE id = ?`, id).
		Scan(&p.ID, &p.Name, &p.CreatedAt)
	if err != nil {
		return p, err
	}
	p.CreatedAt = p.CreatedAt.UTC()
	return p, nil
}

func (s *Store) ListProjects(ctx context.Context) ([]Project, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, COALESCE(is_inbox, 0), COALESCE(owner_id, 0), created_at FROM projects ORDER BY is_inbox DESC, created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	projects := make([]Project, 0)
	for rows.Next() {
		var p Project
		var isInbox int
		if err := rows.Scan(&p.ID, &p.Name, &isInbox, &p.OwnerID, &p.CreatedAt); err != nil {
			return nil, err
		}
		p.CreatedAt = p.CreatedAt.UTC()
		p.IsInbox = isInbox == 1
		projects = append(projects, p)
	}
	return projects, nil
}

func (s *Store) DeleteProject(ctx context.Context, id int64) error {
	if id == DefaultProjectID {
		return ErrProtectedProject
	}
	var isInbox int
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(is_inbox, 0) FROM projects WHERE id = ?`, id).Scan(&isInbox); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sql.ErrNoRows
		}
		return err
	}
	if isInbox == 1 {
		return ErrProtectedProject
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.ExecContext(ctx, `DELETE FROM tasks WHERE project_id = ?`, id); err != nil {
		return err
	}

	res, err := tx.ExecContext(ctx, `DELETE FROM projects WHERE id = ?`, id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rowsAffected: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}

	return tx.Commit()
}

func (s *Store) CreateUser(ctx context.Context, email, username, password, role, firstName, lastName string) (User, error) {
	var u User
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return u, err
	}
	firstName = strings.TrimSpace(firstName)
	lastName = strings.TrimSpace(lastName)
	username = strings.TrimSpace(strings.ToLower(username))
	if username != "" {
		if err := validateUsername(username); err != nil {
			return u, err
		}
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO users (email, username, password_hash, role, first_name, last_name, telegram) VALUES (?, ?, ?, ?, ?, ?, '')`,
		email,
		nullableString(username),
		string(hash),
		role,
		firstName,
		lastName,
	)
	if err != nil {
		return u, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return u, fmt.Errorf("lastInsertId: %w", err)
	}
	err = s.db.QueryRowContext(ctx, `SELECT id, email, COALESCE(username, ''), password_hash, role, created_at, telegram, first_name, last_name FROM users WHERE id = ?`, id).
		Scan(&u.ID, &u.Email, &u.Username, &u.Password, &u.Role, &u.CreatedAt, &u.Telegram, &u.FirstName, &u.LastName)
	if err != nil {
		return u, err
	}
	u.CreatedAt = u.CreatedAt.UTC()
	projectIDs := []int64{DefaultProjectID}
	if inboxID, err := s.EnsureUserInbox(ctx, u.ID); err != nil {
		log.Printf("warning: failed to create inbox project: %v", err)
	} else if inboxID > 0 {
		projectIDs = append(projectIDs, inboxID)
	}
	if err := s.SetUserProjects(ctx, u.ID, projectIDs); err != nil {
		log.Printf("warning: failed to assign default projects: %v", err)
	}
	return u, nil
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx, `SELECT id, email, COALESCE(username, ''), password_hash, role, created_at, telegram, first_name, last_name FROM users WHERE email = ?`, email).
		Scan(&u.ID, &u.Email, &u.Username, &u.Password, &u.Role, &u.CreatedAt, &u.Telegram, &u.FirstName, &u.LastName)
	if err != nil {
		return u, err
	}
	u.CreatedAt = u.CreatedAt.UTC()
	return u, nil
}

func (s *Store) GetUserByEmailOrUsername(ctx context.Context, login string) (User, error) {
	var u User
	login = strings.TrimSpace(strings.ToLower(login))
	err := s.db.QueryRowContext(ctx,
		`SELECT id, email, COALESCE(username, ''), password_hash, role, created_at, telegram, first_name, last_name
		FROM users
		WHERE email = ? OR username = ?
		ORDER BY id
		LIMIT 1`,
		login,
		login,
	).Scan(&u.ID, &u.Email, &u.Username, &u.Password, &u.Role, &u.CreatedAt, &u.Telegram, &u.FirstName, &u.LastName)
	if err != nil {
		return u, err
	}
	u.CreatedAt = u.CreatedAt.UTC()
	return u, nil
}

func (s *Store) GetUserByID(ctx context.Context, id int64) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx, `SELECT id, email, COALESCE(username, ''), password_hash, role, created_at, telegram, first_name, last_name FROM users WHERE id = ?`, id).
		Scan(&u.ID, &u.Email, &u.Username, &u.Password, &u.Role, &u.CreatedAt, &u.Telegram, &u.FirstName, &u.LastName)
	if err != nil {
		return u, err
	}
	u.CreatedAt = u.CreatedAt.UTC()
	return u, nil
}

func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, email, COALESCE(username, ''), password_hash, role, created_at, telegram, first_name, last_name FROM users ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	users := make([]User, 0)
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.Username, &u.Password, &u.Role, &u.CreatedAt, &u.Telegram, &u.FirstName, &u.LastName); err != nil {
			return nil, err
		}
		u.CreatedAt = u.CreatedAt.UTC()
		users = append(users, u)
	}
	return users, nil
}

func (s *Store) SetUsernameOnce(ctx context.Context, id int64, username string) (User, error) {
	username = strings.TrimSpace(strings.ToLower(username))
	if username == "" {
		return User{}, errors.New("username required")
	}
	if err := validateUsername(username); err != nil {
		return User{}, err
	}

	res, err := s.db.ExecContext(ctx,
		`UPDATE users
		SET username = ?
		WHERE id = ? AND (username IS NULL OR username = '')`,
		username,
		id,
	)
	if err != nil {
		return User{}, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return User{}, fmt.Errorf("rowsAffected: %w", err)
	}
	if affected == 0 {
		var current sql.NullString
		if err := s.db.QueryRowContext(ctx, `SELECT username FROM users WHERE id = ?`, id).Scan(&current); err != nil {
			return User{}, err
		}
		if current.Valid && strings.TrimSpace(current.String) != "" {
			return User{}, ErrUsernameSet
		}
		return User{}, sql.ErrNoRows
	}
	return s.GetUserByID(ctx, id)
}

func (s *Store) UpdateUserRole(ctx context.Context, id int64, role string) (User, error) {
	if _, ok := allowedRoles[role]; !ok {
		return User{}, ErrInvalidRole
	}

	var currentRole string
	if err := s.db.QueryRowContext(ctx, `SELECT role FROM users WHERE id = ?`, id).Scan(&currentRole); err != nil {
		return User{}, err
	}

	if currentRole == "admin" && role != "admin" {
		count, err := s.countAdmins(ctx)
		if err != nil {
			return User{}, err
		}
		if count <= 1 {
			return User{}, ErrLastAdmin
		}
	}

	res, err := s.db.ExecContext(ctx, `UPDATE users SET role = ? WHERE id = ?`, role, id)
	if err != nil {
		return User{}, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return User{}, fmt.Errorf("rowsAffected: %w", err)
	}
	if affected == 0 {
		return User{}, sql.ErrNoRows
	}

	return s.GetUserByID(ctx, id)
}

func (s *Store) countAdmins(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role = 'admin'`).Scan(&count)
	return count, err
}

func (s *Store) UpdateUserPassword(ctx context.Context, id int64, password string) (User, error) {
	if len(password) < 6 {
		return User{}, errors.New("password too short")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, err
	}
	res, err := s.db.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, string(hash), id)
	if err != nil {
		return User{}, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return User{}, fmt.Errorf("rowsAffected: %w", err)
	}
	if affected == 0 {
		return User{}, sql.ErrNoRows
	}
	return s.GetUserByID(ctx, id)
}

func (s *Store) UpdateUserProfile(ctx context.Context, id int64, password *string, telegram *string, firstName *string, lastName *string) (User, error) {
	if password == nil && telegram == nil && firstName == nil && lastName == nil {
		return s.GetUserByID(ctx, id)
	}
	sets := make([]string, 0)
	args := make([]any, 0)

	if password != nil {
		if len(*password) < 6 {
			return User{}, errors.New("password too short")
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
		if err != nil {
			return User{}, err
		}
		sets = append(sets, "password_hash = ?")
		args = append(args, string(hash))
	}

	if telegram != nil {
		sets = append(sets, "telegram = ?")
		args = append(args, strings.TrimSpace(*telegram))
	}

	if firstName != nil {
		sets = append(sets, "first_name = ?")
		args = append(args, strings.TrimSpace(*firstName))
	}

	if lastName != nil {
		sets = append(sets, "last_name = ?")
		args = append(args, strings.TrimSpace(*lastName))
	}

	args = append(args, id)

	query := `UPDATE users SET ` + strings.Join(sets, ", ") + ` WHERE id = ?`
	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return User{}, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return User{}, fmt.Errorf("rowsAffected: %w", err)
	}
	if affected == 0 {
		return User{}, sql.ErrNoRows
	}
	return s.GetUserByID(ctx, id)
}

func (s *Store) SetUserProjects(ctx context.Context, userID int64, projectIDs []int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	for _, pid := range projectIDs {
		ok, err := s.projectExistsTx(ctx, tx, pid)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("project not found")
		}
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM user_projects WHERE user_id = ?`, userID); err != nil {
		return err
	}
	for _, pid := range projectIDs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO user_projects (user_id, project_id) VALUES (?, ?)`, userID, pid); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) GetUserProjects(ctx context.Context, userID int64) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT project_id FROM user_projects WHERE user_id = ? ORDER BY project_id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
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

func (s *Store) projectExistsTx(ctx context.Context, tx *sql.Tx, id int64) (bool, error) {
	var exists bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM projects WHERE id = ?)`, id).Scan(&exists)
	return exists, err
}

func (s *Store) FetchTasks(ctx context.Context, projectID int64, status string, allowed map[int64]struct{}) ([]Task, error) {
	query := `SELECT t.id, t.title, t.status, t.priority, COALESCE(t.description, t.comment, ''), t.project_id, t.created_at, t.created_by, COALESCE(t.author_label, u.email, ''), u.first_name, u.last_name FROM tasks t LEFT JOIN users u ON t.created_by = u.id`
	conds := make([]string, 0)
	args := make([]any, 0)

	if projectID > 0 {
		conds = append(conds, "t.project_id = ?")
		args = append(args, projectID)
	}
	if len(allowed) > 0 {
		placeholders := make([]string, 0, len(allowed))
		for pid := range allowed {
			placeholders = append(placeholders, "?")
			args = append(args, pid)
		}
		conds = append(conds, "t.project_id IN ("+strings.Join(placeholders, ",")+")")
	}
	if status != "" {
		conds = append(conds, "t.status = ?")
		args = append(args, status)
	}
	if len(conds) > 0 {
		query += " WHERE " + strings.Join(conds, " AND ")
	}
	query += " ORDER BY t.created_at DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck

	tasks := make([]Task, 0)
	for rows.Next() {
		var t Task
		var created time.Time
		var authorID sql.NullInt64
		var email, first, last sql.NullString
		if err := rows.Scan(&t.ID, &t.Title, &t.Status, &t.Priority, &t.Description, &t.ProjectID, &created, &authorID, &email, &first, &last); err != nil {
			return nil, err
		}
		t.CreatedAt = created.UTC()
		if authorID.Valid {
			t.CreatedBy = authorID.Int64
		}
		if email.Valid {
			t.AuthorEmail = email.String
		}
		if first.Valid {
			t.AuthorFirst = first.String
		}
		if last.Valid {
			t.AuthorLast = last.String
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func (s *Store) AddTaskComment(ctx context.Context, taskID int64, body string, authorID int64) (TaskComment, error) {
	var c TaskComment
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO task_comments (task_id, body, author_id) VALUES (?, ?, ?)`,
		taskID,
		body,
		nullableInt64(authorID),
	)
	if err != nil {
		return c, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return c, fmt.Errorf("lastInsertId: %w", err)
	}
	var created sql.NullInt64
	var email sql.NullString
	err = s.db.QueryRowContext(ctx,
		`SELECT c.id, c.task_id, c.body, c.author_id, c.created_at, u.email
		FROM task_comments c
		LEFT JOIN users u ON c.author_id = u.id
		WHERE c.id = ?`,
		id,
	).Scan(&c.ID, &c.TaskID, &c.Body, &created, &c.CreatedAt, &email)
	if err != nil {
		return c, err
	}
	c.CreatedAt = c.CreatedAt.UTC()
	if created.Valid {
		c.AuthorID = created.Int64
	}
	if email.Valid {
		c.AuthorEmail = email.String
	}
	return c, nil
}

func (s *Store) ListTaskComments(ctx context.Context, taskID int64) ([]TaskComment, error) {
	commentsMap, err := s.ListCommentsByTaskIDs(ctx, []int64{taskID})
	if err != nil {
		return nil, err
	}
	return commentsMap[taskID], nil
}

func (s *Store) GetTaskComment(ctx context.Context, commentID int64) (TaskComment, error) {
	var c TaskComment
	var author sql.NullInt64
	var email sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT c.id, c.task_id, c.body, c.author_id, c.created_at, u.email
		FROM task_comments c
		LEFT JOIN users u ON c.author_id = u.id
		WHERE c.id = ?`,
		commentID,
	).Scan(&c.ID, &c.TaskID, &c.Body, &author, &c.CreatedAt, &email)
	if err != nil {
		return c, err
	}
	c.CreatedAt = c.CreatedAt.UTC()
	if author.Valid {
		c.AuthorID = author.Int64
	}
	if email.Valid {
		c.AuthorEmail = email.String
	}
	return c, nil
}

func (s *Store) DeleteTaskComment(ctx context.Context, commentID int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM task_comments WHERE id = ?`, commentID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rowsAffected: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) ListCommentsByTaskIDs(ctx context.Context, taskIDs []int64) (map[int64][]TaskComment, error) {
	result := make(map[int64][]TaskComment, len(taskIDs))
	if len(taskIDs) == 0 {
		return result, nil
	}
	placeholders := make([]string, 0, len(taskIDs))
	args := make([]any, 0, len(taskIDs))
	for _, id := range taskIDs {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT c.id, c.task_id, c.body, c.author_id, c.created_at, u.email
		FROM task_comments c
		LEFT JOIN users u ON c.author_id = u.id
		WHERE c.task_id IN (`+strings.Join(placeholders, ",")+`)
		ORDER BY c.created_at ASC, c.id ASC`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck

	for rows.Next() {
		var c TaskComment
		var created sql.NullInt64
		var email sql.NullString
		if err := rows.Scan(&c.ID, &c.TaskID, &c.Body, &created, &c.CreatedAt, &email); err != nil {
			return nil, err
		}
		c.CreatedAt = c.CreatedAt.UTC()
		if created.Valid {
			c.AuthorID = created.Int64
		}
		if email.Valid {
			c.AuthorEmail = email.String
		}
		result[c.TaskID] = append(result[c.TaskID], c)
	}
	return result, nil
}

func (s *Store) ProjectNameMap(ctx context.Context) map[int64]string {
	projects, err := s.ListProjects(ctx)
	result := make(map[int64]string, len(projects))
	if err != nil {
		log.Printf("warning: ProjectNameMap: %v", err)
		return result
	}
	for _, p := range projects {
		result[p.ID] = p.Name
	}
	return result
}

func (s *Store) LookupProjectName(ctx context.Context, id int64) string {
	names := s.ProjectNameMap(ctx)
	if name, ok := names[id]; ok {
		return name
	}
	if id == DefaultProjectID {
		return DefaultProjectName
	}
	return fmt.Sprintf("Проект %d", id)
}

func (s *Store) GetUserInboxID(ctx context.Context, userID int64) int64 {
	var id int64
	_ = s.db.QueryRowContext(ctx, `SELECT id FROM projects WHERE is_inbox = 1 AND owner_id = ?`, userID).Scan(&id)
	return id
}

func (s *Store) EnsureUserInbox(ctx context.Context, userID int64) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `SELECT id FROM projects WHERE is_inbox = 1 AND owner_id = ?`, userID).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	inboxName := fmt.Sprintf("%s#%d", DefaultInboxProjectName, userID)
	res, err := s.db.ExecContext(ctx, `INSERT INTO projects (name, is_inbox, owner_id) VALUES (?, 1, ?)`, inboxName, userID)
	if err != nil {
		return 0, err
	}
	id, err = res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("lastInsertId: %w", err)
	}
	return id, nil
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
	username TEXT,
	password_hash TEXT NOT NULL,
	role TEXT NOT NULL DEFAULT 'user',
	first_name TEXT NOT NULL DEFAULT '',
	last_name TEXT NOT NULL DEFAULT '',
	telegram TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS tasks (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	title TEXT NOT NULL,
	status TEXT NOT NULL,
	priority TEXT NOT NULL DEFAULT 'medium',
	comment TEXT DEFAULT '',
	description TEXT DEFAULT '',
	project_id INTEGER NOT NULL DEFAULT 1,
	created_by INTEGER,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(project_id) REFERENCES projects(id) ON DELETE CASCADE,
	FOREIGN KEY(created_by) REFERENCES users(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_project ON tasks(project_id);
CREATE TABLE IF NOT EXISTS task_comments (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	task_id INTEGER NOT NULL,
	author_id INTEGER,
	body TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(task_id) REFERENCES tasks(id) ON DELETE CASCADE,
	FOREIGN KEY(author_id) REFERENCES users(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_task_comments_task ON task_comments(task_id);
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

	if _, err := db.Exec(`ALTER TABLE users ADD COLUMN username TEXT`); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			log.Printf("warning: unable to add username column: %v", err)
		}
	}
	if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users(username) WHERE username IS NOT NULL AND username != ''`); err != nil {
		log.Printf("warning: unable to ensure idx_users_username: %v", err)
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
	if _, err := db.Exec(`ALTER TABLE tasks ADD COLUMN description TEXT DEFAULT ''`); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			log.Printf("warning: unable to add description column: %v", err)
		}
	}
	if _, err := db.Exec(`ALTER TABLE tasks ADD COLUMN created_by INTEGER`); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			log.Printf("warning: unable to add created_by column: %v", err)
		}
	}
	if _, err := db.Exec(`ALTER TABLE users ADD COLUMN telegram TEXT NOT NULL DEFAULT ''`); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			log.Printf("warning: unable to add telegram column: %v", err)
		}
	}
	if _, err := db.Exec(`ALTER TABLE users ADD COLUMN first_name TEXT NOT NULL DEFAULT ''`); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			log.Printf("warning: unable to add first_name column: %v", err)
		}
	}
	if _, err := db.Exec(`ALTER TABLE users ADD COLUMN last_name TEXT NOT NULL DEFAULT ''`); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			log.Printf("warning: unable to add last_name column: %v", err)
		}
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS task_comments (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id INTEGER NOT NULL,
		author_id INTEGER,
		body TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(task_id) REFERENCES tasks(id) ON DELETE CASCADE,
		FOREIGN KEY(author_id) REFERENCES users(id) ON DELETE SET NULL
	)`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_task_comments_task ON task_comments(task_id)`); err != nil {
		log.Printf("warning: unable to ensure idx_task_comments_task: %v", err)
	}
	if _, err := db.Exec(`UPDATE tasks SET project_id = ? WHERE project_id IS NULL OR project_id = 0`, DefaultProjectID); err != nil {
		log.Printf("warning: unable to backfill project_id: %v", err)
	}
	if _, err := db.Exec(`UPDATE tasks SET description = comment WHERE (description IS NULL OR description = '') AND comment IS NOT NULL AND comment != ''`); err != nil {
		log.Printf("warning: unable to backfill description from comment: %v", err)
	}
	if _, err := db.Exec(`ALTER TABLE tasks ADD COLUMN author_label TEXT`); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			log.Printf("warning: unable to add author_label column: %v", err)
		}
	}
	if _, err := db.Exec(`ALTER TABLE projects ADD COLUMN is_inbox INTEGER NOT NULL DEFAULT 0`); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			log.Printf("warning: unable to add is_inbox column: %v", err)
		}
	}
	if _, err := db.Exec(`ALTER TABLE projects ADD COLUMN owner_id INTEGER`); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			log.Printf("warning: unable to add owner_id column to projects: %v", err)
		}
	}

	if _, err := db.Exec(`ALTER TABLE tasks ADD COLUMN priority TEXT NOT NULL DEFAULT 'medium'`); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			log.Printf("warning: unable to add priority column to tasks: %v", err)
		}
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
	var adminID int64
	if err := db.QueryRow(`SELECT id FROM users WHERE email = ?`, adminEmail).Scan(&adminID); err == nil && adminID > 0 {
		var inboxID int64
		if err := db.QueryRow(`SELECT id FROM projects WHERE is_inbox = 1 AND owner_id = ?`, adminID).Scan(&inboxID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				inboxName := fmt.Sprintf("%s#%d", DefaultInboxProjectName, adminID)
				res, err := db.Exec(`INSERT INTO projects (name, is_inbox, owner_id) VALUES (?, 1, ?)`, inboxName, adminID)
				if err == nil {
					inboxID, _ = res.LastInsertId()
				}
			}
		}
		if inboxID > 0 {
			if _, err := db.Exec(`INSERT OR IGNORE INTO user_projects (user_id, project_id) VALUES (?, ?)`, adminID, inboxID); err != nil {
				log.Printf("warning: failed to assign inbox to admin: %v", err)
			}
		}
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

func nullableInt64(val int64) any {
	if val == 0 {
		return nil
	}
	return val
}

func nullableString(val string) any {
	if strings.TrimSpace(val) == "" {
		return nil
	}
	return val
}

func validateUsername(username string) error {
	if len(username) < 3 || len(username) > 32 {
		return errors.New("username must be 3-32 characters")
	}
	if strings.Contains(username, "@") {
		return errors.New("username cannot contain @")
	}
	for _, r := range username {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '_' || r == '-' || r == '.':
		default:
			return errors.New("username has invalid characters")
		}
	}
	return nil
}
