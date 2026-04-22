package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"litetask/internal/store"
)

var testSecret = []byte("test-secret-key-must-be-32-bytes!!")

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	t.Setenv("ADMIN_EMAIL", "admin@example.com")
	t.Setenv("ADMIN_PASSWORD", "secret123")
	path := filepath.Join(t.TempDir(), "test.db")
	st, err := store.Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func newTestServer(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	st := openTestStore(t)
	return New(st, testSecret, true, ""), st
}

func doRequest(t *testing.T, srv *Server, method, path, body string, cookie *http.Cookie) *http.Response {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w, req)
	return w.Result()
}

func mustLogin(t *testing.T, srv *Server, email, password string) *http.Cookie {
	t.Helper()
	b, _ := json.Marshal(map[string]string{"login": email, "password": password})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login failed: status %d, body: %s", w.Code, w.Body.String())
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == "auth" {
			return c
		}
	}
	t.Fatal("auth cookie not found after login")
	return nil
}

func decodeJSON(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
}

func jsonBody(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// ── Auth ──────────────────────────────────────────────────────────────────────

func TestLoginSuccess(t *testing.T) {
	srv, _ := newTestServer(t)
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")
	if cookie.HttpOnly != true {
		t.Error("auth cookie should be HttpOnly")
	}
}

func TestLoginInvalidPassword(t *testing.T) {
	srv, _ := newTestServer(t)
	resp := doRequest(t, srv, http.MethodPost, "/api/auth/login",
		jsonBody(map[string]string{"login": "admin@example.com", "password": "wrong"}), nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestLoginUnknownUser(t *testing.T) {
	srv, _ := newTestServer(t)
	resp := doRequest(t, srv, http.MethodPost, "/api/auth/login",
		jsonBody(map[string]string{"login": "nobody@example.com", "password": "pass"}), nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestLoginMissingFields(t *testing.T) {
	srv, _ := newTestServer(t)
	resp := doRequest(t, srv, http.MethodPost, "/api/auth/login",
		jsonBody(map[string]string{"login": "admin@example.com"}), nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestRegisterAndMe(t *testing.T) {
	srv, _ := newTestServer(t)

	resp := doRequest(t, srv, http.MethodPost, "/api/auth/register",
		jsonBody(map[string]string{"email": "newuser@example.com", "password": "pass123"}), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var me meResponse
	decodeJSON(t, resp, &me)
	if me.Email != "newuser@example.com" {
		t.Fatalf("expected email, got %q", me.Email)
	}

	var authCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "auth" {
			authCookie = c
		}
	}
	if authCookie == nil {
		t.Fatal("no auth cookie after register")
	}

	meResp := doRequest(t, srv, http.MethodGet, "/api/auth/me", "", authCookie)
	if meResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for /me, got %d", meResp.StatusCode)
	}
	var me2 meResponse
	decodeJSON(t, meResp, &me2)
	if me2.Email != "newuser@example.com" {
		t.Fatalf("expected email in /me, got %q", me2.Email)
	}
}

func TestRegisterDuplicateEmail(t *testing.T) {
	srv, _ := newTestServer(t)
	body := jsonBody(map[string]string{"email": "admin@example.com", "password": "pass123"})
	resp := doRequest(t, srv, http.MethodPost, "/api/auth/register", body, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestRegisterDisabled(t *testing.T) {
	st := openTestStore(t)
	srv := New(st, testSecret, false, "")
	resp := doRequest(t, srv, http.MethodPost, "/api/auth/register",
		jsonBody(map[string]string{"email": "x@example.com", "password": "pass123"}), nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestLogout(t *testing.T) {
	srv, _ := newTestServer(t)
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")

	resp := doRequest(t, srv, http.MethodPost, "/api/auth/logout", "", cookie)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
	for _, c := range resp.Cookies() {
		if c.Name == "auth" && c.MaxAge < 0 {
			return
		}
	}
	t.Error("expected auth cookie to be cleared")
}

func TestMeUnauthorized(t *testing.T) {
	srv, _ := newTestServer(t)
	resp := doRequest(t, srv, http.MethodGet, "/api/auth/me", "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

// ── Middleware ────────────────────────────────────────────────────────────────

func TestRequireUserNoAuth(t *testing.T) {
	srv, _ := newTestServer(t)
	for _, path := range []string{"/api/tasks", "/api/projects", "/api/profile"} {
		resp := doRequest(t, srv, http.MethodGet, path, "", nil)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s: expected 401, got %d", path, resp.StatusCode)
		}
	}
}

func TestRequireAdminAsUser(t *testing.T) {
	srv, st := newTestServer(t)
	ctx := context.Background()
	_, _ = st.CreateUser(ctx, "user@example.com", "", "pass123", "user", "", "")
	cookie := mustLogin(t, srv, "user@example.com", "pass123")

	resp := doRequest(t, srv, http.MethodGet, "/api/users", "", cookie)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestBlockedUserForbidden(t *testing.T) {
	srv, st := newTestServer(t)
	ctx := context.Background()
	u, _ := st.CreateUser(ctx, "blocked@example.com", "", "pass123", "user", "", "")
	_, _ = st.UpdateUserRole(ctx, u.ID, "blocked")
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")

	// block them via API then try to use their session
	// create fresh token for blocked user directly
	blockedToken := createToken(store.User{ID: u.ID, Role: "blocked"}, testSecret)
	blockedCookie := &http.Cookie{Name: "auth", Value: blockedToken}

	resp := doRequest(t, srv, http.MethodGet, "/api/projects", "", blockedCookie)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for blocked user, got %d (admin cookie was %v)", resp.StatusCode, cookie != nil)
	}
}

func TestCORSHeaders(t *testing.T) {
	srv, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodOptions, "/api/tasks", nil)
	w := httptest.NewRecorder()
	srv.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for OPTIONS, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Error("expected CORS origin header")
	}
}

// ── Projects ──────────────────────────────────────────────────────────────────

func TestListProjects(t *testing.T) {
	srv, _ := newTestServer(t)
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")

	resp := doRequest(t, srv, http.MethodGet, "/api/projects", "", cookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var projects []map[string]any
	decodeJSON(t, resp, &projects)
	if len(projects) == 0 {
		t.Fatal("expected at least one project")
	}
}

func TestCreateAndDeleteProject(t *testing.T) {
	srv, _ := newTestServer(t)
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")

	resp := doRequest(t, srv, http.MethodPost, "/api/projects",
		jsonBody(map[string]string{"name": "Test Project"}), cookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create project: expected 200, got %d", resp.StatusCode)
	}
	var p map[string]any
	decodeJSON(t, resp, &p)
	id := int64(p["id"].(float64))

	delResp := doRequest(t, srv, http.MethodDelete, "/api/projects/"+strconv.FormatInt(id, 10), "", cookie)
	if delResp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete project: expected 204, got %d", delResp.StatusCode)
	}
}

func TestDeleteDefaultProjectForbidden(t *testing.T) {
	srv, _ := newTestServer(t)
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")

	resp := doRequest(t, srv, http.MethodDelete,
		"/api/projects/"+strconv.Itoa(store.DefaultProjectID), "", cookie)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for default project delete, got %d", resp.StatusCode)
	}
}

func TestCreateProjectDuplicateName(t *testing.T) {
	srv, _ := newTestServer(t)
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")

	doRequest(t, srv, http.MethodPost, "/api/projects",
		jsonBody(map[string]string{"name": "Dup"}), cookie)
	resp := doRequest(t, srv, http.MethodPost, "/api/projects",
		jsonBody(map[string]string{"name": "Dup"}), cookie)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for duplicate project name, got %d", resp.StatusCode)
	}
}

func TestUserCannotDeleteProject(t *testing.T) {
	srv, st := newTestServer(t)
	ctx := context.Background()
	_, _ = st.CreateUser(ctx, "user@example.com", "", "pass123", "user", "", "")
	adminCookie := mustLogin(t, srv, "admin@example.com", "secret123")
	userCookie := mustLogin(t, srv, "user@example.com", "pass123")

	resp := doRequest(t, srv, http.MethodPost, "/api/projects",
		jsonBody(map[string]string{"name": "DeleteMe"}), adminCookie)
	var p map[string]any
	decodeJSON(t, resp, &p)
	id := int64(p["id"].(float64))

	delResp := doRequest(t, srv, http.MethodDelete, "/api/projects/"+strconv.FormatInt(id, 10), "", userCookie)
	if delResp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", delResp.StatusCode)
	}
}

// ── Tasks ─────────────────────────────────────────────────────────────────────

func TestCreateListUpdateDeleteTask(t *testing.T) {
	srv, st := newTestServer(t)
	ctx := context.Background()
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")
	p, _ := st.CreateProject(ctx, "TaskProject")

	// create
	resp := doRequest(t, srv, http.MethodPost, "/api/tasks",
		jsonBody(map[string]any{"title": "My Task", "projectId": p.ID}), cookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create task: expected 200, got %d", resp.StatusCode)
	}
	var task taskResponse
	decodeJSON(t, resp, &task)
	if task.Title != "My Task" || task.Status != "new" {
		t.Fatalf("unexpected task: %+v", task)
	}

	// list
	listResp := doRequest(t, srv, http.MethodGet,
		"/api/tasks?projectId="+strconv.FormatInt(p.ID, 10), "", cookie)
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list tasks: expected 200, got %d", listResp.StatusCode)
	}
	var tasks []taskResponse
	decodeJSON(t, listResp, &tasks)
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	// update status
	statusResp := doRequest(t, srv, http.MethodPatch,
		"/api/tasks/"+strconv.FormatInt(task.ID, 10)+"/status",
		jsonBody(map[string]string{"status": "done"}), cookie)
	if statusResp.StatusCode != http.StatusOK {
		t.Fatalf("update status: expected 200, got %d", statusResp.StatusCode)
	}
	var updated taskResponse
	decodeJSON(t, statusResp, &updated)
	if updated.Status != "done" {
		t.Fatalf("expected status done, got %q", updated.Status)
	}

	// update description
	descResp := doRequest(t, srv, http.MethodPatch,
		"/api/tasks/"+strconv.FormatInt(task.ID, 10),
		jsonBody(map[string]string{"description": "new desc"}), cookie)
	if descResp.StatusCode != http.StatusOK {
		t.Fatalf("update description: expected 200, got %d", descResp.StatusCode)
	}

	// delete
	delResp := doRequest(t, srv, http.MethodDelete,
		"/api/tasks/"+strconv.FormatInt(task.ID, 10), "", cookie)
	if delResp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete task: expected 204, got %d", delResp.StatusCode)
	}
}

func TestCreateTaskInvalidStatus(t *testing.T) {
	srv, st := newTestServer(t)
	ctx := context.Background()
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")
	p, _ := st.CreateProject(ctx, "P")

	resp := doRequest(t, srv, http.MethodPost, "/api/tasks",
		jsonBody(map[string]any{"title": "T", "projectId": p.ID}), cookie)
	var task taskResponse
	decodeJSON(t, resp, &task)

	statusResp := doRequest(t, srv, http.MethodPatch,
		"/api/tasks/"+strconv.FormatInt(task.ID, 10)+"/status",
		jsonBody(map[string]string{"status": "invalid"}), cookie)
	if statusResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid status, got %d", statusResp.StatusCode)
	}
}

func TestCreateTaskEmptyTitle(t *testing.T) {
	srv, _ := newTestServer(t)
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")

	resp := doRequest(t, srv, http.MethodPost, "/api/tasks",
		jsonBody(map[string]any{"title": "", "projectId": store.DefaultProjectID}), cookie)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty title, got %d", resp.StatusCode)
	}
}

func TestGetTaskNotFound(t *testing.T) {
	srv, _ := newTestServer(t)
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")

	resp := doRequest(t, srv, http.MethodPatch, "/api/tasks/999999/status",
		jsonBody(map[string]string{"status": "done"}), cookie)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestUserCannotAccessOtherProject(t *testing.T) {
	srv, st := newTestServer(t)
	ctx := context.Background()
	_, _ = st.CreateUser(ctx, "user@example.com", "", "pass123", "user", "", "")
	userCookie := mustLogin(t, srv, "user@example.com", "pass123")

	// user tries to list tasks from default project they're not assigned to
	// (user has their inbox but not DefaultProjectID unless SetUserProjects was called)
	// Actually CreateUser assigns DefaultProjectID, so let's use a project they don't have
	p, _ := st.CreateProject(ctx, "AdminOnly")

	resp := doRequest(t, srv, http.MethodGet,
		"/api/tasks?projectId="+strconv.FormatInt(p.ID, 10), "", userCookie)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

// ── Comments ──────────────────────────────────────────────────────────────────

func TestTaskComments(t *testing.T) {
	srv, st := newTestServer(t)
	ctx := context.Background()
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")
	p, _ := st.CreateProject(ctx, "CP")

	taskResp := doRequest(t, srv, http.MethodPost, "/api/tasks",
		jsonBody(map[string]any{"title": "T", "projectId": p.ID}), cookie)
	var task taskResponse
	decodeJSON(t, taskResp, &task)
	taskPath := "/api/tasks/" + strconv.FormatInt(task.ID, 10)

	// add comment
	addResp := doRequest(t, srv, http.MethodPost, taskPath+"/comments",
		jsonBody(map[string]string{"body": "hello"}), cookie)
	if addResp.StatusCode != http.StatusOK {
		t.Fatalf("add comment: expected 200, got %d", addResp.StatusCode)
	}
	var comment store.TaskComment
	decodeJSON(t, addResp, &comment)
	if comment.Body != "hello" {
		t.Fatalf("expected body hello, got %q", comment.Body)
	}

	// list comments
	listResp := doRequest(t, srv, http.MethodGet, taskPath+"/comments", "", cookie)
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list comments: expected 200, got %d", listResp.StatusCode)
	}
	var comments []store.TaskComment
	decodeJSON(t, listResp, &comments)
	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}

	// delete comment
	delResp := doRequest(t, srv, http.MethodDelete,
		taskPath+"/comments/"+strconv.FormatInt(comment.ID, 10), "", cookie)
	if delResp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete comment: expected 204, got %d", delResp.StatusCode)
	}
}

func TestAddEmptyComment(t *testing.T) {
	srv, st := newTestServer(t)
	ctx := context.Background()
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")
	p, _ := st.CreateProject(ctx, "CP")

	taskResp := doRequest(t, srv, http.MethodPost, "/api/tasks",
		jsonBody(map[string]any{"title": "T", "projectId": p.ID}), cookie)
	var task taskResponse
	decodeJSON(t, taskResp, &task)

	resp := doRequest(t, srv, http.MethodPost,
		"/api/tasks/"+strconv.FormatInt(task.ID, 10)+"/comments",
		jsonBody(map[string]string{"body": "   "}), cookie)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty comment, got %d", resp.StatusCode)
	}
}

