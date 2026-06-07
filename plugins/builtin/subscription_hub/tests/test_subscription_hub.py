import os
import sys
import unittest
import json
import time


PLUGIN_DIR = os.path.dirname(os.path.dirname(__file__))
BUILTIN_DIR = os.path.dirname(PLUGIN_DIR)
sys.path.insert(0, BUILTIN_DIR)
sys.path.insert(0, PLUGIN_DIR)

from bilibili import dynamic_updates, normalize_user_info, normalize_user_search, parse_preview_url
from main import (
    SUBSCRIBE_BILIBILI_USAGE,
    SubscriptionHubPlugin,
    UNSUBSCRIBE_BILIBILI_USAGE,
    add_bilibili_subscription,
    build_status_text,
    format_subscription_list,
    parse_bilibili_command_args,
    remove_bilibili_subscription,
)
from rendering import build_render_data
from settings import merge_settings
from testkit import FakePluginContext as FakeContext
from rayleabot.protocol import ActionError


class SubscriptionHubTests(unittest.TestCase):
    def subscription_settings(self, **overrides):
        settings = {
            "enabled": True,
            "poll_cron": "*/5 * * * *",
            "poll_timeout_seconds": 12,
            "dynamic_time_range_seconds": 7200,
            "tokens": [],
            "subscriptions": [{
                "id": "bilibili-123456-group-10000",
                "platform": "bilibili",
                "uid": "123456",
                "name": "测试 UP",
                "target_type": "group",
                "target_id": "10000",
                "services": ["video"],
                "subscribers": [{"id": "42", "nickname": "订阅人"}],
                "enabled": True,
            }],
        }
        settings.update(overrides)
        return settings

    def cookie_settings(self):
        return merge_settings({}, {
            "tokens": [{
                "id": "primary",
                "label": "主 Cookie",
                "platform": "bilibili",
                "secret_key": "bili.primary",
                "enabled": True,
            }],
        })

    def dynamic_response(self, items, code=0, message=""):
        return {
            "status_code": 200,
            "body_text": json.dumps({
                "code": code,
                "message": message,
                "data": {"items": items},
            }),
        }

    def nav_response(self, is_login=True, code=0, message=""):
        return {
            "status_code": 200,
            "body_text": json.dumps({
                "code": code,
                "message": message,
                "data": {"isLogin": is_login},
            }),
        }

    def live_status_response(self, live_status=1, live_time=1700000000, include_cover=True):
        entry = {
            "room_id": 22913442,
            "uid": 123456,
            "uname": "测试主播",
            "face": "//i0.hdslb.com/live-face.jpg",
            "title": "真实直播标题",
            "live_status": live_status,
            "liveTime": live_time,
            "live_time": live_time,
            "url": "https://live.bilibili.com/22913442",
            "area_v2_name": "虚拟主播",
        }
        if include_cover:
            entry["cover_from_user"] = "//i0.hdslb.com/live-cover.jpg"
            entry["user_cover"] = "//i0.hdslb.com/live-user-cover.jpg"
        return {
            "status_code": 200,
            "body_text": json.dumps({
                "code": 0,
                "message": "0",
                "data": {"123456": entry},
            }),
        }

    def user_info_response(self, uid="123456", name="测试 UP", code=0, message="", face="//i0.hdslb.com/face.jpg"):
        return {
            "status_code": 200,
            "body_text": json.dumps({
                "code": code,
                "message": message,
                "data": {"mid": uid, "name": name, "face": face},
            }),
        }

    def user_search_response(self, results=None, code=0, message=""):
        return {
            "status_code": 200,
            "body_text": json.dumps({
                "code": code,
                "message": message,
                "data": {"result": results if results is not None else [
                    {"mid": "123456", "uname": "测试 UP", "fans": 1000, "upic": "//i0.hdslb.com/search-face.jpg"},
                ]},
            }),
        }

    def video_preview_response(self, code=0, message=""):
        return {
            "status_code": 200,
            "body_text": json.dumps({
                "code": code,
                "message": message,
                "data": {
                    "bvid": "BV1c5qEBjEtJ",
                    "title": "真实视频标题",
                    "desc": "真实视频简介",
                    "duration": 3671,
                    "pic": "//i0.hdslb.com/video-cover.jpg",
                    "pubdate": 1700000000,
                    "owner": {
                        "mid": 123456,
                        "name": "测试 UP",
                        "face": "//i0.hdslb.com/face.jpg",
                    },
                },
            }),
        }

    def opus_preview_response(self, item=None, code=0, message=""):
        return {
            "status_code": 200,
            "body_text": json.dumps({
                "code": code,
                "message": message,
                "data": {"item": item or self.image_text_item("1194416231669563410")},
            }),
        }

    def opus_detail_modules_response(self):
        return {
            "status_code": 200,
            "body_text": json.dumps({
                "code": 0,
                "message": "0",
                "data": {
                    "item": {
                        "id_str": "1194416231669563410",
                        "basic": {
                            "title": "真实动态标题 - 哔哩哔哩",
                            "uid": 4070943,
                        },
                        "modules": [
                            {
                                "module_type": "MODULE_TYPE_TITLE",
                                "module_title": {"text": "真实动态标题"},
                            },
                            {
                                "module_type": "MODULE_TYPE_AUTHOR",
                                "module_author": {
                                    "name": "真实 UP",
                                    "face": "//i0.hdslb.com/opus-face.jpg",
                                    "mid": "4070943",
                                    "pub_ts": 1776935100,
                                    "pub_time": "2026年04月23日 17:05",
                                },
                            },
                            {
                                "module_type": "MODULE_TYPE_CONTENT",
                                "module_content": {
                                    "paragraphs": [
                                        {
                                            "para_type": 1,
                                            "heading": {
                                                "nodes": [
                                                    {"type": "TEXT_NODE_TYPE_WORD", "word": {"words": "正文标题"}},
                                                ],
                                            },
                                        },
                                        {
                                            "para_type": 1,
                                            "text": {
                                                "nodes": [
                                                    {"type": "TEXT_NODE_TYPE_WORD", "word": {"words": "真实正文 "}},
                                                    {"type": "TEXT_NODE_TYPE_RICH", "rich": {"type": "RICH_TEXT_NODE_TYPE_TOPIC", "text": "#测试话题#"}},
                                                    {"type": "TEXT_NODE_TYPE_RICH", "rich": {"text": " #兜底话题#"}},
                                                    {"type": "TEXT_NODE_TYPE_RICH", "rich": {"type": "RICH_TEXT_NODE_TYPE_AT", "text": "@银狼"}},
                                                    {"type": "TEXT_NODE_TYPE_RICH", "rich": {"type": "RICH_TEXT_NODE_TYPE_LOTTERY", "text": "互动抽奖"}},
                                                    {"type": "TEXT_NODE_TYPE_RICH", "rich": {"type": "RICH_TEXT_NODE_TYPE_BV", "text": "BV1c5qEBjEtJ"}},
                                                    {"type": "TEXT_NODE_TYPE_RICH", "rich": {"type": "RICH_TEXT_NODE_TYPE_GOODS", "text": "周边", "icon_name": "taobao"}},
                                                    {"type": "TEXT_NODE_TYPE_RICH", "rich": {"type": "RICH_TEXT_NODE_TYPE_VOTE", "text": "投票"}},
                                                ],
                                            },
                                        },
                                        {
                                            "para_type": 2,
                                            "pic": {
                                                "pics": [
                                                    {
                                                        "url": "//i0.hdslb.com/opus-1.jpg",
                                                        "width": 1000,
                                                        "height": 1200,
                                                    },
                                                ],
                                            },
                                        },
                                    ],
                                },
                            },
                        ],
                    },
                },
            }),
        }

    def live_preview_response(self, code=0, message="", include_identity=False):
        data = {
            "room_id": 22913442,
            "uid": 123456,
            "title": "真实直播标题",
            "live_status": 1,
            "user_cover": "//i0.hdslb.com/live-cover.jpg",
            "live_time": 1700000000,
        }
        if include_identity:
            data["uname"] = "测试主播"
            data["face"] = "//i0.hdslb.com/live-face.jpg"
        return {
            "status_code": 200,
            "body_text": json.dumps({
                "code": code,
                "message": message,
                "data": data,
            }),
        }

    def video_item(self, dynamic_id, title, pub_ts=None):
        pub_ts = int(pub_ts or time.time())
        return {
            "id_str": dynamic_id,
            "type": "DYNAMIC_TYPE_AV",
            "basic": {"jump_url": f"//www.bilibili.com/video/{dynamic_id}"},
            "modules": {
                "module_author": {"name": "测试 UP", "pub_ts": pub_ts, "pub_time": "今天 12:00"},
                "module_dynamic": {
                    "major": {
                        "type": "MAJOR_TYPE_ARCHIVE",
                        "archive": {"title": title, "desc": "视频简介", "cover": "//i0.hdslb.com/video.jpg", "duration_text": "03:21"},
                    },
                },
            },
        }

    def image_text_item(self, dynamic_id, pub_ts=1700000000):
        return {
            "id_str": dynamic_id,
            "type": "DYNAMIC_TYPE_DRAW",
            "basic": {"jump_url": f"//www.bilibili.com/opus/{dynamic_id}"},
            "modules": {
                "module_author": {
                    "name": "测试 UP",
                    "face": "//i0.hdslb.com/face.jpg",
                    "pub_ts": pub_ts,
                    "pub_time": "今天 12:00",
                },
                "module_dynamic": {
                    "desc": {"text": "真实图文动态正文"},
                    "major": {
                        "type": "MAJOR_TYPE_DRAW",
                        "draw": {
                            "items": [
                                {"src": "//i0.hdslb.com/dyn/1.jpg", "width": 800, "height": 800},
                                {"src": "//i0.hdslb.com/dyn/2.jpg", "width": 800, "height": 800},
                            ],
                        },
                    },
                },
            },
        }

    def test_manifest_declares_visible_command(self):
        with open(os.path.join(PLUGIN_DIR, "info.json"), "r", encoding="utf-8") as handle:
            manifest = json.load(handle)
        self.assertEqual([
            "订阅状态",
            "订阅b站推送",
            "取消b站推送",
            "订阅列表",
            "b站订阅列表",
            "全部订阅列表",
            "全部b站订阅列表",
            "立即检查订阅",
            "预览订阅卡片",
        ], [item.get("name") for item in manifest.get("commands") or []])
        self.assertEqual("super_admin", manifest["commands"][1]["permission"])
        self.assertEqual("super_admin", manifest["commands"][2]["permission"])
        self.assertEqual("/订阅b站推送 [直播|视频|图文|文章|转发] UID或昵称", manifest["commands"][1]["usage"])
        self.assertEqual("/取消b站推送 [直播|视频|图文|文章|转发] UID或昵称", manifest["commands"][2]["usage"])
        self.assertIn("类型可选", manifest["commands"][1]["description"])
        self.assertIn("类型可选", manifest["commands"][2]["description"])
        self.assertEqual("super_admin", manifest["commands"][5]["permission"])
        self.assertEqual("super_admin", manifest["commands"][6]["permission"])
        self.assertEqual("super_admin", manifest["commands"][7]["permission"])
        self.assertEqual("super_admin", manifest["commands"][8]["permission"])
        self.assertEqual("/预览订阅卡片 [直播|视频|图文|文章|转发|Bilibili链接]", manifest["commands"][8]["usage"])
        self.assertIn("Bilibili 链接", manifest["commands"][8]["description"])
        self.assertIn("help", manifest)
        self.assertEqual([
            "订阅操作",
            "列表查看",
            "维护与预览",
            "配置说明",
        ], [group.get("title") for group in manifest["help"].get("groups") or []])
        operation_items = manifest["help"]["groups"][0]["items"]
        self.assertEqual("/订阅b站推送 [直播|视频|图文|文章|转发] UID或昵称", operation_items[1]["usage"])
        self.assertEqual("/取消b站推送 [直播|视频|图文|文章|转发] UID或昵称", operation_items[2]["usage"])
        self.assertIn("不填表示全部类型", operation_items[1]["description"])
        self.assertIn("不填表示全部类型", operation_items[2]["description"])
        preview_item = manifest["help"]["groups"][2]["items"][1]
        self.assertEqual("/预览订阅卡片 [直播|视频|图文|文章|转发|Bilibili链接]", preview_item["usage"])
        self.assertIn("视频、图文动态和直播间链接", preview_item["description"])
        self.assertNotIn("dynamic_commands", manifest)

    def test_bilibili_update_template_marks_rich_text_selectors_blue(self):
        style_path = os.path.join(PLUGIN_DIR, "templates", "bilibili-update", "styles.css")
        with open(style_path, "r", encoding="utf-8") as handle:
            styles = handle.read()

        for selector in (
            ".dynamic-text .rich-text-topic",
            ".dynamic-text .rich-text-at",
            ".dynamic-text .rich-text-link",
            ".dynamic-text .rich-text-lottery",
            ".dynamic-text .bili-rich-text-module",
            ".dynamic-text .bili-rich-text-link",
            ".repost-summary .bili-rich-text-module",
            ".repost-summary .bili-rich-text-link",
        ):
            self.assertIn(selector, styles)
        self.assertIn("color: var(--bili-blue);", styles)
        for asset in (
            'url("assets/rich-text-lottery.svg")',
            'url("assets/rich-text-video.svg")',
            'url("assets/rich-text-goods-taobao.svg")',
            'url("assets/rich-text-vote.svg")',
        ):
            self.assertIn(asset, styles)
        self.assertIn("line-height: inherit;", styles)
        self.assertIn("min-height: 1.12em;", styles)
        self.assertIn("padding-left: 1.28em;", styles)
        self.assertIn("vertical-align: baseline;", styles)
        self.assertIn("background-position: left calc(50% + 0.13em);", styles)
        self.assertIn("background-size: 1.12em 1.12em;", styles)
        self.assertNotIn("vertical-align: -0.12em;", styles)
        self.assertNotIn("background-size: 1em 1em;", styles)

    def test_bilibili_update_rich_text_icons_use_fixed_blue(self):
        asset_dir = os.path.join(PLUGIN_DIR, "templates", "bilibili-update", "assets")
        for name in ("rich-text-lottery.svg", "rich-text-goods-taobao.svg", "rich-text-vote.svg"):
            with open(os.path.join(asset_dir, name), "r", encoding="utf-8") as handle:
                svg = handle.read()
            self.assertIn("#00A1D6", svg)
            self.assertNotIn("currentColor", svg)
            self.assertNotIn("darkreader", svg)

    def test_bilibili_update_preview_includes_rich_text_examples(self):
        preview_path = os.path.join(PLUGIN_DIR, "templates", "bilibili-update", "preview.json")
        with open(preview_path, "r", encoding="utf-8") as handle:
            preview = json.load(handle)

        html = preview["content_html"]
        self.assertIn("。 <span", html)
        for token in (
            "bili-rich-text-module topic",
            "bili-rich-text-module at",
            "bili-rich-text-module lottery",
            "bili-rich-text-link video",
            "bili-rich-text-module goods taobao",
            "bili-rich-text-module vote",
        ):
            self.assertIn(token, html)
        self.assertIn("bili-rich-text-module lottery", preview["original"]["summary_html"])
        self.assertIn("bili-rich-text-module goods taobao", preview["original"]["summary_html"])
        for key in ("live_event", "status_label", "live_started_at", "live_detected_at"):
            self.assertIn(key, preview)

    def test_merge_settings_normalizes_tokens_and_subscriptions(self):
        settings = merge_settings({}, {
            "tokens": [{
                "id": "Primary",
                "label": "主 Cookie",
                "platform": "bilibili",
                "secret_key": "bili.primary",
            }],
            "subscriptions": [{
                "uid": "123456",
                "name": "测试 UP",
                "avatar_url": "https://i0.hdslb.com/face.jpg",
                "target_type": "group",
                "target_id": "10000",
                "target_name": "测试群",
                "services": ["video", "live", "invalid"],
                "subscribers": [{
                    "id": "42",
                    "nickname": "订阅人",
                    "group_nickname": "群名片",
                    "role": "admin",
                    "role_label": "管理员",
                    "avatar_url": "https://q1.qlogo.cn/g?b=qq&nk=42&s=100",
                }],
            }],
        })
        self.assertEqual(settings["tokens"][0]["id"], "primary")
        self.assertEqual(settings["tokens"][0]["platform"], "bilibili")
        self.assertEqual(settings["dynamic_time_range_seconds"], 7200)
        self.assertEqual(settings["subscriptions"][0]["services"], ["video", "live"])
        self.assertEqual(settings["subscriptions"][0]["avatar_url"], "https://i0.hdslb.com/face.jpg")
        self.assertEqual(settings["subscriptions"][0]["target_name"], "测试群")
        self.assertEqual(settings["subscriptions"][0]["subscribers"][0]["nickname"], "订阅人")
        self.assertEqual(settings["subscriptions"][0]["subscribers"][0]["group_nickname"], "群名片")
        self.assertEqual(settings["subscriptions"][0]["subscribers"][0]["avatar_url"], "https://q1.qlogo.cn/g?b=qq&nk=42&s=100")

    def test_merge_settings_requires_bilibili_token_platform(self):
        settings = merge_settings({}, {
            "tokens": [
                {"id": "missing-platform", "label": "未标记平台", "secret_key": "bili.missing", "enabled": True},
                {"id": "other", "platform": "qq", "label": "其它平台", "secret_key": "qq.primary", "enabled": True},
                {"id": "primary", "platform": "bilibili", "label": "主 CK", "secret_key": "bili.primary", "enabled": True},
            ],
        })
        self.assertEqual(settings["tokens"], [{
            "id": "primary",
            "platform": "bilibili",
            "label": "主 CK",
            "secret_key": "bili.primary",
            "enabled": True,
        }])

    def test_dynamic_updates_extract_video(self):
        updates = dynamic_updates({
            "data": {
                "items": [{
                    "id_str": "987",
                    "type": "DYNAMIC_TYPE_AV",
                    "basic": {"jump_url": "//www.bilibili.com/video/BV1"},
                    "modules": {
                        "module_author": {"name": "测试 UP", "pub_ts": 1700000000, "pub_time": "今天 12:00"},
                        "module_dynamic": {
                            "major": {
                                "type": "MAJOR_TYPE_ARCHIVE",
                                "archive": {"title": "新视频", "desc": "视频简介", "cover": "//i0.hdslb.com/cover.jpg", "duration_text": "03:21"},
                            },
                        },
                    },
                }],
            },
        })
        self.assertEqual(len(updates), 1)
        self.assertEqual(updates[0]["service"], "video")
        self.assertEqual(updates[0]["title"], "新视频")
        self.assertEqual(updates[0]["duration_text"], "03:21")
        self.assertEqual(updates[0]["pub_ts"], 1700000000)
        self.assertEqual(updates[0]["images"], [{"url": "https://i0.hdslb.com/cover.jpg"}])

    def test_dynamic_updates_extract_repost_before_major_type(self):
        updates = dynamic_updates({
            "data": {
                "items": [{
                    "id_str": "988",
                    "type": "DYNAMIC_TYPE_FORWARD",
                    "basic": {
                        "comment_type": "17",
                        "jump_url": "//t.bilibili.com/988",
                    },
                    "modules": {
                        "module_author": {"name": "测试 UP", "pub_ts": 1700000100, "pub_time": "今天 13:00"},
                        "module_dynamic": {
                            "desc": {"rich_text_nodes": [{"type": "RICH_TEXT_NODE_TYPE_TEXT", "text": "转发推荐"}]},
                            "major": {
                                "type": "MAJOR_TYPE_OPUS",
                                "opus": {"title": "被转发内容"},
                            },
                        },
                    },
                    "orig": self.video_item("777", "原视频", pub_ts=1700000000),
                }],
            },
        })
        self.assertEqual(len(updates), 1)
        self.assertEqual(updates[0]["service"], "repost")
        self.assertEqual(updates[0]["title"], "转发动态")
        self.assertEqual(updates[0]["summary"], "转发推荐")
        self.assertEqual(updates[0]["original"]["title"], "原视频")

    def test_dynamic_updates_clean_rich_text_node_summary(self):
        updates = dynamic_updates({
            "data": {
                "items": [{
                    "id_str": "989",
                    "type": "DYNAMIC_TYPE_WORD",
                    "basic": {"jump_url": "//t.bilibili.com/989"},
                    "modules": {
                        "module_author": {"name": "测试 UP", "pub_ts": 1700000200, "pub_time": "今天 14:00"},
                        "module_dynamic": {
                            "desc": {
                                "rich_text_nodes": [
                                    {"text": "个人置顶简介", "orig_text": "个人置顶简介"},
                                    {"text": "合作联系：hudie007@vip.qq.com"},
                                ],
                            },
                            "major": {},
                        },
                    },
                }],
            },
        })

        self.assertEqual(len(updates), 1)
        self.assertEqual(updates[0]["title"], "测试 UP 发布文字动态")
        self.assertEqual(updates[0]["summary"], "个人置顶简介 合作联系：hudie007@vip.qq.com")
        self.assertNotIn("rich_text_nodes", updates[0]["summary"])
        self.assertNotIn("orig_text", updates[0]["summary"])

    def test_dynamic_updates_generates_summary_html_from_rich_text_nodes(self):
        updates = dynamic_updates({
            "data": {
                "items": [{
                    "id_str": "990",
                    "type": "DYNAMIC_TYPE_WORD",
                    "basic": {"jump_url": "//t.bilibili.com/990"},
                    "modules": {
                        "module_author": {"name": "测试 UP", "pub_ts": 1700000300, "pub_time": "今天 15:00"},
                        "module_dynamic": {
                            "desc": {
                                "rich_text_nodes": [
                                    {"type": "RICH_TEXT_NODE_TYPE_TEXT", "text": "正文开头 ", "orig_text": "正文开头 "},
                                    {"type": "RICH_TEXT_NODE_TYPE_TOPIC", "text": "#崩坏星穹铁道#", "orig_text": "#崩坏星穹铁道#"},
                                    {"type": "RICH_TEXT_NODE_TYPE_TEXT", "text": " 结尾", "orig_text": " 结尾"},
                                    {"type": "RICH_TEXT_NODE_TYPE_AT", "text": "@银狼", "orig_text": "@银狼"},
                                    {"type": "RICH_TEXT_NODE_TYPE_LOTTERY", "text": "互动抽奖", "orig_text": "互动抽奖"},
                                    {"type": "RICH_TEXT_NODE_TYPE_BV", "text": "BV1c5qEBjEtJ", "orig_text": "BV1c5qEBjEtJ"},
                                    {"type": "RICH_TEXT_NODE_TYPE_GOODS", "text": "周边", "orig_text": "周边", "icon_name": "taobao"},
                                    {"type": "RICH_TEXT_NODE_TYPE_VOTE", "text": "投票", "orig_text": "投票"},
                                ],
                            },
                            "major": {},
                        },
                    },
                }],
            },
        })

        self.assertEqual(len(updates), 1)
        html = updates[0]["summary_html"]
        self.assertIn('正文开头 <span class="rich-text-topic bili-rich-text-module topic">#崩坏星穹铁道#</span> 结尾', html)
        self.assertIn('<span class="rich-text-at bili-rich-text-module at">@银狼</span>', html)
        self.assertIn('<span class="rich-text-lottery bili-rich-text-module lottery">互动抽奖</span>', html)
        self.assertIn('<span class="rich-text-link bili-rich-text-link video">BV1c5qEBjEtJ</span>', html)
        self.assertIn('<span class="rich-text-link bili-rich-text-module goods taobao">周边</span>', html)
        self.assertIn('<span class="rich-text-link bili-rich-text-module vote">投票</span>', html)

    def test_dynamic_updates_uses_short_title_for_untitled_word_dynamic(self):
        long_text = "这是没有独立标题的文字动态，正文内容会比较长。" * 8
        updates = dynamic_updates({
            "data": {
                "items": [{
                    "id_str": "990",
                    "type": "DYNAMIC_TYPE_WORD",
                    "basic": {"jump_url": "//t.bilibili.com/990"},
                    "modules": {
                        "module_author": {"name": "测试 UP", "pub_ts": 1700000300, "pub_time": "今天 15:00"},
                        "module_dynamic": {
                            "desc": {"text": long_text},
                            "major": {},
                        },
                    },
                }],
            },
        })

        self.assertEqual(len(updates), 1)
        self.assertEqual(updates[0]["title"], "测试 UP 发布文字动态")
        self.assertEqual(updates[0]["summary"], long_text)
        self.assertNotEqual(updates[0]["title"], long_text[:40])

    def test_dynamic_updates_uses_short_title_for_untitled_image_dynamic(self):
        long_text = "这是一条没有独立标题的图文动态，正文应该进入摘要区域。" * 6
        updates = dynamic_updates({
            "data": {
                "items": [{
                    "id_str": "991",
                    "type": "DYNAMIC_TYPE_DRAW",
                    "basic": {"jump_url": "//t.bilibili.com/991"},
                    "modules": {
                        "module_author": {"name": "测试 UP", "pub_ts": 1700000400, "pub_time": "今天 16:00"},
                        "module_dynamic": {
                            "desc": {"text": long_text},
                            "major": {
                                "type": "MAJOR_TYPE_DRAW",
                                "draw": {
                                    "items": [
                                        {"src": "//i0.hdslb.com/dyn/1.jpg", "width": 800, "height": 800},
                                        {"src": "//i0.hdslb.com/dyn/2.jpg", "width": 800, "height": 800},
                                    ],
                                },
                            },
                        },
                    },
                }],
            },
        })

        self.assertEqual(len(updates), 1)
        self.assertEqual(updates[0]["title"], "测试 UP 发布图文动态")
        self.assertEqual(updates[0]["summary"], long_text)
        self.assertEqual(updates[0]["images"], [
            {"url": "https://i0.hdslb.com/dyn/1.jpg", "width": 800, "height": 800},
            {"url": "https://i0.hdslb.com/dyn/2.jpg", "width": 800, "height": 800},
        ])

    def test_dynamic_updates_keeps_article_title(self):
        updates = dynamic_updates({
            "data": {
                "items": [{
                    "id_str": "992",
                    "type": "DYNAMIC_TYPE_ARTICLE",
                    "basic": {"jump_url": "//www.bilibili.com/read/cv123"},
                    "modules": {
                        "module_author": {"name": "测试 UP", "pub_ts": 1700000500, "pub_time": "今天 17:00"},
                        "module_dynamic": {
                            "major": {
                                "type": "MAJOR_TYPE_ARTICLE",
                                "article": {
                                    "title": "专栏文章标题",
                                    "desc": "文章摘要",
                                    "covers": ["//i0.hdslb.com/article.jpg"],
                                },
                            },
                        },
                    },
                }],
            },
        })

        self.assertEqual(len(updates), 1)
        self.assertEqual(updates[0]["service"], "article")
        self.assertEqual(updates[0]["title"], "专栏文章标题")
        self.assertEqual(updates[0]["summary"], "文章摘要")

    def test_render_data_contains_subscriber_info(self):
        data = build_render_data({
            "uid": "123456",
            "name": "测试 UP",
            "subscribers": [{
                "id": "42",
                "nickname": "普通昵称",
                "group_nickname": "群名片",
                "title": "专属头衔",
                "role": "admin",
                "role_label": "管理员",
                "avatar_url": "https://q1.qlogo.cn/g?b=qq&nk=42&s=100",
            }],
        }, {
            "service": "video",
            "title": "新视频",
            "summary": "视频简介",
            "duration_text": "03:21",
            "images": [{"url": "https://i0.hdslb.com/cover.jpg"}],
            "pub_ts": 1700000000,
            "original": {
                "service": "image_text",
                "title": "原动态",
                "summary": "原动态正文",
                "images": [{"url": "https://i0.hdslb.com/orig.jpg"}],
                "author": {"name": "原作者"},
            },
            "author": {"name": "测试 UP", "uid": "123456"},
        })
        self.assertEqual(data["subscriber_text"], "群名片")
        self.assertEqual(data["subscriber_cards"][0]["display_name"], "群名片")
        self.assertEqual(data["subscriber_cards"][0]["uid_text"], "42")
        self.assertEqual(data["subscriber_cards"][0]["title"], "专属头衔")
        self.assertEqual(data["subscriber_cards"][0]["role_label"], "管理员")
        self.assertEqual(data["service"], "视频")
        self.assertEqual(data["headline"], "新视频")
        self.assertEqual(data["content_text"], "视频简介")
        self.assertEqual(data["source_label"], "Bilibili · 视频")
        self.assertEqual(data["author_uid_text"], "UID 123456")
        self.assertEqual(data["images"], [{"url": "https://i0.hdslb.com/cover.jpg"}])
        self.assertEqual(data["image_count"], 1)
        self.assertEqual(data["media_grid_class"], "media-grid--single")
        self.assertEqual(data["media_items"][0]["class"], "media-item media-item--wide media-item--video")
        self.assertEqual(data["media_items"][0]["duration_text"], "03:21")
        self.assertEqual(data["original"]["title"], "原动态")
        self.assertEqual(data["original"]["images"], [{"url": "https://i0.hdslb.com/orig.jpg"}])

    def test_render_data_passes_summary_html(self):
        data = build_render_data({
            "uid": "123456",
            "name": "测试 UP",
            "subscribers": [],
        }, {
            "service": "image_text",
            "title": "图文动态",
            "summary": "纯文本摘要",
            "summary_html": '<span class="rich-text-topic">#话题#</span> 正文',
            "pub_ts": 1700000000,
        })
        self.assertEqual(data["content_html"], '<span class="rich-text-topic">#话题#</span> 正文')
        self.assertEqual(data["summary_html"], '<span class="rich-text-topic">#话题#</span> 正文')
        self.assertEqual(data["content_text"], "纯文本摘要")
        self.assertIsNone(data.get("original"))

    def test_render_data_passes_original_summary_html(self):
        data = build_render_data({
            "uid": "123456",
            "name": "测试 UP",
            "subscribers": [],
        }, {
            "service": "repost",
            "title": "转发动态",
            "summary": "转发评论",
            "pub_ts": 1700000000,
            "original": {
                "service": "image_text",
                "title": "原动态",
                "summary": "原动态正文",
                "summary_html": '<span class="rich-text-at">@用户</span>',
                "author": {"name": "原作者"},
            },
        })
        original = data.get("original")
        self.assertIsNotNone(original)
        self.assertEqual(original["summary_html"], '<span class="rich-text-at">@用户</span>')

    def test_render_data_limits_summary_html_by_visible_text(self):
        html = (
            '正文 '
            '<span class="rich-text-topic bili-rich-text-module topic">#话题#</span> '
            '<span class="rich-text-lottery bili-rich-text-module lottery">互动抽奖</span> '
            '<span class="rich-text-link bili-rich-text-module goods taobao">周边</span>'
        ) * 32
        data = build_render_data({
            "uid": "123456",
            "name": "测试 UP",
            "subscribers": [],
        }, {
            "service": "image_text",
            "title": "图文动态",
            "summary": "纯文本摘要",
            "summary_html": html,
            "pub_ts": 1700000000,
        })

        self.assertIn('bili-rich-text-module goods taobao">周边</span>', data["content_html"])
        self.assertIn("...", data["content_html"])
        self.assertEqual(data["content_html"].count("<span"), data["content_html"].count("</span>"))

    def test_render_data_marks_long_and_gif_media(self):
        data = build_render_data({
            "uid": "123456",
            "name": "测试 UP",
        }, {
            "service": "image_text",
            "title": "测试 UP 发布图文动态",
            "images": [
                {"url": "https://i0.hdslb.com/dyn/1.jpg", "width": 800, "height": 800},
                {"url": "https://i0.hdslb.com/dyn/2.jpg", "width": 800, "height": 1800},
                {"url": "https://i0.hdslb.com/dyn/3.gif", "width": 800, "height": 800},
            ],
        })

        self.assertEqual(data["image_count"], 3)
        self.assertEqual(data["media_grid_class"], "media-grid--triple")
        self.assertEqual(data["media_items"][1]["label"], "长图")
        self.assertIn("media-item--long", data["media_items"][1]["class"])
        self.assertEqual(data["media_items"][2]["label"], "动图")
        self.assertIn("media-item--gif", data["media_items"][2]["class"])

    def test_parse_bilibili_command_args_defaults_to_all(self):
        self.assertEqual(parse_bilibili_command_args(["123456"]), {"services": ["all"], "uid": "123456", "query": "123456", "error": False})
        self.assertEqual(parse_bilibili_command_args(["图文", "123456"]), {"services": ["image_text"], "uid": "123456", "query": "123456", "error": False})
        self.assertEqual(parse_bilibili_command_args(["崩坏星穹铁道"]), {"services": ["all"], "uid": "", "query": "崩坏星穹铁道", "error": False})
        self.assertEqual(parse_bilibili_command_args(["番剧", "123456"]), {"services": [], "uid": "123456", "query": "123456", "error": True})

    def test_bilibili_subscription_usage_message_for_invalid_type(self):
        settings = merge_settings({}, {})
        add_result = add_bilibili_subscription(settings, FakeContext(args=["番剧", "123456"]))
        remove_result = remove_bilibili_subscription(settings, FakeContext(args=["番剧", "123456"]))

        self.assertFalse(add_result["ok"])
        self.assertEqual(add_result["message"], SUBSCRIBE_BILIBILI_USAGE)
        self.assertFalse(remove_result["ok"])
        self.assertEqual(remove_result["message"], UNSUBSCRIBE_BILIBILI_USAGE)

    def test_normalize_bilibili_user_info_keeps_avatar(self):
        result = normalize_user_info(json.loads(self.user_info_response()["body_text"]))

        self.assertTrue(result["ok"])
        self.assertEqual(result["uid"], "123456")
        self.assertEqual(result["name"], "测试 UP")
        self.assertEqual(result["avatar_url"], "https://i0.hdslb.com/face.jpg")

    def test_normalize_bilibili_user_search_keeps_avatar(self):
        result = normalize_user_search(json.loads(self.user_search_response([
            {"mid": "123456", "uname": "测试 UP", "upic": "//i0.hdslb.com/upic.jpg"},
        ])["body_text"]), "测试 UP")

        self.assertTrue(result["ok"])
        self.assertEqual(result["avatar_url"], "https://i0.hdslb.com/upic.jpg")

    def test_add_bilibili_subscription_binds_current_target_and_subscriber(self):
        settings = merge_settings({}, {})
        result = add_bilibili_subscription(settings, FakeContext(
            args=["图文", "123456"],
            payload={
                "onebot": {
                    "group_name": "测试群",
                    "user_id": "42",
                    "sender": {
                        "user_id": "42",
                        "nickname": "普通昵称",
                        "card": "群名片",
                        "role": "admin",
                        "title": "专属头衔",
                    },
                },
            },
            http_responses=[self.user_info_response()],
        ))
        self.assertTrue(result["ok"])
        self.assertEqual(len(settings["subscriptions"]), 1)
        subscription = settings["subscriptions"][0]
        self.assertEqual(subscription["id"], "bilibili-123456-group-10000")
        self.assertEqual(subscription["name"], "测试 UP")
        self.assertEqual(subscription["avatar_url"], "https://i0.hdslb.com/face.jpg")
        self.assertEqual(subscription["target_name"], "测试群")
        self.assertEqual(subscription["services"], ["image_text"])
        self.assertEqual(subscription["subscribers"], [{
            "id": "42",
            "nickname": "订阅人",
            "group_nickname": "群名片",
            "title": "专属头衔",
            "role": "admin",
            "role_label": "管理员",
            "avatar_url": "https://q1.qlogo.cn/g?b=qq&nk=42&s=100",
        }])
        self.assertIn("测试 UP（UID 123456）", result["message"])

    def test_add_bilibili_subscription_prefers_platform_super_admin_role(self):
        settings = merge_settings({}, {})
        result = add_bilibili_subscription(settings, FakeContext(
            args=["图文", "123456"],
            payload={
                "onebot": {
                    "user_id": "42",
                    "sender": {
                        "user_id": "42",
                        "nickname": "普通昵称",
                        "card": "群名片",
                        "role": "member",
                    },
                },
            },
            super_admins=["42"],
            http_responses=[self.user_info_response()],
        ))

        self.assertTrue(result["ok"])
        subscriber = settings["subscriptions"][0]["subscribers"][0]
        self.assertEqual(subscriber["role"], "super_admin")
        self.assertEqual(subscriber["role_label"], "超级管理员")

    def test_add_bilibili_subscription_falls_back_to_onebot_member_role(self):
        settings = merge_settings({}, {})
        result = add_bilibili_subscription(settings, FakeContext(
            args=["图文", "123456"],
            payload={
                "onebot": {
                    "user_id": "42",
                    "sender": {
                        "user_id": "42",
                        "nickname": "普通昵称",
                        "role": "member",
                    },
                },
            },
            super_admins=["99"],
            http_responses=[self.user_info_response()],
        ))

        self.assertTrue(result["ok"])
        subscriber = settings["subscriptions"][0]["subscribers"][0]
        self.assertEqual(subscriber["role"], "member")
        self.assertEqual(subscriber["role_label"], "群员")

    def test_add_bilibili_subscription_resolves_nickname(self):
        settings = merge_settings({}, {})
        ctx = FakeContext(args=["视频", "崩坏星穹铁道"], http_responses=[self.user_search_response([
            {"mid": "111111", "uname": "崩坏星穹铁道二创", "fans": 10},
            {"mid": "3537126822012013", "uname": "崩坏星穹铁道", "fans": 5000000, "upic": "//i0.hdslb.com/star-rail.jpg"},
        ])])

        result = add_bilibili_subscription(settings, ctx)

        self.assertTrue(result["ok"])
        subscription = settings["subscriptions"][0]
        self.assertEqual(subscription["uid"], "3537126822012013")
        self.assertEqual(subscription["name"], "崩坏星穹铁道")
        self.assertEqual(subscription["avatar_url"], "https://i0.hdslb.com/star-rail.jpg")
        self.assertEqual(subscription["services"], ["video"])
        self.assertIn("崩坏星穹铁道（UID 3537126822012013）", result["message"])
        self.assertIn("search/type", ctx.http_requests[0]["url"])

    def test_add_bilibili_subscription_merges_services_and_subscribers(self):
        settings = merge_settings({}, {
            "subscriptions": [{
                "id": "custom-subscription-id",
                "uid": "123456",
                "target_type": "group",
                "target_id": "10000",
                "services": ["video"],
                "subscribers": [{"id": "42", "nickname": "旧昵称"}],
            }],
        })
        result = add_bilibili_subscription(settings, FakeContext(
            args=["直播", "123456"],
            actor={"id": "43", "nickname": "新订阅人"},
            http_responses=[self.user_info_response()],
        ))
        self.assertTrue(result["ok"])
        self.assertEqual(len(settings["subscriptions"]), 1)
        self.assertEqual(settings["subscriptions"][0]["id"], "custom-subscription-id")
        self.assertEqual(settings["subscriptions"][0]["services"], ["video", "live"])
        self.assertEqual(settings["subscriptions"][0]["name"], "测试 UP")
        subscribers = settings["subscriptions"][0]["subscribers"]
        self.assertEqual(subscribers[0], {"id": "42", "nickname": "旧昵称"})
        self.assertEqual(subscribers[1]["id"], "43")
        self.assertEqual(subscribers[1]["nickname"], "新订阅人")
        self.assertEqual(subscribers[1]["avatar_url"], "https://q1.qlogo.cn/g?b=qq&nk=43&s=100")

    def test_add_bilibili_subscription_rejects_empty_search_result(self):
        settings = merge_settings({}, {})
        result = add_bilibili_subscription(settings, FakeContext(args=["未知昵称"], http_responses=[self.user_search_response([])]))

        self.assertFalse(result["ok"])
        self.assertEqual(settings["subscriptions"], [])
        self.assertIn("没有搜索到 Bilibili 用户", result["message"])

    def test_add_bilibili_subscription_reports_blocked_search(self):
        settings = merge_settings({}, {})
        result = add_bilibili_subscription(settings, FakeContext(args=["昵称"], http_responses=[self.user_search_response(code=-412)]))

        self.assertFalse(result["ok"])
        self.assertEqual(settings["subscriptions"], [])
        self.assertIn("风控拦截", result["message"])

    def test_add_bilibili_subscription_reports_http_permission_error(self):
        settings = merge_settings({}, {})
        result = add_bilibili_subscription(settings, FakeContext(
            args=["崩坏星穹铁道"],
            http_responses=[ActionError("plugin.internal_error", "http.request target is outside the granted scope")],
        ))

        self.assertFalse(result["ok"])
        self.assertEqual(settings["subscriptions"], [])
        self.assertIn("HTTP 请求权限", result["message"])

    def test_remove_bilibili_subscription_removes_service_or_item(self):
        settings = merge_settings({}, {
            "subscriptions": [{
                "uid": "123456",
                "target_type": "group",
                "target_id": "10000",
                "services": ["video", "live"],
            }],
        })
        result = remove_bilibili_subscription(settings, FakeContext(args=["直播", "123456"]))
        self.assertTrue(result["ok"])
        self.assertEqual(settings["subscriptions"][0]["services"], ["video"])

        result = remove_bilibili_subscription(settings, FakeContext(args=["123456"]))
        self.assertTrue(result["ok"])
        self.assertEqual(settings["subscriptions"], [])

    def test_remove_bilibili_subscription_resolves_nickname(self):
        settings = merge_settings({}, {
            "subscriptions": [{
                "id": "custom-subscription-id",
                "uid": "123456",
                "name": "测试 UP",
                "target_type": "group",
                "target_id": "10000",
                "services": ["video"],
            }],
        })

        result = remove_bilibili_subscription(settings, FakeContext(args=["测试 UP"]))

        self.assertTrue(result["ok"])
        self.assertEqual(settings["subscriptions"], [])
        self.assertIn("测试 UP（UID 123456）", result["message"])

    def test_remove_bilibili_subscription_missing_uid_uses_local_subscriptions(self):
        settings = merge_settings({}, {"subscriptions": []})
        ctx = FakeContext(args=["123456"], http_responses=[self.user_info_response(code=-412)])

        result = remove_bilibili_subscription(settings, ctx)

        self.assertFalse(result["ok"])
        self.assertEqual(result["message"], "当前会话没有订阅 Bilibili 123456。")
        self.assertEqual(ctx.http_requests, [])

    def test_format_subscription_list_can_filter_current_target_and_platform(self):
        settings = merge_settings({}, {
            "subscriptions": [
                {"uid": "123456", "target_type": "group", "target_id": "10000", "services": ["video"], "subscribers": ["订阅人"]},
                {"uid": "654321", "target_type": "private", "target_id": "42", "services": ["live"]},
            ],
        })
        text = format_subscription_list(settings, {"target_type": "group", "target_id": "10000"}, platform="bilibili", title="Bilibili 订阅列表")
        self.assertIn("Bilibili 123456", text)
        self.assertIn("订阅人", text)
        self.assertNotIn("654321", text)

    def test_subscribe_command_registers_scheduler_after_saving(self):
        plugin = SubscriptionHubPlugin()
        plugin._settings_loaded = True
        plugin._settings = merge_settings(plugin._default_settings, {"poll_cron": "*/7 * * * *"})
        ctx = FakeContext(args=["视频", "123456"], http_responses=[self.user_info_response()])

        plugin.handle_subscribe_bilibili(ctx)

        self.assertEqual(len(ctx.config_writes), 1)
        self.assertEqual(ctx.scheduler_creates, [{
            "task_id": "subscription-hub-poll",
            "cron": "*/7 * * * *",
            "payload": {"kind": "subscription_poll"},
            "log_label": "Bilibili 推送轮询",
        }])
        self.assertEqual(ctx.results[-1], {"handled": True})

    def test_unsubscribe_command_registers_scheduler_after_saving(self):
        plugin = SubscriptionHubPlugin()
        plugin._settings_loaded = True
        plugin._settings = merge_settings(plugin._default_settings, {
            "subscriptions": [{
                "uid": "123456",
                "target_type": "group",
                "target_id": "10000",
                "services": ["video"],
            }],
        })
        ctx = FakeContext(args=["123456"])

        plugin.handle_unsubscribe_bilibili(ctx)

        self.assertEqual(len(ctx.config_writes), 1)
        self.assertEqual(ctx.scheduler_creates[0]["task_id"], "subscription-hub-poll")
        self.assertEqual(ctx.results[-1], {"handled": True})

    def test_status_and_list_commands_restore_scheduler_for_existing_subscriptions(self):
        settings = self.subscription_settings()
        plugin = SubscriptionHubPlugin()

        status_ctx = FakeContext(config_values=settings)
        plugin.handle_status_command(status_ctx)

        self.assertEqual(status_ctx.scheduler_creates[0]["task_id"], "subscription-hub-poll")
        self.assertEqual(status_ctx.scheduler_creates[0]["cron"], "*/5 * * * *")

        list_ctx = FakeContext(config_values=settings)
        plugin.handle_subscription_list(list_ctx)

        self.assertEqual(list_ctx.scheduler_creates[0]["task_id"], "subscription-hub-poll")
        self.assertIn("Bilibili 测试 UP（UID 123456）", list_ctx.texts[0])

    def test_bilibili_http_failure_does_not_push_or_mark_seen(self):
        plugin = SubscriptionHubPlugin()
        settings = self.subscription_settings(tokens=[{
            "id": "primary",
            "label": "主 Cookie",
            "platform": "bilibili",
            "secret_key": "bili.primary",
            "enabled": True,
        }])
        ctx = FakeContext(
            config_values=settings,
            http_responses=[
                self.nav_response(),
                {"status_code": 412, "body_text": ""},
            ],
            secrets={"bili.primary": "SESSDATA=token"},
        )

        plugin.handle_scheduler_trigger(ctx)

        self.assertEqual(ctx.results[-1], {"handled": True, "sent": 0})
        self.assertEqual(ctx.render_calls, [])
        self.assertEqual(ctx.messages, [])
        self.assertEqual(ctx.storage_sets, [])
        self.assertTrue(any(log["message"] == "Bilibili 风控拦截" and log["fields"].get("status_code") == 412 for log in ctx.logs))

    def test_scheduler_trigger_handles_http_action_error(self):
        plugin = SubscriptionHubPlugin()
        settings = self.subscription_settings(tokens=[{
            "id": "primary",
            "label": "主 Cookie",
            "platform": "bilibili",
            "secret_key": "bili.primary",
            "enabled": True,
        }])
        ctx = FakeContext(
            config_values=settings,
            http_responses=[ActionError("permission.scope_violation", "http.request target is outside the granted scope")],
            secrets={"bili.primary": "SESSDATA=token"},
        )

        plugin.handle_scheduler_trigger(ctx)

        self.assertEqual(ctx.results[-1], {"handled": True, "sent": 0})
        self.assertEqual(ctx.render_calls, [])
        self.assertEqual(ctx.messages, [])
        self.assertEqual(ctx.storage_sets, [])
        self.assertTrue(any(log["message"] == "Bilibili 请求失败" for log in ctx.logs))

    def test_bilibili_json_error_does_not_push_or_mark_seen(self):
        plugin = SubscriptionHubPlugin()
        settings = self.subscription_settings(tokens=[{
            "id": "primary",
            "label": "主 Cookie",
            "platform": "bilibili",
            "secret_key": "bili.primary",
            "enabled": True,
        }])
        ctx = FakeContext(
            config_values=settings,
            http_responses=[
                self.nav_response(),
                self.dynamic_response([], code=-352, message="风控校验失败"),
            ],
            secrets={"bili.primary": "SESSDATA=token"},
        )

        plugin.handle_scheduler_trigger(ctx)

        self.assertEqual(ctx.results[-1], {"handled": True, "sent": 0})
        self.assertEqual(ctx.render_calls, [])
        self.assertEqual(ctx.messages, [])
        self.assertEqual(ctx.storage_sets, [])
        self.assertTrue(any(log["fields"].get("bilibili_code") == -352 for log in ctx.logs))

    def test_cookie_probe_stops_poll_when_cookie_is_not_logged_in(self):
        plugin = SubscriptionHubPlugin()
        settings = self.subscription_settings(tokens=[{
            "id": "primary",
            "label": "主 Cookie",
            "platform": "bilibili",
            "secret_key": "bili.primary",
            "enabled": True,
        }])
        ctx = FakeContext(
            config_values=settings,
            http_responses=[self.nav_response(is_login=False)],
            secrets={"bili.primary": "SESSDATA=expired"},
        )

        plugin.handle_scheduler_trigger(ctx)

        self.assertEqual(len(ctx.http_requests), 1)
        self.assertIn("/x/web-interface/nav", ctx.http_requests[0]["url"])
        self.assertEqual(ctx.render_calls, [])
        self.assertEqual(ctx.messages, [])
        self.assertEqual(ctx.storage_sets, [])
        self.assertTrue(any(log["message"] == "Bilibili Cookie 无效或已过期" for log in ctx.logs))

    def test_scheduler_trigger_renders_marks_seen_then_sends_image(self):
        plugin = SubscriptionHubPlugin()
        settings = self.subscription_settings(tokens=[{
            "id": "primary",
            "label": "主 Cookie",
            "platform": "bilibili",
            "secret_key": "bili.primary",
            "enabled": True,
        }])
        ctx = FakeContext(
            config_values=settings,
            secrets={"bili.primary": "SESSDATA=token"},
            storage={"dynamic-baseline:bilibili-123456-group-10000": int(time.time()) - 60},
            http_responses=[
                self.nav_response(),
                self.dynamic_response([self.video_item("987", "新视频")]),
            ],
            render_result={"image_path": "rendered.png"},
        )

        plugin.handle_scheduler_trigger(ctx)

        self.assertEqual(ctx.results, [])
        self.assertEqual(ctx.render_calls[0]["template"], "bilibili-update")
        self.assertEqual(ctx.storage_sets, [
            {"key": "seen:bilibili-123456-group-10000:video:987", "value": True},
            {"key": "dynamic-baseline:bilibili-123456-group-10000", "value": ctx.render_calls[0]["data"]["pub_ts"]},
        ])
        self.assertEqual(ctx.messages, [{
            "segments": [{"type": "image", "data": {"file": "rendered.png"}}],
            "target_type": "group",
            "target_id": "10000",
        }])
        self.assertEqual([action["kind"] for action in ctx.actions], ["render_image", "storage_set", "storage_set", "send_message"])
        self.assertEqual(ctx.http_requests[0]["headers"].get("Cookie"), "SESSDATA=token")
        self.assertIn("Chrome/", ctx.http_requests[0]["headers"].get("User-Agent", ""))
        self.assertEqual(ctx.http_requests[0]["headers"].get("Accept-Language"), "zh-CN,zh;q=0.9,en;q=0.8")
        self.assertEqual(ctx.http_requests[1]["headers"].get("Referer"), "https://space.bilibili.com/123456/dynamic")

    def test_scheduler_trigger_sends_only_one_update_per_event(self):
        plugin = SubscriptionHubPlugin()
        settings = self.subscription_settings(tokens=[{
            "id": "primary",
            "label": "主 Cookie",
            "platform": "bilibili",
            "secret_key": "bili.primary",
            "enabled": True,
        }])
        ctx = FakeContext(
            config_values=settings,
            secrets={"bili.primary": "SESSDATA=token"},
            storage={"dynamic-baseline:bilibili-123456-group-10000": int(time.time()) - 60},
            http_responses=[
                self.nav_response(),
                self.dynamic_response([
                    self.video_item("987", "第一条视频", pub_ts=int(time.time()) - 20),
                    self.video_item("988", "第二条视频", pub_ts=int(time.time()) - 10),
                ]),
            ],
        )

        plugin.handle_scheduler_trigger(ctx)

        self.assertEqual(len(ctx.render_calls), 1)
        self.assertEqual(len(ctx.messages), 1)
        self.assertEqual(ctx.storage_sets[0], {"key": "seen:bilibili-123456-group-10000:video:987", "value": True})
        self.assertEqual(ctx.storage_sets[1]["key"], "dynamic-baseline:bilibili-123456-group-10000")

    def test_live_start_push_contains_cover_and_start_time(self):
        plugin = SubscriptionHubPlugin()
        settings = self.subscription_settings(
            tokens=[{
                "id": "primary",
                "label": "主 Cookie",
                "platform": "bilibili",
                "secret_key": "bili.primary",
                "enabled": True,
            }],
            subscriptions=[{
                **self.subscription_settings()["subscriptions"][0],
                "services": ["live"],
            }],
        )
        ctx = FakeContext(
            config_values=settings,
            secrets={"bili.primary": "SESSDATA=token"},
            http_responses=[
                self.nav_response(),
                self.live_status_response(live_status=1, live_time=1700000000),
            ],
            render_result={"image_path": "live-start.png"},
        )

        plugin.handle_scheduler_trigger(ctx)

        data = ctx.render_calls[0]["data"]
        self.assertEqual(data["service"], "直播")
        self.assertEqual(data["status_label"], "直播中")
        self.assertEqual(data["live_event"], "started")
        self.assertEqual(data["content_text"].splitlines()[0], "直播中")
        self.assertIn("开播时间：", data["content_text"])
        self.assertEqual(data["live_started_at"], data["created_at"])
        self.assertEqual(data["images"], [{"url": "https://i0.hdslb.com/live-cover.jpg"}])
        self.assertEqual(data["media_items"][0]["url"], "https://i0.hdslb.com/live-cover.jpg")
        self.assertEqual(ctx.messages[0]["segments"][0]["data"]["file"], "live-start.png")
        self.assertEqual(ctx.storage_sets[0]["key"], "seen:bilibili-123456-group-10000:live:live-123456-1-22913442-1700000000")
        self.assertEqual(ctx.storage_sets[1], {"key": "live-status:bilibili-123456-group-10000", "value": 1})
        self.assertEqual(ctx.storage_sets[2]["key"], "live-info:bilibili-123456-group-10000")

    def test_live_offline_baseline_does_not_push(self):
        plugin = SubscriptionHubPlugin()
        settings = self.subscription_settings(
            tokens=[{
                "id": "primary",
                "label": "主 Cookie",
                "platform": "bilibili",
                "secret_key": "bili.primary",
                "enabled": True,
            }],
            subscriptions=[{
                **self.subscription_settings()["subscriptions"][0],
                "services": ["live"],
            }],
        )
        ctx = FakeContext(
            config_values=settings,
            secrets={"bili.primary": "SESSDATA=token"},
            http_responses=[
                self.nav_response(),
                self.live_status_response(live_status=0, live_time=0),
            ],
        )

        plugin.handle_scheduler_trigger(ctx)

        self.assertEqual(ctx.render_calls, [])
        self.assertEqual(ctx.messages, [])
        self.assertEqual(ctx.storage_sets, [{"key": "live-status:bilibili-123456-group-10000", "value": 0}])
        self.assertEqual(ctx.results[-1], {"handled": True, "sent": 0})

    def test_live_end_push_uses_stored_cover_and_detected_time(self):
        plugin = SubscriptionHubPlugin()
        settings = self.subscription_settings(
            tokens=[{
                "id": "primary",
                "label": "主 Cookie",
                "platform": "bilibili",
                "secret_key": "bili.primary",
                "enabled": True,
            }],
            subscriptions=[{
                **self.subscription_settings()["subscriptions"][0],
                "services": ["live"],
            }],
        )
        storage = {
            "live-status:bilibili-123456-group-10000": 1,
            "live-info:bilibili-123456-group-10000": {
                "title": "真实直播标题",
                "url": "https://live.bilibili.com/22913442",
                "images": [{"url": "https://i0.hdslb.com/live-cover.jpg"}],
                "pub_ts": 1700000000,
                "created_at": "2023年11月15日 06:13",
                "live_started_at": "2023年11月15日 06:13",
                "room_id": "22913442",
                "author": {"name": "测试主播", "avatar": "https://i0.hdslb.com/live-face.jpg", "uid": "123456"},
            },
        }
        ctx = FakeContext(
            config_values=settings,
            secrets={"bili.primary": "SESSDATA=token"},
            storage=storage,
            http_responses=[
                self.nav_response(),
                self.live_status_response(live_status=0, live_time=0, include_cover=False),
            ],
            render_result={"image_path": "live-end.png"},
        )

        plugin.handle_scheduler_trigger(ctx)

        data = ctx.render_calls[0]["data"]
        self.assertEqual(data["status_label"], "已下播")
        self.assertEqual(data["live_event"], "ended")
        self.assertEqual(data["content_text"].splitlines()[0], "已下播")
        self.assertIn("开播时间：2023年11月15日 06:13", data["content_text"])
        self.assertIn("下播检测：", data["content_text"])
        self.assertTrue(data["live_detected_at"])
        self.assertEqual(data["images"], [{"url": "https://i0.hdslb.com/live-cover.jpg"}])
        self.assertEqual(ctx.messages[0]["segments"][0]["data"]["file"], "live-end.png")
        self.assertEqual(ctx.storage_sets[0]["key"], "seen:bilibili-123456-group-10000:live:live-123456-0-22913442-1700000000")
        self.assertEqual(ctx.storage_sets[1], {"key": "live-status:bilibili-123456-group-10000", "value": 0})

    def test_live_start_uses_source_time_in_seen_key(self):
        plugin = SubscriptionHubPlugin()
        settings = self.subscription_settings(
            tokens=[{
                "id": "primary",
                "label": "主 Cookie",
                "platform": "bilibili",
                "secret_key": "bili.primary",
                "enabled": True,
            }],
            subscriptions=[{
                **self.subscription_settings()["subscriptions"][0],
                "services": ["live"],
            }],
        )
        ctx = FakeContext(
            config_values=settings,
            secrets={"bili.primary": "SESSDATA=token"},
            storage={
                "seen:bilibili-123456-group-10000:live:live-123456-1-22913442-1700000000": True,
                "live-status:bilibili-123456-group-10000": 0,
            },
            http_responses=[
                self.nav_response(),
                self.live_status_response(live_status=1, live_time=1700003600),
            ],
        )

        plugin.handle_scheduler_trigger(ctx)

        self.assertEqual(len(ctx.render_calls), 1)
        self.assertEqual(ctx.storage_sets[0]["key"], "seen:bilibili-123456-group-10000:live:live-123456-1-22913442-1700003600")

    def test_manual_check_defaults_to_current_target(self):
        plugin = SubscriptionHubPlugin()
        settings = self.subscription_settings(
            tokens=[{
                "id": "primary",
                "label": "主 Cookie",
                "platform": "bilibili",
                "secret_key": "bili.primary",
                "enabled": True,
            }],
            subscriptions=[
                self.subscription_settings()["subscriptions"][0],
                {
                    "id": "bilibili-654321-group-20000",
                    "platform": "bilibili",
                    "uid": "654321",
                    "name": "其他 UP",
                    "target_type": "group",
                    "target_id": "20000",
                    "services": ["video"],
                    "subscribers": [{"id": "43", "nickname": "其他订阅人"}],
                    "enabled": True,
                },
            ],
        )
        now = int(time.time())
        ctx = FakeContext(
            config_values=settings,
            secrets={"bili.primary": "SESSDATA=token"},
            storage={"dynamic-baseline:bilibili-123456-group-10000": now - 60},
            http_responses=[
                self.nav_response(),
                self.dynamic_response([self.video_item("987", "当前会话视频", pub_ts=now - 20)]),
            ],
        )

        plugin.handle_manual_check(ctx)

        self.assertEqual(len(ctx.render_calls), 1)
        self.assertIn("123456", ctx.http_requests[1]["url"])
        self.assertNotIn("654321", " ".join(request["url"] for request in ctx.http_requests))
        self.assertEqual(ctx.messages[0]["target_id"], "10000")

    def test_manual_check_all_can_scan_other_targets(self):
        plugin = SubscriptionHubPlugin()
        settings = self.subscription_settings(
            tokens=[{
                "id": "primary",
                "label": "主 Cookie",
                "platform": "bilibili",
                "secret_key": "bili.primary",
                "enabled": True,
            }],
            subscriptions=[{
                "id": "bilibili-654321-group-20000",
                "platform": "bilibili",
                "uid": "654321",
                "name": "其他 UP",
                "target_type": "group",
                "target_id": "20000",
                "services": ["video"],
                "subscribers": [{"id": "43", "nickname": "其他订阅人"}],
                "enabled": True,
            }],
        )
        now = int(time.time())
        ctx = FakeContext(
            args=["全部"],
            config_values=settings,
            secrets={"bili.primary": "SESSDATA=token"},
            storage={"dynamic-baseline:bilibili-654321-group-20000": now - 60},
            http_responses=[
                self.nav_response(),
                self.dynamic_response([self.video_item("987", "其他会话视频", pub_ts=now - 20)]),
            ],
        )

        plugin.handle_manual_check(ctx)

        self.assertEqual(len(ctx.render_calls), 1)
        self.assertIn("654321", ctx.http_requests[1]["url"])
        self.assertEqual(ctx.messages[0]["target_id"], "20000")

    def test_preview_card_uses_sample_data_without_http_or_storage(self):
        plugin = SubscriptionHubPlugin()
        ctx = FakeContext(args=["转发"], render_result={"image_path": "preview.png"})

        plugin.handle_preview_card(ctx)

        self.assertEqual(ctx.http_requests, [])
        self.assertEqual(ctx.storage_sets, [])
        self.assertEqual(len(ctx.render_calls), 1)
        self.assertEqual(ctx.render_calls[0]["template"], "bilibili-update")
        self.assertEqual(ctx.render_calls[0]["data"]["service"], "转发")
        self.assertEqual(ctx.render_calls[0]["data"]["original"]["title"], "原动态视频标题")
        self.assertEqual(ctx.messages, [{
            "segments": [{"type": "image", "data": {"file": "preview.png"}}],
            "target_type": None,
            "target_id": None,
        }])

    def test_parse_preview_url_normalizes_supported_links(self):
        self.assertEqual(parse_preview_url("www.bilibili.com/video/BV1c5qEBjEtJ/?trackid=x"), {
            "kind": "video",
            "bvid": "BV1c5qEBjEtJ",
            "url": "https://www.bilibili.com/video/BV1c5qEBjEtJ",
        })
        self.assertEqual(parse_preview_url("https://www.bilibili.com/opus/1194416231669563410?x=1#reply"), {
            "kind": "opus",
            "opus_id": "1194416231669563410",
            "url": "https://www.bilibili.com/opus/1194416231669563410",
        })
        self.assertEqual(parse_preview_url("https://live.bilibili.com/22913442?live_from&launch_id&trackid"), {
            "kind": "live",
            "room_id": "22913442",
            "url": "https://live.bilibili.com/22913442",
        })

    def test_preview_card_fetches_real_video_link(self):
        plugin = SubscriptionHubPlugin()
        ctx = FakeContext(
            args=["https://www.bilibili.com/video/BV1c5qEBjEtJ/?trackid=web_related"],
            config_values=self.cookie_settings(),
            http_responses=[self.video_preview_response()],
            secrets={"bili.primary": "SESSDATA=token; bili_jct=token"},
            render_result={"image_path": "video-preview.png"},
        )

        plugin.handle_preview_card(ctx)

        self.assertEqual(len(ctx.http_requests), 1)
        self.assertIn("/x/web-interface/view?bvid=BV1c5qEBjEtJ", ctx.http_requests[0]["url"])
        self.assertEqual(ctx.http_requests[0]["headers"].get("Cookie"), "SESSDATA=token; bili_jct=token")
        data = ctx.render_calls[0]["data"]
        self.assertEqual(data["service"], "视频")
        self.assertEqual(data["title"], "真实视频标题")
        self.assertEqual(data["content_text"], "真实视频简介")
        self.assertEqual(data["url"], "https://www.bilibili.com/video/BV1c5qEBjEtJ")
        self.assertEqual(data["author"]["name"], "测试 UP")
        self.assertEqual(data["subscription"]["name"], "测试 UP")
        self.assertEqual(data["subscription"]["uid"], "123456")
        self.assertEqual(data["duration_text"], "1:01:11")
        self.assertEqual(data["media_items"][0]["duration_text"], "1:01:11")
        self.assertEqual(data["images"], [{"url": "https://i0.hdslb.com/video-cover.jpg"}])
        self.assertEqual(ctx.messages[0]["segments"][0]["data"]["file"], "video-preview.png")

    def test_preview_card_fetches_real_opus_link(self):
        plugin = SubscriptionHubPlugin()
        ctx = FakeContext(
            args=["https://www.bilibili.com/opus/1194416231669563410?spm_id_from=share"],
            config_values=self.cookie_settings(),
            http_responses=[self.opus_preview_response()],
            secrets={"bili.primary": "SESSDATA=token; bili_jct=token"},
        )

        plugin.handle_preview_card(ctx)

        self.assertEqual(len(ctx.http_requests), 1)
        self.assertIn("/x/polymer/web-dynamic/v1/opus/detail?id=1194416231669563410", ctx.http_requests[0]["url"])
        self.assertEqual(ctx.http_requests[0]["headers"].get("Cookie"), "SESSDATA=token; bili_jct=token")
        data = ctx.render_calls[0]["data"]
        self.assertEqual(data["service"], "图文")
        self.assertEqual(data["title"], "测试 UP 发布图文动态")
        self.assertEqual(data["content_text"], "真实图文动态正文")
        self.assertEqual(data["url"], "https://www.bilibili.com/opus/1194416231669563410")
        self.assertEqual(data["images"], [
            {"url": "https://i0.hdslb.com/dyn/1.jpg", "width": 800, "height": 800},
            {"url": "https://i0.hdslb.com/dyn/2.jpg", "width": 800, "height": 800},
        ])

    def test_preview_card_reports_352_as_cookie_required_without_rendering(self):
        plugin = SubscriptionHubPlugin()
        ctx = FakeContext(
            args=["https://www.bilibili.com/opus/1194416231669563410"],
            http_responses=[self.opus_preview_response(code=-352, message="-352")],
        )

        plugin.handle_preview_card(ctx)

        self.assertEqual(len(ctx.http_requests), 1)
        self.assertEqual(ctx.render_calls, [])
        self.assertIn("Cookie", ctx.texts[0])
        self.assertEqual(ctx.results[-1], {"handled": True, "sent": 0})

    def test_preview_card_parses_opus_detail_module_list(self):
        plugin = SubscriptionHubPlugin()
        ctx = FakeContext(
            args=["https://www.bilibili.com/opus/1194416231669563410"],
            config_values=self.cookie_settings(),
            http_responses=[self.opus_detail_modules_response()],
            secrets={"bili.primary": "SESSDATA=token; bili_jct=token"},
        )

        plugin.handle_preview_card(ctx)

        self.assertEqual(len(ctx.http_requests), 1)
        self.assertIn("/x/polymer/web-dynamic/v1/opus/detail?id=1194416231669563410", ctx.http_requests[0]["url"])
        data = ctx.render_calls[0]["data"]
        self.assertEqual(data["service"], "图文")
        self.assertEqual(data["title"], "真实动态标题")
        self.assertIn("正文标题", data["content_text"])
        self.assertIn("真实正文", data["content_text"])
        self.assertIn("#测试话题#", data["content_text"])
        self.assertIn("#兜底话题#", data["content_text"])
        self.assertIn("BV1c5qEBjEtJ", data["content_text"])
        self.assertIn('真实正文 <span class="rich-text-topic bili-rich-text-module topic">#测试话题#</span>', data["content_html"])
        self.assertIn('<span class="rich-text-topic bili-rich-text-module topic"> #兜底话题#</span>', data["content_html"])
        self.assertIn('<span class="rich-text-at bili-rich-text-module at">@银狼</span>', data["content_html"])
        self.assertIn('<span class="rich-text-lottery bili-rich-text-module lottery">互动抽奖</span>', data["content_html"])
        self.assertIn('<span class="rich-text-link bili-rich-text-link video">BV1c5qEBjEtJ</span>', data["content_html"])
        self.assertIn('<span class="rich-text-link bili-rich-text-module goods taobao">周边</span>', data["content_html"])
        self.assertIn('<span class="rich-text-link bili-rich-text-module vote">投票</span>', data["content_html"])
        self.assertEqual(data["author"]["name"], "真实 UP")
        self.assertEqual(data["subscription"]["uid"], "4070943")
        self.assertEqual(data["images"], [
            {"url": "https://i0.hdslb.com/opus-1.jpg", "width": 1000, "height": 1200},
        ])

    def test_preview_card_fetches_real_live_link(self):
        plugin = SubscriptionHubPlugin()
        ctx = FakeContext(
            args=["live.bilibili.com/22913442?live_from&launch_id&trackid"],
            config_values=self.cookie_settings(),
            http_responses=[
                self.live_preview_response(),
                self.live_status_response(),
            ],
            secrets={"bili.primary": "SESSDATA=token; bili_jct=token"},
        )

        plugin.handle_preview_card(ctx)

        self.assertEqual(len(ctx.http_requests), 2)
        self.assertIn("/room/v1/Room/get_info?room_id=22913442", ctx.http_requests[0]["url"])
        self.assertEqual(ctx.http_requests[0]["headers"].get("Cookie"), "SESSDATA=token; bili_jct=token")
        self.assertIn("/room/v1/Room/get_status_info_by_uids?uids[]=123456", ctx.http_requests[1]["url"])
        data = ctx.render_calls[0]["data"]
        self.assertEqual(data["service"], "直播")
        self.assertEqual(data["title"], "真实直播标题")
        self.assertEqual(data["content_text"], "直播中")
        self.assertEqual(data["url"], "https://live.bilibili.com/22913442")
        self.assertEqual(data["author"]["name"], "测试主播")
        self.assertEqual(data["author"]["avatar"], "https://i0.hdslb.com/live-face.jpg")
        self.assertEqual(data["author"]["uid"], "123456")
        self.assertEqual(data["images"], [{"url": "https://i0.hdslb.com/live-cover.jpg"}])

    def test_preview_card_renders_live_link_when_status_lookup_fails(self):
        plugin = SubscriptionHubPlugin()
        ctx = FakeContext(
            args=["https://live.bilibili.com/22913442"],
            config_values=self.cookie_settings(),
            http_responses=[
                self.live_preview_response(),
                ActionError("permission.scope_violation", "http.request target is outside the granted scope"),
            ],
            secrets={"bili.primary": "SESSDATA=token; bili_jct=token"},
        )

        plugin.handle_preview_card(ctx)

        self.assertEqual(len(ctx.http_requests), 2)
        data = ctx.render_calls[0]["data"]
        self.assertEqual(data["service"], "直播")
        self.assertEqual(data["title"], "真实直播标题")
        self.assertEqual(data["author"], {"name": "123456", "avatar": "", "uid": "123456"})
        self.assertEqual(data["images"], [{"url": "https://i0.hdslb.com/live-cover.jpg"}])
        self.assertEqual(ctx.messages[0]["segments"][0]["data"]["file"], "plugin-test.png")

    def test_preview_card_rejects_unsupported_url_without_rendering(self):
        plugin = SubscriptionHubPlugin()
        ctx = FakeContext(args=["https://example.com/video/BV1c5qEBjEtJ"])

        plugin.handle_preview_card(ctx)

        self.assertEqual(ctx.http_requests, [])
        self.assertEqual(ctx.render_calls, [])
        self.assertIn("暂不支持", ctx.texts[0])
        self.assertEqual(ctx.results[-1], {"handled": True, "sent": 0})

    def test_preview_card_reports_api_error_without_rendering(self):
        plugin = SubscriptionHubPlugin()
        ctx = FakeContext(
            args=["https://www.bilibili.com/video/BV1c5qEBjEtJ"],
            http_responses=[self.video_preview_response(code=-412)],
        )

        plugin.handle_preview_card(ctx)

        self.assertEqual(len(ctx.http_requests), 1)
        self.assertEqual(ctx.render_calls, [])
        self.assertIn("风控拦截", ctx.texts[0])
        self.assertEqual(ctx.results[-1], {"handled": True, "sent": 0})

    def test_preview_card_keeps_sample_types_without_http(self):
        plugin = SubscriptionHubPlugin()
        ctx = FakeContext(args=["直播"])

        plugin.handle_preview_card(ctx)

        self.assertEqual(ctx.http_requests, [])
        self.assertEqual(len(ctx.render_calls), 1)
        self.assertEqual(ctx.render_calls[0]["data"]["service"], "直播")

    def test_first_successful_dynamic_poll_sets_baseline_without_push(self):
        plugin = SubscriptionHubPlugin()
        settings = self.subscription_settings(tokens=[{
            "id": "primary",
            "label": "主 Cookie",
            "platform": "bilibili",
            "secret_key": "bili.primary",
            "enabled": True,
        }])
        pub_ts = int(time.time()) - 30
        ctx = FakeContext(
            config_values=settings,
            secrets={"bili.primary": "SESSDATA=token"},
            http_responses=[
                self.nav_response(),
                self.dynamic_response([self.video_item("987", "已有动态", pub_ts=pub_ts)]),
            ],
        )

        plugin.handle_scheduler_trigger(ctx)

        self.assertEqual(ctx.render_calls, [])
        self.assertEqual(ctx.messages, [])
        self.assertEqual(ctx.storage_sets, [{"key": "dynamic-baseline:bilibili-123456-group-10000", "value": pub_ts}])
        self.assertEqual(ctx.results[-1], {"handled": True, "sent": 0})

    def test_old_and_pinned_dynamic_items_are_skipped(self):
        plugin = SubscriptionHubPlugin()
        settings = self.subscription_settings(
            dynamic_time_range_seconds=120,
            tokens=[{
                "id": "primary",
                "label": "主 Cookie",
                "platform": "bilibili",
                "secret_key": "bili.primary",
                "enabled": True,
            }],
        )
        baseline = int(time.time()) - 3600
        old_item = self.video_item("987", "旧动态", pub_ts=int(time.time()) - 600)
        pinned_item = self.video_item("988", "置顶动态", pub_ts=int(time.time()) - 30)
        pinned_item["modules"]["module_tag"] = {"text": "置顶"}
        ctx = FakeContext(
            config_values=settings,
            secrets={"bili.primary": "SESSDATA=token"},
            storage={"dynamic-baseline:bilibili-123456-group-10000": baseline},
            http_responses=[
                self.nav_response(),
                self.dynamic_response([pinned_item, old_item]),
            ],
        )

        plugin.handle_scheduler_trigger(ctx)

        self.assertEqual(ctx.render_calls, [])
        self.assertEqual(ctx.messages, [])
        self.assertEqual(ctx.storage_sets, [])
        self.assertEqual(ctx.results[-1], {"handled": True, "sent": 0})

    def test_dynamic_push_uses_oldest_new_item_first(self):
        plugin = SubscriptionHubPlugin()
        settings = self.subscription_settings(tokens=[{
            "id": "primary",
            "label": "主 Cookie",
            "platform": "bilibili",
            "secret_key": "bili.primary",
            "enabled": True,
        }])
        now = int(time.time())
        ctx = FakeContext(
            config_values=settings,
            secrets={"bili.primary": "SESSDATA=token"},
            storage={"dynamic-baseline:bilibili-123456-group-10000": now - 60},
            http_responses=[
                self.nav_response(),
                self.dynamic_response([
                    self.video_item("988", "较新视频", pub_ts=now - 10),
                    self.video_item("987", "较早视频", pub_ts=now - 30),
                ]),
            ],
        )

        plugin.handle_scheduler_trigger(ctx)

        self.assertEqual(ctx.render_calls[0]["data"]["title"], "较早视频")
        self.assertEqual(ctx.storage_sets[0]["key"], "seen:bilibili-123456-group-10000:video:987")

    def test_missing_cookie_logs_clear_warning(self):
        plugin = SubscriptionHubPlugin()
        settings = self.subscription_settings()
        ctx = FakeContext(
            config_values=settings,
            http_responses=[self.dynamic_response([])],
        )

        plugin.handle_scheduler_trigger(ctx)

        self.assertTrue(any("Bilibili Cookie" in log["message"] for log in ctx.logs))
        self.assertEqual(ctx.messages, [])

    def test_scheduler_uses_only_bilibili_tokens(self):
        plugin = SubscriptionHubPlugin()
        settings = self.subscription_settings(tokens=[
            {
                "id": "qq-primary",
                "platform": "qq",
                "label": "QQ CK",
                "secret_key": "qq.primary",
                "enabled": True,
            },
            {
                "id": "bilibili-primary",
                "platform": "bilibili",
                "label": "主 CK",
                "secret_key": "bili.primary",
                "enabled": True,
            },
        ])
        ctx = FakeContext(
            config_values=settings,
            secrets={
                "qq.primary": "SESSDATA=wrong",
                "bili.primary": "SESSDATA=token",
            },
            storage={"dynamic-baseline:bilibili-123456-group-10000": int(time.time()) - 60},
            http_responses=[
                self.nav_response(),
                self.dynamic_response([self.video_item("987", "新视频")]),
            ],
        )

        plugin.handle_scheduler_trigger(ctx)

        self.assertEqual(ctx.http_requests[0]["headers"].get("Cookie"), "SESSDATA=token")

    def test_build_status_text_summarizes_visible_state(self):
        settings = merge_settings({}, {
            "enabled": True,
            "poll_cron": "*/10 * * * *",
            "dynamic_time_range_seconds": 1800,
            "tokens": [
                {"id": "primary", "label": "主 Cookie", "platform": "bilibili", "secret_key": "bili.primary", "enabled": True},
                {"id": "backup", "label": "备用 Cookie", "platform": "bilibili", "secret_key": "bili.backup", "enabled": False},
            ],
            "subscriptions": [
                {"uid": "123456", "target_type": "group", "target_id": "10000"},
                {"uid": "654321", "target_type": "private", "target_id": "42", "enabled": False},
            ],
        })
        self.assertEqual(build_status_text(settings), "\n".join([
            "订阅中心",
            "状态：启用",
            "订阅：1/2",
            "Cookie：1/2",
            "轮询：*/10 * * * *",
            "动态窗口：1800 秒",
        ]))


if __name__ == "__main__":
    unittest.main()
