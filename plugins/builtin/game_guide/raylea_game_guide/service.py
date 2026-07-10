"""Guide lookup and local image cache for the built-in game guide plugin."""

import base64
import hashlib
import json
import re
from datetime import datetime, timezone
from pathlib import Path
from urllib.parse import quote, urlparse


STAR_RAIL_GIDS = 6
STAR_RAIL_GUIDE_FORUM_ID = 61
SEARCH_ENDPOINT = "https://bbs-api.miyoushe.com/post/wapi/searchPosts"
DETAIL_ENDPOINT = "https://bbs-api.miyoushe.com/post/wapi/getPostFull"
DEFAULT_HEADERS = {
    "Accept": "application/json",
    "Referer": "https://www.miyoushe.com/sr/",
    "User-Agent": "Mozilla/5.0 RayleaBot/0.1 game-guide",
    "x-rpc-app_version": "2.83.1",
    "x-rpc-client_type": "4",
}
DEFAULT_TRIGGER_PREFIXES = ["*", "＊"]
DEFAULT_SUFFIXES = ["攻略"]
DEFAULT_MAX_SOURCES = 4
DEFAULT_MAX_IMAGES_PER_SOURCE = 60
DEFAULT_MAX_TOTAL_IMAGES = 120
DEFAULT_FORWARD_IMAGES_PER_MESSAGE = 100
DEFAULT_FORWARD_SEND_TIMEOUT_SECONDS = 45
SUPPORTED_IMAGE_HOSTS = {
    "bbs-static.miyoushe.com",
    "upload-bbs.mihoyo.com",
    "upload-bbs.miyoushe.com",
}
SUPPORTED_CACHE_EXTENSIONS = {".gif", ".jpg", ".jpeg", ".png", ".webp"}


def normalize_key(value):
    text = str(value or "").strip().lower()
    text = text.replace("·", "").replace("•", "").replace("・", "")
    return re.sub(r"[\s_\-:：，,。.!！?？()\[\]{}<>《》【】\"'“”‘’/\\]+", "", text)


def parse_guide_request(plain_text, command_prefixes=None, trigger_prefixes=None, suffixes=None):
    text = str(plain_text or "").strip()
    if not text:
        return None

    prefixes = list(command_prefixes or []) + list(trigger_prefixes or DEFAULT_TRIGGER_PREFIXES)
    prefixes = sorted({str(item) for item in prefixes if str(item or "")}, key=len, reverse=True)
    body = None
    for prefix in prefixes:
        if text.startswith(prefix):
            body = text[len(prefix):].strip()
            break
    if body is None:
        return None

    for suffix in sorted(suffixes or DEFAULT_SUFFIXES, key=len, reverse=True):
        suffix = str(suffix)
        if body.endswith(suffix):
            character = body[:-len(suffix)].strip()
            return character or None
    return None


def parse_game_guide_command(command, args):
    if str(command or "").strip() != "游戏攻略":
        return None
    character = " ".join(str(item) for item in (args or [])).strip()
    return character or None


def parse_character_list_request(plain_text, trigger_prefixes=None):
    text = str(plain_text or "").strip()
    if not text:
        return False
    prefixes = list(trigger_prefixes or DEFAULT_TRIGGER_PREFIXES)
    prefixes = sorted({str(item) for item in prefixes if str(item or "")}, key=len, reverse=True)
    for prefix in prefixes:
        if text.startswith(prefix):
            body = text[len(prefix):].strip()
            if body == "角色列表":
                return True
    return False