// ── Users (admin) ─────────────────────────────────────────────────────────────

func TestListUsers(t *testing.T) {
	srv, _ := newTestServer(t)
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")

	resp := doRequest(t, srv, http.MethodGet, "/api/users", "", cookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var users []userResponse
	decodeJSON(t, resp, &users)
	if len(users) == 0 {
		t.Fatal("expected at least one user")
	}
}

func TestCreateUserAdmin(t *testing.T) {
	srv, _ := newTestServer(t)
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")

	resp := doRequest(t, srv, http.MethodPost, "/api/users",
		jsonBody(map[string]string{
			"email": "newbie@example.com", "password": "pass123", "role": "user",
		}), cookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var u userResponse
	decodeJSON(t, resp, &u)
	if u.Email != "newbie@example.com" || u.Role != "user" {
		t.Fatalf("unexpected user: %+v", u)
	}
}

func TestUpdateUserRole(t *testing.T) {
	srv, st := newTestServer(t)
	ctx := context.Background()
	u, _ := st.CreateUser(ctx, "target@example.com", "", "pass123", "user", "", "")
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")

	resp := doRequest(t, srv, http.MethodPatch,
		"/api/users/"+strconv.FormatInt(u.ID, 10),
		jsonBody(map[string]string{"role": "admin"}), cookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var updated userResponse
	decodeJSON(t, resp, &updated)
	if updated.Role != "admin" {
		t.Fatalf("expected role admin, got %q", updated.Role)
	}
}

func TestUpdateUserProjects(t *testing.T) {
	srv, st := newTestServer(t)
	ctx := context.Background()
	u, _ := st.CreateUser(ctx, "target@example.com", "", "pass123", "user", "", "")
	p, _ := st.CreateProject(ctx, "NewProject")
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")

	resp := doRequest(t, srv, http.MethodPatch,
		"/api/users/"+strconv.FormatInt(u.ID, 10),
		jsonBody(map[string]any{"projectIds": []int64{store.DefaultProjectID, p.ID}}), cookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var updated userResponse
	decodeJSON(t, resp, &updated)
	if len(updated.ProjectIDs) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(updated.ProjectIDs))
	}
}

// ── Profile ───────────────────────────────────────────────────────────────────

func TestGetProfile(t *testing.T) {
	srv, _ := newTestServer(t)
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")

	resp := doRequest(t, srv, http.MethodGet, "/api/profile", "", cookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var me meResponse
	decodeJSON(t, resp, &me)
	if me.Email != "admin@example.com" {
		t.Fatalf("expected admin email, got %q", me.Email)
	}
}

func TestUpdateProfile(t *testing.T) {
	srv, _ := newTestServer(t)
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")

	first := "Alice"
	resp := doRequest(t, srv, http.MethodPatch, "/api/profile",
		jsonBody(map[string]*string{"firstName": &first}), cookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var me meResponse
	decodeJSON(t, resp, &me)
	if me.FirstName != "Alice" {
		t.Fatalf("expected firstName Alice, got %q", me.FirstName)
	}
}

func TestUpdateProfileNothingToUpdate(t *testing.T) {
	srv, _ := newTestServer(t)
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")

	resp := doRequest(t, srv, http.MethodPatch, "/api/profile",
		jsonBody(map[string]string{}), cookie)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestProfileUnauthorized(t *testing.T) {
	srv, _ := newTestServer(t)
	resp := doRequest(t, srv, http.MethodGet, "/api/profile", "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

// ── Token helpers ─────────────────────────────────────────────────────────────

func TestCreateAndParseToken(t *testing.T) {
	u := store.User{ID: 42, Role: "admin"}
	token := createToken(u, testSecret)
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	claims, err := parseToken(token, testSecret)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if claims.UserID != 42 || claims.Role != "admin" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestParseTokenWrongSecret(t *testing.T) {
	u := store.User{ID: 1, Role: "user"}
	token := createToken(u, testSecret)
	if _, err := parseToken(token, []byte("other-secret-key-32-bytes-padding")); err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestParseTokenInvalid(t *testing.T) {
	cases := []string{"", "notavalidtoken", "a.b.c"}
	for _, tc := range cases {
		if _, err := parseToken(tc, testSecret); err == nil {
			t.Errorf("expected error for token %q", tc)
		}
	}
}

// ── Error response format ─────────────────────────────────────────────────────

func TestErrorResponseIsJSON(t *testing.T) {
	srv, _ := newTestServer(t)
	resp := doRequest(t, srv, http.MethodGet, "/api/tasks", "", nil)
	if resp.Header.Get("Content-Type") != "application/json" {
		t.Errorf("expected JSON content-type, got %q", resp.Header.Get("Content-Type"))
	}
	var body map[string]string
	decodeJSON(t, resp, &body)
	if _, ok := body["error"]; !ok {
		t.Error("expected 'error' field in response body")
	}
}

// ── Method not allowed ────────────────────────────────────────────────────────

func TestMethodNotAllowed(t *testing.T) {
	srv, _ := newTestServer(t)
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")

	resp := doRequest(t, srv, http.MethodDelete, "/api/tasks", "", cookie)
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.StatusCode)
	}
}

// ── handleUsers edge cases ────────────────────────────────────────────────────

func TestCreateUserMissingFields(t *testing.T) {
	srv, _ := newTestServer(t)
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")

	resp := doRequest(t, srv, http.MethodPost, "/api/users",
		jsonBody(map[string]string{"email": "x@example.com"}), cookie)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing password, got %d", resp.StatusCode)
	}
}

func TestCreateUserShortPassword(t *testing.T) {
	srv, _ := newTestServer(t)
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")

	resp := doRequest(t, srv, http.MethodPost, "/api/users",
		jsonBody(map[string]string{"email": "x@example.com", "password": "ab", "role": "user"}), cookie)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for short password, got %d", resp.StatusCode)
	}
}

func TestCreateUserInvalidRole(t *testing.T) {
	srv, _ := newTestServer(t)
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")

	resp := doRequest(t, srv, http.MethodPost, "/api/users",
		jsonBody(map[string]string{"email": "x@example.com", "password": "pass123", "role": "superuser"}), cookie)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid role, got %d", resp.StatusCode)
	}
}

func TestUsersMethodNotAllowed(t *testing.T) {
	srv, _ := newTestServer(t)
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")

	resp := doRequest(t, srv, http.MethodDelete, "/api/users", "", cookie)
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.StatusCode)
	}
}

// ── handleUserActions edge cases ──────────────────────────────────────────────

func TestUserActionsInvalidID(t *testing.T) {
	srv, _ := newTestServer(t)
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")

	resp := doRequest(t, srv, http.MethodPatch, "/api/users/notanumber",
		jsonBody(map[string]string{"role": "user"}), cookie)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid id, got %d", resp.StatusCode)
	}
}

func TestUserActionsMethodNotAllowed(t *testing.T) {
	srv, _ := newTestServer(t)
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")

	resp := doRequest(t, srv, http.MethodPost, "/api/users/1",
		jsonBody(map[string]string{"role": "user"}), cookie)
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.StatusCode)
	}
}

