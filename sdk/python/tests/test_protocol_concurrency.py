import importlib
import os
import sys
import threading
import time
import unittest

sys.path.insert(0, os.path.dirname(os.path.dirname(__file__)))

from rayleabot import protocol as protocol_module


class ProtocolConcurrencyTests(unittest.TestCase):
    def setUp(self):
        self.protocol = importlib.reload(protocol_module)
        self.protocol._ensure_reader = lambda: None

    def test_request_local_action_routes_interleaved_responses(self):
        protocol = self.protocol
        sent_actions = []
        sent_lock = threading.Lock()

        def fake_send_action(plugin_id, request_id, action, data, parent_request_id=None):
            with sent_lock:
                sent_actions.append({
                    "plugin_id": plugin_id,
                    "request_id": request_id,
                    "parent_request_id": parent_request_id,
                    "action": action,
                    "data": data,
                })

        protocol.send_action = fake_send_action

        results = {}
        failures = []

        def call_local_action(name, parent_request_id):
            try:
                results[name] = protocol.request_local_action(
                    "helper-plugin",
                    parent_request_id,
                    "http.request",
                    {"url": f"https://example.com/{name}"},
                    timeout_seconds=1,
                )
            except Exception as exc:  # pragma: no cover - exercised only on failure
                failures.append(exc)

        first = threading.Thread(target=call_local_action, args=("first", "evt-first"))
        second = threading.Thread(target=call_local_action, args=("second", "evt-second"))
        first.start()
        second.start()

        deadline = time.time() + 1
        while time.time() < deadline:
            with sent_lock:
                if len(sent_actions) == 2:
                    break
            time.sleep(0.01)
        else:
            self.fail(f"expected two local actions, got {sent_actions!r}")

        with sent_lock:
            request_ids = {item["parent_request_id"]: item["request_id"] for item in sent_actions}

        self.assertEqual(set(request_ids.keys()), {"evt-first", "evt-second"})

        protocol._dispatch_frame({
            "protocol_version": "1",
            "type": "result",
            "timestamp": int(time.time()),
            "plugin_id": "helper-plugin",
            "request_id": request_ids["evt-second"],
            "status": "success",
            "data": {"session": "second"},
        })
        protocol._dispatch_frame({
            "protocol_version": "1",
            "type": "result",
            "timestamp": int(time.time()),
            "plugin_id": "helper-plugin",
            "request_id": request_ids["evt-first"],
            "status": "success",
            "data": {"session": "first"},
        })

        first.join(timeout=1)
        second.join(timeout=1)

        self.assertFalse(first.is_alive())
        self.assertFalse(second.is_alive())
        self.assertEqual(failures, [])
        self.assertEqual(results["first"]["session"], "first")
        self.assertEqual(results["second"]["session"], "second")


if __name__ == "__main__":
    unittest.main()
