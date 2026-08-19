#!/usr/bin/env python3

import importlib.util
import json
import sys
import tempfile
import unittest
from pathlib import Path


TOOL_PATH = Path(__file__).resolve().parents[1] / "quality.py"
SPEC = importlib.util.spec_from_file_location("quality", TOOL_PATH)
quality = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = quality
SPEC.loader.exec_module(quality)


class QualityRunTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name)
        self.config_path = self.root / "case.json"
        self.config = {
            "case": "demo",
            "fixture": {"project_dir": "project"},
            "skill_roots": ["generated-skill"],
            "dimensions": {
                "delivery": {"points": 50},
                "knowledge": {"points": 50},
            },
            "thresholds": {"total": 100, "dimensions": {"delivery": 100, "knowledge": 100}},
            "checks": [
                {
                    "id": "files",
                    "type": "files_exist",
                    "dimension": "delivery",
                    "gate": True,
                    "score_cap": 40,
                    "paths": ["SKILL.md", "references/modules.md"],
                },
                {
                    "id": "links",
                    "type": "markdown_links_valid",
                    "dimension": "delivery",
                    "gate": True,
                    "score_cap": 40,
                    "files": ["**/*.md", "SKILL.md"],
                },
                {
                    "id": "rule",
                    "type": "contains_groups",
                    "dimension": "knowledge",
                    "groups": [["tenant context", "validated tenant"]],
                    "files": ["references/project-spec.md"],
                },
                {
                    "id": "relations",
                    "type": "module_relations_valid",
                    "dimension": "knowledge",
                    "gate": True,
                    "score_cap": 60,
                    "files": ["references/modules.md"],
                },
            ],
        }
        self.write_config()
        self.project = self.root / ".test" / "quality" / "runtime" / "demo" / "project"
        self.skill = self.project / "generated-skill"
        (self.skill / "references").mkdir(parents=True)
        (self.skill / "SKILL.md").write_text("[Spec](./references/project-spec.md)\n", encoding="utf-8")
        (self.skill / "references" / "project-spec.md").write_text("validated tenant context\n", encoding="utf-8")
        (self.skill / "references" / "modules.md").write_text(
            "## Orders\n\n- **Source path**: `internal/order`\n",
            encoding="utf-8",
        )

    def tearDown(self):
        self.temp.cleanup()

    def write_config(self):
        self.config_path.write_text(json.dumps(self.config), encoding="utf-8")

    def run_quality(self):
        return quality.QualityRun(self.root, self.config_path).score()

    def test_complete_skill_passes(self):
        report = self.run_quality()

        self.assertTrue(report["passed"])
        self.assertEqual(100, report["total_score"])

    def test_missing_rule_lowers_only_knowledge_dimension(self):
        (self.skill / "references" / "project-spec.md").write_text("background only\n", encoding="utf-8")

        report = self.run_quality()

        self.assertFalse(report["passed"])
        self.assertEqual(100, report["scores"]["delivery"]["percent"])
        self.assertLess(report["scores"]["knowledge"]["percent"], 100)

    def test_partial_check_receives_partial_points(self):
        self.config["checks"][2]["groups"] = [["tenant context"], ["missing rule"]]
        self.write_config()

        report = self.run_quality()
        rule = next(check for check in report["checks"] if check["check_id"] == "rule")

        self.assertEqual(0.5, rule["score_ratio"])
        self.assertGreater(rule["earned_points"], 0)
        self.assertLess(rule["earned_points"], rule["possible_points"])

    def test_broken_link_is_a_gate_failure(self):
        (self.skill / "SKILL.md").write_text("[Missing](./references/missing.md)\n", encoding="utf-8")

        report = self.run_quality()

        self.assertFalse(report["passed"])
        self.assertIn("links", report["gate_failures"])
        self.assertEqual(40, report["score_cap"])
        self.assertLessEqual(report["total_score"], 40)

    def test_unknown_and_self_module_relations_fail(self):
        (self.skill / "references" / "modules.md").write_text(
            "## Orders\n\n"
            "- **Source path**: `internal/order`\n"
            "- **Dependency hints**:\n"
            "  - internal/order\n"
            "  - internal/missing\n",
            encoding="utf-8",
        )

        report = self.run_quality()

        self.assertFalse(report["passed"])
        self.assertIn("relations", report["gate_failures"])

    def test_command_failure_is_not_scored_and_reports_the_failure(self):
        report = quality.QualityRun(self.root, self.config_path).score(command_exit=1, failed_step="sync")

        self.assertFalse(report["passed"])
        self.assertEqual("run_failed", report["status"])
        self.assertIsNone(report["total_score"])
        self.assertEqual("not_scored", report["grade"])
        self.assertEqual(1, len(report["checks"]))
        self.assertIn("failed before Skill quality could be evaluated", report["checks"][0]["description"])

    def test_invalid_dimension_points_are_rejected(self):
        self.config["dimensions"]["delivery"]["points"] = 40
        self.write_config()

        with self.assertRaisesRegex(ValueError, "must sum to 100"):
            quality.QualityRun(self.root, self.config_path)

    def test_section_contract_requires_an_explicit_source_anchor(self):
        self.config["checks"].append({
            "id": "business-pattern",
            "type": "section_contract",
            "dimension": "knowledge",
            "groups": [["boundary"]],
        })
        self.write_config()

        with self.assertRaisesRegex(ValueError, "requires source anchors"):
            quality.QualityRun(self.root, self.config_path)

    def test_frontmatter_contract_only_scores_the_entry_metadata(self):
        (self.skill / "SKILL.md").write_text(
            "---\n"
            "name: demo-dev\n"
            "description: Use for requirements, debugging, APIs, configuration, workflows, and verification.\n"
            "---\n\n"
            "The body mentions a business flow.\n",
            encoding="utf-8",
        )
        run = self.run_quality_instance()
        item = {
            "files": ["SKILL.md"],
            "required_fields": ["name", "description"],
            "name_pattern": "^[a-z0-9-]+$",
            "description_groups": [["requirements"], ["business flow"], ["APIs"], ["verification"]],
            "max_description_length": 200,
        }

        ratio, details = run._frontmatter_contract(item)

        self.assertLess(ratio, 1)
        self.assertTrue(any("business flow" in detail for detail in details))

    def test_frontmatter_contract_accepts_complete_trigger_description(self):
        (self.skill / "SKILL.md").write_text(
            "---\n"
            "name: demo-dev\n"
            "description: Use for requirements, business flows, APIs, configuration, workflows, and verification.\n"
            "---\n",
            encoding="utf-8",
        )
        run = self.run_quality_instance()
        item = {
            "files": ["SKILL.md"],
            "required_fields": ["name", "description"],
            "name_pattern": "^[a-z0-9-]+$",
            "description_groups": [["requirements"], ["business flow"], ["APIs"], ["verification"]],
            "max_description_length": 200,
        }

        ratio, details = run._frontmatter_contract(item)

        self.assertEqual(1, ratio)
        self.assertEqual([], details)

    def test_route_contract_requires_reference_and_evidence_for_each_focus(self):
        (self.skill / "references" / "focus.md").write_text("focus evidence\n", encoding="utf-8")
        (self.skill / "SKILL.md").write_text(
            "## Development Focuses\n\n"
            "Use changed path -> request signal -> reference page -> source evidence.\n"
            "For multiple matches, read every matched reference.\n\n"
            "| Focus | Request terms | References | Evidence entries |\n"
            "|---|---|---|---|\n"
            "| Orders | `order` | [open](./references/focus.md) | `internal/order/service.ext:1` |\n",
            encoding="utf-8",
        )
        run = self.run_quality_instance()
        ratio, details = run._route_contract({
            "files": ["SKILL.md"],
            "minimum_focuses": 1,
            "requirements": [
                ["changed path"],
                ["request signal"],
                ["every matched reference"],
                ["source evidence"],
            ],
        })

        self.assertEqual(1, ratio)
        self.assertEqual([], details)

    def test_existing_project_and_skill_root_can_be_scored(self):
        external_project = self.root / "existing"
        external_skill = external_project / "skill"
        (external_skill / "references").mkdir(parents=True)
        (external_skill / "SKILL.md").write_text("[Spec](./references/project-spec.md)\n", encoding="utf-8")
        (external_skill / "references" / "project-spec.md").write_text("validated tenant context\n", encoding="utf-8")
        (external_skill / "references" / "modules.md").write_text(
            "## Orders\n\n- **Source path**: `internal/order`\n",
            encoding="utf-8",
        )

        report = quality.QualityRun(self.root, self.config_path, external_project, [external_skill]).score()

        self.assertEqual(100, report["total_score"])
        self.assertEqual(str(external_skill.resolve()), report["artifacts"]["skill_roots"][0])

    def test_explicit_relative_skill_root_uses_current_directory(self):
        relative = Path(".test") / "quality" / "runtime" / "demo" / "project" / "generated-skill"

        run = quality.QualityRun(self.root, self.config_path, self.project, [relative])

        self.assertEqual((Path.cwd() / relative).resolve(), run.skill_roots[0])

    def test_section_contract_requires_anchor_and_groups_in_one_block(self):
        item = {
            "files": ["references/patterns/**/*.md"],
            "anchors": ["internal/payment/service\\.go"],
            "groups": [["idempoten"], ["uncertain"]],
        }
        path = self.skill / "references" / "patterns" / "business.md"
        path.parent.mkdir(parents=True)
        path.write_text(
            "## Payment\n\n"
            "Source: `internal/payment/service.go`\n\n"
            "The operation is idempotent and returns an uncertain outcome.\n",
            encoding="utf-8",
        )

        ratio, details = self.run_quality_instance()._section_contract(item)

        self.assertEqual(1, ratio)
        self.assertEqual([], details)

    def test_section_contract_does_not_join_semantics_across_blocks(self):
        item = {
            "files": ["references/patterns/**/*.md"],
            "anchors": ["internal/payment/service\\.go"],
            "groups": [["idempoten"], ["uncertain"]],
        }
        path = self.skill / "references" / "patterns" / "business.md"
        path.parent.mkdir(parents=True)
        path.write_text(
            "## Payment\n\nSource: `internal/payment/service.go`\n\n"
            "## Replay\n\nIdempotency reuses a result.\n\n"
            "## Persistence\n\nThe result can be uncertain.\n",
            encoding="utf-8",
        )

        ratio, details = self.run_quality_instance()._section_contract(item)

        self.assertEqual(0, ratio)
        self.assertTrue(any("best matching block misses" in detail for detail in details))

    def test_section_contract_can_aggregate_blocks_with_the_same_source_anchor(self):
        item = {
            "files": ["references/patterns/**/*.md"],
            "anchors": ["internal/payment/service\\.go"],
            "groups": [["idempoten"], ["uncertain"]],
            "aggregate_by_anchor": True,
        }
        path = self.skill / "references" / "patterns" / "business.md"
        path.parent.mkdir(parents=True)
        path.write_text(
            "## Replay\n\nSource: `internal/payment/service.go`\n\nIdempotency reuses a result.\n\n"
            "## Persistence\n\nSource: `internal/payment/service.go`\n\nThe result can be uncertain.\n\n"
            "## Unrelated\n\nA separate component uses fallback ordering.\n",
            encoding="utf-8",
        )

        ratio, details = self.run_quality_instance()._section_contract(item)

        self.assertEqual(1, ratio)
        self.assertEqual([], details)

    def test_section_contract_awards_partial_credit_within_anchored_block(self):
        item = {
            "files": ["references/patterns/**/*.md"],
            "anchors": ["internal/payment/service\\.go"],
            "groups": [["idempoten"], ["uncertain"]],
        }
        path = self.skill / "references" / "patterns" / "business.md"
        path.parent.mkdir(parents=True)
        path.write_text(
            "## Payment\n\nSource: `internal/payment/service.go`\n\nIdempotency reuses a result.\n",
            encoding="utf-8",
        )

        ratio, details = self.run_quality_instance()._section_contract(item)

        self.assertEqual(0.5, ratio)
        self.assertEqual(1, len(details))

    def test_minimum_evidence_prevents_empty_link_and_source_checks_from_passing(self):
        run = self.run_quality_instance()

        link_ratio, link_details = run._markdown_links_valid({
            "files": ["references/project-spec.md"],
            "minimum": 1,
        })
        source_ratio, source_details = run._source_paths_valid({
            "files": ["references/project-spec.md"],
            "minimum": 1,
            "prefixes": ["internal/"],
            "suffixes": [".go"],
        })

        self.assertEqual(0, link_ratio)
        self.assertEqual(0, source_ratio)
        self.assertTrue(any("expected at least" in detail for detail in link_details))
        self.assertTrue(any("expected at least" in detail for detail in source_details))

    def test_output_stability_accepts_identical_skill_trees(self):
        run = self.run_quality_instance()
        for index in (1, 2):
            root = run.runtime / "snapshots" / f"run-{index}" / "skills" / "0"
            root.mkdir(parents=True)
            (root / "SKILL.md").write_text("stable\n", encoding="utf-8")
            (root / "reference.md").write_text("knowledge\n", encoding="utf-8")

        ratio, details = run._output_stability({"runs": [1, 2], "minimum": 2})

        self.assertEqual(1, ratio)
        self.assertEqual([], details)

    def test_output_stability_reports_changed_files(self):
        run = self.run_quality_instance()
        first = run.runtime / "snapshots" / "run-1" / "skills" / "0"
        second = run.runtime / "snapshots" / "run-2" / "skills" / "0"
        first.mkdir(parents=True)
        second.mkdir(parents=True)
        (first / "SKILL.md").write_text("first\n", encoding="utf-8")
        (second / "SKILL.md").write_text("second\n", encoding="utf-8")

        ratio, details = run._output_stability({"runs": [1, 2]})

        self.assertEqual(0, ratio)
        self.assertEqual(["run 2 changed 0/SKILL.md"], details)

    def test_output_stability_requires_snapshots(self):
        ratio, details = self.run_quality_instance()._output_stability({"runs": [1, 2]})

        self.assertEqual(0, ratio)
        self.assertEqual(2, len(details))

    def test_repeat_sync_arguments_default_to_sync_args(self):
        self.config["run"] = {"sync_args": ["sync", "--no-interactive"]}
        self.write_config()
        run = self.run_quality_instance()

        self.assertEqual(["sync", "--no-interactive"], run._sync_args(1))
        self.assertEqual(["sync", "--no-interactive"], run._sync_args(2))

    def test_repeat_sync_arguments_can_avoid_initial_rebuild(self):
        self.config["run"] = {
            "initial_sync_args": ["sync", "--restart"],
            "repeat_sync_args": ["sync", "--no-interactive"],
        }
        self.write_config()
        run = self.run_quality_instance()

        self.assertEqual(["sync", "--restart"], run._sync_args(1))
        self.assertEqual(["sync", "--no-interactive"], run._sync_args(2))

    def run_quality_instance(self):
        return quality.QualityRun(self.root, self.config_path)


if __name__ == "__main__":
    unittest.main()
