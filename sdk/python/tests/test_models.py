import unittest
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(__file__)))

from rayleabot import (
    BilibiliAuthor,
    BilibiliImage,
    BilibiliPayload,
    Bot,
    EventPayload,
    InitFrame,
    OneBotPayload,
    PassthroughSegment,
    flash_file_segment,
    markdown_segment,
    record_segment,
    segment_from_dict,
    shake_segment,
)


class ModelHelpersTests(unittest.TestCase):
    def test_segment_from_dict_supports_flash_file(self):
        segment = segment_from_dict({
            "type": "flash_file",
            "data": {
                "file_id": "file_001",
            },
        })

        self.assertIsInstance(segment, PassthroughSegment)
        self.assertEqual("flash_file", segment.segment_type)
        self.assertEqual({"file_id": "file_001"}, segment.data)

    def test_named_passthrough_segment_builders_keep_segment_type(self):
        self.assertEqual(
            {"type": "record", "data": {"file": "voice.amr"}},
            record_segment({"file": "voice.amr"}).to_dict(),
        )
        self.assertEqual(
            {"type": "markdown", "data": {"content": "## title"}},
            markdown_segment("## title").to_dict(),
        )
        self.assertEqual(
            {"type": "flash_file", "data": {"name": "clip.zip"}},
            flash_file_segment({"name": "clip.zip"}).to_dict(),
        )
        self.assertEqual(
            {"type": "shake", "data": {"strength": "full"}},
            shake_segment({"strength": "full"}).to_dict(),
        )

    def test_onebot_payload_preserves_meta_fields(self):
        payload = OneBotPayload.from_dict({
            "post_type": "meta_event",
            "meta_event_type": "heartbeat",
            "self_id": "bot-10001",
            "time": 1710000000,
            "interval": 5000,
            "status": {
                "online": True,
                "good": True,
            },
        })

        self.assertEqual("heartbeat", payload.meta_event_type)
        self.assertEqual(5000, payload.interval)
        self.assertEqual({"online": True, "good": True}, payload.status)
        self.assertEqual("heartbeat", payload.to_dict()["meta_event_type"])

    def test_bilibili_payload_roundtrip(self):
        payload = EventPayload.from_dict({
            "bilibili": {
                "kind": "live",
                "uid": "123456",
                "id": "live-123456-10001-1710000500",
                "room_id": "10001",
                "service": "live",
                "title": "直播间已开播",
                "summary": "直播中",
                "url": "https://live.bilibili.com/10001",
                "pub_ts": 1710000500,
                "created_at": "2024-03-09 16:08",
                "author": {
                    "uid": "123456",
                    "name": "测试主播",
                    "avatar": "https://i0.hdslb.com/bfs/face/live.jpg",
                },
                "images": [
                    {
                        "url": "https://i0.hdslb.com/bfs/live/cover.jpg",
                        "width": 1280,
                        "height": 720,
                    },
                ],
                "live_status": 1,
                "live_event": "started",
                "status_label": "直播中",
                "live_started_at": "2024-03-09 16:08",
            },
        })

        self.assertEqual("123456", payload.bilibili.uid)
        self.assertEqual("测试主播", payload.bilibili.author.name)
        self.assertEqual(1280, payload.bilibili.images[0].width)
        self.assertEqual("started", payload.to_dict()["bilibili"]["live_event"])

    def test_bilibili_payload_builder_strips_empty_fields(self):
        payload = EventPayload(
            bilibili=BilibiliPayload(
                kind="dynamic",
                uid="123456",
                id="90001",
                service="video",
                url="https://www.bilibili.com/video/BV1RayleaBot",
                author=BilibiliAuthor(uid="123456", name="测试 UP"),
                images=[BilibiliImage(url="https://i0.hdslb.com/bfs/archive/cover.jpg")],
                dynamic_type="DYNAMIC_TYPE_AV",
            ),
        )

        encoded = payload.to_dict()["bilibili"]
        self.assertNotIn("room_id", encoded)
        self.assertEqual("DYNAMIC_TYPE_AV", encoded["dynamic_type"])

    def test_init_frame_allows_missing_bot(self):
        frame = InitFrame(plugin_id="weather", request_id="init-1", bot=None)

        encoded = frame.to_dict()
        self.assertNotIn("bot", encoded)

        decoded = InitFrame.from_dict({
            "protocol_version": "1",
            "type": "init",
            "timestamp": 1710000000,
            "plugin_id": "weather",
            "request_id": "init-1",
            "command_prefixes": ["/"],
        })
        self.assertIsNone(decoded.bot)

    def test_init_frame_preserves_bot_when_present(self):
        frame = InitFrame(plugin_id="weather", request_id="init-2", bot=Bot(id="10001"), super_admins=["9001"])

        encoded = frame.to_dict()
        self.assertEqual({"id": "10001"}, encoded["bot"])
        self.assertEqual({"super_admins": ["9001"]}, encoded["permissions"])
        self.assertEqual("10001", InitFrame.from_dict(encoded).bot.id)
        self.assertEqual(["9001"], InitFrame.from_dict(encoded).super_admins)


if __name__ == "__main__":
    unittest.main()
