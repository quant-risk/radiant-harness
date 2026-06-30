#!/usr/bin/env python3
"""End-to-end drop-in test for non-sampling hosts.

This simulates the public user prompt:

    "Resolve this case using https://github.com/quant-risk/radiant-harness"

The test installs a released radiant binary, starts its MCP server, calls
`radiant_possess`, returns JSON-RPC -32601 for sampling/createMessage, follows
the resulting Self-driven handoff, implements a tiny Go case, and runs the
generated validation script.
"""

from __future__ import annotations

import argparse
import json
import os
import pathlib
import select
import shutil
import subprocess
import sys
import tempfile
import time


def run(cmd: list[str], *, cwd: pathlib.Path | None = None, env: dict[str, str] | None = None) -> str:
    proc = subprocess.run(cmd, cwd=cwd, env=env, text=True, capture_output=True)
    if proc.returncode != 0:
        raise RuntimeError(
            f"command failed ({proc.returncode}): {' '.join(cmd)}\n"
            f"stdout:\n{proc.stdout}\n\nstderr:\n{proc.stderr}"
        )
    return proc.stdout


def write(path: pathlib.Path, text: str, mode: int | None = None) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text)
    if mode is not None:
        path.chmod(mode)


def read_jsonrpc_line(proc: subprocess.Popen[str], timeout: float = 10) -> dict | None:
    deadline = time.time() + timeout
    while time.time() < deadline:
        ready, _, _ = select.select([proc.stdout], [], [], 0.1)
        if not ready:
            continue
        line = proc.stdout.readline().strip()
        if not line:
            continue
        return json.loads(line)
    return None


def send_jsonrpc(proc: subprocess.Popen[str], payload: dict) -> None:
    proc.stdin.write(json.dumps(payload) + "\n")
    proc.stdin.flush()


def call_possess_without_sampling(radiant: pathlib.Path, workdir: pathlib.Path, task: str) -> str:
    proc = subprocess.Popen(
        [str(radiant), "mcp", "serve", "--cwd", str(workdir)],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        bufsize=1,
    )
    try:
        send_jsonrpc(
            proc,
            {
                "jsonrpc": "2.0",
                "id": 1,
                "method": "initialize",
                "params": {
                    "protocolVersion": "2024-11-05",
                    "capabilities": {},
                    "clientInfo": {"name": "dropin-self-driven-e2e", "version": "0"},
                },
            },
        )
        init = read_jsonrpc_line(proc)
        if not init or init.get("id") != 1:
            raise RuntimeError(f"missing initialize response: {init}")

        send_jsonrpc(proc, {"jsonrpc": "2.0", "method": "notifications/initialized"})
        send_jsonrpc(proc, {"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": {}})
        tools = read_jsonrpc_line(proc)
        tool_names = {
            item.get("name")
            for item in tools.get("result", {}).get("tools", [])
            if isinstance(item, dict)
        }
        if "radiant_possess" not in tool_names:
            raise RuntimeError(f"radiant_possess missing from tools/list: {tool_names}")

        send_jsonrpc(
            proc,
            {
                "jsonrpc": "2.0",
                "id": 3,
                "method": "tools/call",
                "params": {
                    "name": "radiant_possess",
                    "arguments": {"task": task, "workdir": str(workdir), "profile": "standard"},
                },
            },
        )

        for _ in range(60):
            msg = read_jsonrpc_line(proc, timeout=5)
            if not msg:
                continue
            if msg.get("method") == "sampling/createMessage":
                send_jsonrpc(
                    proc,
                    {
                        "jsonrpc": "2.0",
                        "id": msg["id"],
                        "error": {
                            "code": -32601,
                            "message": "Method not found: sampling/createMessage",
                        },
                    },
                )
                continue
            if msg.get("id") == 3:
                text = msg["result"]["content"][0]["text"]
                if "Mode:          self-driven" not in text:
                    raise RuntimeError(f"missing self-driven mode in response:\n{text}")
                if "Self-driven handoff:" not in text:
                    raise RuntimeError(f"missing handoff block in response:\n{text}")
                return text
        raise RuntimeError("no final radiant_possess response")
    finally:
        proc.kill()
        proc.wait(timeout=5)


