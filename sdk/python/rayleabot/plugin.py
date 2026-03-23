"""High-level plugin framework for RayleaBot Python plugins."""

from rayleabot import protocol


class RayleaBotPlugin:
    """Base class for RayleaBot plugins with event and command handler registration."""

    def __init__(self):
        self._event_handlers = []
        self._command_handlers = {}
        self._plugin_id = ""
        self._bot_id = ""
        self._capabilities = []
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

    def send_message(self, request_id, target_type, target_id, text):
        """Send a message to a target."""
        protocol.send_action(self._plugin_id, request_id, "message.send", {
            "target_type": target_type,
            "target_id": target_id,
            "text": text,
        })

    def send_reply(self, request_id, reply_to_message_id, text):
        """Reply to a specific message."""
        protocol.send_action(self._plugin_id, request_id, "message.reply", {
            "reply_to_message_id": reply_to_message_id,
            "text": text,
        })

    def send_message_segments(self, request_id, target_type, target_id, segments):
        """Send a rich message to a target using shared message.segments."""
        protocol.send_action(self._plugin_id, request_id, "message.send", {
            "target_type": target_type,
            "target_id": target_id,
            "message": {
                "segments": segments,
            },
        })

    def reply_to_event(self, request_id, reply_to_event_id, segments, fallback_to_send_if_missing=False):
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

    sendMessageSegments = send_message_segments
    replyToEvent = reply_to_event

    def send_image(self, request_id, target_type, target_id, file):
        """Send an image to a target."""
        protocol.send_action(self._plugin_id, request_id, "message.send_image", {
            "target_type": target_type,
            "target_id": target_id,
            "file": file,
        })

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

    storageFileRead = storage_file_read
    storageFileWrite = storage_file_write
    storageFileDelete = storage_file_delete
    storageFileList = storage_file_list
    httpRequest = http_request

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
                protocol.send_init_ack(plugin_id, request_id, self._subscriptions)

            elif frame_type == "event":
                self._handle_event(frame, plugin_id, request_id)

            elif frame_type == "ping":
                protocol.send_pong(plugin_id, request_id)

            elif frame_type == "shutdown":
                break

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
