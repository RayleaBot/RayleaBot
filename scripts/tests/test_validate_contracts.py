from __future__ import annotations

import copy
import importlib.util
import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch


SCRIPT = Path(__file__).resolve().parents[1] / "ci/validate_contracts.py"
SPEC = importlib.util.spec_from_file_location("validate_contracts", SCRIPT)
assert SPEC is not None and SPEC.loader is not None
validator = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(validator)


class ContractValidatorTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.documents = {
            path.resolve(): validator.load_any(path)
            for path in validator.CONTRACTS.iterdir()
            if path.suffix in {".json", ".yaml", ".yml"}
        }
        cls.web_api = cls.documents[(validator.CONTRACTS / "web-api.openapi.yaml").resolve()]
        cls.registry = validator.build_contract_registry(cls.documents)
        cls.catalog = cls.documents[(validator.CONTRACTS / "error-codes.yaml").resolve()]["codes"]
        cls.mappings = validator.load_yaml(validator.EXAMPLES / "http/index.yaml")["examples"]

    def bridge_errors(self, payload: object) -> list[str]:
        return validator.schema_errors_at_pointer(
            validator.CONTRACTS / "plugin-management-ui-bridge.schema.json", self.registry, "", payload,
        )

    def test_devcontainer_base_images_follow_changed_tool_versions(self) -> None:
        versions = validator.read_tool_versions(validator.ROOT)
        dockerfile = (validator.ROOT / ".devcontainer/Dockerfile").read_text(encoding="utf-8")
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            (root / ".devcontainer").mkdir()
            (root / ".devcontainer/Dockerfile").write_text(dockerfile, encoding="utf-8")
            with patch.object(validator, "ROOT", root):
                validator.validate_devcontainer_versions(versions)
                for tool in ("golang", "python"):
                    changed = dict(versions, **{tool: "9.8.7"})
                    with self.subTest(tool=tool), self.assertRaisesRegex(SystemExit, "base images must follow"):
                        validator.validate_devcontainer_versions(changed)

    def test_bridge_valid_handoff_and_resize_boundary(self) -> None:
        for name in ["ok.bridge-page-ready.json", "ok.bridge-host-connect.json", "ok.bridge-ui-resize.json"]:
            with self.subTest(name=name):
                fixture = validator.load_json(validator.FIXTURES / "plugin-management-ui" / name)
                self.assertEqual(self.bridge_errors(fixture["input"]), [])

    def test_bridge_rejects_bad_nonce_type_version_and_height(self) -> None:
        ready = validator.load_json(validator.FIXTURES / "plugin-management-ui/ok.bridge-page-ready.json")["input"]
        resize = validator.load_json(validator.FIXTURES / "plugin-management-ui/ok.bridge-ui-resize.json")["input"]
        cases = [dict(ready, nonce="short"), dict(ready, version="2"), dict(ready, type="settings.delete")]
        for height in [319, 1601, "600"]:
            cases.append(dict(resize, payload={"height": height}))
        for case in cases:
            with self.subTest(case=case):
                self.assertTrue(self.bridge_errors(case))

    def test_bridge_protocol_requests_require_instance(self) -> None:
        for name in ["ok.bridge-protocol-targets-reload.json", "ok.bridge-protocol-identities-resolve.json"]:
            payload = validator.load_json(validator.FIXTURES / "plugin-management-ui" / name)["input"]
            self.assertEqual(self.bridge_errors(payload), [])
            payload["payload"].pop("adapter_id")
            self.assertTrue(self.bridge_errors(payload))

    def test_error_catalog_fixtures_check_shape_and_stable_metadata(self) -> None:
        fixture = validator.load_yaml(validator.FIXTURES / "errors/ok.core-catalog.yaml")
        self.assertEqual(validator.error_fixture_errors(fixture, self.catalog), [])
        for field, value in [("http_status", 418), ("retryable", "false"), ("message_key", "wrong.key"),
                             ("applies_to", ["invented_surface"]), ("code", "unregistered.code")]:
            mutated = copy.deepcopy(fixture)
            mutated["input"]["codes"][0][field] = value
            with self.subTest(field=field):
                self.assertTrue(validator.error_fixture_errors(mutated, self.catalog))
        fixture["input"]["codes"][0].pop("description")
        self.assertTrue(validator.error_fixture_errors(fixture, self.catalog))

    def test_error_catalog_acceptance_is_not_filename_based(self) -> None:
        fixture = validator.load_yaml(validator.FIXTURES / "errors/invalid.missing-required-fields.yaml")
        self.assertTrue(validator.error_fixture_errors(fixture, self.catalog))
        code = fixture["input"]["codes"][0]["code"]
        fixture["input"]["codes"][0] = self.catalog[code]
        errors = validator.error_fixture_errors(fixture, self.catalog)
        self.assertEqual(errors, [])
        with self.assertRaisesRegex(SystemExit, "invalid fixture did not fail"):
            validator.require_fixture_outcome(validator.FIXTURES / "errors/invalid.missing-required-fields.yaml", False, errors)

    def test_error_applicability_detects_semantic_drift(self) -> None:
        fixture = validator.load_yaml(validator.FIXTURES / "errors/edge.applicability.yaml")
        self.assertEqual(validator.error_fixture_errors(fixture, self.catalog), [])
        fixture["input"]["cases"][0]["applies_to"] = ["http"]
        self.assertTrue(validator.error_fixture_errors(fixture, self.catalog))

    def test_error_details_use_declared_schema(self) -> None:
        catalog = copy.deepcopy(self.catalog)
        code = "plugin.install_failed"
        catalog[code]["details_schema"] = {
            "type": "object", "additionalProperties": False, "required": ["operation_state"],
            "properties": {"operation_state": {"enum": ["rolled_back", "rollback_failed"]}},
        }
        fixture = {"input": {"errors": [{"code": code, "message": "安装失败", "applies_to": "task",
                                        "details": {"operation_state": "rolled_back"}}]}}
        self.assertEqual(validator.error_fixture_errors(fixture, catalog), [])
        fixture["input"]["errors"][0]["details"]["operation_state"] = "success"
        self.assertTrue(validator.error_fixture_errors(fixture, catalog))
        catalog[code].pop("details_schema")
        self.assertTrue(validator.error_fixture_errors(fixture, catalog))

    def test_error_details_schema_itself_is_validated(self) -> None:
        entry = dict(self.catalog["plugin.install_failed"], details_schema={"type": "not-a-type"})
        self.assertTrue(validator.error_entry_errors(entry))

    def test_http_status_and_applicability_must_agree_in_both_directions(self) -> None:
        for code, field, value in [("plugin.shutdown", "http_status", 503),
                                   ("platform.internal_error", "http_status", None)]:
            entry = dict(self.catalog[code])
            entry[field] = value
            with self.subTest(code=code):
                self.assertTrue(validator.error_entry_errors(entry))

    def test_websocket_plugin_diagnosis_uses_the_http_definition(self) -> None:
        fixture = validator.load_json(validator.FIXTURES / "websocket/edge.events-received-plugin-initialization-failed.json")
        event = fixture["frame"]["data"]
        document = self.documents[(validator.CONTRACTS / "websocket-events.yaml").resolve()]
        channel_index = next(i for i, channel in enumerate(document["channels"]) if channel["path"] == "/ws/events")
        channel = document["channels"][channel_index]
        event_index = next(i for i, item in enumerate(channel["events"]) if item["event"] == "events.received")
        pointer = f"/channels/{channel_index}/events/{event_index}/payload_schema"
        self.assertEqual(validator.schema_errors_at_pointer(validator.CONTRACTS / "websocket-events.yaml", self.registry, pointer, event), [])
        event["state_diagnosis"]["kind"] = "invented_state"
        self.assertTrue(validator.schema_errors_at_pointer(validator.CONTRACTS / "websocket-events.yaml", self.registry, pointer, event))

    def test_http_error_code_status_scope_and_key_follow_catalog(self) -> None:
        response = {"status": 404, "body": {"error": {
            "code": "platform.resource_not_found", "message_key": "errors.platform.resource_not_found",
        }}}
        self.assertEqual(validator.http_error_catalog_errors(response, self.catalog), [])
        response["status"] = 503
        self.assertTrue(validator.http_error_catalog_errors(response, self.catalog))
        response["status"] = 404
        response["body"]["error"]["message_key"] = "errors.platform.resource_missing"
        self.assertTrue(validator.http_error_catalog_errors(response, self.catalog))
        response["body"]["error"]["code"] = "permission.unavailable"
        self.assertTrue(validator.http_error_catalog_errors(response, self.catalog))
        response["body"]["error"]["code"] = "unknown.error"
        self.assertTrue(validator.http_error_catalog_errors(response, self.catalog))

    def example_errors(self, name: str, instance: object, mapping: dict | None = None) -> list[str]:
        return validator.http_example_errors(self.web_api, self.registry, mapping or self.mappings[name], instance)

    def test_http_request_and_response_examples_follow_openapi_shapes(self) -> None:
        for name in ["governance-blacklist-entry.request.json", "governance-blacklist-entry.response.json"]:
            instance = validator.load_json(validator.EXAMPLES / "http" / name)
            self.assertEqual(self.example_errors(name, instance), [])
            instance.pop("scope")
            self.assertTrue(self.example_errors(name, instance))

    def test_http_examples_validate_operation_status_and_media_type(self) -> None:
        name = "governance-blacklist-entry.response.json"
        instance = validator.load_json(validator.EXAMPLES / "http" / name)
        for field, value in [("operationId", "missingOperation"), ("status", 999), ("status", 418),
                             ("media_type", "text/plain"), ("direction", "input"), ("representation", "guess")]:
            with self.subTest(field=field, value=value):
                self.assertTrue(self.example_errors(name, instance, dict(self.mappings[name], **{field: value})))

    def test_http_request_examples_validate_method_route_and_parameters(self) -> None:
        name = "logs-current-session.request.json"
        instance = validator.load_json(validator.EXAMPLES / "http" / name)
        self.assertEqual(self.example_errors(name, instance), [])
        for change in [{"method": "POST"}, {"path": "/api/not-logs"}, {"path": "/api/logs?limit=invalid"}]:
            with self.subTest(change=change):
                self.assertTrue(self.example_errors(name, dict(instance, **change)))

    def test_shared_body_validation_rejects_missing_json_and_required_request(self) -> None:
        route, method, operation = validator.openapi_operations(self.web_api)["upsertGovernanceBlacklistEntry"]
        pointer = f"/paths/{validator.pointer_escape(route)}/{method}"
        for direction, message in [("request", {}), ("response", {"status": 200})]:
            self.assertTrue(validator.openapi_message_body_errors(
                self.web_api, self.registry, pointer, operation, direction, message,
            ))

    def test_new_example_requires_explicit_mapping(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            examples = Path(directory)
            (examples / "http").mkdir()
            (examples / "http/index.yaml").write_text("examples: {}\n", encoding="utf-8")
            (examples / "http/new.response.json").write_text("{}\n", encoding="utf-8")
            with patch.object(validator, "EXAMPLES", examples):
                with self.assertRaisesRegex(SystemExit, "mappings drift"):
                    validator.validate_http_examples(self.web_api, self.registry)

    def test_openapi_coverage_tracks_new_methods_and_requires_exemption_reason(self) -> None:
        api = {"paths": {"/new": {"get": {"operationId": "readNew"}, "post": {"operationId": "createNew"}}}}
        operations = validator.openapi_operations(api)
        covered = {("/new", "get")}
        self.assertTrue(validator.openapi_coverage_errors(operations, covered))
        operation = api["paths"]["/new"]["post"]
        operation["x-fixture-exemption"] = ""
        self.assertTrue(validator.openapi_coverage_errors(operations, covered))
        operation["x-fixture-exemption"] = "The route upgrades to a stream covered by the transport integration suite."
        self.assertEqual(validator.openapi_coverage_errors(operations, covered), [])
        covered.add(("/new", "post"))
        self.assertTrue(validator.openapi_coverage_errors(operations, covered))
        operation.pop("x-fixture-exemption")
        self.assertEqual(validator.openapi_coverage_errors(operations, covered), [])

    def test_openapi_operation_identifiers_are_unique_and_required(self) -> None:
        for operations in [{"get": {}}, {"get": {"operationId": "same"}, "post": {"operationId": "same"}}]:
            with self.assertRaises(SystemExit):
                validator.openapi_operations({"paths": {"/resource": operations}})

    def test_fixture_requires_contract_reference_even_when_directory_enumerated(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            fixtures = root / "fixtures"
            fixtures.mkdir()
            for name in ["ok.first.json", "ok.orphan.json"]:
                (fixtures / name).write_text(json.dumps({"input": {}}), encoding="utf-8")
            with patch.object(validator, "ROOT", root), patch.object(validator, "FIXTURES", fixtures):
                documents = [{"x-fixtures": ["fixtures/ok.first.json"]}]
                with self.assertRaisesRegex(SystemExit, "missing a contract reference"):
                    validator.validate_fixture_refs(documents)
                documents[0]["x-fixtures"].append("fixtures/ok.orphan.json")
                validator.validate_fixture_refs(documents)


if __name__ == "__main__":
    unittest.main()
