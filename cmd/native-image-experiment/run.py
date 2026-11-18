"""Opt-in paid synthetic benchmark: raw vs identical projected/archive coverage."""
import argparse
import hashlib
import json
from pathlib import Path

from probe import invoke, private_json


def score(answer, expected):
    return {key: isinstance(answer, dict) and type(answer.get(key)) is type(value) and answer.get(key) == value for key, value in expected.items()}


def checked_file(directory, name, expected_hash):
    path = (directory / name).resolve()
    if not path.is_relative_to(directory):
        raise ValueError("representation must be inside the synthetic directory")
    if hashlib.sha256(path.read_bytes()).hexdigest() != expected_hash:
        raise ValueError(f"representation hash mismatch: {name}")
    return path


def readback(manifest, answer):
    # One bounded recovery round, selected only by explicit NEED_SOURCE fields.
    # The expected answers never enter any prompt.
    fields = [key for key in manifest["expected"] if isinstance(answer, dict) and answer.get(key) == "NEED_SOURCE"]
    ids = list(dict.fromkeys(manifest["field_sources"][key] for key in fields))[:3]
    records = {record["ID"]: record for record in manifest["input"]["History"]}
    chunks, supplied = [], []
    used = 0
    for source_id in ids:
        text = records[source_id]["Text"]
        if used + len(text) > 6000:
            break
        chunks.append(f"[source={source_id}]\n{text}")
        supplied.append(source_id)
        used += len(text)
    return "\n\n".join(chunks), supplied, used


