import base64
import json
import tempfile
import unittest
from pathlib import Path

import sys

PLUGIN_DIR = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(PLUGIN_DIR))

from raylea_game_guide import (  # noqa: E402
    GameGuideService,
    parse_game_guide_command,
    parse_guide_request,
)
from main import GameGuidePlugin  # noqa: E402


class FakeContext:
    def __init__(self, plain_text="*昔涟攻略", command=None, args=None, responses=None):
        self.event_type = "message.group"
        self.command_prefixes = ["*"]
        self.plain_text = plain_text
        self.command = command
        self.args = list(args or [])
        self.target_type = "group"
        self.target_id = "553855023"
        self.bot_id = "2609164374"
        self.responses = list(responses or [])
        self.requests = []
        self.messages = []
        self.forward_messages = []
        self.texts = []
        self.results = []
        self.logs = []

    def http_request(self, method, url, headers=None, timeout_seconds=30):
        self.requests.append({
            "method": method,
            "url": url,
            "headers": headers or {},
            "timeout_seconds": timeout_seconds,
        })
        if not self.responses:
            return {"status_code": 404, "body_text": ""}
        response = self.responses.pop(0)
        if isinstance(response, Exception):
            raise response
        return response

    def send_message(self, segments, target_type=None, target_id=None):
        self.messages.append({"segments": segments, "target_type": target_type, "target_id": target_id})

    def message_forward_send(self, target_type, target_id, messages, timeout_seconds=30):
        self.forward_messages.append({
            "target_type": target_type,
            "target_id": target_id,
            "messages": messages,
            "timeout_seconds": timeout_seconds,
        })
        return {"message_id": f"forward-{len(self.forward_messages)}"}

    def send_text(self, text):
        self.texts.append(text)

    def send_result(self, result):
        self.results.append(result)

    def logger_write(self, level, message, fields=None):
        self.logs.append({"level": level, "message": message, "fields": fields or {}})


def search_response():
    document = {
        "retcode": 0,
        "message": "OK",
        "data": {
            "posts": [
                {
                    "post": {
                        "game_id": 6,
                        "post_id": "70078539",
                        "f_forum_id": 61,
                        "subject": "【V3.6攻略】昔涟前瞻攻略",
                        "content": "昔涟角色攻略图",
                        "images": [
                            "https://upload-bbs.miyoushe.com/upload/guide-1.png",
                            "https://upload-bbs.miyoushe.com/upload/guide-2.png",
                        ],
                    },
                    "user": {"nickname": "赋赋"},
                },
                {
                    "post": {
                        "game_id": 6,
                        "post_id": "70078540",
                        "f_forum_id": 61,
                        "subject": "昔涟攻略一图流",
                        "content": "昔涟攻略",
                        "images": ["https://bbs-static.miyoushe.com/static/guide-3.jpg"],
                    },
                    "user": {"nickname": "另一位作者"},
                },
            ]
        },
    }
    return {"status_code": 200, "body_text": json.dumps(document, ensure_ascii=False)}


def detail_response(post_id, image_urls):
    document = {
        "retcode": 0,
        "message": "OK",
        "data": {
            "post": {
                "post": {
                    "game_id": 6,
                    "post_id": post_id,
                    "f_forum_id": 61,
                    "subject": "昔涟攻略",
                    "content": "<p>攻略正文</p>",
                    "images": list(image_urls),
                },
                "image_list": [{"url": url} for url in image_urls],
            }
        },
    }
    return {"status_code": 200, "body_text": json.dumps(document, ensure_ascii=False)}


def image_response(content):
    return {
        "status_code": 200,
        "body_base64": base64.b64encode(content).decode("ascii"),
    }


def forwarded_image_files(ctx):
    files = []
    for batch in ctx.forward_messages:
        for node in batch["messages"]:
            for segment in node["data"]["content"]:
                if segment["type"] == "image":
                    files.append(segment["data"]["file"])
    return files


def cached_image_count(cache_root, character_slug):
    index_path = Path(cache_root) / "guides" / character_slug / "index.json"
    document = json.loads(index_path.read_text(encoding="utf-8"))
    return sum(len(source.get("images") or []) for source in document.get("sources") or [])