# Mapping of common Chinese characters to pinyin initials used for grouping
# character names in the character list.  Covers all first characters that
# appear in data/characters.json plus common extensions.
_PINYIN_INITIAL = {
    "阿": "A", "爱": "A", "安": "A", "暗": "A",
    "白": "B", "百": "B", "半": "B", "宝": "B", "北": "B", "本": "B", "比": "B",
    "波": "B", "不": "B", "布": "B",
    "才": "C", "苍": "C", "草": "C", "长": "C", "朝": "C", "沉": "C", "成": "C",
    "赤": "C", "初": "C", "春": "C", "纯": "C",
    "大": "D", "丹": "D", "东": "D", "冬": "D", "独": "D", "渡": "D",
    "俄": "E", "恩": "E",
    "菲": "F", "飞": "F", "绯": "F", "翡": "F", "风": "F", "枫": "F",
    "伏": "F", "芙": "F", "符": "F", "福": "F",
    "盖": "G", "甘": "G", "钢": "G", "格": "G", "公": "G", "古": "G",
    "关": "G", "光": "G", "归": "G", "桂": "G", "国": "G",
    "海": "H", "寒": "H", "好": "H", "浩": "H", "河": "H", "黑": "H",
    "红": "H", "虎": "H", "花": "H", "华": "H", "幻": "H", "黄": "H",
    "辉": "H", "回": "H", "火": "H", "霍": "H", "藿": "H",
    "疾": "J", "极": "J", "季": "J", "嘉": "J", "加": "J", "迦": "J",
    "剑": "J", "姜": "J", "椒": "J", "杰": "J", "洁": "J", "金": "J",
    "近": "J", "京": "J", "晶": "J", "景": "J", "静": "J", "镜": "J",
    "九": "J", "姬": "J",
    "卡": "K", "开": "K", "凯": "K", "康": "K", "柯": "K", "克": "K",
    "刻": "K", "空": "K", "苦": "K", "库": "K",
    "拉": "L", "莱": "L", "蓝": "L", "老": "L", "雷": "L", "冷": "L",
    "李": "L", "丽": "L", "莲": "L", "炼": "L", "烈": "L", "林": "L",
    "铃": "L", "玲": "L", "零": "L", "灵": "L", "流": "L", "龙": "L",
    "露": "L", "卢": "L", "陆": "L", "路": "L", "乱": "L", "伦": "L",
    "罗": "L", "洛": "L", "绿": "L",
    "玛": "M", "麦": "M", "满": "M", "猫": "M", "梅": "M", "美": "M",
    "梦": "M", "米": "M", "密": "M", "明": "M", "鸣": "M", "铭": "M",
    "摩": "M", "魔": "M", "墨": "M", "木": "M", "牧": "M", "貊": "M",
    "那": "N", "娜": "N", "南": "N", "尼": "N", "逆": "N", "诺": "N",
    "帕": "P", "派": "P", "庞": "P", "佩": "P", "皮": "P", "平": "P",
    "七": "Q", "奇": "Q", "契": "Q", "千": "Q", "浅": "Q", "乔": "Q",
    "切": "Q", "琴": "Q", "青": "Q", "清": "Q", "轻": "Q", "秋": "Q",
    "泉": "Q",
    "然": "R", "日": "R", "荣": "R", "如": "R", "瑞": "R", "若": "R",
    "阮": "R", "刃": "R",
    "萨": "S", "赛": "S", "三": "S", "桑": "S", "瑟": "S", "森": "S",
    "沙": "S", "砂": "S", "山": "S", "深": "S", "神": "S", "圣": "S",
    "十": "S", "石": "S", "时": "S", "史": "S", "霜": "S", "水": "S",
    "丝": "S", "死": "S", "四": "S", "苏": "S", "素": "S", "速": "S",
    "岁": "S",
    "塔": "T", "太": "T", "泰": "T", "唐": "T", "桃": "T", "特": "T",
    "天": "T", "铁": "T", "停": "T", "同": "T", "托": "T", "缇": "T",
    "万": "W", "王": "W", "忘": "W", "威": "W", "维": "W", "未": "W",
    "温": "W", "文": "W", "乌": "W", "无": "W", "瓦": "W",
    "夕": "X", "希": "X", "昔": "X", "西": "X", "夏": "X", "仙": "X",
    "先": "X", "贤": "X", "香": "X", "享": "X", "小": "X", "晓": "X",
    "邪": "X", "辛": "X", "星": "X", "行": "X", "修": "X", "虚": "X",
    "墟": "X", "雪": "X", "血": "X", "巡": "X", "希": "X", "遐": "X",
    "雪": "X",
    "鸦": "Y", "芽": "Y", "炎": "Y", "彦": "Y", "雁": "Y", "夜": "Y",
    "伊": "Y", "依": "Y", "以": "Y", "银": "Y", "饮": "Y", "樱": "Y",
    "影": "Y", "永": "Y", "幽": "Y", "尤": "Y", "游": "Y", "宇": "Y",
    "雨": "Y", "羽": "Y", "御": "Y", "元": "Y", "原": "Y", "缘": "Y",
    "远": "Y", "月": "Y", "云": "Y", "驭": "Y", "爻": "Y",
    "灾": "Z", "造": "Z", "泽": "Z", "战": "Z", "张": "Z", "长": "Z",
    "哲": "Z", "真": "Z", "正": "Z", "之": "Z", "知": "Z", "止": "Z",
    "至": "Z", "志": "Z", "中": "Z", "终": "Z", "重": "Z", "朱": "Z",
    "诛": "Z", "逐": "Z", "主": "Z", "祝": "Z", "追": "Z", "自": "Z",
    "罪": "Z", "佐": "Z",
}


