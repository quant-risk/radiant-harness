#!/usr/bin/env python3
"""
Agentic LLM simulator for radiant-harness MCP sampling.

The harness's v3.7.0+ agentic driver expects a real LLM that:
  1. Receives a `sampling/createMessage` request (system + task).
  2. Responds with content = text + tool_use blocks.
  3. Lets the harness execute the tool_use blocks (real Read/Write/Bash).
  4. Receives a follow-up sampling/createMessage with tool_results
     appended as a "TOOL RESULTS: {...}" user message.
  5. Loops until it emits a text-only response containing
     "VERDICT: APPROVED" on its own line.

This harness **simulates** a real Claude-class agent: it knows what to do
in each round (inspect → write spec → write tasks → write code → run
pytest → verdict) and emits appropriate tool_use blocks.

Tool calls implemented in the harness's registry:
  - read_file, search_code     (input_schema presents relative path)
  - write_file                  (input_schema presents path + content)
  - run_gate                    (input_schema presents command + cwd)

We do NOT execute these here — the harness does that itself and feeds
back TOOL RESULTS in the next sampling round.
"""

import json
import os
import subprocess
import sys
import threading
import time

WORKDIR = sys.argv[2]
BINARY = sys.argv[1]
TASK = sys.argv[3]


def send(req):
    body = (json.dumps(req) + "\n").encode()
    proc.stdin.write(body)
    proc.stdin.flush()


def respond_sampling(req_id, content):
    """Send a sampling/createMessage result back to the harness."""
    send({"jsonrpc": "2.0", "id": req_id, "result": {"role": "assistant",
                                                       "content": content}})


def text(s):
    return {"type": "text", "text": s}


def tool_use(call_id, name, inputs):
    return {"type": "tool_use", "id": call_id, "name": name, "input": inputs}


SPEC_MD = """# spec.md — 0001-rollingavg

## Goal

A Python CLI tool that reads a CSV of (timestamp, value) pairs and emits a
rolling mean per row using a configurable window. Stdlib-only (no
pandas/numpy).

## Acceptance criteria

- AC1: `rollingavg --input data.csv --window 5 --output out.csv` produces
  a CSV where each row's value column is the arithmetic mean of the
  trailing N=window rows inclusive of itself.
- AC2: Empty input → empty output. Single-row input → single-row output
  with the same value.
- AC3: Window >= 2; window < 1 raises ValueError("window must be >= 1").
- AC4: Header row preserved on output if present.
- AC5: `--window 0` is rejected; `--window 1` is a no-op (rolling mean of
  1 element == the element itself).

## Non-goals

- Weighted moving averages, streaming input.

## Profile: standard
"""

TASKS_MD = """# tasks.md — 0001-rollingavg

1. **T1** — write `src/rollingavg/__init__.py` and
   `src/rollingavg/core.py` with `def rolling_mean(values, window)`.
2. **T2** — write `src/rollingavg/cli.py` with argparse plumbing.
3. **T3** — write `tests/test_core.py` (5 pytest tests).
4. **T4** — write `pyproject.toml`.
5. **T5** — write `README.md`.
6. **T6** — write `.github/workflows/test.yml`.
7. **T7** — write `AGENTS.md`.
8. **T8** — run `pytest tests/test_core.py -v` and capture exit code.

## Gates

- `python -m pytest tests/test_core.py -v` → 5 passed.
"""

INIT_PY = '''"""rollingavg — CLI tool for rolling arithmetic mean over CSV data."""

__version__ = "0.1.0"
'''

CORE_PY = '''"""Rolling arithmetic mean over a window of values."""
from typing import List


def rolling_mean(values: List[float], window: int) -> List[float]:
    if window < 1:
        raise ValueError(f"window must be >= 1, got {window}")
    out: List[float] = []
    running_sum = 0.0
    for i, v in enumerate(values):
        running_sum += v
        if i >= window:
            running_sum -= values[i - window]
        size = min(i + 1, window)
        out.append(running_sum / size)
    return out


if __name__ == "__main__":
    print("rollingavg.core — internal helper, see cli.py for the CLI.")
'''

