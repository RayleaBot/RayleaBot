import importlib
import threading
import time
import unittest

import rayleabot.plugin as plugin_module
import rayleabot.protocol as protocol_module


class PluginHelperTests(unittest.TestCase):
    def setUp(self):
        self.protocol = importlib.reload(protocol_module)
        self.protocol._ensure_reader = lambda: None
        self.plugin_module = importlib.reload(plugin_module)
        self.plugin_module.protocol = self.protocol

    def _invoke_local_action(self, invoker):
        plugin = self.plugin_module.RayleaBotPlugin()
        plugin._plugin_id = "helper-plugin"

        sent = {}

        def fake_send_action(plugin_id, request_id, action, data, parent_request_id=None):
            sent.update({
                "plugin_id": plugin_id,
                "request_id": request_id,
                "action": action,
                "data": data,
                "parent_request_id": parent_request_id,
            })

        self.protocol.send_action = fake_send_action

        failures = []

        def call():
            try:
                invoker(plugin)
            except Exception as exc:  # pragma: no cover - exercised only on failure
                failures.append(exc)

        worker = threading.Thread(target=call)
        worker.start()

        deadline = time.time() + 1
        while time.time() < deadline:
            if sent:
                break
            time.sleep(0.01)
        else:
            self.fail("expected helper to send one local action")

        self.protocol._dispatch_frame({
            "protocol_version": "1",
            "type": "result",
            "timestamp": int(time.time()),
            "plugin_id": "helper-plugin",
            "request_id": sent["request_id"],
            "status": "success",
            "data": {"ok": True},
        })

        worker.join(timeout=1)
        self.assertFalse(worker.is_alive())
        self.assertEqual([], failures)
        return sent

    def test_message_forward_get_uses_frozen_action_name(self):
        sent = self._invoke_local_action(
            lambda plugin: plugin.message_forward_get("evt-1", forward_id="forward-001", timeout_seconds=1),
        )

        self.assertEqual("message.forward.get", sent["action"])
        self.assertEqual({"forward_id": "forward-001"}, sent["data"])
        self.assertEqual("evt-1", sent["parent_request_id"])

    def test_file_group_fs_delete_requires_folder_or_file_id(self):
        plugin = self.plugin_module.RayleaBotPlugin()
        with self.assertRaises(ValueError):
            plugin.file_group_fs_delete("evt-2", "group-10001")

        sent = self._invoke_local_action(
            lambda live_plugin: live_plugin.file_group_fs_delete(
                "evt-2",
                "group-10001",
                file_id="file-001",
                timeout_seconds=1,
            ),
        )

        self.assertEqual("file.group.fs.delete", sent["action"])
        self.assertEqual(
            {
                "group_id": "group-10001",
                "file_id": "file-001",
            },
            sent["data"],
        )

    def test_provider_helper_uses_frozen_provider_action_name(self):
        sent = self._invoke_local_action(
            lambda plugin: plugin.napcat_group_sign_set("evt-3", "group-10001", timeout_seconds=1),
        )

        self.assertEqual("provider.napcat.group.sign.set", sent["action"])
        self.assertEqual({"group_id": "group-10001"}, sent["data"])

    def test_governance_helpers_use_frozen_action_names(self):
        sent = self._invoke_local_action(
            lambda plugin: plugin.governance_blacklist_read("evt-4", timeout_seconds=1),
        )
        self.assertEqual("governance.blacklist.read", sent["action"])
        self.assertEqual({}, sent["data"])

        sent = self._invoke_local_action(
            lambda plugin: plugin.governance_blacklist_write(
                "evt-5",
                "upsert",
                entry_type="user",
                target_id="10001",
                reason="manual_review",
                timeout_seconds=1,
            ),
        )
        self.assertEqual("governance.blacklist.write", sent["action"])
        self.assertEqual(
            {
                "operation": "upsert",
                "entry_type": "user",
                "target_id": "10001",
                "reason": "manual_review",
            },
            sent["data"],
        )

        sent = self._invoke_local_action(
            lambda plugin: plugin.governance_whitelist_write(
                "evt-6",
                "set_enabled",
                enabled=True,
                timeout_seconds=1,
            ),
        )
        self.assertEqual("governance.whitelist.write", sent["action"])
        self.assertEqual({"operation": "set_enabled", "enabled": True}, sent["data"])

        sent = self._invoke_local_action(
            lambda plugin: plugin.governance_command_policy_read("evt-7", timeout_seconds=1),
        )
        self.assertEqual("governance.command_policy.read", sent["action"])
        self.assertEqual({}, sent["data"])

    def test_governance_helpers_validate_required_fields(self):
        plugin = self.plugin_module.RayleaBotPlugin()

        with self.assertRaises(ValueError):
            plugin.governance_blacklist_write("evt-8", "upsert", entry_type="user", reason="missing-target")

        with self.assertRaises(ValueError):
            plugin.governance_whitelist_write("evt-9", "set_enabled")

    def test_runtime_context_properties_return_copies(self):
        plugin = self.plugin_module.RayleaBotPlugin()
        plugin._bot_id = "bot-10001"
        plugin._capabilities = ["message.history.get", "provider.napcat.group.sign.set"]
        plugin._command_prefixes = ["!", "/"]

        capabilities = plugin.capabilities
        capabilities.append("tampered")

        prefixes = plugin.command_prefixes
        prefixes.append("#")

        self.assertEqual("bot-10001", plugin.bot_id)
        self.assertEqual(
            ["message.history.get", "provider.napcat.group.sign.set"],
            plugin.capabilities,
        )
        self.assertEqual(["!", "/"], plugin.command_prefixes)
        self.assertEqual("!", plugin.primary_command_prefix)


if __name__ == "__main__":
    unittest.main()
