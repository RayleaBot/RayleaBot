import json
import os
import sys
import time
import unittest


PLUGIN_DIR = os.path.dirname(os.path.dirname(__file__))
BUILTIN_DIR = os.path.dirname(PLUGIN_DIR)
sys.path.insert(0, BUILTIN_DIR)
sys.path.insert(0, PLUGIN_DIR)

from bilibili import dynamic_detail_url, dynamic_updates, opus_detail_url, parse_preview_url
from main import (
    SUBSCRIBE_BILIBILI_USAGE,
    SubscriptionHubPlugin,
    UNSUBSCRIBE_BILIBILI_USAGE,
    add_bilibili_subscription,
    build_status_text,
    format_subscription_list,
    normalize_bilibili_event_payload,
    parse_bilibili_command_args,
    preview_response_document,
    remove_bilibili_subscription,
    subscription_matches_event,
)
from rendering import build_render_data
from settings import merge_settings
from testkit import FakePluginContext as FakeContext


class SubscriptionHubTests(unittest.TestCase):
    def subscription_settings(self, **overrides):
        settings = {
            "enabled": True,
            "subscriptions": [{
                "id": "bilibili-123456-group-10000",
                "platform": "bilibili",
                "uid": "123456",
                "name": "测试 UP",
                "target_type": "group",
                "target_id": "10000",
                "target_name": "测试群",
                "services": ["video"],
                "subscribers": [{"id": "42", "nickname": "订阅人"}],
                "enabled": True,
            }],
        }
        settings.update(overrides)
        return settings

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

    def repost_item(self, dynamic_id="1210053730841395205", pub_ts=1780585560):
        return {
            "id_str": dynamic_id,
            "type": "DYNAMIC_TYPE_FORWARD",
            "basic": {"jump_url": f"//t.bilibili.com/{dynamic_id}"},
            "modules": {
                "module_author": {
                    "mid": "123456",
                    "name": "乐正绫",
                    "face": "//i0.hdslb.com/bfs/face/repost.jpg",
                    "pub_ts": pub_ts,
                    "pub_time": "2026年06月04日 20:26",
                },
                "module_dynamic": {
                    "desc": {
                        "text": "你是我爱这世界的原因[星星眼] #乐正绫单人集#",
                        "rich_text_nodes": [
                            {"type": "RICH_TEXT_NODE_TYPE_TEXT", "text": "你是我爱这世界的原因"},
                            {
                                "type": "RICH_TEXT_NODE_TYPE_EMOJI",
                                "text": "[星星眼]",
                                "emoji": {"icon_url": "//i0.hdslb.com/bfs/emote/star.png", "text": "[星星眼]", "size": 1},
                            },
                            {"type": "RICH_TEXT_NODE_TYPE_WEB", "text": "#乐正绫单人集#"},
                        ],
                    },
                },
            },
            "orig": {
                "id_str": "90001",
                "type": "DYNAMIC_TYPE_AV",
                "basic": {"jump_url": "//www.bilibili.com/video/BV1ORIGINAL"},
                "modules": {
                    "module_author": {
                        "mid": "271828",
                        "name": "WOVOP",
                        "face": "//i0.hdslb.com/bfs/face/original.jpg",
                        "pub_ts": 1780585200,
                        "pub_time": "2026年06月04日 20:20",
                    },
                    "module_dynamic": {
                        "topic": {
                            "id": 10001,
                            "name": "洛天依",
                            "jump_url": "https://m.bilibili.com/topic-detail?topic_id=10001",
                        },
                        "desc": {
                            "text": "新歌来了！",
                            "rich_text_nodes": [
                                {"type": "", "orig_text": "#洛天依#"},
                                {"type": "RICH_TEXT_NODE_TYPE_TEXT", "text": " 新歌来了！"},
                            ],
                        },
                        "major": {
                            "type": "MAJOR_TYPE_ARCHIVE",
                            "archive": {
                                "title": "【乐正绫ACE原创】你是我爱这世界的原因",
                                "desc": "视策划旧斜日红狐泽生日快乐！",
                                "cover": "//i0.hdslb.com/bfs/archive/original-cover.jpg",
                                "duration_text": "03:17",
                            },
                        },
                    },
                },
            },
        }

    def dynamic_detail_response(self, item=None):
        return {
            "status_code": 200,
            "body_text": json.dumps({"code": 0, "data": {"item": item or self.repost_item()}}, ensure_ascii=False),
        }

    def opus_detail_response(self):
        return {
            "status_code": 200,
            "body_text": json.dumps({
                "code": 0,
                "data": {
                    "item": {
                        "id_str": "1212948650493214729",
                        "type": "DYNAMIC_TYPE_DRAW",
                        "basic": {"title": "洛天依 发布图文动态"},
                        "modules": [
                            {
                                "module_type": "MODULE_TYPE_AUTHOR",
                                "module_author": {
                                    "mid": "36081646",
                                    "name": "洛天依",
                                    "face": "//i0.hdslb.com/bfs/face/luotianyi.jpg",
                                    "pub_ts": 1781250000,
                                    "pub_time": "2026年06月12日 15:40",
                                },
                            },
                            {
                                "module_type": "MODULE_TYPE_TOPIC",
                                "module_topic": {
                                    "id": 1156147,
                                    "name": "BML-PLAY! 2026",
                                    "jump_url": "https://m.bilibili.com/topic-detail?topic_id=1156147",
                                },
                            },
                            {
                                "module_type": "MODULE_TYPE_CONTENT",
                                "module_content": {
                                    "paragraphs": [
                                        {
                                            "text": {
                                                "nodes": [
                                                    {
                                                        "type": "TEXT_NODE_TYPE_RICH",
                                                        "rich": {
                                                            "type": "RICH_TEXT_NODE_TYPE_WEB",
                                                            "text": "#BML-PLAY! 2026#",
                                                        },
                                                    },
                                                    {
                                                        "type": "TEXT_NODE_TYPE_WORD",
                                                        "word": {"words": "线下演唱会，天依今年也来啦！"},
                                                    },
                                                ],
                                            },
                                            "pic": {
                                                "pics": [
                                                    {"url": "//i0.hdslb.com/bfs/new_dyn/single.jpg", "width": 900, "height": 1600},
                                                ],
                                            },
                                        }
                                    ],
                                },
                            },
                        ],
                    }
                },
            }, ensure_ascii=False),
        }

    def live_event_payload(self, **overrides):
        payload = {
            "kind": "live",
            "uid": "123456",
            "id": "live:123456:22913442:1700000000",
            "room_id": "22913442",
            "service": "live",
            "title": "真实直播标题",
            "summary": "直播中",
            "url": "https://live.bilibili.com/22913442",
            "pub_ts": 1700000000,
            "created_at": "2026-06-08 12:00",
            "author": {"uid": "123456", "name": "测试 UP", "avatar": "https://i0.hdslb.com/face.jpg"},
            "images": [{"url": "https://i0.hdslb.com/live-cover.jpg"}],
            "live_status": 1,
            "live_event": "started",
            "status_label": "直播中",
            "live_started_at": "2026-06-08 12:00",
            "live_detected_at": "2026-06-08 12:00",
        }
        payload.update(overrides)
        return payload

    def dynamic_event_payload(self, **overrides):
        payload = {
            "kind": "dynamic",
            "uid": "123456",
            "id": "dynamic:1194416231669563410",
            "service": "video",
            "title": "真实视频标题",
            "summary": "视频简介",
            "url": "https://www.bilibili.com/video/BV1RayleaBot",
            "pub_ts": 1700000100,
            "created_at": "2026-06-08 12:05",
            "author": {"uid": "123456", "name": "测试 UP"},
            "images": [{"url": "https://i0.hdslb.com/video-cover.jpg"}],
        }
        payload.update(overrides)
        return payload

    def test_manifest_declares_event_consumer_permissions(self):
        with open(os.path.join(PLUGIN_DIR, "info.json"), "r", encoding="utf-8") as handle:
            manifest = json.load(handle)

        self.assertEqual([{"id": "subscriptions", "label": "订阅设置", "entry": "web/index.html"}], manifest["management_ui"]["pages"])
        self.assertIn("event.subscribe", manifest["capabilities"])
        self.assertIn("http.request", manifest["capabilities"])
        self.assertNotIn("scheduler.create", manifest["capabilities"])
        self.assertNotIn("secret.read", manifest["capabilities"])
        self.assertNotIn("scheduler.create", manifest["permissions"]["required"])
        self.assertNotIn("secret.read", manifest["permissions"]["required"])
        self.assertEqual("/立即检查订阅", manifest["commands"][7]["usage"])
        help_text = json.dumps(manifest["help"], ensure_ascii=False)
        self.assertIn("Web 三方账号页面", help_text)
        self.assertNotIn("轮询", help_text)

    def test_merge_settings_normalizes_current_subscription_shape(self):
        settings = merge_settings({}, {
            "poll_cron": "*/1 * * * *",
            "tokens": [{"id": "legacy", "platform": "bilibili"}],
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

        self.assertNotIn("tokens", settings)
        self.assertNotIn("poll_cron", settings)
        self.assertEqual(settings["subscriptions"][0]["services"], ["video", "live"])
        self.assertEqual(settings["subscriptions"][0]["target_name"], "测试群")
        self.assertEqual(settings["subscriptions"][0]["subscribers"][0]["group_nickname"], "群名片")

    def test_parse_bilibili_command_args_supports_optional_service(self):
        self.assertEqual(parse_bilibili_command_args(["直播", "123456"]), {
            "services": ["live"],
            "uid": "123456",
            "query": "123456",
            "error": False,
        })
        self.assertEqual(parse_bilibili_command_args(["123456"])["services"], ["all"])
        self.assertTrue(parse_bilibili_command_args(["未知", "123456"])["error"])

    def test_subscribe_command_saves_subscription_without_scheduler_or_secret(self):
        settings = merge_settings({}, {"subscriptions": []})
        ctx = FakeContext(
            args=["直播", "123456"],
            target_name="测试群",
            http_responses=[self.user_info_response()],
        )

        result = add_bilibili_subscription(settings, ctx)

        self.assertTrue(result["ok"])
        self.assertIn("已订阅", result["message"])
        self.assertEqual(settings["subscriptions"][0]["services"], ["live"])
        self.assertEqual(settings["subscriptions"][0]["target_name"], "测试群")
        self.assertEqual(ctx.http_requests[0]["headers"].get("Cookie"), None)
        self.assertEqual(ctx.scheduler_creates, [])

    def test_subscribe_command_validates_usage(self):
        settings = merge_settings({}, {})
        ctx = FakeContext(args=[])

        result = add_bilibili_subscription(settings, ctx)

        self.assertFalse(result["ok"])
        self.assertEqual(result["message"], SUBSCRIBE_BILIBILI_USAGE)

    def test_unsubscribe_command_removes_matching_subscription(self):
        settings = merge_settings({}, self.subscription_settings())
        ctx = FakeContext(args=["视频", "123456"])

        result = remove_bilibili_subscription(settings, ctx)

        self.assertTrue(result["ok"])
        self.assertEqual(settings["subscriptions"], [])

    def test_unsubscribe_command_validates_usage(self):
        settings = merge_settings({}, self.subscription_settings())
        ctx = FakeContext(args=[])

        result = remove_bilibili_subscription(settings, ctx)

        self.assertFalse(result["ok"])
        self.assertEqual(result["message"], UNSUBSCRIBE_BILIBILI_USAGE)

    def test_status_text_points_to_platform_source(self):
        text = build_status_text(self.subscription_settings())

        self.assertIn("平台 Bilibili 实时源", text)
        self.assertIn("Web 三方账号页面", text)
        self.assertNotIn("轮询", text)

    def test_subscription_list_formats_targets_and_services(self):
        text = format_subscription_list(self.subscription_settings(), None, platform="bilibili", title="全部 Bilibili 订阅列表")

        self.assertIn("全部 Bilibili 订阅列表", text)
        self.assertIn("群聊 10000", text)
        self.assertIn("测试 UP（UID 123456）", text)
        self.assertIn("视频", text)

    def test_normalize_event_payload_fills_author_and_filters_images(self):
        payload = self.dynamic_event_payload(author={}, images=[{"url": "https://i0.hdslb.com/a.jpg"}, "bad"])

        update = normalize_bilibili_event_payload(payload)

        self.assertEqual(update["author"]["uid"], "123456")
        self.assertEqual(update["author"]["name"], "123456")
        self.assertEqual(update["category"], "视频")
        self.assertEqual(update["images"], [{"url": "https://i0.hdslb.com/a.jpg"}])

    def test_normalize_event_payload_keeps_rich_original(self):
        payload = self.dynamic_event_payload(
            service="repost",
            summary_html='<span class="rich-text-topic">#转发#</span>',
            topic={
                "id": 1156147,
                "name": "BML-PLAY! 2026",
                "jump_url": "https://m.bilibili.com/topic-detail?topic_id=1156147",
            },
            original={
                "id": "80001",
                "service": "image_text",
                "title": "图文动态更新",
                "summary": "#原动态# 正文",
                "summary_html": '<span class="rich-text-topic">#原动态#</span>',
                "url": "https://t.bilibili.com/80001/",
                "author": {"uid": "654321", "name": "原作者"},
                "topic": {
                    "id": 10001,
                    "name": "原动态话题",
                    "jump_url": "https://m.bilibili.com/topic-detail?topic_id=10001",
                },
                "images": [
                    {"url": "https://i0.hdslb.com/original.jpg", "width": 900, "height": 1600},
                    "bad",
                ],
            },
        )

        update = normalize_bilibili_event_payload(payload)

        self.assertEqual(update["service"], "repost")
        self.assertIn("rich-text-topic", update["summary_html"])
        self.assertEqual(update["topic"]["name"], "BML-PLAY! 2026")
        self.assertEqual(update["topic"]["id"], 1156147)
        self.assertEqual(update["original"]["category"], "图文")
        self.assertIn("rich-text-topic", update["original"]["summary_html"])
        self.assertEqual(update["original"]["topic"]["name"], "原动态话题")
        self.assertEqual(update["original"]["images"], [{"url": "https://i0.hdslb.com/original.jpg", "width": 900, "height": 1600}])

    def test_template_css_keeps_single_images_contained(self):
        styles_path = os.path.join(PLUGIN_DIR, "templates", "bilibili-update", "styles.css")
        with open(styles_path, "r", encoding="utf-8") as handle:
            styles = handle.read()

        self.assertIn(".media-grid--single .media-item:not(.media-item--wide) img", styles)
        self.assertIn(".repost-media.media-grid--single .media-item:not(.media-item--wide) img", styles)
        self.assertIn(".topic-line", styles)
        self.assertIn('background-image: url("assets/topic.svg")', styles)
        self.assertIn("object-fit: contain", styles)
        self.assertNotIn("max-height: 760px", styles)
        self.assertNotIn("max-height: 460px", styles)
        self.assertTrue(os.path.exists(os.path.join(PLUGIN_DIR, "templates", "bilibili-update", "assets", "topic.svg")))

    def test_event_matching_requires_uid_and_service(self):
        subscription = self.subscription_settings()["subscriptions"][0]

        self.assertTrue(subscription_matches_event(subscription, self.dynamic_event_payload(service="video")))
        self.assertFalse(subscription_matches_event(subscription, self.dynamic_event_payload(service="live")))
        self.assertFalse(subscription_matches_event(subscription, self.dynamic_event_payload(uid="999999")))

    def test_live_started_event_renders_marks_seen_then_sends_image(self):
        settings = merge_settings({}, self.subscription_settings(subscriptions=[{
            **self.subscription_settings()["subscriptions"][0],
            "services": ["live"],
        }]))
        plugin = SubscriptionHubPlugin()
        ctx = FakeContext(config_values=settings, payload={"bilibili": self.live_event_payload()})

        plugin.handle_bilibili_live_started(ctx)

        self.assertEqual(ctx.results[-1], {"handled": True, "sent": 1})
        self.assertEqual(ctx.render_calls[0]["template"], "bilibili-update")
        self.assertEqual(ctx.render_calls[0]["data"]["title"], "真实直播标题")
        self.assertEqual(ctx.storage_sets, [{
            "key": "seen:bilibili-123456-group-10000:live:live:123456:22913442:1700000000",
            "value": True,
        }])
        self.assertEqual(ctx.messages[0]["target_type"], "group")
        self.assertEqual(ctx.messages[0]["target_id"], "10000")
        self.assertEqual(ctx.messages[0]["segments"][0]["data"]["file"], "plugin-test.png")

    def test_duplicate_event_is_skipped(self):
        settings = merge_settings({}, self.subscription_settings(subscriptions=[{
            **self.subscription_settings()["subscriptions"][0],
            "services": ["live"],
        }]))
        plugin = SubscriptionHubPlugin()
        storage = {"seen:bilibili-123456-group-10000:live:live:123456:22913442:1700000000": True}
        ctx = FakeContext(config_values=settings, payload={"bilibili": self.live_event_payload()}, storage=storage)

        plugin.handle_bilibili_live_started(ctx)

        self.assertEqual(ctx.results[-1], {"handled": True, "sent": 0})
        self.assertEqual(ctx.render_calls, [])
        self.assertEqual(ctx.messages, [])

    def test_dynamic_event_respects_service_filter(self):
        settings = merge_settings({}, self.subscription_settings(subscriptions=[{
            **self.subscription_settings()["subscriptions"][0],
            "services": ["live"],
        }]))
        plugin = SubscriptionHubPlugin()
        ctx = FakeContext(config_values=settings, payload={"bilibili": self.dynamic_event_payload(service="video")})

        plugin.handle_bilibili_dynamic_published(ctx)

        self.assertEqual(ctx.results[-1], {"handled": True, "sent": 0})
        self.assertEqual(ctx.render_calls, [])

    def test_disabled_settings_skip_events(self):
        settings = merge_settings({}, self.subscription_settings(enabled=False))
        plugin = SubscriptionHubPlugin()
        ctx = FakeContext(config_values=settings, payload={"bilibili": self.dynamic_event_payload()})

        plugin.handle_bilibili_dynamic_published(ctx)

        self.assertEqual(ctx.results[-1], {"handled": True, "skipped": "disabled"})
        self.assertEqual(ctx.render_calls, [])

    def test_preview_url_parser_supports_video_opus_dynamic_and_live(self):
        self.assertEqual(parse_preview_url("https://www.bilibili.com/video/BV1c5qEBjEtJ")["kind"], "video")
        self.assertEqual(parse_preview_url("https://www.bilibili.com/opus/1194416231669563410")["kind"], "opus")
        self.assertEqual(parse_preview_url("https://t.bilibili.com/1210053730841395205")["kind"], "dynamic")
        self.assertEqual(parse_preview_url("https://live.bilibili.com/22913442")["kind"], "live")

    def test_preview_card_supports_t_bilibili_repost_link(self):
        plugin = SubscriptionHubPlugin()
        ctx = FakeContext(
            args=["https://t.bilibili.com/1210053730841395205"],
            http_responses=[self.dynamic_detail_response()],
        )

        plugin.handle_preview_card(ctx)

        self.assertEqual(ctx.texts, [])
        self.assertEqual(ctx.http_requests[0]["url"], dynamic_detail_url("1210053730841395205"))
        self.assertEqual(ctx.http_requests[0]["headers"]["Referer"], "https://t.bilibili.com/1210053730841395205")
        render_data = ctx.render_calls[0]["data"]
        self.assertEqual(render_data["service"], "转发")
        self.assertEqual(render_data["original"]["title"], "【乐正绫ACE原创】你是我爱这世界的原因")
        self.assertIn("rich-text-topic", render_data["content_html"])
        self.assertIn("rich-text-emoji", render_data["content_html"])
        self.assertIn("rich-text-topic", render_data["original"]["summary_html"])

    def test_preview_card_supports_opus_topic_module(self):
        plugin = SubscriptionHubPlugin()
        ctx = FakeContext(
            args=["https://www.bilibili.com/opus/1212948650493214729"],
            http_responses=[self.opus_detail_response()],
        )

        plugin.handle_preview_card(ctx)

        self.assertEqual(ctx.texts, [])
        self.assertEqual(ctx.http_requests[0]["url"], opus_detail_url("1212948650493214729"))
        render_data = ctx.render_calls[0]["data"]
        self.assertEqual(render_data["topic"]["name"], "BML-PLAY! 2026")
        self.assertEqual(render_data["topic"]["url"], "https://m.bilibili.com/topic-detail?topic_id=1156147")
        self.assertIn("rich-text-topic", render_data["content_html"])
        self.assertEqual(render_data["media_grid_class"], "media-grid--single")

    def test_preview_response_reports_http_status_and_body(self):
        message = preview_response_document({
            "status_code": 412,
            "body_text": '{"code":-412,"message":"请求被拦截"}',
        }, "动态")

        self.assertIn("HTTP 412", message)
        self.assertIn('"code":-412', message)
        self.assertIn("请求被拦截", message)

    def test_preview_response_reports_bilibili_code_and_body(self):
        message = preview_response_document({
            "status_code": 200,
            "body_text": json.dumps({"code": -352, "message": "风控校验失败", "data": None}, ensure_ascii=False),
        }, "动态")

        self.assertIn("Bilibili code -352", message)
        self.assertIn("HTTP 200", message)
        self.assertIn("风控校验失败", message)
        self.assertIn('"code": -352', message)

    def test_preview_response_reports_non_json_body(self):
        message = preview_response_document({
            "status_code": 200,
            "body_text": "<html><title>blocked</title></html>",
        }, "动态")

        self.assertIn("Bilibili 返回内容不是 JSON", message)
        self.assertIn("HTTP 200", message)
        self.assertIn("<html><title>blocked</title></html>", message)
        self.assertNotIn("响应格式不正确", message)

    def test_preview_card_reports_unrecognized_dynamic_detail(self):
        plugin = SubscriptionHubPlugin()
        ctx = FakeContext(
            args=["https://t.bilibili.com/1210053730841395205"],
            http_responses=[{
                "status_code": 200,
                "body_text": json.dumps({"code": 0, "data": {"item": {}}}, ensure_ascii=False),
            }],
        )

        plugin.handle_preview_card(ctx)

        self.assertIn("未识别到可预览的动态内容", ctx.texts[-1])
        self.assertNotIn("响应格式不正确", ctx.texts[-1])

    def test_subscribe_user_lookup_reports_bilibili_code_and_body(self):
        settings = merge_settings({}, {"subscriptions": []})
        ctx = FakeContext(
            args=["123456"],
            http_responses=[{
                "status_code": 200,
                "body_text": json.dumps({"code": -352, "message": "风控校验失败", "data": None}, ensure_ascii=False),
            }],
        )

        result = add_bilibili_subscription(settings, ctx)

        self.assertFalse(result["ok"])
        self.assertIn("Bilibili code -352", result["message"])
        self.assertIn("HTTP 200", result["message"])
        self.assertIn('"code": -352', result["message"])

    def test_dynamic_updates_extract_video(self):
        updates = dynamic_updates({"data": {"items": [self.video_item("987", "新视频", pub_ts=1700000000)]}})

        self.assertEqual(len(updates), 1)
        self.assertEqual(updates[0]["service"], "video")
        self.assertEqual(updates[0]["title"], "新视频")
        self.assertEqual(updates[0]["duration_text"], "03:21")
        self.assertEqual(updates[0]["images"], [{"url": "https://i0.hdslb.com/video.jpg"}])

    def test_dynamic_updates_extract_opus_summary_rich_text(self):
        updates = dynamic_updates({"data": {"items": [{
            "id_str": "1212948650493214729",
            "type": "DYNAMIC_TYPE_DRAW",
            "modules": {
                "module_author": {"mid": "123456", "name": "洛天依", "pub_ts": 1700000000, "pub_time": "今天 12:00"},
                "module_dynamic": {
                    "topic": {"name": "洛天依2026巡演"},
                    "major": {
                        "type": "MAJOR_TYPE_OPUS",
                        "opus": {
                            "summary": {
                                "text": "#BML-PLAY! 2026#\n线下演唱会，天依今年也来啦！[打call]",
                                "rich_text_nodes": [
                                    {"type": "RICH_TEXT_NODE_TYPE_WEB", "text": "#BML-PLAY! 2026#"},
                                    {"type": "RICH_TEXT_NODE_TYPE_TEXT", "text": "\n线下演唱会，天依今年也来啦！"},
                                    {
                                        "type": "RICH_TEXT_NODE_TYPE_TEXT",
                                        "text": "[打call]",
                                        "emoji": {"icon_url": "//i0.hdslb.com/bfs/emote/call.png", "text": "[打call]", "size": 1},
                                    },
                                ],
                            },
                            "pics": [{"url": "//i0.hdslb.com/bfs/new_dyn/single.jpg", "width": 900, "height": 1600}],
                        },
                    },
                },
            },
        }]}})

        self.assertEqual(len(updates), 1)
        update = updates[0]
        self.assertEqual(update["topic"]["name"], "洛天依2026巡演")
        self.assertIn("rich-text-topic", update["summary_html"])
        self.assertIn("#洛天依2026巡演#", update["summary_html"])
        self.assertIn("rich-text-emoji", update["summary_html"])
        self.assertIn("https://i0.hdslb.com/bfs/emote/call.png", update["summary_html"])
        self.assertEqual(update["images"], [{"url": "https://i0.hdslb.com/bfs/new_dyn/single.jpg", "width": 900, "height": 1600}])

    def test_dynamic_updates_extract_repost_original_and_rich_text(self):
        updates = dynamic_updates({"data": {"items": [self.repost_item()]}})

        self.assertEqual(len(updates), 1)
        update = updates[0]
        self.assertEqual(update["service"], "repost")
        self.assertIn("rich-text-topic", update["summary_html"])
        self.assertIn("rich-text-emoji", update["summary_html"])
        self.assertEqual(update["original"]["service"], "video")
        self.assertEqual(update["original"]["title"], "【乐正绫ACE原创】你是我爱这世界的原因")
        self.assertEqual(update["original"]["topic"]["name"], "洛天依")
        self.assertIn("rich-text-topic", update["original"]["summary_html"])

    def test_repost_event_fetches_original_before_render(self):
        settings = merge_settings({}, self.subscription_settings(subscriptions=[{
            **self.subscription_settings()["subscriptions"][0],
            "services": ["repost"],
        }]))
        plugin = SubscriptionHubPlugin()
        ctx = FakeContext(
            config_values=settings,
            payload={"bilibili": self.dynamic_event_payload(
                id="1210053730841395205",
                service="repost",
                title="转发动态",
                summary="你是我爱这世界的原因",
                url="https://t.bilibili.com/1210053730841395205",
                author={"uid": "123456", "name": "乐正绫"},
            )},
            http_responses=[self.dynamic_detail_response()],
        )

        plugin.handle_bilibili_dynamic_published(ctx)

        self.assertEqual(ctx.results[-1], {"handled": True, "sent": 1})
        self.assertEqual(ctx.http_requests[0]["url"], dynamic_detail_url("1210053730841395205"))
        self.assertEqual(ctx.http_requests[0]["headers"]["Referer"], "https://t.bilibili.com/1210053730841395205")
        render_data = ctx.render_calls[0]["data"]
        self.assertEqual(render_data["original"]["title"], "【乐正绫ACE原创】你是我爱这世界的原因")
        self.assertEqual(render_data["original"]["topic"]["name"], "洛天依")
        self.assertIn("rich-text-topic", render_data["content_html"])
        self.assertIn("rich-text-emoji", render_data["content_html"])
        self.assertIn("rich-text-topic", render_data["original"]["summary_html"])

    def test_repost_event_still_pushes_when_original_lookup_fails(self):
        settings = merge_settings({}, self.subscription_settings(subscriptions=[{
            **self.subscription_settings()["subscriptions"][0],
            "services": ["repost"],
        }]))
        plugin = SubscriptionHubPlugin()
        ctx = FakeContext(
            config_values=settings,
            payload={"bilibili": self.dynamic_event_payload(
                id="1210053730841395205",
                service="repost",
                title="转发动态",
                summary="你是我爱这世界的原因",
                url="https://t.bilibili.com/1210053730841395205",
                author={"uid": "123456", "name": "乐正绫"},
            )},
            http_responses=[{
                "status_code": 200,
                "body_text": json.dumps({"code": -352, "message": "风控校验失败"}, ensure_ascii=False),
            }],
        )

        plugin.handle_bilibili_dynamic_published(ctx)

        self.assertEqual(ctx.results[-1], {"handled": True, "sent": 1})
        self.assertEqual(ctx.render_calls[0]["data"]["original"], None)
        self.assertEqual(ctx.messages[0]["segments"][0]["data"]["file"], "plugin-test.png")

    def test_render_data_keeps_subscribers_and_live_fields(self):
        subscription = self.subscription_settings(subscriptions=[{
            **self.subscription_settings()["subscriptions"][0],
            "services": ["live"],
        }])["subscriptions"][0]
        render_data = build_render_data(subscription, self.live_event_payload())

        self.assertEqual(render_data["service"], "直播")
        self.assertEqual(render_data["status_label"], "直播中")
        self.assertEqual(render_data["subscribers"][0]["nickname"], "订阅人")

    def test_render_data_keeps_bilibili_topic(self):
        subscription = self.subscription_settings()["subscriptions"][0]
        render_data = build_render_data(subscription, self.dynamic_event_payload(
            topic={
                "name": "BML-PLAY! 2026",
                "jump_url": "https://m.bilibili.com/topic-detail?topic_id=1156147",
            },
            original={
                "id": "80001",
                "service": "image_text",
                "title": "原动态",
                "summary": "原动态正文",
                "url": "https://t.bilibili.com/80001",
                "author": {"uid": "654321", "name": "原作者"},
                "topic": {"name": "原动态话题"},
            },
        ))

        self.assertEqual(render_data["topic"]["name"], "BML-PLAY! 2026")
        self.assertEqual(render_data["topic"]["label"], "# BML-PLAY! 2026")
        self.assertEqual(render_data["original"]["topic"]["name"], "原动态话题")


if __name__ == "__main__":
    unittest.main()
