"""Typed dataclasses for the RayleaBot plugin JSONL protocol.

All types correspond to ``contracts/plugin-protocol.schema.json``.
Zero external dependencies — only ``dataclasses`` and standard library.
"""

from __future__ import annotations

import time
from dataclasses import dataclass, field, asdict
from typing import Any, Literal

PROTOCOL_VERSION = "1"


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _now() -> int:
    return int(time.time())


def _strip_none(d: dict) -> dict:
    """Recursively remove keys whose value is ``None``."""
    out: dict[str, Any] = {}
    for k, v in d.items():
        if v is None:
            continue
        if isinstance(v, dict):
            out[k] = _strip_none(v)
        elif isinstance(v, list):
            out[k] = [_strip_none(i) if isinstance(i, dict) else i for i in v]
        else:
            out[k] = v
    return out


# ---------------------------------------------------------------------------
# Outbound message segments
# ---------------------------------------------------------------------------

@dataclass(slots=True)
class TextSegment:
    text: str

    def to_dict(self) -> dict:
        return {"type": "text", "data": {"text": self.text}}


@dataclass(slots=True)
class ImageSegment:
    file: str | None = None
    url: str | None = None

    def to_dict(self) -> dict:
        data: dict[str, str] = {}
        if self.file is not None:
            data["file"] = self.file
        if self.url is not None:
            data["url"] = self.url
        return {"type": "image", "data": data}


@dataclass(slots=True)
class AtSegment:
    user_id: str

    def to_dict(self) -> dict:
        return {"type": "at", "data": {"user_id": self.user_id}}


@dataclass(slots=True)
class AtAllSegment:
    def to_dict(self) -> dict:
        return {"type": "at_all"}


@dataclass(slots=True)
class FaceSegment:
    face_id: str

    def to_dict(self) -> dict:
        return {"type": "face", "data": {"face_id": self.face_id}}


@dataclass(slots=True)
class ReplySegment:
    message_id: str

    def to_dict(self) -> dict:
        return {"type": "reply", "data": {"message_id": self.message_id}}


PassthroughSegmentType = Literal[
    "record",
    "video",
    "file",
    "flash_file",
    "json",
    "xml",
    "markdown",
    "music",
    "contact",
    "forward",
    "node",
    "poke",
    "dice",
    "rps",
    "mface",
    "keyboard",
    "shake",
]


@dataclass(slots=True)
class PassthroughSegment:
    segment_type: PassthroughSegmentType
    data: dict[str, Any] = field(default_factory=dict)

    def to_dict(self) -> dict:
        return {"type": self.segment_type, "data": self.data}


Segment = TextSegment | ImageSegment | AtSegment | AtAllSegment | FaceSegment | ReplySegment | PassthroughSegment


def segment_from_dict(d: dict) -> Segment:
    """Reconstruct a segment dataclass from a raw dict."""
    seg_type = d.get("type", "")
    data = d.get("data", {})
    match seg_type:
        case "text":
            return TextSegment(text=data["text"])
        case "image":
            return ImageSegment(file=data.get("file"), url=data.get("url"))
        case "at":
            return AtSegment(user_id=data["user_id"])
        case "at_all":
            return AtAllSegment()
        case "face":
            return FaceSegment(face_id=data["face_id"])
        case "reply":
            return ReplySegment(message_id=data["message_id"])
        case "record" | "video" | "file" | "flash_file" | "json" | "xml" | "markdown" | "music" | "contact" | "forward" | "node" | "poke" | "dice" | "rps" | "mface" | "keyboard" | "shake":
            return PassthroughSegment(segment_type=seg_type, data=data)
        case _:
            raise ValueError(f"unknown segment type: {seg_type}")


def passthrough_segment(
    segment_type: PassthroughSegmentType,
    data: dict[str, Any] | None = None,
) -> PassthroughSegment:
    return PassthroughSegment(segment_type=segment_type, data=data or {})


def record_segment(data: dict[str, Any] | None = None) -> PassthroughSegment:
    return passthrough_segment("record", data)


def video_segment(data: dict[str, Any] | None = None) -> PassthroughSegment:
    return passthrough_segment("video", data)


def file_segment(data: dict[str, Any] | None = None) -> PassthroughSegment:
    return passthrough_segment("file", data)


def flash_file_segment(data: dict[str, Any] | None = None) -> PassthroughSegment:
    return passthrough_segment("flash_file", data)


