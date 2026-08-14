#!/usr/bin/env python3

import json
import unittest
from pathlib import Path


QUALITY_ROOT = Path(__file__).resolve().parents[2]
PROJECT_ROOT = QUALITY_ROOT.parents[1]
PRODUCTION_ROOTS = (
    PROJECT_ROOT / "embedfs" / "templates",
    PROJECT_ROOT / "internal",
)


class PromptIndependenceTest(unittest.TestCase):
    def test_production_knowledge_pipeline_contains_no_fixture_markers(self):
        findings = []
        for config_path in sorted((QUALITY_ROOT / "cases").glob("*.json")):
            config = json.loads(config_path.read_text(encoding="utf-8"))
            markers = config.get("prompt_independence", {}).get("markers", [])
            self.assertEqual(len(markers), len(set(markers)), f"duplicate marker in {config_path.name}")
            for path in self.production_files():
                text = path.read_text(encoding="utf-8", errors="ignore")
                for marker in markers:
                    if marker.casefold() in text.casefold():
                        findings.append(
                            f"{path.relative_to(PROJECT_ROOT)} contains fixture marker {marker!r} from {config_path.name}"
                        )

        self.assertEqual([], findings, "\n" + "\n".join(findings))

    @staticmethod
    def production_files():
        for root in PRODUCTION_ROOTS:
            for path in sorted(candidate for candidate in root.rglob("*") if candidate.is_file()):
                if path.name.endswith("_test.go") or "testdata" in path.parts:
                    continue
                yield path


if __name__ == "__main__":
    unittest.main()
