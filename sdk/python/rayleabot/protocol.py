"""JSONL stdin/stdout protocol implementation for RayleaBot plugins."""

import json
import sys
import time


def read_frame():
    """Read and parse one JSONL frame from stdin."""
    line = sys.stdin.readline()
    if not line:
        return None
    return json.loads(line.strip())


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
    """Send an outbound action (message.send, message.reply, message.send_image)."""
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
