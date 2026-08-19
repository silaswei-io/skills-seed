#!/usr/bin/env python3
"""Prepare fixtures and score generated project skills with configured assertions."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import shutil
import subprocess
import sys
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Optional


@dataclass
class CheckResult:
    check_id: str
    dimension: str
    description: str
    passed: bool
    gate: bool
    weight: float
    score_ratio: float
    possible_points: float
    earned_points: float
    score_cap: Optional[float]
    details: list[str]
    hint: str


class QualityRun:
    def __init__(self, root: Path, config_path: Path, project: Optional[Path] = None, skill_roots: Optional[list[Path]] = None):
        self.root = root.resolve()
        self.config_path = config_path.resolve()
        self.config = json.loads(self.config_path.read_text(encoding="utf-8"))
        self.external_score = project is not None or skill_roots is not None
        self._validate_config()
        self.case = self.config["case"]
        self.runtime = self.root / ".test" / "quality" / "runtime" / self.case
        default_project = self.runtime / self.config.get("fixture", {}).get("project_dir", "project")
        self.project = (project or default_project).resolve()
        self.report_dir = self.root / ".test" / "quality" / "reports" / self.case
        self.skill_roots = self._skill_roots(skill_roots)

    def prepare(self) -> None:
        if self.runtime.exists():
            shutil.rmtree(self.runtime)
        self.runtime.mkdir(parents=True)
        (self.runtime / "run.json").write_text(
            json.dumps({"case": self.case, "prepared_at": int(time.time())}, indent=2),
            encoding="utf-8",
        )

    def value(self, dotted_key: str):
        value = self.config
        for part in dotted_key.split("."):
            value = value[part]
        if isinstance(value, (dict, list)):
            print(json.dumps(value, ensure_ascii=False))
        else:
            print(value)

    def score(self, command_exit: Optional[int] = None, failed_step: str = "") -> dict:
        if command_exit not in (None, 0):
            results = [CheckResult(
                check_id="generation-command",
                dimension="delivery",
                description="The configured generation command failed before Skill quality could be evaluated.",
                passed=False,
                gate=True,
                weight=0,
                score_ratio=0,
                possible_points=0,
                earned_points=0,
                score_cap=None,
                details=[f"failed step: {failed_step}", f"command exit code: {command_exit}"],
                hint="Inspect the sync log before interpreting content scores.",
            )]
        else:
            checks = [item for item in self.config.get("checks", []) if not (self.external_score and item.get("run_only"))]
            results = [self._check(item) for item in checks]
            self._assign_points(results)
        report = self._report(results, command_exit not in (None, 0))
        self._write_report(report)
        return report

    def run(self) -> dict:
        self.prepare()
        run = self.config["run"]
        log_dir = self.runtime / "logs"
        log_dir.mkdir(parents=True)
        binary = self.runtime / "bin" / "skills-seed"
        binary.parent.mkdir(parents=True)

        steps = [
            ("build-skills-seed", ["go", "build", "-o", str(binary), "./cmd/skills-seed"], self.root),
            ("build-fixture", [str(self.root / run["builder"]), str(self.project)], self.root),
            ("git-init", ["git", "init", "-q"], self.project),
            ("git-add", ["git", "add", "."], self.project),
            ("git-commit", ["git", "-c", "user.name=Quality Suite", "-c", "user.email=quality@example.invalid", "commit", "-qm", "quality fixture"], self.project),
        ]
        agent = os.environ.get("QUALITY_AGENT", run.get("agent", "codex"))
        init_args = [value.replace("{agent}", agent) for value in run["init_args"]]
        steps.append(("init", [str(binary), *init_args], self.project))
        command_exit = 0
        failed_step = ""
        for name, command, cwd in steps:
            result = subprocess.run(command, cwd=cwd, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, check=False)
            (log_dir / f"{name}.log").write_text(result.stdout, encoding="utf-8")
            if result.returncode != 0:
                command_exit = result.returncode
                failed_step = name
                break
        repeat_count = max(1, int(run.get("repeat_sync", 1)))
        if command_exit == 0:
            for index in range(1, repeat_count + 1):
                name = "sync" if index == 1 else f"sync-repeat-{index}"
                sync_args = self._sync_args(index)
                result = subprocess.run(
                    [str(binary), *sync_args], cwd=self.project, text=True,
                    stdout=subprocess.PIPE, stderr=subprocess.STDOUT, check=False,
                )
                (log_dir / f"{name}.log").write_text(result.stdout, encoding="utf-8")
                if result.returncode != 0:
                    command_exit = result.returncode
                    failed_step = name
                    break
                self._snapshot_run(index)
        return self.score(command_exit, failed_step)

    def _sync_args(self, index: int) -> list[str]:
        run = self.config["run"]
        if index == 1:
            if "initial_sync_args" in run:
                return run["initial_sync_args"]
        elif "repeat_sync_args" in run:
            return run["repeat_sync_args"]
        return run["sync_args"]

    def _snapshot_run(self, index: int) -> None:
        snapshot = self.runtime / "snapshots" / f"run-{index}"
        if snapshot.exists():
            shutil.rmtree(snapshot)
        for root_index, skill_root in enumerate(self.skill_roots):
            if skill_root.exists():
                shutil.copytree(skill_root, snapshot / "skills" / str(root_index))
        documents = self.project / ".skills-seed" / "store" / "documents"
        if documents.exists():
            shutil.copytree(documents, snapshot / "documents")

    def _validate_config(self) -> None:
        dimensions = self.config.get("dimensions", {})
        if not dimensions:
            raise ValueError("quality config requires dimensions")
        total_points = sum(float(item.get("points", 0)) for item in dimensions.values())
        if abs(total_points - 100) > 0.000001:
            raise ValueError(f"dimension points must sum to 100, got {total_points}")
        known_types = {
            "files_exist", "files_absent", "contains_groups", "not_contains",
            "markdown_links_valid", "source_paths_valid", "frontmatter_contract", "entry_contract",
            "module_relations_valid", "max_overlap", "section_contract",
            "output_stability", "route_contract",
        }
        seen = set()
        for item in self.config.get("checks", []):
            check_id = item.get("id", "")
            if not check_id or check_id in seen:
                raise ValueError(f"quality check id must be unique and non-empty: {check_id!r}")
            seen.add(check_id)
            if item.get("dimension") not in dimensions:
                raise ValueError(f"quality check {check_id} references unknown dimension {item.get('dimension')!r}")
            if item.get("type") not in known_types:
                raise ValueError(f"quality check {check_id} uses unknown type {item.get('type')!r}")
            if float(item.get("weight", 1)) <= 0:
                raise ValueError(f"quality check {check_id} weight must be positive")
            if item.get("gate") and "score_cap" not in item:
                raise ValueError(f"quality gate {check_id} requires score_cap")
            if "score_cap" in item and not 0 <= float(item["score_cap"]) <= 100:
                raise ValueError(f"quality check {check_id} score_cap must be between 0 and 100")
            if int(item.get("minimum", 0)) < 0:
                raise ValueError(f"quality check {check_id} minimum must not be negative")
            if item.get("type") == "section_contract":
                if not item.get("anchors"):
                    raise ValueError(f"quality check {check_id} requires source anchors")
                if not item.get("groups"):
                    raise ValueError(f"quality check {check_id} requires semantic groups")

        for name in dimensions:
            if not any(item.get("dimension") == name for item in self.config.get("checks", [])):
                raise ValueError(f"quality dimension {name} has no checks")

    def _skill_roots(self, overrides: Optional[list[Path]] = None) -> list[Path]:
        if overrides:
            return [path.resolve() for path in overrides]
        configured = self.config.get("skill_roots", [])
        return [self.project / path for path in configured]

    def _check(self, item: dict) -> CheckResult:
        kind = item["type"]
        handlers = {
            "files_exist": self._files_exist,
            "files_absent": self._files_absent,
            "contains_groups": self._contains_groups,
            "not_contains": self._not_contains,
            "markdown_links_valid": self._markdown_links_valid,
            "source_paths_valid": self._source_paths_valid,
            "frontmatter_contract": self._frontmatter_contract,
            "entry_contract": self._entry_contract,
            "module_relations_valid": self._module_relations_valid,
            "max_overlap": self._max_overlap,
            "section_contract": self._section_contract,
            "output_stability": self._output_stability,
            "route_contract": self._route_contract,
        }
        if kind not in handlers:
            raise ValueError(f"unknown quality check type: {kind}")
        score_ratio, details = handlers[kind](item)
        score_ratio = min(1.0, max(0.0, score_ratio))
        return CheckResult(
            check_id=item["id"],
            dimension=item["dimension"],
            description=item.get("description", item["id"]),
            passed=score_ratio == 1,
            gate=bool(item.get("gate", False)),
            weight=float(item.get("weight", 1)),
            score_ratio=score_ratio,
            possible_points=0,
            earned_points=0,
            score_cap=float(item["score_cap"]) if "score_cap" in item else None,
            details=details,
            hint=item.get("hint", ""),
        )

    def _assign_points(self, results: list[CheckResult]) -> None:
        for name, settings in self.config["dimensions"].items():
            selected = [result for result in results if result.dimension == name]
            total_weight = sum(result.weight for result in selected)
            for result in selected:
                result.possible_points = float(settings["points"]) * result.weight / total_weight
                result.earned_points = result.possible_points * result.score_ratio

    def _files(self, patterns: Optional[list[str]] = None) -> list[Path]:
        files: set[Path] = set()
        for skill_root in self.skill_roots:
            if not skill_root.exists():
                continue
            if not patterns:
                files.update(path for path in skill_root.rglob("*") if path.is_file())
                continue
            for pattern in patterns:
                files.update(path for path in skill_root.glob(pattern) if path.is_file())
        return sorted(files)

    @staticmethod
    def _read(files: list[Path]) -> str:
        return "\n".join(path.read_text(encoding="utf-8", errors="ignore") for path in files)

    def _display(self, path: Path) -> str:
        try:
            return path.relative_to(self.project).as_posix()
        except ValueError:
            return str(path)

    @staticmethod
    def _fraction(matched: int, total: int) -> float:
        return 1.0 if total == 0 else matched / total

    def _files_exist(self, item: dict) -> tuple[float, list[str]]:
        missing = [pattern for pattern in item["paths"] if not self._files([pattern])]
        return self._fraction(len(item["paths"]) - len(missing), len(item["paths"])), [f"missing: {path}" for path in missing]

    def _files_absent(self, item: dict) -> tuple[float, list[str]]:
        if not any(root.exists() for root in self.skill_roots):
            return 0, ["no generated Skill root exists"]
        found_patterns = [pattern for pattern in item["paths"] if self._files([pattern])]
        details = [f"unexpected match: {pattern}" for pattern in found_patterns]
        return self._fraction(len(item["paths"]) - len(found_patterns), len(item["paths"])), details

    def _contains_groups(self, item: dict) -> tuple[float, list[str]]:
        files = self._files(item.get("files"))
        text = self._read(files)
        missing = []
        for group in item.get("groups", []):
            alternatives = group if isinstance(group, list) else [group]
            if not any(re.search(pattern, text, re.IGNORECASE | re.MULTILINE) for pattern in alternatives):
                missing.append(" | ".join(alternatives))
        details = []
        if not files:
            details.append("no files matched the configured scope")
        details.extend(f"missing expression: {group}" for group in missing)
        if not files:
            return 0, details
        return self._fraction(len(item.get("groups", [])) - len(missing), len(item.get("groups", []))), details

    def _not_contains(self, item: dict) -> tuple[float, list[str]]:
        files = self._files(item.get("files"))
        if item.get("files") and not files:
            return 0, ["no files matched the configured scope"]
        findings = []
        matched_patterns = set()
        for path in files:
            text = path.read_text(encoding="utf-8", errors="ignore")
            for pattern in item.get("patterns", []):
                if re.search(pattern, text, re.IGNORECASE | re.MULTILINE):
                    findings.append(f"{self._display(path)} matches /{pattern}/")
                    matched_patterns.add(pattern)
        patterns = item.get("patterns", [])
        return self._fraction(len(patterns) - len(matched_patterns), len(patterns)), findings

    def _markdown_links_valid(self, item: dict) -> tuple[float, list[str]]:
        files = self._files(item.get("files", ["**/*.md"]))
        if not files:
            return 0, ["no Markdown files were generated"]
        findings = []
        checked = 0
        link_pattern = re.compile(r"(?<!!)\[[^\]]+\]\(([^)]+)\)")
        for path in files:
            text = path.read_text(encoding="utf-8", errors="ignore")
            for target in link_pattern.findall(text):
                target = target.strip().split("#", 1)[0]
                if not target or "://" in target or target.startswith("mailto:"):
                    continue
                checked += 1
                if not (path.parent / target).resolve().exists():
                    findings.append(f"{self._display(path)} -> {target}")
        minimum = int(item.get("minimum", 0))
        if checked < minimum:
            findings.append(f"only {checked} local links found; expected at least {minimum}")
        valid = checked - len([finding for finding in findings if " -> " in finding])
        evidence_ratio = self._fraction(min(checked, minimum), minimum)
        return self._fraction(valid, checked) * evidence_ratio, findings

    def _source_paths_valid(self, item: dict) -> tuple[float, list[str]]:
        files = self._files(item.get("files", ["**/*.md"]))
        if not files:
            return 0, ["no generated references were available for source validation"]
        prefixes = tuple(item.get("prefixes", []))
        suffixes = tuple(item.get("suffixes", []))
        code_span = re.compile(r"`([^`\n]+)`")
        findings = []
        checked: set[str] = set()
        for path in files:
            text = path.read_text(encoding="utf-8", errors="ignore")
            for raw in code_span.findall(text):
                candidate = raw.split(":", 1)[0].strip().lstrip("./")
                if not candidate or (prefixes and not candidate.startswith(prefixes)):
                    continue
                if suffixes and not candidate.endswith(suffixes):
                    continue
                checked.add(candidate)
                if not (self.project / candidate).exists():
                    findings.append(f"{self._display(path)} references missing source {candidate}")
        unique_findings = sorted(set(findings))
        missing_paths = {finding.rsplit(" ", 1)[-1] for finding in unique_findings}
        minimum = int(item.get("minimum", 0))
        if len(checked) < minimum:
            unique_findings.append(f"only {len(checked)} source paths found; expected at least {minimum}")
        evidence_ratio = self._fraction(min(len(checked), minimum), minimum)
        return self._fraction(len(checked) - len(missing_paths), len(checked)) * evidence_ratio, unique_findings

    def _frontmatter_contract(self, item: dict) -> tuple[float, list[str]]:
        files = self._files(item.get("files", ["SKILL.md"]))
        if not files:
            return 0, ["no Skill entry file matched the configured scope"]

        matched = 0
        total = 0
        details = []
        for path in files:
            text = path.read_text(encoding="utf-8", errors="ignore")
            block_match = re.match(r"\A---\s*\n(.*?)\n---(?:\s*\n|\Z)", text, re.DOTALL)
            total += 1
            if not block_match:
                details.append(f"{self._display(path)} has no leading YAML frontmatter")
                continue
            matched += 1

            fields = {}
            for line in block_match.group(1).splitlines():
                key, separator, value = line.partition(":")
                if separator:
                    fields[key.strip()] = value.strip().strip('"\'')

            for field in item.get("required_fields", ["name", "description"]):
                total += 1
                if fields.get(field):
                    matched += 1
                else:
                    details.append(f"{self._display(path)} frontmatter misses {field}")

            name_pattern = item.get("name_pattern", "")
            if name_pattern:
                total += 1
                if re.fullmatch(name_pattern, fields.get("name", "")):
                    matched += 1
                else:
                    details.append(f"{self._display(path)} name does not match /{name_pattern}/")

            description = fields.get("description", "")
            for group in item.get("description_groups", []):
                total += 1
                alternatives = group if isinstance(group, list) else [group]
                if any(re.search(pattern, description, re.IGNORECASE) for pattern in alternatives):
                    matched += 1
                else:
                    details.append(
                        f"{self._display(path)} frontmatter description misses: {' | '.join(alternatives)}"
                    )

            maximum = int(item.get("max_description_length", 0))
            if maximum > 0:
                total += 1
                if 0 < len(description) <= maximum:
                    matched += 1
                else:
                    details.append(
                        f"{self._display(path)} description length is {len(description)}; expected 1-{maximum}"
                    )

        return self._fraction(matched, total), details

    def _entry_contract(self, item: dict) -> tuple[float, list[str]]:
        text = self._read(self._files(item.get("files")))
        entry_patterns = item.get("entries", [])
        fields = item.get("required_fields", [])
        missing = []
        matched = 0
        total = 0
        headings = list(re.finditer(r"(?m)^###\s+(.+?)\s*$", text))
        blocks = []
        for index, match in enumerate(headings):
            end = headings[index + 1].start() if index + 1 < len(headings) else len(text)
            blocks.append((match.group(1), text[match.start():end]))
        for entry in entry_patterns:
            total += 1 + len(fields)
            block = next((body for title, body in blocks if re.search(entry, title, re.IGNORECASE)), "")
            if not block:
                missing.append(f"missing entry: {entry}")
                continue
            matched += 1
            for field in fields:
                if re.search(field, block, re.IGNORECASE | re.MULTILINE):
                    matched += 1
                else:
                    missing.append(f"entry {entry} missing field /{field}/")
        return self._fraction(matched, total), missing

    def _module_relations_valid(self, item: dict) -> tuple[float, list[str]]:
        files = self._files(item.get("files"))
        if not files:
            return 0, ["no module reference was generated"]
        text = self._read(files)
        blocks = re.split(r"(?m)^##\s+", text)[1:]
        modules: dict[str, str] = {}
        for block in blocks:
            title, _, body = block.partition("\n")
            match = re.search(r"(?im)^- \*\*Source path\*\*:\s*`([^`]+)`", body)
            if match:
                modules[title.strip()] = match.group(1).strip()
        known = set(modules) | set(modules.values())
        findings = []
        for block in blocks:
            title, _, body = block.partition("\n")
            source = modules.get(title.strip(), "")
            for label in ("Dependency hints", "Dependent hints"):
                section = re.search(rf"(?ims)^- \*\*{re.escape(label)}\*\*:\s*\n(.*?)(?=^- \*\*|^##|\Z)", body)
                if not section:
                    continue
                for value in re.findall(r"(?m)^\s+-\s+`?([^`\n]+?)`?\s*$", section.group(1)):
                    value = value.strip()
                    if value == source or value == title.strip():
                        findings.append(f"module {title.strip()} has a self relation: {value}")
                    elif value not in known:
                        findings.append(f"module {title.strip()} references unknown module: {value}")
        return (1.0 if not findings else 0.0), findings

    def _max_overlap(self, item: dict) -> tuple[float, list[str]]:
        left_files = self._files(item["left_files"])
        right_files = self._files(item["right_files"])
        if not left_files or not right_files:
            return 0, ["one or both overlap scopes contain no generated files"]
        left = self._read(left_files).lower()
        right = self._read(right_files).lower()
        terms = item.get("terms", [])
        overlap = [term for term in terms if term.lower() in left and term.lower() in right]
        maximum = int(item.get("maximum", 0))
        if len(overlap) <= maximum:
            return 1, []
        excess = len(overlap) - maximum
        return max(0, 1 - excess / max(1, len(terms))), [f"overlap ({len(overlap)} > {maximum}): {', '.join(overlap)}"]

    def _section_contract(self, item: dict) -> tuple[float, list[str]]:
        files = self._files(item.get("files"))
        if not files:
            return 0, ["no files matched the configured scope"]
        anchors = item.get("anchors", [])
        groups = item.get("groups", [])
        best_score = 0.0
        best_missing = []
        best_matched = -1
        anchored = False
        anchored_sections = []
        for path in files:
            text = path.read_text(encoding="utf-8", errors="ignore")
            sections = re.split(r"(?m)(?=^#{2,4}\s+)", text)
            for section in sections:
                anchor_match = any(re.search(pattern, section, re.IGNORECASE | re.MULTILINE) for pattern in anchors)
                if not anchor_match:
                    continue
                anchored = True
                anchored_sections.append(section)
                matched_groups = 0
                missing = []
                for group in groups:
                    alternatives = group if isinstance(group, list) else [group]
                    if any(re.search(pattern, section, re.IGNORECASE | re.MULTILINE) for pattern in alternatives):
                        matched_groups += 1
                    else:
                        missing.append(" | ".join(alternatives))
                score = self._fraction(matched_groups, len(groups))
                if matched_groups > best_matched:
                    best_score = score
                    best_missing = missing
                    best_matched = matched_groups
        if item.get("aggregate_by_anchor") and anchored_sections:
            combined = "\n".join(anchored_sections)
            matched_groups = 0
            missing = []
            for group in groups:
                alternatives = group if isinstance(group, list) else [group]
                if any(re.search(pattern, combined, re.IGNORECASE | re.MULTILINE) for pattern in alternatives):
                    matched_groups += 1
                else:
                    missing.append(" | ".join(alternatives))
            best_score = self._fraction(matched_groups, len(groups))
            best_missing = missing
        details = []
        if best_score < 1:
            if not anchored:
                details.append(f"no knowledge block matched source anchor: {' | '.join(anchors)}")
            else:
                details.extend(f"best matching block misses: {group}" for group in best_missing)
        return best_score, details

    def _route_contract(self, item: dict) -> tuple[float, list[str]]:
        """检查入口是否把焦点、参考页和源码证据连成可执行路由。"""
        files = self._files(item.get("files", ["SKILL.md"]))
        if not files:
            return 0, ["no Skill entry file matched the configured scope"]

        text = self._read(files)
        details = []
        requirements = item.get("requirements", [])
        matched = 0
        for requirement in requirements:
            alternatives = requirement if isinstance(requirement, list) else [requirement]
            if any(re.search(pattern, text, re.IGNORECASE | re.MULTILINE) for pattern in alternatives):
                matched += 1
            else:
                details.append(f"missing route contract: {' | '.join(alternatives)}")

        heading = re.search(r"(?im)^##\s+(Development Focuses|开发焦点)\s*$", text)
        if not heading:
            minimum = int(item.get("minimum_focuses", 0))
            if minimum > 0:
                details.append("no development-focus route table found")
                return self._fraction(matched, len(requirements) + 1), details
            return self._fraction(matched, len(requirements)), details

        table = text[heading.end():]
        table = table.split("\n## ", 1)[0]
        rows = []
        for line in table.splitlines():
            if not line.lstrip().startswith("|") or re.search(r"^\|\s*-", line):
                continue
            cells = [cell.strip() for cell in line.strip().strip("|").split("|")]
            if cells and cells[0].lower() not in {"focus", "焦点"}:
                rows.append(cells)
        minimum = int(item.get("minimum_focuses", 0))
        if len(rows) < minimum:
            details.append(f"only {len(rows)} focus rows found; expected at least {minimum}")
        valid_rows = 0
        entry_file = files[0]
        for index, cells in enumerate(rows, 1):
            if len(cells) < 4:
                details.append(f"focus row {index} has {len(cells)} columns; expected at least 4")
                continue
            links = re.findall(r"\[[^\]]+\]\(([^)]+)\)", cells[2])
            if not links:
                details.append(f"focus row {index} has no reference link")
                continue
            missing = [target for target in links if not (entry_file.parent / target).resolve().exists()]
            if missing:
                details.append(f"focus row {index} references missing files: {', '.join(missing)}")
                continue
            if not re.search(r"`[^`]+`", cells[3]):
                details.append(f"focus row {index} has no source evidence entry")
                continue
            valid_rows += 1

        score = self._fraction(matched, len(requirements))
        if minimum > 0:
            score = (score + self._fraction(valid_rows, max(minimum, len(rows)))) / 2
        return score, details

    def _output_stability(self, item: dict) -> tuple[float, list[str]]:
        run_indexes = item.get("runs", [1, 2])
        if len(run_indexes) < 2:
            return 0, ["output stability requires at least two run indexes"]
        roots = [self.runtime / "snapshots" / f"run-{index}" / "skills" for index in run_indexes]
        missing = [path for path in roots if not path.exists()]
        if missing:
            return 0, [f"missing snapshot: {path}" for path in missing]

        trees = [self._snapshot_hashes(path, item.get("ignore", [])) for path in roots]
        all_paths = set().union(*(tree.keys() for tree in trees))
        stable_paths = {
            path for path in all_paths
            if all(tree.get(path) == trees[0].get(path) for tree in trees[1:])
        }
        details = []
        for run_index, tree in zip(run_indexes[1:], trees[1:]):
            for path in sorted(all_paths):
                if trees[0].get(path) == tree.get(path):
                    continue
                if path not in trees[0]:
                    details.append(f"run {run_index} added {path}")
                elif path not in tree:
                    details.append(f"run {run_index} removed {path}")
                else:
                    details.append(f"run {run_index} changed {path}")
        minimum = int(item.get("minimum", 1))
        evidence_ratio = self._fraction(min(len(all_paths), minimum), minimum)
        return self._fraction(len(stable_paths), len(all_paths)) * evidence_ratio, details

    @staticmethod
    def _snapshot_hashes(root: Path, ignored: list[str]) -> dict[str, str]:
        hashes = {}
        for path in sorted(candidate for candidate in root.rglob("*") if candidate.is_file()):
            relative = path.relative_to(root).as_posix()
            if any(path.match(pattern) or relative == pattern for pattern in ignored):
                continue
            hashes[relative] = hashlib.sha256(path.read_bytes()).hexdigest()
        return hashes

    def _report(self, results: list[CheckResult], run_failed: bool = False) -> dict:
        dimensions = self.config.get("dimensions", {})
        if run_failed:
            return {
                "case": self.case,
                "generated_at": time.strftime("%Y-%m-%dT%H:%M:%S%z"),
                "passed": False,
                "status": "run_failed",
                "total_score": None,
                "raw_score": None,
                "grade": "not_scored",
                "scores": {},
                "score_cap": None,
                "gate_failures": [result.check_id for result in results if result.gate and not result.passed],
                "threshold_failures": [],
                "checks": [result.__dict__ for result in results],
                "failed_checks": len([result for result in results if not result.passed]),
                "deductions": [],
                "artifacts": {
                    "project": str(self.project),
                    "skill_roots": [str(path) for path in self.skill_roots],
                    "report": str(self.report_dir / "latest.md"),
                    "snapshots": str(self.runtime / "snapshots"),
                },
            }
        scores = {}
        for name, settings in dimensions.items():
            selected = [result for result in results if result.dimension == name]
            earned = sum(result.earned_points for result in selected)
            possible = float(settings["points"])
            scores[name] = {
                "earned": round(earned, 2),
                "possible": possible,
                "percent": round(100 * earned / possible, 1),
            }
        raw_score = round(sum(result.earned_points for result in results), 1)
        gates = [result for result in results if result.gate and not result.passed]
        score_cap = min((result.score_cap for result in gates if result.score_cap is not None), default=100.0)
        total = round(min(raw_score, score_cap), 1)
        thresholds = self.config.get("thresholds", {})
        failures = [result for result in results if not result.passed]
        threshold_failures = []
        for name, minimum in thresholds.get("dimensions", {}).items():
            if scores.get(name, {"percent": 0})["percent"] < minimum:
                threshold_failures.append(f"dimension {name}: {scores.get(name, {'percent': 0})['percent']} < {minimum}")
        if total < thresholds.get("total", 0):
            threshold_failures.append(f"total: {total} < {thresholds['total']}")
        passed = not gates and not threshold_failures
        deductions = sorted(
            (
                {
                    "check_id": result.check_id,
                    "dimension": result.dimension,
                    "points_lost": round(result.possible_points - result.earned_points, 2),
                    "hint": result.hint,
                }
                for result in failures
                if result.possible_points > result.earned_points
            ),
            key=lambda item: (-item["points_lost"], item["check_id"]),
        )
        return {
            "case": self.case,
            "generated_at": time.strftime("%Y-%m-%dT%H:%M:%S%z"),
            "passed": passed,
            "status": "passed" if passed else "quality_failed",
            "total_score": total,
            "raw_score": raw_score,
            "grade": self._grade(total),
            "scores": scores,
            "score_cap": score_cap if gates else None,
            "gate_failures": [result.check_id for result in gates],
            "threshold_failures": threshold_failures,
            "checks": [result.__dict__ for result in results],
            "failed_checks": len(failures),
            "deductions": deductions,
            "artifacts": {
                "project": str(self.project),
                "skill_roots": [str(path) for path in self.skill_roots],
                "report": str(self.report_dir / "latest.md"),
                "snapshots": str(self.runtime / "snapshots"),
            },
        }

    @staticmethod
    def _grade(score: float) -> str:
        if score >= 95:
            return "excellent"
        if score >= 85:
            return "good"
        if score >= 70:
            return "qualified"
        if score >= 50:
            return "weak"
        return "unqualified"

    def _write_report(self, report: dict) -> None:
        self.report_dir.mkdir(parents=True, exist_ok=True)
        (self.report_dir / "latest.json").write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
        lines = [
            f"# Quality Report: {report['case']}", "",
            f"- Passed: `{str(report['passed']).lower()}`",
            f"- Status: `{report['status']}`",
            f"- Total score: `{report['total_score'] if report['total_score'] is not None else 'not scored'}`",
            f"- Grade: `{report['grade']}`",
            f"- Failed checks: `{report['failed_checks']}`", "",
        ]
        if report["raw_score"] is not None and report["score_cap"] is not None:
            lines.extend([
                f"- Raw score: `{report['raw_score']}`",
                f"- Gate cap: `{report['score_cap']}`", "",
            ])
        if report["scores"]:
            lines.extend(["## Dimensions", ""])
            for name, score in report["scores"].items():
                lines.append(f"- `{name}`: {score['earned']} / {score['possible']} ({score['percent']}%)")
        lines.extend(["", "## Checks", ""])
        for check in report["checks"]:
            state = "PASS" if check["passed"] else "FAIL"
            gate = ", gate" if check["gate"] else ""
            lines.append(f"### {state}: {check['check_id']} ({check['dimension']}{gate})")
            lines.extend([
                "",
                f"Score: {check['earned_points']:.2f} / {check['possible_points']:.2f}",
                "",
                check["description"],
            ])
            if check["details"]:
                lines.extend(["", "Evidence:", ""])
                lines.extend(f"- {detail}" for detail in check["details"])
            if not check["passed"] and check["hint"]:
                lines.extend(["", f"Optimization hint: {check['hint']}"])
            lines.append("")
        if report["deductions"]:
            lines.extend(["## Largest Deductions", ""])
            for deduction in report["deductions"]:
                lines.append(f"- `{deduction['check_id']}`: -{deduction['points_lost']} ({deduction['dimension']})")
            lines.append("")
        if report["threshold_failures"]:
            lines.extend(["## Threshold Failures", ""])
            lines.extend(f"- {failure}" for failure in report["threshold_failures"])
            lines.append("")
        lines.extend(["## Artifacts", ""])
        lines.extend(f"- `{key}`: `{value}`" for key, value in report["artifacts"].items())
        (self.report_dir / "latest.md").write_text("\n".join(lines) + "\n", encoding="utf-8")


def parser() -> argparse.ArgumentParser:
    result = argparse.ArgumentParser()
    result.add_argument("--root", required=True, type=Path)
    result.add_argument("--config", required=True, type=Path)
    result.add_argument("--project", type=Path, help="existing project root to score; relative paths use the current directory")
    result.add_argument("--skill-root", action="append", type=Path, help="generated Skill root; relative paths use the current directory; repeat for multiple targets")
    commands = result.add_subparsers(dest="command", required=True)
    commands.add_parser("prepare")
    commands.add_parser("run")
    value = commands.add_parser("value")
    value.add_argument("key")
    score = commands.add_parser("score")
    score.add_argument("--command-exit", type=int)
    score.add_argument("--failed-step", default="")
    return result


def main() -> int:
    args = parser().parse_args()
    run = QualityRun(args.root, args.config, args.project, args.skill_root)
    if args.command == "prepare":
        run.prepare()
        print(run.project)
        return 0
    if args.command == "run":
        report = run.run()
        print(json.dumps({"passed": report["passed"], "status": report["status"], "score": report["total_score"], "grade": report["grade"], "report": report["artifacts"]["report"]}, ensure_ascii=False))
        return 0 if report["passed"] else 1
    if args.command == "value":
        run.value(args.key)
        return 0
    report = run.score(args.command_exit, args.failed_step)
    print(json.dumps({"passed": report["passed"], "status": report["status"], "score": report["total_score"], "grade": report["grade"], "report": report["artifacts"]["report"]}, ensure_ascii=False))
    return 0 if report["passed"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
