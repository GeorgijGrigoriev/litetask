package github

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"litetask/internal/store"
)

type Syncer struct {
	store   store.Storer
	authKey []byte
}

type SyncResult struct {
	Created int      `json:"created"`
	Updated int      `json:"updated"`
	Skipped int      `json:"skipped"`
	Errors  []string `json:"errors"`
}

func NewSyncer(s store.Storer, authKey []byte) *Syncer {
	return &Syncer{store: s, authKey: authKey}
}

func (s *Syncer) Sync(ctx context.Context, ig store.GitHubIntegration) (SyncResult, error) {
	var result SyncResult

	token, err := DecryptToken(ig.AccessToken, s.authKey)
	if err != nil {
		return result, fmt.Errorf("decrypt token: %w", err)
	}

	parts := strings.SplitN(ig.RepoFullName, "/", 2)
	if len(parts) != 2 {
		return result, fmt.Errorf("invalid repo: %s", ig.RepoFullName)
	}
	owner, repo := parts[0], parts[1]

	client := NewClient(token)

	issues, err := client.ListIssues(ctx, owner, repo)
	if err != nil {
		return result, fmt.Errorf("list issues: %w", err)
	}

	for _, issue := range issues {
		if issue.PullRequest != nil {
			continue
		}
		externalID := strconv.Itoa(issue.Number)
		status := IssueToStatus(issue)

		existing, err := s.store.GetTaskByExternalID(ctx, externalID, "github", ig.ProjectID)
		if errors.Is(err, sql.ErrNoRows) {
			task, createErr := s.store.InsertTask(ctx, issue.Title, issue.Body, "github:@"+issue.User.Login, ig.ProjectID, 0)
			if createErr != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("issue %d: create: %v", issue.Number, createErr))
				continue
			}
			if setErr := s.store.SetTaskExternal(ctx, task.ID, externalID, "github", issue.HTMLURL); setErr != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("issue %d: set external: %v", issue.Number, setErr))
			}
			if status != "new" {
				if _, setErr := s.store.SetTaskStatus(ctx, task.ID, status); setErr != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("issue %d: set status: %v", issue.Number, setErr))
				}
			}
			s.syncComments(ctx, client, owner, repo, issue.Number, task.ID, &result)
			result.Created++
			continue
		}
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("issue %d: lookup: %v", issue.Number, err))
			continue
		}

		changed := false
		if existing.Title != issue.Title {
			if _, titleErr := s.store.SetTaskTitle(ctx, existing.ID, issue.Title); titleErr != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("issue %d: update title: %v", issue.Number, titleErr))
			} else {
				changed = true
			}
		}
		if existing.Description != issue.Body {
			if _, descErr := s.store.SetTaskDescription(ctx, existing.ID, issue.Body); descErr != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("issue %d: update desc: %v", issue.Number, descErr))
			} else {
				changed = true
			}
		}
		if existing.Status != status {
			if _, statusErr := s.store.SetTaskStatus(ctx, existing.ID, status); statusErr != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("issue %d: update status: %v", issue.Number, statusErr))
			} else {
				changed = true
			}
		}
		s.syncComments(ctx, client, owner, repo, issue.Number, existing.ID, &result)
		if changed {
			result.Updated++
		} else {
			result.Skipped++
		}
	}

	// Reverse sync: push local task status changes back to GitHub issues.
	localTasks, listErr := s.store.ListTasksWithExternalSource(ctx, ig.ProjectID, "github")
	if listErr != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("list external tasks: %v", listErr))
	} else {
		for _, task := range localTasks {
			issueNum, parseErr := strconv.Atoi(task.ExternalID)
			if parseErr != nil || issueNum <= 0 {
				continue
			}
			wantState := "open"
			if task.Status == "done" {
				wantState = "closed"
			}
			if setErr := client.SetIssueState(ctx, owner, repo, issueNum, wantState); setErr != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("task %d: set issue state: %v", task.ID, setErr))
			}
		}
	}

	_ = s.store.UpdateGitHubIntegrationSyncedAt(ctx, ig.ID, time.Now())

	return result, nil
}

func (s *Syncer) syncComments(ctx context.Context, client *Client, owner, repo string, issueNum int, taskID int64, result *SyncResult) {
	ghComments, err := client.GetIssueComments(ctx, owner, repo, issueNum)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("issue %d: get comments: %v", issueNum, err))
		return
	}
	existing, err := s.store.ListTaskComments(ctx, taskID)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("task %d: list comments: %v", taskID, err))
		return
	}
	for _, ghc := range ghComments {
		marker := fmt.Sprintf("[github:#%d @%s]", ghc.ID, ghc.User.Login)
		already := false
		for _, ec := range existing {
			if strings.Contains(ec.Body, marker) {
				already = true
				break
			}
		}
		if !already {
			body := marker + " " + ghc.Body
			if _, addErr := s.store.AddTaskComment(ctx, taskID, body, 0); addErr != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("task %d: add comment: %v", taskID, addErr))
			}
		}
	}
}
