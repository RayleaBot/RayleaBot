"""High-level plugin framework for RayleaBot Python plugins."""

import inspect
import threading

from rayleabot import protocol


def command(name, aliases=None):
    """Decorator for class-based command handlers."""
    def decorator(func):
        func._rayleabot_command = (name, list(aliases or []))
        return func
    return decorator


def event_handler(event_type=None):
    """Decorator for class-based event handlers."""
    def decorator(func):
        func._rayleabot_event_type = event_type
        return func
    return decorator


class EventContext:
    """Request-bound event context passed to class-based handlers."""

    def __init__(self, plugin, event, request_id):
        self._plugin = plugin
        self.event = event or {}
        self.request_id = request_id

    @property
    def payload(self):
        return self.event.get("payload") or {}

    @property
    def target(self):
        return self.event.get("target") or {}

    @property
    def actor(self):
        return self.event.get("actor") or {}

    @property
    def message(self):
        return self.event.get("message") or {}

    @property
    def event_type(self):
        return self.event.get("event_type", "")

    @property
    def command(self):
        return self.payload.get("command")

    @property
    def args(self):
        args = self.payload.get("args")
        return list(args) if isinstance(args, list) else []

    @property
    def plain_text(self):
        return self.message.get("plain_text", "")

    @property
    def target_type(self):
        return self.target.get("type", "group")

    @property
    def target_id(self):
        return self.target.get("id", "")

    @property
    def bot_id(self):
        return self._plugin.bot_id

    @property
    def capabilities(self):
        return self._plugin.capabilities

    @property
    def command_prefixes(self):
        return self._plugin.command_prefixes

    @property
    def super_admins(self):
        return self._plugin.super_admins

    @property
    def primary_command_prefix(self):
        return self._plugin.primary_command_prefix

    def send_message(self, segments, target_type=None, target_id=None):
        """Send message segments to the current target by default."""
        self._plugin.send_message(
            self.request_id,
            target_type or self.target_type,
            target_id or self.target_id,
            segments,
        )

    def send_text(self, text, target_type=None, target_id=None):
        """Send one text segment to the current target by default."""
        self.send_message(
            [{
                "type": "text",
                "data": {"text": text},
            }],
            target_type=target_type,
            target_id=target_id,
        )

    def send_reply(self, reply_to_event_id, segments, fallback_to_send_if_missing=False):
        """Reply to a recent upstream event."""
        self._plugin.send_reply(
            self.request_id,
            reply_to_event_id,
            segments,
            fallback_to_send_if_missing=fallback_to_send_if_missing,
        )

    def send_result(self, data=None):
        """Send a success result for the current event."""
        self._plugin.send_result(self.request_id, data or {})

    def __getattr__(self, name):
        attr = getattr(self._plugin, name)
        if not callable(attr):
            return attr

        def request_bound_helper(*args, **kwargs):
            return attr(self.request_id, *args, **kwargs)

        return request_bound_helper


