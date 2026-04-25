package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	t.Setenv("ADMIN_EMAIL", "admin@example.com")
	t.Setenv("ADMIN_PASSWORD", "secret123")
	path := filepath.Join(t.TempDir(), "test.db")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		_ = st.Close()
	})
	return st
}

func findAdmin(t *testing.T, st *Store) User {
	t.Helper()
	ctx := context.Background()
	users, err := st.ListUsers(ctx)
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	for _, u := range users {
		if u.Role == "admin" {
			return u
		}
	}
	t.Fatal("admin user not found")
	return User{}
}

func createUser(t *testing.T, st *Store, email string) User {
	t.Helper()
	u, err := st.CreateUser(context.Background(), email, "", "password1", "user", "First", "Last")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u
}

func TestProjectLifecycle(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	p, err := st.CreateProject(ctx, "Project A")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	ok, err := st.ProjectExists(ctx, p.ID)
	if err != nil {
		t.Fatalf("project exists: %v", err)
	}
	if !ok {
		t.Fatalf("project should exist")
	}

	if name := st.LookupProjectName(ctx, p.ID); name != p.Name {
		t.Fatalf("lookup project name: got %q want %q", name, p.Name)
	}

	projects, err := st.ListProjects(ctx)
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}
	if len(projects) == 0 {
		t.Fatalf("expected projects to be non-empty")
	}
}

