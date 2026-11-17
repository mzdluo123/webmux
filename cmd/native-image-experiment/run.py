"""Opt-in paid CLI probe. Inputs must come from this command's synthetic generator."""
import argparse
import json
import os
from pathlib import Path
import signal
import subprocess
import tempfile
import time


def private_json(path, value):
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n")
    path.chmod(0o600)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("directory", type=Path)
    parser.add_argument("--backend", choices=["codex", "omp"], required=True)
    parser.add_argument("--model", help="Explicit CLI model; omit to use CLI default")
    parser.add_argument("--modes", nargs="+", choices=["raw", "projected", "image", "hybrid"], default=["raw", "projected", "image", "hybrid"])
    parser.add_argument("--timeout", type=int, default=120)
    args = parser.parse_args()
    directory = args.directory.resolve()
    manifest = json.loads((directory / "manifest.json").read_text())
    task = (directory / "task.txt").read_text()
    for mode in args.modes:
        prompt = task
        images = []
        if mode in ("raw", "projected", "hybrid"):
            prompt += "\n\nHISTORY:\n" + (directory / (mode + ".txt")).read_text()
        if mode in ("image", "hybrid"):
            images = [str(directory / page["file"]) for page in manifest[mode]["pages"]]
        with tempfile.TemporaryDirectory(prefix="webmux-image-cli-") as cwd:
            if args.backend == "codex":
                command = ["codex", "exec", "--ignore-user-config", "--ignore-rules", "--ephemeral", "--skip-git-repo-check", "--sandbox", "read-only", "--json", "-C", cwd]
                for image in images:
                    command += ["--image", image]
                command += ["-"]
            else:
                command = ["omp", "--cwd", cwd, "--print", "--mode", "json", "--no-session", "--no-title", "--no-tools", "--no-extensions", "--no-skills", "--no-rules", "--no-context-files", "--no-lsp", "--offline", "--no-judge", "--no-spotlight", "--thinking", "off", "--max-cost", "1", "--max-time", str(args.timeout)]
                command += ["@" + image for image in images]
            if args.model:
                command += ["--model", args.model]
            # stdin carries only synthetic text. No shell, no config/auth file reads.
            started = time.monotonic()
            process = subprocess.Popen(command, cwd=cwd, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, start_new_session=True)
            timed_out = False
            try:
                stdout, stderr = process.communicate(prompt, timeout=args.timeout + 10)
            except subprocess.TimeoutExpired:
                timed_out = True
                os.killpg(process.pid, signal.SIGTERM)
                try:
                    stdout, stderr = process.communicate(timeout=5)
                except subprocess.TimeoutExpired:
                    os.killpg(process.pid, signal.SIGKILL)
                    stdout, stderr = process.communicate()
            elapsed = round(time.monotonic() - started, 3)
        stem = args.backend + "-" + mode
        # Raw CLI events remain outside the repository; inspect before sharing.
        for suffix, content in (("stdout.jsonl", stdout), ("stderr.txt", stderr)):
            path = directory / (stem + "-" + suffix)
            path.write_text(content)
            path.chmod(0o600)
        events = []
        for line in stdout.splitlines():
            try:
                events.append(json.loads(line))
            except json.JSONDecodeError:
                pass
        text = []
        observations = []
        tool_calls = []

        def visit(value):
            if isinstance(value, dict):
                for key, item in value.items():
                    if key in ("usage", "cost", "model", "modelId", "provider", "total_cost_usd"):
                        observations.append({key: item})
                    if key == "type" and item in ("toolCall", "tool_use", "command_execution", "mcp_tool_call"):
                        tool_calls.append(value)
                    if key == "text" and isinstance(item, str):
                        text.append(item)
                    else:
                        visit(item)
            elif isinstance(value, list):
                for item in value:
                    visit(item)

        for event in events:
            # Avoid counting streamed deltas and final messages twice.
            if event.get("type") not in ("message_update", "agent_message_delta"):
                visit(event)
        answer = None
        for candidate in reversed(text):
            start, end = candidate.find("{"), candidate.rfind("}")
            if start >= 0 and end > start:
                try:
                    parsed = json.loads(candidate[start:end + 1])
                    if isinstance(parsed, dict) and "port" in parsed:
                        answer = parsed
                        break
                except json.JSONDecodeError:
                    pass
        expected = dict(manifest["expected"])
        if mode == "raw":
            expected["middle_marker"] = "JADE-120"
        report = {"backend": args.backend, "mode": mode, "requested_model": args.model, "exit_code": process.returncode, "timeout": timed_out, "wall_seconds": elapsed, "prompt_bytes": len(prompt.encode()), "images": len(images), "answer": answer, "correct_fields": {key: answer is not None and answer.get(key) == value for key, value in expected.items()}, "observed_usage_model_cost": observations, "tool_calls": len(tool_calls), "supplementary_requests": 0, "cost_note": "Only CLI-reported usage/cost; absent is unknown, not zero. NEED_SOURCE is not a completed lookup. No WebMux server or native injection exercised."}
        private_json(directory / (stem + "-report.json"), report)
        print(json.dumps(report, ensure_ascii=False), flush=True)
        if process.returncode != 0 or timed_out:
            break


if __name__ == "__main__":
    main()
