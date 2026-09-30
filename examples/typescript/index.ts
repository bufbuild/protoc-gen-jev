import { TypeSafeClient } from "@typesafe-ai/sdk";
import { create, toJson } from "@bufbuild/protobuf";
import {
  JevIncidentTriageService,
} from "./gen/incident/v1/incident_jev.js";
import {
  TriageRequest,
  TriageRequestSchema,
  TriageResponse,
  TriageDetailsResponseSchema,
  PriorityLevel,
} from "./gen/incident/v1/incident_pb.js";

function printDecision(id: string, d: TriageResponse) {
  const columns = [id, d.requiresImmediatePaging ? "yes" : "no", PriorityLevel[d.priority].replace(/^PRIORITY_LEVEL_/, ""), String(d.urgencyRating), `${d.blastRadiusPercentage.toFixed(1)}%`, d.routingTarget.case?.replace(/[A-Z]/g, c => `_${c.toLowerCase()}`) ?? "none", d.complianceClassification];
  printRow(columns);
}

function printRow(columns: string[]) {
  const widths = [10, 5, 8, 7, 7, 20];
  console.log("  " + columns.map((value, i) => value.padEnd(widths[i] ?? 0)).join(" "));
}

async function main() {
  const verbose = process.env.JEV_VERBOSE === "1";
  const log = (...args: unknown[]) => { if (verbose) console.log(...args); };
  if (!verbose) {
    console.log("TypeScript");
  }

  log("==================================================");
  log("  protoc-gen-jev: TypeScript Client End-to-End Example");
  log("==================================================");

  const apiKey = process.env.TYPESAFE_API_KEY;
  const customEndpoint = process.env.JEV_ENDPOINT;
  let client: JevIncidentTriageService;

  if (customEndpoint) {
    log(`\n[INFO] Using custom Jev/Laya endpoint: ${customEndpoint}`);
    const sdkClient = new TypeSafeClient({
      apiKey: apiKey ?? "local",
      baseURL: customEndpoint,
    });
    client = new JevIncidentTriageService(sdkClient);
  } else if (apiKey) {
    log("\n[INFO] Using live TypeSafe AI Jev SDK with TYPESAFE_API_KEY");
    client = new JevIncidentTriageService();
  } else {
    log("\n[INFO] Using default TypeSafe AI Jev SDK client");
    client = new JevIncidentTriageService();
  }

  // 1. Inspect generated questions
  const questions = client.buildTriageDetailsQuestions();
  log(`\n1. Built Jev Questions (${Object.keys(questions).length} total):`);
  for (const [key, q] of Object.entries(questions)) {
    const crit = (q as any).criteria ? JSON.stringify((q as any).criteria) : undefined;
    log(`  - ${key}: ${(q as any).instructions}`);
    if (crit) {
      log(`      criteria: ${crit}`);
    }
  }

  // 2. Evaluate single input state
  const req: TriageRequest = create(TriageRequestSchema, {
    incidentId: "INC-8891",
    title: "Database connection pool exhausted",
    description: "API latency increased to 4500ms and 500 errors spike to 12%",
    rawLogs: "Connection refused on port 5432 after 100 pool max connections",
  }) as TriageRequest;

  log("\n2. Evaluating Single Incident State...");
  const decision = await client.triageDetails(req);
  console.log(JSON.stringify(toJson(TriageDetailsResponseSchema, decision), null, 2));

  // 3. Batch evaluation
  log("\n3. Batch Evaluating 3 Incident States...");
  const batchReqs: TriageRequest[] = [
    create(TriageRequestSchema, {
      incidentId: "INC-8892",
      title: "Ingress 502 bad gateway spikes across region us-east-1",
      description: "Edge proxy reports connection reset by peer from upstream cluster",
      rawLogs: "HTTP 502 Bad Gateway - upstream connect error or disconnect/reset before headers",
    }) as TriageRequest,
    create(TriageRequestSchema, {
      incidentId: "INC-8893",
      title: "Low-priority deprecation warning logged in analytics service",
      description: "Client library using deprecated v1 query endpoint; scheduled for removal in Q3",
      rawLogs: "WARN [analytics-worker] Endpoint /v1/query is deprecated, migrate to /v2/query",
    }) as TriageRequest,
    create(TriageRequestSchema, {
      incidentId: "INC-8894",
      title: "Routine memory compaction completed without customer impact",
      description: "Background compaction cycle reclaimed 4.2GB memory; latency within SLO",
      rawLogs: "INFO [compactor] Compaction cycle finished in 45s, 0 errors, 4200MB reclaimed",
    }) as TriageRequest,
  ];

  const batchDecisions: TriageResponse[] = await client.batchTriage(batchReqs);
  log(`✔ Successfully evaluated ${batchDecisions.length} batch items.`);
  if (!verbose) {
    console.log("\n  Triage · scalar decisions");
    printRow(["INCIDENT", "PAGE", "PRIORITY", "URGENCY", "BLAST", "ROUTE", "CLASSIFICATION"]);
  }
  batchDecisions.forEach((d, i) => {
    if (!verbose) { printDecision(batchReqs[i].incidentId, d); return; }
    log(
      `  - Item [${i + 1}]: RoutingTarget=${d.routingTarget.case}:${d.routingTarget.value}, ` +
        `Paging=${d.requiresImmediatePaging}, Priority=${PriorityLevel[d.priority]}, ` +
        `Urgency=${d.urgencyRating}, BlastRadius=${d.blastRadiusPercentage}%`,
    );
  });

  log("\n✔ TypeScript End-to-End Test PASSED successfully!");
}

main().catch((err) => {
  console.error("Fatal error running TypeScript example:", err);
  process.exit(1);
});