CLI_PY = '''"""Command-line interface for rollingavg."""
from __future__ import annotations

import argparse
import csv
import sys
from pathlib import Path

from .core import rolling_mean


def _build_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(
        prog="rollingavg",
        description="Rolling arithmetic mean over a (timestamp, value) CSV.",
    )
    p.add_argument("--input", required=True, type=Path, help="Input CSV path.")
    p.add_argument("--window", required=True, type=int, help="Window size (>= 1).")
    p.add_argument("--output", required=True, type=Path, help="Output CSV path.")
    p.add_argument("--value-column", default="value", help="Column to smooth.")
    p.add_argument("--has-header", action="store_true", help="CSV has header.")
    return p


def main(argv=None) -> int:
    parser = _build_parser()
    args = parser.parse_args(argv)

    if args.window < 1:
        print(f"error: --window must be >= 1, got {args.window}", file=sys.stderr)
        return 2

    with args.input.open(newline="") as f:
        if args.has_header:
            reader = csv.DictReader(f)
            rows = list(reader)
            fieldnames = reader.fieldnames or []
            values = [float(r[args.value_column]) for r in rows]
        else:
            reader = csv.reader(f)
            rows = [row for row in reader]
            fieldnames = []
            values = [float(row[1]) for row in rows]

    try:
        smoothed = rolling_mean(values, args.window)
    except ValueError as e:
        print(f"error: {e}", file=sys.stderr)
        return 2

    with args.output.open("w", newline="") as f:
        if args.has_header:
            writer = csv.DictWriter(f, fieldnames=[*fieldnames, "rolling_mean"])
            writer.writeheader()
            for r, m in zip(rows, smoothed):
                row = dict(r)
                row["rolling_mean"] = f"{m:.6f}"
                writer.writerow(row)
        else:
            writer = csv.writer(f)
            for row, m in zip(rows, smoothed):
                writer.writerow([row[0], f"{m:.6f}"])

    return 0


if __name__ == "__main__":
    sys.exit(main())
'''

TESTS_PY = '''"""Unit tests for rollingavg.core.rolling_mean."""
import pytest

from rollingavg.core import rolling_mean


def test_empty_input_returns_empty_output():
    assert rolling_mean([], 3) == []


def test_single_element_window_one_is_passthrough():
    assert rolling_mean([42.0], 1) == [42.0]


def test_window_two_computes_pairwise_mean():
    result = rolling_mean([1.0, 3.0, 5.0, 7.0], 2)
    assert result == [1.0, 2.0, 4.0, 6.0]


def test_window_larger_than_series_partial_at_start():
    result = rolling_mean([2.0, 4.0, 6.0], 5)
    assert result == [2.0, 3.0, 4.0]


def test_invalid_window_raises_value_error():
    with pytest.raises(ValueError):
        rolling_mean([1.0, 2.0], 0)
'''

PYPROJECT_TOML = '''[build-system]
requires = ["setuptools>=68", "wheel"]
build-backend = "setuptools.build_meta"

[project]
name = "rollingavg"
version = "0.1.0"
description = "CLI tool for rolling arithmetic mean over (timestamp, value) CSV."
readme = "README.md"
requires-python = ">=3.10"
license = { text = "MIT" }

[project.scripts]
rollingavg = "rollingavg.cli:main"

[tool.setuptools.packages.find]
where = ["src"]

[tool.pytest.ini_options]
testpaths = ["tests"]
'''

README_MD = '''# rollingavg

A small Python CLI that reads a CSV of `(timestamp, value)` pairs and
emits a rolling arithmetic mean over a configurable window. Python
stdlib only — no pandas, no numpy.

## Install

```bash
pip install -e .
```

## Usage

```bash
rollingavg --input data.csv --window 5 --output out.csv --has-header
```

## Run tests

```bash
pip install pytest
pytest tests/test_core.py -v
```

MIT.
'''

