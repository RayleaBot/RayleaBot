from __future__ import annotations

import copy
import importlib.util
from pathlib import Path
import unittest
from urllib.parse import parse_qs, urlsplit

SPEC = importlib.util.spec_from_file_location("nightly_status", Path(__file__).resolve().parents[1] / "ci/nightly_status.py")
nightly = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(nightly)
SHA = "a" * 40


def run(number=1, **changes):
    return {"id": number, "run_number": number, "run_attempt": 1, "head_sha": SHA,
            "head_branch": "main", "event": "workflow_dispatch", "status": "completed",
            "conclusion": "success", "head_repository": {"full_name": "RayleaBot/RayleaBot"}, **changes}


class FakeGitHub(nightly.GitHub):
    def __init__(self, runs, issue=None):
        super().__init__("RayleaBot/RayleaBot", "fixture-token")
        self.runs = runs
        self.issue = issue
        self.writes = []
        self.pages = {}

    def request(self, method, path, body=None):
        if method != "GET":
            self.writes.append((method, path, body))
            return {"number": 7}
        parts = urlsplit(path)
        query = parse_qs(parts.query)
        if path == "":
            return {"default_branch": "main"}
        if parts.path.endswith("nightly.yml/runs"):
            return {"workflow_runs": copy.deepcopy(self.runs)}
        if parts.path == "issues":
            return [self.issue] if self.issue else []
        if parts.path.endswith("/jobs"):
            page = int(query["page"][0])
            return {"jobs": self.pages.get(page, [{"name": "server", "conclusion": "failure"}])}
        if parts.path.startswith("actions/runs/"):
            identifier = int(parts.path.split("/")[-1])
            return copy.deepcopy(next(item for item in self.runs if item["id"] == identifier))
        raise AssertionError(path)


class NightlyGateTests(unittest.TestCase):
    def test_requires_the_release_commit_and_trusted_nightly_event(self):
        self.assertEqual(nightly.require_successful_nightly(FakeGitHub([run()]), SHA)["id"], 1)
        for item in (run(head_sha="b" * 40), run(event="pull_request"), run(head_repository={"full_name": "fork/RayleaBot"})):
            with self.subTest(item=item), self.assertRaises(ValueError):
                nightly.require_successful_nightly(FakeGitHub([item]), SHA)
        with self.assertRaises(ValueError):
            nightly.require_successful_nightly(FakeGitHub([run()]), SHA[:7])

    def test_latest_failure_or_incomplete_attempt_overrides_old_success(self):
        for status, conclusion in (("completed", "failure"), ("completed", "cancelled"), ("in_progress", None), ("queued", None)):
            with self.subTest(status=status, conclusion=conclusion), self.assertRaises(ValueError):
                nightly.require_successful_nightly(FakeGitHub([run(2, status=status, conclusion=conclusion), run(1)]), SHA)
        with self.assertRaises(ValueError):
            nightly.require_successful_nightly(FakeGitHub([run(1), run(1, run_attempt=2, conclusion="failure")]), SHA)
        self.assertEqual(nightly.require_successful_nightly(FakeGitHub([run(2), run(1, conclusion="failure")]), SHA)["id"], 2)

    def test_missing_evidence_or_api_failure_cannot_approve_publication(self):
        with self.assertRaises(ValueError):
            nightly.require_successful_nightly(FakeGitHub([]), SHA)
        api = FakeGitHub([])
        def unavailable(*_):
            raise RuntimeError("API unavailable")
        api.request = unavailable
        with self.assertRaises(RuntimeError):
            nightly.require_successful_nightly(api, SHA)


class NightlyIssueTests(unittest.TestCase):
    def issue(self, state="open"):
        return {"number": 7, "title": nightly.ISSUE_TITLE, "body": nightly.ISSUE_MARKER, "state": state}

    def test_failure_creates_one_issue_and_later_failures_update_or_reopen_it(self):
        for state in (None, "open", "closed"):
            with self.subTest(state=state):
                api = FakeGitHub([run(conclusion="failure")], self.issue(state) if state else None)
                nightly.reconcile_nightly_issue(api, 1)
                self.assertEqual(len(api.writes), 1)
                method, endpoint, payload = api.writes[0]
                self.assertEqual((method, endpoint), ("PATCH", "issues/7") if state else ("POST", "issues"))
                self.assertIn(nightly.ISSUE_MARKER, payload["body"])
                self.assertIn(SHA, payload["body"])
                if state:
                    self.assertEqual(payload["state"], "open")

    def test_success_closes_existing_issue_without_creating_success_notifications(self):
        api = FakeGitHub([run()], self.issue())
        nightly.reconcile_nightly_issue(api, 1)
        self.assertEqual(api.writes[0][2]["state"], "closed")
        for issue in (None, self.issue("closed")):
            api = FakeGitHub([run()], issue)
            nightly.reconcile_nightly_issue(api, 1)
            self.assertEqual(api.writes, [])

    def test_stale_branch_fork_and_cancelled_runs_do_not_change_the_issue(self):
        cases = ([run(1, conclusion="failure"), run(2)], [run(head_branch="feature", conclusion="failure")],
                 [run(head_repository={"full_name": "fork/repo"}, conclusion="failure")], [run(conclusion="cancelled")])
        for runs in cases:
            api = FakeGitHub(runs, self.issue())
            nightly.reconcile_nightly_issue(api, 1)
            self.assertEqual(api.writes, [])

    def test_failed_job_listing_reads_later_pages(self):
        api = FakeGitHub([run(conclusion="failure")])
        api.pages = {1: [{"name": "ok", "conclusion": "success"}] * 100,
                     2: [{"name": "windows", "conclusion": "failure"}]}
        nightly.reconcile_nightly_issue(api, 1)
        self.assertIn("windows", api.writes[0][2]["body"])


if __name__ == "__main__":
    unittest.main()
