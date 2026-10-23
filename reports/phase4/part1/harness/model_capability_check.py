#!/usr/bin/env python3
"""Phase 4A local-model tool-calling capability check.

Drives a locally served chat model through the same tool schema the six-task
agent harness uses, against a tiny sandboxed Python project, and reports whether
the model can:

  1. emit syntactically valid tool calls,
  2. pass valid arguments,
  3. consume tool results,
  4. inspect and modify a sandboxed coding project,
  5. produce a final task output.

A model that fails this check is classified UNSUITABLE for the agent benchmark;
its downstream task failures must not be read as memory-system failures.

Read-only with respect to the repo: everything happens under --workdir.
"""
import argparse
import json
import subprocess
import sys
import urllib.request
from pathlib import Path

TOOLS = [
    {"type": "function", "function": {"name": "read_file", "description": "Read a file.", "parameters": {"type": "object", "required": ["name"], "properties": {"name": {"type": "string"}}}}},
    {"type": "function", "function": {"name": "write_file", "description": "Write a file.", "parameters": {"type": "object", "required": ["name", "content"], "properties": {"name": {"type": "string"}, "content": {"type": "string"}}}}},
    {"type": "function", "function": {"name": "run_tests", "description": "Run the project tests.", "parameters": {"type": "object", "properties": {}}}},
]

SOURCE = "def add(a, b):\n    return a - b\n"
TESTS = ("import unittest\nfrom solution import add\n\n"
         "class T(unittest.TestCase):\n"
         "    def test_add(self):\n"
         "        self.assertEqual(add(2, 3), 5)\n\n"
         "if __name__ == '__main__':\n    unittest.main()\n")

SYSTEM = ("You are a coding agent in a disposable Python fixture. Use read_file, "
          "write_file and run_tests to make the tests pass. Do not edit tests.")
REQUEST = ("solution.py defines add(a, b) but it is wrong. Read solution.py, fix it so "
           "the tests pass, run the tests, and then say DONE.")


def chat(url, model, messages):
    payload = {"model": model, "messages": messages, "tools": TOOLS, "stream": False,
               "options": {"temperature": 0, "seed": 42, "num_ctx": 4096, "num_predict": 512}}
    req = urllib.request.Request(url, data=json.dumps(payload).encode(),
                                 headers={"Content-Type": "application/json"})
    with urllib.request.urlopen(req, timeout=180) as resp:
        return json.load(resp)


def _extract_json_objects(text):
    dec = json.JSONDecoder()
    objs = []
    i, n = 0, len(text)
    while i < n:
        while i < n and text[i] in " \t\r\n`":
            i += 1
        if i >= n:
            break
        try:
            obj, end = dec.raw_decode(text, i)
        except json.JSONDecodeError:
            nl = text.find("\n", i)
            if nl == -1:
                break
            i = nl + 1
            continue
        objs.append(obj)
        i = end
    return objs


def normalize(message):
    calls = message.get("tool_calls") or []
    if calls:
        return calls, "native"
    content = (message.get("content") or "").strip()
    if content.startswith("```"):
        content = content.split("\n", 1)[-1].rsplit("```", 1)[0].strip()
    parsed = []
    for obj in _extract_json_objects(content):
        if (isinstance(obj, dict) and obj.get("name") in {"read_file", "write_file", "run_tests"}
                and isinstance(obj.get("arguments"), dict)):
            parsed.append({"function": obj})
    if parsed:
        return parsed, "text_json"
    return [], "none"


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--ollama-url", default="http://127.0.0.1:11434/api/chat")
    p.add_argument("--model", default="qwen2.5-coder:7b")
    p.add_argument("--workdir", required=True)
    p.add_argument("--output", required=True)
    args = p.parse_args()

    work = Path(args.workdir)
    work.mkdir(parents=True, exist_ok=True)
    (work / "solution.py").write_text(SOURCE)
    (work / "test_solution.py").write_text(TESTS)

    messages = [{"role": "system", "content": SYSTEM},
                {"role": "user", "content": REQUEST}]
    tool_calls = []
    call_sources = []
    final_text = ""
    syntax_ok = True
    args_valid = True

    for _ in range(8):
        resp = chat(args.ollama_url, args.model, messages)
        message = resp.get("message", {})
        final_text = message.get("content", "") or final_text
        calls, source = normalize(message)
        messages.append(message)
        if not calls:
            break
        call_sources.append(source)
        for call in calls:
            fn = call.get("function", {})
            name = fn.get("name", "")
            arguments = fn.get("arguments", {})
            if isinstance(arguments, str):
                try:
                    arguments = json.loads(arguments)
                except json.JSONDecodeError:
                    args_valid = False
                    arguments = {}
            if name == "read_file":
                if arguments.get("name") != "solution.py":
                    args_valid = False
                result = (work / "solution.py").read_text()
            elif name == "write_file":
                if arguments.get("name") != "solution.py" or not isinstance(arguments.get("content"), str):
                    args_valid = False
                else:
                    (work / "solution.py").write_text(arguments["content"])
                result = "written"
            elif name == "run_tests":
                proc = subprocess.run([sys.executable, "-m", "unittest", "-q", "test_solution.py"],
                                      cwd=work, text=True, capture_output=True, timeout=20)
                result = json.dumps({"passed": proc.returncode == 0,
                                     "output": (proc.stdout + proc.stderr)[-500:]})
            else:
                syntax_ok = False
                result = "unknown tool"
            tool_calls.append({"name": name, "arguments": arguments if name != "write_file" else {"name": arguments.get("name")}, "result": result[:300]})
            messages.append({"role": "tool", "tool_name": name, "content": result})
            if len(tool_calls) >= 12:
                break
        if len(tool_calls) >= 12:
            break

    proc = subprocess.run([sys.executable, "-m", "unittest", "-q", "test_solution.py"],
                          cwd=work, text=True, capture_output=True, timeout=20)
    fixed = proc.returncode == 0
    report = {
        "model": args.model,
        "endpoint": args.ollama_url,
        "tool_call_syntax_ok": syntax_ok and any(c["name"] in {"read_file", "write_file", "run_tests"} for c in tool_calls),
        "tool_arguments_valid": args_valid,
        "consumed_tool_results": len(tool_calls) > 0,
        "used_write_file": any(c["name"] == "write_file" for c in tool_calls),
        "used_run_tests": any(c["name"] == "run_tests" for c in tool_calls),
        "fixed_project_tests_pass": fixed,
        "final_output_present": bool(final_text.strip()),
        "tool_calls": tool_calls,
        "call_sources": call_sources,
        "final_text": final_text[:500],
        "suitable": bool(syntax_ok and args_valid and fixed and final_text.strip()),
    }
    Path(args.output).write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({k: v for k, v in report.items() if k not in ("tool_calls",)}, indent=2))


if __name__ == "__main__":
    main()
