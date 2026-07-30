"""Account-driven Bilibili source used by the subscription hub."""

import hashlib
import json
import time
from urllib.parse import parse_qsl, quote, urlencode, urlparse, urlunparse

from business.http_utils import bilibili_diagnostic_text
from business.thirdparty_accounts import read_thirdparty_accounts

from .api import dynamic_updates, live_update, parse_json_response


DYNAMIC_FEED_URL = "https://api.bilibili.com/x/polymer/web-dynamic/v1/feed/all"
LIVE_STATUS_URL = "https://api.live.bilibili.com/room/v1/Room/get_status_info_by_uids"
NAV_URL = "https://api.bilibili.com/x/web-interface/nav"
RELATION_URL = "https://api.bilibili.com/x/relation"
FOLLOW_URL = "https://api.bilibili.com/x/relation/modify"

WBI_MIXIN_KEY_ORDER = (
    46, 47, 18, 2, 53, 8, 23, 32, 15, 50, 10, 31, 58, 3, 45, 35,
    27, 43, 5, 49, 33, 9, 42, 19, 29, 28, 14, 39, 12, 38, 41, 13,
    37, 48, 7, 16, 24, 55, 40, 61, 26, 17, 0, 1, 60, 51, 30, 4,
    22, 25, 54, 21, 56, 59, 6, 63, 57, 62, 11, 36, 20, 34, 44, 52,
)

USER_AGENT = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36"
DM_IMG_STR = "V2ViR0wgMS4wIChPcGVuR0wgRVMgMi4wIENocm9taXVtKQ"
DM_COVER_IMG_STR = (
    "R29vZ2xlIEluYy4gKEludGVsKUFOR0xFIChJbnRlbCwgSW50ZWwoUikgVUhEIEdyYXBoaWNz"
    "IERpcmVjdDNEMTEgdnNfNV8wIHBzXzVfMCwgRDNEMTEp"
)

RISK_CODES = {-412, -352, 352}
RATE_LIMIT_CODES = {-509, -799}
AUTH_CODES = {-101, -102, -658}
FOLLOW_CHECK_INTERVAL_SECONDS = 6 * 60 * 60
WBI_CACHE_SECONDS = 12 * 60 * 60
COOLDOWN_BASE_SECONDS = 5 * 60
COOLDOWN_MAX_SECONDS = 60 * 60


def storage_value(result, fallback=None):
    if isinstance(result, dict) and "value" in result:
        return result.get("value")
    return fallback


def account_key(account):
    account_id = str(account.get("account_id") or "").strip()
    if account_id:
        return account_id
    cookie = str(account.get("cookie") or "")
    return "cookie-" + hashlib.sha256(cookie.encode("utf-8")).hexdigest()[:12]


def cookie_value(cookie, key):
    for part in str(cookie or "").split(";"):
        name, separator, value = part.strip().partition("=")
        if separator and name.strip() == key:
            return value.strip()
    return ""


def extract_wbi_key(value):
    path = urlparse(str(value or "").strip()).path
    filename = path.rsplit("/", 1)[-1]
    return filename.rsplit(".", 1)[0].strip()


def wbi_mixin_key(img_key, sub_key):
    raw = (str(img_key or "").strip() + str(sub_key or "").strip())
    if len(raw) < len(WBI_MIXIN_KEY_ORDER):
        return ""
    return "".join(raw[index] for index in WBI_MIXIN_KEY_ORDER)[:32]


def sanitize_wbi_value(value):
    return "".join(char for char in str(value) if char not in "!'()*")


def sign_wbi_url(raw_url, img_key, sub_key, timestamp):
    mixin = wbi_mixin_key(img_key, sub_key)
    if not mixin:
        raise BilibiliSourceError("signature", "WBI 签名密钥不可用。")
    parsed = urlparse(raw_url)
    values = [(key, sanitize_wbi_value(value)) for key, value in parse_qsl(parsed.query, keep_blank_values=True) if key != "w_rid"]
    values.append(("wts", str(int(timestamp))))
    values.sort(key=lambda item: (item[0], item[1]))
    query = urlencode(values)
    digest = hashlib.md5((query + mixin).encode("utf-8")).hexdigest()
    return urlunparse(parsed._replace(query=query + "&w_rid=" + digest))


def dynamic_feed_url():
    query = urlencode({
        "timezone_offset": -480,
        "type": "all",
        "page": 1,
        "dm_img_list": "[]",
        "dm_img_str": DM_IMG_STR,
        "dm_cover_img_str": DM_COVER_IMG_STR,
        "dm_img_inter": json.dumps({"ds": [], "wh": [0, 0, 0], "of": [0, 0, 0]}, separators=(",", ":")),
    })
    return DYNAMIC_FEED_URL + "?" + query


