package store

import "context"

// Storer is the interface implemented by Store, enabling test fakes and mock implementations.
type Storer interface {
	Close() error

	InsertTask(ctx context.Context, title, description, authorLabel string, projectID, createdBy int64, priority string) (Task, error)
	GetTask(ctx context.Context, id int64) (Task, error)
	SetTaskStatus(ctx context.Context, id int64, status string) (Task, error)
	SetTaskPriority(ctx context.Context, id int64, priority string) (Task, error)
	SetTaskDescription(ctx context.Context, id int64, description string) (Task, error)
	DeleteTask(ctx context.Context, id int64) error
	MoveTask(ctx context.Context, taskID, newProjectID int64) (Task, error)
	FetchTasks(ctx context.Context, projectID int64, status string, allowed map[int64]struct{}) ([]Task, error)

	CreateProject(ctx context.Context, name string) (Project, error)
	ListProjects(ctx context.Context) ([]Project, error)
	DeleteProject(ctx context.Context, id int64) error
	ProjectExists(ctx context.Context, id int64) (bool, error)
	ProjectNameMap(ctx context.Context) map[int64]string
	LookupProjectName(ctx context.Context, id int64) string

	CreateUser(ctx context.Context, email, username, password, role, firstName, lastName string) (User, error)
	GetUserByEmail(ctx context.Context, email string) (User, error)
	GetUserByEmailOrUsername(ctx context.Context, login string) (User, error)
	GetUserByID(ctx context.Context, id int64) (User, error)
	ListUsers(ctx context.Context) ([]User, error)
	SetUsernameOnce(ctx context.Context, id int64, username string) (User, error)
	UpdateUserRole(ctx context.Context, id int64, role string) (User, error)
	UpdateUserPassword(ctx context.Context, id int64, password string) (User, error)
	UpdateUserProfile(ctx context.Context, id int64, password *string, telegram *string, firstName *string, lastName *string, language *string) (User, error)
	GetUserProjects(ctx context.Context, userID int64) ([]int64, error)
	SetUserProjects(ctx context.Context, userID int64, projectIDs []int64) error
	GetUserInboxID(ctx context.Context, userID int64) int64
	EnsureUserInbox(ctx context.Context, userID int64) (int64, error)

	AddTaskComment(ctx context.Context, taskID int64, body string, authorID int64) (TaskComment, error)
	ListTaskComments(ctx context.Context, taskID int64) ([]TaskComment, error)
	GetTaskComment(ctx context.Context, commentID int64) (TaskComment, error)
	DeleteTaskComment(ctx context.Context, commentID int64) error
	ListCommentsByTaskIDs(ctx context.Context, taskIDs []int64) (map[int64][]TaskComment, error)
}