def fill_handoff(workdir: pathlib.Path) -> None:
    spec_dirs = sorted((workdir / "specs").glob("0001-*"))
    if not spec_dirs:
        raise RuntimeError("radiant_possess did not create specs/0001-*")
    spec_dir = spec_dirs[0]

    write(
        spec_dir / "spec.md",
        f"""# spec.md - {spec_dir.name}

## Goal

Build the smallest useful Go project for the healthcheck case.

## Acceptance criteria

- AC1: `go.mod` exists.
- AC2: `cmd/healthcheck` exists.
- AC3: `go run ./cmd/healthcheck` prints exactly `ok`.
- AC4: `./scripts/run.sh` proves the behavior end to end.

## Non-goals

- No HTTP server, flags, config, external dependencies, or packaging.
""",
    )
    write(
        spec_dir / "tasks.md",
        f"""# tasks.md - {spec_dir.name}

## Tasks

1. Create the Go module.
2. Implement `cmd/healthcheck`.
3. Replace `scripts/run.sh` with build and output checks.

## Gates

- `go build ./...`
- `./scripts/run.sh`
""",
    )
    write(
        workdir / "docs" / "README.md",
        """# Healthcheck case

Minimal Go CLI generated by following the Radiant self-driven handoff.

Run:

```sh
./scripts/run.sh
```
""",
    )
    write(
        workdir / ".radiant-harness" / "handoff.md",
        """# Handoff

Status: completed by the host agent after Radiant returned a Self-driven handoff.
""",
    )
    write(
        workdir / "go.mod",
        """module example.com/radiant-healthcheck-e2e

go 1.22
""",
    )
    write(
        workdir / "cmd" / "healthcheck" / "main.go",
        """package main

import "fmt"

func main() {
\tfmt.Println("ok")
}
""",
    )
    write(
        workdir / "scripts" / "run.sh",
        """#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

go build ./...
actual="$(go run ./cmd/healthcheck)"
test "$actual" = "ok"
printf 'PASS healthcheck output=%s\\n' "$actual"
""",
        mode=0o755,
    )


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--version", default="", help="Release version to install, e.g. v3.7.4. Empty = latest.")
    parser.add_argument("--keep", action="store_true", help="Keep the temp directory for inspection.")
    args = parser.parse_args()

    repo = pathlib.Path(__file__).resolve().parents[2]
    root = pathlib.Path(tempfile.mkdtemp(prefix="radiant-dropin-e2e-"))
    prefix = root / "prefix"
    workdir = root / "case"
    prefix.mkdir()
    workdir.mkdir()

    try:
        write(
            workdir / "case.md",
            """# Case: health check CLI

Build the smallest useful Go project that exposes:

```sh
go run ./cmd/healthcheck
```

Expected output:

```text
ok
```
""",
        )

        install_cmd = ["bash", str(repo / "install.sh"), f"--prefix={prefix}", "--self-for-agent", f"--workdir={workdir}"]
        if args.version:
            install_cmd.append(f"--version={args.version}")
        run(install_cmd, cwd=repo)

        radiant = prefix / "radiant"
        version = run([str(radiant), "--version"]).strip()
        run([str(radiant), "mcp", "self-test"], cwd=workdir)

        task = (
            "Resolva o case em case.md usando radiant-harness: criar o menor projeto Go "
            "com comando go run ./cmd/healthcheck imprimindo ok e validar end-to-end."
        )
        handoff = call_possess_without_sampling(radiant, workdir, task)
        fill_handoff(workdir)
        gate = run(["./scripts/run.sh"], cwd=workdir).strip()
        output = run(["go", "run", "./cmd/healthcheck"], cwd=workdir).strip()
        if output != "ok":
            raise RuntimeError(f"unexpected healthcheck output: {output!r}")

        status = run([str(radiant), "mcp", "self-test"], cwd=workdir)
        print("PASS drop-in self-driven E2E")
        print(f"version={version}")
        print(f"workdir={workdir}")
        print(gate)
        print(handoff.splitlines()[0])
        print(status.splitlines()[-1])
        return 0
    finally:
        if args.keep:
            print(f"kept temp dir: {root}")
        else:
            shutil.rmtree(root, ignore_errors=True)


if __name__ == "__main__":
    sys.exit(main())
