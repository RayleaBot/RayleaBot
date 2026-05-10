import json


class FakePluginContext:
    def __init__(
        self,
        args=None,
        request_id="req_plugin_test",
        target_type="group",
        target_id="10000",
        actor=None,
        config_values=None,
        http_responses=None,
        secrets=None,
        storage=None,
        render_result=None,
    ):
        self.args = args or []
        self.request_id = request_id
        self.target_type = target_type
        self.target_id = target_id
        self.actor = actor or {"id": "42", "nickname": "订阅人"}
        self.config_values = config_values or {}
        self.http_responses = list(http_responses or [])
        self.secrets = secrets or {}
        self.storage = storage or {}
        self.render_result = {"image_path": "plugin-test.png"} if render_result is None else render_result
        self.config_writes = []
        self.scheduler_creates = []
        self.texts = []
        self.text_messages = self.texts
        self.results = []
        self.logs = []
        self.http_requests = []
        self.render_calls = []
        self.messages = []
        self.storage_sets = []
        self.actions = []

    def config_read(self, keys):
        return {"values": {key: self.config_values[key] for key in keys if key in self.config_values}}

    def config_write(self, values):
        self.config_writes.append(values)
        return {"changed_keys": sorted(values.keys())}

    def scheduler_create(self, task_id, cron, payload=None):
        self.scheduler_creates.append({"task_id": task_id, "cron": cron, "payload": payload})
        return {"task_id": task_id}

    def send_text(self, text):
        self.texts.append(text)

    def send_result(self, result):
        self.results.append(result)

    def logger_write(self, level, message, fields=None):
        self.logs.append({"level": level, "message": message, "fields": fields or {}})
        return {"ok": True}

    def http_request(self, method, url, headers=None, timeout_seconds=30):
        self.http_requests.append({
            "method": method,
            "url": url,
            "headers": headers or {},
            "timeout_seconds": timeout_seconds,
        })
        if self.http_responses:
            return self.http_responses.pop(0)
        return {"status_code": 200, "body_text": json.dumps({"code": 0, "data": {"items": []}})}

    def secret_read(self, secret_key):
        return {"value": self.secrets.get(secret_key, "")}

    def storage_get(self, key):
        if key in self.storage:
            return {"exists": True, "value": self.storage[key]}
        return {"exists": False}

    def storage_set(self, key, value):
        self.actions.append({"kind": "storage_set", "key": key, "value": value})
        self.storage_sets.append({"key": key, "value": value})
        self.storage[key] = value
        return {"ok": True}

    def render_image(self, template, data, theme, output, fallback_text):
        call = {
            "template": template,
            "data": data,
            "theme": theme,
            "output": output,
            "fallback_text": fallback_text,
        }
        self.actions.append({"kind": "render_image", "call": call})
        self.render_calls.append(call)
        return self.render_result

    def send_message(self, segments, target_type=None, target_id=None):
        message = {"segments": segments, "target_type": target_type, "target_id": target_id}
        self.actions.append({"kind": "send_message", "message": message})
        self.messages.append(message)
