#!/usr/bin/env python3
"""Local Clef test server.

Exposes a fast, zero-dependency HTTP server implementing the Jev/Clef
``POST /v1/systemone`` wire protocol (and Cloudflare Workers AI routes).
Used for testing protoc-gen-jev client SDKs against Clef decision models.
"""

from __future__ import annotations

import argparse
import json
import logging
import re
import sys
from http.server import BaseHTTPRequestHandler, HTTPServer
from typing import Any

logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(message)s")
logger = logging.getLogger("clef_serve")

DEFAULT_PORT = 8787
DEFAULT_HOST = "127.0.0.1"
DEFAULT_MODEL = "clef"


def evaluate_state_text(state: Any) -> str:
    """Normalize input state into text for evaluation."""
    if isinstance(state, str):
        return state
    try:
        return json.dumps(state, ensure_ascii=False)
    except Exception:
        return str(state)


def _detect_signals(lower: str) -> tuple[bool, bool]:
    # Strip benign negations like "0 errors", "no error", "latency within slo"
    cleaned = re.sub(r"\b(0|no|zero)\s+errors?\b", "", lower)
    cleaned = re.sub(r"\blatency\s+within\s+slo\b", "", cleaned)
    cleaned = re.sub(r"\bno\s+customer\s+impact\b", "", cleaned)

    urgent_signals = [
        "exhausted",
        "500",
        "502",
        "spike",
        "outage",
        "error",
        "failing",
        "refused",
        "connection reset",
        "bad gateway",
        "immediate",
        "urgent",
        "critical",
    ]
    benign_signals = [
        "routine",
        "compaction",
        "scheduled",
        "within slo",
        "reclaimed",
        "deprecation",
        "deprecated",
        "low-priority",
        "finished successfully",
        "no customer impact",
    ]
    has_urgent = any(w in cleaned for w in urgent_signals)
    has_benign = any(w in lower for w in benign_signals)
    return has_urgent, has_benign


def evaluate_noul(text: str, question: dict[str, Any]) -> dict[str, Any]:
    """Evaluate a noul (boolean probability) question."""
    lower = text.lower()
    instructions = (question.get("instructions") or "").lower()
    has_urgent, has_benign = _detect_signals(lower)

    if "page" in instructions or "outage" in instructions or "urgent" in instructions:
        if has_urgent and not has_benign:
            prob = 0.94
        elif has_benign:
            prob = 0.03
        else:
            prob = 0.88 if has_urgent else 0.15
    else:
        # General question heuristic
        if has_urgent:
            prob = 0.92
        elif has_benign:
            prob = 0.08
        else:
            prob = 0.50

    return {
        "type": "noul",
        "noul": prob,
        "confidence": round(abs(prob - 0.5) * 2, 4),
    }


def evaluate_choice(text: str, question: dict[str, Any]) -> dict[str, Any]:
    """Evaluate a choice (categorical selection) question."""
    lower = text.lower()
    has_urgent, has_benign = _detect_signals(lower)
    criteria = question.get("criteria")
    options: list[str] = []
    if isinstance(criteria, dict):
        options = list(criteria.keys())
    elif isinstance(criteria, list):
        options = [str(x) for x in criteria]

    if not options:
        options = ["default"]

    selected = options[0]

    # Priority level heuristic
    if any("PRIORITY_LEVEL" in opt for opt in options):
        if has_benign and not has_urgent:
            selected = "PRIORITY_LEVEL_LOW" if "PRIORITY_LEVEL_LOW" in options else options[0]
        elif "exhausted" in lower or "total failure" in lower or "complete service outage" in lower:
            selected = "PRIORITY_LEVEL_CRITICAL" if "PRIORITY_LEVEL_CRITICAL" in options else options[-1]
        elif "500" in lower or "latency spike" in lower or "502" in lower or "spikes" in lower or "refused" in lower:
            selected = "PRIORITY_LEVEL_HIGH" if "PRIORITY_LEVEL_HIGH" in options else options[-2]
        elif "PRIORITY_LEVEL_MEDIUM" in options:
            selected = "PRIORITY_LEVEL_MEDIUM"
        else:
            selected = options[0]

    # Routing target heuristic (oneof fields)
    elif "automated_runbook" in options or "oncall_engineer" in options:
        if "deprecation" in lower or "routine" in lower or "compactor" in lower:
            selected = "automated_runbook" if "automated_runbook" in options else options[0]
        elif "exhausted" in lower or "pool" in lower or "commander" in lower:
            selected = "incident_commander" if "incident_commander" in options else ("oncall_engineer" if "oncall_engineer" in options else options[0])
        else:
            selected = "oncall_engineer" if "oncall_engineer" in options else options[0]

    # Compliance classification heuristic
    elif "PUBLIC" in options:
        if "pii" in lower or "ssn" in lower:
            selected = "RESTRICTED_PII" if "RESTRICTED_PII" in options else "PUBLIC"
        elif "pci" in lower or "card" in lower:
            selected = "PCI_DSS" if "PCI_DSS" in options else "PUBLIC"
        else:
            selected = "PUBLIC"
    else:
        # Match options directly against text words
        for opt in options:
            if re.search(r"\b" + re.escape(opt.lower()) + r"\b", lower):
                selected = opt
                break

    # Build calibrated probabilities
    probs: dict[str, float] = {}
    rem = round(0.15 / max(1, len(options) - 1), 4) if len(options) > 1 else 0.0
    for opt in options:
        probs[opt] = 0.85 if opt == selected else rem

    return {
        "type": "choice",
        "choice": selected,
        "confidence": 0.85,
        "probabilities": probs,
    }


