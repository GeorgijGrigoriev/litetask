package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Issue struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	State       string `json:"state"`
	HTMLURL     string `json:"html_url"`
	Labels      []struct {
		Name string `json:"name"`
	} `json:"labels"`
	User struct {
		Login string `json:"login"`
	} `json:"user"`
	PullRequest *struct{} `json:"pull_request"`
	CreatedAt   time.Time `json:"created_at"`
}

type IssueComment struct {
	ID   int64  `json:"id"`
	Body string `json:"body"`
	User struct {
		Login string `json:"login"`
	} `json:"user"`
	CreatedAt time.Time `json:"created_at"`
}

type Client struct {
	token string
	http  *http.Client
}

func NewClient(token string) *Client {
	return &Client{
		token: token,
		http:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	return c.http.Do(req)
}

// IssueToStatus maps a GitHub issue state and labels to a LiteTask status.
func IssueToStatus(issue Issue) string {
	if issue.State == "closed" {
		return "done"
	}
	for _, label := range issue.Labels {
		name := strings.ToLower(label.Name)
		if strings.Contains(name, "in progress") || strings.Contains(name, "in-progress") || strings.Contains(name, "wip") {
			return "in_progress"
		}
	}
	return "new"
}

func (c *Client) ListIssues(ctx context.Context, owner, repo string) ([]Issue, error) {
	var all []Issue
	for page := 1; ; page++ {
		url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues?state=all&per_page=100&page=%d", owner, repo, page)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		resp, err := c.do(req)
		if err != nil {
			return nil, err
		}
		var issues []Issue
		decErr := json.NewDecoder(resp.Body).Decode(&issues)
		resp.Body.Close() //nolint:errcheck
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("github: list issues returned %d", resp.StatusCode)
		}
		if decErr != nil {
			return nil, decErr
		}
		all = append(all, issues...)
		if len(issues) < 100 {
			break
		}
	}
	return all, nil
}

func (c *Client) GetIssueComments(ctx context.Context, owner, repo string, n int) ([]IssueComment, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%d/comments?per_page=100", owner, repo, n)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github: get comments returned %d", resp.StatusCode)
	}
	var comments []IssueComment
	return comments, json.NewDecoder(resp.Body).Decode(&comments)
}

func (c *Client) SetIssueState(ctx context.Context, owner, repo string, n int, state string) error {
	body := fmt.Sprintf(`{"state":%q}`, state)
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%d", owner, repo, n)
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("github: set issue state returned %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) ListRepos(ctx context.Context) ([]string, error) {
	var all []string
	for page := 1; ; page++ {
		url := fmt.Sprintf("https://api.github.com/user/repos?per_page=100&page=%d&sort=updated", page)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		resp, err := c.do(req)
		if err != nil {
			return nil, err
		}
		var repos []struct {
			FullName string `json:"full_name"`
		}
		decErr := json.NewDecoder(resp.Body).Decode(&repos)
		resp.Body.Close() //nolint:errcheck
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("github: list repos returned %d", resp.StatusCode)
		}
		if decErr != nil {
			return nil, decErr
		}
		for _, r := range repos {
			all = append(all, r.FullName)
		}
		if len(repos) < 100 {
			break
		}
	}
	return all, nil
}