def json_segment(data: dict[str, Any] | None = None) -> PassthroughSegment:
    return passthrough_segment("json", data)


def xml_segment(data: dict[str, Any] | None = None) -> PassthroughSegment:
    return passthrough_segment("xml", data)


def markdown_segment(content_or_data: str | dict[str, Any] | None = None) -> PassthroughSegment:
    if isinstance(content_or_data, dict):
        return passthrough_segment("markdown", content_or_data)
    if content_or_data is None:
        return passthrough_segment("markdown", {})
    return passthrough_segment("markdown", {"content": content_or_data})


def music_segment(data: dict[str, Any] | None = None) -> PassthroughSegment:
    return passthrough_segment("music", data)


def contact_segment(data: dict[str, Any] | None = None) -> PassthroughSegment:
    return passthrough_segment("contact", data)


def forward_segment(data: dict[str, Any] | None = None) -> PassthroughSegment:
    return passthrough_segment("forward", data)


def node_segment(data: dict[str, Any] | None = None) -> PassthroughSegment:
    return passthrough_segment("node", data)


def poke_segment(data: dict[str, Any] | None = None) -> PassthroughSegment:
    return passthrough_segment("poke", data)


def dice_segment(data: dict[str, Any] | None = None) -> PassthroughSegment:
    return passthrough_segment("dice", data)


def rps_segment(data: dict[str, Any] | None = None) -> PassthroughSegment:
    return passthrough_segment("rps", data)


def mface_segment(data: dict[str, Any] | None = None) -> PassthroughSegment:
    return passthrough_segment("mface", data)


def keyboard_segment(data: dict[str, Any] | None = None) -> PassthroughSegment:
    return passthrough_segment("keyboard", data)


def shake_segment(data: dict[str, Any] | None = None) -> PassthroughSegment:
    return passthrough_segment("shake", data)


# ---------------------------------------------------------------------------
# Nested event sub-objects
# ---------------------------------------------------------------------------

@dataclass(slots=True)
class Bot:
    id: str
    nickname: str | None = None

    def to_dict(self) -> dict:
        return _strip_none(asdict(self))

    @classmethod
    def from_dict(cls, d: dict) -> Bot:
        return cls(id=d["id"], nickname=d.get("nickname"))


@dataclass(slots=True)
class Actor:
    id: str
    nickname: str | None = None
    role: str | None = None

    def to_dict(self) -> dict:
        return _strip_none(asdict(self))

    @classmethod
    def from_dict(cls, d: dict) -> Actor:
        return cls(id=d["id"], nickname=d.get("nickname"), role=d.get("role"))


@dataclass(slots=True)
class Target:
    type: str
    id: str
    name: str | None = None

    def to_dict(self) -> dict:
        return _strip_none(asdict(self))

    @classmethod
    def from_dict(cls, d: dict) -> Target:
        return cls(type=d["type"], id=d["id"], name=d.get("name"))


@dataclass(slots=True)
class OneBotSender:
    user_id: str | None = None
    nickname: str | None = None
    card: str | None = None
    role: str | None = None
    title: str | None = None
    sex: str | None = None
    age: int | None = None

    def to_dict(self) -> dict:
        return _strip_none(asdict(self))

    @classmethod
    def from_dict(cls, d: dict) -> OneBotSender:
        return cls(
            user_id=d.get("user_id"),
            nickname=d.get("nickname"),
            card=d.get("card"),
            role=d.get("role"),
            title=d.get("title"),
            sex=d.get("sex"),
            age=d.get("age"),
        )


@dataclass(slots=True)
class OneBotPayload:
    post_type: str | None = None
    meta_event_type: str | None = None
    message_type: str | None = None
    request_type: str | None = None
    notice_type: str | None = None
    sub_type: str | None = None
    self_id: str | None = None
    user_id: str | None = None
    group_id: str | None = None
    target_id: str | None = None
    time: int | None = None
    interval: int | None = None
    message_id: str | None = None
    real_id: str | None = None
    message_seq: str | None = None
    raw_message: str | None = None
    font: int | None = None
    message_format: str | None = None
    sender: OneBotSender | None = None
    comment: str | None = None
    flag: str | None = None
    status: dict[str, Any] | None = None

    def to_dict(self) -> dict:
        d = _strip_none(asdict(self))
        if self.sender is not None:
            d["sender"] = self.sender.to_dict()
        return d

    @classmethod
    def from_dict(cls, d: dict) -> OneBotPayload:
        return cls(
            post_type=d.get("post_type"),
            meta_event_type=d.get("meta_event_type"),
            message_type=d.get("message_type"),
            request_type=d.get("request_type"),
            notice_type=d.get("notice_type"),
            sub_type=d.get("sub_type"),
            self_id=d.get("self_id"),
            user_id=d.get("user_id"),
            group_id=d.get("group_id"),
            target_id=d.get("target_id"),
            time=d.get("time"),
            interval=d.get("interval"),
            message_id=d.get("message_id"),
            real_id=d.get("real_id"),
            message_seq=d.get("message_seq"),
            raw_message=d.get("raw_message"),
            font=d.get("font"),
            message_format=d.get("message_format"),
            sender=OneBotSender.from_dict(d["sender"]) if "sender" in d else None,
            comment=d.get("comment"),
            flag=d.get("flag"),
            status=d.get("status"),
        )


