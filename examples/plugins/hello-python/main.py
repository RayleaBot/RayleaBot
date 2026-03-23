import json
import sys
import time


def write_frame(frame: dict) -> None:
    sys.stdout.write(json.dumps(frame, ensure_ascii=False) + "\n")
    sys.stdout.flush()


for line in sys.stdin:
    raw = line.strip()
    if not raw:
        continue

    frame = json.loads(raw)
    frame_type = frame.get("type")
    request_id = frame.get("request_id", "")
    plugin_id = frame.get("plugin_id", "hello-python")

    if frame_type == "init":
        write_frame(
            {
                "protocol_version": "1",
                "type": "init_ack",
                "timestamp": int(time.time()),
                "plugin_id": plugin_id,
                "request_id": request_id,
                "status": "ready",
                "subscriptions": ["message.group"],
            }
        )
        continue

    if frame_type == "event":
        event = frame.get("event", {})
        write_frame(
            {
                "protocol_version": "1",
                "type": "result",
                "timestamp": int(time.time()),
                "plugin_id": plugin_id,
                "request_id": request_id,
                "status": "success",
                "data": {
                    "handled": True,
                    "summary": f"hello-python accepted {event.get('event_type', 'unknown')}",
                },
            }
        )
        continue

    # This example intentionally keeps the protocol surface narrow.
    # shutdown, error handling, and other message types stay outside this
    # minimal sample.