func TestUserActionsNothingToUpdate(t *testing.T) {
	srv, st := newTestServer(t)
	ctx := context.Background()
	u, _ := st.CreateUser(ctx, "target@example.com", "", "pass123", "user", "", "")
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")

	resp := doRequest(t, srv, http.MethodPatch,
		"/api/users/"+strconv.FormatInt(u.ID, 10),
		jsonBody(map[string]string{}), cookie)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for nothing to update, got %d", resp.StatusCode)
	}
}

func TestUserActionsRoleLastAdmin(t *testing.T) {
	srv, _ := newTestServer(t)
	adminCookie := mustLogin(t, srv, "admin@example.com", "secret123")

	// get admin ID
	resp := doRequest(t, srv, http.MethodGet, "/api/users", "", adminCookie)
	var users []userResponse
	decodeJSON(t, resp, &users)
	var adminID int64
	for _, u := range users {
		if u.Role == "admin" {
			adminID = u.ID
		}
	}

	resp = doRequest(t, srv, http.MethodPatch,
		"/api/users/"+strconv.FormatInt(adminID, 10),
		jsonBody(map[string]string{"role": "user"}), adminCookie)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for last admin demotion, got %d", resp.StatusCode)
	}
}

func TestUserActionsRoleNotFound(t *testing.T) {
	srv, _ := newTestServer(t)
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")

	resp := doRequest(t, srv, http.MethodPatch, "/api/users/99999",
		jsonBody(map[string]string{"role": "user"}), cookie)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestUserActionsUpdatePassword(t *testing.T) {
	srv, st := newTestServer(t)
	ctx := context.Background()
	u, _ := st.CreateUser(ctx, "pwduser@example.com", "", "pass123", "user", "", "")
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")

	resp := doRequest(t, srv, http.MethodPatch,
		"/api/users/"+strconv.FormatInt(u.ID, 10),
		jsonBody(map[string]string{"password": "newpassword"}), cookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestUserActionsUpdatePasswordNotFound(t *testing.T) {
	srv, _ := newTestServer(t)
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")

	resp := doRequest(t, srv, http.MethodPatch, "/api/users/99999",
		jsonBody(map[string]string{"password": "newpassword"}), cookie)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestUserActionsUpdateNameFields(t *testing.T) {
	srv, st := newTestServer(t)
	ctx := context.Background()
	u, _ := st.CreateUser(ctx, "nameuser@example.com", "", "pass123", "user", "", "")
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")

	first := "Bob"
	last := "Smith"
	resp := doRequest(t, srv, http.MethodPatch,
		"/api/users/"+strconv.FormatInt(u.ID, 10),
		jsonBody(map[string]*string{"firstName": &first, "lastName": &last}), cookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var updated userResponse
	decodeJSON(t, resp, &updated)
	if updated.ID != u.ID {
		t.Fatalf("expected user ID %d, got %d", u.ID, updated.ID)
	}
}

// ── Project edge cases ────────────────────────────────────────────────────────

func TestProjectActionsInvalidID(t *testing.T) {
	srv, _ := newTestServer(t)
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")

	resp := doRequest(t, srv, http.MethodDelete, "/api/projects/notanumber", "", cookie)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestProjectActionsMethodNotAllowed(t *testing.T) {
	srv, _ := newTestServer(t)
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")

	resp := doRequest(t, srv, http.MethodPost, "/api/projects/1",
		jsonBody(map[string]string{"name": "x"}), cookie)
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.StatusCode)
	}
}

func TestDeleteProjectNotFound(t *testing.T) {
	srv, _ := newTestServer(t)
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")

	resp := doRequest(t, srv, http.MethodDelete, "/api/projects/99999", "", cookie)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for missing project, got %d", resp.StatusCode)
	}
}

func TestCreateProjectEmptyName(t *testing.T) {
	srv, _ := newTestServer(t)
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")

	resp := doRequest(t, srv, http.MethodPost, "/api/projects",
		jsonBody(map[string]string{"name": ""}), cookie)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty name, got %d", resp.StatusCode)
	}
}

func TestCreateProjectNameTooLong(t *testing.T) {
	srv, _ := newTestServer(t)
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")

	resp := doRequest(t, srv, http.MethodPost, "/api/projects",
		jsonBody(map[string]string{"name": strings.Repeat("x", 256)}), cookie)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for name too long, got %d", resp.StatusCode)
	}
}

func TestUserCreatesProjectGetsAccess(t *testing.T) {
	srv, st := newTestServer(t)
	ctx := context.Background()
	_, _ = st.CreateUser(ctx, "creator@example.com", "", "pass123", "user", "", "")
	userCookie := mustLogin(t, srv, "creator@example.com", "pass123")

	resp := doRequest(t, srv, http.MethodPost, "/api/projects",
		jsonBody(map[string]string{"name": "MyProject"}), userCookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// after create, user should see the project in their list
	listResp := doRequest(t, srv, http.MethodGet, "/api/projects", "", userCookie)
	var projects []store.Project
	decodeJSON(t, listResp, &projects)
	found := false
	for _, p := range projects {
		if p.Name == "MyProject" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected user to see newly created project")
	}
}

// ── Task edge cases ───────────────────────────────────────────────────────────

func TestListTasksInvalidProjectID(t *testing.T) {
	srv, _ := newTestServer(t)
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")

	resp := doRequest(t, srv, http.MethodGet, "/api/tasks?projectId=abc", "", cookie)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid projectId, got %d", resp.StatusCode)
	}
}

func TestCreateTaskTitleTooLong(t *testing.T) {
	srv, _ := newTestServer(t)
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")

	resp := doRequest(t, srv, http.MethodPost, "/api/tasks",
		jsonBody(map[string]any{"title": strings.Repeat("x", 501), "projectId": store.DefaultProjectID}), cookie)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for title too long, got %d", resp.StatusCode)
	}
}