@dataclass(slots=True)
class EventPayload:
    command: str | None = None
    args: list[str] | None = None
    message_id: str | None = None
    sub_type: str | None = None
    operator_id: str | None = None
    onebot: OneBotPayload | None = None

    def to_dict(self) -> dict:
        d = _strip_none(asdict(self))
        if self.onebot is not None:
            d["onebot"] = self.onebot.to_dict()
        return d

    @classmethod
    def from_dict(cls, d: dict) -> EventPayload:
        return cls(
            command=d.get("command"),
            args=d.get("args"),
            message_id=d.get("message_id"),
            sub_type=d.get("sub_type"),
            operator_id=d.get("operator_id"),
            onebot=OneBotPayload.from_dict(d["onebot"]) if "onebot" in d else None,
        )


@dataclass(slots=True)
class EventMessage:
    segments: list[Segment] = field(default_factory=list)
    plain_text: str | None = None

    def to_dict(self) -> dict:
        d: dict[str, Any] = {"segments": [s.to_dict() for s in self.segments]}
        if self.plain_text is not None:
            d["plain_text"] = self.plain_text
        return d

    @classmethod
    def from_dict(cls, d: dict) -> EventMessage:
        segs = [segment_from_dict(s) for s in d.get("segments", [])]
        return cls(segments=segs, plain_text=d.get("plain_text"))


@dataclass(slots=True)
class EventBody:
    event_id: str
    source_protocol: str
    source_adapter: str
    event_type: str
    timestamp: int
    actor: Actor | None = None
    target: Target | None = None
    payload: EventPayload | None = None
    message: EventMessage | None = None
    raw_payload: dict | None = None

    def to_dict(self) -> dict:
        d: dict[str, Any] = {
            "event_id": self.event_id,
            "source_protocol": self.source_protocol,
            "source_adapter": self.source_adapter,
            "event_type": self.event_type,
            "timestamp": self.timestamp,
        }
        if self.actor is not None:
            d["actor"] = self.actor.to_dict()
        if self.target is not None:
            d["target"] = self.target.to_dict()
        if self.payload is not None:
            d["payload"] = self.payload.to_dict()
        if self.message is not None:
            d["message"] = self.message.to_dict()
        if self.raw_payload is not None:
            d["raw_payload"] = self.raw_payload
        return d

    @classmethod
    def from_dict(cls, d: dict) -> EventBody:
        return cls(
            event_id=d["event_id"],
            source_protocol=d["source_protocol"],
            source_adapter=d["source_adapter"],
            event_type=d["event_type"],
            timestamp=d["timestamp"],
            actor=Actor.from_dict(d["actor"]) if "actor" in d else None,
            target=Target.from_dict(d["target"]) if "target" in d else None,
            payload=EventPayload.from_dict(d["payload"]) if "payload" in d else None,
            message=EventMessage.from_dict(d["message"]) if "message" in d else None,
            raw_payload=d.get("raw_payload"),
        )


# ---------------------------------------------------------------------------
# Protocol frames (platform → plugin)
# ---------------------------------------------------------------------------

