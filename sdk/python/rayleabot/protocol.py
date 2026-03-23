"""JSONL stdin/stdout protocol implementation for RayleaBot plugins."""

import json
import queue
import sys
import threading
import time

_frame_queue = queue.Queue()
_reader_started = False
_reader_lock = threading.Lock()
_local_request_counter = 0


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
            _frame_queue.put(None)
            return
        line = line.strip()
        if not line:
            continue
        _frame_queue.put(json.loads(line))


def _ensure_reader():
    global _reader_started
    with _reader_lock:
        if _reader_started:
            return
        thread = threading.Thread(target=_reader_loop, daemon=True)
        thread.start()
        _reader_started = True


def read_frame(timeout_seconds=None):
    """Read and parse one JSONL frame from stdin."""
    if timeout_seconds is None and not _reader_started:
        line = sys.stdin.readline()
        if not line:
            return None
        line = line.strip()
        if not line:
            return read_frame(timeout_seconds=None)
        return json.loads(line)

    _ensure_reader()
    try:
        frame = _frame_queue.get(timeout=timeout_seconds)
    except queue.Empty as exc:
        raise TimeoutError("timed out waiting for platform frame") from exc
    return frame


def write_frame(frame):
    """Write a JSONL frame to stdout."""
    sys.stdout.write(json.dumps(frame, ensure_ascii=False) + "\n")
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


def send_action(plugin_id, request_id, action, data):
    """Send an outbound action."""
    write_frame({
        "protocol_version": "1",
        "type": "action",
        "timestamp": int(time.time()),
        "plugin_id": plugin_id,
        "request_id": request_id,
        "action": action,
        "data": data,
    })


def send_error(plugin_id, request_id, code, message):
    """Send an error frame."""
    write_frame({
        "protocol_version": "1",
        "type": "error",
        "timestamp": int(time.time()),
        "plugin_id": plugin_id,
        "request_id": request_id,
        "code": code,
        "message": message,
    })


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
    send_action(plugin_id, request_id, action, data)
    deadline = time.monotonic() + timeout_seconds

    while True:
        remaining = deadline - time.monotonic()
        if remaining <= 0:
            raise TimeoutError(f"timed out waiting for local action response: {action}")

        frame = read_frame(timeout_seconds=remaining)
        if frame is None:
            raise ProtocolError("stdin closed while waiting for local action response")

        frame_type = frame.get("type")
        frame_request_id = frame.get("request_id")

        if frame_type == "ping":
            send_pong(plugin_id, frame_request_id)
            continue

        if frame_type == "shutdown":
            raise ProtocolError("received shutdown while waiting for local action response")

        if frame_request_id != request_id:
            raise ProtocolError(f"unexpected frame while waiting for local action response: {frame_type}")

        if frame_type == "result":
            return frame.get("data", {})

        if frame_type == "error":
            raise ActionError(frame.get("code", "plugin.internal_error"), frame.get("message", "local action failed"), frame.get("details"))

        raise ProtocolError(f"unexpected frame type while waiting for local action response: {frame_type}")