func TestTaskCRUD(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	u := createUser(t, st, "user1@example.com")
	p, err := st.CreateProject(ctx, "Project A")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	task, err := st.InsertTask(ctx, "Title", "Desc", "", p.ID, u.ID)
	if err != nil {
		t.Fatalf("insert task: %v", err)
	}
	if task.Status != "new" {
		t.Fatalf("expected status new, got %q", task.Status)
	}
	if task.CreatedBy != u.ID {
		t.Fatalf("expected CreatedBy %d, got %d", u.ID, task.CreatedBy)
	}

	tasks, err := st.FetchTasks(ctx, p.ID, "", nil)
	if err != nil {
		t.Fatalf("fetch tasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	updated, err := st.SetTaskStatus(ctx, task.ID, "done")
	if err != nil {
		t.Fatalf("set task status: %v", err)
	}
	if updated.Status != "done" {
		t.Fatalf("expected status done, got %q", updated.Status)
	}
	if _, err := st.SetTaskStatus(ctx, task.ID, "bad"); !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("expected ErrInvalidStatus, got %v", err)
	}

	updated, err = st.SetTaskDescription(ctx, task.ID, "New Desc")
	if err != nil {
		t.Fatalf("set task description: %v", err)
	}
	if updated.Description != "New Desc" {
		t.Fatalf("expected description updated")
	}

	if err := st.DeleteTask(ctx, task.ID); err != nil {
		t.Fatalf("delete task: %v", err)
	}
	if _, err := st.GetTask(ctx, task.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestTaskComments(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	u := createUser(t, st, "user2@example.com")
	p, err := st.CreateProject(ctx, "Project A")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	task, err := st.InsertTask(ctx, "Title", "", "", p.ID, u.ID)
	if err != nil {
		t.Fatalf("insert task: %v", err)
	}

	comment, err := st.AddTaskComment(ctx, task.ID, "hello", u.ID)
	if err != nil {
		t.Fatalf("add comment: %v", err)
	}
	if comment.TaskID != task.ID {
		t.Fatalf("comment task id mismatch")
	}

	comments, err := st.ListTaskComments(ctx, task.ID)
	if err != nil {
		t.Fatalf("list comments: %v", err)
	}
	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}

	got, err := st.GetTaskComment(ctx, comment.ID)
	if err != nil {
		t.Fatalf("get comment: %v", err)
	}
	if got.ID != comment.ID {
		t.Fatalf("comment id mismatch")
	}

	if err := st.DeleteTaskComment(ctx, comment.ID); err != nil {
		t.Fatalf("delete comment: %v", err)
	}
	comments, err = st.ListTaskComments(ctx, task.ID)
	if err != nil {
		t.Fatalf("list comments after delete: %v", err)
	}
	if len(comments) != 0 {
		t.Fatalf("expected 0 comments, got %d", len(comments))
	}
}

func TestUserRolesAndProjects(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	admin := findAdmin(t, st)

	if _, err := st.UpdateUserRole(ctx, admin.ID, "user"); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("expected ErrLastAdmin, got %v", err)
	}

	u := createUser(t, st, "user3@example.com")
	if _, err := st.UpdateUserRole(ctx, u.ID, "admin"); err != nil {
		t.Fatalf("promote to admin: %v", err)
	}
	if _, err := st.UpdateUserRole(ctx, admin.ID, "user"); err != nil {
		t.Fatalf("demote admin: %v", err)
	}

	if err := st.SetUserProjects(ctx, u.ID, []int64{9999}); err == nil {
		t.Fatalf("expected error for missing project")
	}

	p, err := st.CreateProject(ctx, "Project A")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	if err := st.SetUserProjects(ctx, u.ID, []int64{DefaultProjectID, p.ID}); err != nil {
		t.Fatalf("set user projects: %v", err)
	}
	ids, err := st.GetUserProjects(ctx, u.ID)
	if err != nil {
		t.Fatalf("get user projects: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(ids))
	}
}

func TestUsernameAndProfile(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	if _, err := st.CreateUser(ctx, "bad@example.com", "ab", "password1", "user", "", ""); err == nil {
		t.Fatalf("expected username validation error")
	}

	u := createUser(t, st, "user4@example.com")
	updated, err := st.SetUsernameOnce(ctx, u.ID, "valid_name")
	if err != nil {
		t.Fatalf("set username once: %v", err)
	}
	if updated.Username != "valid_name" {
		t.Fatalf("expected username set, got %q", updated.Username)
	}
	if _, err := st.SetUsernameOnce(ctx, u.ID, "another"); !errors.Is(err, ErrUsernameSet) {
		t.Fatalf("expected ErrUsernameSet, got %v", err)
	}

	if _, err := st.UpdateUserPassword(ctx, u.ID, "123"); err == nil {
		t.Fatalf("expected password too short error")
	}

	telegram := "tg"
	first := "Alice"
	last := "Smith"
	updated, err = st.UpdateUserProfile(ctx, u.ID, nil, &telegram, &first, &last)
	if err != nil {
		t.Fatalf("update profile: %v", err)
	}
	if updated.Telegram != telegram || updated.FirstName != first || updated.LastName != last {
		t.Fatalf("profile update mismatch")
	}
}

func TestGetUserByEmailAndUsername(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	u := createUser(t, st, "lookup@example.com")
	_, _ = st.SetUsernameOnce(ctx, u.ID, "lookupuser")

	byEmail, err := st.GetUserByEmail(ctx, "lookup@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	if byEmail.ID != u.ID {
		t.Fatalf("expected user ID %d, got %d", u.ID, byEmail.ID)
	}

	byLogin, err := st.GetUserByEmailOrUsername(ctx, "lookupuser")
	if err != nil {
		t.Fatalf("GetUserByEmailOrUsername by username: %v", err)
	}
	if byLogin.ID != u.ID {
		t.Fatalf("expected user ID %d, got %d", u.ID, byLogin.ID)
	}

	byEmail2, err := st.GetUserByEmailOrUsername(ctx, "lookup@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmailOrUsername by email: %v", err)
	}
	if byEmail2.ID != u.ID {
		t.Fatalf("expected user ID %d, got %d", u.ID, byEmail2.ID)
	}

	if _, err := st.GetUserByEmail(ctx, "noexist@example.com"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected ErrNoRows, got %v", err)
	}
}

func TestDeleteProject(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	p, err := st.CreateProject(ctx, "ToDelete")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	if err := st.DeleteProject(ctx, p.ID); err != nil {
		t.Fatalf("delete project: %v", err)
	}

	ok, err := st.ProjectExists(ctx, p.ID)
	if err != nil {
		t.Fatalf("project exists: %v", err)
	}
	if ok {
		t.Fatal("project should not exist after delete")
	}

	if err := st.DeleteProject(ctx, DefaultProjectID); !errors.Is(err, ErrProtectedProject) {
		t.Fatalf("expected ErrProtectedProject, got %v", err)
	}

	if err := st.DeleteProject(ctx, 99999); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected ErrNoRows for missing project, got %v", err)
	}
}

