import json
import os
import sys
import time
import unittest


PLUGIN_DIR = os.path.dirname(os.path.dirname(__file__))
BUILTIN_DIR = os.path.dirname(PLUGIN_DIR)
sys.path.insert(0, BUILTIN_DIR)
sys.path.insert(0, PLUGIN_DIR)

from bilibili import dynamic_updates, parse_preview_url
from main import (
    SUBSCRIBE_BILIBILI_USAGE,
    SubscriptionHubPlugin,
    UNSUBSCRIBE_BILIBILI_USAGE,
    add_bilibili_subscription,
    build_status_text,
    format_subscription_list,
    normalize_bilibili_event_payload,
    parse_bilibili_command_args,
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

    def test_preview_url_parser_supports_video_opus_and_live(self):
        self.assertEqual(parse_preview_url("https://www.bilibili.com/video/BV1c5qEBjEtJ")["kind"], "video")
        self.assertEqual(parse_preview_url("https://www.bilibili.com/opus/1194416231669563410")["kind"], "opus")
        self.assertEqual(parse_preview_url("https://live.bilibili.com/22913442")["kind"], "live")

    def test_dynamic_updates_extract_video(self):
        updates = dynamic_updates({"data": {"items": [self.video_item("987", "新视频", pub_ts=1700000000)]}})

        self.assertEqual(len(updates), 1)
        self.assertEqual(updates[0]["service"], "video")
        self.assertEqual(updates[0]["title"], "新视频")
        self.assertEqual(updates[0]["duration_text"], "03:21")
        self.assertEqual(updates[0]["images"], [{"url": "https://i0.hdslb.com/video.jpg"}])

    def test_render_data_keeps_subscribers_and_live_fields(self):
        subscription = self.subscription_settings(subscriptions=[{
            **self.subscription_settings()["subscriptions"][0],
            "services": ["live"],
        }])["subscriptions"][0]
        render_data = build_render_data(subscription, self.live_event_payload())

        self.assertEqual(render_data["service"], "直播")
        self.assertEqual(render_data["status_label"], "直播中")
        self.assertEqual(render_data["subscribers"][0]["nickname"], "订阅人")


if __name__ == "__main__":
    unittest.main()
