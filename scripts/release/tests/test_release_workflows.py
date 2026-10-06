"""Release callers share native build and trust gates without sharing publish rights."""
import os
import shutil
from pathlib import Path
import sys
import unittest

import yaml

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "scripts"))
from process_output import run_utf8
sys.path.insert(0, str(ROOT / "scripts/release"))
from artifact_matrix import ARTIFACT_MATRIX


def workflow(name):
    return yaml.load((ROOT / ".github/workflows" / name).read_text(encoding="utf-8"), Loader=yaml.BaseLoader)


def step_named(job, name):
    return next(step for step in job["steps"] if step.get("name") == name)


class ReleaseWorkflowTests(unittest.TestCase):
    def test_release_calls_the_build_without_granting_publish_rights_to_it(self):
        release, build = (workflow(name) for name in ("release.yml", "release-build.yml"))
        self.assertEqual(release["jobs"]["build"]["uses"], "./.github/workflows/release-build.yml")
        self.assertEqual(release["permissions"], {"contents": "read"})
        self.assertEqual(set(build["on"]), {"workflow_call"})
        self.assertEqual(build["permissions"], {"contents": "read"})
        for job in build["jobs"].values():
            self.assertNotIn("write", job.get("permissions", {}).values())
            for step in job.get("steps", []):
                self.assertFalse(step.get("uses", "").startswith("softprops/action-gh-release"))
        publisher = release["jobs"]["publish"]
        self.assertEqual(publisher["permissions"], {"contents": "write", "actions": "read"})
        self.assertEqual(set(publisher["needs"]), {"version", "build"})
        self.assertIn("github.event_name == 'push'", publisher["if"])
        self.assertIn("'refs/tags/v'", publisher["if"])
        self.assertEqual(release["on"], {"push": {"tags": ["v*"]}})

    def test_nightly_gates_check_the_checked_out_commit_before_build_and_publish(self):
        jobs = workflow("release.yml")["jobs"]
        self.assertEqual(jobs["build"]["needs"], "version")
        for job_name, step_name in (("version", "Require successful nightly for the release commit"),
                                    ("publish", "Recheck nightly before publication")):
            job = jobs[job_name]
            gate = step_named(job, step_name)
            self.assertIn('nightly_status.py check --sha "$(git rev-parse HEAD)"', gate["run"])
            self.assertEqual(job["permissions"]["actions"], "read")
            if job_name == "publish":
                self.assertLess(job["steps"].index(gate), job["steps"].index(step_named(job, "Publish GitHub release")))

    def test_channel_and_prerelease_outputs_reach_metadata_and_publication(self):
        release, build = workflow("release.yml"), workflow("release-build.yml")
        self.assertEqual(release["jobs"]["build"]["with"]["channel"], "${{ needs.version.outputs.channel }}")
        publisher = step_named(release["jobs"]["publish"], "Publish GitHub release")["with"]
        self.assertEqual(publisher["prerelease"], "${{ needs.version.outputs.prerelease == 'true' }}")
        self.assertEqual(publisher["make_latest"], "${{ needs.version.outputs.make_latest }}")
        self.assertEqual(build["on"]["workflow_call"]["inputs"]["channel"]["required"], "true")
        self.assertEqual(build["env"]["RELEASE_CHANNEL"], "${{ inputs.channel }}")
        self.assertIn('--channel "$RELEASE_CHANNEL"', step_named(build["jobs"]["assemble"], "Generate release metadata")["run"])

    def test_nightly_reporting_is_separate_and_only_checks_out_the_trusted_branch(self):
        reporter = workflow("nightly-status.yml")
        self.assertEqual(reporter["on"], {"workflow_run": {"workflows": ["nightly"], "types": ["completed"]}})
        self.assertEqual(reporter["permissions"], {"contents": "read", "actions": "read", "issues": "write"})
        self.assertEqual(reporter["concurrency"]["cancel-in-progress"], "false")
        checkout = step_named(reporter["jobs"]["reconcile"], "Checkout trusted default branch")
        self.assertEqual(checkout["with"]["ref"], "${{ github.event.repository.default_branch }}")

    def test_server_artifacts_disable_cgo_without_disabling_desktop_builds(self):
        build = workflow("release-build.yml")
        self.assertNotIn("CGO_ENABLED", build.get("env", {}))
        for name in ("build-full", "build-linux-server"):
            self.assertEqual(step_named(build["jobs"][name], "Build server")["env"]["CGO_ENABLED"], "0")

    def test_every_formal_artifact_keeps_native_runner_and_full_validation_budgets(self):
        jobs = workflow("release-build.yml")["jobs"]
        matrix = jobs["build-full"]["strategy"]["matrix"]["include"]
        expected = {"windows-x64-full": "windows-latest", "linux-x64-full": "ubuntu-latest", "macos-arm64-full": "macos-26"}
        self.assertEqual({item["artifact_id"]: item["runner"] for item in matrix}, expected)
        self.assertEqual(set(expected) | {"linux-x64-server"}, set(ARTIFACT_MATRIX))
        self.assertEqual(jobs["build-linux-server"]["runs-on"], "ubuntu-latest")
        for job_name, step_name in (("build-full", "Package full artifact"), ("build-linux-server", "Package linux server artifact")):
            command = step_named(jobs[job_name], step_name)["run"]
            for option in ("--run-smoke", "--evidence-dir"):
                self.assertIn(option, command)
            evidence = step_named(jobs[job_name], "Upload validation evidence")
            self.assertEqual(evidence["if"], "always()")
            self.assertTrue(evidence["with"]["name"].startswith("validation-evidence-"))
        for job_name, step_name in (("build-full", "Build launcher"), ("build-full", "Build web"), ("build-linux-server", "Build web")):
            self.assertEqual(step_named(jobs[job_name], step_name)["env"]["RAYLEA_BUILD_VERSION"], "${{ inputs.version }}")

    def test_native_packaging_shell_preserves_arguments(self):
        candidates = ([str(Path(os.environ.get("ProgramFiles", "C:/Program Files")) / "Git/bin/bash.exe")]
                      if os.name == "nt" else ["/bin/bash"])
        candidates.append(shutil.which("bash") or "")
        bash = next((candidate for candidate in candidates if candidate and Path(candidate).is_file()), None)
        if bash is None:
            self.skipTest("bash is unavailable")
        job = workflow("release-build.yml")["jobs"]["build-full"]
        for platform in job["strategy"]["matrix"]["include"]:
            with self.subTest(artifact=platform["artifact_id"]):
                command = step_named(job, "Package full artifact")["run"]
                for key, value in platform.items():
                    command = command.replace("${{ matrix." + key + " }}", value)
                command = command.replace("python scripts/release/package_artifact.py", "capture")
                environment = {**os.environ, "RELEASE_VERSION": "0.4.0", "RELEASE_NOTES_REF": "https://example.invalid/notes"}
                completed = run_utf8([bash, "--noprofile", "--norc", "-c",
                                            'capture() { printf "%s\\n" "$@"; };\n' + command],
                                           cwd=ROOT, env=environment, capture_output=True, timeout=20)
                self.assertEqual(completed.returncode, 0, completed.stderr)
                args = completed.stdout.splitlines()
                self.assertEqual(args.count("--artifact-id"), 1, args)
                self.assertEqual(args[args.index("--artifact-id") + 1], platform["artifact_id"])
                self.assertNotIn("", args)

    def test_metadata_only_consumes_release_packages(self):
        jobs = workflow("release-build.yml")["jobs"]
        assemble = jobs["assemble"]
        self.assertEqual(set(assemble["needs"]), {"build-full", "build-linux-server"})
        download = step_named(assemble, "Download release artifacts")
        self.assertEqual(download["with"]["pattern"], "package-*")
        for artifact_id in ARTIFACT_MATRIX:
            self.assertIn("dist/downloads/package-" + artifact_id + "/", step_named(assemble, "Generate release metadata")["run"])
        self.assertIn('--download-base-url "https://github.com/${GITHUB_REPOSITORY}/releases/download/v${VERSION}"', step_named(assemble, "Generate release metadata")["run"])
        uploaded = step_named(assemble, "Upload release metadata")["with"]["path"]
        self.assertEqual(uploaded.strip(), "dist/release/release_manifest.v2.json")
        self.assertNotIn("secrets", workflow("release-build.yml")["on"]["workflow_call"])


if __name__ == "__main__":
    unittest.main()