def live_status_url(uids):
    query = "&".join("uids[]=" + quote(str(uid), safe="") for uid in uids)
    return LIVE_STATUS_URL + ("?" + query if query else "")


class BilibiliSourceError(Exception):
    def __init__(self, kind, message, code=0, http_status=0):
        super().__init__(message)
        self.kind = kind
        self.code = int(code or 0)
        self.http_status = int(http_status or 0)

    @property
    def cooldown(self):
        return self.kind in {"risk_control", "rate_limit"}


class BilibiliSourceClient:
    def __init__(self, ctx, now=None):
        self.ctx = ctx
        self.now = now or time.time

    def poll(self, subscriptions):
        watched = self._watched_subjects(subscriptions)
        result = {
            "checked": len(watched),
            "updates": [],
            "errors": [],
            "account_count": 0,
            "dynamic_ok": not any(item["dynamic"] for item in watched.values()),
            "live_ok": not any(item["live"] for item in watched.values()),
        }
        if not watched:
            return result

        accounts, error = read_thirdparty_accounts(self.ctx, "bilibili")
        result["account_count"] = len(accounts)
        if not accounts:
            result["errors"].append(error or "没有可用的 Bilibili 账号 CK。")
            return result

        dynamic_uids = sorted(uid for uid, item in watched.items() if item["dynamic"])
        live_uids = sorted(uid for uid, item in watched.items() if item["live"])
        if dynamic_uids:
            updates, errors, ok = self._poll_dynamics(dynamic_uids, accounts)
            result["updates"].extend(updates)
            result["errors"].extend(errors)
            result["dynamic_ok"] = ok
        if live_uids:
            updates, errors, ok = self._poll_live(live_uids, accounts)
            result["updates"].extend(updates)
            result["errors"].extend(errors)
            result["live_ok"] = ok
        return result

    def _watched_subjects(self, subscriptions):
        watched = {}
        for subscription in subscriptions:
            uid = str(subscription.get("uid") or "").strip()
            if not uid:
                continue
            services = {str(value or "").strip() for value in subscription.get("services") or []}
            item = watched.setdefault(uid, {"dynamic": False, "live": False})
            item["live"] = item["live"] or "all" in services or "live" in services
            item["dynamic"] = item["dynamic"] or "all" in services or bool(services - {"live"})
        return watched

    def _ordered_accounts(self, accounts):
        offset = int(storage_value(self.ctx.storage_get("source:bilibili:account_offset"), 0) or 0)
        start = offset % len(accounts)
        return accounts[start:] + accounts[:start]

    def _advance_account(self, accounts, selected):
        selected_key = account_key(selected)
        index = next((index for index, account in enumerate(accounts) if account_key(account) == selected_key), 0)
        self.ctx.storage_set("source:bilibili:account_offset", (index + 1) % len(accounts))

    def _poll_dynamics(self, uids, accounts):
        errors = []
        skipped_delays = []
        for account in self._ordered_accounts(accounts):
            delay = self._cooldown_remaining("dynamic", account)
            if delay > 0:
                skipped_delays.append(delay)
                continue
            try:
                errors.extend(self._ensure_following(account, uids))
                document = self._request_json("GET", dynamic_feed_url(), account, signed=True)
                updates = []
                watched = set(uids)
                for update in dynamic_updates(document):
                    author = update.get("author") if isinstance(update.get("author"), dict) else {}
                    uid = str(author.get("uid") or "").strip()
                    if uid not in watched:
                        continue
                    update["uid"] = uid
                    updates.append(update)
                self._clear_cooldown("dynamic", account)
                self._advance_account(accounts, account)
                return updates, errors, True
            except BilibiliSourceError as exc:
                self._record_source_error("dynamic", account, exc)
                if exc.cooldown:
                    self._remember_cooldown("dynamic", account, exc)
                errors.append(self._friendly_error("动态", exc))
            except Exception as exc:
                self._log_error("dynamic", account, exc)
                errors.append("Bilibili 动态检查失败。")
        if skipped_delays and not errors:
            errors.append(f"Bilibili 动态检查因平台风控暂停，剩余约 {max(1, int(min(skipped_delays) / 60 + 0.99))} 分钟。")
        return [], self._dedupe(errors), False

    def _poll_live(self, uids, accounts):
        errors = []
        skipped_delays = []
        for account in self._ordered_accounts(accounts):
            delay = self._cooldown_remaining("live", account)
            if delay > 0:
                skipped_delays.append(delay)
                continue
            try:
                document = self._request_json("GET", live_status_url(uids), account, live=True)
                updates = self._live_transitions(document, uids)
                self._clear_cooldown("live", account)
                self._advance_account(accounts, account)
                return updates, [], True
            except BilibiliSourceError as exc:
                self._record_source_error("live", account, exc)
                if exc.cooldown:
                    self._remember_cooldown("live", account, exc)
                errors.append(self._friendly_error("直播", exc))
            except Exception as exc:
                self._log_error("live", account, exc)
                errors.append("Bilibili 直播检查失败。")
        if skipped_delays and not errors:
            errors.append(f"Bilibili 直播检查因平台风控暂停，剩余约 {max(1, int(min(skipped_delays) / 60 + 0.99))} 分钟。")
        return [], self._dedupe(errors), False

    def _ensure_following(self, account, uids):
        errors = []
        csrf = cookie_value(account.get("cookie"), "bili_jct")
        if not csrf:
            return ["Bilibili 账号 CK 缺少 bili_jct，无法自动关注订阅账号。"]
        own_uid = str((account.get("profile") or {}).get("uid") or "").strip()
        for uid in uids:
            if uid == own_uid:
                continue
            key = f"source:bilibili:follow:{account_key(account)}:{uid}"
            state = storage_value(self.ctx.storage_get(key), {})
            checked_at = float(state.get("checked_at") or 0) if isinstance(state, dict) else 0
            if checked_at and self.now() - checked_at < FOLLOW_CHECK_INTERVAL_SECONDS:
                continue
            try:
                relation = self._request_json("GET", RELATION_URL + "?" + urlencode({"fid": uid}), account, signed=True)
                data = relation.get("data") if isinstance(relation, dict) else {}
                following = int(data.get("attribute") or 0) > 0 if isinstance(data, dict) else False
                if not following:
                    body = urlencode({"fid": uid, "act": "1", "re_src": "11", "csrf": csrf})
                    self._request_json("POST", FOLLOW_URL, account, signed=True, body_text=body)
                    following = True
                self.ctx.storage_set(key, {"checked_at": int(self.now()), "following": following})
            except BilibiliSourceError as exc:
                if exc.cooldown or exc.kind == "auth":
                    raise
                errors.append(f"Bilibili 订阅账号 {uid} 自动关注失败。")
                self._record_source_error("auto_follow", account, exc, uid=uid)
            except Exception as exc:
                errors.append(f"Bilibili 订阅账号 {uid} 自动关注失败。")
                self._log_error("auto_follow", account, exc, uid=uid)
        return errors

    def _live_transitions(self, document, uids):
        data = document.get("data") if isinstance(document, dict) else {}
        if not isinstance(data, dict):
            return []
        updates = []
        for uid in uids:
            entry = data.get(uid)
            if not isinstance(entry, dict):
                continue
            update = live_update(document, uid)
            if not update:
                continue
            status = int(update.get("live_status") or 0)
            session = str(update.get("pub_ts") or update.get("room_id") or "unknown")
            key = f"source:bilibili:live:{uid}"
            previous = storage_value(self.ctx.storage_get(key), None)
            current = {"status": status, "session": session, "room_id": str(update.get("room_id") or "")}
            self.ctx.storage_set(key, current)
            if not isinstance(previous, dict):
                continue
            previous_status = int(previous.get("status") or 0)
            previous_session = str(previous.get("session") or "unknown")
            if previous_status == status and (status == 0 or previous_session == session):
                continue
            if status == 1:
                update["id"] = f"live-{uid}-started-{session}"
                update["live_event"] = "started"
            else:
                update["id"] = f"live-{uid}-ended-{previous_session}"
                update["live_event"] = "ended"
            update["uid"] = uid
            updates.append(update)
        return updates

    def _request_json(self, method, url, account, signed=False, body_text=None, live=False, allow_signature_retry=True):
        request_url = self._signed_url(url, account) if signed else url
        headers = self._headers(account.get("cookie"), live=live, form=body_text is not None)
        response = self.ctx.http_request(
            method,
            request_url,
            headers=headers,
            timeout_seconds=30,
            body_text=body_text,
        )
        status = int(response.get("status_code") or 0) if isinstance(response, dict) else 0
        document = parse_json_response(response)
        if not isinstance(document, dict) or "code" not in document:
            kind = self._error_kind(status, 0)
            if kind == "upstream":
                kind = "invalid_response"
            raise BilibiliSourceError(
                kind,
                bilibili_diagnostic_text(response, document),
                http_status=status,
            )
        code = int(document.get("code") or 0) if isinstance(document, dict) else 0
        if status < 200 or status >= 300 or code != 0:
            error = BilibiliSourceError(
                self._error_kind(status, code),
                bilibili_diagnostic_text(response, document),
                code,
                status,
            )
            if signed and allow_signature_retry and error.kind == "signature" and body_text is None:
                self._invalidate_wbi(account)
                return self._request_json(
                    method,
                    url,
                    account,
                    signed=True,
                    body_text=body_text,
                    live=live,
                    allow_signature_retry=False,
                )
            raise error
        return document

    def _signed_url(self, url, account):
        key = self._wbi_cache_key(account)
        cached = storage_value(self.ctx.storage_get(key), {})
        now = self.now()
        if not isinstance(cached, dict) or float(cached.get("expires_at") or 0) <= now:
            response = self._request_json("GET", NAV_URL, account)
            data = response.get("data") if isinstance(response, dict) else {}
            wbi_img = data.get("wbi_img") if isinstance(data, dict) and isinstance(data.get("wbi_img"), dict) else {}
            cached = {
                "img_key": extract_wbi_key(wbi_img.get("img_url")),
                "sub_key": extract_wbi_key(wbi_img.get("sub_url")),
                "expires_at": int(now + WBI_CACHE_SECONDS),
            }
            if not cached["img_key"] or not cached["sub_key"]:
                raise BilibiliSourceError("signature", "Bilibili WBI 签名密钥缺失。")
            self.ctx.storage_set(key, cached)
        return sign_wbi_url(url, cached.get("img_key"), cached.get("sub_key"), now)

    def _invalidate_wbi(self, account):
        self.ctx.storage_delete(self._wbi_cache_key(account))

    def _wbi_cache_key(self, account):
        return f"source:bilibili:wbi:{account_key(account)}"

    def _headers(self, cookie, live=False, form=False):
        origin = "https://live.bilibili.com" if live else "https://www.bilibili.com"
        headers = {
            "Accept": "application/json, text/plain, */*",
            "Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
            "User-Agent": USER_AGENT,
            "Referer": origin + "/",
            "Origin": origin,
            "DNT": "1",
            "Sec-GPC": "1",
            "Sec-CH-UA": '"Chromium";v="134", "Google Chrome";v="134", "Not?A_Brand";v="99"',
            "Sec-CH-UA-Mobile": "?0",
            "Sec-CH-UA-Platform": '"Windows"',
            "Sec-Fetch-Dest": "empty",
            "Sec-Fetch-Mode": "cors",
            "Sec-Fetch-Site": "same-site",
            "Cookie": str(cookie or "").strip(),
        }
        if form:
            headers["Content-Type"] = "application/x-www-form-urlencoded"
        return headers

    def _cooldown_key(self, scope, account):
        return f"source:bilibili:cooldown:{scope}:{account_key(account)}"

    def _cooldown_remaining(self, scope, account):
        state = storage_value(self.ctx.storage_get(self._cooldown_key(scope, account)), {})
        until = float(state.get("until") or 0) if isinstance(state, dict) else 0
        return max(0, until - self.now())

    def _remember_cooldown(self, scope, account, error):
        key = self._cooldown_key(scope, account)
        previous = storage_value(self.ctx.storage_get(key), {})
        attempts = int(previous.get("attempts") or 0) + 1 if isinstance(previous, dict) else 1
        delay = min(COOLDOWN_BASE_SECONDS * (2 ** (attempts - 1)), COOLDOWN_MAX_SECONDS)
        self.ctx.storage_set(key, {
            "attempts": attempts,
            "until": int(self.now() + delay),
            "kind": error.kind,
            "code": error.code,
        })

    def _clear_cooldown(self, scope, account):
        key = self._cooldown_key(scope, account)
        if storage_value(self.ctx.storage_get(key), None) is not None:
            self.ctx.storage_delete(key)

    def _record_source_error(self, scope, account, error, **fields):
        self._log_error(scope, account, error, kind=error.kind, code=error.code, http_status=error.http_status, **fields)

    def _log_error(self, scope, account, error, **fields):
        payload = {"scope": scope, "account_id": account_key(account), "error": str(error), **fields}
        try:
            self.ctx.logger_write("warn", "Bilibili 订阅源检查失败", payload)
        except Exception:
            pass

    def _friendly_error(self, label, error):
        if error.kind == "risk_control":
            return f"Bilibili {label}检查被风控拦截，已切换账号或进入退避。"
        if error.kind == "rate_limit":
            return f"Bilibili {label}检查触发频率限制，已切换账号或进入退避。"
        if error.kind == "auth":
            return f"Bilibili {label}检查失败：账号 CK 已失效，请重新扫码。"
        if error.kind == "signature":
            return f"Bilibili {label}检查失败：WBI 签名不可用。"
        return f"Bilibili {label}检查失败。"

    @staticmethod
    def _error_kind(status, code):
        if code in {-403, 403}:
            return "signature"
        if code in RISK_CODES or status == 412:
            return "risk_control"
        if code in RATE_LIMIT_CODES or status == 429:
            return "rate_limit"
        if code in AUTH_CODES or status in {401, 403}:
            return "auth"
        if status >= 500:
            return "server"
        return "upstream"

    @staticmethod
    def _dedupe(values):
        return list(dict.fromkeys(value for value in values if value))