@dataclass(slots=True)
class InitFrame:
    plugin_id: str
    request_id: str
    bot: Bot
    capabilities: list[str] = field(default_factory=list)
    command_prefixes: list[str] = field(default_factory=lambda: ["/"])
    timestamp: int = field(default_factory=_now)

    def to_dict(self) -> dict:
        d: dict[str, Any] = {
            "protocol_version": PROTOCOL_VERSION,
            "type": "init",
            "timestamp": self.timestamp,
            "plugin_id": self.plugin_id,
            "request_id": self.request_id,
            "bot": self.bot.to_dict(),
        }
        if self.capabilities:
            d["capabilities"] = self.capabilities
        d["command_prefixes"] = self.command_prefixes or ["/"]
        return d

    @classmethod
    def from_dict(cls, d: dict) -> InitFrame:
        return cls(
            plugin_id=d["plugin_id"],
            request_id=d["request_id"],
            bot=Bot.from_dict(d["bot"]),
            capabilities=d.get("capabilities", []),
            command_prefixes=d.get("command_prefixes", ["/"]),
            timestamp=d.get("timestamp", _now()),
        )


@dataclass(slots=True)
class EventFrame:
    plugin_id: str
    request_id: str
    event: EventBody
    timestamp: int = field(default_factory=_now)

    def to_dict(self) -> dict:
        return {
            "protocol_version": PROTOCOL_VERSION,
            "type": "event",
            "timestamp": self.timestamp,
            "plugin_id": self.plugin_id,
            "request_id": self.request_id,
            "event": self.event.to_dict(),
        }

    @classmethod
    def from_dict(cls, d: dict) -> EventFrame:
        return cls(
            plugin_id=d["plugin_id"],
            request_id=d["request_id"],
            event=EventBody.from_dict(d["event"]),
            timestamp=d.get("timestamp", _now()),
        )


@dataclass(slots=True)
class PingFrame:
    plugin_id: str
    request_id: str
    timestamp: int = field(default_factory=_now)

    def to_dict(self) -> dict:
        return {
            "protocol_version": PROTOCOL_VERSION,
            "type": "ping",
            "timestamp": self.timestamp,
            "plugin_id": self.plugin_id,
            "request_id": self.request_id,
        }

    @classmethod
    def from_dict(cls, d: dict) -> PingFrame:
        return cls(
            plugin_id=d["plugin_id"],
            request_id=d["request_id"],
            timestamp=d.get("timestamp", _now()),
        )


@dataclass(slots=True)
class ShutdownFrame:
    plugin_id: str
    request_id: str
    reason: Literal["stop", "restart", "reload"]
    timestamp: int = field(default_factory=_now)

    def to_dict(self) -> dict:
        return {
            "protocol_version": PROTOCOL_VERSION,
            "type": "shutdown",
            "timestamp": self.timestamp,
            "plugin_id": self.plugin_id,
            "request_id": self.request_id,
            "reason": self.reason,
        }

    @classmethod
    def from_dict(cls, d: dict) -> ShutdownFrame:
        return cls(
            plugin_id=d["plugin_id"],
            request_id=d["request_id"],
            reason=d["reason"],
            timestamp=d.get("timestamp", _now()),
        )


@dataclass(slots=True)
class ResultFrame:
    plugin_id: str
    request_id: str
    data: dict = field(default_factory=dict)
    timestamp: int = field(default_factory=_now)

    def to_dict(self) -> dict:
        return {
            "protocol_version": PROTOCOL_VERSION,
            "type": "result",
            "timestamp": self.timestamp,
            "plugin_id": self.plugin_id,
            "request_id": self.request_id,
            "status": "success",
            "data": self.data,
        }

    @classmethod
    def from_dict(cls, d: dict) -> ResultFrame:
        return cls(
            plugin_id=d["plugin_id"],
            request_id=d["request_id"],
            data=d.get("data", {}),
            timestamp=d.get("timestamp", _now()),
        )


@dataclass(slots=True)
class ErrorFrame:
    plugin_id: str
    request_id: str
    code: str
    message: str
    details: dict | None = None
    timestamp: int = field(default_factory=_now)

    def to_dict(self) -> dict:
        d: dict[str, Any] = {
            "protocol_version": PROTOCOL_VERSION,
            "type": "error",
            "timestamp": self.timestamp,
            "plugin_id": self.plugin_id,
            "request_id": self.request_id,
            "code": self.code,
            "message": self.message,
        }
        if self.details is not None:
            d["details"] = self.details
        return d

    @classmethod
    def from_dict(cls, d: dict) -> ErrorFrame:
        return cls(
            plugin_id=d["plugin_id"],
            request_id=d["request_id"],
            code=d["code"],
            message=d["message"],
            details=d.get("details"),
            timestamp=d.get("timestamp", _now()),
        )


# ---------------------------------------------------------------------------
# Protocol frames (plugin → platform)
# ---------------------------------------------------------------------------

