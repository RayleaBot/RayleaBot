import os
import sys
import unittest


PLUGIN_DIR = os.path.dirname(os.path.dirname(__file__))
sys.path.insert(0, PLUGIN_DIR)

from bilibili import dynamic_updates
from rendering import build_render_data
from settings import merge_settings


class SubscriptionHubTests(unittest.TestCase):
    def test_merge_settings_normalizes_tokens_and_subscriptions(self):
        settings = merge_settings({}, {
            "tokens": [{
                "id": "Primary",
                "label": "主 token",
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
        self.assertEqual(settings["subscriptions"][0]["services"], ["video", "live"])
        self.assertEqual(settings["subscriptions"][0]["subscribers"][0]["nickname"], "订阅人")

    def test_dynamic_updates_extract_video(self):
        updates = dynamic_updates({
            "data": {
                "items": [{
                    "id_str": "987",
                    "basic": {"jump_url": "//www.bilibili.com/video/BV1"},
                    "modules": {
                        "module_author": {"name": "测试 UP", "pub_time": "今天 12:00"},
                        "module_dynamic": {
                            "major": {
                                "type": "MAJOR_TYPE_ARCHIVE",
                                "archive": {"title": "新视频", "desc": "视频简介"},
                            },
                        },
                    },
                }],
            },
        })
        self.assertEqual(len(updates), 1)
        self.assertEqual(updates[0]["service"], "video")
        self.assertEqual(updates[0]["title"], "新视频")

    def test_dynamic_updates_extract_repost_before_major_type(self):
        updates = dynamic_updates({
            "data": {
                "items": [{
                    "id_str": "988",
                    "basic": {
                        "comment_type": "17",
                        "jump_url": "//t.bilibili.com/988",
                    },
                    "modules": {
                        "module_author": {"name": "测试 UP", "pub_time": "今天 13:00"},
                        "module_dynamic": {
                            "desc": {"text": "转发推荐"},
                            "major": {
                                "type": "MAJOR_TYPE_OPUS",
                                "opus": {"title": "被转发内容"},
                            },
                        },
                    },
                }],
            },
        })
        self.assertEqual(len(updates), 1)
        self.assertEqual(updates[0]["service"], "repost")

    def test_render_data_contains_subscriber_info(self):
        data = build_render_data({
            "uid": "123456",
            "name": "测试 UP",
            "subscribers": [{"id": "42", "nickname": "订阅人"}],
        }, {
            "service": "video",
            "title": "新视频",
            "summary": "视频简介",
            "author": {"name": "测试 UP"},
        })
        self.assertEqual(data["subscriber_text"], "订阅人")
        self.assertEqual(data["service"], "视频")


if __name__ == "__main__":
    unittest.main()
