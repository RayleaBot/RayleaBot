"""High-level plugin framework for RayleaBot Python plugins."""

import threading

from rayleabot import protocol


class RayleaBotPlugin:
    """Base class for RayleaBot plugins with event and command handler registration."""

    def __init__(self):
        self._event_handlers = []
        self._command_handlers = {}
        self._active_handlers = set()
        self._handler_lock = threading.Lock()
        self._plugin_id = ""
        self._bot_id = ""
        self._capabilities = []
        self._command_prefixes = ["/"]
        self._subscriptions = None

    def on_event(self, event_type=None):
        """Decorator to register an event handler. If event_type is None, matches all events."""
        def decorator(func):
            self._event_handlers.append((event_type, func))
            return func
        return decorator

    def on_command(self, name, aliases=None):
        """Decorator to register a command handler by name and optional aliases."""
        def decorator(func):
            self._command_handlers[name] = func
            if aliases:
                for alias in aliases:
                    self._command_handlers[alias] = func
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

    def scheduler_create(self, request_id, task_id, cron, payload=None, timeout_seconds=30):
        """Create or update one scheduler.trigger task through scheduler.create."""
        data = {
            "task_id": task_id,
            "cron": cron,
            "event_type": "scheduler.trigger",
        }
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

    def plugin_list(self, request_id, timeout_seconds=30):
        """List installed plugins through plugin.list."""
        return protocol.request_local_action(
            self._plugin_id,
            request_id,
            "plugin.list",
            {},
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

    def message_history_get(self, request_id, conversation_type, conversation_id, limit=None, timeout_seconds=30):
        data = {
            "conversation_type": conversation_type,
            "conversation_id": conversation_id,
        }
        if limit is not None:
            data["limit"] = limit
        return self.onebot_action(request_id, "message.history.get", data, timeout_seconds=timeout_seconds)

    def group_announcement_create(self, request_id, group_id, content, timeout_seconds=30):
        return self.onebot_action(
            request_id,
            "group.announcement.create",
            {"group_id": group_id, "content": content},
            timeout_seconds=timeout_seconds,
        )

    def file_group_upload(self, request_id, group_id, file_name, file_url, timeout_seconds=30):
        return self.onebot_action(
            request_id,
            "file.group.upload",
            {"group_id": group_id, "file_name": file_name, "file_url": file_url},
            timeout_seconds=timeout_seconds,
        )

    def reaction_set(self, request_id, message_id, emoji, enabled=True, timeout_seconds=30):
        return self.onebot_action(
            request_id,
            "reaction.set",
            {"message_id": message_id, "emoji": emoji, "enabled": enabled},
            timeout_seconds=timeout_seconds,
        )

    def poke_send(self, request_id, target_type, target_id, user_id, timeout_seconds=30):
        return self.onebot_action(
            request_id,
            "poke.send",
            {"target_type": target_type, "target_id": target_id, "user_id": user_id},
            timeout_seconds=timeout_seconds,
        )

    def napcat_message_emoji_like_set(self, request_id, message_id, emoji_id, enabled=True, timeout_seconds=30):
        return self.provider_action(
            request_id,
            "napcat",
            "message_emoji.like.set",
            {"message_id": message_id, "emoji_id": emoji_id, "enabled": enabled},
            timeout_seconds=timeout_seconds,
        )

    def luckylillia_friend_groups_get(self, request_id, user_id, timeout_seconds=30):
        return self.provider_action(
            request_id,
            "luckylillia",
            "friend_groups.get",
            {"user_id": user_id},
            timeout_seconds=timeout_seconds,
        )

    storageFileRead = storage_file_read
    storageFileWrite = storage_file_write
    storageFileDelete = storage_file_delete
    storageFileList = storage_file_list
    httpRequest = http_request
    configRead = config_read
    configWrite = config_write
    schedulerCreate = scheduler_create
    exposeWebhook = expose_webhook
    renderImage = render_image
    pluginList = plugin_list
    onebotAction = onebot_action
    providerAction = provider_action
    messageHistoryGet = message_history_get
    groupAnnouncementCreate = group_announcement_create
    fileGroupUpload = file_group_upload
    reactionSet = reaction_set
    pokeSend = poke_send
    napcatMessageEmojiLikeSet = napcat_message_emoji_like_set
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
                self._bot_id = bot.get("id", "")
                self._capabilities = frame.get("capabilities", [])
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
        payload = event.get("payload", {})
        command = payload.get("command")

        # Try command handler first.
        if command and command in self._command_handlers:
            handler = self._command_handlers[command]
            handler(event, request_id)
            return

        # Try event type handlers.
        for type_filter, handler in self._event_handlers:
            if type_filter is None or type_filter == event_type:
                handler(event, request_id)
                return

        # No handler matched.
        protocol.send_result(plugin_id, request_id, {"handled": False})

    @property
    def command_prefixes(self):
        return list(self._command_prefixes)

    @property
    def primary_command_prefix(self):
        if self._command_prefixes:
            return self._command_prefixes[0]
        return "/"
