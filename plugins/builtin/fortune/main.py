#!/usr/bin/env python3
"""Built-in daily fortune plugin for RayleaBot."""

import copy
import os
import sys
import threading

PLUGIN_DIR = os.path.dirname(__file__)
sys.path.insert(0, PLUGIN_DIR)
sys.path.insert(0, os.path.join(PLUGIN_DIR, "..", "..", "..", "sdk", "python"))

from rayleabot import RayleaBotPlugin, event_handler

from engine import (
    DailyFortuneEngine,
    build_daily_record,
    choose_from_list,
    daily_record_matches_settings,
    draw_source_fingerprint,
    find_fortune_by_name,
    fortune_for_date,
    local_date_for_timezone,
    match_special_date,
    parse_trigger,
    stable_fingerprint,
    stable_seed,
)
from rendering import (
    FortuneRenderBuilder,
    build_fallback_text,
    build_render_data,
    build_stats_fallback_text,
    build_stats_render_data,
    group_identity_from_context,
    user_identity_from_context,
)
from settings import (
    COUNTED_FORTUNES,
    DEFAULT_TIMEZONE,
    EXPECTED_STARS,
    FIERCE_STARS,
    FIXED_TIMEZONES,
    FORTUNE_ORDER,
    LEGACY_DEFAULT_BAD_ACTION_FINGERPRINTS,
    LEGACY_DEFAULT_FORTUNE_FINGERPRINTS,
    LEGACY_DEFAULT_GOOD_ACTION_FINGERPRINTS,
    SETTINGS_KEYS,
    SPECIAL_FORTUNE,
    FortuneSettingsService,
    SettingsValidationError,
    fixed_timezone_info,
    fortune_library_fingerprint,
    normalize_fortune,
    normalize_fortunes,
    normalize_special_dates,
    normalize_string_list,
    normalize_timezone_name,
    resolve_timezone_info,
    stable_json,
    stored_actions_are_legacy_defaults,
    stored_fortunes_are_legacy_defaults,
    timezone_name_supported,
    valid_stars_for_fortune,
)
from stats import (
    FortuneStatsService,
    build_stats_summary,
    empty_stats,
    non_negative_int,
    normalize_stats,
    parse_iso_date,
    replace_current_day_fortune_in_stats,
    update_stats,
)


DEFAULT_SETTINGS_PATH = os.path.join(PLUGIN_DIR, "fortunes.json")
_SETTINGS_SERVICE = FortuneSettingsService(DEFAULT_SETTINGS_PATH)


def load_default_settings(path=DEFAULT_SETTINGS_PATH):
    return FortuneSettingsService(path).load_default_settings()


def validate_settings(raw_settings, require_usable=False):
    return _SETTINGS_SERVICE.validate(raw_settings, require_usable=require_usable)


def merge_settings(default_settings, stored_values):
    return _SETTINGS_SERVICE.merge(default_settings, stored_values)


def settings_issue_messages(stored_values):
    return _SETTINGS_SERVICE.issue_messages(stored_values)


def storage_value(result, fallback=None):
    if isinstance(result, dict):
        if result.get("exists") and "value" in result:
            return result.get("value")
        if "value" in result:
            return result.get("value")
    return fallback


