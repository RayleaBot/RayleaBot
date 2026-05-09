import importlib.util
import pathlib
import unittest


PLUGIN_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = PLUGIN_ROOT / "main.py"
spec = importlib.util.spec_from_file_location("help_plugin_main", MODULE_PATH)
help_plugin = importlib.util.module_from_spec(spec)
spec.loader.exec_module(help_plugin)


class FakeHelpContext:
    def __init__(self, args=None):
        self.args = args or []
        self.request_id = "req_help_test"
        self.text_messages = []
        self.render_calls = []

    def render_image(self, template, data, theme, output, fallback_text):
        self.render_calls.append({
            "template": template,
            "data": data,
            "theme": theme,
            "output": output,
            "fallback_text": fallback_text,
        })
        return {}

    def send_text(self, text):
        self.text_messages.append(text)


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
                ],
            }

        plugin.plugin_list = fake_plugin_list
        return plugin

    def test_visible_plugins_requests_caller_visibility(self):
        plugin = self.make_plugin()

        items = plugin.visible_plugins("req_help_test")

        self.assertEqual(plugin.plugin_list_calls, [("req_help_test", "caller")])
        self.assertEqual([item["id"] for item in items], ["raylea.public"])

    def test_root_menu_uses_only_visible_commands(self):
        plugin = self.make_plugin()
        ctx = FakeHelpContext()

        plugin.handle_help(ctx)

        self.assertEqual(plugin.plugin_list_calls, [("req_help_test", "caller")])
        self.assertEqual(len(ctx.text_messages), 1)
        self.assertIn("Public - 公开插件", ctx.text_messages[0])
        self.assertNotIn("Hidden", ctx.text_messages[0])

    def test_plugin_detail_uses_only_filtered_catalog(self):
        plugin = self.make_plugin()
        ctx = FakeHelpContext(["Public"])

        plugin.handle_help(ctx)

        self.assertEqual(len(ctx.text_messages), 1)
        self.assertIn("公开指令", ctx.text_messages[0])
        self.assertNotIn("不可见插件", ctx.text_messages[0])

    def test_command_lookup_uses_only_filtered_catalog(self):
        plugin = self.make_plugin()
        ctx = FakeHelpContext(["hidden"])

        plugin.handle_help(ctx)

        self.assertEqual(len(ctx.text_messages), 1)
        self.assertIn("没有找到", ctx.text_messages[0])
        self.assertNotIn("不可见插件", ctx.text_messages[0])


if __name__ == "__main__":
    unittest.main()
