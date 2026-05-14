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

    def test_init_without_bot_keeps_empty_bot_id(self):
        sent = {}

        def fake_send_init_ack(plugin_id, request_id, subscriptions=None):
            sent.update({
                "plugin_id": plugin_id,
                "request_id": request_id,
                "subscriptions": subscriptions,
            })

        frames = iter([
            {
                "protocol_version": "1",
                "type": "init",
                "timestamp": int(time.time()),
                "plugin_id": "helper-plugin",
                "request_id": "init-1",
                "command_prefixes": ["/"],
            },
            {
                "protocol_version": "1",
                "type": "shutdown",
                "timestamp": int(time.time()),
                "plugin_id": "helper-plugin",
                "request_id": "shutdown-1",
                "reason": "stop",
            },
        ])

        self.protocol.read_frame = lambda: next(frames, None)
        self.protocol.send_init_ack = fake_send_init_ack

        plugin = self.plugin_module.RayleaBotPlugin()
        plugin.run()

        self.assertEqual("helper-plugin", sent["plugin_id"])
        self.assertEqual("", plugin.bot_id)

    def test_bot_identity_changed_updates_bot_id_before_handler(self):
        plugin = self.plugin_module.RayleaBotPlugin()
        seen = {}

        @plugin.on_event("bot.identity.changed")
        def handle_identity(event, request_id):
            seen["request_id"] = request_id
            seen["bot_id"] = plugin.bot_id
            seen["target"] = event["target"]["id"]

        plugin._handle_event(
            {
                "event": {
                    "event_id": "identity-1",
                    "source_protocol": "onebot11",
                    "source_adapter": "adapter.onebot11",
                    "event_type": "bot.identity.changed",
                    "timestamp": int(time.time()),
                    "target": {
                        "type": "bot",
                        "id": "10001",
                    },
                    "payload": {
                        "onebot": {
                            "self_id": "10001",
                        },
                    },
                }
            },
            "helper-plugin",
            "evt-identity-1",
        )

        self.assertEqual({
            "request_id": "evt-identity-1",
            "bot_id": "10001",
            "target": "10001",
        }, seen)

    def test_class_command_handler_receives_event_context(self):
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

        class ContextPlugin(self.plugin_module.RayleaBotPlugin):
            def __init__(self):
                super().__init__()
                self.subscribe("message.group")

            @self.plugin_module.command("hello", aliases=["hi"])
            def handle_hello(self, ctx):
                self.seen = {
                    "request_id": ctx.request_id,
                    "args": ctx.args,
                    "target_id": ctx.target_id,
                    "bot_id": ctx.bot_id,
                    "prefix": ctx.primary_command_prefix,
                }
                ctx.send_text("ok")

        plugin = ContextPlugin()
        plugin._plugin_id = "context-plugin"
        plugin._bot_id = "bot-10001"
        plugin._command_prefixes = ["!"]

        plugin._handle_event(
            {
                "event": {
                    "event_id": "evt-ctx",
                    "source_protocol": "onebot11",
                    "source_adapter": "adapter.onebot11",
                    "event_type": "message.group",
                    "timestamp": int(time.time()),
                    "target": {
                        "type": "group",
                        "id": "20001",
                    },
                    "payload": {
                        "command": "hi",
                        "args": ["world"],
                    },
                }
            },
            "context-plugin",
            "evt-ctx-request",
        )

        self.assertEqual({
            "request_id": "evt-ctx-request",
            "args": ["world"],
            "target_id": "20001",
            "bot_id": "bot-10001",
            "prefix": "!",
        }, plugin.seen)
        self.assertEqual("context-plugin", sent["plugin_id"])
        self.assertEqual("evt-ctx-request", sent["request_id"])
        self.assertEqual("message.send", sent["action"])
        self.assertEqual({
            "target_type": "group",
            "target_id": "20001",
            "message": {
                "segments": [{
                    "type": "text",
                    "data": {"text": "ok"},
                }],
            },
        }, sent["data"])

    def test_decorated_handler_registration_skips_unrelated_properties(self):
        class ContextPlugin(self.plugin_module.RayleaBotPlugin):
            def __init__(self):
                super().__init__()
                self.ready = True

            @property
            def late_value(self):
                if not getattr(self, "ready", False):
                    raise RuntimeError("late value accessed too early")
                return "ready"

            @self.plugin_module.command("hello")
            def handle_hello(self, ctx):
                ctx.send_result({"handled": True})

        plugin = ContextPlugin()

        self.assertEqual("ready", plugin.late_value)
        self.assertIn("hello", plugin._command_handlers)

    def test_instance_decorator_accepts_context_handler(self):
        plugin = self.plugin_module.RayleaBotPlugin()
        seen = {}

        @plugin.on_event("message.private")
        def handle_private(ctx):
            seen["request_id"] = ctx.request_id
            seen["plain_text"] = ctx.plain_text

        plugin._handle_event(
            {
                "event": {
                    "event_id": "evt-private",
                    "source_protocol": "onebot11",
                    "source_adapter": "adapter.onebot11",
                    "event_type": "message.private",
                    "timestamp": int(time.time()),
                    "message": {
                        "plain_text": "hello",
                    },
                }
            },
            "context-plugin",
            "evt-private-request",
        )

        self.assertEqual({
            "request_id": "evt-private-request",
            "plain_text": "hello",
        }, seen)

    def test_await_bot_identity_returns_immediately_when_identity_known(self):
        plugin = self.plugin_module.RayleaBotPlugin()
        plugin._set_bot_id("bot-10001")

        self.assertEqual("bot-10001", plugin.await_bot_identity(timeout_seconds=0.05))

    def test_await_bot_identity_blocks_until_identity_set(self):
        plugin = self.plugin_module.RayleaBotPlugin()
        plugin._set_bot_id("")

        def deliver_identity():
            time.sleep(0.05)
            plugin._set_bot_id("bot-10002")

        worker = threading.Thread(target=deliver_identity)
        worker.start()
        try:
            result = plugin.await_bot_identity(timeout_seconds=1.0)
        finally:
            worker.join()

        self.assertEqual("bot-10002", result)

    def test_await_bot_identity_returns_empty_on_timeout(self):
        plugin = self.plugin_module.RayleaBotPlugin()
        plugin._set_bot_id("")

        self.assertEqual("", plugin.await_bot_identity(timeout_seconds=0.05))

    def test_bot_identity_changed_with_no_target_clears_bot_id(self):
        plugin = self.plugin_module.RayleaBotPlugin()
        plugin._set_bot_id("bot-10001")

        plugin._update_bot_identity({
            "event_type": "bot.identity.changed",
            "target": {},
            "payload": {},
        })

        self.assertEqual("", plugin.bot_id)
        self.assertEqual("", plugin.await_bot_identity(timeout_seconds=0.01))


if __name__ == "__main__":
    unittest.main()