def evaluate_score(text: str, question: dict[str, Any]) -> dict[str, Any]:
    """Evaluate a score (rubric scale) question."""
    lower = text.lower()
    instructions = (question.get("instructions") or "").lower()
    criteria = question.get("criteria")
    levels: list[str] = []
    if isinstance(criteria, list):
        levels = [str(x) for x in criteria]
    num_levels = max(2, len(levels))
    max_idx = num_levels - 1

    # Urgency rating heuristic (typically 1 to 5)
    if "urgency" in instructions or "urgently" in instructions:
        has_urgent, has_benign = _detect_signals(lower)
        if has_benign and not has_urgent:
            pos = 0.0
        elif "exhausted" in lower or "complete" in lower or "critical" in lower:
            pos = float(max_idx)
        elif "502" in lower or "spikes" in lower or "500" in lower or "refused" in lower:
            pos = float(max_idx - 1)
        elif "deprecation" in lower:
            pos = 1.0 if max_idx >= 4 else 0.5
        elif "routine" in lower or "compaction" in lower:
            pos = 0.0
        else:
            pos = float(max_idx) / 2.0

    # Blast radius heuristic (percentage 0 to 100)
    elif "blast" in instructions or "impact" in instructions:
        if "exhausted" in lower:
            # 12% errors -> ~50% blast radius in the rubric tiers [0, 25, 50, 75, 100]
            pos = 2.0
        elif "502" in lower:
            pos = 1.4  # ~35%
        elif "routine" in lower or "deprecation" in lower:
            pos = 0.0  # 0%
        else:
            pos = 1.0
    else:
        pos = float(max_idx) / 2.0

    pos = max(0.0, min(float(max_idx), pos))
    legend = {str(i): levels[i] if i < len(levels) else f"Level {i}" for i in range(num_levels)}

    probs: dict[str, float] = {}
    for i in range(num_levels):
        dist = abs(float(i) - pos)
        probs[str(i)] = round(max(0.05, 1.0 - (dist / max(1, max_idx))), 4)
    total = sum(probs.values())
    probs = {k: round(v / total, 4) for k, v in probs.items()}

    return {
        "type": "score",
        "score": round(pos, 4),
        "confidence": 0.90,
        "legend": legend,
        "probabilities": probs,
    }


def evaluate_questions(model_name: str, state: Any, questions: dict[str, Any]) -> dict[str, Any]:
    """Execute evaluation for all questions in the request."""
    text = evaluate_state_text(state)
    answers: dict[str, Any] = {}

    for name, q in questions.items():
        q_type = q.get("type", "noul")
        if q_type == "noul":
            answers[name] = evaluate_noul(text, q)
        elif q_type == "choice":
            answers[name] = evaluate_choice(text, q)
        elif q_type == "score":
            answers[name] = evaluate_score(text, q)
        else:
            answers[name] = {"type": q_type, "error": f"unsupported type {q_type}"}

    return {
        "model": model_name,
        "answers": answers,
        "usage": {
            "input_tokens": max(1, len(text.split())),
            "output_tokens": 0,
        },
    }


class ClefRequestHandler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"
    model_name: str = DEFAULT_MODEL

    def do_GET(self) -> None:
        if self.path in ("/health", "/healthz", "/"):
            resp_bytes = json.dumps({
                "status": "healthy",
                "service": "clef-test-server",
                "model": self.model_name,
            }).encode("utf-8")
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(resp_bytes)))
            self.end_headers()
            self.wfile.write(resp_bytes)
            return

        self.send_error(404, "Endpoint not found")

    def do_POST(self) -> None:
        path = self.path.split("?")[0]
        is_cf_route = "/ai/run/" in path or path.startswith("/client/v4/")
        is_systemone_route = path.endswith("/v1/systemone")

        if not (is_systemone_route or is_cf_route):
            self.send_error(404, f"Unknown endpoint: {path}")
            return

        content_length = int(self.headers.get("Content-Length", 0))
        if content_length <= 0:
            self.send_error(400, "Empty request body")
            return

        body_bytes = self.rfile.read(content_length)
        try:
            req = json.loads(body_bytes.decode("utf-8"))
        except Exception as e:
            self.send_error(400, f"Malformed JSON: {e}")
            return

        model = req.get("model") or self.model_name
        state = req.get("state")
        questions = req.get("questions")

        if not isinstance(questions, dict):
            self.send_error(400, "Missing or invalid 'questions' object in payload")
            return

        result = evaluate_questions(model, state, questions)

        if is_cf_route:
            payload = {
                "result": result,
                "success": True,
                "errors": [],
                "messages": [],
            }
        else:
            payload = result

        resp_bytes = json.dumps(payload).encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(resp_bytes)))
        self.end_headers()
        self.wfile.write(resp_bytes)

    def log_message(self, format: str, *args: Any) -> None:
        pass


def run_server(host: str, port: int, model: str) -> None:
    ClefRequestHandler.model_name = model
    from http.server import ThreadingHTTPServer
    server = ThreadingHTTPServer((host, port), ClefRequestHandler)
    server.daemon_threads = True
    logger.info(f"Clef test server running at http://{host}:{port} (model: {model})")
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        logger.info("Clef test server stopped.")
    finally:
        server.server_close()


def main() -> None:
    parser = argparse.ArgumentParser(description="Run local Clef test server.")
    parser.add_argument("--host", default=DEFAULT_HOST, help=f"Host to bind (default: {DEFAULT_HOST})")
    parser.add_argument("--port", type=int, default=DEFAULT_PORT, help=f"Port to bind (default: {DEFAULT_PORT})")
    parser.add_argument("--model", default=DEFAULT_MODEL, help=f"Model name (default: {DEFAULT_MODEL})")
    args = parser.parse_args()

    run_server(args.host, args.port, args.model)


if __name__ == "__main__":
    main()