class GameGuideServiceTest(unittest.TestCase):
    def test_parse_star_prefixed_character_guide(self):
        self.assertEqual(parse_guide_request("*昔涟攻略", ["!"]), "昔涟")
        self.assertIsNone(parse_guide_request("昔涟攻略", ["!"]))
        self.assertEqual(parse_guide_request("＊昔莲攻略", []), "昔莲")
        self.assertIsNone(parse_guide_request("*昔涟图鉴", ["*"]))

    def test_parse_game_guide_command(self):
        self.assertEqual(parse_game_guide_command("游戏攻略", ["昔涟"]), "昔涟")
        self.assertIsNone(parse_game_guide_command("echo", ["昔涟"]))

    def test_plugin_does_not_subscribe_to_all_messages(self):
        plugin = GameGuidePlugin()

        self.assertNotIn("message.group", plugin._subscriptions)
        self.assertNotIn("message.private", plugin._subscriptions)

    def test_downloads_caches_and_sends_multiple_source_images(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            service = GameGuideService(plugin_dir=PLUGIN_DIR, cache_root=temp_dir)
            ctx = FakeContext(responses=[
                search_response(),
                detail_response("70078539", [
                    "https://upload-bbs.miyoushe.com/upload/guide-1.png",
                    "https://upload-bbs.miyoushe.com/upload/guide-2.png",
                ]),
                detail_response("70078540", [
                    "https://bbs-static.miyoushe.com/static/guide-3.jpg",
                ]),
                image_response(b"first image"),
                image_response(b"second image"),
                image_response(b"third image"),
            ])

            service.handle_message(ctx)

            self.assertEqual(len(ctx.forward_messages), 1)
            self.assertEqual(len(forwarded_image_files(ctx)), 3)
            self.assertEqual(len(ctx.messages), 0)
            self.assertEqual(cached_image_count(temp_dir, "xilian"), 3)
            self.assertEqual(ctx.results[-1]["character"], "昔涟")
            self.assertEqual(ctx.results[-1]["images"], 3)
            self.assertFalse(ctx.results[-1]["from_cache"])
            self.assertTrue(any(item["message"] == "游戏攻略开始下载来源图片" for item in ctx.logs))
            self.assertTrue(any(item["message"] == "游戏攻略合并转发发送完成" for item in ctx.logs))
            for file_ref in forwarded_image_files(ctx):
                self.assertTrue(file_ref.startswith("file:"))
                self.assertTrue(Path(file_ref.replace("file:///", "")).exists() or file_ref.startswith("file:///"))

            cached_ctx = FakeContext(responses=[])
            service.handle_message(cached_ctx)
            self.assertEqual(len(cached_ctx.requests), 0)
            self.assertEqual(len(cached_ctx.forward_messages), 1)
            self.assertEqual(len(forwarded_image_files(cached_ctx)), 3)
            self.assertTrue(cached_ctx.results[-1]["from_cache"])
            self.assertTrue(any(item["message"] == "游戏攻略命中缓存" for item in cached_ctx.logs))

    def test_alias_resolves_before_search(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            service = GameGuideService(plugin_dir=PLUGIN_DIR, cache_root=temp_dir)
            ctx = FakeContext(plain_text="*昔莲攻略", responses=[
                search_response(),
                detail_response("70078539", ["https://upload-bbs.miyoushe.com/upload/guide-1.png"]),
                detail_response("70078540", ["https://bbs-static.miyoushe.com/static/guide-3.jpg"]),
                image_response(b"first image"),
            ])

            service.handle_message(ctx)

            self.assertIn("keyword=%E6%98%94%E6%B6%9F%E6%94%BB%E7%95%A5", ctx.requests[0]["url"])
            self.assertEqual(len(forwarded_image_files(ctx)), 1)
            self.assertEqual(ctx.results[-1]["character"], "昔涟")

    def test_plain_suffix_request_is_ignored(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            service = GameGuideService(plugin_dir=PLUGIN_DIR, cache_root=temp_dir)
            ctx = FakeContext(plain_text="昔涟攻略", responses=[search_response()])

            service.handle_message(ctx)

            self.assertEqual(len(ctx.requests), 0)
            self.assertEqual(len(ctx.forward_messages), 0)
            self.assertEqual(ctx.results[-1], {"handled": False})

    def test_skips_failed_image_downloads_without_aborting_query(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            service = GameGuideService(plugin_dir=PLUGIN_DIR, cache_root=temp_dir)
            ctx = FakeContext(responses=[
                search_response(),
                detail_response("70078539", [
                    "https://upload-bbs.miyoushe.com/upload/guide-1.png",
                    "https://upload-bbs.miyoushe.com/upload/guide-2.png",
                ]),
                detail_response("70078540", [
                    "https://bbs-static.miyoushe.com/static/guide-3.jpg",
                ]),
                image_response(b"first image"),
                RuntimeError("http.request failed"),
                image_response(b"third image"),
            ])

            service.handle_message(ctx)

            self.assertEqual(len(ctx.forward_messages), 1)
            self.assertEqual(len(forwarded_image_files(ctx)), 2)
            self.assertEqual(ctx.results[-1]["images"], 2)

    def test_expands_search_result_with_post_detail_images(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            service = GameGuideService(plugin_dir=PLUGIN_DIR, cache_root=temp_dir)
            ctx = FakeContext(responses=[
                search_response(),
                detail_response("70078539", [
                    "https://upload-bbs.miyoushe.com/upload/full-1.png",
                    "https://upload-bbs.miyoushe.com/upload/full-2.png",
                    "https://upload-bbs.miyoushe.com/upload/full-3.png",
                ]),
                detail_response("70078540", [
                    "https://bbs-static.miyoushe.com/static/full-4.jpg",
                ]),
                image_response(b"first image"),
                image_response(b"second image"),
                image_response(b"third image"),
                image_response(b"fourth image"),
            ])

            service.handle_message(ctx)

            self.assertEqual(len(ctx.forward_messages), 1)
            self.assertEqual(len(forwarded_image_files(ctx)), 4)
            requested_urls = [request["url"] for request in ctx.requests]
            self.assertTrue(any("getPostFull?post_id=70078539" in url for url in requested_urls))
            self.assertTrue(any("getPostFull?post_id=70078540" in url for url in requested_urls))

    def test_refreshes_legacy_cache_without_index_before_replying(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            service = GameGuideService(plugin_dir=PLUGIN_DIR, cache_root=temp_dir)
            character = service.characters.resolve("昔涟")
            guide_dir = service.guide_dir(character)
            guide_dir.mkdir(parents=True)
            (guide_dir / "legacy.png").write_bytes(b"legacy image")

            ctx = FakeContext(responses=[
                search_response(),
                detail_response("70078539", [
                    "https://upload-bbs.miyoushe.com/upload/full-1.png",
                    "https://upload-bbs.miyoushe.com/upload/full-2.png",
                    "https://upload-bbs.miyoushe.com/upload/full-3.png",
                ]),
                detail_response("70078540", []),
                image_response(b"first image"),
                image_response(b"second image"),
                image_response(b"third image"),
            ])

            service.handle_message(ctx)

            self.assertEqual(len(ctx.forward_messages), 1)
            self.assertEqual(len(forwarded_image_files(ctx)), 3)
            self.assertFalse(ctx.results[-1]["from_cache"])

    def test_falls_back_to_cached_images_when_index_is_missing_and_refresh_fails(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            service = GameGuideService(plugin_dir=PLUGIN_DIR, cache_root=temp_dir)
            character = service.characters.resolve("昔涟")
            guide_dir = service.guide_dir(character)
            guide_dir.mkdir(parents=True)
            (guide_dir / "02.png").write_bytes(b"second image")
            (guide_dir / "01.png").write_bytes(b"first image")

            ctx = FakeContext(responses=[])
            service.handle_message(ctx)

            self.assertEqual(len(ctx.requests), 3)
            self.assertEqual(len(ctx.forward_messages), 1)
            sent_files = forwarded_image_files(ctx)
            self.assertIn("01.png", sent_files[0])
            self.assertIn("02.png", sent_files[1])
            self.assertTrue(ctx.results[-1]["from_cache"])

    def test_default_forward_batch_uses_qq_group_limit(self):
        service = GameGuideService(plugin_dir=PLUGIN_DIR)
        ctx = FakeContext()
        character = service.characters.resolve("昔涟")
        images = [{"file_uri": f"file:///tmp/{index}.png"} for index in range(101)]

        service.send_images(ctx, character, images, from_cache=True)

        self.assertEqual([100, 1], [len(batch["messages"]) for batch in ctx.forward_messages])

    def test_chunks_large_forward_messages(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            service = GameGuideService(
                plugin_dir=PLUGIN_DIR,
                cache_root=temp_dir,
                forward_images_per_message=2,
            )
            ctx = FakeContext(responses=[
                search_response(),
                detail_response("70078539", [
                    "https://upload-bbs.miyoushe.com/upload/full-1.png",
                    "https://upload-bbs.miyoushe.com/upload/full-2.png",
                    "https://upload-bbs.miyoushe.com/upload/full-3.png",
                ]),
                detail_response("70078540", [
                    "https://bbs-static.miyoushe.com/static/full-4.jpg",
                ]),
                image_response(b"first image"),
                image_response(b"second image"),
                image_response(b"third image"),
                image_response(b"fourth image"),
            ])

            service.handle_message(ctx)

            self.assertEqual([2, 2], [len(batch["messages"]) for batch in ctx.forward_messages])
            self.assertEqual(len(forwarded_image_files(ctx)), 4)
            self.assertEqual(ctx.results[-1]["images"], 4)


if __name__ == "__main__":
    unittest.main()