CI_YML = '''name: tests
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        python-version: ["3.10", "3.11", "3.12"]
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-python@v5
        with:
          python-version: ${{ matrix.python-version }}
      - run: |
          python -m pip install --upgrade pip
          pip install -e .
          pip install pytest
      - run: pytest tests/ -v
'''

# Tracks which step of the script we're at; advanced by tool results.
step = 0


def main():
    global proc, inbox, step
    proc = subprocess.Popen(
        [BINARY, "mcp", "serve", "--cwd", WORKDIR],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        bufsize=0,
    )
    inbox = []

    def reader():
        buf = b""
        while True:
            c = proc.stdout.read(1)
            if not c:
                return
            if c == b"\n":
                line = buf.strip()
                if line:
                    try:
                        inbox.append(json.loads(line.decode()))
                    except Exception:
                        pass
                buf = b""
            else:
                buf += c

    threading.Thread(target=reader, daemon=True).start()

    def wait_for(target_id=None, target_method=None, timeout=20):
        deadline = time.time() + timeout
        while time.time() < deadline:
            for r in list(inbox):
                inbox.remove(r)
                if target_id is not None and r.get("id") == target_id:
                    return r
                if target_method is not None and r.get("method") == target_method:
                    return r
            time.sleep(0.2)
        return None

    # 1. initialize
    send({"jsonrpc": "2.0", "id": 1, "method": "initialize",
          "params": {"protocolVersion": "2024-11-05", "capabilities": {},
                     "clientInfo": {"name": "agentic-llm-host", "version": "0.1.0"}}})
    init = wait_for(target_id=1, timeout=10)
    assert init, "no initialize"
    si = init["result"]["serverInfo"]
    print(f"[+] Server: {si['name']} v{si['version']}")

    # 2. notifications/initialized
    send({"jsonrpc": "2.0", "method": "notifications/initialized"})

    # 3. tools/list
    send({"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": {}})
    tools_resp = wait_for(target_id=2, timeout=10)
    assert tools_resp, "no tools/list"
    tools = [t["name"] for t in tools_resp["result"]["tools"]]
    print(f"[+] Tools: {tools}")

    # 4. tools/call radiant_possess
    send({"jsonrpc": "2.0", "id": 3, "method": "tools/call",
          "params": {"name": "radiant_possess",
                     "arguments": {"task": TASK, "workdir": WORKDIR}}})

    # Drive sampling loops
    sampling_count = 0
    deadline = time.time() + 180  # 3 min, generous
    while time.time() < deadline:
        for r in list(inbox):
            inbox.remove(r)
            # Log everything that arrives
            rid = r.get("id")
            rmethod = r.get("method")
            rresult = r.get("result")
            print(f"\n[inbox] id={rid} method={rmethod} has_result={bool(rresult)}")
            if r.get("error"):
                print(f"  ERROR: {r['error']}")
                deadline = 0
                break
            # JSON-RPC response for tools/call radiant_possess (final answer).
            # Distinguish from sampling requests which have method set.
            if rmethod is None and rresult is not None:
                # Final response
                content = rresult.get("content", [])
                for c in content:
                    if c.get("type") == "text":
                        print(f"\n=== tools/call final ===\n{c['text'][:1500]}\n")
                deadline = 0
                break
            if rmethod == "sampling/createMessage":
                req_id = r["id"]
                # Inspect message list. Wire shape: {role, content: {type, text}}
                msgs = r.get("params", {}).get("messages", [])
                tool_results = sum(
                    1 for m in msgs
                    if isinstance((m.get("content") or {}).get("text", ""), str)
                    and "TOOL RESULTS" in (m.get("content") or {}).get("text", "")
                )
                sampling_count += 1
                print(f"\n>>> Round {sampling_count} (tool_results already in prompt = {tool_results}, total msgs = {len(msgs)})")
                handle_sampling(req_id, tool_results)
        if deadline == 0:
            break
        time.sleep(0.3)

    print(f"\n=== Done. {sampling_count} sampling rounds. ===")
    # Drain harness stderr
    if proc.stderr:
        try:
            _, err = proc.communicate(timeout=3)
            print("=== Harness stderr ===")
            print(err.decode()[-3000:])
        except subprocess.TimeoutExpired:
            proc.kill()
    try:
        proc.terminate()
        proc.wait(timeout=5)
    except subprocess.TimeoutExpired:
        proc.kill()


