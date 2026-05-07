from stats import build_stats_summary, normalize_stats


class FortuneRenderBuilder:
    def user_identity_from_context(self, ctx):
        payload = ctx.payload
        onebot = payload.get("onebot") if isinstance(payload.get("onebot"), dict) else {}
        sender = onebot.get("sender") if isinstance(onebot.get("sender"), dict) else {}
        actor = ctx.actor
        user_id = str(onebot.get("user_id") or sender.get("user_id") or actor.get("id") or "").strip()
        nickname = str(sender.get("nickname") or actor.get("nickname") or user_id or "访客").strip()
        group_nickname = str(sender.get("card") or "").strip()
        title = str(sender.get("title") or actor.get("title") or "").strip()
        avatar_url = ""
        if user_id and user_id.isdigit():
            avatar_url = f"https://q1.qlogo.cn/g?b=qq&nk={user_id}&s=100"
        return {
            "id": user_id or "unknown",
            "nickname": nickname or user_id or "访客",
            "group_nickname": group_nickname,
            "title": title,
            "avatar_url": avatar_url,
        }

    def group_identity_from_context(self, ctx):
        target = ctx.target
        if target.get("type") != "group":
            return {}
        name = str(target.get("name") or target.get("id") or "").strip()
        return {"name": name} if name else {}

    def build_render_data(self, ctx, settings, record, stats, repeated):
        local_date = record["date"]
        return {
            "title": "今日运势",
            "subtitle": local_date,
            "repeat_notice": "今日运势已经抽取过，以下为当日结果。" if repeated else "",
            "user": self.user_identity_from_context(ctx),
            "group": self.group_identity_from_context(ctx),
            "fortune": record["fortune"],
            "today_good": record.get("today_good") or [],
            "today_bad": record.get("today_bad") or [],
            "streak": {
                "current": normalize_stats(stats)["current_streak"],
                "total": normalize_stats(stats)["total_days"],
            },
            "stats": build_stats_summary(stats),
        }

    def build_fallback_text(self, render_data):
        fortune = render_data["fortune"]
        streak = render_data["streak"]
        lines = [
            render_data["title"],
            render_data.get("repeat_notice") or "",
            f"运势：{fortune['name']}",
            f"星级：{fortune['stars']}",
            f"签文：{fortune['sign']}",
            f"解签：{fortune['explanation']}",
            f"今日宜：{'、'.join(render_data.get('today_good') or [])}",
            f"今日忌：{'、'.join(render_data.get('today_bad') or [])}",
            f"你已经连续查看运势 {streak['current']} 天。累计查看运势 {streak['total']} 天。",
        ]
        return "\n".join(line for line in lines if line)


_DEFAULT_BUILDER = FortuneRenderBuilder()


def user_identity_from_context(ctx):
    return _DEFAULT_BUILDER.user_identity_from_context(ctx)


def group_identity_from_context(ctx):
    return _DEFAULT_BUILDER.group_identity_from_context(ctx)


def build_render_data(ctx, settings, record, stats, repeated):
    return _DEFAULT_BUILDER.build_render_data(ctx, settings, record, stats, repeated)


def build_fallback_text(render_data):
    return _DEFAULT_BUILDER.build_fallback_text(render_data)