func TestUpdateDescriptionNotFound(t *testing.T) {
	srv, _ := newTestServer(t)
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")

	resp := doRequest(t, srv, http.MethodPatch, "/api/tasks/99999",
		jsonBody(map[string]string{"description": "x"}), cookie)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestDeleteTaskNotFound(t *testing.T) {
	srv, _ := newTestServer(t)
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")

	resp := doRequest(t, srv, http.MethodDelete, "/api/tasks/99999", "", cookie)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestListTaskCommentsNotFound(t *testing.T) {
	srv, _ := newTestServer(t)
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")

	resp := doRequest(t, srv, http.MethodGet, "/api/tasks/99999/comments", "", cookie)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestAddCommentTaskNotFound(t *testing.T) {
	srv, _ := newTestServer(t)
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")

	resp := doRequest(t, srv, http.MethodPost, "/api/tasks/99999/comments",
		jsonBody(map[string]string{"body": "hi"}), cookie)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestAddCommentTooLong(t *testing.T) {
	srv, st := newTestServer(t)
	ctx := context.Background()
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")
	p, _ := st.CreateProject(ctx, "CommentProject")

	taskResp := doRequest(t, srv, http.MethodPost, "/api/tasks",
		jsonBody(map[string]any{"title": "T", "projectId": p.ID}), cookie)
	var task taskResponse
	decodeJSON(t, taskResp, &task)

	resp := doRequest(t, srv, http.MethodPost,
		"/api/tasks/"+strconv.FormatInt(task.ID, 10)+"/comments",
		jsonBody(map[string]string{"body": strings.Repeat("x", 5001)}), cookie)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for comment too long, got %d", resp.StatusCode)
	}
}

func TestDeleteCommentWrongTask(t *testing.T) {
	srv, st := newTestServer(t)
	ctx := context.Background()
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")
	p, _ := st.CreateProject(ctx, "CP2")

	t1Resp := doRequest(t, srv, http.MethodPost, "/api/tasks",
		jsonBody(map[string]any{"title": "T1", "projectId": p.ID}), cookie)
	var t1 taskResponse
	decodeJSON(t, t1Resp, &t1)

	t2Resp := doRequest(t, srv, http.MethodPost, "/api/tasks",
		jsonBody(map[string]any{"title": "T2", "projectId": p.ID}), cookie)
	var t2 taskResponse
	decodeJSON(t, t2Resp, &t2)

	addResp := doRequest(t, srv, http.MethodPost,
		"/api/tasks/"+strconv.FormatInt(t1.ID, 10)+"/comments",
		jsonBody(map[string]string{"body": "comment on t1"}), cookie)
	var comment store.TaskComment
	decodeJSON(t, addResp, &comment)

	// try to delete t1's comment via t2's route
	resp := doRequest(t, srv, http.MethodDelete,
		"/api/tasks/"+strconv.FormatInt(t2.ID, 10)+"/comments/"+strconv.FormatInt(comment.ID, 10),
		"", cookie)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for comment/task mismatch, got %d", resp.StatusCode)
	}
}

func TestDeleteCommentNotFound(t *testing.T) {
	srv, st := newTestServer(t)
	ctx := context.Background()
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")
	p, _ := st.CreateProject(ctx, "CP3")

	taskResp := doRequest(t, srv, http.MethodPost, "/api/tasks",
		jsonBody(map[string]any{"title": "T", "projectId": p.ID}), cookie)
	var task taskResponse
	decodeJSON(t, taskResp, &task)

	resp := doRequest(t, srv, http.MethodDelete,
		"/api/tasks/"+strconv.FormatInt(task.ID, 10)+"/comments/99999",
		"", cookie)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for missing comment, got %d", resp.StatusCode)
	}
}

func TestDeleteCommentNotAuthor(t *testing.T) {
	srv, st := newTestServer(t)
	ctx := context.Background()
	_, _ = st.CreateUser(ctx, "other@example.com", "", "pass123", "user", "", "")
	adminCookie := mustLogin(t, srv, "admin@example.com", "secret123")
	otherCookie := mustLogin(t, srv, "other@example.com", "pass123")
	p, _ := st.CreateProject(ctx, "CP4")

	taskResp := doRequest(t, srv, http.MethodPost, "/api/tasks",
		jsonBody(map[string]any{"title": "T", "projectId": p.ID}), adminCookie)
	var task taskResponse
	decodeJSON(t, taskResp, &task)

	addResp := doRequest(t, srv, http.MethodPost,
		"/api/tasks/"+strconv.FormatInt(task.ID, 10)+"/comments",
		jsonBody(map[string]string{"body": "admin comment"}), adminCookie)
	var comment store.TaskComment
	decodeJSON(t, addResp, &comment)

	// other user tries to delete admin's comment from their own task access
	// (other user has DefaultProjectID access via inbox, but p is admin-only)
	// Let's assign other user to this project first
	otherUsers, _ := st.ListUsers(ctx)
	var otherID int64
	for _, u := range otherUsers {
		if u.Email == "other@example.com" {
			otherID = u.ID
		}
	}
	_ = st.SetUserProjects(ctx, otherID, []int64{store.DefaultProjectID, p.ID})

	resp := doRequest(t, srv, http.MethodDelete,
		"/api/tasks/"+strconv.FormatInt(task.ID, 10)+"/comments/"+strconv.FormatInt(comment.ID, 10),
		"", otherCookie)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 when non-author deletes comment, got %d", resp.StatusCode)
	}
}

