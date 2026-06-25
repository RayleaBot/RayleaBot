"""JSONL stdin/stdout protocol implementation for RayleaBot plugins."""

import json
import queue
import re
import sys
import threading
import time

_frame_queue = queue.Queue()
_pending_requests = {}
_reader_started = False
_state_lock = threading.Lock()
_write_lock = threading.Lock()
_local_request_counter = 0
_SENSITIVE_TEXT_PATTERNS = (
    re.compile(r"(?i)\b(SESSDATA|bili_jct|access_token|refresh_token|authorization|cookie|token|secret|password)\b(\s*[:=]\s*)([^;,\s]+)"),
)


class ProtocolError(RuntimeError):
    """Raised when the platform violates the expected local action flow."""


class ActionError(RuntimeError):
    """Raised when a local platform action returns an error frame."""

    def __init__(self, code, message, details=None):
        super().__init__(message)
        self.code = code
        self.details = details or {}


def _reader_loop():
    while True:
        line = sys.stdin.readline()
        if not line:
            _close_stream(ProtocolError("stdin closed while waiting for local action response"))
            return
        line = line.strip()
        if not line:
            continue
        try:
            frame = json.loads(line)
        except json.JSONDecodeError as exc:
            _close_stream(ProtocolError(f"received malformed protocol json: {exc.msg}"))
            return
        _dispatch_frame(frame)


def _ensure_reader():
    global _reader_started
    with _state_lock:
        if _reader_started:
            return
        thread = threading.Thread(target=_reader_loop, daemon=True)
        thread.start()
        _reader_started = True


def read_frame(timeout_seconds=None):
    """Read and parse one JSONL frame from stdin."""
    _ensure_reader()
    try:
        frame = _frame_queue.get(timeout=timeout_seconds)
    except queue.Empty as exc:
        raise TimeoutError("timed out waiting for platform frame") from exc
    return frame


def write_frame(frame):
    """Write one JSONL protocol frame to stdout."""
    line = json.dumps(frame, ensure_ascii=False) + "\n"
    encoded = line.encode("utf-8")
    with _write_lock:
        sys.stdout.buffer.write(encoded)
        sys.stdout.flush()


def send_init_ack(plugin_id, request_id, subscriptions=None):
    """Send init_ack with status=ready."""
    frame = {
        "protocol_version": "1",
        "type": "init_ack",
        "timestamp": int(time.time()),
        "plugin_id": plugin_id,
        "request_id": request_id,
        "status": "ready",
    }
    if subscriptions:
        frame["subscriptions"] = subscriptions
    write_frame(frame)


def send_pong(plugin_id, request_id):
    """Respond to a ping frame."""
    write_frame({
        "protocol_version": "1",
        "type": "pong",
        "timestamp": int(time.time()),
        "plugin_id": plugin_id,
        "request_id": request_id,
    })


def send_result(plugin_id, request_id, data=None):
    """Send a success result for an event."""
    write_frame({
        "protocol_version": "1",
        "type": "result",
        "timestamp": int(time.time()),
        "plugin_id": plugin_id,
        "request_id": request_id,
        "status": "success",
        "data": data or {},
    })


def send_action(plugin_id, request_id, action, data, parent_request_id=None):
    """Send an outbound action."""
    frame = {
        "protocol_version": "1",
        "type": "action",
        "timestamp": int(time.time()),
        "plugin_id": plugin_id,
        "request_id": request_id,
        "action": action,
        "data": data,
    }
    if parent_request_id:
        frame["parent_request_id"] = parent_request_id
    write_frame(frame)


def send_error(plugin_id, request_id, code, message):
    """Send an error frame."""
    write_frame({
        "protocol_version": "1",
        "type": "error",
        "timestamp": int(time.time()),
        "plugin_id": plugin_id,
        "request_id": request_id,
        "code": code,
        "message": redact_sensitive_text(message),
    })


def redact_sensitive_text(value):
    """Redact credential-shaped fragments from user-visible protocol errors."""
    text = str(value or "")
    for pattern in _SENSITIVE_TEXT_PATTERNS:
        text = pattern.sub(r"\1\2[REDACTED]", text)
    return text


def next_local_request_id(parent_request_id):
    """Generate a local action request_id distinct from the parent event request_id."""
    global _local_request_counter
    _local_request_counter += 1
    request_id = f"local_{int(time.time() * 1000)}_{_local_request_counter}"
    if request_id == parent_request_id:
        request_id += "_1"
    return request_id


def request_local_action(plugin_id, parent_request_id, action, data, timeout_seconds=30):
    """Send a local platform action and wait for the matching result/error frame."""
    request_id = next_local_request_id(parent_request_id)
    _ensure_reader()
    response_queue = queue.Queue(maxsize=1)
    with _state_lock:
        _pending_requests[request_id] = response_queue

    try:
        send_action(plugin_id, request_id, action, data, parent_request_id=parent_request_id)
    except Exception:
        with _state_lock:
            _pending_requests.pop(request_id, None)
        raise

    try:
        frame = response_queue.get(timeout=timeout_seconds)
    except queue.Empty as exc:
        with _state_lock:
            _pending_requests.pop(request_id, None)
        raise TimeoutError(f"timed out waiting for local action response: {action}") from exc

    if isinstance(frame, Exception):
        raise frame

    frame_type = frame.get("type")
    if frame_type == "result":
        return frame.get("data", {})

    if frame_type == "error":
        raise ActionError(frame.get("code", "plugin.internal_error"), frame.get("message", "local action failed"), frame.get("details"))

    raise ProtocolError(f"unexpected frame type while waiting for local action response: {frame_type}")


def _dispatch_frame(frame):
    frame_type = frame.get("type")
    request_id = frame.get("request_id")

    with _state_lock:
        response_queue = None
        if frame_type in ("result", "error"):
            response_queue = _pending_requests.pop(request_id, None)

    if response_queue is not None:
        response_queue.put(frame)
        return

    _frame_queue.put(frame)
    if frame_type == "shutdown":
        _reject_pending_requests(ProtocolError("received shutdown while waiting for local action response"))


def _reject_pending_requests(error):
    with _state_lock:
        if not _pending_requests:
            return
        pending = list(_pending_requests.values())
        _pending_requests.clear()

    for response_queue in pending:
        response_queue.put(error)


def _close_stream(error):
    _reject_pending_requests(error)
    _frame_queue.put(None)