@dataclass(slots=True)
class InitAckFrame:
    plugin_id: str
    request_id: str
    status: Literal["ready", "error"] = "ready"
    subscriptions: list[str] | None = None
    error_message: str | None = None
    timestamp: int = field(default_factory=_now)

    def to_dict(self) -> dict:
        d: dict[str, Any] = {
            "protocol_version": PROTOCOL_VERSION,
            "type": "init_ack",
            "timestamp": self.timestamp,
            "plugin_id": self.plugin_id,
            "request_id": self.request_id,
            "status": self.status,
        }
        if self.subscriptions is not None:
            d["subscriptions"] = self.subscriptions
        if self.error_message is not None:
            d["error_message"] = self.error_message
        return d

    @classmethod
    def from_dict(cls, d: dict) -> InitAckFrame:
        return cls(
            plugin_id=d["plugin_id"],
            request_id=d["request_id"],
            status=d.get("status", "ready"),
            subscriptions=d.get("subscriptions"),
            error_message=d.get("error_message"),
            timestamp=d.get("timestamp", _now()),
        )


@dataclass(slots=True)
class InitProgressFrame:
    plugin_id: str
    request_id: str
    summary: str
    timestamp: int = field(default_factory=_now)

    def to_dict(self) -> dict:
        return {
            "protocol_version": PROTOCOL_VERSION,
            "type": "init_progress",
            "timestamp": self.timestamp,
            "plugin_id": self.plugin_id,
            "request_id": self.request_id,
            "summary": self.summary,
        }

    @classmethod
    def from_dict(cls, d: dict) -> InitProgressFrame:
        return cls(
            plugin_id=d["plugin_id"],
            request_id=d["request_id"],
            summary=d["summary"],
            timestamp=d.get("timestamp", _now()),
        )


@dataclass(slots=True)
class PongFrame:
    plugin_id: str
    request_id: str
    timestamp: int = field(default_factory=_now)

    def to_dict(self) -> dict:
        return {
            "protocol_version": PROTOCOL_VERSION,
            "type": "pong",
            "timestamp": self.timestamp,
            "plugin_id": self.plugin_id,
            "request_id": self.request_id,
        }

    @classmethod
    def from_dict(cls, d: dict) -> PongFrame:
        return cls(
            plugin_id=d["plugin_id"],
            request_id=d["request_id"],
            timestamp=d.get("timestamp", _now()),
        )


@dataclass(slots=True)
class ActionFrame:
    plugin_id: str
    request_id: str
    action: str
    data: dict
    parent_request_id: str | None = None
    timestamp: int = field(default_factory=_now)

    def to_dict(self) -> dict:
        frame = {
            "protocol_version": PROTOCOL_VERSION,
            "type": "action",
            "timestamp": self.timestamp,
            "plugin_id": self.plugin_id,
            "request_id": self.request_id,
            "action": self.action,
            "data": self.data,
        }
        if self.parent_request_id is not None:
            frame["parent_request_id"] = self.parent_request_id
        return frame

    @classmethod
    def from_dict(cls, d: dict) -> ActionFrame:
        return cls(
            plugin_id=d["plugin_id"],
            request_id=d["request_id"],
            parent_request_id=d.get("parent_request_id"),
            action=d["action"],
            data=d["data"],
            timestamp=d.get("timestamp", _now()),
        )


# ---------------------------------------------------------------------------
# Discriminated union + parser
# ---------------------------------------------------------------------------

Frame = (
    InitFrame
    | InitProgressFrame
    | InitAckFrame
    | EventFrame
    | ActionFrame
    | ResultFrame
    | ErrorFrame
    | PingFrame
    | PongFrame
    | ShutdownFrame
)


def frame_from_dict(d: dict) -> Frame:
    """Parse a raw dict into the appropriate frame dataclass."""
    frame_type = d.get("type", "")
    match frame_type:
        case "init":
            return InitFrame.from_dict(d)
        case "init_progress":
            return InitProgressFrame.from_dict(d)
        case "init_ack":
            return InitAckFrame.from_dict(d)
        case "event":
            return EventFrame.from_dict(d)
        case "action":
            return ActionFrame.from_dict(d)
        case "result":
            return ResultFrame.from_dict(d)
        case "error":
            return ErrorFrame.from_dict(d)
        case "ping":
            return PingFrame.from_dict(d)
        case "pong":
            return PongFrame.from_dict(d)
        case "shutdown":
            return ShutdownFrame.from_dict(d)
        case _:
            raise ValueError(f"unknown frame type: {frame_type}")
