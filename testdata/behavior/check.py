"""Shared response contract checks against generated Python clients."""
import json
import importlib
import shutil
import sys
import tempfile
from pathlib import Path
from typing import Any, cast

sys.path.insert(0, str(Path("gen").resolve()))
from typesafe_sdk import TypeSafeClient
from jev.ai.contract.v1.service_jev import JevContractService
from jev.ai.contract.v1.contract_pb import EvaluateRequest, Container
from jev.ai.shared.v1.shared_pb import ExternalRequest

class Fake:
    def __init__(self, response: dict[str, Any], check_state: bool = True):
        self.response = response
        self.check_state = check_state

    def system_one(self, **kwargs: Any) -> dict[str, Any]:
        if self.check_state:
            assert kwargs["state"] == {"inputText":"hello", "inputCount":"5"}
        return self.response

cases = json.loads(Path("testdata/behavior/responses.json").read_text())
for tc in cases:
    client = JevContractService(client=cast(TypeSafeClient, Fake(tc["response"])))
    try:
        result = client.evaluate(EvaluateRequest(input_text="hello", input_count=5))
    except (ValueError, TypeError):
        assert tc.get("error"), tc["name"]
        continue
    assert not tc.get("error"), tc["name"]
    assert json.loads(result.to_json()) == tc["expected"], (tc["name"], result)
    assert result.has_field("flag") and result.has_field("rating")

client = JevContractService(client=cast(TypeSafeClient, Fake({"answers":{"accepted":{"type":"noul", "noul":0.8}}}, False)))
assert client.external(ExternalRequest(input_text="hello")).accepted
assert client.nested(Container.NestedRequest(input_text="hello")).accepted
print(f"Python: {len(cases)} response fixtures and imported/nested RPCs passed")

# Move the complete generated package to a different name. Assert that response
# types belong to that package too, catching accidental imports from the original.
with tempfile.TemporaryDirectory() as root:
    shutil.copytree("gen/jev", Path(root) / "contracts", ignore=shutil.ignore_patterns("__pycache__"))
    sys.path.insert(0, root)
    try:
        service = importlib.import_module("contracts.ai.contract.v1.service_jev")
        contract = importlib.import_module("contracts.ai.contract.v1.contract_pb")
        shared = importlib.import_module("contracts.ai.shared.v1.shared_pb")
        relocated = service.JevContractService(client=Fake({"answers":{"accepted":{"type":"noul", "noul":0.8}}}, False))
        external = relocated.external(shared.ExternalRequest(input_text="hello"))
        nested = relocated.nested(contract.Container.NestedRequest(input_text="hello"))
        assert isinstance(external, shared.ExternalResponse) and external.accepted
        assert isinstance(nested, contract.Container.NestedResponse) and nested.accepted
    finally:
        sys.path.remove(root)
print("Python: relocated package imports and response types passed")
