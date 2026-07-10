import json
import tempfile
import unittest
from pathlib import Path

import sys

PLUGIN_DIR = Path(__file__).resolve().parents[1]
TEMPLATE_DIR = PLUGIN_DIR / "templates" / "character-list"
sys.path.insert(0, str(PLUGIN_DIR))

from raylea_game_guide import (  # noqa: E402
    GameGuideService,
    build_character_list_render_data,
    parse_character_list_request,
)

from test_game_guide import FakeContext  # noqa: E402


def make_fake_context(plain_text="*角色列表", command_prefixes=None, render_result=None, render_error=None):
    ctx = FakeContext(plain_text=plain_text)
    ctx.command_prefixes = list(command_prefixes or ["*"])
    ctx.render_calls = []

    def render_image(template, data, theme=None, output=None, fallback_text=None, timeout_seconds=30):
        ctx.render_calls.append({
            "template": template,
            "data": data,
            "theme": theme,
            "output": output,
            "fallback_text": fallback_text,
        })
        if render_error is not None:
            raise render_error
        return render_result or {}
    ctx.render_image = render_image
    return ctx


SAMPLE_CHARACTERS = [
    {"name": "昔涟", "slug": "xilian", "aliases": ["昔莲", "xilian", "cyrene"]},
    {"name": "Archer", "slug": "archer", "aliases": ["阿彻"]},
    {"name": "白厄", "slug": "白厄", "aliases": ["phainon"]},
    {"name": "丹恒", "slug": "丹恒", "aliases": ["danheng"]},
    {"name": "卡芙卡", "slug": "kafka", "aliases": ["卡夫卡", "kafka"]},
    {"name": "三月七•存护", "slug": "三月七存护", "aliases": ["三月七", "小三月"]},
]


class ParseCharacterListRequestTest(unittest.TestCase):
    def test_triggers_with_star_prefix(self):
        self.assertTrue(parse_character_list_request("*角色列表"))

    def test_triggers_with_fullwidth_star_prefix(self):
        self.assertTrue(parse_character_list_request("＊角色列表"))

    def test_triggers_with_custom_prefixes(self):
        self.assertTrue(parse_character_list_request("!角色列表", trigger_prefixes=["!"]))
        self.assertFalse(parse_character_list_request("*角色列表", trigger_prefixes=["!"]))

    def test_does_not_trigger_character_guide(self):
        self.assertFalse(parse_character_list_request("*昔涟攻略"))

    def test_does_not_trigger_partial_match(self):
        self.assertFalse(parse_character_list_request("*角色"))
        self.assertFalse(parse_character_list_request("*角色列表攻略"))
        self.assertFalse(parse_character_list_request("*角色列表查询"))

    def test_does_not_trigger_without_prefix(self):
        self.assertFalse(parse_character_list_request("角色列表"))

    def test_does_not_trigger_empty_text(self):
        self.assertFalse(parse_character_list_request(""))
        self.assertFalse(parse_character_list_request("*"))


class BuildCharacterListRenderDataTest(unittest.TestCase):
    def test_uses_character_directory_copy_by_default(self):
        result = build_character_list_render_data(SAMPLE_CHARACTERS)

        self.assertEqual(result["title"], "星穹铁道角色目录")
        self.assertEqual(result["subtitle"], "已适配 6 名角色 · 名称与别名均可用于查询")

    def test_groups_characters_by_pinyin_initial(self):
        result = build_character_list_render_data(SAMPLE_CHARACTERS)

        self.assertEqual(result["total"], 6)
        self.assertIn("title", result)
        self.assertIn("subtitle", result)

        labels = [group["label"] for group in result["groups"]]
        self.assertIn("A", labels)
        self.assertIn("B", labels)
        self.assertIn("D", labels)
        self.assertIn("K", labels)
        self.assertIn("S", labels)
        self.assertIn("X", labels)

    def test_groups_are_sorted_by_label(self):
        result = build_character_list_render_data(SAMPLE_CHARACTERS)
        labels = [group["label"] for group in result["groups"]]
        self.assertEqual(labels, sorted(labels))

    def test_characters_within_group_are_sorted_by_name(self):
        multi_chars = [
            {"name": "丹恒•饮月", "slug": "丹恒饮月", "aliases": []},
            {"name": "丹恒", "slug": "丹恒", "aliases": []},
            {"name": "大黑塔", "slug": "the-herta", "aliases": []},
        ]
        result = build_character_list_render_data(multi_chars)
        d_group = next(g for g in result["groups"] if g["label"] == "D")
        names = [c["name"] for c in d_group["characters"]]
        self.assertEqual(names, ["丹恒", "丹恒•饮月", "大黑塔"])

    def test_aliases_are_preserved(self):
        result = build_character_list_render_data(SAMPLE_CHARACTERS)
        a_group = next(g for g in result["groups"] if g["label"] == "A")
        archer = next(c for c in a_group["characters"] if c["name"] == "Archer")
        self.assertEqual(archer["aliases"], ["阿彻"])

    def test_empty_aliases_are_filtered(self):
        chars = [{"name": "测试", "slug": "test", "aliases": ["", "   "]}]
        result = build_character_list_render_data(chars)
        c = result["groups"][0]["characters"][0]
        self.assertEqual(c["aliases"], [])

    def test_digits_and_symbols_group_into_hash(self):
        chars = [
            {"name": "123Bot", "slug": "123bot", "aliases": []},
            {"name": "•特殊", "slug": "special", "aliases": []},
        ]
        result = build_character_list_render_data(chars)
        labels = [group["label"] for group in result["groups"]]
        self.assertIn("#", labels)

    def test_custom_title_and_subtitle(self):
        result = build_character_list_render_data(
            SAMPLE_CHARACTERS,
            title="自定义标题",
            subtitle="自定义副标题",
        )
        self.assertEqual(result["title"], "自定义标题")
        self.assertEqual(result["subtitle"], "自定义副标题")

    def test_empty_list_returns_valid_structure(self):
        result = build_character_list_render_data([])
        self.assertEqual(result["total"], 0)
        self.assertEqual(result["groups"], [])
        self.assertEqual(result["subtitle"], "已适配 0 名角色 · 名称与别名均可用于查询")

        schema = json.loads((TEMPLATE_DIR / "input.schema.json").read_text(encoding="utf-8"))
        self.assertNotIn("minItems", schema["properties"]["groups"])

    def test_filters_out_characters_without_name(self):
        chars = [
            {"name": "有效角色", "slug": "valid", "aliases": []},
            {"slug": "no-name", "aliases": []},
        ]
        result = build_character_list_render_data(chars)
        self.assertEqual(result["total"], 1)
        self.assertEqual(result["groups"][0]["characters"][0]["name"], "有效角色")