def comparison(reports):
    by_mode = {report["mode"]: report for report in reports}
    baseline = by_mode.get("projected")
    candidate = by_mode.get("archive")
    if not baseline or not candidate:
        return None
    base_tokens = baseline["rounds"][0]["metrics"]["input_tokens"]
    candidate_tokens = candidate["rounds"][0]["metrics"]["input_tokens"]
    valid = all(r["initial_valid"] for r in (baseline, candidate))
    initial_correct = all(all(r["initial_correct_fields"].values()) for r in (baseline, candidate))
    return {
        "baseline": "projected", "candidate": "archive",
        "same_projected_coverage": True, "initial_runs_valid": valid,
        "all_initial_fields_correct": initial_correct,
        "baseline_input_tokens": base_tokens, "candidate_input_tokens": candidate_tokens,
        "input_token_savings_fraction": 1 - candidate_tokens / base_tokens if valid and base_tokens and candidate_tokens is not None else None,
        "eligible_accuracy_preserving_savings": valid and initial_correct,
        "baseline_total_cost": baseline["total_measured_cost"],
        "candidate_total_cost_including_readback": candidate["total_measured_cost"],
        "note": "Input includes cached tokens and CLI/system overhead. Token savings use initial calls only; readback costs and final accuracy are separate. Missing usage/cost is unknown.",
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("directory", type=Path)
    parser.add_argument("--backend", choices=["codex", "omp"], required=True)
    parser.add_argument("--model", required=True, help="Explicit model ID; must match generator geometry")
    parser.add_argument("--modes", nargs="+", choices=["raw", "projected", "archive"], default=["raw", "projected", "archive"])
    parser.add_argument("--timeout", type=int, default=240)
    parser.add_argument("--max-cost", type=float, default=2, help="OMP per-call cost ceiling")
    parser.add_argument("--readback", action="store_true", help="Allow one supplemental source round for up to three NEED_SOURCE records (6000 runes)")
    args = parser.parse_args()
    if args.timeout <= 0 or args.max_cost <= 0 or len(set(args.modes)) != len(args.modes):
        parser.error("timeout/cost must be positive and modes unique")
    args.directory = args.directory.resolve()
    manifest = json.loads((args.directory / "manifest.json").read_text())
    if manifest.get("schema") != 2 or not manifest.get("synthetic_only"):
        parser.error("regenerate synthetic inputs with the current generator")
    if manifest["model"] != args.model:
        parser.error("--model must equal manifest.model; regenerate for different model geometry")
    coverage = manifest["coverage"]
    if not coverage["same_projected_coverage"] or coverage["projection_omissions"] or coverage["archive_omissions"] or coverage["archive_source_runes"] != coverage["archive_retained_runes"]:
        parser.error("this benchmark requires equal complete projected coverage")
    files = {name: checked_file(args.directory, name, value["sha256"]) for name, value in manifest["representations"].items()}
    pages = [checked_file(args.directory, page["file"], page["sha256"]) for page in manifest["archive"]["pages"]]
    if files["projected.txt"].read_bytes() != files["archive-retained.txt"].read_bytes():
        parser.error("projected baseline differs from archive retained history")
    task = files["task.txt"].read_text()
    reports = []
    for mode in args.modes:
        prompt = task + "\n\nHISTORY:\n" + files[mode + ".txt"].read_text()
        images = pages if mode == "archive" else []
        stem = args.backend + "-" + mode
        first = invoke(args, prompt, images, stem)
        rounds = [first]
        initial_valid = first["exit_code"] == 0 and not first["timeout"] and first["tool_calls"] == 0 and first["final_usage_snapshot"]["source"] is not None
        supplied, readback_runes = [], 0
        if args.readback and initial_valid:
            extra, supplied, readback_runes = readback(manifest, first["answer"])
            if supplied:
                recovery_prompt = prompt + "\n\nPREVIOUS ANSWER:\n" + json.dumps(first["answer"], ensure_ascii=False) + "\n\nREQUESTED SOURCE READBACK:\n" + extra + "\n\nReturn the complete corrected JSON object; use no tools."
                rounds.append(invoke(args, recovery_prompt, images, stem + "-readback"))
        final = rounds[-1]
        costs = [round_["metrics"]["total_cost"] for round_ in rounds]
        tokens = [round_["metrics"]["input_tokens"] for round_ in rounds]
        report = {
            "backend": args.backend, "mode": mode, "requested_model": args.model,
            "scenario": manifest["scenario"], "seed": manifest["seed"], "coverage": coverage,
            "expected": manifest["expected"], "answer": final["answer"],
            "initial_correct_fields": score(first["answer"], manifest["expected"]),
            "correct_fields": score(final["answer"], manifest["expected"]),
            "initial_valid": initial_valid,
            "valid": all(r["exit_code"] == 0 and not r["timeout"] and r["tool_calls"] == 0 and r["final_usage_snapshot"]["source"] is not None for r in rounds),
            "rounds": rounds, "supplementary_requests": len(rounds) - 1,
            "readback_source_ids": supplied, "readback_runes": readback_runes,
            "total_measured_cost": sum(costs) if all(isinstance(c, (int, float)) for c in costs) else None,
            "total_input_tokens_including_readback": sum(tokens) if all(isinstance(t, (int, float)) for t in tokens) else None,
            "archive_cache_key": manifest["archive"]["cache_key"] if mode == "archive" else None,
            "cost_note": "One authoritative final usage snapshot per CLI invocation; repeated delta/message events are excluded. CLI-reported billing only; absent means unknown. Readback is a fresh bounded call with repeated prompt, not free recovery. Codex sandbox does not disable tools; any observed call invalidates that run. No WebMux/native transport is exercised.",
        }
        private_json(args.directory / (stem + "-report.json"), report)
        print(json.dumps(report, ensure_ascii=False), flush=True)
        reports.append(report)
        if not report["valid"]:
            break
    summary = {"backend": args.backend, "requested_model": args.model, "modes": [r["mode"] for r in reports], "comparison": comparison(reports)}
    private_json(args.directory / (args.backend + "-comparison.json"), summary)
    print(json.dumps(summary, ensure_ascii=False), flush=True)
    if any(not r["valid"] for r in reports):
        raise SystemExit(1)


if __name__ == "__main__":
    main()
