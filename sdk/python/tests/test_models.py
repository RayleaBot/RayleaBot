import unittest

from rayleabot import (
    Bot,
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
        frame = InitFrame(plugin_id="weather", request_id="init-2", bot=Bot(id="10001"))

        encoded = frame.to_dict()
        self.assertEqual({"id": "10001"}, encoded["bot"])
        self.assertEqual("10001", InitFrame.from_dict(encoded).bot.id)


if __name__ == "__main__":
    unittest.main()