def _character_group_key(name):
    """Return the uppercase grouping key for a character name.

    Chinese names use the pinyin initial of the first character.
    ASCII letters use the first character uppercased.
    Digits and symbols fall into '#'.
    """
    if not name:
        return "#"
    first = name[0]
    mapped = _PINYIN_INITIAL.get(first)
    if mapped:
        return mapped
    if "A" <= first.upper() <= "Z":
        return first.upper()
    return "#"


def build_character_list_render_data(characters, title=None, subtitle=None):
    """Group a list of character dicts by first-letter for template rendering."""
    grouped = {}
    for character in characters:
        name = str(character.get("name") or "").strip()
        if not name:
            continue
        key = _character_group_key(name)
        grouped.setdefault(key, []).append({
            "name": name,
            "slug": str(character.get("slug") or "").strip(),
            "aliases": [str(alias) for alias in character.get("aliases") or [] if str(alias).strip()],
        })

    groups = [
        {
            "label": letter,
            "characters": sorted(items, key=lambda c: c["name"]),
        }
        for letter, items in sorted(grouped.items())
    ]

    total = sum(len(group["characters"]) for group in groups)
    return {
        "title": title or "星穹铁道角色目录",
        "subtitle": subtitle or f"已适配 {total} 名角色 · 名称与别名均可用于查询",
        "total": total,
        "groups": groups,
    }


def response_text(response):
    if not isinstance(response, dict):
        return ""
    if isinstance(response.get("body_text"), str):
        return response["body_text"]
    encoded = response.get("body_base64")
    if isinstance(encoded, str) and encoded:
        try:
            return base64.b64decode(encoded).decode("utf-8")
        except Exception:
            return ""
    return ""


def response_bytes(response):
    if not isinstance(response, dict):
        return b""
    encoded = response.get("body_base64")
    if isinstance(encoded, str) and encoded:
        try:
            return base64.b64decode(encoded)
        except Exception:
            return b""
    text = response.get("body_text")
    if isinstance(text, str):
        return text.encode("utf-8")
    return b""


def is_success_response(response):
    try:
        status = int(response.get("status_code"))
    except Exception:
        return False
    return 200 <= status < 300


class CharacterIndex:
    def __init__(self, characters):
        self._characters = list(characters)
        self._by_alias = {}
        for item in self._characters:
            name = str(item.get("name") or "").strip()
            if not name:
                continue
            aliases = [name] + [str(alias) for alias in item.get("aliases") or []]
            for alias in aliases:
                key = normalize_key(alias)
                if key:
                    self._by_alias[key] = item

    @classmethod
    def load(cls, path):
        with Path(path).open("r", encoding="utf-8") as handle:
            document = json.load(handle)
        return cls(document.get("characters") or [])

    def resolve(self, query):
        key = normalize_key(query)
        item = self._by_alias.get(key)
        if item:
            return {
                "name": str(item["name"]).strip(),
                "slug": str(item.get("slug") or key).strip() or key,
                "aliases": [str(alias) for alias in item.get("aliases") or []],
                "matched": True,
            }
        fallback = str(query or "").strip()
        fallback_key = normalize_key(fallback)
        return {
            "name": fallback,
            "slug": fallback_key or hashlib.sha1(fallback.encode("utf-8")).hexdigest()[:12],
            "aliases": [],
            "matched": False,
        }

    def all(self):
        """Return every indexed character with name, slug and aliases."""
        result = []
        for item in self._characters:
            name = str(item.get("name") or "").strip()
            if not name:
                continue
            result.append({
                "name": name,
                "slug": str(item.get("slug") or "").strip() or name,
                "aliases": [str(alias) for alias in item.get("aliases") or [] if str(alias).strip()],
            })
        return result


