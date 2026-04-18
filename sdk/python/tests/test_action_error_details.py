import importlib
import threading
import time
import unittest

from rayleabot import protocol as protocol_module


class ActionErrorDetailsTests(unittest.TestCase):
    def setUp(self):
        self.protocol = importlib.reload(protocol_module)
        self.protocol._ensure_reader = lambda: None

    def test_request_local_action_preserves_error_details(self):
        protocol = self.protocol
        sent_requests = []

        def fake_send_action(plugin_id, request_id, action, data, parent_request_id=None):
            sent_requests.append(request_id)

        protocol.send_action = fake_send_action

        caught = {}

        def request():
            try:
                protocol.request_local_action(
                    "helper-plugin",
                    "evt-1",
                    "http.request",
                    {"url": "https://example.com/data"},
                    timeout_seconds=1,
                )
            except Exception as exc:  # pragma: no cover - exercised only on failure
                caught["error"] = exc

        worker = threading.Thread(target=request)
        worker.start()

        deadline = time.time() + 1
        while time.time() < deadline:
            if sent_requests:
                break
            time.sleep(0.01)
        else:
            self.fail(f"expected local action request, got {sent_requests!r}")

        protocol._dispatch_frame({
            "protocol_version": "1",
            "type": "error",
            "timestamp": int(time.time()),
            "plugin_id": "helper-plugin",
            "request_id": sent_requests[0],
            "code": "platform.rate_limited",
            "message": "outbound request rejected by policy",
            "details": {"retry_after_seconds": 30, "policy": "http.egress"},
        })

        worker.join(timeout=1)
        self.assertFalse(worker.is_alive())

        error = caught["error"]
        self.assertIsInstance(error, protocol.ActionError)
        self.assertEqual("platform.rate_limited", error.code)
        self.assertEqual(30, error.details["retry_after_seconds"])

    def test_request_local_action_defaults_missing_error_details(self):
        protocol = self.protocol
        sent_requests = []

        def fake_send_action(plugin_id, request_id, action, data, parent_request_id=None):
            sent_requests.append(request_id)

        protocol.send_action = fake_send_action

        caught = {}

        def request():
            try:
                protocol.request_local_action(
                    "helper-plugin",
                    "evt-2",
                    "logger.write",
                    {"level": "warn", "message": "attempt denied"},
                    timeout_seconds=1,
                )
            except Exception as exc:  # pragma: no cover - exercised only on failure
                caught["error"] = exc

        worker = threading.Thread(target=request)
        worker.start()

        deadline = time.time() + 1
        while time.time() < deadline:
            if sent_requests:
                break
            time.sleep(0.01)
        else:
            self.fail(f"expected local action request, got {sent_requests!r}")

        protocol._dispatch_frame({
            "protocol_version": "1",
            "type": "error",
            "timestamp": int(time.time()),
            "plugin_id": "helper-plugin",
            "request_id": sent_requests[0],
            "code": "permission.scope_violation",
            "message": "capability not granted",
        })

        worker.join(timeout=1)
        self.assertFalse(worker.is_alive())

        error = caught["error"]
        self.assertIsInstance(error, protocol.ActionError)
        self.assertEqual({}, error.details)


if __name__ == "__main__":
    unittest.main()
