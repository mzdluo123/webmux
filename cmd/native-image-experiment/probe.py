"""Isolated CLI execution and authoritative final-event decoding."""
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


def parse_events(stdout, backend):
    events = []
    for line in stdout.splitlines():
        try:
            value = json.loads(line)
            if isinstance(value, dict):
                events.append(value)
        except json.JSONDecodeError:
            pass
    # agent_end repeats message_end; prefer its final snapshot, never add both.
    messages = []
    source = None
    if backend == "omp":
        for event in events:
            if event.get("type") == "agent_end":
                candidates = [m for m in event.get("messages", []) if m.get("role") == "assistant"]
                if candidates:
                    messages, source = candidates, "agent_end"
        if not messages:
            messages = [e["message"] for e in events if e.get("type") == "message_end" and e.get("message", {}).get("role") == "assistant"]
            source = "message_end" if messages else None
        message = messages[-1] if messages else {}
        text = "\n".join(c.get("text", "") for c in message.get("content", []) if c.get("type") == "text")
        usage = message.get("usage")
        snapshot = {"source": source, "usage": usage, "model": message.get("model"), "provider": message.get("provider"), "cost": usage.get("cost") if isinstance(usage, dict) else None}
        calls = {c.get("id", json.dumps(c, sort_keys=True)) for m in messages for c in m.get("content", []) if c.get("type") in ("toolCall", "tool_use")}
        # Execution events also detect calls when the final message omits them.
        calls.update(e.get("toolCallId", json.dumps(e, sort_keys=True)) for e in events if e.get("type") == "tool_execution_start")
    else:
        items = [e.get("item", {}) for e in events if e.get("type") == "item.completed"]
        text = "\n".join(i.get("text", "") for i in items if i.get("type") == "agent_message")
        finals = [e for e in events if e.get("type") == "turn.completed"]
        final = finals[-1] if finals else {}
        snapshot = {"source": "turn.completed" if finals else None, "usage": final.get("usage"), "model": final.get("model"), "provider": final.get("provider"), "cost": final.get("total_cost_usd")}
        calls = {i.get("id", json.dumps(i, sort_keys=True)) for i in items if i.get("type") in ("command_execution", "mcp_tool_call", "web_search", "file_change")}
    answer = None
    start, end = text.find("{"), text.rfind("}")
    if start >= 0 and end > start:
        try:
            value = json.loads(text[start:end + 1])
            if isinstance(value, dict):
                answer = value
        except json.JSONDecodeError:
            pass
    return {"answer": answer, "response_text": text, "final_usage_snapshot": snapshot, "tool_calls": len(calls)}


def usage_metrics(snapshot, backend):
    usage = snapshot.get("usage")
    if not isinstance(usage, dict):
        return {"input_tokens": None, "cache_read_tokens": None, "cache_write_tokens": None, "output_tokens": None, "total_cost": None}
    if backend == "omp":
        parts = [usage.get(key) for key in ("input", "cacheRead", "cacheWrite")]
        total_input = sum(parts) if all(isinstance(p, (int, float)) for p in parts) else None
        cost = usage.get("cost")
        return {"input_tokens": total_input, "uncached_input_tokens": usage.get("input"), "cache_read_tokens": usage.get("cacheRead"), "cache_write_tokens": usage.get("cacheWrite"), "output_tokens": usage.get("output"), "total_cost": cost.get("total") if isinstance(cost, dict) else None}
    return {"input_tokens": usage.get("input_tokens"), "cache_read_tokens": usage.get("cached_input_tokens"), "cache_write_tokens": None, "output_tokens": usage.get("output_tokens"), "total_cost": snapshot.get("cost")}


def invoke(args, prompt, images, stem):
    with tempfile.TemporaryDirectory(prefix="webmux-image-cli-") as cwd:
        if args.backend == "codex":
            command = ["codex", "exec", "--ignore-user-config", "--ignore-rules", "--ephemeral", "--skip-git-repo-check", "--sandbox", "read-only", "--json", "-C", cwd, "--model", args.model]
            for image in images:
                command += ["--image", str(image)]
            command += ["-"]
        else:
            command = ["omp", "--cwd", cwd, "--print", "--mode", "json", "--no-session", "--no-title", "--no-tools", "--no-extensions", "--no-skills", "--no-rules", "--no-context-files", "--no-lsp", "--offline", "--no-judge", "--no-spotlight", "--thinking", "off", "--max-cost", str(args.max_cost), "--max-time", str(args.timeout), "--model", args.model]
            command += ["@" + str(image) for image in images]
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
    for suffix, content in (("stdout.jsonl", stdout), ("stderr.txt", stderr)):
        path = args.directory / (stem + "-" + suffix)
        path.write_text(content)
        path.chmod(0o600)
    result = parse_events(stdout, args.backend)
    result.update({"exit_code": process.returncode, "timeout": timed_out, "wall_seconds": elapsed, "prompt_bytes": len(prompt.encode()), "images": len(images)})
    result["metrics"] = usage_metrics(result["final_usage_snapshot"], args.backend)
    return result
