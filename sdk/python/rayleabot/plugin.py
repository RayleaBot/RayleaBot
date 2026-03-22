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