class GameGuideService:
    def __init__(
        self,
        plugin_dir=None,
        characters_path=None,
        cache_root=None,
        max_sources=DEFAULT_MAX_SOURCES,
        max_images_per_source=DEFAULT_MAX_IMAGES_PER_SOURCE,
        max_total_images=DEFAULT_MAX_TOTAL_IMAGES,
        forward_images_per_message=DEFAULT_FORWARD_IMAGES_PER_MESSAGE,
        forward_send_timeout_seconds=DEFAULT_FORWARD_SEND_TIMEOUT_SECONDS,
    ):
        self.plugin_dir = Path(plugin_dir or Path(__file__).resolve().parents[1])
        self.characters = CharacterIndex.load(characters_path or self.plugin_dir / "data" / "characters.json")
        self.cache_root = Path(cache_root or self.plugin_dir.parents[2] / "cache" / "game_guide")
        self.max_sources = max_sources
        self.max_images_per_source = max_images_per_source
        self.max_total_images = max_total_images
        self.forward_images_per_message = max(1, int(forward_images_per_message or DEFAULT_FORWARD_IMAGES_PER_MESSAGE))
        self.forward_send_timeout_seconds = max(
            1,
            int(forward_send_timeout_seconds or DEFAULT_FORWARD_SEND_TIMEOUT_SECONDS),
        )

    def handle_message(self, ctx):
        if getattr(ctx, "event_type", "") not in {"message.group", "message.private"}:
            ctx.send_result({"handled": False})
            return

        if parse_character_list_request(
            getattr(ctx, "plain_text", ""),
            trigger_prefixes=getattr(ctx, "command_prefixes", []),
        ):
            self.handle_character_list(ctx)
            return

        requested = parse_game_guide_command(getattr(ctx, "command", None), getattr(ctx, "args", []))
        if requested is None:
            requested = parse_guide_request(
                getattr(ctx, "plain_text", ""),
                command_prefixes=getattr(ctx, "command_prefixes", []),
            )
        if requested is None:
            ctx.send_result({"handled": False})
            return

        character = self.characters.resolve(requested)
        if not character["name"]:
            ctx.send_text("请在攻略前写角色名，例如「*昔涟攻略」。")
            return

        log_info(ctx, "游戏攻略开始查询", {
            "query": requested,
            "character": character["name"],
            "matched_alias": character["matched"],
            "target_type": getattr(ctx, "target_type", ""),
            "target_id": getattr(ctx, "target_id", ""),
        })

        images, from_cache = self.load_cached_images(character)
        if images:
            log_info(ctx, "游戏攻略命中缓存", {
                "character": character["name"],
                "images": len(images),
                "cache_dir": str(self.guide_dir(character)),
            })
        if not images:
            log_info(ctx, "游戏攻略缓存未命中，开始刷新", {
                "character": character["name"],
                "cache_dir": str(self.guide_dir(character)),
            })
            legacy_images, legacy_from_cache = self.scan_cached_images(character)
            if legacy_images:
                log_info(ctx, "游戏攻略发现旧缓存", {
                    "character": character["name"],
                    "images": len(legacy_images),
                })
            images = self.refresh_cache(ctx, character)
            from_cache = False
            if not images and legacy_images:
                images = legacy_images
                from_cache = legacy_from_cache

        if not images:
            log_warn(ctx, "游戏攻略没有可发送图片", {
                "character": character["name"],
            })
            ctx.send_text(f"没有找到「{character['name']}」的星穹铁道攻略图。")
            return

        self.send_images(ctx, character, images, from_cache)
        ctx.send_result({
            "handled": True,
            "character": character["name"],
            "images": len(images),
            "from_cache": from_cache,
        })

    def handle_character_list(self, ctx):
        characters = self.characters.all()
        render_data = build_character_list_render_data(characters)
        log_info(ctx, "游戏攻略生成角色列表图片", {
            "total": render_data["total"],
            "groups": len(render_data["groups"]),
        })
        try:
            result = ctx.render_image(
                "character-list",
                render_data,
                output="png",
                fallback_text=f"当前已适配 {render_data['total']} 名角色，发送「*角色名攻略」查看角色攻略图。",
            )
        except Exception as exc:
            log_warn(ctx, "游戏攻略角色列表渲染失败", {"error": str(exc)})
            ctx.send_text("角色列表图片生成失败，请稍后再试。")
            ctx.send_result({"handled": True, "command": "character-list", "error": str(exc)})
            return

        image_path = str(result.get("image_path") or "").strip()
        if not image_path:
            log_warn(ctx, "游戏攻略角色列表图片结果缺少路径")
            ctx.send_text("角色列表图片生成失败，请稍后再试。")
            ctx.send_result({"handled": True, "command": "character-list", "error": "missing image_path"})
            return

        ctx.send_message([{"type": "image", "data": {"file": image_path}}])
        log_info(ctx, "游戏攻略角色列表已发送", {
            "total": render_data["total"],
        })
        ctx.send_result({
            "handled": True,
            "command": "character-list",
            "total": render_data["total"],
        })

    def send_images(self, ctx, character, images, from_cache):
        forward_sender = getattr(ctx, "message_forward_send", None)
        if callable(forward_sender):
            batches = list(chunked(images, self.forward_images_per_message))
            log_info(ctx, "游戏攻略开始发送合并转发", {
                "character": character["name"],
                "images": len(images),
                "batches": len(batches),
                "from_cache": from_cache,
            })
            for batch_index, batch in enumerate(batches, start=1):
                log_info(ctx, "游戏攻略发送合并转发分批", {
                    "character": character["name"],
                    "batch": batch_index,
                    "batches": len(batches),
                    "images": len(batch),
                })
                forward_sender(
                    getattr(ctx, "target_type", ""),
                    getattr(ctx, "target_id", ""),
                    build_forward_messages(character, batch, getattr(ctx, "bot_id", "")),
                    timeout_seconds=self.forward_send_timeout_seconds,
                )
            log_info(ctx, "游戏攻略合并转发发送完成", {
                "character": character["name"],
                "images": len(images),
                "batches": len(batches),
            })
            return

        log_info(ctx, "游戏攻略开始发送普通图片消息", {
            "character": character["name"],
            "images": len(images),
            "from_cache": from_cache,
        })
        ctx.send_message([
            {
                "type": "image",
                "data": {"file": image["file_uri"]},
            }
            for image in images
        ])

    def load_cached_images(self, character):
        index_path = self.cache_index_path(character)
        if not index_path.exists():
            return [], False
        try:
            document = json.loads(index_path.read_text(encoding="utf-8"))
        except Exception:
            return [], False

        images = []
        for source in document.get("sources") or []:
            for image in source.get("images") or []:
                rel_file = str(image.get("file") or "").strip()
                if not rel_file:
                    continue
                path = self.cache_root / rel_file
                if path.is_file():
                    images.append({"file_uri": path.resolve().as_uri(), "path": str(path)})
                if len(images) >= self.max_total_images:
                    return images, True
        if images:
            return images, True
        return [], False

    def scan_cached_images(self, character):
        guide_dir = self.guide_dir(character)
        if not guide_dir.is_dir():
            return [], False
        images = []
        for path in sorted(guide_dir.iterdir(), key=lambda item: item.name):
            if not path.is_file() or path.suffix.lower() not in SUPPORTED_CACHE_EXTENSIONS:
                continue
            images.append({"file_uri": path.resolve().as_uri(), "path": str(path)})
            if len(images) >= self.max_total_images:
                break
        return images, bool(images)

    def refresh_cache(self, ctx, character):
        log_info(ctx, "游戏攻略开始搜索来源", {
            "character": character["name"],
            "max_sources": self.max_sources,
        })
        sources = self.search_sources(ctx, character)
        if not sources:
            log_warn(ctx, "游戏攻略没有搜索到来源", {
                "character": character["name"],
            })
            return []

        guide_dir = self.guide_dir(character)
        guide_dir.mkdir(parents=True, exist_ok=True)
        record = {
            "schema_version": 1,
            "game": "honkai_star_rail",
            "character": character["name"],
            "updated_at": datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z"),
            "sources": [],
        }
        images = []
        for source_index, source in enumerate(sources):
            source_images = source["images"][:self.max_images_per_source]
            log_info(ctx, "游戏攻略开始下载来源图片", {
                "character": character["name"],
                "source": source_index + 1,
                "post_id": source["post_id"],
                "title": source["title"],
                "image_candidates": len(source_images),
            })
            source_record = {
                "post_id": source["post_id"],
                "title": source["title"],
                "author": source["author"],
                "url": source["url"],
                "images": [],
            }
            before_count = len(images)
            for image_index, image_url in enumerate(source_images):
                content = self.download_image(ctx, image_url)
                if not content:
                    log_warn(ctx, "游戏攻略图片下载失败", {
                        "character": character["name"],
                        "source": source_index + 1,
                        "post_id": source["post_id"],
                        "image": image_index + 1,
                        "host": urlparse(image_url).hostname or "",
                    })
                    continue
                path = self.image_cache_path(character, source_index, source["post_id"], image_index, image_url)
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_bytes(content)
                rel_file = path.relative_to(self.cache_root).as_posix()
                source_record["images"].append({"url": image_url, "file": rel_file})
                images.append({"file_uri": path.resolve().as_uri(), "path": str(path)})
                if len(images) >= self.max_total_images:
                    break
            if source_record["images"]:
                record["sources"].append(source_record)
            log_info(ctx, "游戏攻略来源图片下载完成", {
                "character": character["name"],
                "source": source_index + 1,
                "post_id": source["post_id"],
                "downloaded": len(images) - before_count,
            })
            if len(images) >= self.max_total_images:
                break

        if images:
            self.cache_index_path(character).write_text(
                json.dumps(record, ensure_ascii=False, indent=2) + "\n",
                encoding="utf-8",
            )
        return images

    def search_sources(self, ctx, character):
        query_terms = [character["name"]]
        query_terms.extend(alias for alias in character.get("aliases") or [] if alias)

        sources = []
        seen_posts = set()
        seen_images = set()
        for term in query_terms[:3]:
            url = self.search_url(f"{term}攻略")
            log_info(ctx, "游戏攻略请求米游社搜索", {
                "character": character["name"],
                "term": term,
            })
            try:
                response = ctx.http_request("GET", url, headers=DEFAULT_HEADERS, timeout_seconds=15)
            except Exception as exc:
                log_warn(ctx, "游戏攻略米游社搜索请求失败", {
                    "character": character["name"],
                    "term": term,
                    "error": str(exc),
                })
                continue
            if not is_success_response(response):
                log_warn(ctx, "游戏攻略米游社搜索返回异常", {
                    "character": character["name"],
                    "term": term,
                    "status_code": response.get("status_code") if isinstance(response, dict) else "",
                })
                continue
            document = self.parse_search_document(response_text(response))
            candidates = self.sources_from_document(document, character)
            log_info(ctx, "游戏攻略米游社搜索完成", {
                "character": character["name"],
                "term": term,
                "sources": len(candidates),
            })
            for source in candidates:
                if source["post_id"] in seen_posts:
                    continue
                detailed_images = self.fetch_post_images(ctx, source["post_id"])
                if detailed_images:
                    source["images"] = detailed_images
                filtered_images = []
                for image_url in source["images"]:
                    if image_url in seen_images:
                        continue
                    filtered_images.append(image_url)
                    seen_images.add(image_url)
                if not filtered_images:
                    continue
                source["images"] = filtered_images
                sources.append(source)
                seen_posts.add(source["post_id"])
                if len(sources) >= self.max_sources:
                    return sources
            if sources:
                return sources
        return sources

    def fetch_post_images(self, ctx, post_id):
        post_id = str(post_id or "").strip()
        if not post_id:
            return []
        log_info(ctx, "游戏攻略读取帖子详情", {"post_id": post_id})
        try:
            response = ctx.http_request("GET", self.detail_url(post_id), headers=DEFAULT_HEADERS, timeout_seconds=15)
        except Exception as exc:
            log_warn(ctx, "游戏攻略帖子详情请求失败", {
                "post_id": post_id,
                "error": str(exc),
            })
            return []
        if not is_success_response(response):
            log_warn(ctx, "游戏攻略帖子详情返回异常", {
                "post_id": post_id,
                "status_code": response.get("status_code") if isinstance(response, dict) else "",
            })
            return []
        images = image_urls_from_post_detail(self.parse_detail_document(response_text(response)))
        log_info(ctx, "游戏攻略帖子详情读取完成", {
            "post_id": post_id,
            "images": len(images),
        })
        return images

    def search_url(self, keyword):
        params = (
            f"gids={STAR_RAIL_GIDS}"
            f"&keyword={quote(keyword)}"
            f"&page_size={max(self.max_sources * 4, 8)}"
        )
        return f"{SEARCH_ENDPOINT}?{params}"

    def detail_url(self, post_id):
        return f"{DETAIL_ENDPOINT}?post_id={quote(str(post_id))}"

    def parse_search_document(self, body_text):
        try:
            document = json.loads(body_text)
        except Exception:
            return {}
        if document.get("retcode") != 0:
            return {}
        data = document.get("data")
        return data if isinstance(data, dict) else {}

    def parse_detail_document(self, body_text):
        try:
            document = json.loads(body_text)
        except Exception:
            return {}
        if document.get("retcode") != 0:
            return {}
        data = document.get("data")
        return data if isinstance(data, dict) else {}

    def sources_from_document(self, document, character):
        raw_posts = document.get("posts") or document.get("list") or []
        sources = []
        for item in raw_posts:
            if not isinstance(item, dict):
                continue
            post = item.get("post") if isinstance(item.get("post"), dict) else item
            if not isinstance(post, dict) or int_value(post.get("game_id")) != STAR_RAIL_GIDS:
                continue
            forum_id = int_value(post.get("f_forum_id"))
            if forum_id and forum_id != STAR_RAIL_GUIDE_FORUM_ID:
                continue
            if not self.post_matches_character(post, character):
                continue
            images = image_urls_from_post(post)
            if not images:
                continue
            user = item.get("user") if isinstance(item.get("user"), dict) else {}
            post_id = str(post.get("post_id") or "").strip()
            sources.append({
                "post_id": post_id,
                "title": str(post.get("subject") or "").strip(),
                "author": str(user.get("nickname") or "").strip(),
                "url": f"https://www.miyoushe.com/sr/article/{post_id}" if post_id else "",
                "images": images,
            })
        return sources

    def post_matches_character(self, post, character):
        subject = str(post.get("subject") or "")
        content = str(post.get("content") or "")
        haystack = normalize_key(subject + " " + content)
        if normalize_key("攻略") not in haystack:
            return False
        terms = [character["name"]] + list(character.get("aliases") or [])
        return any(normalize_key(term) and normalize_key(term) in haystack for term in terms)

    def download_image(self, ctx, url):
        host = urlparse(url).hostname or ""
        if host.lower() not in SUPPORTED_IMAGE_HOSTS:
            return b""
        try:
            response = ctx.http_request("GET", url, headers=image_headers(url), timeout_seconds=30)
        except Exception:
            return b""
        if not is_success_response(response):
            return b""
        return response_bytes(response)

    def cache_index_path(self, character):
        return self.guide_dir(character) / "index.json"

    def guide_dir(self, character):
        return self.cache_root / "guides" / sanitize_path_part(character["slug"])

    def image_cache_path(self, character, source_index, post_id, image_index, image_url):
        digest = hashlib.sha1(image_url.encode("utf-8")).hexdigest()[:12]
        ext = image_extension(image_url)
        name = f"{source_index + 1:02d}_{sanitize_path_part(post_id or 'post')}_{image_index + 1:02d}_{digest}{ext}"
        return self.guide_dir(character) / name