class FortunePlugin(RayleaBotPlugin):
    def __init__(self):
        super().__init__()
        self.subscribe("message.group", "message.private", "config.changed")
        self._settings_lock = threading.Lock()
        self._settings_service = FortuneSettingsService(DEFAULT_SETTINGS_PATH)
        self._stats_service = FortuneStatsService()
        self._render_builder = FortuneRenderBuilder()
        self._default_settings = self._settings_service.load_default_settings()
        self._settings = copy.deepcopy(self._default_settings)
        self._engine = DailyFortuneEngine(self._settings)
        self._settings_loaded = False

    def current_settings(self):
        with self._settings_lock:
            return copy.deepcopy(self._settings)

    def current_engine(self):
        with self._settings_lock:
            return self._engine

    def set_current_settings(self, settings):
        with self._settings_lock:
            self._settings = copy.deepcopy(settings)
            self._engine = DailyFortuneEngine(self._settings)
            self._settings_loaded = True

    def settings_loaded(self):
        with self._settings_lock:
            return self._settings_loaded

    def load_settings(self, ctx, force=False):
        if self.settings_loaded() and not force:
            return self.current_settings()

        try:
            response = ctx.config_read(SETTINGS_KEYS)
            values = response.get("values", {}) if isinstance(response, dict) else {}
            settings = self._settings_service.merge(self._default_settings, values)
            for message in self._settings_service.issue_messages(values):
                self.try_log(ctx, "warn", message)
        except Exception as exc:
            settings = copy.deepcopy(self._default_settings)
            self.try_log(ctx, "warn", "运势设置读取失败，使用默认设置", {"error": str(exc)})

        self.set_current_settings(settings)
        return settings

    def try_log(self, ctx, level, message, fields=None):
        try:
            ctx.logger_write(level, message, fields or {})
        except Exception:
            pass

    @event_handler("config.changed")
    def handle_config_changed(self, ctx):
        self.load_settings(ctx, force=True)
        ctx.send_result({"handled": True})

    @event_handler()
    def handle_message(self, ctx):
        if ctx.event_type not in {"message.group", "message.private"}:
            ctx.send_result({"handled": False})
            return

        settings = self.load_settings(ctx)
        engine = self.current_engine()

        fortune_trigger = parse_trigger(ctx.plain_text, ctx.command_prefixes, settings["trigger_commands"])
        stats_trigger = None
        if fortune_trigger is None:
            stats_trigger = parse_trigger(ctx.plain_text, ctx.command_prefixes, settings["stats_trigger_commands"])

        if fortune_trigger is None and stats_trigger is None:
            ctx.send_result({"handled": False})
            return

        user = self._render_builder.user_identity_from_context(ctx)
        user_id = user["id"]

        if fortune_trigger is not None:
            self._handle_fortune_command(ctx, settings, engine, user_id)
        else:
            self._handle_stats_command(ctx, settings, user_id)

    def _handle_fortune_command(self, ctx, settings, engine, user_id):
        local_day = local_date_for_timezone(None, settings["timezone"])
        daily_key = f"daily:{user_id}:{local_day.isoformat()}"
        stats_key = f"stats:{user_id}"

        record = storage_value(ctx.storage_get(daily_key))
        repeated = isinstance(record, dict)
        stats = self._stats_service.normalize(storage_value(ctx.storage_get(stats_key), self._stats_service.empty()))
        should_store = False

        if repeated and not engine.record_matches_settings(record):
            previous_fortune_name = ""
            if isinstance(record.get("fortune"), dict):
                previous_fortune_name = str(record["fortune"].get("name") or "")
            record = engine.build_daily_record(user_id, local_day)
            stats = self._stats_service.replace_current_day_fortune(stats, previous_fortune_name, record["fortune"]["name"])
            repeated = False
            should_store = True
        elif not repeated:
            record = engine.build_daily_record(user_id, local_day)
            stats = self._stats_service.update(stats, record["fortune"]["name"], local_day)
            should_store = True

        if should_store:
            ctx.storage_set(daily_key, record)
            ctx.storage_set(stats_key, stats)

        render_data = self._render_builder.build_render_data(ctx, settings, record, stats, repeated)
        self.send_fortune_image(ctx, render_data)
        ctx.send_result({"handled": True})

    def _handle_stats_command(self, ctx, settings, user_id):
        stats_key = f"stats:{user_id}"
        stats = self._stats_service.normalize(storage_value(ctx.storage_get(stats_key), self._stats_service.empty()))

        if stats["total_days"] == 0:
            ctx.send_message([{
                "type": "text",
                "data": {"text": "你还没有抽取过运势，发送「我的运势」来抽取今日运势吧！"},
            }])
            ctx.send_result({"handled": True})
            return

        render_data = self._render_builder.build_stats_render_data(ctx, settings, stats)
        self.send_stats_image(ctx, render_data)
        ctx.send_result({"handled": True})

    def send_fortune_image(self, ctx, render_data):
        fallback_text = self._render_builder.build_fallback_text(render_data)
        result = ctx.render_image(
            "fortune.card",
            render_data,
            theme="default",
            output="png",
            fallback_text=fallback_text,
        )
        image_path = str(result.get("image_path") or "").strip()
        if image_path:
            ctx.send_message([{
                "type": "image",
                "data": {"file": image_path},
            }])
            return
        self.try_log(ctx, "warn", "运势图片生成结果缺少图片路径")

    def send_stats_image(self, ctx, render_data):
        fallback_text = self._render_builder.build_stats_fallback_text(render_data)
        result = ctx.render_image(
            "fortune.stats",
            render_data,
            theme="default",
            output="png",
            fallback_text=fallback_text,
        )
        image_path = str(result.get("image_path") or "").strip()
        if image_path:
            ctx.send_message([{
                "type": "image",
                "data": {"file": image_path},
            }])
            return
        self.try_log(ctx, "warn", "运势统计图片生成结果缺少图片路径")


if __name__ == "__main__":
    FortunePlugin().run()
