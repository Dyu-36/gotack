#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""CP-SAT timetable solver with feasibility and optimization phases.

Usage:
    python -X utf8 runtime/solver.py <problem.json> <schedule.json> [options]

Exit codes: 0 success, 2 infeasible/timeout, 3 invalid input,
4 missing OR-Tools, 5 plugin failure, 6 self-check failure.
"""

import argparse
import json
import os
import sys
import threading
import time
import traceback
from collections import defaultdict

RUNTIME_DIR = os.path.dirname(os.path.abspath(__file__))
if RUNTIME_DIR not in sys.path:
    sys.path.insert(0, RUNTIME_DIR)
sys.dont_write_bytecode = True

if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8")
if hasattr(sys.stderr, "reconfigure"):
    sys.stderr.reconfigure(encoding="utf-8")

try:
    from ortools.sat.python import cp_model
except ImportError:
    sys.stderr.write(
        "Lỗi: Thư viện 'ortools' chưa được cài đặt. "
        "Hãy chạy: pip install ortools\n"
    )
    sys.exit(4)

from timetable_model import (  # noqa: E402
    DAY_LABEL,
    EXIT_BAD_INPUT,
    EXIT_INFEASIBLE,
    EXIT_OK,
    EXIT_PLUGIN,
    EXIT_SELFCHECK,
    Frame,
    LESSON_SELECTOR_KEYS,
    die,
    match_slot_selector,
    normalize_selector,
    slot_label,
    validate_problem,
)
from timetable_requirements import (  # noqa: E402
    add_requirement,
    check_requirement,
)


class PluginBuildAPI:
    """Stable API exposed to ``constraints_extra.py`` build callbacks."""

    def __init__(self, builder, literal):
        self._builder = builder
        self._literal = literal

    @property
    def model(self):
        return self._builder.model

    def rows(self, selector=None):
        return self._builder.rows_for(selector or {})

    def slot_keys(self, slot_selector=None):
        return match_slot_selector(self._builder.frame, slot_selector)

    def x(self, row_id, slot):
        return self._builder.x[(row_id, slot)]

    def occ(self, selector, slots):
        rows = self._builder.rows_for(selector or {})
        return sum(
            self._builder.x[(row["id"], slot)]
            for row in rows
            for slot in slots
        )

    def add_hard(self, expression):
        self._builder.model.add(expression)

    def add_under(self, expression):
        self._builder._add(expression, self._literal)

    def add_penalty(self, expression, weight, note=""):
        self._builder.penalties.append((expression, weight, note))

    def new_bool(self, name):
        return self._builder.model.new_bool_var(name)


class PluginCheckAPI:
    """Stable API exposed to plugin schedule-check callbacks."""

    def __init__(self, builder, placed):
        self._builder = builder
        self.placed = placed

    def rows(self, selector=None):
        return self._builder.rows_for(selector or {})

    def slot_keys(self, slot_selector=None):
        return match_slot_selector(self._builder.frame, slot_selector)

    def placed_of(self, selector=None, slots=None):
        row_ids = {
            row["id"] for row in self._builder.rows_for(selector or {})
        }
        slot_set = set(slots) if slots is not None else None
        return [
            item
            for item in self.placed
            if item["row_id"] in row_ids
            and (slot_set is None or item["slot"] in slot_set)
        ]


class Builder:
    def __init__(self, problem):
        self.problem = problem
        self.frame = Frame(problem["frame"]["days"])
        self.rows = []
        for index, assignment in enumerate(problem["assignments"]):
            row = dict(assignment)
            row.setdefault("id", f"a{index}")
            self.rows.append(row)
        self.row_by_id = {row["id"]: row for row in self.rows}
        self.model = cp_model.CpModel()
        self.x = {
            (row["id"], slot): self.model.new_bool_var(
                f"x_{row['id']}_{slot[0]}_{slot[1]}_{slot[2]}"
            )
            for row in self.rows
            for slot in self.frame.slots
        }
        self.assumption_names = {}
        self.assume_mode = False
        self.penalties = []
        self.requirements = []
        self.plugin_requirements = []
        self.resource_labels = {}
        self._occ_cache = {}

    def _add(self, expression, literal):
        constraint = self.model.add(expression)
        if literal is not None:
            return constraint.only_enforce_if(literal)
        return constraint

    def _add_at_most_one(self, literals, literal):
        if literal is not None:
            return self.model.add(sum(literals) <= 1).only_enforce_if(literal)
        return self.model.add_at_most_one(literals)

    def _add_exactly_one(self, literals, literal):
        if literal is not None:
            return self.model.add(sum(literals) == 1).only_enforce_if(literal)
        return self.model.add_exactly_one(literals)

    def _add_bool_or(self, literals, literal):
        constraint = self.model.add_bool_or(literals)
        if literal is not None:
            return constraint.only_enforce_if(literal)
        return constraint

    def rows_for(self, selector):
        if not selector:
            return list(self.rows)
        return [
            row
            for row in self.rows
            if not selector.get("teachers")
            or row["teacher"] in selector["teachers"]
            if not selector.get("subjects")
            or row["subject"] in selector["subjects"]
            if not selector.get("classes")
            or row["class"] in selector["classes"]
        ]

    def occ(self, rows, slot):
        key = tuple(sorted(row["id"] for row in rows)), slot
        if key not in self._occ_cache:
            self._occ_cache[key] = sum(
                self.x[(row["id"], slot)] for row in rows
            )
        return self._occ_cache[key]

    def class_ids(self, rows):
        return sorted({row["class"] for row in rows})

    def teacher_ids(self, rows):
        return sorted({row["teacher"] for row in rows})

    def new_lit(self, requirement_id):
        if not self.assume_mode:
            return None
        literal = self.model.new_bool_var(f"req_{requirement_id}")
        self.assumption_names[literal.index] = requirement_id
        self.model.add_assumption(literal)
        return literal

    def build_core(self):
        for row in self.rows:
            variables = [
                self.x[(row["id"], slot)] for slot in self.frame.slots
            ]
            if int(row["periods"]) == 1:
                self.model.add_exactly_one(variables)
            else:
                self.model.add(sum(variables) == int(row["periods"]))

        for class_id in self.class_ids(self.rows):
            rows = [row for row in self.rows if row["class"] == class_id]
            for slot in self.frame.slots:
                self.model.add_at_most_one(
                    [self.x[(row["id"], slot)] for row in rows]
                )
        for teacher_id in self.teacher_ids(self.rows):
            rows = [row for row in self.rows if row["teacher"] == teacher_id]
            for slot in self.frame.slots:
                self.model.add_at_most_one(
                    [self.x[(row["id"], slot)] for row in rows]
                )

        if self.problem.get("compact", False):
            for class_id in self.class_ids(self.rows):
                rows = [row for row in self.rows if row["class"] == class_id]
                for slots in self.frame.slots_by_day_session.values():
                    ordered = sorted(slots, key=lambda slot: slot[2])
                    for previous, current in zip(ordered, ordered[1:]):
                        self.model.add(
                            self.occ(rows, current) <= self.occ(rows, previous)
                        )

    def add_requirement(self, requirement, index):
        add_requirement(self, requirement, index)

    def _subject_day_balance_terms(self):
        terms = []
        by_class_subject = defaultdict(list)
        for row in self.rows:
            by_class_subject[(row["class"], row["subject"])].append(row)
        for (class_id, subject), rows in by_class_subject.items():
            for day, slots in self.frame.slots_by_day.items():
                total = sum(
                    self.x[(row["id"], slot)]
                    for row in rows
                    for slot in slots
                )
                over = self.model.new_int_var(
                    0, 10, f"ov_{class_id}_{subject}_{day}"
                )
                self.model.add(over >= total - 1)
                terms.append((over, 3))
        return terms

    def _teacher_session_gap_terms(self, teacher_id, rows, key, slots):
        ordered = sorted(slots, key=lambda slot: slot[2])
        if len(ordered) < 3:
            return []
        busy = {slot: self.occ(rows, slot) for slot in ordered}
        before = {}
        after = {}
        for index in range(1, len(ordered)):
            before[index] = self.model.new_bool_var(
                f"bf_{teacher_id}_{key}_{index}"
            )
            self.model.add_max_equality(
                before[index], [busy[slot] for slot in ordered[:index]]
            )
        for index in range(len(ordered) - 1):
            after[index] = self.model.new_bool_var(
                f"af_{teacher_id}_{key}_{index}"
            )
            self.model.add_max_equality(
                after[index], [busy[slot] for slot in ordered[index + 1 :]]
            )

        terms = []
        for index in range(1, len(ordered) - 1):
            slot = ordered[index]
            gap = self.model.new_bool_var(f"gap_{teacher_id}_{key}_{index}")
            self.model.add(gap <= 1 - busy[slot])
            self.model.add(gap <= before[index])
            self.model.add(gap <= after[index])
            self.model.add(
                gap >= before[index] + after[index] - busy[slot] - 1
            )
            terms.append((gap, 2))
        return terms

    def _teacher_gap_terms(self):
        by_teacher = defaultdict(list)
        for row in self.rows:
            by_teacher[row["teacher"]].append(row)
        terms = []
        for teacher_id, rows in by_teacher.items():
            for key, slots in self.frame.slots_by_day_session.items():
                terms.extend(
                    self._teacher_session_gap_terms(teacher_id, rows, key, slots)
                )
        return terms

    def _preference_terms(self):
        terms = []
        for index, preference in enumerate(self.problem.get("preferences") or []):
            selector = normalize_selector(
                preference.get("selector"),
                LESSON_SELECTOR_KEYS,
                f"ưu tiên {preference.get('id', index)}",
            )
            weight = int(preference.get("weight", 1))
            avoid = bool(preference.get("avoid", False))
            chosen = set(
                match_slot_selector(
                    self.frame, preference.get("slot_selector")
                )
            )
            rows = self.rows_for(selector)
            penalized_slots = (
                chosen if avoid else set(self.frame.slots) - chosen
            )
            variables = [
                self.x[(row["id"], slot)]
                for row in rows
                for slot in self.frame.slots
                if slot in penalized_slots
            ]
            if variables:
                terms.append((sum(variables), weight))
        return terms

    def build_objective(self):
        terms = self._subject_day_balance_terms()
        terms.extend(self._teacher_gap_terms())
        terms.extend(self._preference_terms())
        terms.extend((expression, weight) for expression, weight, _ in self.penalties)
        if not terms:
            return None
        return sum(int(weight) * expression for expression, weight in terms)


class EarlyStoppingSolutionCallback(cp_model.CpSolverSolutionCallback):
    """Stop phase B after the objective has stopped improving."""

    def __init__(self, stop_after_no_improve_seconds=5.0):
        super().__init__()
        self._stop_seconds = stop_after_no_improve_seconds
        self._best_objective = None
        self._timer = None
        self._done = False
        self._lock = threading.Lock()

    def _arm_timer(self):
        with self._lock:
            if self._done:
                return
            if self._timer is not None:
                self._timer.cancel()
            self._timer = threading.Timer(self._stop_seconds, self._on_timeout)
            self._timer.daemon = True
            self._timer.start()

    def _on_timeout(self):
        with self._lock:
            if not self._done:
                self.stop_search()

    def on_solution_callback(self):
        objective = self.objective_value
        if self._best_objective is None or objective < self._best_objective - 1e-4:
            self._best_objective = objective
            self._arm_timer()

    def close(self):
        with self._lock:
            self._done = True
            if self._timer is not None:
                self._timer.cancel()
                self._timer = None


def check_core(builder, placed):
    failures = []
    counts = defaultdict(int)
    for item in placed:
        counts[item["row_id"]] += 1
    for row in builder.rows:
        if counts[row["id"]] != int(row["periods"]):
            failures.append(
                f"LÕI-1: số tiết {row['teacher']} - {row['subject']} - "
                f"{row['class']} là {counts[row['id']]} thay vì {row['periods']}"
            )

    class_slots = defaultdict(list)
    teacher_slots = defaultdict(list)
    for item in placed:
        class_slots[(item["class"], item["slot"])].append(item)
        teacher_slots[(item["teacher"], item["slot"])].append(item)
    for (class_id, slot), items in class_slots.items():
        if len(items) > 1:
            failures.append(
                f"LÕI-2: lớp {class_id} bị trùng tại {slot_label(slot)}"
            )
    for (teacher_id, slot), items in teacher_slots.items():
        if len(items) > 1:
            failures.append(
                f"LÕI-2: giáo viên {teacher_id} bị trùng tại {slot_label(slot)}"
            )

    if builder.problem.get("compact", False):
        used_periods = defaultdict(set)
        for item in placed:
            slot = item["slot"]
            used_periods[(item["class"], slot[0], slot[1])].add(slot[2])
        for class_id in builder.class_ids(builder.rows):
            for day, session in builder.frame.slots_by_day_session:
                used = used_periods.get((class_id, day, session), set())
                if used and used != set(range(1, max(used) + 1)):
                    failures.append(
                        f"LÕI-3: lớp {class_id} {DAY_LABEL.get(day, day)}-"
                        f"{session} bị trống tiết giữa buổi"
                    )
    return failures


def load_plugin(builder, problem_path):
    """Load an optional ``constraints_extra.py`` beside the problem file."""
    plugin_path = os.path.join(
        os.path.dirname(os.path.abspath(problem_path)), "constraints_extra.py"
    )
    if not os.path.isfile(plugin_path):
        return
    try:
        import importlib.util

        spec = importlib.util.spec_from_file_location(
            "constraints_extra", plugin_path
        )
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
    except Exception:
        sys.stderr.write(
            f"Lỗi khi nạp plugin {plugin_path}:\n{traceback.format_exc()}\n"
        )
        sys.exit(EXIT_PLUGIN)
    register = getattr(module, "register", None)
    if register is None:
        sys.stderr.write(
            f"Plugin {plugin_path} phải định nghĩa hàm register(reg). "
            "Xem reference/problem-schema.md mục API mở rộng.\n"
        )
        sys.exit(EXIT_PLUGIN)

    registry = []

    def requirement(name, build, check=None):
        registry.append((name, build, check))

    register(requirement=requirement)
    for name, build, check in registry:
        requirement_id = f"PLUGIN:{name}"
        api = PluginBuildAPI(builder, builder.new_lit(requirement_id))
        try:
            build(api)
        except Exception:
            sys.stderr.write(
                f"Lỗi khi dựng ràng buộc plugin '{name}':\n"
                f"{traceback.format_exc()}\n"
            )
            sys.exit(EXIT_PLUGIN)
        builder.requirements.append(
            (requirement_id, name, {"type": "__plugin__", "name": name})
        )
        builder.plugin_requirements.append((name, check))


def run_plugin_checks(builder, placed):
    results = []
    for name, check in builder.plugin_requirements:
        if check is None:
            results.append(
                (
                    f"PLUGIN:{name}",
                    name,
                    None,
                    "đã giải trong mô hình, không có hàm kiểm tra riêng",
                )
            )
            continue
        try:
            output = check(PluginCheckAPI(builder, placed))
        except Exception:
            output = (
                False,
                "hàm kiểm tra plugin lỗi: "
                + traceback.format_exc(limit=1).strip(),
            )
        if isinstance(output, tuple):
            ok = output[0]
            detail = output[1] if len(output) > 1 else ""
        else:
            ok, detail = output, ""
        results.append((f"PLUGIN:{name}", name, bool(ok), detail))
    return results


def read_solution(builder, solver):
    placed = []
    for row in builder.rows:
        for slot in builder.frame.slots:
            if solver.boolean_value(builder.x[(row["id"], slot)]):
                placed.append(
                    {
                        "row_id": row["id"],
                        "teacher": row["teacher"],
                        "class": row["class"],
                        "subject": row["subject"],
                        "slot": slot,
                        "labels": [],
                    }
                )
    for resource_name, usage in builder.resource_labels.items():
        for (row_id, slot), variable in usage.items():
            if not solver.boolean_value(variable):
                continue
            for item in placed:
                if item["row_id"] == row_id and item["slot"] == slot:
                    item["labels"].append(resource_name)
    return placed


def hint_from(builder, solver):
    for variable in builder.x.values():
        builder.model.add_hint(variable, solver.boolean_value(variable))
    for usage in builder.resource_labels.values():
        for variable in usage.values():
            builder.model.add_hint(variable, solver.boolean_value(variable))


def write_schedule(builder, placed, output_path):
    data = {
        "classes": builder.class_ids(builder.rows),
        "days": builder.problem["frame"]["days"],
        "assignments": [
            {
                "day": item["slot"][0],
                "session": item["slot"][1],
                "period": item["slot"][2],
                "class": item["class"],
                "subject": item["subject"],
                "teacher": item["teacher"],
                **({"labels": item["labels"]} if item["labels"] else {}),
            }
            for item in sorted(
                placed,
                key=lambda item: (
                    builder.frame.sort_key(item["slot"]),
                    item["class"],
                ),
            )
        ],
    }
    os.makedirs(os.path.dirname(os.path.abspath(output_path)) or ".", exist_ok=True)
    with open(output_path, "w", encoding="utf-8") as output:
        json.dump(data, output, ensure_ascii=False, indent=2)


class _ArgParser(argparse.ArgumentParser):
    def error(self, message):
        self.print_usage(sys.stderr)
        die(f"Tham số dòng lệnh không hợp lệ: {message}", EXIT_BAD_INPUT)


def _solver_settings(problem, args):
    config = problem.get("solver") or {}
    if args.time_budget is not None and args.time_budget > 0:
        if args.phase_a_only:
            return float(args.time_budget), 0.0, config
        return float(args.time_budget) * 0.7, float(args.time_budget) * 0.3, config

    wanted_a = float(config.get("phase_a_seconds", 20.0))
    wanted_b = float(config.get("phase_b_seconds", 15.0))
    phase_a = min(wanted_a, 60.0)
    phase_b = min(wanted_b, 15.0)
    if wanted_a > phase_a:
        print(
            f"CẢNH BÁO: solver.phase_a_seconds={wanted_a:.0f}s vượt trần cứng "
            f"60s, đã hạ xuống {phase_a:.0f}s. Muốn chạy dài hơn thì dùng "
            "--time-budget (không bị áp trần)."
        )
    if wanted_b > phase_b and not args.phase_a_only:
        print(
            f"CẢNH BÁO: solver.phase_b_seconds={wanted_b:.0f}s vượt trần cứng "
            f"15s, đã hạ xuống {phase_b:.0f}s."
        )
    return max(phase_a, 1.0), phase_b, config


def _build(problem, problem_path, assume_mode):
    builder = Builder(problem)
    builder.assume_mode = assume_mode
    builder.build_core()
    for index, requirement in enumerate(problem.get("requirements") or []):
        try:
            builder.add_requirement(requirement, index)
        except ValueError as error:
            die(f"Lỗi yêu cầu: {error}", EXIT_BAD_INPUT)
    load_plugin(builder, problem_path)
    return builder


def _diagnose(problem, problem_path, phase_a_seconds, workers, seed, verbose):
    builder = _build(problem, problem_path, assume_mode=True)
    solver = cp_model.CpSolver()
    duration = min(phase_a_seconds, 20.0)
    solver.parameters.max_time_in_seconds = duration
    solver.parameters.num_search_workers = workers
    solver.parameters.random_seed = seed
    solver.parameters.log_search_progress = verbose
    status = solver.solve(builder.model)
    conflicts = []
    if status == cp_model.INFEASIBLE:
        conflicts = [
            builder.assumption_names[index]
            for index in solver.sufficient_assumptions_for_infeasibility()
            if index in builder.assumption_names
        ]
    if conflicts:
        conflict_set = set(conflicts)
        ordered = [
            requirement_id
            for requirement_id, _name, _requirement in builder.requirements
            if requirement_id in conflict_set
        ]
        print("CÁC YÊU CẦU MÂU THUẪN NHAU (cần nới bớt một trong các yêu cầu):")
        for requirement_id in ordered or sorted(conflict_set):
            print(f"  - {requirement_id}")
    elif status == cp_model.INFEASIBLE:
        print(
            "Bản thân dữ liệu phân công mâu thuẫn với khung thời gian "
            "(thiếu slot, vượt tải lớp/giáo viên). Hãy rà lại tổng số tiết "
            "so với số slot của khung."
        )
    else:
        print(
            "Bài toán xác định là vô nghiệm nhưng chưa chỉ ra được nhóm yêu cầu "
            f"mâu thuẫn trong {duration:.0f}s chẩn đoán. Hãy nới bớt các yêu "
            "cầu ràng buộc hoặc kiểm tra dữ liệu phân công / số slot khả dụng."
        )


def _report(builder, placed, args, output_path, seconds_a, seconds_b, improved):
    failures = check_core(builder, placed)
    table = []
    for requirement_id, name, requirement in builder.requirements:
        if requirement.get("type") == "__plugin__":
            continue
        requirement_failures = check_requirement(
            builder, requirement_id, requirement, placed
        )
        table.append(
            (
                requirement_id,
                name,
                not requirement_failures,
                "; ".join(requirement_failures[:3]),
            )
        )
        failures.extend(requirement_failures)
    for requirement_id, name, ok, detail in run_plugin_checks(builder, placed):
        table.append((requirement_id, name, ok if ok is not None else None, detail))
        if ok is False:
            failures.append(f"{requirement_id}: {detail}")

    passed = sum(1 for _id, _name, ok, _detail in table if ok is True)
    unavailable = sum(1 for _id, _name, ok, _detail in table if ok is None)
    failed = sum(1 for _id, _name, ok, _detail in table if ok is False)
    print("KẾT QUẢ: THÀNH CÔNG" if not failures else "KẾT QUẢ: LỖI TỰ KIỂM TRA")
    print(
        f"- Phân công: {len(builder.teacher_ids(builder.rows))} giáo viên, "
        f"{len(builder.class_ids(builder.rows))} lớp, {len(placed)} tiết/tuần"
    )
    print(
        f"- Kiểm tra: {passed}/{passed + failed} yêu cầu PASS"
        + (f", {unavailable} không có hàm kiểm tra riêng" if unavailable else "")
        + ("" if not failures else f" — CÒN {len(failures)} LỖI")
    )
    if args.phase_a_only:
        print(f"- Thời gian: pha tìm nghiệm {seconds_a:.1f}s (chỉ chạy Pha A)")
    else:
        suffix = " (đã dùng bản tinh chỉnh)" if improved else " (giữ bản pha 1)"
        print(
            f"- Thời gian: pha tìm nghiệm {seconds_a:.1f}s, "
            f"pha tinh chỉnh {seconds_b:.1f}s{suffix}"
        )
    print(f"- File kết quả: {output_path}")

    rows = table if args.verbose else [row for row in table if row[2] is not True]
    if rows:
        heading = "Bảng kiểm từng yêu cầu" if args.verbose else "Các yêu cầu chưa đạt / cần lưu ý"
        print(f"- {heading}:")
        for requirement_id, name, ok, detail in rows:
            mark = "PASS" if ok is True else ("??? " if ok is None else "FAIL")
            suffix = f" | {detail}" if detail and (args.verbose or ok is not True) else ""
            print(f"    [{mark}] {requirement_id} — {name}{suffix}")
    if failures:
        die("", EXIT_SELFCHECK)


def main():
    parser = _ArgParser(description="Bộ xếp thời khóa biểu trường học (CP-SAT 2 pha).")
    parser.add_argument("problem", help="Đường dẫn file problem.json")
    parser.add_argument("output", help="Đường dẫn file schedule.json")
    parser.add_argument(
        "--phase-a-only",
        action="store_true",
        help="Chỉ chạy Pha A (tìm nghiệm hợp lệ) rồi xuất kết quả ngay.",
    )
    parser.add_argument(
        "--time-budget",
        type=float,
        default=None,
        help="Tổng ngân sách thời gian (giây), tự chia 70%% pha A và 30%% pha B.",
    )
    parser.add_argument(
        "--diagnose",
        action="store_true",
        help="Chạy chẩn đoán nhóm yêu cầu mâu thuẫn khi bài toán vô nghiệm.",
    )
    parser.add_argument(
        "--verbose",
        action="store_true",
        help="In chi tiết tiến trình tìm kiếm và toàn bộ bảng kiểm PASS/FAIL.",
    )
    args = parser.parse_args()

    try:
        with open(args.problem, encoding="utf-8") as problem_file:
            problem = json.load(problem_file)
    except (OSError, json.JSONDecodeError) as error:
        die(f"Không đọc được problem.json: {error}", EXIT_BAD_INPUT)
    errors = validate_problem(problem)
    if errors:
        sys.stderr.write("problem.json không hợp lệ:\n")
        for error in errors:
            sys.stderr.write(f"  - {error}\n")
        sys.exit(EXIT_BAD_INPUT)

    phase_a_seconds, phase_b_seconds, config = _solver_settings(problem, args)
    workers = int(config.get("workers", min(os.cpu_count() or 4, 8)))
    seed = int(config.get("random_seed", 7))
    builder = _build(problem, args.problem, assume_mode=False)

    solver_a = cp_model.CpSolver()
    solver_a.parameters.max_time_in_seconds = phase_a_seconds
    solver_a.parameters.num_search_workers = workers
    solver_a.parameters.random_seed = seed
    solver_a.parameters.stop_after_first_solution = True
    solver_a.parameters.log_search_progress = args.verbose
    started = time.time()
    status_a = solver_a.solve(builder.model)
    seconds_a = time.time() - started
    if status_a == cp_model.INFEASIBLE:
        print("KẾT QUẢ: CHƯA XẾP ĐƯỢC (VÔ NGHIỆM)")
        if args.diagnose:
            _diagnose(
                problem,
                args.problem,
                phase_a_seconds,
                workers,
                seed,
                args.verbose,
            )
        else:
            print(
                "Bài toán không tìm được phương án thỏa mãn toàn bộ ràng buộc.\n"
                "Hãy nới bớt yêu cầu ràng buộc hoặc kiểm tra dữ liệu phân công / "
                "số slot khả dụng.\n(Chạy lại với cờ `--diagnose` để phân tích "
                "chi tiết nhóm yêu cầu mâu thuẫn)."
            )
        die("", EXIT_INFEASIBLE)
    if status_a not in (cp_model.OPTIMAL, cp_model.FEASIBLE):
        print("KẾT QUẢ: CHƯA TÌM ĐƯỢC LỊCH HỢP LỆ TRONG THỜI GIAN CHO PHÉP")
        print(
            f"Pha tìm nghiệm chạy hết {seconds_a:.1f}s mà chưa ra lịch. "
            "Hãy nới bớt yêu cầu ràng buộc hoặc kiểm tra dữ liệu đầu vào."
        )
        die("", EXIT_INFEASIBLE)

    placed = read_solution(builder, solver_a)
    write_schedule(builder, placed, args.output)
    seconds_b = 0.0
    improved = False
    if not args.phase_a_only:
        objective = builder.build_objective()
        if objective is not None:
            hint_from(builder, solver_a)
            builder.model.minimize(objective)
            solver_b = cp_model.CpSolver()
            solver_b.parameters.max_time_in_seconds = phase_b_seconds
            solver_b.parameters.num_search_workers = workers
            solver_b.parameters.random_seed = seed
            solver_b.parameters.relative_gap_limit = 0.05
            solver_b.parameters.log_search_progress = args.verbose
            callback = EarlyStoppingSolutionCallback(5.0)
            started = time.time()
            try:
                status_b = solver_b.solve(builder.model, callback)
            finally:
                callback.close()
            seconds_b = time.time() - started
            if status_b in (cp_model.OPTIMAL, cp_model.FEASIBLE):
                placed = read_solution(builder, solver_b)
                improved = True
                write_schedule(builder, placed, args.output)

    _report(
        builder,
        placed,
        args,
        args.output,
        seconds_a,
        seconds_b,
        improved,
    )
    sys.exit(EXIT_OK)


if __name__ == "__main__":
    main()
