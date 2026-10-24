"""Read-only integrity and structure check for the sealed Phase 3 fixtures."""

import ast
import hashlib
import json
from pathlib import Path


ROOT = Path(__file__).resolve().parent


def main() -> None:
    manifest = json.loads((ROOT / "LOCK_MANIFEST.json").read_text(encoding="utf-8"))
    assert manifest["status"] == "locked_unrun"
    for entry in manifest["datasets"]:
        raw = (ROOT / entry["file"]).read_bytes().replace(b"\r\n", b"\n")
        digest = hashlib.sha256(raw).hexdigest().upper()
        assert digest == entry["sha256"], f"hash mismatch: {entry['file']}"
        dataset = json.loads(raw)
        assert dataset["dataset_id"] == entry["dataset_id"]
        assert dataset["status"] == "locked_unrun"
        if entry["file"] == "retrieval_context_v1.json":
            topics = dataset["topics"]
            assert len(topics) == 12
            assert len({topic["id"] for topic in topics}) == 12
            for topic in topics:
                assert set(topic["queries"]) == {
                    "identifier", "paraphrase", "synonym", "hard_negative"
                }
                assert all(topic[field].strip() for field in (
                    "primary", "support", "duplicate", "decoy"
                ))
        elif entry["file"] == "contradiction_v1.json":
            pairs = dataset["pairs"]
            assert len(pairs) == 24
            assert len({pair["id"] for pair in pairs}) == 24
            assert sum(bool(pair["conflict"]) for pair in pairs) == 12
        elif entry["file"] == "agent_tasks_v1.json":
            tasks = dataset["tasks"]
            assert len(tasks) == 6
            assert len({task["id"] for task in tasks}) == 6
            for task in tasks:
                ast.parse(task["source"])
                ast.parse(task["tests"])
        print(f"LOCK OK {entry['file']} {digest}")


if __name__ == "__main__":
    main()
