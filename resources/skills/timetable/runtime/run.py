#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Run the basic timetable workflow with one command."""

from __future__ import annotations

import json
import os
import subprocess
import sys
import tempfile
from pathlib import Path

CORE_REQUIREMENT_TYPES = {
    "pin",
    "forbid_slots",
    "allow_slots",
    "per_day_limit",
    "spread_days",
    "same_day_adjacent",
    "no_k_consecutive",
    "class_slot_used",
}


def fail(message: str, code: int = 3) -> int:
    sys.stderr.write(message.rstrip() + "\n")
    return code


def load_problem(path: Path) -> dict:
    try:
        with path.open("r", encoding="utf-8") as handle:
            problem = json.load(handle)
    except OSError as error:
        raise ValueError(f"Không đọc được problem.json: {error}") from error
    except json.JSONDecodeError as error:
        raise ValueError(f"problem.json không phải JSON hợp lệ: {error}") from error

    if not isinstance(problem, dict):
        raise ValueError("problem.json phải là một đối tượng JSON")
    requirements = problem.get("requirements", [])
    if not isinstance(requirements, list):
        raise ValueError("requirements phải là một danh sách")
    return problem


def check_basic_scope(problem: dict) -> None:
    for index, requirement in enumerate(problem.get("requirements", [])):
        if not isinstance(requirement, dict):
            continue
        requirement_type = requirement.get("type")
        if requirement_type == "resource":
            raise ValueError(
                "Chế độ cơ bản không hỗ trợ yêu cầu về phòng học hoặc tài nguyên. "
                f"Hãy bỏ requirements[{index}] trước khi xếp lịch."
            )
        if requirement_type not in CORE_REQUIREMENT_TYPES:
            raise ValueError(
                f"Chế độ cơ bản không hỗ trợ loại yêu cầu '{requirement_type}' "
                f"tại requirements[{index}]."
            )


def run_command(arguments: list[str]) -> int:
    completed = subprocess.run(arguments, check=False)
    return int(completed.returncode)


def main(argv: list[str] | None = None) -> int:
    args = list(sys.argv[1:] if argv is None else argv)
    if len(args) != 2:
        return fail(
            "Sử dụng: python -X utf8 runtime/run.py problem.json thoi-khoa-bieu.xlsx",
            2,
        )

    problem_path = Path(args[0]).expanduser().resolve()
    output_path = Path(args[1]).expanduser().resolve()
    if output_path.suffix.lower() != ".xlsx":
        return fail("File kết quả phải có đuôi .xlsx")

    try:
        problem = load_problem(problem_path)
        check_basic_scope(problem)
    except ValueError as error:
        return fail(str(error))

    runtime_dir = Path(__file__).resolve().parent
    solver_path = runtime_dir / "solver.py"
    exporter_path = runtime_dir / "exporter.py"
    if not solver_path.is_file() or not exporter_path.is_file():
        return fail("Thiếu solver.py hoặc exporter.py trong thư mục runtime", 1)

    try:
        output_path.parent.mkdir(parents=True, exist_ok=True)

        # Work from a clean temporary directory so a stale constraints_extra.py
        # beside the user's input file can never be loaded accidentally.
        with tempfile.TemporaryDirectory(
            prefix=".timetable-", dir=str(output_path.parent)
        ) as temp_dir_name:
            temp_dir = Path(temp_dir_name)
            clean_problem = temp_dir / "problem.json"
            schedule_path = temp_dir / "schedule.json"
            workbook_path = temp_dir / "thoi-khoa-bieu.xlsx"

            with clean_problem.open("w", encoding="utf-8") as handle:
                json.dump(problem, handle, ensure_ascii=False, indent=2)

            solver_code = run_command(
                [
                    sys.executable,
                    "-X",
                    "utf8",
                    str(solver_path),
                    str(clean_problem),
                    str(schedule_path),
                    "--diagnose",
                ]
            )
            if solver_code != 0:
                return solver_code
            if not schedule_path.is_file():
                return fail("Bộ xếp lịch kết thúc nhưng không tạo schedule.json", 1)

            exporter_code = run_command(
                [
                    sys.executable,
                    "-X",
                    "utf8",
                    str(exporter_path),
                    str(schedule_path),
                    str(workbook_path),
                ]
            )
            if exporter_code != 0:
                return exporter_code
            if not workbook_path.is_file() or workbook_path.stat().st_size == 0:
                return fail("Bộ xuất Excel kết thúc nhưng không tạo file hợp lệ", 1)

            os.replace(workbook_path, output_path)
    except OSError as error:
        return fail(f"Không thể tạo file kết quả: {error}", 1)

    print(f"Đã tạo thời khóa biểu: {output_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