func TestDeleteProjectWithTasks(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	u := createUser(t, st, "user6@example.com")

	p, _ := st.CreateProject(ctx, "WithTasks")
	_, _ = st.InsertTask(ctx, "Task", "", "", p.ID, u.ID)

	if err := st.DeleteProject(ctx, p.ID); err != nil {
		t.Fatalf("delete project with tasks: %v", err)
	}

	tasks, err := st.FetchTasks(ctx, p.ID, "", nil)
	if err != nil {
		t.Fatalf("fetch tasks: %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("expected tasks to be cascade-deleted, got %d", len(tasks))
	}
}

func TestGetUserInboxID(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	u := createUser(t, st, "inbox@example.com")

	id := st.GetUserInboxID(ctx, u.ID)
	if id == 0 {
		t.Fatal("expected non-zero inbox ID after CreateUser")
	}

	// non-existent user → 0
	if got := st.GetUserInboxID(ctx, 99999); got != 0 {
		t.Fatalf("expected 0 for missing user, got %d", got)
	}
}

func TestUpdateUserPassword(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	u := createUser(t, st, "pwdchange@example.com")

	updated, err := st.UpdateUserPassword(ctx, u.ID, "newpassword")
	if err != nil {
		t.Fatalf("UpdateUserPassword: %v", err)
	}
	if updated.ID != u.ID {
		t.Fatalf("unexpected user ID")
	}

	if _, err := st.UpdateUserPassword(ctx, u.ID, "123"); err == nil {
		t.Fatal("expected error for short password")
	}

	if _, err := st.UpdateUserPassword(ctx, 99999, "newpassword"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected ErrNoRows for missing user, got %v", err)
	}
}

func TestMoveTask(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	u := createUser(t, st, "mover@example.com")

	p1, err := st.CreateProject(ctx, "Source")
	if err != nil {
		t.Fatalf("create project A: %v", err)
	}
	p2, err := st.CreateProject(ctx, "Target")
	if err != nil {
		t.Fatalf("create project B: %v", err)
	}

	task, err := st.InsertTask(ctx, "Task", "", "", p1.ID, u.ID)
	if err != nil {
		t.Fatalf("insert task: %v", err)
	}

	moved, err := st.MoveTask(ctx, task.ID, p2.ID)
	if err != nil {
		t.Fatalf("MoveTask: %v", err)
	}
	if moved.ProjectID != p2.ID {
		t.Fatalf("expected projectID %d, got %d", p2.ID, moved.ProjectID)
	}

	if _, err := st.MoveTask(ctx, task.ID, 99999); err == nil || !strings.Contains(err.Error(), "project not found") {
		t.Fatalf("expected 'project not found' error, got %v", err)
	}

	if _, err := st.MoveTask(ctx, 99999, p1.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected ErrNoRows for missing task, got %v", err)
	}
}

func TestFetchTasksAllowedFilter(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	u := createUser(t, st, "user5@example.com")
	p1, err := st.CreateProject(ctx, "Project A")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	p2, err := st.CreateProject(ctx, "Project B")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	if _, err := st.InsertTask(ctx, "Task 1", "", "", p1.ID, u.ID); err != nil {
		t.Fatalf("insert task: %v", err)
	}
	if _, err := st.InsertTask(ctx, "Task 2", "", "", p2.ID, u.ID); err != nil {
		t.Fatalf("insert task: %v", err)
	}

	allowed := map[int64]struct{}{p1.ID: {}}
	tasks, err := st.FetchTasks(ctx, 0, "", allowed)
	if err != nil {
		t.Fatalf("fetch tasks: %v", err)
	}
	if len(tasks) != 1 || tasks[0].ProjectID != p1.ID {
		t.Fatalf("expected only tasks from allowed project")
	}
}
