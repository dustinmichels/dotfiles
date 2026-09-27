#!/usr/bin/env python3
"""
Diagnostic & validation script for RTK and CodeGraph across:
- OMP (oh-my-pi)
- Claude Code
- Gemini / Antigravity
"""

import json
import os
import shutil
import subprocess
import sys

GREEN = "\033[32m"
YELLOW = "\033[33m"
RED = "\033[31m"
CYAN = "\033[36m"
BOLD = "\033[1m"
RESET = "\033[0m"


def status_badge(ok, note=""):
    if ok:
        badge = f"{GREEN}✓ OK{RESET}"
    else:
        badge = f"{RED}✗ FAIL{RESET}"
    return f"{badge}  {note}" if note else badge


def run(cmd, timeout=5, check=False):
    return subprocess.run(
        cmd,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        timeout=timeout,
        check=check,
    )


def main():
    print(f"\n{BOLD}{CYAN}=== AI Tools & Agents Health Check ==={RESET}\n")

    # -------------------------------------------------------------
    # 1. Binaries & Updates
    # -------------------------------------------------------------
    print(f"{BOLD}1. Binaries & Versions{RESET}")

    rtk_bin = shutil.which("rtk")
    if rtk_bin:
        res = run(["rtk", "--version"])
        ver = res.stdout.strip()
        print(f"  RTK binary         : {status_badge(True, f'{rtk_bin} ({ver})')}")
    else:
        print(
            f"  RTK binary         : {status_badge(False, 'Not found on PATH (run: brew install rtk)')}"
        )

    cg_bin = shutil.which("codegraph")
    if cg_bin:
        res = run(["codegraph", "upgrade", "--check"])
        cg_status = res.stdout.strip().replace("\n", " | ")
        print(f"  CodeGraph binary   : {status_badge(True, f'{cg_bin}')}")
        print(f"  CodeGraph version  :   {cg_status}")
    else:
        print(
            f"  CodeGraph binary   : {status_badge(False, 'Not found on PATH (run: mise install npm:@colbymchenry/codegraph)')}"
        )

    print()

    # -------------------------------------------------------------
    # 2. Oh My Pi (OMP)
    # -------------------------------------------------------------
    print(f"{BOLD}2. OMP (oh-my-pi / pi-coding-agent){RESET}")

    omp_rtk = os.path.expanduser("~/.omp/agent/extensions/rtk.ts")
    omp_rtk_ok = os.path.exists(omp_rtk)
    print(f"  RTK Extension      : {status_badge(omp_rtk_ok, omp_rtk)}")

    omp_mcp = os.path.expanduser("~/.omp/agent/mcp.json")
    omp_mcp_ok = False
    if os.path.exists(omp_mcp):
        try:
            with open(omp_mcp) as f:
                d = json.load(f)
            has_cg = "codegraph" in d.get("mcpServers", {})
            disabled = "codegraph" in d.get("disabledServers", [])
            omp_mcp_ok = has_cg and not disabled
            note = "configured & enabled" if omp_mcp_ok else "not enabled in mcp.json"
        except Exception as e:
            note = f"JSON error: {e}"
    else:
        note = "File missing"
    print(f"  CodeGraph MCP      : {status_badge(omp_mcp_ok, note)}")

    print()

    # -------------------------------------------------------------
    # 3. Claude Code
    # -------------------------------------------------------------
    print(f"{BOLD}3. Claude Code{RESET}")

    # RTK PreToolUse hook
    claude_hook_ok = False
    try:
        payload = json.dumps(
            {"tool_name": "Bash", "tool_input": {"command": "git status"}}
        )
        res = subprocess.run(
            ["rtk", "hook", "claude"],
            input=payload,
            text=True,
            capture_output=True,
            timeout=3,
        )
        claude_hook_ok = "rtk git status" in res.stdout
        note = (
            "auto-rewriting 'git status' -> 'rtk git status'"
            if claude_hook_ok
            else f"unexpected response: {res.stdout.strip()[:60]}"
        )
    except Exception as e:
        note = f"Hook execution error: {e}"
    print(f"  RTK Hook           : {status_badge(claude_hook_ok, note)}")

    # CodeGraph MCP in ~/.claude.json
    claude_json = os.path.expanduser("~/.claude.json")
    claude_mcp_ok = False
    if os.path.exists(claude_json):
        try:
            with open(claude_json) as f:
                d = json.load(f)
            claude_mcp_ok = "codegraph" in d.get("mcpServers", {})
            note = (
                "registered in ~/.claude.json"
                if claude_mcp_ok
                else "missing in ~/.claude.json"
            )
        except Exception as e:
            note = f"JSON error: {e}"
    else:
        note = "~/.claude.json not found"
    print(f"  CodeGraph MCP      : {status_badge(claude_mcp_ok, note)}")

    # CodeGraph prompt-hook
    try:
        p_res = run(["codegraph", "prompt-hook"], timeout=3)
        prompt_hook_ok = p_res.returncode == 0
        note = "exited 0" if prompt_hook_ok else f"exit code {p_res.returncode}"
    except Exception as e:
        prompt_hook_ok = False
        note = str(e)
    print(f"  CodeGraph Hook     : {status_badge(prompt_hook_ok, note)}")

    print()

    # -------------------------------------------------------------
    # 4. Gemini / Antigravity
    # -------------------------------------------------------------
    print(f"{BOLD}4. Gemini / Antigravity{RESET}")

    # RTK Hook script & test
    gem_hook = os.path.expanduser("~/.gemini/hooks/rtk-hook-antigravity.py")
    gem_hook_ok = False
    if os.path.exists(gem_hook) and os.access(gem_hook, os.X_OK):
        try:
            payload = json.dumps(
                {
                    "toolCall": {
                        "name": "run_command",
                        "args": {"CommandLine": "git status"},
                    }
                }
            )
            res = subprocess.run(
                [gem_hook], input=payload, text=True, capture_output=True, timeout=3
            )
            data = json.loads(res.stdout)
            gem_hook_ok = (
                data.get("overwrite", {}).get("CommandLine") == "rtk git status"
            )
            note = (
                "rewriting CommandLine -> 'rtk git status'"
                if gem_hook_ok
                else f"unexpected output: {res.stdout.strip()[:60]}"
            )
        except Exception as e:
            note = f"execution failed: {e}"
    else:
        note = "Script missing or not executable"
    print(f"  RTK Hook           : {status_badge(gem_hook_ok, note)}")

    # Antigravity hooks.json
    gem_hooks_json = os.path.expanduser("~/.gemini/config/hooks.json")
    hooks_registered = False
    if os.path.exists(gem_hooks_json):
        try:
            with open(gem_hooks_json) as f:
                d = json.load(f)
            hooks_registered = "rtk-rewrite" in d
            note = (
                "~/.gemini/config/hooks.json active"
                if hooks_registered
                else "rtk-rewrite not found in hooks.json"
            )
        except Exception as e:
            note = f"JSON error: {e}"
    else:
        note = "~/.gemini/config/hooks.json missing"
    print(f"  RTK Hook Config    : {status_badge(hooks_registered, note)}")

    # CodeGraph MCP in Antigravity
    gem_mcp = os.path.expanduser("~/.gemini/config/mcp_config.json")
    gem_mcp_ok = False
    if os.path.exists(gem_mcp):
        try:
            with open(gem_mcp) as f:
                d = json.load(f)
            cg_cmd = d.get("mcpServers", {}).get("codegraph", {}).get("command")
            if cg_cmd and os.path.exists(cg_cmd) and os.access(cg_cmd, os.X_OK):
                gem_mcp_ok = True
                note = f"points to valid binary: {cg_cmd}"
            else:
                note = f"points to dead path: {cg_cmd}"
        except Exception as e:
            note = f"JSON error: {e}"
    else:
        note = "~/.gemini/config/mcp_config.json missing"
    print(f"  CodeGraph MCP      : {status_badge(gem_mcp_ok, note)}")

    print(f"\n{BOLD}{CYAN}======================================{RESET}\n")


if __name__ == "__main__":
    main()