def handle_sampling(req_id, tool_results_count):
    """
    Decide what the LLM does based on how many tool_result echoes are
    already in the conversation. This is the deterministic LLM:
      round 0: inspection
      round 1: write spec.md
      round 2: write tasks.md
      round 3: write all source files
      round 4: run pytest
      round 5: emit VERDICT
    """
    # Map tool_results_count → round (offset by 1; we count echoes AFTER the first).
    # Round k means: k tool batches already executed. We respond by emitting
    # the next batch.
    r = tool_results_count

    if r == 0:
        # First round: inspect the workdir.
        respond_sampling(req_id, [
            text("Inspecting the existing project structure."),
            tool_use("tu-0", "read_file", {"path": "AGENTS.md"}),
        ])
        return

    if r == 1:
        # First tool ran. Now write the spec.
        respond_sampling(req_id, [
            text("Plan: I will write a 4-AC spec for rollingavg (stdlib only)."),
            tool_use("tu-1", "write_file",
                     {"path": "specs/0001-rollingavg/spec.md", "content": SPEC_MD}),
        ])
        return

    if r == 2:
        respond_sampling(req_id, [
            text("Now writing the tasks list."),
            tool_use("tu-2", "write_file",
                     {"path": "specs/0001-rollingavg/tasks.md", "content": TASKS_MD}),
        ])
        return

    if r == 3:
        # Now write all the implementation files in one batch
        respond_sampling(req_id, [
            text("Implementing per the spec + tasks."),
            tool_use("tu-3a", "write_file",
                     {"path": "src/rollingavg/__init__.py", "content": INIT_PY}),
            tool_use("tu-3b", "write_file",
                     {"path": "src/rollingavg/core.py", "content": CORE_PY}),
            tool_use("tu-3c", "write_file",
                     {"path": "src/rollingavg/cli.py", "content": CLI_PY}),
            tool_use("tu-3d", "write_file",
                     {"path": "tests/test_core.py", "content": TESTS_PY}),
            tool_use("tu-3e", "write_file",
                     {"path": "pyproject.toml", "content": PYPROJECT_TOML}),
            tool_use("tu-3f", "write_file",
                     {"path": "README.md", "content": README_MD}),
            tool_use("tu-3g", "write_file",
                     {"path": ".github/workflows/test.yml", "content": CI_YML}),
        ])
        return

    if r == 4:
        # Last batch executed. Run the gate.
        respond_sampling(req_id, [
            text("Running pytest now."),
            tool_use("tu-4", "run_gate",
                     {"command": "pip install -e . --quiet 2>&1 | tail -5; "
                                  "pip install pytest --quiet 2>&1 | tail -5; "
                                  "pytest tests/test_core.py -v 2>&1",
                      "cwd": WORKDIR}),
        ])
        return

    # r >= 5: respond with the verdict — no tool calls.
    respond_sampling(req_id, [
        text("All five acceptance criteria met. Tests pass. Closing the loop.\n\n"
             "VERDICT: APPROVED\n"
             "SCORE: 0.95\n"
             "EVIDENCE: 5/5 unit tests pass; rolling-mean is O(N) via running-sum; stdlib only; CLI signature matches; CI workflow + README + AGENTS.md populated.\n"
             "ESCALATE: false\n"
             "ISSUES:\n"
             "- none\n"),
    ])


if __name__ == "__main__":
    main()
