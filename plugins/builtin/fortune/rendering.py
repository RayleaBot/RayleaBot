from settings import COUNTED_FORTUNES, EXPECTED_STARS, FORTUNE_ORDER, normalize_string_list
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

    def build_stats_render_data(self, ctx, settings, stats):
        from engine import local_date_for_timezone
        from stats import normalize_stats

        local_date = local_date_for_timezone(None, settings["timezone"]).isoformat()
        normalized = normalize_stats(stats)
        total_days = normalized["total_days"]
        total_draws = sum(normalized["counts"].get(name, 0) for name in COUNTED_FORTUNES)

        summary = []
        if total_days:
            summary.append({"label": "累计查看", "value": f"{total_days} 天"})
        current_streak = normalized.get("current_streak", 0)
        if current_streak:
            summary.append({"label": "当前连续", "value": f"{current_streak} 天"})

        distribution = []
        for name in FORTUNE_ORDER:
            count = normalized["counts"].get(name, 0)
            if count == 0 and total_draws == 0:
                continue
            percentage = round((count / total_draws) * 100, 1) if total_draws else 0
            distribution.append({
                "name": name,
                "count": count,
                "percentage": percentage,
                "stars": EXPECTED_STARS.get(name, "☆☆☆☆☆☆☆"),
            })
        special_name = "吉凶未定"
        special_count = normalized["counts"].get(special_name, 0)
        if special_count > 0 or total_draws == 0:
            percentage = round((special_count / total_draws) * 100, 1) if total_draws else 0
            distribution.append({
                "name": special_name,
                "count": special_count,
                "percentage": percentage,
                "stars": EXPECTED_STARS.get(special_name, "???????"),
            })

        records = []
        longest_daji = normalized.get("longest_daji_streak", 0)
        if longest_daji:
            records.append({"label": "最长连续大吉", "value": f"{longest_daji} 天"})
        longest_daxiong = normalized.get("longest_daxiong_streak", 0)
        if longest_daxiong:
            records.append({"label": "最长连续大凶", "value": f"{longest_daxiong} 天"})

        return {
            "title": "运势统计",
            "subtitle": local_date,
            "user": self.user_identity_from_context(ctx),
            "group": self.group_identity_from_context(ctx),
            "summary": summary,
            "distribution": distribution,
            "records": records,
        }

    def build_stats_fallback_text(self, render_data):
        lines = [render_data["title"]]
        for item in render_data.get("summary") or []:
            lines.append(f"{item['label']}：{item['value']}")
        lines.append("")
        lines.append("运势分布：")
        for item in render_data.get("distribution") or []:
            lines.append(f"  {item['name']}：{item['count']} 次（{item['percentage']}%）")
        if render_data.get("records"):
            lines.append("")
            lines.append("连续记录：")
            for item in render_data["records"]:
                lines.append(f"  {item['label']}：{item['value']}")
        return "\n".join(lines)


_DEFAULT_BUILDER = FortuneRenderBuilder()


def user_identity_from_context(ctx):
    return _DEFAULT_BUILDER.user_identity_from_context(ctx)


def group_identity_from_context(ctx):
    return _DEFAULT_BUILDER.group_identity_from_context(ctx)


def build_render_data(ctx, settings, record, stats, repeated):
    return _DEFAULT_BUILDER.build_render_data(ctx, settings, record, stats, repeated)


def build_fallback_text(render_data):
    return _DEFAULT_BUILDER.build_fallback_text(render_data)


def build_stats_render_data(ctx, settings, stats):
    return _DEFAULT_BUILDER.build_stats_render_data(ctx, settings, stats)


def build_stats_fallback_text(render_data):
    return _DEFAULT_BUILDER.build_stats_fallback_text(render_data)
