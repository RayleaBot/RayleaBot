import json
import os
import sys
import time
import unittest
from urllib.parse import parse_qs, urlparse


PLUGIN_DIR = os.path.dirname(os.path.dirname(__file__))
TEST_DIR = os.path.dirname(__file__)
sys.path.insert(0, TEST_DIR)
sys.path.insert(0, PLUGIN_DIR)

from platforms.bilibili import dynamic_detail_url, dynamic_updates, opus_detail_url, parse_preview_url, user_search_url
from main import (
    BILIBILI_SEARCH_UP_USAGE,
    SCHEDULER_TASK_ID,
    SUBSCRIBE_BILIBILI_USAGE,
    SubscriptionHubPlugin,
    UNSUBSCRIBE_BILIBILI_USAGE,
    add_bilibili_subscription,
    add_platform_subscription,
    build_status_text,
    format_subscription_list,
    parse_bilibili_command_args,
    preview_response_document,
    remove_bilibili_subscription,
    remove_platform_subscription,
)
from platforms.bilibili.source import (
    DM_COVER_IMG_STR,
    DM_IMG_STR,
    DYNAMIC_FEED_URL,
    FOLLOW_URL,
    LIVE_STATUS_URL,
    RELATION_URL,
    dynamic_feed_url,
    sign_wbi_url,
)
from features.rendering import build_render_data
from business.events import normalize_bilibili_event_payload, subscription_matches_event
from business.settings import merge_settings
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

    def video_item(self, dynamic_id, title, pub_ts=None, uid="123456"):
        pub_ts = int(pub_ts or time.time())
        return {
            "id_str": dynamic_id,
            "type": "DYNAMIC_TYPE_AV",
            "basic": {"jump_url": f"//www.bilibili.com/video/{dynamic_id}"},
            "modules": {
                "module_author": {"mid": uid, "name": "测试 UP", "pub_ts": pub_ts, "pub_time": "今天 12:00"},
                "module_dynamic": {
                    "major": {
                        "type": "MAJOR_TYPE_ARCHIVE",
                        "archive": {"title": title, "desc": "视频简介", "cover": "//i0.hdslb.com/video.jpg", "duration_text": "03:21"},
                    },
                },
            },
        }

    def repost_item(self, dynamic_id="100000000000000001", pub_ts=1780585560):
        return {
            "id_str": dynamic_id,
            "type": "DYNAMIC_TYPE_FORWARD",
            "basic": {"jump_url": f"//t.bilibili.com/{dynamic_id}"},
            "modules": {
                "module_author": {
                    "mid": "123456",
                    "name": "测试转发用户",
                    "face": "//i0.hdslb.com/bfs/face/repost.jpg",
                    "pub_ts": pub_ts,
                    "pub_time": "2026年06月04日 20:26",
                },
                "module_dynamic": {
                    "desc": {
                        "text": "测试原创作品标题[星星眼] #测试转发话题#",
                        "rich_text_nodes": [
                            {"type": "RICH_TEXT_NODE_TYPE_TEXT", "text": "测试原创作品标题"},
                            {
                                "type": "RICH_TEXT_NODE_TYPE_EMOJI",
                                "text": "[星星眼]",
                                "emoji": {"icon_url": "//i0.hdslb.com/bfs/emote/star.png", "text": "[星星眼]", "size": 1},
                            },
                            {"type": "RICH_TEXT_NODE_TYPE_WEB", "text": "#测试转发话题#"},
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
                        "name": "测试原创作者",
                        "face": "//i0.hdslb.com/bfs/face/original.jpg",
                        "pub_ts": 1780585200,
                        "pub_time": "2026年06月04日 20:20",
                    },
                    "module_dynamic": {
                        "topic": {
                            "id": 10001,
                            "name": "测试 UP",
                            "jump_url": "https://m.bilibili.com/topic-detail?topic_id=10001",
                        },
                        "desc": {
                            "text": "新歌来了！",
                            "rich_text_nodes": [
                                {"type": "", "orig_text": "#测试 UP#"},
                                {"type": "RICH_TEXT_NODE_TYPE_TEXT", "text": " 新歌来了！"},
                            ],
                        },
                        "major": {
                            "type": "MAJOR_TYPE_ARCHIVE",
                            "archive": {
                                "title": "测试原创作品标题",
                                "desc": "原创作品简介。",
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
                        "id_str": "100000000000000002",
                        "type": "DYNAMIC_TYPE_DRAW",
                        "basic": {"title": "测试 UP 发布图文动态"},
                        "modules": [
                            {
                                "module_type": "MODULE_TYPE_AUTHOR",
                                "module_author": {
                                    "mid": "1000001",
                                    "name": "测试 UP",
                                    "face": "//i0.hdslb.com/bfs/face/test-up.jpg",
                                    "pub_ts": 1781250000,
                                    "pub_time": "2026年06月12日 15:40",
                                },
                            },
                            {
                                "module_type": "MODULE_TYPE_TOPIC",
                                "module_topic": {
                                    "id": 1156147,
                                    "name": "测试活动 2026",
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
                                                            "text": "#测试活动 2026#",
                                                        },
                                                    },
                                                    {
                                                        "type": "TEXT_NODE_TYPE_WORD",
                                                        "word": {"words": "线下演唱会，测试内容更新。"},
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
            "id": "live:123456:10001:1700000000",
            "room_id": "10001",
            "service": "live",
            "title": "测试直播标题",
            "summary": "直播中",
            "url": "https://live.bilibili.com/10001",
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
            "id": "dynamic:100000000000000003",
            "service": "video",
            "title": "测试视频标题",
            "summary": "视频简介",
            "url": "https://www.bilibili.com/video/BV1RayleaBot",
            "pub_ts": 1700000100,
            "created_at": "2026-06-08 12:05",
            "author": {"uid": "123456", "name": "测试 UP"},
            "images": [{"url": "https://i0.hdslb.com/video-cover.jpg"}],
        }
        payload.update(overrides)
        return payload

    def bilibili_cookie_accounts(self):
        return {
            "bilibili": [{
                "account_id": "primary",
                "cookie": {"secret": True, "value": "SESSDATA=fixture; bili_jct=fixture;"},
            }],
        }

    def dynamic_feed_response(self, items=None):
        return {
            "status_code": 200,
            "body_text": json.dumps({"code": 0, "data": {"items": items or []}}, ensure_ascii=False),
        }

    def source_storage(self, *, initialized=True, live_state=None, account_ids=("primary",), uids=("123456",)):
        now = int(time.time())
        storage = {}
        for account_id in account_ids:
            storage[f"source:bilibili:wbi:{account_id}"] = {
                "img_key": "a" * 32,
                "sub_key": "b" * 32,
                "expires_at": now + 3600,
            }
            for uid in uids:
                storage[f"source:bilibili:follow:{account_id}:{uid}"] = {
                    "checked_at": now,
                    "following": True,
                }
        if initialized:
            storage["source:bilibili:dynamic:initialized:bilibili-123456-group-10000"] = True
        if live_state is not None:
            storage["source:bilibili:live:123456"] = live_state
        return storage

    def relation_response(self, attribute=0):
        return {
            "status_code": 200,
            "body_text": json.dumps({"code": 0, "data": {"attribute": attribute}}, ensure_ascii=False),
        }

    def ok_response(self):
        return {"status_code": 200, "body_text": '{"code":0,"data":{}}'}

    def nav_response(self):
        return {
            "status_code": 200,
            "body_text": json.dumps({
                "code": 0,
                "data": {
                    "wbi_img": {
                        "img_url": "https://i0.hdslb.com/bfs/wbi/7cd084941338484aae1ad9425b84077c.png",
                        "sub_url": "https://i0.hdslb.com/bfs/wbi/4932caff0ff746eab6f01bf08b70ac45.png",
                    },
                },
            }),
        }

    def live_status_response(self, **overrides):
        entry = {
            "room_id": 10001,
            "live_status": 1,
            "title": "测试直播标题",
            "uname": "测试 UP",
            "face": "//i0.hdslb.com/face.jpg",
            "cover_from_user": "//i0.hdslb.com/live-cover.jpg",
            "liveTime": 1700000000,
        }
        entry.update(overrides)
        return {
            "status_code": 200,
            "body_text": json.dumps({"code": 0, "data": {"123456": entry}}, ensure_ascii=False),
        }

    def empty_live_status_response(self):
        return {
            "status_code": 200,
            "body_text": json.dumps({"code": 0, "data": {}}, ensure_ascii=False),
        }

    def test_manifest_declares_event_consumer_capabilities(self):
        with open(os.path.join(PLUGIN_DIR, "info.json"), "r", encoding="utf-8") as handle:
            manifest = json.load(handle)

        self.assertEqual([{"id": "subscriptions", "label": "订阅设置", "entry": "web/index.html"}], manifest["management_ui"]["pages"])
        self.assertIn("event.subscribe", manifest["capabilities"])
        self.assertIn("http.request", manifest["capabilities"])
        self.assertEqual([
            "api.bilibili.com",
            "api.live.bilibili.com",
        ], manifest["capability_parameters"]["http_hosts"])
        self.assertIn("scheduler.create", manifest["capabilities"])
        self.assertIn("thirdparty.account.read", manifest["capabilities"])
        self.assertNotIn("secret.read", manifest["capabilities"])
        self.assertEqual([
            "bilibili",
            "weibo",
            "douyin",
            "netease_music",
        ], manifest["capability_parameters"]["third_party_account_platforms"])
        self.assertNotIn("permissions", manifest)
        usages = [item["usage"] for item in manifest["commands"]]
        self.assertIn("/b站搜索up UP昵称关键词", usages)
        self.assertIn("/订阅微博推送 [微博|图片|视频|转发] UID或主页标识", usages)
        self.assertIn("/订阅抖音推送 [视频|图文|直播] 抖音号或主页标识", usages)
        self.assertIn("/订阅网易云音乐推送 [歌曲|专辑|歌单|音乐人] ID或主页标识", usages)
        self.assertIn("/立即检查订阅", usages)
        help_text = json.dumps(manifest["help"], ensure_ascii=False)
        self.assertIn("搜索 Bilibili UP", help_text)
        self.assertIn("订阅微博", help_text)
        self.assertIn("订阅抖音", help_text)
        self.assertIn("订阅网易云音乐", help_text)
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

    def test_merge_settings_accepts_supported_platform_subscriptions(self):
        settings = merge_settings({}, {
            "subscriptions": [
                {
                    "platform": "weibo",
                    "uid": "7556659984",
                    "name": "测试微博",
                    "target_type": "group",
                    "target_id": "10000",
                    "services": ["post", "video", "invalid"],
                },
                {
                    "platform": "douyin",
                    "uid": "douyin-test",
                    "name": "测试抖音",
                    "target_type": "group",
                    "target_id": "10000",
                    "services": ["video", "live"],
                },
                {
                    "platform": "netease_music",
                    "uid": "12345",
                    "name": "测试歌单",
                    "target_type": "private",
                    "target_id": "42",
                    "services": ["song", "playlist"],
                },
            ],
        })

        self.assertEqual([item["platform"] for item in settings["subscriptions"]], ["weibo", "douyin", "netease_music"])
        self.assertEqual(settings["subscriptions"][0]["services"], ["post", "video"])
        self.assertEqual(settings["subscriptions"][1]["services"], ["video", "live"])
        self.assertEqual(settings["subscriptions"][2]["services"], ["song", "playlist"])

    def test_parse_bilibili_command_args_supports_optional_service(self):
        self.assertEqual(parse_bilibili_command_args(["直播", "123456"]), {
            "services": ["live"],
            "uid": "123456",
            "query": "123456",
            "error": False,
        })
        self.assertEqual(parse_bilibili_command_args(["123456"])["services"], ["all"])
        self.assertTrue(parse_bilibili_command_args(["未知", "123456"])["error"])

    def test_subscribe_command_reads_ck_and_sends_cookie(self):
        settings = merge_settings({}, {"subscriptions": []})
        ctx = FakeContext(
            args=["直播", "123456"],
            target_name="测试群",
            thirdparty_accounts=self.bilibili_cookie_accounts(),
            http_responses=[self.user_info_response()],
        )

        result = add_bilibili_subscription(settings, ctx)

        self.assertTrue(result["ok"])
        self.assertIn("已订阅", result["message"])
        self.assertEqual(settings["subscriptions"][0]["services"], ["live"])
        self.assertEqual(settings["subscriptions"][0]["target_name"], "测试群")
        self.assertEqual(ctx.thirdparty_reads[0]["platform"], "bilibili")
        self.assertIn("SESSDATA=fixture", ctx.http_requests[0]["headers"].get("Cookie", ""))

    def test_subscribe_command_reads_ck_for_name_search(self):
        settings = merge_settings({}, {"subscriptions": []})
        ctx = FakeContext(
            args=["测试 UP"],
            target_name="测试群",
            thirdparty_accounts=self.bilibili_cookie_accounts(),
            http_responses=[self.user_search_response()],
        )

        result = add_bilibili_subscription(settings, ctx)

        self.assertTrue(result["ok"])
        self.assertEqual(ctx.http_requests[0]["url"], user_search_url("测试 UP"))
        self.assertEqual(ctx.thirdparty_reads[0]["platform"], "bilibili")
        self.assertIn("SESSDATA=fixture", ctx.http_requests[0]["headers"].get("Cookie", ""))

    def test_subscribe_command_requires_bilibili_ck(self):
        settings = merge_settings({}, {"subscriptions": []})
        ctx = FakeContext(args=["直播", "123456"])

        result = add_bilibili_subscription(settings, ctx)

        self.assertFalse(result["ok"])
        self.assertIn("没有可用的 Bilibili 账号 CK", result["message"])
        self.assertEqual(ctx.http_requests, [])
        self.assertEqual(ctx.thirdparty_reads[0]["platform"], "bilibili")

    def test_subscribe_command_validates_usage(self):
        settings = merge_settings({}, {})
        ctx = FakeContext(args=[])

        result = add_bilibili_subscription(settings, ctx)

        self.assertFalse(result["ok"])
        self.assertEqual(result["message"], SUBSCRIBE_BILIBILI_USAGE)

    def test_subscribe_platform_command_saves_weibo_subscription(self):
        settings = merge_settings({}, {"subscriptions": []})
        ctx = FakeContext(args=["视频", "7556659984"], target_name="测试群")

        result = add_platform_subscription(settings, ctx, "weibo")

        self.assertTrue(result["ok"])
        self.assertIn("已订阅 微博", result["message"])
        self.assertEqual(settings["subscriptions"][0]["platform"], "weibo")
        self.assertEqual(settings["subscriptions"][0]["uid"], "7556659984")
        self.assertEqual(settings["subscriptions"][0]["services"], ["video"])
        self.assertEqual(settings["subscriptions"][0]["target_name"], "测试群")

    def test_unsubscribe_platform_command_removes_matching_subscription(self):
        settings = merge_settings({}, {
            "subscriptions": [{
                "id": "douyin-douyin-test-group-10000",
                "platform": "douyin",
                "uid": "douyin-test",
                "name": "测试抖音",
                "target_type": "group",
                "target_id": "10000",
                "services": ["video"],
                "subscribers": [],
                "enabled": True,
            }],
        })
        ctx = FakeContext(args=["视频", "douyin-test"])

        result = remove_platform_subscription(settings, ctx, "douyin")

        self.assertTrue(result["ok"])
        self.assertEqual(settings["subscriptions"], [])

    def test_load_settings_registers_scheduler(self):
        plugin = SubscriptionHubPlugin()
        ctx = FakeContext(config_values=self.subscription_settings())

        plugin.load_settings(ctx)
        plugin.load_settings(ctx)

        self.assertEqual(len(ctx.scheduler_creates), 1)
        self.assertEqual(ctx.scheduler_creates[0]["task_id"], SCHEDULER_TASK_ID)
        self.assertEqual(ctx.scheduler_creates[0]["payload"], {"action": "check_subscriptions"})

    def test_plugin_started_registers_scheduler_and_logs_info(self):
        plugin = SubscriptionHubPlugin()
        ctx = FakeContext()

        plugin.handle_plugin_started(ctx)

        self.assertEqual(len(ctx.scheduler_creates), 1)
        self.assertEqual(ctx.scheduler_creates[0]["task_id"], SCHEDULER_TASK_ID)
        self.assertEqual(ctx.scheduler_creates[0]["cron"], plugin.SCHEDULER_CRON)
        self.assertEqual(ctx.scheduler_creates[0]["log_label"], "订阅检查")
        self.assertEqual(ctx.results[-1], {"handled": True, "scheduler_registered": True})
        self.assertIn({
            "level": "info",
            "message": f"订阅中心插件创建定时任务订阅检查（{plugin.SCHEDULER_CRON}）",
            "fields": {
                "task_id": SCHEDULER_TASK_ID,
                "cron": plugin.SCHEDULER_CRON,
                "log_label": "订阅检查",
            },
        }, ctx.logs)

    def test_check_subscriptions_reports_missing_ck(self):
        plugin = SubscriptionHubPlugin()
        ctx = FakeContext()

        result = plugin.check_subscriptions(ctx, self.subscription_settings())

        self.assertEqual(result["checked"], 1)
        self.assertEqual(result["sent"], 0)
        self.assertTrue(result["degraded"])
        self.assertEqual(ctx.http_requests, [])
        self.assertIn("没有可用的 Bilibili 账号 CK", result["errors"][0])

    def test_check_subscriptions_reads_ck_and_checks_bilibili(self):
        plugin = SubscriptionHubPlugin()
        cookie = "SESSDATA=fixture; bili_jct=fixture;"
        ctx = FakeContext(
            thirdparty_accounts={
                "bilibili": [{
                    "account_id": "primary",
                    "cookie": {"secret": True, "value": cookie},
                }],
            },
            http_responses=[self.dynamic_feed_response()],
            storage=self.source_storage(),
        )

        result = plugin.check_subscriptions(ctx, self.subscription_settings())

        self.assertEqual(result["checked"], 1)
        self.assertEqual(result["sent"], 0)
        self.assertEqual(result["errors"], [])
        self.assertTrue(result["source"]["dynamic_ok"])
        self.assertEqual(ctx.thirdparty_reads[0]["platform"], "bilibili")
        self.assertEqual(len(ctx.http_requests), 1)
        self.assertTrue(ctx.http_requests[0]["url"].startswith(DYNAMIC_FEED_URL + "?"))
        self.assertNotIn("/feed/space", ctx.http_requests[0]["url"])
        self.assertIn("w_rid=", ctx.http_requests[0]["url"])
        self.assertIn("SESSDATA=fixture", ctx.http_requests[0]["headers"].get("Cookie", ""))

    def test_dynamic_source_baselines_existing_items_then_pushes_new_items(self):
        settings = merge_settings({}, self.subscription_settings())
        storage = self.source_storage(initialized=False)
        plugin = SubscriptionHubPlugin()
        first_ctx = FakeContext(
            thirdparty_accounts=self.bilibili_cookie_accounts(),
            http_responses=[self.dynamic_feed_response([self.video_item("old", "已有视频", pub_ts=1700000000)])],
            storage=storage,
        )

        first_result = plugin.check_subscriptions(first_ctx, settings)

        self.assertEqual(first_result["sent"], 0)
        self.assertTrue(storage["source:bilibili:dynamic:initialized:bilibili-123456-group-10000"])
        self.assertTrue(storage["seen:bilibili-123456-group-10000:video:old"])

        second_ctx = FakeContext(
            thirdparty_accounts=self.bilibili_cookie_accounts(),
            http_responses=[self.dynamic_feed_response([
                self.video_item("new", "新视频", pub_ts=1700000060),
                self.video_item("old", "已有视频", pub_ts=1700000000),
            ])],
            storage=storage,
        )

        second_result = plugin.check_subscriptions(second_ctx, settings)

        self.assertEqual(second_result["sent"], 1)
        self.assertEqual(second_ctx.render_calls[0]["data"]["title"], "新视频")
        self.assertTrue(storage["seen:bilibili-123456-group-10000:video:new"])

    def test_wbi_signing_matches_previous_source_vector(self):
        signed = sign_wbi_url(
            "https://api.bilibili.com/x/polymer/web-dynamic/v1/feed/all?type=all&page=1",
            "7cd084941338484aae1ad9425b84077c",
            "4932caff0ff746eab6f01bf08b70ac45",
            1780905600,
        )

        self.assertIn("wts=1780905600", signed)
        self.assertIn("w_rid=389c1304f65697bbde60fdd8f8a6f9b6", signed)

    def test_dynamic_feed_uses_single_encoded_device_fingerprint(self):
        query = parse_qs(urlparse(dynamic_feed_url()).query)

        self.assertEqual(query["dm_img_str"], [DM_IMG_STR])
        self.assertEqual(query["dm_cover_img_str"], [DM_COVER_IMG_STR])
        self.assertIn("itemOpusStyle", query["features"][0].split(","))
        self.assertTrue(DM_IMG_STR.startswith("V2ViR0wg"))
        self.assertFalse(DM_IMG_STR.startswith("VjJWaVIw"))

    def test_dynamic_source_auto_follows_subject_before_reading_account_feed(self):
        storage = self.source_storage()
        storage.pop("source:bilibili:follow:primary:123456")
        ctx = FakeContext(
            thirdparty_accounts=self.bilibili_cookie_accounts(),
            http_responses=[self.relation_response(attribute=0), self.ok_response(), self.dynamic_feed_response()],
            storage=storage,
        )

        result = SubscriptionHubPlugin().check_subscriptions(ctx, self.subscription_settings())

        self.assertEqual(result["errors"], [])
        self.assertTrue(ctx.http_requests[0]["url"].startswith(RELATION_URL + "?"))
        self.assertEqual(ctx.http_requests[1]["method"], "POST")
        self.assertTrue(ctx.http_requests[1]["url"].startswith(FOLLOW_URL + "?"))
        self.assertIn("fid=123456", ctx.http_requests[1]["body_text"])
        self.assertIn("csrf=fixture", ctx.http_requests[1]["body_text"])
        self.assertTrue(ctx.http_requests[2]["url"].startswith(DYNAMIC_FEED_URL + "?"))

    def test_dynamic_source_rotates_accounts_after_risk_control(self):
        accounts = {
            "bilibili": [
                {"account_id": "primary", "cookie": {"secret": True, "value": "SESSDATA=primary; bili_jct=one;"}},
                {"account_id": "backup", "cookie": {"secret": True, "value": "SESSDATA=backup; bili_jct=two;"}},
            ],
        }
        storage = self.source_storage(account_ids=("primary", "backup"))
        ctx = FakeContext(
            thirdparty_accounts=accounts,
            http_responses=[
                {"status_code": 412, "body_text": '{"code":-412,"message":"request was banned"}'},
                self.dynamic_feed_response(),
            ],
            storage=storage,
        )

        result = SubscriptionHubPlugin().check_subscriptions(ctx, self.subscription_settings())

        self.assertTrue(result["source"]["dynamic_ok"])
        self.assertTrue(result["degraded"])
        self.assertIn("风控", result["errors"][0])
        self.assertIn("SESSDATA=primary", ctx.http_requests[0]["headers"]["Cookie"])
        self.assertIn("SESSDATA=backup", ctx.http_requests[1]["headers"]["Cookie"])
        self.assertIn("source:bilibili:cooldown:dynamic:primary", storage)
        self.assertTrue(any(log["message"] == "Bilibili 订阅源检查失败" for log in ctx.logs))

    def test_dynamic_source_classifies_non_json_http_risk_control_with_diagnostics(self):
        accounts = {
            "bilibili": [
                {"account_id": "primary", "cookie": {"secret": True, "value": "SESSDATA=primary; bili_jct=one;"}},
                {"account_id": "backup", "cookie": {"secret": True, "value": "SESSDATA=backup; bili_jct=two;"}},
            ],
        }
        storage = self.source_storage(account_ids=("primary", "backup"))
        ctx = FakeContext(
            thirdparty_accounts=accounts,
            http_responses=[
                {
                    "status_code": 412,
                    "body_text": "<html>request blocked; SESSDATA=should-not-leak;</html>",
                    "headers": {"x-bili-trace-id": "trace-fixture"},
                },
                self.dynamic_feed_response(),
            ],
            storage=storage,
        )

        result = SubscriptionHubPlugin().check_subscriptions(ctx, self.subscription_settings())

        self.assertTrue(result["source"]["dynamic_ok"])
        self.assertTrue(result["degraded"])
        self.assertIn("source:bilibili:cooldown:dynamic:primary", storage)
        error_fields = next(log["fields"] for log in ctx.logs if log["fields"].get("scope") == "dynamic")
        self.assertEqual(error_fields["kind"], "risk_control")
        self.assertIn("HTTP 412", error_fields["error"])
        self.assertIn("trace-fixture", error_fields["error"])
        self.assertNotIn("should-not-leak", error_fields["error"])

    def test_dynamic_source_refreshes_wbi_keys_after_signature_rejection(self):
        storage = self.source_storage()
        ctx = FakeContext(
            thirdparty_accounts=self.bilibili_cookie_accounts(),
            http_responses=[
                {"status_code": 200, "body_text": '{"code":-403,"message":"signature expired"}'},
                self.nav_response(),
                self.dynamic_feed_response(),
            ],
            storage=storage,
        )

        result = SubscriptionHubPlugin().check_subscriptions(ctx, self.subscription_settings())

        self.assertTrue(result["source"]["dynamic_ok"])
        self.assertEqual(result["errors"], [])
        self.assertTrue(ctx.http_requests[0]["url"].startswith(DYNAMIC_FEED_URL + "?"))
        self.assertIn("/x/web-interface/nav", ctx.http_requests[1]["url"])
        self.assertTrue(ctx.http_requests[2]["url"].startswith(DYNAMIC_FEED_URL + "?"))
        self.assertTrue(any(action["kind"] == "storage_delete" for action in ctx.actions))

    def test_account_feed_fans_out_multiple_subjects_from_one_request(self):
        base = self.subscription_settings()["subscriptions"][0]
        second = {
            **base,
            "id": "bilibili-654321-private-20000",
            "uid": "654321",
            "name": "第二个 UP",
            "target_type": "private",
            "target_id": "20000",
        }
        settings = merge_settings({}, self.subscription_settings(subscriptions=[base, second]))
        storage = self.source_storage(uids=("123456", "654321"))
        storage["source:bilibili:dynamic:initialized:bilibili-654321-private-20000"] = True
        ctx = FakeContext(
            thirdparty_accounts=self.bilibili_cookie_accounts(),
            http_responses=[self.dynamic_feed_response([
                self.video_item("one", "第一个更新", pub_ts=1700000100, uid="123456"),
                self.video_item("two", "第二个更新", pub_ts=1700000200, uid="654321"),
            ])],
            storage=storage,
        )

        result = SubscriptionHubPlugin().check_subscriptions(ctx, settings)

        feed_requests = [request for request in ctx.http_requests if request["url"].startswith(DYNAMIC_FEED_URL + "?")]
        self.assertEqual(len(feed_requests), 1)
        self.assertEqual(result["checked"], 2)
        self.assertEqual(result["sent"], 2)
        self.assertEqual(
            [(message["target_type"], message["target_id"]) for message in ctx.messages],
            [("group", "10000"), ("private", "20000")],
        )

    def test_bilibili_user_search_returns_multiple_candidates(self):
        plugin = SubscriptionHubPlugin()
        ctx = FakeContext(
            args=["测试 UP"],
            thirdparty_accounts=self.bilibili_cookie_accounts(),
            http_responses=[self.user_search_response(results=[
                {"mid": "1000001", "uname": "测试 UP", "fans": 1280000, "upic": "//i0.hdslb.com/test-up.jpg"},
                {"mid": "123456", "uname": "测试 UP官方粉丝团", "fans": 2048},
            ])],
        )

        plugin.handle_bilibili_user_search(ctx)

        self.assertEqual(ctx.http_requests[0]["url"], user_search_url("测试 UP"))
        self.assertEqual(ctx.thirdparty_reads[0]["platform"], "bilibili")
        self.assertIn("SESSDATA=fixture", ctx.http_requests[0]["headers"].get("Cookie", ""))
        self.assertEqual(ctx.results[-1], {"handled": True, "count": 2})
        self.assertIn("Bilibili UP 搜索结果：测试 UP", ctx.texts[0])
        self.assertIn("1. 测试 UP（UID 1000001）｜粉丝 128万", ctx.texts[0])
        self.assertIn("2. 测试 UP官方粉丝团（UID 123456）｜粉丝 2048", ctx.texts[0])

    def test_bilibili_user_search_validates_usage(self):
        plugin = SubscriptionHubPlugin()
        ctx = FakeContext(args=[])

        plugin.handle_bilibili_user_search(ctx)

        self.assertEqual(ctx.texts, [BILIBILI_SEARCH_UP_USAGE])
        self.assertEqual(ctx.results[-1], {"handled": True, "count": 0})

    def test_bilibili_user_search_not_found_omits_raw_response(self):
        plugin = SubscriptionHubPlugin()
        ctx = FakeContext(
            args=["test-user"],
            thirdparty_accounts=self.bilibili_cookie_accounts(),
            http_responses=[self.user_search_response(results=[])],
        )

        plugin.handle_bilibili_user_search(ctx)

        self.assertEqual(ctx.texts, ["没有搜索到 Bilibili 用户：test-user"])
        self.assertNotIn("HTTP 200", ctx.texts[0])
        self.assertNotIn('"code"', ctx.texts[0])
        self.assertEqual(ctx.results[-1], {"handled": True, "count": 0})

    def test_bilibili_user_search_requires_bilibili_ck(self):
        plugin = SubscriptionHubPlugin()
        ctx = FakeContext(args=["测试 UP"])

        plugin.handle_bilibili_user_search(ctx)

        self.assertIn("没有可用的 Bilibili 账号 CK", ctx.texts[0])
        self.assertEqual(ctx.http_requests, [])
        self.assertEqual(ctx.thirdparty_reads[0]["platform"], "bilibili")
        self.assertEqual(ctx.results[-1], {"handled": True, "count": 0})

    def test_bilibili_user_search_reports_risk_control(self):
        plugin = SubscriptionHubPlugin()
        ctx = FakeContext(
            args=["测试 UP"],
            thirdparty_accounts=self.bilibili_cookie_accounts(),
            http_responses=[{
                "status_code": 412,
                "body_text": '{"code":-412,"message":"request was banned"}',
                "headers": {"X-Bili-Trace-Id": "trace-search-412"},
            }],
        )

        plugin.handle_bilibili_user_search(ctx)

        self.assertIn("Bilibili 请求被风控拦截，请稍后再试或重新扫码更新 CK", ctx.texts[0])
        self.assertIn("HTTP 412", ctx.texts[0])
        self.assertIn("Bilibili code -412", ctx.texts[0])
        self.assertIn("原始原因：request was banned", ctx.texts[0])
        self.assertIn("请求标识：trace-search-412", ctx.texts[0])
        self.assertNotIn('"code"', ctx.texts[0])
        self.assertEqual(ctx.results[-1], {"handled": True, "count": 0})

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

        self.assertIn("Bilibili、微博、抖音、网易云音乐", text)
        self.assertIn("Web 三方账号页面", text)
        self.assertNotIn("轮询", text)

    def test_subscription_list_formats_targets_and_services(self):
        text = format_subscription_list(self.subscription_settings(), None, platform="bilibili", title="全部 Bilibili 订阅列表")

        self.assertIn("全部 Bilibili 订阅列表", text)
        self.assertIn("群聊 10000", text)
        self.assertIn("测试 UP（UID 123456）", text)
        self.assertIn("视频", text)

    def test_plugin_does_not_subscribe_plain_message_link_events(self):
        plugin = SubscriptionHubPlugin()
        registered_events = {event for event, _ in plugin._event_handlers}

        self.assertNotIn("message.group", registered_events)
        self.assertNotIn("message.private", registered_events)

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
                "name": "测试活动 2026",
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
        self.assertEqual(update["topic"]["name"], "测试活动 2026")
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

    def test_check_subscriptions_renders_live_update_sends_image_then_marks_seen(self):
        settings = merge_settings({}, self.subscription_settings(subscriptions=[{
            **self.subscription_settings()["subscriptions"][0],
            "services": ["live"],
        }]))
        plugin = SubscriptionHubPlugin()
        ctx = FakeContext(
            config_values=settings,
            thirdparty_accounts=self.bilibili_cookie_accounts(),
            http_responses=[self.live_status_response()],
            storage=self.source_storage(live_state={"status": 0, "session": "previous", "room_id": "10001"}),
        )

        result = plugin.check_subscriptions(ctx, settings)

        self.assertEqual(result["checked"], 1)
        self.assertEqual(result["sent"], 1)
        self.assertEqual(result["errors"], [])
        self.assertEqual(ctx.render_calls[0]["template"], "bilibili-update")
        self.assertEqual(ctx.render_calls[0]["data"]["title"], "测试直播标题")
        self.assertTrue(ctx.storage["seen:bilibili-123456-group-10000:live:live-123456-started-1700000000"])
        self.assertEqual(ctx.messages[0]["target_type"], "group")
        self.assertEqual(ctx.messages[0]["target_id"], "10000")
        self.assertEqual(ctx.messages[0]["segments"][0]["data"]["file"], "plugin-test.png")
        action_kinds = [action["kind"] for action in ctx.actions]
        self.assertLess(action_kinds.index("message_send"), next(
            index for index, action in enumerate(ctx.actions)
            if action.get("key", "").startswith("seen:")
        ))

    def test_live_source_establishes_baseline_without_historical_push(self):
        settings = merge_settings({}, self.subscription_settings(subscriptions=[{
            **self.subscription_settings()["subscriptions"][0],
            "services": ["live"],
        }]))
        storage = {}
        ctx = FakeContext(
            thirdparty_accounts=self.bilibili_cookie_accounts(),
            http_responses=[self.live_status_response()],
            storage=storage,
        )

        result = SubscriptionHubPlugin().check_subscriptions(ctx, settings)

        self.assertEqual(result["sent"], 0)
        self.assertEqual(storage["source:bilibili:live:123456"], {
            "status": 1,
            "session": "1700000000",
            "room_id": "10001",
        })
        self.assertEqual(ctx.messages, [])

    def test_scheduler_logs_source_failures_instead_of_silent_success(self):
        settings = merge_settings({}, self.subscription_settings())
        ctx = FakeContext(
            payload={},
            config_values=settings,
        )

        SubscriptionHubPlugin().handle_scheduler_trigger(ctx)

        self.assertTrue(ctx.results[-1]["degraded"])
        self.assertTrue(any(log["message"] == "Bilibili 订阅源检查降级" for log in ctx.logs))

    def test_scheduler_accepts_contract_nested_payload(self):
        settings = merge_settings({}, self.subscription_settings(enabled=False))
        ctx = FakeContext(
            payload={"payload": {"action": "check_subscriptions"}},
            config_values=settings,
        )

        SubscriptionHubPlugin().handle_scheduler_trigger(ctx)

        self.assertEqual(ctx.results[-1], {
            "handled": True,
            "checked": 0,
            "sent": 0,
            "errors": [],
            "degraded": False,
            "skipped": "disabled",
        })

    def test_check_subscriptions_fans_out_one_update_to_every_target(self):
        base_subscription = self.subscription_settings()["subscriptions"][0]
        settings = merge_settings({}, self.subscription_settings(subscriptions=[
            {**base_subscription, "services": ["live"]},
            {
                **base_subscription,
                "id": "bilibili-123456-private-20000",
                "target_type": "private",
                "target_id": "20000",
                "target_name": "测试私聊",
                "services": ["live"],
            },
        ]))
        plugin = SubscriptionHubPlugin()
        ctx = FakeContext(
            config_values=settings,
            thirdparty_accounts=self.bilibili_cookie_accounts(),
            http_responses=[self.live_status_response()],
            storage=self.source_storage(live_state={"status": 0, "session": "previous", "room_id": "10001"}),
        )

        result = plugin.check_subscriptions(ctx, settings)

        self.assertEqual(result["checked"], 1)
        self.assertEqual(result["sent"], 2)
        self.assertEqual(result["errors"], [])
        self.assertEqual(
            [(message["target_type"], message["target_id"]) for message in ctx.messages],
            [("group", "10000"), ("private", "20000")],
        )
        self.assertEqual(
            [item["key"] for item in ctx.storage_sets if item["key"].startswith("seen:")],
            [
                "seen:bilibili-123456-group-10000:live:live-123456-started-1700000000",
                "seen:bilibili-123456-private-20000:live:live-123456-started-1700000000",
            ],
        )

    def test_failed_target_remains_unseen_and_does_not_stop_fanout(self):
        base_subscription = self.subscription_settings()["subscriptions"][0]
        settings = merge_settings({}, self.subscription_settings(subscriptions=[
            {**base_subscription, "services": ["live"]},
            {
                **base_subscription,
                "id": "bilibili-123456-private-20000",
                "target_type": "private",
                "target_id": "20000",
                "target_name": "测试私聊",
                "services": ["live"],
            },
        ]))
        plugin = SubscriptionHubPlugin()
        ctx = FakeContext(
            config_values=settings,
            thirdparty_accounts=self.bilibili_cookie_accounts(),
            http_responses=[self.live_status_response()],
            storage=self.source_storage(live_state={"status": 0, "session": "previous", "room_id": "10001"}),
            message_send_errors=[RuntimeError("send failed"), None],
        )

        result = plugin.check_subscriptions(ctx, settings)

        self.assertEqual(result["sent"], 1)
        self.assertEqual(result["errors"], ["Bilibili 订阅推送失败。"])
        self.assertEqual(
            [(message["target_type"], message["target_id"]) for message in ctx.message_send_attempts],
            [("group", "10000"), ("private", "20000")],
        )
        self.assertEqual(
            [(message["target_type"], message["target_id"]) for message in ctx.messages],
            [("private", "20000")],
        )
        self.assertNotIn(
            "seen:bilibili-123456-group-10000:live:live-123456-started-1700000000",
            ctx.storage,
        )
        self.assertTrue(ctx.storage[
            "seen:bilibili-123456-private-20000:live:live-123456-started-1700000000"
        ])

    def test_duplicate_event_is_skipped(self):
        settings = merge_settings({}, self.subscription_settings(subscriptions=[{
            **self.subscription_settings()["subscriptions"][0],
            "services": ["live"],
        }]))
        plugin = SubscriptionHubPlugin()
        storage = self.source_storage(live_state={"status": 0, "session": "previous", "room_id": "10001"})
        storage["seen:bilibili-123456-group-10000:live:live-123456-started-1700000000"] = True
        ctx = FakeContext(
            config_values=settings,
            thirdparty_accounts=self.bilibili_cookie_accounts(),
            http_responses=[self.live_status_response()],
            storage=storage,
        )

        result = plugin.check_subscriptions(ctx, settings)

        self.assertEqual(result["checked"], 1)
        self.assertEqual(result["sent"], 0)
        self.assertEqual(ctx.render_calls, [])
        self.assertEqual(ctx.messages, [])

    def test_check_subscriptions_respects_service_filter(self):
        settings = merge_settings({}, self.subscription_settings(subscriptions=[{
            **self.subscription_settings()["subscriptions"][0],
            "services": ["live"],
        }]))
        plugin = SubscriptionHubPlugin()
        ctx = FakeContext(
            config_values=settings,
            thirdparty_accounts=self.bilibili_cookie_accounts(),
            http_responses=[self.empty_live_status_response()],
        )

        result = plugin.check_subscriptions(ctx, settings)

        self.assertEqual(result["checked"], 1)
        self.assertEqual(result["sent"], 0)
        self.assertEqual(ctx.render_calls, [])
        self.assertTrue(ctx.http_requests[0]["url"].startswith(LIVE_STATUS_URL + "?"))

    def test_disabled_settings_skip_events(self):
        settings = merge_settings({}, self.subscription_settings(enabled=False))
        plugin = SubscriptionHubPlugin()
        ctx = FakeContext(config_values=settings)

        result = plugin.check_subscriptions(ctx, settings)

        self.assertEqual(result["skipped"], "disabled")
        self.assertEqual(ctx.render_calls, [])

    def test_preview_url_parser_supports_video_opus_dynamic_and_live(self):
        self.assertEqual(parse_preview_url("https://www.bilibili.com/video/BV1RayleaBot")["kind"], "video")
        self.assertEqual(parse_preview_url("https://www.bilibili.com/opus/100000000000000003")["kind"], "opus")
        self.assertEqual(parse_preview_url("https://t.bilibili.com/100000000000000001")["kind"], "dynamic")
        self.assertEqual(parse_preview_url("https://live.bilibili.com/10001")["kind"], "live")

    def test_preview_card_supports_t_bilibili_repost_link(self):
        plugin = SubscriptionHubPlugin()
        ctx = FakeContext(
            args=["https://t.bilibili.com/100000000000000001"],
            http_responses=[self.dynamic_detail_response()],
        )

        plugin.handle_preview_card(ctx)

        self.assertEqual(ctx.texts, [])
        self.assertEqual(ctx.http_requests[0]["url"], dynamic_detail_url("100000000000000001"))
        self.assertEqual(ctx.http_requests[0]["headers"]["Referer"], "https://t.bilibili.com/100000000000000001")
        render_data = ctx.render_calls[0]["data"]
        self.assertEqual(render_data["service"], "转发")
        self.assertEqual(render_data["original"]["title"], "测试原创作品标题")
        self.assertIn("rich-text-topic", render_data["content_html"])
        self.assertIn("rich-text-emoji", render_data["content_html"])
        self.assertIn("rich-text-topic", render_data["original"]["summary_html"])

    def test_preview_card_supports_opus_topic_module(self):
        plugin = SubscriptionHubPlugin()
        ctx = FakeContext(
            args=["https://www.bilibili.com/opus/100000000000000002"],
            http_responses=[self.opus_detail_response()],
        )

        plugin.handle_preview_card(ctx)

        self.assertEqual(ctx.texts, [])
        self.assertEqual(ctx.http_requests[0]["url"], opus_detail_url("100000000000000002"))
        render_data = ctx.render_calls[0]["data"]
        self.assertEqual(render_data["topic"]["name"], "测试活动 2026")
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
            args=["https://t.bilibili.com/100000000000000001"],
            http_responses=[{
                "status_code": 200,
                "body_text": json.dumps({"code": 0, "data": {"item": {}}}, ensure_ascii=False),
            }],
        )

        plugin.handle_preview_card(ctx)

        self.assertIn("未识别到可预览的动态内容", ctx.texts[-1])
        self.assertNotIn("响应格式不正确", ctx.texts[-1])

    def test_subscribe_user_lookup_reports_bilibili_risk_control(self):
        settings = merge_settings({}, {"subscriptions": []})
        ctx = FakeContext(
            args=["123456"],
            thirdparty_accounts=self.bilibili_cookie_accounts(),
            http_responses=[{
                "status_code": 200,
                "body_text": json.dumps({"code": -352, "message": "风控校验失败", "data": None}, ensure_ascii=False),
            }],
        )

        result = add_bilibili_subscription(settings, ctx)

        self.assertFalse(result["ok"])
        self.assertIn("Bilibili 请求被风控拦截，请稍后再试或重新扫码更新 CK", result["message"])
        self.assertIn("Bilibili code -352", result["message"])
        self.assertIn("HTTP 200", result["message"])
        self.assertIn("原始原因：风控校验失败", result["message"])
        self.assertNotIn('"code"', result["message"])

    def test_subscribe_user_lookup_redacts_secrets_from_risk_control_reason(self):
        settings = merge_settings({}, {"subscriptions": []})
        ctx = FakeContext(
            args=["123456"],
            thirdparty_accounts=self.bilibili_cookie_accounts(),
            http_responses=[{
                "status_code": 412,
                "body_text": json.dumps({
                    "code": -412,
                    "message": "blocked SESSDATA=private-value; bili_jct=private-csrf; Authorization: Bearer private-token",
                }),
            }],
        )

        result = add_bilibili_subscription(settings, ctx)

        self.assertFalse(result["ok"])
        self.assertIn("SESSDATA=[已隐藏]", result["message"])
        self.assertIn("bili_jct=[已隐藏]", result["message"])
        self.assertIn("authorization=[已隐藏]", result["message"])
        self.assertNotIn("private-value", result["message"])
        self.assertNotIn("private-csrf", result["message"])
        self.assertNotIn("private-token", result["message"])

    def test_subscribe_command_handler_reports_risk_control_as_handled(self):
        plugin = SubscriptionHubPlugin()
        ctx = FakeContext(
            args=["123456"],
            thirdparty_accounts=self.bilibili_cookie_accounts(),
            http_responses=[{
                "status_code": 412,
                "body_text": '{"code":-412,"message":"request was banned"}',
            }],
        )

        plugin.handle_subscribe_bilibili(ctx)

        self.assertIn("Bilibili 请求被风控拦截，请稍后再试或重新扫码更新 CK", ctx.texts[-1])
        self.assertEqual(ctx.results[-1], {"handled": True})
        self.assertEqual(ctx.config_writes, [])

    def test_dynamic_updates_extract_video(self):
        updates = dynamic_updates({"data": {"items": [self.video_item("987", "新视频", pub_ts=1700000000)]}})

        self.assertEqual(len(updates), 1)
        self.assertEqual(updates[0]["service"], "video")
        self.assertEqual(updates[0]["title"], "新视频")
        self.assertEqual(updates[0]["duration_text"], "03:21")
        self.assertEqual(updates[0]["images"], [{"url": "https://i0.hdslb.com/video.jpg"}])

    def test_dynamic_updates_extract_opus_summary_rich_text(self):
        updates = dynamic_updates({"data": {"items": [{
            "id_str": "100000000000000002",
            "type": "DYNAMIC_TYPE_DRAW",
            "modules": {
                "module_author": {"mid": "123456", "name": "测试 UP", "pub_ts": 1700000000, "pub_time": "今天 12:00"},
                "module_dynamic": {
                    "topic": {"name": "测试 UP2026巡演"},
                    "major": {
                        "type": "MAJOR_TYPE_OPUS",
                        "opus": {
                            "summary": {
                                "text": "#测试活动 2026#\n线下演唱会，测试内容更新。[打call]",
                                "rich_text_nodes": [
                                    {"type": "RICH_TEXT_NODE_TYPE_WEB", "text": "#测试活动 2026#"},
                                    {"type": "RICH_TEXT_NODE_TYPE_TEXT", "text": "\n线下演唱会，测试内容更新。"},
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
        self.assertEqual(update["topic"]["name"], "测试 UP2026巡演")
        self.assertIn("rich-text-topic", update["summary_html"])
        self.assertIn("#测试 UP2026巡演#", update["summary_html"])
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
        self.assertEqual(update["original"]["title"], "测试原创作品标题")
        self.assertEqual(update["original"]["topic"]["name"], "测试 UP")
        self.assertIn("rich-text-topic", update["original"]["summary_html"])

    def test_repost_update_fetches_original_before_render(self):
        settings = merge_settings({}, self.subscription_settings(subscriptions=[{
            **self.subscription_settings()["subscriptions"][0],
            "services": ["repost"],
        }]))
        subscription = settings["subscriptions"][0]
        plugin = SubscriptionHubPlugin()
        ctx = FakeContext(
            config_values=settings,
            http_responses=[self.dynamic_detail_response()],
        )
        update = self.dynamic_event_payload(
            id="100000000000000001",
            service="repost",
            title="转发动态",
            summary="测试原创作品标题",
            url="https://t.bilibili.com/100000000000000001",
            author={"uid": "123456", "name": "测试转发用户"},
        )

        prepared = plugin.prepare_push_update(ctx, subscription, update)

        self.assertEqual(prepared["image_path"], "plugin-test.png")
        self.assertEqual(ctx.http_requests[0]["url"], dynamic_detail_url("100000000000000001"))
        self.assertEqual(ctx.http_requests[0]["headers"]["Referer"], "https://t.bilibili.com/100000000000000001")
        render_data = ctx.render_calls[0]["data"]
        self.assertEqual(render_data["original"]["title"], "测试原创作品标题")
        self.assertEqual(render_data["original"]["topic"]["name"], "测试 UP")
        self.assertIn("rich-text-topic", render_data["content_html"])
        self.assertIn("rich-text-emoji", render_data["content_html"])
        self.assertIn("rich-text-topic", render_data["original"]["summary_html"])

    def test_image_text_update_fetches_detail_when_feed_omits_content(self):
        subscription = self.subscription_settings()["subscriptions"][0]
        plugin = SubscriptionHubPlugin()
        ctx = FakeContext(http_responses=[self.opus_detail_response()])
        update = self.dynamic_event_payload(
            id="100000000000000002",
            service="image_text",
            title="测试 UP 发布图文动态",
            summary="",
            summary_html="",
            url="https://t.bilibili.com/100000000000000002",
            images=[{"url": "https://i0.hdslb.com/bfs/new_dyn/single.jpg"}],
        )

        prepared = plugin.prepare_push_update(ctx, subscription, update)

        self.assertEqual(prepared["image_path"], "plugin-test.png")
        self.assertEqual(ctx.http_requests[0]["url"], dynamic_detail_url("100000000000000002"))
        render_data = ctx.render_calls[0]["data"]
        self.assertIn("线下演唱会，测试内容更新。", render_data["content_text"])
        self.assertIn("rich-text-topic", render_data["content_html"])
        self.assertEqual(render_data["topic"]["name"], "测试活动 2026")

    def test_image_text_detail_enrichment_is_reused_across_targets(self):
        plugin = SubscriptionHubPlugin()
        ctx = FakeContext(http_responses=[self.opus_detail_response()])
        update = self.dynamic_event_payload(
            id="100000000000000002",
            service="image_text",
            summary="",
            summary_html="",
            url="https://t.bilibili.com/100000000000000002",
        )
        cache = {}
        first = self.subscription_settings()["subscriptions"][0]
        second = {**first, "target_type": "private", "target_id": "20000"}

        plugin.prepare_push_update(ctx, first, update, cache)
        plugin.prepare_push_update(ctx, second, update, cache)

        self.assertEqual(len(ctx.http_requests), 1)
        self.assertEqual(len(ctx.render_calls), 2)
        self.assertIn("线下演唱会，测试内容更新。", ctx.render_calls[1]["data"]["content_text"])

    def test_repost_update_still_pushes_when_original_lookup_fails(self):
        settings = merge_settings({}, self.subscription_settings(subscriptions=[{
            **self.subscription_settings()["subscriptions"][0],
            "services": ["repost"],
        }]))
        subscription = settings["subscriptions"][0]
        plugin = SubscriptionHubPlugin()
        ctx = FakeContext(
            config_values=settings,
            http_responses=[{
                "status_code": 200,
                "body_text": json.dumps({"code": -352, "message": "风控校验失败"}, ensure_ascii=False),
            }],
        )
        update = self.dynamic_event_payload(
            id="100000000000000001",
            service="repost",
            title="转发动态",
            summary="测试原创作品标题",
            url="https://t.bilibili.com/100000000000000001",
            author={"uid": "123456", "name": "测试转发用户"},
        )

        prepared = plugin.prepare_push_update(ctx, subscription, update)

        self.assertEqual(prepared["image_path"], "plugin-test.png")
        self.assertEqual(ctx.render_calls[0]["data"]["original"], None)

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
                "name": "测试活动 2026",
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

        self.assertEqual(render_data["topic"]["name"], "测试活动 2026")
        self.assertEqual(render_data["topic"]["label"], "# 测试活动 2026")
        self.assertEqual(render_data["original"]["topic"]["name"], "原动态话题")


if __name__ == "__main__":
    unittest.main()
