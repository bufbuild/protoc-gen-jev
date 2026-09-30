#!/usr/bin/env python3
"""protoc-gen-jev: Python Client End-to-End Example."""

import json
import os
import sys
from pathlib import Path

# Add local gen to module path (jev package lives at examples/python/gen/jev)
sys.path.insert(0, str(Path(__file__).resolve().parent / "gen"))

from jev.incident.v1.incident_pb import (
    TriageRequest,
    TriageResponse,
)
from jev.incident.v1.incident_jev import (
    JevIncidentTriageService,
)
from typesafe_sdk import TypeSafeClient


def print_decision(incident_id: str, decision: TriageResponse):
    route = decision.routing_target.field if decision.routing_target else "none"
    page = "yes" if decision.requires_immediate_paging else "no"
    priority = decision.priority.name.removeprefix("PRIORITY_LEVEL_")
    blast = f"{decision.blast_radius_percentage:.1f}%"
    print(f"  {incident_id:<10} {page:<5} {priority:<8} {decision.urgency_rating:<7} {blast:<7} {route:<20} {decision.compliance_classification}")


def main():
    verbose = os.getenv("JEV_VERBOSE") == "1"
    def log(*args, **kwargs):
        if verbose:
            print(*args, **kwargs)

    if not verbose:
        print("Python", flush=True)

    log("==================================================")
    log("  protoc-gen-jev: Python Client End-to-End Example")
    log("==================================================")

    api_key = os.getenv("TYPESAFE_API_KEY")
    custom_endpoint = os.getenv("JEV_ENDPOINT")

    if custom_endpoint:
        log(f"\n[INFO] Using custom Jev/Laya endpoint: {custom_endpoint}")
        # A local Laya server on CPU can take well over the SDK default of
        # 10 seconds per request, so allow a generous per-attempt timeout.
        sdk_client = TypeSafeClient(
            api_key=api_key or "local",
            base_url=custom_endpoint,
            timeout=120.0,
        )
        client = JevIncidentTriageService(client=sdk_client)
    elif api_key:
        log("\n[INFO] Using live TypeSafe AI Jev SDK with TYPESAFE_API_KEY")
        client = JevIncidentTriageService(api_key=api_key)
    else:
        log("\n[INFO] Using default TypeSafe AI Jev SDK client")
        client = JevIncidentTriageService()

    # 1. Inspect generated questions
    questions = client.build_triage_details_questions()
    log(f"\n1. Built Jev Questions ({len(questions)} total):")
    for q_name, q in questions.items():
        crit = getattr(q, "criteria", None)
        log(f"  - {q_name} [{q.type}]: {q.instructions}")
        if crit:
            log(f"      criteria: {crit}")

    # 2. Evaluate single input state
    req = TriageRequest(
        incident_id="INC-8891",
        title="Database connection pool exhausted",
        description="API latency increased to 4500ms and 500 errors spike to 12%",
        raw_logs="Connection refused on port 5432 after 100 pool max connections",
    )

    log("\n2. Evaluating Single Incident State...")
    decision = client.triage_details(req)
    print(json.dumps(json.loads(decision.to_json()), indent=2))

    # 3. Batch evaluation
    log("\n3. Batch Evaluating 3 Incident States...")
    batch_reqs = [
        TriageRequest(
            incident_id="INC-8892",
            title="Ingress 502 bad gateway spikes across region us-east-1",
            description="Edge proxy reports connection reset by peer from upstream cluster",
            raw_logs="HTTP 502 Bad Gateway - upstream connect error or disconnect/reset before headers",
        ),
        TriageRequest(
            incident_id="INC-8893",
            title="Low-priority deprecation warning logged in analytics service",
            description="Client library using deprecated v1 query endpoint; scheduled for removal in Q3",
            raw_logs="WARN [analytics-worker] Endpoint /v1/query is deprecated, migrate to /v2/query",
        ),
        TriageRequest(
            incident_id="INC-8894",
            title="Routine memory compaction completed without customer impact",
            description="Background compaction cycle reclaimed 4.2GB memory; latency within SLO",
            raw_logs="INFO [compactor] Compaction cycle finished in 45s, 0 errors, 4200MB reclaimed",
        ),
    ]

    batch_decisions = client.batch_triage(batch_reqs)
    log(f"✔ Successfully evaluated {len(batch_decisions)} batch items.")
    if not verbose:
        print("\n  Triage · scalar decisions")
        print(f"  {'INCIDENT':<10} {'PAGE':<5} {'PRIORITY':<8} {'URGENCY':<7} {'BLAST':<7} {'ROUTE':<20} CLASSIFICATION")
    for i, d in enumerate(batch_decisions, start=1):
        if not verbose:
            print_decision(batch_reqs[i - 1].incident_id, d)
            continue
        routing = d.routing_target
        which_target = routing.field if routing else "none"
        target_val = routing.value if routing else ""
        priority_name = d.priority.name
        log(
            f"  - Item [{i}]: RoutingTarget={which_target}:{target_val}, "
            f"Paging={d.requires_immediate_paging}, Priority={priority_name}, "
            f"Urgency={d.urgency_rating}, BlastRadius={d.blast_radius_percentage}%"
        )

    log("\n✔ Python End-to-End Test PASSED successfully!")


if __name__ == "__main__":
    main()
