package nightly

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/tools/internal/cli"
)

const issueTitle = "[nightly] 自动回归失败"
const issueMarker = "<!-- rayleabot:nightly-status -->"

type GitHub struct {
	Repository, Token, APIURL, ServerURL string
	Client                               *http.Client
}
type RunInfo struct {
	ID             int64  `json:"id"`
	Number         int64  `json:"run_number"`
	Attempt        int64  `json:"run_attempt"`
	SHA            string `json:"head_sha"`
	Branch         string `json:"head_branch"`
	Event          string `json:"event"`
	Status         string `json:"status"`
	Conclusion     string `json:"conclusion"`
	HeadRepository struct {
		FullName string `json:"full_name"`
	} `json:"head_repository"`
}
type Issue struct {
	Number             int64
	Title, Body, State string
	PullRequest        json.RawMessage `json:"pull_request"`
}
type Job struct{ Name, Conclusion string }

func New(repository, token string) (*GitHub, error) {
	if !regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`).MatchString(repository) {
		return nil, fmt.Errorf("repository must be owner/name")
	}
	env := func(key, def string) string {
		if v, ok := os.LookupEnv(key); ok {
			return strings.TrimRight(v, "/")
		}
		return def
	}
	return &GitHub{repository, token, env("GITHUB_API_URL", "https://api.github.com"), env("GITHUB_SERVER_URL", "https://github.com"), &http.Client{Timeout: 30 * time.Second}}, nil
}
func (g *GitHub) request(method, path string, body any, target any) error {
	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	req, err := http.NewRequest(method, g.APIURL+"/repos/"+g.Repository+"/"+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "RayleaBot-nightly")
	if g.Token != "" {
		req.Header.Set("Authorization", "Bearer "+g.Token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := g.Client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= 400 {
		base, _, _ := strings.Cut(path, "?")
		return fmt.Errorf("GitHub %s %s returned HTTP %d", method, base, response.StatusCode)
	}
	if target == nil {
		_, err = io.Copy(io.Discard, response.Body)
		return err
	}
	return json.NewDecoder(response.Body).Decode(target)
}
func (g *GitHub) runURL(run RunInfo) string {
	return fmt.Sprintf("%s/%s/actions/runs/%d", g.ServerURL, g.Repository, run.ID)
}
func (g *GitHub) own(run RunInfo) bool {
	return slices.Contains([]string{"schedule", "workflow_dispatch"}, run.Event) && strings.EqualFold(run.HeadRepository.FullName, g.Repository)
}
func attempt(run RunInfo) int64 {
	if run.Attempt == 0 {
		return 1
	}
	return run.Attempt
}
func newest(runs []RunInfo) *RunInfo {
	var latest *RunInfo
	for i := range runs {
		r := &runs[i]
		if latest == nil || r.Number > latest.Number || r.Number == latest.Number && attempt(*r) > attempt(*latest) {
			latest = r
		}
	}
	return latest
}
func (g *GitHub) runs(query url.Values) ([]RunInfo, error) {
	var response struct {
		Runs []RunInfo `json:"workflow_runs"`
	}
	err := g.request("GET", "actions/workflows/nightly.yml/runs?"+query.Encode(), nil, &response)
	var own []RunInfo
	for _, r := range response.Runs {
		if g.own(r) {
			own = append(own, r)
		}
	}
	return own, err
}
func (g *GitHub) RequireSuccess(sha string) (*RunInfo, error) {
	if !regexp.MustCompile(`^[0-9a-fA-F]{40}$`).MatchString(sha) {
		return nil, fmt.Errorf("nightly gate requires the full commit SHA")
	}
	sha = strings.ToLower(sha)
	runs, err := g.runs(url.Values{"head_sha": {sha}, "per_page": {"100"}})
	if err != nil {
		return nil, err
	}
	var matching []RunInfo
	for _, r := range runs {
		if r.SHA == sha {
			matching = append(matching, r)
		}
	}
	latest := newest(matching)
	if latest == nil {
		return nil, fmt.Errorf("no nightly run for %s; run nightly on this commit before tagging a release", sha)
	}
	if latest.Status != "completed" || latest.Conclusion != "success" {
		return nil, fmt.Errorf("latest nightly for %s is %s/%s: %s", sha, latest.Status, latest.Conclusion, g.runURL(*latest))
	}
	return latest, nil
}
func (g *GitHub) trackingIssue() (*Issue, error) {
	for page := 1; page <= 100; page++ {
		query := url.Values{"state": {"all"}, "creator": {"github-actions[bot]"}, "per_page": {"100"}, "page": {fmt.Sprint(page)}}
		var issues []Issue
		if err := g.request("GET", "issues?"+query.Encode(), nil, &issues); err != nil {
			return nil, err
		}
		for _, issue := range issues {
			if (len(issue.PullRequest) == 0 || string(issue.PullRequest) == "null") && issue.Title == issueTitle && strings.Contains(issue.Body, issueMarker) {
				return &issue, nil
			}
		}
		if len(issues) < 100 {
			return nil, nil
		}
	}
	return nil, fmt.Errorf("nightly issue search exceeded its pagination limit")
}
func (g *GitHub) failedJobs(id int64) ([]Job, error) {
	var result []Job
	for page := 1; page <= 100; page++ {
		query := url.Values{"filter": {"latest"}, "per_page": {"100"}, "page": {fmt.Sprint(page)}}
		var response struct{ Jobs []Job }
		if err := g.request("GET", fmt.Sprintf("actions/runs/%d/jobs?%s", id, query.Encode()), nil, &response); err != nil {
			return nil, err
		}
		for _, job := range response.Jobs {
			if slices.Contains([]string{"failure", "timed_out", "action_required"}, job.Conclusion) {
				result = append(result, job)
			}
		}
		if len(response.Jobs) < 100 {
			return result, nil
		}
	}
	return nil, fmt.Errorf("nightly jobs exceeded the pagination limit")
}
func (g *GitHub) Reconcile(id int64) (string, error) {
	var run RunInfo
	if err := g.request("GET", fmt.Sprintf("actions/runs/%d", id), nil, &run); err != nil {
		return "", err
	}
	var repo struct {
		Branch string `json:"default_branch"`
	}
	if err := g.request("GET", "", nil, &repo); err != nil {
		return "", err
	}
	if !g.own(run) || run.Branch != repo.Branch || run.Status != "completed" {
		return "ignored non-default, foreign, or unfinished run", nil
	}
	runs, err := g.runs(url.Values{"branch": {repo.Branch}, "per_page": {"100"}})
	if err != nil {
		return "", err
	}
	latest := newest(runs)
	if latest == nil || latest.ID != id || attempt(*latest) != attempt(run) {
		return "ignored superseded run", nil
	}
	if latest.Conclusion != run.Conclusion || latest.Status != "completed" {
		return "ignored run changed during reconciliation", nil
	}
	if !slices.Contains([]string{"success", "failure", "timed_out", "action_required", "startup_failure"}, run.Conclusion) {
		return "ignored cancelled or non-actionable run", nil
	}
	issue, err := g.trackingIssue()
	if err != nil {
		return "", err
	}
	success := run.Conclusion == "success"
	if success && (issue == nil || issue.State == "closed") {
		return "nightly passed; no open issue", nil
	}
	status := "失败"
	if success {
		status = "已恢复"
	}
	body := fmt.Sprintf("%s\n\nNightly **%s**。\n\n- 提交：`%s`\n- 运行：[#%d](%s)（第 %d 次尝试）\n", issueMarker, status, run.SHA, run.Number, g.runURL(run), attempt(run))
	if !success {
		jobs, err := g.failedJobs(id)
		if err != nil {
			return "", err
		}
		for _, job := range jobs {
			name := strings.NewReplacer("`", "'", "@", "＠", "\n", " ").Replace(job.Name)
			body += fmt.Sprintf("- `%s`：%s\n", name, job.Conclusion)
		}
		body += "\n修复后对同一提交重新运行 nightly；只有最新运行成功才允许发布该提交。\n"
	}
	if issue == nil {
		err = g.request("POST", "issues", map[string]string{"title": issueTitle, "body": body}, nil)
		return "created nightly issue", err
	}
	update := map[string]string{"body": body, "state": "open"}
	if success {
		update["state"] = "closed"
		update["state_reason"] = "completed"
	}
	err = g.request("PATCH", fmt.Sprintf("issues/%d", issue.Number), update, nil)
	return "updated nightly issue", err
}
func Run(args []string, out, stderr io.Writer) int {
	fs := flag.NewFlagSet("nightly-status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cli.Usage(fs, "nightly-status {check,report} [options]", "Require successful nightly evidence or reconcile its single tracking issue.")
	repository := fs.String("repository", os.Getenv("GITHUB_REPOSITORY"), "owner/name")
	sha := fs.String("sha", "", "full commit SHA")
	id := fs.Int64("run-id", 0, "nightly run ID")
	if err := cli.Parse(fs, args, 1, 1); err != nil {
		return cli.ErrorTo(stderr, err)
	}
	command := fs.Arg(0)
	if command != "check" && command != "report" {
		fmt.Fprintln(stderr, "expected check or report")
		return 2
	}
	api, err := New(*repository, os.Getenv("GITHUB_TOKEN"))
	result := ""
	if err == nil {
		if command == "check" {
			var run *RunInfo
			run, err = api.RequireSuccess(*sha)
			if err == nil {
				result = "nightly gate passed: " + api.runURL(*run)
			}
		} else if *id == 0 || api.Token == "" {
			err = fmt.Errorf("report requires a run ID and GITHUB_TOKEN with issues:write")
		} else {
			result, err = api.Reconcile(*id)
		}
	}
	if err != nil {
		fmt.Fprintf(stderr, "nightly %s failed: %v\n", command, err)
		return 1
	}
	fmt.Fprintln(out, result)
	return 0
}
