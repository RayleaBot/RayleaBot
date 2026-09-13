import io
import os
from pathlib import Path
import subprocess
import sys
from tempfile import TemporaryDirectory
import unittest
from unittest import mock
import urllib.error

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "scripts" / "release"))

import rehearse_current_recovery as recovery


class RunningServerTests(unittest.TestCase):
    def process(self):
        process = mock.Mock(spec=subprocess.Popen)
        process.returncode = None
        process.poll.side_effect = lambda: process.returncode

        def wait(*, timeout):
            if process.returncode is None:
                process.returncode = 0
            return process.returncode

        def kill():
            process.returncode = -9

        process.wait.side_effect = wait
        process.kill.side_effect = kill
        return process

    def test_normal_shutdown_is_clean(self):
        process = self.process()
        with TemporaryDirectory() as tmp, \
                mock.patch.object(recovery.subprocess, "Popen", return_value=process), \
                mock.patch.object(recovery, "request", return_value={}) as request:
            with recovery.running_server(Path("server"), Path(tmp), 1234) as origin:
                self.assertEqual(origin, "http://127.0.0.1:1234")
        self.assertEqual(process.returncode, 0)
        process.kill.assert_not_called()
        request.assert_any_call(origin, "/api/launcher/shutdown", data={}, control=True)

    def test_http_shutdown_failure_reaps_a_real_owned_process(self):
        real_popen = subprocess.Popen
        children = []

        def launch(*args, **kwargs):
            child = real_popen(
                [sys.executable, "-c", "import time; time.sleep(60)"],
                stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
                creationflags=subprocess.CREATE_NO_WINDOW if os.name == "nt" else 0,
            )
            children.append(child)
            return child

        health = mock.MagicMock()
        health.__enter__.return_value.read.return_value = b"{}"
        http_error = urllib.error.HTTPError(
            "http://127.0.0.1/api/launcher/shutdown", 503, "Unavailable", {},
            io.BytesIO(b'{"error":{"code":"system.shutdown_failed"}}'),
        )
        try:
            with TemporaryDirectory() as tmp, \
                    mock.patch.object(recovery.subprocess, "Popen", side_effect=launch), \
                    mock.patch.object(recovery.urllib.request, "urlopen", side_effect=[health, http_error]):
                with self.assertRaises(ExceptionGroup) as caught:
                    with recovery.running_server(Path("server"), Path(tmp), 1234):
                        self.assertIsNone(children[0].poll())
            self.assertIsNotNone(children[0].poll(), "the owned child must be reaped")
            failures = caught.exception.exceptions
            self.assertEqual(len(failures), 2)
            self.assertIn("HTTP 503 (system.shutdown_failed)", str(failures[0]))
            self.assertIs(failures[0].__cause__, http_error)
            self.assertIn("Server did not exit cleanly", str(failures[1]))
        finally:
            http_error.close()
            for child in children:
                if child.poll() is None:
                    child.kill()
                child.wait(timeout=5)

    def test_shutdown_timeout_is_preserved_after_forced_reap(self):
        process = self.process()
        timeout = subprocess.TimeoutExpired("server", 15)
        process.wait.side_effect = [timeout, -9]
        with TemporaryDirectory() as tmp, \
                mock.patch.object(recovery.subprocess, "Popen", return_value=process), \
                mock.patch.object(recovery, "request", return_value={}):
            with self.assertRaises(ExceptionGroup) as caught:
                with recovery.running_server(Path("server"), Path(tmp), 1234):
                    pass
        self.assertIn(timeout, caught.exception.exceptions)
        self.assertEqual(process.returncode, -9)
        process.wait.assert_has_calls([mock.call(timeout=15), mock.call(timeout=5)])
        self.assertTrue(any("did not exit cleanly" in str(exc) for exc in caught.exception.exceptions))

    def test_already_exited_process_skips_shutdown_and_reports_nonzero_status(self):
        for status in (0, 7):
            with self.subTest(status=status):
                process = self.process()
                with TemporaryDirectory() as tmp, \
                        mock.patch.object(recovery.subprocess, "Popen", return_value=process), \
                        mock.patch.object(recovery, "request", return_value={}) as request:
                    def exercise():
                        with recovery.running_server(Path("server"), Path(tmp), 1234):
                            process.returncode = status
                    if status:
                        with self.assertRaisesRegex(RuntimeError, "did not exit cleanly: 7"):
                            exercise()
                    else:
                        exercise()
                process.kill.assert_not_called()
                self.assertEqual(request.call_args_list, [mock.call("http://127.0.0.1:1234", "/healthz")])

    def test_rehearsal_failure_survives_clean_shutdown(self):
        process = self.process()
        original = ValueError("recovery verification failed")
        with TemporaryDirectory() as tmp, \
                mock.patch.object(recovery.subprocess, "Popen", return_value=process), \
                mock.patch.object(recovery, "request", return_value={}):
            with self.assertRaises(ValueError) as caught:
                with recovery.running_server(Path("server"), Path(tmp), 1234):
                    raise original
        self.assertIs(caught.exception, original)
        self.assertEqual(process.returncode, 0)

    def test_rehearsal_shutdown_and_reap_failures_are_all_visible(self):
        process = self.process()
        original = KeyboardInterrupt("cancelled")
        shutdown = RuntimeError("shutdown unavailable")
        kill = OSError("kill denied")
        wait = subprocess.TimeoutExpired("server", 5)
        process.kill.side_effect = kill
        process.wait.side_effect = wait
        with TemporaryDirectory() as tmp, \
                mock.patch.object(recovery.subprocess, "Popen", return_value=process), \
                mock.patch.object(recovery, "request", side_effect=[{}, shutdown]):
            with self.assertRaises(BaseExceptionGroup) as caught:
                with recovery.running_server(Path("server"), Path(tmp), 1234):
                    raise original
        self.assertEqual(caught.exception.exceptions, (original, shutdown, kill, wait))
        process.wait.assert_called_once_with(timeout=5)
        self.assertIsNone(process.returncode)

    def test_startup_http_failure_still_shuts_down_owned_process(self):
        process = self.process()
        startup = RuntimeError("/healthz returned HTTP 503")
        with TemporaryDirectory() as tmp, \
                mock.patch.object(recovery.subprocess, "Popen", return_value=process), \
                mock.patch.object(recovery, "request", side_effect=[startup, {}]):
            with self.assertRaises(RuntimeError) as caught:
                with recovery.running_server(Path("server"), Path(tmp), 1234):
                    self.fail("unhealthy server must not reach rehearsal")
        self.assertIs(caught.exception, startup)
        self.assertEqual(process.returncode, 0)


if __name__ == "__main__":
    unittest.main()
