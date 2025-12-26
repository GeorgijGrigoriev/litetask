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
	ErrUsernameSet   = errors.New("username already set")
)

type Task struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Status      string    `json:"status"`
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

func (s *Store) InsertTask(title, description string, projectID, createdBy int64) (Task, error) {
	var t Task
	ok, err := s.ProjectExists(projectID)
	if err != nil {
		return t, err
	}
	if !ok {
		return t, fmt.Errorf("project not found")
	}

	res, err := s.db.Exec(
		`INSERT INTO tasks (title, status, description, project_id, created_by) VALUES (?, 'new', ?, ?, ?)`,
		title,
		description,
		projectID,
		nullableInt64(createdBy),
	)
	if err != nil {
		return t, err
	}
	id, _ := res.LastInsertId()
	var created sql.NullInt64
	var email sql.NullString
	var first sql.NullString
	var last sql.NullString
	err = s.db.QueryRow(
		`SELECT t.id, t.title, t.status, COALESCE(t.description, t.comment, ''), t.project_id, t.created_at, t.created_by, u.email, u.first_name, u.last_name
			FROM tasks t
			LEFT JOIN users u ON t.created_by = u.id
			WHERE t.id = ?`,
		id,
	).Scan(&t.ID, &t.Title, &t.Status, &t.Description, &t.ProjectID, &t.CreatedAt, &created, &email, &first, &last)
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

	var created sql.NullInt64
	var email sql.NullString
	var first sql.NullString
	var last sql.NullString
	err = s.db.QueryRow(
		`SELECT t.id, t.title, t.status, COALESCE(t.description, t.comment, ''), t.project_id, t.created_at, t.created_by, u.email, u.first_name, u.last_name
			FROM tasks t
			LEFT JOIN users u ON t.created_by = u.id
			WHERE t.id = ?`,
		id,
	).Scan(&t.ID, &t.Title, &t.Status, &t.Description, &t.ProjectID, &t.CreatedAt, &created, &email, &first, &last)
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

func (s *Store) SetTaskDescription(id int64, description string) (Task, error) {
	var t Task
	res, err := s.db.Exec(`UPDATE tasks SET description = ? WHERE id = ?`, description, id)
	if err != nil {
		return t, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return t, sql.ErrNoRows
	}
	var created sql.NullInt64
	var email sql.NullString
	var first sql.NullString
	var last sql.NullString
	err = s.db.QueryRow(
		`SELECT t.id, t.title, t.status, COALESCE(t.description, t.comment, ''), t.project_id, t.created_at, t.created_by, u.email, u.first_name, u.last_name
			FROM tasks t
			LEFT JOIN users u ON t.created_by = u.id
			WHERE t.id = ?`,
		id,
	).Scan(&t.ID, &t.Title, &t.Status, &t.Description, &t.ProjectID, &t.CreatedAt, &created, &email, &first, &last)
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
	var created sql.NullInt64
	var email sql.NullString
	var first sql.NullString
	var last sql.NullString
	err := s.db.QueryRow(
		`SELECT t.id, t.title, t.status, COALESCE(t.description, t.comment, ''), t.project_id, t.created_at, t.created_by, u.email, u.first_name, u.last_name
			FROM tasks t
			LEFT JOIN users u ON t.created_by = u.id
			WHERE t.id = ?`,
		id,
	).Scan(&t.ID, &t.Title, &t.Status, &t.Description, &t.ProjectID, &t.CreatedAt, &created, &email, &first, &last)
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
	defer tx.Rollback() //nolint: errcheck

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

func (s *Store) CreateUser(email, username, password, role, firstName, lastName string) (User, error) {
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
	res, err := s.db.Exec(
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
	id, _ := res.LastInsertId()
	err = s.db.QueryRow(`SELECT id, email, COALESCE(username, ''), password_hash, role, created_at, telegram, first_name, last_name FROM users WHERE id = ?`, id).
		Scan(&u.ID, &u.Email, &u.Username, &u.Password, &u.Role, &u.CreatedAt, &u.Telegram, &u.FirstName, &u.LastName)
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
	err := s.db.QueryRow(`SELECT id, email, COALESCE(username, ''), password_hash, role, created_at, telegram, first_name, last_name FROM users WHERE email = ?`, email).
		Scan(&u.ID, &u.Email, &u.Username, &u.Password, &u.Role, &u.CreatedAt, &u.Telegram, &u.FirstName, &u.LastName)
	if err != nil {
		return u, err
	}
	u.CreatedAt = u.CreatedAt.UTC()
	return u, nil
}

func (s *Store) GetUserByEmailOrUsername(login string) (User, error) {
	var u User
	login = strings.TrimSpace(strings.ToLower(login))
	err := s.db.QueryRow(
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

func (s *Store) GetUserByID(id int64) (User, error) {
	var u User
	err := s.db.QueryRow(`SELECT id, email, COALESCE(username, ''), password_hash, role, created_at, telegram, first_name, last_name FROM users WHERE id = ?`, id).
		Scan(&u.ID, &u.Email, &u.Username, &u.Password, &u.Role, &u.CreatedAt, &u.Telegram, &u.FirstName, &u.LastName)
	if err != nil {
		return u, err
	}
	u.CreatedAt = u.CreatedAt.UTC()
	return u, nil
}

func (s *Store) ListUsers() ([]User, error) {
	rows, err := s.db.Query(`SELECT id, email, COALESCE(username, ''), password_hash, role, created_at, telegram, first_name, last_name FROM users ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
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

func (s *Store) SetUsernameOnce(id int64, username string) (User, error) {
	username = strings.TrimSpace(strings.ToLower(username))
	if username == "" {
		return User{}, errors.New("username required")
	}
	if err := validateUsername(username); err != nil {
		return User{}, err
	}

	res, err := s.db.Exec(
		`UPDATE users
		SET username = ?
		WHERE id = ? AND (username IS NULL OR username = '')`,
		username,
		id,
	)
	if err != nil {
		return User{}, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		var current sql.NullString
		if err := s.db.QueryRow(`SELECT username FROM users WHERE id = ?`, id).Scan(&current); err != nil {
			return User{}, err
		}
		if current.Valid && strings.TrimSpace(current.String) != "" {
			return User{}, ErrUsernameSet
		}
		return User{}, sql.ErrNoRows
	}
	return s.GetUserByID(id)
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

func (s *Store) UpdateUserProfile(id int64, password *string, telegram *string, firstName *string, lastName *string) (User, error) {
	if password == nil && telegram == nil && firstName == nil && lastName == nil {
		return s.GetUserByID(id)
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
	res, err := s.db.Exec(query, args...)
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
	defer tx.Rollback() //nolint:errcheck

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
	query := `SELECT t.id, t.title, t.status, COALESCE(t.description, t.comment, ''), t.project_id, t.created_at, t.created_by, u.email, u.first_name, u.last_name FROM tasks t LEFT JOIN users u ON t.created_by = u.id`
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

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := make([]Task, 0)
	for rows.Next() {
		var t Task
		var created time.Time
		var authorID sql.NullInt64
		var email sql.NullString
		var first sql.NullString
		var last sql.NullString
		if err := rows.Scan(&t.ID, &t.Title, &t.Status, &t.Description, &t.ProjectID, &created, &authorID, &email, &first, &last); err != nil {
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

func (s *Store) AddTaskComment(taskID int64, body string, authorID int64) (TaskComment, error) {
	var c TaskComment
	res, err := s.db.Exec(
		`INSERT INTO task_comments (task_id, body, author_id) VALUES (?, ?, ?)`,
		taskID,
		body,
		nullableInt64(authorID),
	)
	if err != nil {
		return c, err
	}
	id, _ := res.LastInsertId()
	var created sql.NullInt64
	var email sql.NullString
	err = s.db.QueryRow(
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

func (s *Store) ListTaskComments(taskID int64) ([]TaskComment, error) {
	commentsMap, err := s.ListCommentsByTaskIDs([]int64{taskID})
	if err != nil {
		return nil, err
	}
	return commentsMap[taskID], nil
}

func (s *Store) GetTaskComment(commentID int64) (TaskComment, error) {
	var c TaskComment
	var author sql.NullInt64
	var email sql.NullString
	err := s.db.QueryRow(
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

func (s *Store) DeleteTaskComment(commentID int64) error {
	res, err := s.db.Exec(`DELETE FROM task_comments WHERE id = ?`, commentID)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) ListCommentsByTaskIDs(taskIDs []int64) (map[int64][]TaskComment, error) {
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
	rows, err := s.db.Query(
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
	defer rows.Close()

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
