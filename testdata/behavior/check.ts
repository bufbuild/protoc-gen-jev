import assert from "node:assert/strict";
import fs from "node:fs";
import { create, toJson } from "@bufbuild/protobuf";
import { TypeSafeClient } from "@typesafe-ai/sdk";
import { JevContractService } from "../../gen/jev/ai/contract/v1/service_jev.js";
import { EvaluateRequestSchema, EvaluateResponseSchema, Container_NestedRequestSchema } from "../../gen/jev/ai/contract/v1/contract_pb.js";
import { ExternalRequestSchema } from "../../gen/jev/ai/shared/v1/shared_pb.js";

const cases = JSON.parse(fs.readFileSync("testdata/behavior/responses.json", "utf8"));
async function main() {
  for (const tc of cases) {
    // Replace the network boundary only; use the real SDK types and generated Protobuf schemas.
    const sdk = new TypeSafeClient({apiKey:"test"});
    sdk.systemOne = (async (params: {state: unknown}) => {
      assert.deepEqual(params.state, {inputText:"hello", inputCount:"5"});
      return tc.response;
    }) as unknown as typeof sdk.systemOne;
    const client = new JevContractService(sdk);
    const evaluate = () => client.evaluate(create(EvaluateRequestSchema, {inputText:"hello",inputCount:5n}));
    if (tc.error) { await assert.rejects(evaluate, tc.name); continue; }
    const result = await evaluate();
    assert.deepEqual(toJson(EvaluateResponseSchema, result), tc.expected, tc.name);
    assert.equal(typeof result.bigCount, "bigint");
    assert.equal(result.bigCount + 1n, 4n);
  }
  const sdk = new TypeSafeClient({apiKey:"test"});
  sdk.systemOne = (async () => ({answers:{accepted:{type:"noul",noul:0.8}}})) as unknown as typeof sdk.systemOne;
  const client = new JevContractService(sdk);
  assert.equal((await client.external(create(ExternalRequestSchema,{inputText:"hello"}))).accepted,true);
  assert.equal((await client.nested(create(Container_NestedRequestSchema,{inputText:"hello"}))).accepted,true);
  console.log(`TypeScript: ${cases.length} response fixtures and imported/nested RPCs passed`);
}
main().catch(e=>{console.error(e);process.exitCode=1;});
