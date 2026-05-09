import os
import sys
import unittest
import json
import time


PLUGIN_DIR = os.path.dirname(os.path.dirname(__file__))
sys.path.insert(0, PLUGIN_DIR)

from bilibili import dynamic_updates
from main import (
    SubscriptionHubPlugin,
    add_bilibili_subscription,
    build_status_text,
    format_subscription_list,
    parse_bilibili_command_args,
    remove_bilibili_subscription,
)
from rendering import build_render_data
from settings import merge_settings


class FakeContext:
    def __init__(
        self,
        args=None,
        target_type="group",
        target_id="10000",
        actor=None,
        config_values=None,
        http_responses=None,
        secrets=None,
        storage=None,
        render_result=None,
    ):
        self.args = args or []
        self.target_type = target_type
        self.target_id = target_id
        self.actor = actor or {"id": "42", "nickname": "订阅人"}
        self.config_values = config_values or {}
        self.http_responses = list(http_responses or [])
        self.secrets = secrets or {}
        self.storage = storage or {}
        self.render_result = render_result or {"image_path": "subscription.png"}
        self.config_writes = []
        self.scheduler_creates = []
        self.texts = []
        self.results = []
        self.logs = []
        self.http_requests = []
        self.render_calls = []
        self.messages = []
        self.storage_sets = []
        self.actions = []

    def config_read(self, keys):
        return {"values": {key: self.config_values[key] for key in keys if key in self.config_values}}

    def config_write(self, values):
        self.config_writes.append(values)
        return {"changed_keys": sorted(values.keys())}

    def scheduler_create(self, task_id, cron, payload=None):
        self.scheduler_creates.append({"task_id": task_id, "cron": cron, "payload": payload})
        return {"task_id": task_id}

    def send_text(self, text):
        self.texts.append(text)

    def send_result(self, result):
        self.results.append(result)

    def logger_write(self, level, message, fields):
        self.logs.append({"level": level, "message": message, "fields": fields})
        return {"ok": True}

    def http_request(self, method, url, headers=None, timeout_seconds=30):
        self.http_requests.append({
            "method": method,
            "url": url,
            "headers": headers or {},
            "timeout_seconds": timeout_seconds,
        })
        if self.http_responses:
            return self.http_responses.pop(0)
        return {"status_code": 200, "body_text": json.dumps({"code": 0, "data": {"items": []}})}

    def secret_read(self, secret_key):
        return {"value": self.secrets.get(secret_key, "")}

    def storage_get(self, key):
        if key in self.storage:
            return {"exists": True, "value": self.storage[key]}
        return {"exists": False}

    def storage_set(self, key, value):
        self.actions.append({"kind": "storage_set", "key": key, "value": value})
        self.storage_sets.append({"key": key, "value": value})
        self.storage[key] = value
        return {"ok": True}

    def render_image(self, template, data, theme, output, fallback_text):
        call = {
            "template": template,
            "data": data,
            "theme": theme,
            "output": output,
            "fallback_text": fallback_text,
        }
        self.actions.append({"kind": "render_image", "call": call})
        self.render_calls.append(call)
        return self.render_result

    def send_message(self, segments, target_type=None, target_id=None):
        message = {"segments": segments, "target_type": target_type, "target_id": target_id}
        self.actions.append({"kind": "send_message", "message": message})
        self.messages.append(message)


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
                        "archive": {"title": title, "desc": "视频简介", "cover": "//i0.hdslb.com/video.jpg"},
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
        ], [item.get("name") for item in manifest.get("commands") or []])
        self.assertEqual("super_admin", manifest["commands"][1]["permission"])
        self.assertEqual("super_admin", manifest["commands"][2]["permission"])
        self.assertEqual("super_admin", manifest["commands"][5]["permission"])
        self.assertEqual("super_admin", manifest["commands"][6]["permission"])
        self.assertNotIn("dynamic_commands", manifest)

    def test_merge_settings_normalizes_tokens_and_subscriptions(self):
        settings = merge_settings({}, {
            "tokens": [{
                "id": "Primary",
                "label": "主 Cookie",
                "secret_key": "bili.primary",
            }],
            "subscriptions": [{
                "uid": "123456",
                "name": "测试 UP",
                "target_type": "group",
                "target_id": "10000",
                "services": ["video", "live", "invalid"],
                "subscribers": [{"id": "42", "nickname": "订阅人"}],
            }],
        })
        self.assertEqual(settings["tokens"][0]["id"], "primary")
        self.assertEqual(settings["dynamic_time_range_seconds"], 7200)
        self.assertEqual(settings["subscriptions"][0]["services"], ["video", "live"])
        self.assertEqual(settings["subscriptions"][0]["subscribers"][0]["nickname"], "订阅人")

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
                                "archive": {"title": "新视频", "desc": "视频简介", "cover": "//i0.hdslb.com/cover.jpg"},
                            },
                        },
                    },
                }],
            },
        })
        self.assertEqual(len(updates), 1)
        self.assertEqual(updates[0]["service"], "video")
        self.assertEqual(updates[0]["title"], "新视频")
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
        self.assertEqual(updates[0]["summary"], "个人置顶简介 合作联系：hudie007@vip.qq.com")
        self.assertNotIn("rich_text_nodes", updates[0]["summary"])
        self.assertNotIn("orig_text", updates[0]["summary"])

    def test_render_data_contains_subscriber_info(self):
        data = build_render_data({
            "uid": "123456",
            "name": "测试 UP",
            "subscribers": [{"id": "42", "nickname": "订阅人"}],
        }, {
            "service": "video",
            "title": "新视频",
            "summary": "视频简介",
            "images": [{"url": "https://i0.hdslb.com/cover.jpg"}],
            "pub_ts": 1700000000,
            "original": {
                "service": "image_text",
                "title": "原动态",
                "summary": "原动态正文",
                "images": [{"url": "https://i0.hdslb.com/orig.jpg"}],
                "author": {"name": "原作者"},
            },
            "author": {"name": "测试 UP"},
        })
        self.assertEqual(data["subscriber_text"], "订阅人")
        self.assertEqual(data["service"], "视频")
        self.assertEqual(data["images"], [{"url": "https://i0.hdslb.com/cover.jpg"}])
        self.assertEqual(data["original"]["title"], "原动态")
        self.assertEqual(data["original"]["images"], [{"url": "https://i0.hdslb.com/orig.jpg"}])

    def test_parse_bilibili_command_args_defaults_to_all(self):
        self.assertEqual(parse_bilibili_command_args(["123456"]), {"services": ["all"], "uid": "123456", "error": False})
        self.assertEqual(parse_bilibili_command_args(["图文", "123456"]), {"services": ["image_text"], "uid": "123456", "error": False})
        self.assertEqual(parse_bilibili_command_args(["番剧", "123456"]), {"services": [], "uid": "123456", "error": True})

    def test_add_bilibili_subscription_binds_current_target_and_subscriber(self):
        settings = merge_settings({}, {})
        result = add_bilibili_subscription(settings, FakeContext(args=["图文", "123456"]))
        self.assertTrue(result["ok"])
        self.assertEqual(len(settings["subscriptions"]), 1)
        subscription = settings["subscriptions"][0]
        self.assertEqual(subscription["id"], "bilibili-123456-group-10000")
        self.assertEqual(subscription["services"], ["image_text"])
        self.assertEqual(subscription["subscribers"], [{"id": "42", "nickname": "订阅人"}])

    def test_add_bilibili_subscription_merges_services_and_subscribers(self):
        settings = merge_settings({}, {
            "subscriptions": [{
                "uid": "123456",
                "target_type": "group",
                "target_id": "10000",
                "services": ["video"],
                "subscribers": [{"id": "42", "nickname": "旧昵称"}],
            }],
        })
        result = add_bilibili_subscription(settings, FakeContext(args=["直播", "123456"], actor={"id": "43", "nickname": "新订阅人"}))
        self.assertTrue(result["ok"])
        self.assertEqual(settings["subscriptions"][0]["services"], ["video", "live"])
        self.assertEqual(settings["subscriptions"][0]["subscribers"], [
            {"id": "42", "nickname": "旧昵称"},
            {"id": "43", "nickname": "新订阅人"},
        ])

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

    def test_format_subscription_list_can_filter_current_target_and_platform(self):
        settings = merge_settings({}, {
            "subscriptions": [
                {"uid": "123456", "target_type": "group", "target_id": "10000", "services": ["video"], "subscribers": ["订阅人"]},
                {"uid": "654321", "target_type": "private", "target_id": "42", "services": ["live"]},
            ],
        })
        text = format_subscription_list(settings, {"target_type": "group", "target_id": "10000"}, platform="bilibili", title="Bilibili 订阅列表")
        self.assertIn("Bilibili 123456", text)
        self.assertNotIn("654321", text)

    def test_subscribe_command_registers_scheduler_after_saving(self):
        plugin = SubscriptionHubPlugin()
        plugin._settings_loaded = True
        plugin._settings = merge_settings(plugin._default_settings, {"poll_cron": "*/7 * * * *"})
        ctx = FakeContext(args=["视频", "123456"])

        plugin.handle_subscribe_bilibili(ctx)

        self.assertEqual(len(ctx.config_writes), 1)
        self.assertEqual(ctx.scheduler_creates, [{
            "task_id": "subscription-hub-poll",
            "cron": "*/7 * * * *",
            "payload": {"kind": "subscription_poll"},
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
        self.assertIn("Bilibili 123456", list_ctx.texts[0])

    def test_bilibili_http_failure_does_not_push_or_mark_seen(self):
        plugin = SubscriptionHubPlugin()
        settings = self.subscription_settings(tokens=[{
            "id": "primary",
            "label": "主 Cookie",
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

    def test_bilibili_json_error_does_not_push_or_mark_seen(self):
        plugin = SubscriptionHubPlugin()
        settings = self.subscription_settings(tokens=[{
            "id": "primary",
            "label": "主 Cookie",
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

    def test_first_successful_dynamic_poll_sets_baseline_without_push(self):
        plugin = SubscriptionHubPlugin()
        settings = self.subscription_settings(tokens=[{
            "id": "primary",
            "label": "主 Cookie",
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

    def test_build_status_text_summarizes_visible_state(self):
        settings = merge_settings({}, {
            "enabled": True,
            "poll_cron": "*/10 * * * *",
            "dynamic_time_range_seconds": 1800,
            "tokens": [
                {"id": "primary", "label": "主 Cookie", "secret_key": "bili.primary", "enabled": True},
                {"id": "backup", "label": "备用 Cookie", "secret_key": "bili.backup", "enabled": False},
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