// ── Login blocked user ────────────────────────────────────────────────────────

func TestLoginBlockedUser(t *testing.T) {
	srv, st := newTestServer(t)
	ctx := context.Background()
	u, _ := st.CreateUser(ctx, "blocked2@example.com", "", "pass123", "user", "", "")
	_, _ = st.UpdateUserRole(ctx, u.ID, "blocked")

	resp := doRequest(t, srv, http.MethodPost, "/api/auth/login",
		jsonBody(map[string]string{"login": "blocked2@example.com", "password": "pass123"}), nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for blocked user login, got %d", resp.StatusCode)
	}
}

// ── Register edge cases ───────────────────────────────────────────────────────

func TestRegisterMissingPassword(t *testing.T) {
	srv, _ := newTestServer(t)

	resp := doRequest(t, srv, http.MethodPost, "/api/auth/register",
		jsonBody(map[string]string{"email": "x@example.com"}), nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestRegisterShortPassword(t *testing.T) {
	srv, _ := newTestServer(t)

	resp := doRequest(t, srv, http.MethodPost, "/api/auth/register",
		jsonBody(map[string]string{"email": "x@example.com", "password": "ab"}), nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for short password, got %d", resp.StatusCode)
	}
}

// ── Profile edge cases ────────────────────────────────────────────────────────

func TestProfileUpdatePassword(t *testing.T) {
	srv, _ := newTestServer(t)
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")

	newPass := "newsecret"
	resp := doRequest(t, srv, http.MethodPatch, "/api/profile",
		jsonBody(map[string]*string{"password": &newPass}), cookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestProfileUpdatePasswordTooShort(t *testing.T) {
	srv, _ := newTestServer(t)
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")

	short := "ab"
	resp := doRequest(t, srv, http.MethodPatch, "/api/profile",
		jsonBody(map[string]*string{"password": &short}), cookie)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for short password, got %d", resp.StatusCode)
	}
}

func TestProfileSetUsername(t *testing.T) {
	srv, st := newTestServer(t)
	ctx := context.Background()
	_, _ = st.CreateUser(ctx, "usernamer@example.com", "", "pass123", "user", "", "")
	cookie := mustLogin(t, srv, "usernamer@example.com", "pass123")

	uname := "mynewname"
	resp := doRequest(t, srv, http.MethodPatch, "/api/profile",
		jsonBody(map[string]*string{"username": &uname}), cookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var me meResponse
	decodeJSON(t, resp, &me)
	if me.Username != "mynewname" {
		t.Fatalf("expected username mynewname, got %q", me.Username)
	}
}

func TestProfileSetUsernameTwiceForbidden(t *testing.T) {
	srv, st := newTestServer(t)
	ctx := context.Background()
	_, _ = st.CreateUser(ctx, "usernamer2@example.com", "", "pass123", "user", "", "")
	cookie := mustLogin(t, srv, "usernamer2@example.com", "pass123")

	uname := "firstname"
	doRequest(t, srv, http.MethodPatch, "/api/profile",
		jsonBody(map[string]*string{"username": &uname}), cookie)

	other := "secondname"
	resp := doRequest(t, srv, http.MethodPatch, "/api/profile",
		jsonBody(map[string]*string{"username": &other}), cookie)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for changing username, got %d", resp.StatusCode)
	}
}

func TestProfileBlockedUserForbidden(t *testing.T) {
	srv, st := newTestServer(t)
	ctx := context.Background()
	u, _ := st.CreateUser(ctx, "blocked3@example.com", "", "pass123", "user", "", "")
	_, _ = st.UpdateUserRole(ctx, u.ID, "blocked")

	blockedToken := createToken(store.User{ID: u.ID, Role: "blocked"}, testSecret)
	blockedCookie := &http.Cookie{Name: "auth", Value: blockedToken}

	resp := doRequest(t, srv, http.MethodGet, "/api/profile", "", blockedCookie)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for blocked user on /api/profile, got %d", resp.StatusCode)
	}
}

func TestProfileMethodNotAllowed(t *testing.T) {
	srv, _ := newTestServer(t)
	cookie := mustLogin(t, srv, "admin@example.com", "secret123")

	resp := doRequest(t, srv, http.MethodDelete, "/api/profile", "", cookie)
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.StatusCode)
	}
}

