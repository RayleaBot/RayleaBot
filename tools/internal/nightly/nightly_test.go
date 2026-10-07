package nightly

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

const testSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func sample(number int64) RunInfo {
	r := RunInfo{ID: number, Number: number, Attempt: 1, SHA: testSHA, Branch: "main", Event: "workflow_dispatch", Status: "completed", Conclusion: "success"}
	r.HeadRepository.FullName = "RayleaBot/RayleaBot"
	return r
}

type fixture struct {
	runs       []RunInfo
	run        *RunInfo
	issue      *Issue
	jobs       map[int][]Job
	writes     []map[string]string
	methods    []string
	apiError   bool
	issuePages bool
}

func fake(t *testing.T, f *fixture) *GitHub {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if f.apiError {
			http.Error(w, "unavailable", 503)
			return
		}
		if r.Header.Get("Authorization") != "Bearer fixture-token" {
			t.Error("missing authentication")
		}
		path := strings.TrimPrefix(r.URL.Path, "/repos/RayleaBot/RayleaBot/")
		var value any
		if r.Method != "GET" {
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			f.writes = append(f.writes, body)
			f.methods = append(f.methods, r.Method+" "+path)
			value = map[string]int{"number": 7}
		} else {
			switch {
			case path == "":
				value = map[string]string{"default_branch": "main"}
			case path == "actions/workflows/nightly.yml/runs":
				value = map[string]any{"workflow_runs": f.runs}
			case path == "issues":
				issues := []Issue{}
				if f.issuePages && r.URL.Query().Get("page") == "1" {
					issues = make([]Issue, 100)
				} else if f.issue != nil {
					issues = append(issues, *f.issue)
				}
				value = issues
			case strings.HasSuffix(path, "/jobs"):
				if r.URL.Query().Get("filter") != "latest" {
					t.Error("jobs from older attempts included")
				}
				page, _ := strconv.Atoi(r.URL.Query().Get("page"))
				value = map[string]any{"jobs": f.jobs[page]}
			case strings.HasPrefix(path, "actions/runs/"):
				if f.run != nil {
					value = f.run
				} else {
					id, _ := strconv.ParseInt(strings.TrimPrefix(path, "actions/runs/"), 10, 64)
					for _, run := range f.runs {
						if run.ID == id {
							value = run
							break
						}
					}
				}
			default:
				t.Errorf("unexpected endpoint %s", path)
			}
		}
		if err := json.NewEncoder(w).Encode(value); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	api, err := New("RayleaBot/RayleaBot", "fixture-token")
	if err != nil {
		t.Fatal(err)
	}
	api.APIURL = server.URL
	api.Client = server.Client()
	return api
}
func TestGateRejectsUntrustedMissingOrUnsuccessfulEvidence(t *testing.T) {
	for _, change := range []func(*RunInfo){func(r *RunInfo) { r.SHA = strings.Repeat("b", 40) }, func(r *RunInfo) { r.Event = "pull_request" }, func(r *RunInfo) { r.HeadRepository.FullName = "fork/repo" }, func(r *RunInfo) { r.Conclusion = "failure" }, func(r *RunInfo) { r.Conclusion = "cancelled" }, func(r *RunInfo) { r.Status = "in_progress" }, func(r *RunInfo) { r.Status = "queued" }} {
		r := sample(2)
		change(&r)
		api := fake(t, &fixture{runs: []RunInfo{r}})
		if _, err := api.RequireSuccess(testSHA); err == nil {
			t.Fatalf("accepted %+v", r)
		}
	}
	for _, f := range []*fixture{{}, {apiError: true}} {
		if _, err := fake(t, f).RequireSuccess(testSHA); err == nil {
			t.Fatal("accepted missing evidence")
		}
	}
	api := fake(t, &fixture{runs: []RunInfo{sample(1)}})
	if _, err := api.RequireSuccess(testSHA[:7]); err == nil {
		t.Fatal("accepted abbreviated SHA")
	}
	if run, err := api.RequireSuccess(strings.ToUpper(testSHA)); err != nil || run.ID != 1 {
		t.Fatalf("%+v %v", run, err)
	}
}
func TestLatestRunAndAttempt(t *testing.T) {
	for _, status := range []string{"completed", "in_progress", "queued"} {
		bad := sample(2)
		bad.Status = status
		bad.Conclusion = "failure"
		if _, err := fake(t, &fixture{runs: []RunInfo{sample(1), bad}}).RequireSuccess(testSHA); err == nil {
			t.Fatal("old success hid latest attempt")
		}
	}
	bad := sample(1)
	bad.Attempt = 2
	bad.Conclusion = "failure"
	if _, err := fake(t, &fixture{runs: []RunInfo{sample(1), bad}}).RequireSuccess(testSHA); err == nil {
		t.Fatal("old attempt approved")
	}
	run, err := fake(t, &fixture{runs: []RunInfo{sample(2), bad}}).RequireSuccess(testSHA)
	if err != nil || run.ID != 2 {
		t.Fatalf("%+v %v", run, err)
	}
}
func tracking(state string) *Issue {
	return &Issue{Number: 7, Title: issueTitle, Body: issueMarker, State: state}
}
func TestIssueLifecycle(t *testing.T) {
	for _, state := range []string{"", "open", "closed"} {
		for _, conclusion := range []string{"success", "failure"} {
			t.Run(state+conclusion, func(t *testing.T) {
				r := sample(1)
				r.Conclusion = conclusion
				f := &fixture{runs: []RunInfo{r}, jobs: map[int][]Job{1: {{"server", "failure"}}}}
				if state != "" {
					f.issue = tracking(state)
				}
				if _, err := fake(t, f).Reconcile(1); err != nil {
					t.Fatal(err)
				}
				if conclusion == "success" && state != "open" {
					if len(f.writes) != 0 {
						t.Fatal("success created notification")
					}
					return
				}
				if len(f.writes) != 1 {
					t.Fatal(f.writes)
				}
				want := "PATCH issues/7"
				if state == "" {
					want = "POST issues"
				}
				if f.methods[0] != want {
					t.Fatal(f.methods)
				}
				body := f.writes[0]
				if !strings.Contains(body["body"], issueMarker) || !strings.Contains(body["body"], testSHA) {
					t.Fatal("missing tracking identity")
				}
				if state != "" {
					wantState := "open"
					if conclusion == "success" {
						wantState = "closed"
					}
					if body["state"] != wantState {
						t.Fatal(body)
					}
				}
			})
		}
	}
}
func TestIgnoredReportsHaveNoWrites(t *testing.T) {
	cases := []fixture{{runs: []RunInfo{sample(1), sample(2)}}}
	for _, change := range []func(*RunInfo){func(r *RunInfo) { r.Branch = "feature" }, func(r *RunInfo) { r.HeadRepository.FullName = "fork/repo" }, func(r *RunInfo) { r.Conclusion = "cancelled" }, func(r *RunInfo) { r.Status = "in_progress" }} {
		r := sample(1)
		change(&r)
		cases = append(cases, fixture{runs: []RunInfo{r}})
	}
	for _, change := range []func(*RunInfo){func(r *RunInfo) { r.Attempt = 2 }, func(r *RunInfo) { r.Status = "in_progress" }, func(r *RunInfo) { r.Conclusion = "failure" }} {
		original := sample(1)
		latest := original
		change(&latest)
		cases = append(cases, fixture{runs: []RunInfo{latest}, run: &original})
	}
	for i := range cases {
		f := &cases[i]
		f.issue = tracking("open")
		if _, err := fake(t, f).Reconcile(1); err != nil {
			t.Fatal(err)
		}
		if len(f.writes) != 0 {
			t.Fatalf("case %d wrote", i)
		}
	}
}
func TestPaginationAndJobSanitization(t *testing.T) {
	r := sample(1)
	r.Conclusion = "failure"
	f := &fixture{runs: []RunInfo{r}, issue: tracking("open"), issuePages: true, jobs: map[int][]Job{1: make([]Job, 100), 2: {{"windows@bot`\njob", "failure"}}}}
	if _, err := fake(t, f).Reconcile(1); err != nil {
		t.Fatal(err)
	}
	if len(f.writes) != 1 || f.methods[0] != "PATCH issues/7" {
		t.Fatal(f.methods)
	}
	body := f.writes[0]["body"]
	if !strings.Contains(body, "windows＠bot' job") || strings.Contains(body, "@bot") {
		t.Fatal("job report missing or unsanitized")
	}
}
func TestCLI(t *testing.T) {
	f := &fixture{runs: []RunInfo{sample(1)}}
	api := fake(t, f)
	t.Setenv("GITHUB_API_URL", api.APIURL)
	t.Setenv("GITHUB_TOKEN", "fixture-token")
	var out, stderr strings.Builder
	for _, command := range []string{"check", "report"} {
		args := []string{command, "--repository", api.Repository, "--sha", testSHA, "--run-id", "1"}
		if code := Run(args, &out, &stderr); code != 0 {
			t.Fatalf("%s: %d %s", command, code, stderr.String())
		}
	}
	if len(f.writes) != 0 {
		t.Fatal(fmt.Sprint(f.writes))
	}
}
