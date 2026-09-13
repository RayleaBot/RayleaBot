"""Release callers share native build and trust gates without sharing publish rights."""
import importlib.util
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
    def test_validation_and_release_call_the_same_build_without_publishing_validation(self):
        validation, release, build = (workflow(name) for name in ("artifact-validation.yml", "release.yml", "release-build.yml"))
        self.assertEqual(validation["on"]["push"]["branches"], ["codex/validation-*"])
        self.assertEqual(validation["on"]["workflow_dispatch"]["inputs"]["version"]["default"], "0.5.0")
        for value in validation["jobs"]["validate"]["with"].values():
            self.assertIn("inputs.version || '0.5.0'", value)
        self.assertEqual(set(validation["on"]), {"push", "workflow_dispatch"})
        for caller, job_id in ((validation, "validate"), (release, "build")):
            self.assertEqual(caller["jobs"][job_id]["uses"], "./.github/workflows/release-build.yml")
            self.assertEqual(caller["permissions"], {"contents": "read"})
        self.assertIn("github.sha", validation["jobs"]["validate"]["with"]["release_notes_ref"])
        self.assertEqual(set(build["on"]), {"workflow_call"})
        self.assertEqual(build["permissions"], {"contents": "read"})
        for data in (validation, build):
            for job in data["jobs"].values():
                self.assertNotIn("write", job.get("permissions", {}).values())
                for step in job.get("steps", []):
                    self.assertFalse(step.get("uses", "").startswith("softprops/action-gh-release"))
        publisher = release["jobs"]["publish"]
        self.assertEqual(publisher["permissions"], {"contents": "write"})
        self.assertEqual(publisher["needs"], "build")
        self.assertIn("github.event_name == 'push'", publisher["if"])
        self.assertIn("'refs/tags/v'", publisher["if"])
        self.assertEqual(release["on"], {"push": {"tags": ["v*"]}})

    def test_every_formal_artifact_keeps_native_runner_and_full_validation_budgets(self):
        jobs = workflow("release-build.yml")["jobs"]
        matrix = jobs["build-full"]["strategy"]["matrix"]["include"]
        expected = {"windows-x64-full": "windows-latest", "linux-x64-full": "ubuntu-latest", "macos-arm64-full": "macos-26"}
        self.assertEqual({item["artifact_id"]: item["runner"] for item in matrix}, expected)
        self.assertEqual(set(expected) | {"linux-x64-server"}, set(ARTIFACT_MATRIX))
        self.assertEqual(jobs["build-linux-server"]["runs-on"], "ubuntu-latest")
        for job_name, step_name in (("build-full", "Package full artifact"), ("build-linux-server", "Package linux server artifact")):
            command = step_named(jobs[job_name], step_name)["run"]
            for option in ("--run-smoke", "--run-recovery-drill", "--recovery-plugin-fixture", "--run-self-host-smoke", "--evidence-dir", "--observation-window-seconds 300", "--window-seconds 600", "--probe-interval-seconds 30"):
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
                self.assertNotIn("--updater-bin", args)
                self.assertNotIn("--windows-signer-sha256", args)
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

    def test_new_workflows_select_release_and_ci_checks(self):
        spec = importlib.util.spec_from_file_location("validation_detect_changes", ROOT / "scripts/ci/detect_changes.py")
        detector = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(detector)
        for path in (".github/workflows/release-build.yml", ".github/workflows/artifact-validation.yml"):
            with self.subTest(path=path):
                areas = detector.classify([path])
                self.assertTrue(areas["release"])
                self.assertTrue(areas["ci"])
                self.assertFalse(areas["docs_only"])


if __name__ == "__main__":
    unittest.main()