class RayleaBotPlugin:
    """Base class for RayleaBot plugins with event and command handler registration."""

    def __init__(self):
        self._event_handlers = []
        self._command_handlers = {}
        self._active_handlers = set()
        self._handler_lock = threading.Lock()
        self._plugin_id = ""
        self._bot_id = ""
        self._bot_identity_event = threading.Event()
        self._capabilities = []
        self._super_admins = []
        self._command_prefixes = ["/"]
        self._subscriptions = None
        self._register_decorated_handlers()

    def _register_decorated_handlers(self):
        seen = set()
        for cls in type(self).__mro__:
            for attr_name, source in vars(cls).items():
                if attr_name in seen:
                    continue
                seen.add(attr_name)
                source_func = getattr(source, "__func__", source)
                command_meta = getattr(source_func, "_rayleabot_command", None)
                has_event_handler = hasattr(source_func, "_rayleabot_event_type")
                if command_meta is None and not has_event_handler:
                    continue

                handler = getattr(self, attr_name)
                if not callable(handler):
                    continue
                self._register_decorated_handler(source_func, handler, command_meta, has_event_handler)

    def _register_decorated_handler(self, source, handler, command_meta, has_event_handler):
        if command_meta is not None:
            name, aliases = command_meta
            self._register_command_handler(name, handler, aliases)
        if has_event_handler:
            self._event_handlers.append((getattr(source, "_rayleabot_event_type"), handler))

    def _register_command_handler(self, name, handler, aliases=None):
        self._command_handlers[name] = handler
        if aliases:
            for alias in aliases:
                self._command_handlers[alias] = handler

    def on_event(self, event_type=None):
        """Decorator to register an event handler. If event_type is None, matches all events."""
        def decorator(func):
            self._event_handlers.append((event_type, func))
            return func
        return decorator

    def on_command(self, name, aliases=None):
        """Decorator to register a command handler by name and optional aliases."""
        def decorator(func):
            self._register_command_handler(name, func, aliases)
            return func
        return decorator

    def subscribe(self, *event_types):
        """Declare event types this plugin subscribes to (used in init_ack)."""
        self._subscriptions = list(event_types)
        return self

    def send_message(self, request_id, target_type, target_id, segments):
        """Send a message to a target."""
        protocol.send_action(self._plugin_id, request_id, "message.send", {
            "target_type": target_type,
            "target_id": target_id,
            "message": {
                "segments": segments,
            },
        })

    def send_reply(self, request_id, reply_to_event_id, segments, fallback_to_send_if_missing=False):
        """Reply to a recent upstream event using reply_to_event_id."""
        data = {
            "reply_to_event_id": reply_to_event_id,
            "message": {
                "segments": segments,
            },
        }
        if fallback_to_send_if_missing:
            data["fallback_to_send_if_missing"] = True
        protocol.send_action(self._plugin_id, request_id, "message.reply", data)

    def send_result(self, request_id, data=None):
        """Send a success result for an event."""
        protocol.send_result(self._plugin_id, request_id, data or {})

    def logger_write(self, request_id, level, message, fields=None, timeout_seconds=30):
        """Write a management log entry through the platform-local logger.write action."""
        data = {
            "level": level,
            "message": message,
        }
        if fields is not None:
            data["fields"] = fields
        return protocol.request_local_action(
            self._plugin_id,
            request_id,
            "logger.write",
            data,
            timeout_seconds=timeout_seconds,
        )

    def storage_get(self, request_id, key, timeout_seconds=30):
        """Read one plugin-scoped KV value."""
        return protocol.request_local_action(
            self._plugin_id,
            request_id,
            "storage.kv",
            {"operation": "get", "key": key},
            timeout_seconds=timeout_seconds,
        )

    def storage_set(self, request_id, key, value, timeout_seconds=30):
        """Write one plugin-scoped KV value."""
        return protocol.request_local_action(
            self._plugin_id,
            request_id,
            "storage.kv",
            {"operation": "set", "key": key, "value": value},
            timeout_seconds=timeout_seconds,
        )

    def storage_delete(self, request_id, key, timeout_seconds=30):
        """Delete one plugin-scoped KV value."""
        return protocol.request_local_action(
            self._plugin_id,
            request_id,
            "storage.kv",
            {"operation": "delete", "key": key},
            timeout_seconds=timeout_seconds,
        )

    def storage_list(self, request_id, prefix="", timeout_seconds=30):
        """List plugin-scoped KV keys under a prefix."""
        return protocol.request_local_action(
            self._plugin_id,
            request_id,
            "storage.kv",
            {"operation": "list", "prefix": prefix},
            timeout_seconds=timeout_seconds,
        )

    def storage_file_read(self, request_id, path, root="plugin_data", timeout_seconds=30):
        """Read one plugin_data file path through storage.file."""
        return protocol.request_local_action(
            self._plugin_id,
            request_id,
            "storage.file",
            {"operation": "read", "root": root, "path": path},
            timeout_seconds=timeout_seconds,
        )

    def storage_file_write(self, request_id, path, content_text=None, content_base64=None, root="plugin_data", timeout_seconds=30):
        """Write one plugin_data file path through storage.file."""
        if (content_text is None) == (content_base64 is None):
            raise ValueError("storage_file_write requires exactly one of content_text or content_base64")
        data = {"operation": "write", "root": root, "path": path}
        if content_text is not None:
            data["content_text"] = content_text
        else:
            data["content_base64"] = content_base64
        return protocol.request_local_action(
            self._plugin_id,
            request_id,
            "storage.file",
            data,
            timeout_seconds=timeout_seconds,
        )

    def storage_file_delete(self, request_id, path, root="plugin_data", timeout_seconds=30):
        """Delete one plugin_data file path through storage.file."""
        return protocol.request_local_action(
            self._plugin_id,
            request_id,
            "storage.file",
            {"operation": "delete", "root": root, "path": path},
            timeout_seconds=timeout_seconds,
        )

    def storage_file_list(self, request_id, prefix="", root="plugin_data", timeout_seconds=30):
        """List plugin_data file paths under one prefix."""
        return protocol.request_local_action(
            self._plugin_id,
            request_id,
            "storage.file",
            {"operation": "list", "root": root, "prefix": prefix},
            timeout_seconds=timeout_seconds,
        )

    def http_request(self, request_id, method, url, headers=None, timeout_seconds=30, body_text=None, body_base64=None):
        """Issue one scoped http.request through the platform-local HTTP client."""
        if body_text is not None and body_base64 is not None:
            raise ValueError("http_request requires at most one of body_text or body_base64")
        data = {
            "method": method,
            "url": url,
        }
        if headers:
            data["headers"] = headers
        if timeout_seconds is not None:
            data["timeout_seconds"] = timeout_seconds
        if body_text is not None:
            data["body_text"] = body_text
        if body_base64 is not None:
            data["body_base64"] = body_base64
        return protocol.request_local_action(
            self._plugin_id,
            request_id,
            "http.request",
            data,
            timeout_seconds=timeout_seconds,
        )

    def config_read(self, request_id, keys, timeout_seconds=30):
        """Read plugin-scoped config keys through config.read."""
        if not keys:
            raise ValueError("config_read requires at least one key")
        return protocol.request_local_action(
            self._plugin_id,
            request_id,
            "config.read",
            {"keys": list(keys)},
            timeout_seconds=timeout_seconds,
        )

    def config_write(self, request_id, values, timeout_seconds=30):
        """Write plugin-scoped config values through config.write."""
        if not values:
            raise ValueError("config_write requires at least one key/value pair")
        return protocol.request_local_action(
            self._plugin_id,
            request_id,
            "config.write",
            {"values": values},
            timeout_seconds=timeout_seconds,
        )

    def governance_blacklist_read(self, request_id, timeout_seconds=30):
        """Read the current governance blacklist snapshot."""
        return protocol.request_local_action(
            self._plugin_id,
            request_id,
            "governance.blacklist.read",
            {},
            timeout_seconds=timeout_seconds,
        )

    def governance_blacklist_write(self, request_id, operation, entry_type=None, target_id=None, reason=None, timeout_seconds=30):
        """Write one governance blacklist change."""
        data = {"operation": operation}
        if operation == "upsert":
            if not entry_type or not target_id or not reason:
                raise ValueError("governance_blacklist_write upsert requires entry_type, target_id, and reason")
            data["entry_type"] = entry_type
            data["target_id"] = target_id
            data["reason"] = reason
        elif operation == "delete":
            if not entry_type or not target_id:
                raise ValueError("governance_blacklist_write delete requires entry_type and target_id")
            data["entry_type"] = entry_type
            data["target_id"] = target_id
        else:
            raise ValueError("governance_blacklist_write requires operation upsert or delete")
        return protocol.request_local_action(
            self._plugin_id,
            request_id,
            "governance.blacklist.write",
            data,
            timeout_seconds=timeout_seconds,
        )

    def governance_whitelist_read(self, request_id, timeout_seconds=30):
        """Read the current governance whitelist snapshot."""
        return protocol.request_local_action(
            self._plugin_id,
            request_id,
            "governance.whitelist.read",
            {},
            timeout_seconds=timeout_seconds,
        )

    def governance_whitelist_write(self, request_id, operation, enabled=None, entry_type=None, target_id=None, reason=None, timeout_seconds=30):
        """Write one governance whitelist change."""
        data = {"operation": operation}
        if operation == "set_enabled":
            if enabled is None:
                raise ValueError("governance_whitelist_write set_enabled requires enabled")
            data["enabled"] = enabled
        elif operation == "upsert":
            if not entry_type or not target_id or not reason:
                raise ValueError("governance_whitelist_write upsert requires entry_type, target_id, and reason")
            data["entry_type"] = entry_type
            data["target_id"] = target_id
            data["reason"] = reason
        elif operation == "delete":
            if not entry_type or not target_id:
                raise ValueError("governance_whitelist_write delete requires entry_type and target_id")
            data["entry_type"] = entry_type
            data["target_id"] = target_id
        else:
            raise ValueError("governance_whitelist_write requires operation set_enabled, upsert, or delete")
        return protocol.request_local_action(
            self._plugin_id,
            request_id,
            "governance.whitelist.write",
            data,
            timeout_seconds=timeout_seconds,
        )

    def governance_command_policy_read(self, request_id, timeout_seconds=30):
        """Read the current governance command policy projection."""
        return protocol.request_local_action(
            self._plugin_id,
            request_id,
            "governance.command_policy.read",
            {},
            timeout_seconds=timeout_seconds,
        )

    def scheduler_create(self, request_id, task_id, cron, payload=None, log_label=None, timeout_seconds=30):
        """Create or update one scheduler.trigger task through scheduler.create."""
        data = {
            "task_id": task_id,
            "cron": cron,
            "event_type": "scheduler.trigger",
        }
        if log_label is not None:
            data["log_label"] = log_label
        if payload is not None:
            data["payload"] = payload
        return protocol.request_local_action(
            self._plugin_id,
            request_id,
            "scheduler.create",
            data,
            timeout_seconds=timeout_seconds,
        )

    def expose_webhook(
        self,
        request_id,
        route,
        secret_ref,
        methods=None,
        auth_strategy="fixed_token",
        header="X-Webhook-Token",
        signature_prefix=None,
        source_ips=None,
        replay_protection=None,
        timeout_seconds=30,
    ):
        """Register a controlled webhook route through event.expose_webhook."""
        data = {
            "route": route,
            "methods": list(methods or ["POST"]),
            "auth_strategy": auth_strategy,
            "header": header,
            "secret_ref": secret_ref,
        }
        if signature_prefix is not None:
            data["signature_prefix"] = signature_prefix
        if source_ips:
            data["source_ips"] = list(source_ips)
        data["replay_protection"] = _normalise_replay_protection(replay_protection)
        return protocol.request_local_action(
            self._plugin_id,
            request_id,
            "event.expose_webhook",
            data,
            timeout_seconds=timeout_seconds,
        )

    def render_image(self, request_id, template, data, theme=None, output=None, fallback_text=None, timeout_seconds=30):
        """Render one image artifact through render.image."""
        payload = {
            "template": template,
            "data": data,
        }
        if theme is not None:
            payload["theme"] = theme
        if output is not None:
            payload["output"] = output
        if fallback_text is not None:
            payload["fallback_text"] = fallback_text
        return protocol.request_local_action(
            self._plugin_id,
            request_id,
            "render.image",
            payload,
            timeout_seconds=timeout_seconds,
        )

    def plugin_list(self, request_id, visibility=None, timeout_seconds=30):
        """List installed plugins through plugin.list."""
        data = {}
        if visibility is not None:
            data["visibility"] = visibility
        return protocol.request_local_action(
            self._plugin_id,
            request_id,
            "plugin.list",
            data,
            timeout_seconds=timeout_seconds,
        )

    def secret_read(self, request_id, key, timeout_seconds=30):
        """Read one plugin-owned secret value through secret.read."""
        if not key:
            raise ValueError("secret_read requires key")
        return protocol.request_local_action(
            self._plugin_id,
            request_id,
            "secret.read",
            {"key": key},
            timeout_seconds=timeout_seconds,
        )

    def onebot_action(self, request_id, action, data=None, timeout_seconds=30):
        """Call one frozen OneBot family action through the shared local action path."""
        return protocol.request_local_action(
            self._plugin_id,
            request_id,
            action,
            data or {},
            timeout_seconds=timeout_seconds,
        )

    def provider_action(self, request_id, provider, action, data=None, timeout_seconds=30):
        """Call one provider-specific OneBot extension action."""
        return self.onebot_action(
            request_id,
            f"provider.{provider}.{action}",
            data=data,
            timeout_seconds=timeout_seconds,
        )

    def _named_onebot_action(self, request_id, action, data=None, extra_data=None, timeout_seconds=30):
        payload = dict(data or {})
        if extra_data:
            payload.update(extra_data)
        return self.onebot_action(request_id, action, payload, timeout_seconds=timeout_seconds)

    def message_get(self, request_id, message_id, timeout_seconds=30, extra_data=None):
        return self._named_onebot_action(request_id, "message.get", {"message_id": message_id}, extra_data, timeout_seconds)

    def message_delete(self, request_id, message_id, timeout_seconds=30, extra_data=None):
        return self._named_onebot_action(request_id, "message.delete", {"message_id": message_id}, extra_data, timeout_seconds)

    def message_history_get(self, request_id, conversation_type, conversation_id, limit=None, timeout_seconds=30, extra_data=None):
        data = {"conversation_type": conversation_type, "conversation_id": conversation_id}
        if limit is not None:
            data["limit"] = limit
        return self._named_onebot_action(request_id, "message.history.get", data, extra_data, timeout_seconds)

    def message_forward_get(self, request_id, message_id=None, forward_id=None, timeout_seconds=30, extra_data=None):
        if not message_id and not forward_id:
            raise ValueError("message_forward_get requires message_id or forward_id")
        data = {}
        if message_id is not None:
            data["message_id"] = message_id
        if forward_id is not None:
            data["forward_id"] = forward_id
        return self._named_onebot_action(request_id, "message.forward.get", data, extra_data, timeout_seconds)

    def message_forward_send(self, request_id, target_type, target_id, messages, timeout_seconds=30, extra_data=None):
        return self._named_onebot_action(
            request_id,
            "message.forward.send",
            {"target_type": target_type, "target_id": target_id, "messages": messages},
            extra_data,
            timeout_seconds,
        )

    def message_read_mark(self, request_id, message_id=None, conversation_type=None, conversation_id=None, timeout_seconds=30, extra_data=None):
        if message_id is None and (conversation_type is None or conversation_id is None):
            raise ValueError("message_read_mark requires message_id or conversation_type with conversation_id")
        data = {}
        if message_id is not None:
            data["message_id"] = message_id
        if conversation_type is not None:
            data["conversation_type"] = conversation_type
        if conversation_id is not None:
            data["conversation_id"] = conversation_id
        return self._named_onebot_action(request_id, "message.read.mark", data, extra_data, timeout_seconds)

    def friend_request_handle(self, request_id, flag, approve, timeout_seconds=30, extra_data=None):
        return self._named_onebot_action(request_id, "friend.request.handle", {"flag": flag, "approve": approve}, extra_data, timeout_seconds)

    def friend_list(self, request_id, timeout_seconds=30, extra_data=None):
        return self._named_onebot_action(request_id, "friend.list", {}, extra_data, timeout_seconds)

    def friend_remark_set(self, request_id, user_id, remark, timeout_seconds=30, extra_data=None):
        return self._named_onebot_action(request_id, "friend.remark.set", {"user_id": user_id, "remark": remark}, extra_data, timeout_seconds)

    def user_info_get(self, request_id, user_id, timeout_seconds=30, extra_data=None):
        return self._named_onebot_action(request_id, "user.info.get", {"user_id": user_id}, extra_data, timeout_seconds)

    def user_like_send(self, request_id, user_id, timeout_seconds=30, extra_data=None):
        return self._named_onebot_action(request_id, "user.like.send", {"user_id": user_id}, extra_data, timeout_seconds)

    def group_list(self, request_id, timeout_seconds=30, extra_data=None):
        return self._named_onebot_action(request_id, "group.list", {}, extra_data, timeout_seconds)

    def group_info_get(self, request_id, group_id, timeout_seconds=30, extra_data=None):
        return self._named_onebot_action(request_id, "group.info.get", {"group_id": group_id}, extra_data, timeout_seconds)

    def group_member_get(self, request_id, group_id, user_id, timeout_seconds=30, extra_data=None):
        return self._named_onebot_action(request_id, "group.member.get", {"group_id": group_id, "user_id": user_id}, extra_data, timeout_seconds)

    def group_member_list(self, request_id, group_id, timeout_seconds=30, extra_data=None):
        return self._named_onebot_action(request_id, "group.member.list", {"group_id": group_id}, extra_data, timeout_seconds)

    def group_request_handle(self, request_id, flag, approve, timeout_seconds=30, extra_data=None):
        return self._named_onebot_action(request_id, "group.request.handle", {"flag": flag, "approve": approve}, extra_data, timeout_seconds)

    def group_leave(self, request_id, group_id, timeout_seconds=30, extra_data=None):
        return self._named_onebot_action(request_id, "group.leave", {"group_id": group_id}, extra_data, timeout_seconds)

    def group_admin_set(self, request_id, group_id, user_id, enabled, timeout_seconds=30, extra_data=None):
        return self._named_onebot_action(request_id, "group.admin.set", {"group_id": group_id, "user_id": user_id, "enabled": enabled}, extra_data, timeout_seconds)

    def group_ban_set(self, request_id, group_id, user_id=None, duration_seconds=None, whole_group=False, timeout_seconds=30, extra_data=None):
        data = {"group_id": group_id, "whole_group": whole_group}
        if user_id is not None:
            data["user_id"] = user_id
        if duration_seconds is not None:
            data["duration_seconds"] = duration_seconds
        return self._named_onebot_action(request_id, "group.ban.set", data, extra_data, timeout_seconds)

    def group_card_set(self, request_id, group_id, user_id, card, timeout_seconds=30, extra_data=None):
        return self._named_onebot_action(request_id, "group.card.set", {"group_id": group_id, "user_id": user_id, "card": card}, extra_data, timeout_seconds)

    def group_title_set(self, request_id, group_id, user_id, title, timeout_seconds=30, extra_data=None):
        return self._named_onebot_action(request_id, "group.title.set", {"group_id": group_id, "user_id": user_id, "title": title}, extra_data, timeout_seconds)

    def group_name_set(self, request_id, group_id, name, timeout_seconds=30, extra_data=None):
        return self._named_onebot_action(request_id, "group.name.set", {"group_id": group_id, "name": name}, extra_data, timeout_seconds)

    def group_announcement_list(self, request_id, group_id, timeout_seconds=30, extra_data=None):
        return self._named_onebot_action(request_id, "group.announcement.list", {"group_id": group_id}, extra_data, timeout_seconds)

    def group_announcement_create(self, request_id, group_id, content, timeout_seconds=30, extra_data=None):
        return self._named_onebot_action(request_id, "group.announcement.create", {"group_id": group_id, "content": content}, extra_data, timeout_seconds)

    def group_announcement_delete(self, request_id, group_id, notice_id, timeout_seconds=30, extra_data=None):
        return self._named_onebot_action(request_id, "group.announcement.delete", {"group_id": group_id, "notice_id": notice_id}, extra_data, timeout_seconds)

    def group_essence_list(self, request_id, group_id, timeout_seconds=30, extra_data=None):
        return self._named_onebot_action(request_id, "group.essence.list", {"group_id": group_id}, extra_data, timeout_seconds)

    def group_essence_set(self, request_id, message_id, timeout_seconds=30, extra_data=None):
        return self._named_onebot_action(request_id, "group.essence.set", {"message_id": message_id}, extra_data, timeout_seconds)

    def group_essence_unset(self, request_id, message_id, timeout_seconds=30, extra_data=None):
        return self._named_onebot_action(request_id, "group.essence.unset", {"message_id": message_id}, extra_data, timeout_seconds)

    def group_honor_get(self, request_id, group_id, timeout_seconds=30, extra_data=None):
        return self._named_onebot_action(request_id, "group.honor.get", {"group_id": group_id}, extra_data, timeout_seconds)

    def group_todo_set(self, request_id, group_id, todo, timeout_seconds=30, extra_data=None):
        return self._named_onebot_action(request_id, "group.todo.set", {"group_id": group_id, "todo": todo}, extra_data, timeout_seconds)

    def file_get(self, request_id, file_id, timeout_seconds=30, extra_data=None):
        return self._named_onebot_action(request_id, "file.get", {"file_id": file_id}, extra_data, timeout_seconds)

    def file_download(self, request_id, file_id, timeout_seconds=30, extra_data=None):
        return self._named_onebot_action(request_id, "file.download", {"file_id": file_id}, extra_data, timeout_seconds)

    def file_group_upload(self, request_id, group_id, file_name, file_url, timeout_seconds=30, extra_data=None):
        return self._named_onebot_action(request_id, "file.group.upload", {"group_id": group_id, "file_name": file_name, "file_url": file_url}, extra_data, timeout_seconds)

    def file_private_upload(self, request_id, user_id, file_name, file_url, timeout_seconds=30, extra_data=None):
        return self._named_onebot_action(request_id, "file.private.upload", {"user_id": user_id, "file_name": file_name, "file_url": file_url}, extra_data, timeout_seconds)

    def file_group_url_get(self, request_id, group_id, file_id, timeout_seconds=30, extra_data=None):
        return self._named_onebot_action(request_id, "file.group.url.get", {"group_id": group_id, "file_id": file_id}, extra_data, timeout_seconds)

    def file_private_url_get(self, request_id, user_id, file_id, timeout_seconds=30, extra_data=None):
        return self._named_onebot_action(request_id, "file.private.url.get", {"user_id": user_id, "file_id": file_id}, extra_data, timeout_seconds)

    def file_group_fs_info(self, request_id, group_id, timeout_seconds=30, extra_data=None):
        return self._named_onebot_action(request_id, "file.group.fs.info", {"group_id": group_id}, extra_data, timeout_seconds)

    def file_group_fs_list(self, request_id, group_id, folder_id=None, timeout_seconds=30, extra_data=None):
        data = {"group_id": group_id}
        if folder_id is not None:
            data["folder_id"] = folder_id
        return self._named_onebot_action(request_id, "file.group.fs.list", data, extra_data, timeout_seconds)

    def file_group_fs_mkdir(self, request_id, group_id, name, timeout_seconds=30, extra_data=None):
        return self._named_onebot_action(request_id, "file.group.fs.mkdir", {"group_id": group_id, "name": name}, extra_data, timeout_seconds)

    def file_group_fs_delete(self, request_id, group_id, folder_id=None, file_id=None, timeout_seconds=30, extra_data=None):
        if folder_id is None and file_id is None:
            raise ValueError("file_group_fs_delete requires folder_id or file_id")
        data = {"group_id": group_id}
        if folder_id is not None:
            data["folder_id"] = folder_id
        if file_id is not None:
            data["file_id"] = file_id
        return self._named_onebot_action(request_id, "file.group.fs.delete", data, extra_data, timeout_seconds)

    def reaction_set(self, request_id, message_id, emoji, enabled=True, timeout_seconds=30, extra_data=None):
        return self._named_onebot_action(request_id, "reaction.set", {"message_id": message_id, "emoji": emoji, "enabled": enabled}, extra_data, timeout_seconds)

    def reaction_list(self, request_id, message_id, timeout_seconds=30, extra_data=None):
        return self._named_onebot_action(request_id, "reaction.list", {"message_id": message_id}, extra_data, timeout_seconds)

    def poke_send(self, request_id, target_type, target_id, user_id, timeout_seconds=30, extra_data=None):
        return self._named_onebot_action(request_id, "poke.send", {"target_type": target_type, "target_id": target_id, "user_id": user_id}, extra_data, timeout_seconds)

    def napcat_message_emoji_like_set(self, request_id, message_id, emoji_id, enabled=True, timeout_seconds=30, extra_data=None):
        return self.provider_action(
            request_id,
            "napcat",
            "message_emoji.like.set",
            self._named_onebot_action_data({"message_id": message_id, "emoji_id": emoji_id, "enabled": enabled}, extra_data),
            timeout_seconds=timeout_seconds,
        )

    def napcat_group_sign_set(self, request_id, group_id, timeout_seconds=30, extra_data=None):
        return self.provider_action(
            request_id,
            "napcat",
            "group.sign.set",
            self._named_onebot_action_data({"group_id": group_id}, extra_data),
            timeout_seconds=timeout_seconds,
        )

    def luckylillia_friend_groups_get(self, request_id, user_id, timeout_seconds=30, extra_data=None):
        return self.provider_action(
            request_id,
            "luckylillia",
            "friend_groups.get",
            self._named_onebot_action_data({"user_id": user_id}, extra_data),
            timeout_seconds=timeout_seconds,
        )

    @staticmethod
    def _named_onebot_action_data(data=None, extra_data=None):
        payload = dict(data or {})
        if extra_data:
            payload.update(extra_data)
        return payload

    storageFileRead = storage_file_read
    storageFileWrite = storage_file_write
    storageFileDelete = storage_file_delete
    storageFileList = storage_file_list
    httpRequest = http_request
    configRead = config_read
    configWrite = config_write
    sendResult = send_result
    governanceBlacklistRead = governance_blacklist_read
    governanceBlacklistWrite = governance_blacklist_write
    governanceWhitelistRead = governance_whitelist_read
    governanceWhitelistWrite = governance_whitelist_write
    governanceCommandPolicyRead = governance_command_policy_read
    schedulerCreate = scheduler_create
    exposeWebhook = expose_webhook
    renderImage = render_image
    pluginList = plugin_list
    secretRead = secret_read
    onebotAction = onebot_action
    providerAction = provider_action
    messageGet = message_get
    messageDelete = message_delete
    messageHistoryGet = message_history_get
    messageForwardGet = message_forward_get
    messageForwardSend = message_forward_send
    messageReadMark = message_read_mark
    friendRequestHandle = friend_request_handle
    friendList = friend_list
    friendRemarkSet = friend_remark_set
    userInfoGet = user_info_get
    userLikeSend = user_like_send
    groupList = group_list
    groupInfoGet = group_info_get
    groupMemberGet = group_member_get
    groupMemberList = group_member_list
    groupRequestHandle = group_request_handle
    groupLeave = group_leave
    groupAdminSet = group_admin_set
    groupBanSet = group_ban_set
    groupCardSet = group_card_set
    groupTitleSet = group_title_set
    groupNameSet = group_name_set
    groupAnnouncementList = group_announcement_list
    groupAnnouncementCreate = group_announcement_create
    groupAnnouncementDelete = group_announcement_delete
    groupEssenceList = group_essence_list
    groupEssenceSet = group_essence_set
    groupEssenceUnset = group_essence_unset
    groupHonorGet = group_honor_get
    groupTodoSet = group_todo_set
    fileGet = file_get
    fileDownload = file_download
    fileGroupUpload = file_group_upload
    filePrivateUpload = file_private_upload
    fileGroupUrlGet = file_group_url_get
    filePrivateUrlGet = file_private_url_get
    fileGroupFsInfo = file_group_fs_info
    fileGroupFsList = file_group_fs_list
    fileGroupFsMkdir = file_group_fs_mkdir
    fileGroupFsDelete = file_group_fs_delete
    reactionSet = reaction_set
    reactionList = reaction_list
    pokeSend = poke_send
    napcatMessageEmojiLikeSet = napcat_message_emoji_like_set
    napcatGroupSignSet = napcat_group_sign_set
    luckylilliaFriendGroupsGet = luckylillia_friend_groups_get

    def run(self):
        """Main event loop: handles init, events, ping, and shutdown."""
        while True:
            frame = protocol.read_frame()
            if frame is None:
                break

            frame_type = frame.get("type")
            plugin_id = frame.get("plugin_id", "")
            request_id = frame.get("request_id", "")

            if frame_type == "init":
                self._plugin_id = plugin_id
                bot = frame.get("bot", {})
                self._set_bot_id(bot.get("id", ""))
                self._capabilities = frame.get("capabilities", [])
                permissions = frame.get("permissions") if isinstance(frame.get("permissions"), dict) else {}
                self._super_admins = [str(item).strip() for item in permissions.get("super_admins", []) if str(item).strip()]
                prefixes = [prefix for prefix in frame.get("command_prefixes", ["/"]) if isinstance(prefix, str) and prefix]
                self._command_prefixes = prefixes or ["/"]
                protocol.send_init_ack(plugin_id, request_id, self._subscriptions)

            elif frame_type == "event":
                self._start_event_handler(frame, plugin_id, request_id)

            elif frame_type == "ping":
                protocol.send_pong(plugin_id, request_id)

            elif frame_type == "shutdown":
                break

        self._wait_for_handlers()

    def _start_event_handler(self, frame, plugin_id, request_id):
        handler_thread = threading.Thread(
            target=self._handle_event_safely,
            args=(frame, plugin_id, request_id),
            daemon=False,
        )
        with self._handler_lock:
            self._active_handlers.add(handler_thread)
        handler_thread.start()

    def _handle_event_safely(self, frame, plugin_id, request_id):
        try:
            self._handle_event(frame, plugin_id, request_id)
        except Exception as exc:
            message = str(exc) or exc.__class__.__name__
            protocol.send_error(plugin_id, request_id, "plugin.internal_error", message)
        finally:
            with self._handler_lock:
                self._active_handlers.discard(threading.current_thread())

    def _wait_for_handlers(self):
        while True:
            with self._handler_lock:
                handlers = [thread for thread in self._active_handlers if thread.is_alive()]
            if not handlers:
                return
            for handler in handlers:
                handler.join(timeout=0.05)

    def _handle_event(self, frame, plugin_id, request_id):
        event = frame.get("event", {})
        event_type = event.get("event_type", "")
        self._update_bot_identity(event)
        payload = event.get("payload") or {}
        command = payload.get("command")

        # Try command handler first.
        if command and command in self._command_handlers:
            handler = self._command_handlers[command]
            self._invoke_handler(handler, event, request_id)
            return

        # Try event type handlers.
        for type_filter, handler in self._event_handlers:
            if type_filter is None or type_filter == event_type:
                self._invoke_handler(handler, event, request_id)
                return

        # No handler matched.
        protocol.send_result(plugin_id, request_id, {"handled": False})

    def _invoke_handler(self, handler, event, request_id):
        if _uses_context_handler(handler):
            return handler(EventContext(self, event, request_id))
        return handler(event, request_id)

    def _update_bot_identity(self, event):
        if event.get("event_type") != "bot.identity.changed":
            return

        target = event.get("target") or {}
        target_id = target.get("id") if target.get("type") == "bot" else None
        if target_id:
            self._set_bot_id(str(target_id))
            return

        payload = event.get("payload") or {}
        onebot = payload.get("onebot") or {}
        self_id = onebot.get("self_id")
        if self_id:
            self._set_bot_id(str(self_id))
            return

        # bot.identity.changed with no usable identity means the bot is
        # currently unavailable; downstream send_message / send_reply calls
        # must wait for await_bot_identity before issuing protocol actions.
        self._set_bot_id("")

    def _set_bot_id(self, value):
        bot_id = str(value or "")
        self._bot_id = bot_id
        if bot_id:
            self._bot_identity_event.set()
        else:
            self._bot_identity_event.clear()

    def await_bot_identity(self, timeout_seconds=30):
        """Block until the bot identity is available or the timeout elapses.

        Returns the current ``bot_id`` when available, an empty string when
        the timeout expired without a known identity. Safe to call from event
        handler threads.

        Plugins that need to send outbound messages immediately after
        ``bot.identity.changed`` should call this helper before issuing any
        action; while the identity is unavailable the platform refuses
        identity-dependent OneBot actions.
        """
        if timeout_seconds is None or timeout_seconds < 0:
            timeout_seconds = 0
        # Already known: return immediately.
        if self._bot_identity_event.is_set():
            return self._bot_id
        self._bot_identity_event.wait(timeout=timeout_seconds)
        return self._bot_id

    @property
    def command_prefixes(self):
        return list(self._command_prefixes)

    @property
    def primary_command_prefix(self):
        if self._command_prefixes:
            return self._command_prefixes[0]
        return "/"

    @property
    def bot_id(self):
        return self._bot_id

    @property
    def capabilities(self):
        return list(self._capabilities)

    @property
    def super_admins(self):
        return list(self._super_admins)


