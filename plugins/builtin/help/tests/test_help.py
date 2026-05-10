import importlib.util
import pathlib
import sys
import unittest


PLUGIN_ROOT = pathlib.Path(__file__).resolve().parents[1]
BUILTIN_ROOT = PLUGIN_ROOT.parent
sys.path.insert(0, str(BUILTIN_ROOT))
MODULE_PATH = PLUGIN_ROOT / "main.py"
spec = importlib.util.spec_from_file_location("help_plugin_main", MODULE_PATH)
help_plugin = importlib.util.module_from_spec(spec)
spec.loader.exec_module(help_plugin)

from testkit import FakePluginContext


class HelpPluginVisibilityTests(unittest.TestCase):
    def make_plugin(self):
        plugin = help_plugin.HelpPlugin()
        plugin.plugin_list_calls = []

        def fake_plugin_list(request_id, visibility=None):
            plugin.plugin_list_calls.append((request_id, visibility))
            return {
                "items": [
                    {
                        "id": "raylea.public",
                        "name": "Public",
                        "description": "公开插件",
                        "registration_state": "installed",
                        "desired_state": "enabled",
                        "commands": [
                            {
                                "name": "public",
                                "description": "公开指令",
                                "usage": "/public",
                                "permission": "everyone",
                            }
                        ],
                    },
                    {
                        "id": "raylea.hidden",
                        "name": "Hidden",
                        "description": "不可见插件",
                        "registration_state": "installed",
                        "desired_state": "enabled",
                        "commands": [],
                    },
                    {
                        "id": "raylea.guided",
                        "name": "Guided",
                        "description": "个性化帮助插件",
                        "registration_state": "installed",
                        "desired_state": "enabled",
                        "commands": [],
                        "help": {
                            "title": "Guided",
                            "summary": "个性化帮助插件",
                            "groups": [
                                {
                                    "title": "功能说明",
                                    "items": [
                                        {
                                            "title": "独立说明",
                                            "description": "不绑定指令的帮助条目",
                                            "usage": "/guided",
                                        }
                                    ],
                                }
                            ],
                        },
                    },
                ],
            }

        plugin.plugin_list = fake_plugin_list
        return plugin

    def test_visible_plugins_requests_caller_visibility(self):
        plugin = self.make_plugin()

        items = plugin.visible_plugins("req_help_test")

        self.assertEqual(plugin.plugin_list_calls, [("req_help_test", "caller")])
        self.assertEqual([item["id"] for item in items], ["raylea.public", "raylea.guided"])

    def test_root_menu_uses_only_visible_commands(self):
        plugin = self.make_plugin()
        ctx = FakePluginContext(request_id="req_help_test", render_result={})

        plugin.handle_help(ctx)

        self.assertEqual(plugin.plugin_list_calls, [("req_help_test", "caller")])
        self.assertEqual(len(ctx.text_messages), 1)
        self.assertIn("Public - 公开插件", ctx.text_messages[0])
        self.assertIn("Guided - 个性化帮助插件", ctx.text_messages[0])
        self.assertNotIn("Hidden", ctx.text_messages[0])

    def test_plugin_detail_uses_only_filtered_catalog(self):
        plugin = self.make_plugin()
        ctx = FakePluginContext(args=["Public"], request_id="req_help_test", render_result={})

        plugin.handle_help(ctx)

        self.assertEqual(len(ctx.text_messages), 1)
        self.assertIn("公开指令", ctx.text_messages[0])
        self.assertNotIn("不可见插件", ctx.text_messages[0])

    def test_command_lookup_uses_only_filtered_catalog(self):
        plugin = self.make_plugin()
        ctx = FakePluginContext(args=["hidden"], request_id="req_help_test", render_result={})

        plugin.handle_help(ctx)

        self.assertEqual(len(ctx.text_messages), 1)
        self.assertIn("没有找到", ctx.text_messages[0])
        self.assertNotIn("不可见插件", ctx.text_messages[0])

    def test_plugin_detail_prefers_manifest_help_groups(self):
        plugin = self.make_plugin()
        ctx = FakePluginContext(args=["Guided"], request_id="req_help_test", render_result={})

        plugin.handle_help(ctx)

        self.assertEqual(len(ctx.text_messages), 1)
        self.assertIn("功能说明", ctx.text_messages[0])
        self.assertIn("独立说明", ctx.text_messages[0])

    def test_manifest_help_render_data_omits_empty_permission(self):
        plugin = self.make_plugin()
        ctx = FakePluginContext(args=["Guided"], request_id="req_help_test", render_result={"image_path": "help.png"})

        plugin.handle_help(ctx)

        self.assertEqual(ctx.text_messages, [])
        self.assertEqual(ctx.messages, [{
            "segments": [{"type": "image", "data": {"file": "help.png"}}],
            "target_type": None,
            "target_id": None,
        }])
        help_item = ctx.render_calls[0]["data"]["groups"][0]["items"][0]
        self.assertNotIn("permission", help_item)
        self.assertEqual(help_item["description"], "不绑定指令的帮助条目")

    def test_plugin_detail_fallback_text_is_compact(self):
        plugin = self.make_plugin()
        ctx = FakePluginContext(args=["Guided"], request_id="req_help_test", render_result={})

        plugin.handle_help(ctx)

        self.assertEqual(len(ctx.text_messages), 1)
        self.assertIn("功能说明", ctx.text_messages[0])
        self.assertIn("- 独立说明：/guided", ctx.text_messages[0])
        self.assertLess(len(ctx.text_messages[0]), 160)


if __name__ == "__main__":
    unittest.main()