class CharacterListCommandTest(unittest.TestCase):
    def test_render_and_send_successfully(self):
        with tempfile.TemporaryDirectory() as cache_dir:
            service = GameGuideService(plugin_dir=PLUGIN_DIR, cache_root=cache_dir)
            ctx = make_fake_context(
                plain_text="*角色列表",
                render_result={"image_path": "file:///tmp/character-list.png"},
            )
            service.handle_message(ctx)

            self.assertEqual(len(ctx.render_calls), 1)
            self.assertEqual(ctx.render_calls[0]["template"], "character-list")
            self.assertEqual(ctx.render_calls[0]["output"], "png")
            self.assertIn("title", ctx.render_calls[0]["data"])
            self.assertIn("groups", ctx.render_calls[0]["data"])
            self.assertTrue(ctx.results[-1]["handled"])
            self.assertEqual(len(ctx.messages), 1)
            self.assertEqual(ctx.messages[0]["segments"][0]["type"], "image")

    def test_does_not_interfere_with_guide_query(self):
        with tempfile.TemporaryDirectory() as cache_dir:
            service = GameGuideService(plugin_dir=PLUGIN_DIR, cache_root=cache_dir)
            ctx = make_fake_context(
                plain_text="*昔涟攻略",
                render_result={"image_path": "file:///tmp/should-not-render.png"},
            )
            service.handle_message(ctx)
            # "*昔涟攻略" should go through the guide query path, not character list.
            # The guide query will fail to find images (no network), but it should
            # not trigger the character list render_image path.
            self.assertEqual(len(ctx.render_calls), 0)
            self.assertTrue(
                any("没有找到" in t or "攻略图" in t for t in ctx.texts),
                "expected guide-not-found message, got texts: %s" % ctx.texts,
            )

    def test_fallback_message_on_render_failure(self):
        with tempfile.TemporaryDirectory() as cache_dir:
            service = GameGuideService(plugin_dir=PLUGIN_DIR, cache_root=cache_dir)
            ctx = make_fake_context(
                plain_text="*角色列表",
                render_error=RuntimeError("render failed"),
            )
            service.handle_message(ctx)

            self.assertTrue(ctx.results[-1]["handled"])
            self.assertTrue(any("角色列表图片生成失败" in t for t in ctx.texts))

    def test_fallback_message_on_missing_image_path(self):
        with tempfile.TemporaryDirectory() as cache_dir:
            service = GameGuideService(plugin_dir=PLUGIN_DIR, cache_root=cache_dir)
            ctx = make_fake_context(
                plain_text="*角色列表",
                render_result={"image_path": ""},
            )
            service.handle_message(ctx)

            self.assertTrue(ctx.results[-1]["handled"])
            self.assertTrue(any("角色列表图片生成失败" in t for t in ctx.texts))

    def test_result_includes_total(self):
        with tempfile.TemporaryDirectory() as cache_dir:
            service = GameGuideService(plugin_dir=PLUGIN_DIR, cache_root=cache_dir)
            ctx = make_fake_context(
                plain_text="*角色列表",
                render_result={"image_path": "file:///tmp/character-list.png"},
            )
            service.handle_message(ctx)
            self.assertIn("total", ctx.results[-1])


if __name__ == "__main__":
    unittest.main()
