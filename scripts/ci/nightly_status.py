"""Require successful nightly evidence or reconcile its single tracking issue."""
from __future__ import annotations

import argparse
import json
import os
import re
import sys
from urllib.error import HTTPError, URLError
from urllib.parse import urlencode
from urllib.request import Request, urlopen

WORKFLOW = "nightly.yml"
ISSUE_TITLE = "[nightly] 自动回归失败"
ISSUE_MARKER = "<!-- rayleabot:nightly-status -->"
NIGHTLY_EVENTS = {"schedule", "workflow_dispatch"}


class GitHub:
    def __init__(self, repository: str, token: str):
        if not re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", repository):
            raise ValueError("repository must be owner/name")
        self.repository = repository
        self.token = token
        self.api_url = os.environ.get("GITHUB_API_URL", "https://api.github.com").rstrip("/")
        self.server_url = os.environ.get("GITHUB_SERVER_URL", "https://github.com").rstrip("/")

    def request(self, method: str, path: str, body: dict | None = None):
        headers = {"Accept": "application/vnd.github+json", "User-Agent": "RayleaBot-nightly"}
        if self.token:
            headers["Authorization"] = "Bearer " + self.token
        data = None
        if body is not None:
            headers["Content-Type"] = "application/json"
            data = json.dumps(body, ensure_ascii=False).encode("utf-8")
        request = Request(f"{self.api_url}/repos/{self.repository}/{path}", data=data, headers=headers, method=method)
        try:
            with urlopen(request, timeout=30) as response:
                return json.load(response)
        except HTTPError as exc:
            raise RuntimeError(f"GitHub {method} {path.split('?')[0]} returned HTTP {exc.code}") from exc

    def run_url(self, run: dict) -> str:
        return f"{self.server_url}/{self.repository}/actions/runs/{int(run['id'])}"


def own_nightly(api: GitHub, run: dict) -> bool:
    return (
        run.get("event") in NIGHTLY_EVENTS
        and (run.get("head_repository") or {}).get("full_name", "").casefold() == api.repository.casefold()
    )

def newest_run(runs: list[dict]) -> dict | None:
    return max(runs, key=lambda run: (int(run["run_number"]), int(run.get("run_attempt", 1))), default=None)


def require_successful_nightly(api: GitHub, sha: str) -> dict:
    if not re.fullmatch(r"[0-9a-fA-F]{40}", sha):
        raise ValueError("nightly gate requires the full commit SHA")
    sha = sha.lower()
    query = urlencode({"head_sha": sha, "per_page": 100})
    response = api.request("GET", f"actions/workflows/{WORKFLOW}/runs?{query}")
    runs = [run for run in response["workflow_runs"] if own_nightly(api, run) and run.get("head_sha") == sha]
    latest = newest_run(runs)
    if latest is None:
        raise ValueError(f"no nightly run for {sha}; run nightly on this commit before tagging a release")
    if latest.get("status") != "completed" or latest.get("conclusion") != "success":
        raise ValueError(f"latest nightly for {sha} is {latest.get('status')}/{latest.get('conclusion')}: {api.run_url(latest)}")
    return latest


def tracking_issue(api: GitHub) -> dict | None:
    for page in range(1, 101):
        query = urlencode({"state": "all", "creator": "github-actions[bot]", "per_page": 100, "page": page})
        issues = api.request("GET", "issues?" + query)
        for issue in issues:
            if not issue.get("pull_request") and issue.get("title") == ISSUE_TITLE and ISSUE_MARKER in (issue.get("body") or ""):
                return issue
        if len(issues) < 100:
            return None
    raise RuntimeError("nightly issue search exceeded its pagination limit")


def failed_jobs(api: GitHub, run_id: int) -> list[dict]:
    result = []
    for page in range(1, 101):
        query = urlencode({"filter": "latest", "per_page": 100, "page": page})
        jobs = api.request("GET", f"actions/runs/{run_id}/jobs?{query}")["jobs"]
        result.extend(job for job in jobs if job.get("conclusion") in {"failure", "timed_out", "action_required"})
        if len(jobs) < 100:
            return result
    raise RuntimeError("nightly jobs exceeded the pagination limit")


def reconcile_nightly_issue(api: GitHub, run_id: int) -> str:
    run = api.request("GET", f"actions/runs/{run_id}")
    repo = api.request("GET", "")
    branch = repo["default_branch"]
    if not own_nightly(api, run) or run.get("head_branch") != branch or run.get("status") != "completed":
        return "ignored non-default, foreign, or unfinished run"
    query = urlencode({"branch": branch, "per_page": 100})
    runs = api.request("GET", f"actions/workflows/{WORKFLOW}/runs?{query}")["workflow_runs"]
    latest = newest_run([candidate for candidate in runs if own_nightly(api, candidate)])
    if latest is None or latest["id"] != run_id or latest.get("run_attempt", 1) != run.get("run_attempt", 1):
        return "ignored superseded run"
    if latest.get("conclusion") != run.get("conclusion") or latest.get("status") != "completed":
        return "ignored run changed during reconciliation"
    conclusion = run.get("conclusion")
    if conclusion not in {"success", "failure", "timed_out", "action_required", "startup_failure"}:
        return "ignored cancelled or non-actionable run"
    issue = tracking_issue(api)
    if conclusion == "success" and (issue is None or issue.get("state") == "closed"):
        return "nightly passed; no open issue"
    status = "已恢复" if conclusion == "success" else "失败"
    body = f"{ISSUE_MARKER}\n\nNightly **{status}**。\n\n- 提交：`{run['head_sha']}`\n- 运行：[#{run['run_number']}]({api.run_url(run)})（第 {run.get('run_attempt', 1)} 次尝试）\n"
    if conclusion != "success":
        for job in failed_jobs(api, run_id):
            name = str(job["name"]).replace("`", "'").replace("@", "＠").replace("\n", " ")
            body += f"- `{name}`：{job['conclusion']}\n"
        body += "\n修复后对同一提交重新运行 nightly；只有最新运行成功才允许发布该提交。\n"
    if issue is None:
        api.request("POST", "issues", {"title": ISSUE_TITLE, "body": body})
        return "created nightly issue"
    update = {"body": body, "state": "closed" if conclusion == "success" else "open"}
    if conclusion == "success":
        update["state_reason"] = "completed"
    api.request("PATCH", f"issues/{int(issue['number'])}", update)
    return "updated nightly issue"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=["check", "report"])
    parser.add_argument("--repository", default=os.environ.get("GITHUB_REPOSITORY", ""))
    parser.add_argument("--sha")
    parser.add_argument("--run-id", type=int)
    args = parser.parse_args()
    try:
        api = GitHub(args.repository, os.environ.get("GITHUB_TOKEN", ""))
        if args.command == "check":
            run = require_successful_nightly(api, args.sha or "")
            print("nightly gate passed: " + api.run_url(run))
        else:
            if not args.run_id or not api.token:
                raise ValueError("report requires a run ID and GITHUB_TOKEN with issues:write")
            print(reconcile_nightly_issue(api, args.run_id))
    except (HTTPError, URLError, OSError, ValueError, KeyError, TypeError, RuntimeError) as exc:
        print(f"nightly {args.command} failed: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
