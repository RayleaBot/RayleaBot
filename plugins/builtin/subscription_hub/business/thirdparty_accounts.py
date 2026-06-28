"""Third-party account helpers."""

from rayleabot.protocol import ActionError

from .platforms import platform_name


def account_label(platform):
    label = platform_name(platform)
    return f"{label} 账号" if label and all(ord(char) < 128 for char in label) else f"{label}账号"


def read_thirdparty_cookie(ctx, platform):
    label = account_label(platform)
    try:
        response = ctx.thirdparty_account_read(platform)
    except ActionError as exc:
        detail = str(exc) or getattr(exc, "code", "")
        return "", f"{label}读取失败：{detail}".strip()
    except Exception:
        return "", f"{label}读取失败。"
    accounts = response.get("accounts") if isinstance(response, dict) else []
    for account in accounts if isinstance(accounts, list) else []:
        cookie = account.get("cookie") if isinstance(account, dict) else {}
        value = str(cookie.get("value") or "").strip() if isinstance(cookie, dict) else ""
        if value:
            return value, ""
    return "", f"没有可用的 {label} CK，请在 Web 三方账号页面保存账号。"


def read_bilibili_cookie(ctx):
    return read_thirdparty_cookie(ctx, "bilibili")