def _uses_context_handler(handler):
    try:
        signature = inspect.signature(handler)
    except (TypeError, ValueError):
        return False

    positional = [
        parameter
        for parameter in signature.parameters.values()
        if parameter.kind in (
            inspect.Parameter.POSITIONAL_ONLY,
            inspect.Parameter.POSITIONAL_OR_KEYWORD,
        )
    ]
    return len(positional) == 1


_DEFAULT_TIMESTAMP_HEADER = "X-Raylea-Timestamp"
_DEFAULT_EVENT_ID_HEADER = "X-Raylea-Event-Id"
_DEFAULT_TOLERANCE_SECONDS = 300


def _normalise_replay_protection(value):
    """Normalise replay_protection input into the formal contract shape.

    None defaults to enforce=True with the standard header names. A dict is
    accepted as-is once defaults are filled. The protocol requires the field
    on every event.expose_webhook action.
    """
    if value is None:
        value = {}
    if not isinstance(value, dict):
        raise TypeError("replay_protection must be a dict or None")
    return {
        "timestamp_header": str(value.get("timestamp_header", _DEFAULT_TIMESTAMP_HEADER)),
        "event_id_header": str(value.get("event_id_header", _DEFAULT_EVENT_ID_HEADER)),
        "tolerance_seconds": int(value.get("tolerance_seconds", _DEFAULT_TOLERANCE_SECONDS)),
        "enforce": bool(value.get("enforce", True)),
    }