def int_value(value):
    try:
        return int(value)
    except Exception:
        return 0


def image_urls_from_post(post):
    urls = []
    for raw_url in post.get("images") or []:
        add_image_url(urls, raw_url)
    if not urls:
        add_image_url(urls, post.get("cover"))
    return urls


def image_urls_from_post_detail(document):
    post_bundle = document.get("post") if isinstance(document.get("post"), dict) else {}
    post = post_bundle.get("post") if isinstance(post_bundle.get("post"), dict) else post_bundle

    urls = []
    for raw_url in post.get("images") or []:
        add_image_url(urls, raw_url)

    for item in post_bundle.get("image_list") or []:
        if isinstance(item, dict):
            add_image_url(urls, item.get("url"))

    add_image_urls_from_structured_content(urls, post.get("structured_content"))
    add_image_urls_from_html(urls, post.get("content"))
    add_image_url(urls, post.get("cover"))

    cover = post_bundle.get("cover")
    if isinstance(cover, dict):
        add_image_url(urls, cover.get("url"))
    return urls


def add_image_urls_from_structured_content(urls, raw_content):
    if not isinstance(raw_content, str) or not raw_content:
        return
    try:
        document = json.loads(raw_content)
    except Exception:
        return
    if not isinstance(document, list):
        return
    for item in document:
        if not isinstance(item, dict):
            continue
        insert = item.get("insert")
        if isinstance(insert, dict):
            add_image_url(urls, insert.get("image"))