// ── Restricted user access ────────────────────────────────────────────────────

func TestRestrictedUserCannotListForbiddenProject(t *testing.T) {
	srv, st := newTestServer(t)
	ctx := context.Background()
	_, _ = st.CreateUser(ctx, "restricted@example.com", "", "pass123", "user", "", "")
	userCookie := mustLogin(t, srv, "restricted@example.com", "pass123")
	adminCookie := mustLogin(t, srv, "admin@example.com", "secret123")

	p, _ := st.CreateProject(ctx, "AdminProject")

	// admin creates task in that project
	doRequest(t, srv, http.MethodPost, "/api/tasks",
		jsonBody(map[string]any{"title": "Secret", "projectId": p.ID}), adminCookie)

	// restricted user: list all tasks (no projectId) with isRestricted — should get 403
	resp := doRequest(t, srv, http.MethodGet,
		"/api/tasks?projectId="+strconv.FormatInt(p.ID, 10), "", userCookie)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for restricted user on forbidden project, got %d", resp.StatusCode)
	}
}

func TestRestrictedUserCanCreateAndAccessOwnProject(t *testing.T) {
	srv, st := newTestServer(t)
	ctx := context.Background()
	_, _ = st.CreateUser(ctx, "selfproject@example.com", "", "pass123", "user", "", "")
	userCookie := mustLogin(t, srv, "selfproject@example.com", "pass123")

	// user creates a project — should get access automatically
	pResp := doRequest(t, srv, http.MethodPost, "/api/projects",
		jsonBody(map[string]string{"name": "SelfProject"}), userCookie)
	if pResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", pResp.StatusCode)
	}
	var p store.Project
	decodeJSON(t, pResp, &p)

	// now list tasks in that project — should succeed
	resp := doRequest(t, srv, http.MethodGet,
		"/api/tasks?projectId="+strconv.FormatInt(p.ID, 10), "", userCookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}
