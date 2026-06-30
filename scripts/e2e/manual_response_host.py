#!/usr/bin/env python3
"""
Real E2E MCP client for radiant-harness.

Spawns `radiant mcp serve` as a subprocess, drives the canonical JSON-RPC
sequence, and DELEGATES `sampling/createMessage` requests to the human
operator (Mavis) by:

  1. Receiving the sampling request
  2. Writing the prompt to /tmp/mcp-prompt-<id>.md
  3. Reading /tmp/mcp-response-<id>.md (the human's reply)
  4. Sending that back as the sampling/createMessage response

This is the manual equivalent of a real MCP host (Claude Code, Cursor,
etc.) that has an LLM behind it.
"""

import json
import os
import subprocess
import sys
import threading
import time


def main():
    binary = sys.argv[1]
    workdir = sys.argv[2]
    task = sys.argv[3]

    proc = subprocess.Popen(
        [binary, "mcp", "serve", "--cwd", workdir],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
        bufsize=0,
    )

    inbox: list[dict] = []
    pending_tools_call: dict | None = None

    def reader():
        """Read JSON-RPC responses from the harness."""
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

    def send(req):
        body = (json.dumps(req) + "\n").encode()
        proc.stdin.write(body)
        proc.stdin.flush()

    def wait_for(target_id=None, method=None, timeout=60):
        """Block until a matching response arrives."""
        deadline = time.time() + timeout
        while time.time() < deadline:
            for r in list(inbox):
                inbox.remove(r)
                if target_id is not None and r.get("id") == target_id:
                    return r
                if method is not None and r.get("method") == method:
                    return r
            time.sleep(0.2)
        return None

    def write_prompt(prompt_id, content):
        path = f"/tmp/mcp-prompt-{prompt_id}.md"
        with open(path, "w") as f:
            f.write(content)
        print(f"\n{'='*70}", flush=True)
        print(f"SAMPLING REQUEST id={prompt_id}", flush=True)
        print(f"  → /tmp/mcp-prompt-{prompt_id}.md", flush=True)
        print(f"  operator (Mavis): review the prompt, then save your", flush=True)
        print(f"  response to /tmp/mcp-response-{prompt_id}.md", flush=True)
        print(f"{'='*70}\n", flush=True)

    def read_response(prompt_id, timeout=110):
        """Poll for /tmp/mcp-response-<id>.md. Times out before the harness
        hits its own 2-minute sampling deadline so the host returns its
        sentinel to the harness — the driver then continues (response
        present) or downgrades (timeout reached → driver fallback).
        """
        path = f"/tmp/mcp-response-{prompt_id}.md"
        deadline = time.time() + timeout
        while time.time() < deadline:
            if os.path.exists(path):
                # Wait briefly so the file is fully written.
                for _ in range(20):
                    try:
                        if os.path.getsize(path) > 0:
                            break
                    except OSError:
                        pass
                    time.sleep(0.05)
                with open(path) as f:
                    return f.read()
            time.sleep(0.3)
        raise TimeoutError(
            f"no response at {path} within {timeout}s — operator did not respond in time"
        )

    # 1. initialize
    print("[1/5] initialize")
    send({
        "jsonrpc": "2.0",
        "id": 1,
        "method": "initialize",
        "params": {
            "protocolVersion": "2024-11-05",
            "capabilities": {},
            "clientInfo": {"name": "real-e2e-host", "version": "0.0.1"},
        },
    })
    r = wait_for(target_id=1, timeout=10)
    assert r, "no initialize response"
    si = r["result"]["serverInfo"]
    print(f"      ✓ serverInfo: {si['name']} v{si['version']}")

    # 2. notifications/initialized
    print("[2/5] notifications/initialized")
    send({"jsonrpc": "2.0", "method": "notifications/initialized"})

    # 3. tools/list
    print("[3/5] tools/list")
    send({"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": {}})
    r = wait_for(target_id=2, timeout=10)
    assert r, "no tools/list response"
    tools = [t["name"] for t in r["result"]["tools"]]
    print(f"      ✓ tools: {tools}")

    # 4. tools/call radiant_possess
    print("[4/5] tools/call radiant_possess")
    print(f"      task: {task}")
    send({
        "jsonrpc": "2.0",
        "id": 3,
        "method": "tools/call",
        "params": {
            "name": "radiant_possess",
            "arguments": {"task": task, "workdir": workdir},
        },
    })

    # 5. Drive sampling/createMessage round-trips
    print("[5/5] awaiting sampling/createMessage requests…")
    sampling_done = False
    while not sampling_done:
        # Find any new sampling requests
        for r in list(inbox):
            inbox.remove(r)
            if r.get("method") == "sampling/createMessage":
                prompt_id = r["id"]
                # Extract the prompt text from the params
                msgs = r.get("params", {}).get("messages", [])
                text_parts = []
                for m in msgs:
                    content = m.get("content", {})
                    if content.get("type") == "text":
                        text_parts.append(content.get("text", ""))
                full_prompt = "\n---\n".join(text_parts)

                write_prompt(prompt_id, full_prompt)
                print(f"      waiting for /tmp/mcp-response-{prompt_id}.md …")
                response_text = read_response(prompt_id)
                print(f"      ✓ got response ({len(response_text)} chars)")

                # The response may be either:
                #   - Plain text (treated as a single text block)
                #   - A JSON array with text + tool_use blocks
                #   - A JSON array with content blocks (Anthropic-style)
                # Try to parse as JSON; otherwise wrap as a single text block.
                response_content = None
                stripped = response_text.lstrip()
                if stripped.startswith("["):
                    try:
                        parsed = json.loads(stripped)
                        if isinstance(parsed, list):
                            response_content = parsed
                    except Exception:
                        pass
                if response_content is None:
                    response_content = [
                        {"type": "text", "text": response_text}
                    ]

                # Send back as sampling/createMessage response
                send({
                    "jsonrpc": "2.0",
                    "id": prompt_id,
                    "result": {
                        "role": "assistant",
                        "content": response_content,
                    },
                })

        # Check if workdir has reached a stopping criterion
        if (
            os.path.exists(workdir + "/AGENTS.md")
            and os.path.exists(workdir + "/specs")
            and any(
                d.startswith("0001-")
                for d in os.listdir(workdir + "/specs")
            )
            and os.path.exists(workdir + "/.radiant-harness/state")
        ):
            # All four artefacts present — wait briefly for harness to wrap up
            for r in list(inbox):
                if r.get("id") == 3:
                    # tools/call final response
                    content = r.get("result", {}).get("content", [])
                    text = "\n".join(
                        c.get("text", "") for c in content
                        if c.get("type") == "text"
                    )
                    if "all phases done" in text or "Loop finished" in text:
                        print(f"\n      ✓ response from tools/call id=3:")
                        print(textwrap_indent(text, "        "))
                        sampling_done = True
                        break

        # Also check if the phase order has reached "verify" and "done"
        state_files = []
        sd = workdir + "/.radiant-harness/state"
        if os.path.isdir(sd):
            for d in os.listdir(sd):
                if d.startswith("possess-"):
                    state_path = os.path.join(sd, d, "state.json")
                    if os.path.exists(state_path):
                        try:
                            with open(state_path) as f:
                                st = json.load(f)
                            state_files.append(st)
                        except Exception:
                            pass

        if state_files:
            current = state_files[-1].get("current_phase", "")
            if current == "done":
                # Drain any pending responses
                for r in list(inbox):
                    if r.get("id") == 3:
                        content = r.get("result", {}).get("content", [])
                        text = "\n".join(
                            c.get("text", "")
                            for c in content
                            if c.get("type") == "text"
                        )
                        print(f"\n      ✓ Phase done — tools/call response:")
                        print(textwrap_indent(text, "        "))
                sampling_done = True

        time.sleep(0.4)

    print("\n[end] killing subprocess")
    proc.terminate()
    try:
        proc.wait(timeout=5)
    except subprocess.TimeoutExpired:
        proc.kill()


def textwrap_indent(text, prefix):
    out = []
    for line in text.split("\n"):
        out.append(prefix + line)
    return "\n".join(out)


if __name__ == "__main__":
    main()