def add_image_urls_from_html(urls, raw_content):
    if not isinstance(raw_content, str) or not raw_content:
        return
    for match in re.finditer(r'<img[^>]+src=["\']([^"\']+)["\']', raw_content):
        add_image_url(urls, match.group(1))


def add_image_url(urls, raw_url):
    url = str(raw_url or "").strip()
    if not url:
        return
    parsed = urlparse(url)
    if parsed.scheme not in {"http", "https"}:
        return
    if (parsed.hostname or "").lower() not in SUPPORTED_IMAGE_HOSTS:
        return
    if url not in urls:
        urls.append(url)


def image_headers(image_url):
    return {
        "Accept": "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8",
        "Referer": "https://www.miyoushe.com/sr/",
        "User-Agent": DEFAULT_HEADERS["User-Agent"],
    }


def image_extension(url):
    suffix = Path(urlparse(url).path).suffix.lower()
    if suffix in {".jpg", ".jpeg", ".png", ".webp", ".gif"}:
        return ".jpg" if suffix == ".jpeg" else suffix
    return ".jpg"


def sanitize_path_part(value):
    text = normalize_key(value)
    if not text:
        text = hashlib.sha1(str(value).encode("utf-8")).hexdigest()[:12]
    return re.sub(r"[^a-z0-9\u4e00-\u9fff]+", "-", text).strip("-")[:80] or "guide"


def chunked(items, size):
    for index in range(0, len(items), size):
        yield items[index:index + size]


def build_forward_messages(character, images, bot_id):
    uin = str(bot_id or "10000")
    name = f"{character['name']}攻略"
    return [
        {
            "type": "node",
            "data": {
                "name": name,
                "uin": uin,
                "content": [{
                    "type": "image",
                    "data": {"file": image["file_uri"]},
                }],
            },
        }
        for image in images
    ]


def log_info(ctx, message, fields=None):
    write_log(ctx, "info", message, fields)


def log_warn(ctx, message, fields=None):
    write_log(ctx, "warn", message, fields)


def write_log(ctx, level, message, fields=None):
    logger = getattr(ctx, "logger_write", None)
    if not callable(logger):
        return
    try:
        logger(level, message, fields or {})
    except Exception:
        return
