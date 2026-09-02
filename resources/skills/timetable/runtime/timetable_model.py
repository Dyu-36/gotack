"""Shared timetable schema, selectors, and validation."""

import sys
from collections import defaultdict


EXIT_OK = 0
EXIT_INFEASIBLE = 2
EXIT_BAD_INPUT = 3
EXIT_NO_ORTOOLS = 4
EXIT_PLUGIN = 5
EXIT_SELFCHECK = 6

SESSION_ORDER = {"Sáng": 0, "Chiều": 1, "Tối": 2}
SLOT_SELECTOR_KEYS = {
    "days",
    "sessions",
    "periods",
    "from_start",
    "from_end",
    "periods_from",
}
LESSON_SELECTOR_KEYS = {"teachers", "subjects", "classes"}
REQ_COMMON_KEYS = {"id", "name", "type", "selector"}
REQ_TYPE_KEYS = {
    "pin": {"slots", "count"},
    "forbid_slots": {"slot_selector", "slots"},
    "allow_slots": {"slot_selector", "slots"},
    "per_day_limit": {"per", "max", "min", "exactly"},
    "spread_days": {"slot_selector", "min_days", "exactly_days"},
    "same_day_adjacent": set(),
    "shared_days": {"pairs", "min_days"},
    "pair_days_disjoint": {"selector_b"},
    "no_k_consecutive": {"per", "k"},
    "min_total_in_slots": {"slot_selector", "slots", "min"},
    "class_slot_used": {"classes", "slot", "selector"},
    "resource": {
        "resource",
        "per_class_count",
        "capacity",
        "slot_selector",
        "exclude_adjacent",
    },
}
TOP_LEVEL_KEYS = {
    "frame",
    "assignments",
    "requirements",
    "preferences",
    "solver",
    "compact",
}
FRAME_KEYS = {"days"}
DAY_KEYS = {"day", "sessions"}
SESSION_KEYS = {"name", "periods"}
ASSIGNMENT_KEYS = {"id", "teacher", "class", "subject", "periods"}
PREFERENCE_KEYS = {
    "id",
    "name",
    "selector",
    "slot_selector",
    "weight",
    "avoid",
}
SOLVER_KEYS = {"phase_a_seconds", "phase_b_seconds", "workers", "random_seed"}
DAY_LABEL = {
    2: "Thứ 2",
    3: "Thứ 3",
    4: "Thứ 4",
    5: "Thứ 5",
    6: "Thứ 6",
    7: "Thứ 7",
    8: "Chủ nhật",
}


def die(msg, code):
    if msg:
        sys.stderr.write(msg.rstrip() + "\n")
    sys.exit(code)


def slot_label(slot):
    day, session, period = slot
    return f"{DAY_LABEL.get(day, day)}-{session}-tiết {period}"


class Frame:
    """Slots derived from ``frame.days``."""

    def __init__(self, days):
        self.days = days
        self.slots = []
        self.day_sessions = {}
        for day in days:
            self.day_sessions[day["day"]] = [
                session["name"] for session in day["sessions"]
            ]
            for session in day["sessions"]:
                for period in range(1, session["periods"] + 1):
                    self.slots.append((day["day"], session["name"], period))

        self.slots_by_day = defaultdict(list)
        self.slots_by_day_session = defaultdict(list)
        self._session_len = {}
        for slot in self.slots:
            self.slots_by_day[slot[0]].append(slot)
            self.slots_by_day_session[(slot[0], slot[1])].append(slot)
            self._session_len[(slot[0], slot[1])] = max(
                self._session_len.get((slot[0], slot[1]), 0), slot[2]
            )
        self.slot_set = set(self.slots)

    def session_len(self, slot):
        return self._session_len[(slot[0], slot[1])]

    def sort_key(self, slot):
        day, session, period = slot
        return day, SESSION_ORDER.get(session, 5), period

    def neighbors(self, slot):
        day, session, period = slot
        return [
            neighbor
            for delta in (-1, 1)
            if (neighbor := (day, session, period + delta)) in self.slot_set
        ]

    def adjacent(self, first, second):
        return first[1] == second[1] and abs(first[2] - second[2]) == 1


def match_slot_selector(frame, selector, explicit_slots=None):
    """Return slots matching an intersection, or a union of selectors."""
    if explicit_slots is not None:
        slots = []
        for slot in explicit_slots:
            key = (int(slot["day"]), str(slot["session"]), int(slot["period"]))
            if key not in frame.slot_set:
                raise ValueError(f"slot không tồn tại trong khung: {slot}")
            slots.append(key)
        return slots
    if not selector:
        return list(frame.slots)
    if isinstance(selector, list):
        seen = set()
        slots = []
        for sub_selector in selector:
            for slot in match_slot_selector(frame, sub_selector):
                if slot not in seen:
                    seen.add(slot)
                    slots.append(slot)
        return slots

    days = selector.get("days")
    sessions = selector.get("sessions")
    periods = selector.get("periods")
    from_start = selector.get("from_start")
    from_end = selector.get("from_end")
    periods_from = selector.get("periods_from")
    slots = []
    for slot in frame.slots:
        day, session, period = slot
        if days is not None and day not in days:
            continue
        if sessions is not None and session not in sessions:
            continue
        if periods is not None and period not in periods:
            continue
        if from_start is not None and period > from_start:
            continue
        if periods_from is not None and period < periods_from:
            continue
        if from_end is not None and period <= frame.session_len(slot) - from_end:
            continue
        slots.append(slot)
    return slots


def normalize_selector(selector, allowed, what):
    if selector is None:
        return {}
    if not isinstance(selector, dict):
        raise ValueError(f"{what} phải là đối tượng")
    unknown = set(selector) - allowed
    if unknown:
        raise ValueError(f"{what} chứa khóa lạ: {sorted(unknown)}")
    return dict(selector)


def _is_int(value):
    return isinstance(value, int) and not isinstance(value, bool)


REQUIREMENT_VALUE_RULES = {
    "pin": ("slots", bool, "pin cần 'slots'"),
    "no_k_consecutive": ("k", _is_int, "cần 'k' nguyên"),
    "class_slot_used": ("classes", bool, "cần 'classes'"),
    "resource": ("resource", bool, "resource cần 'resource' (tên tài nguyên)"),
}


def _unknown_keys(value, allowed):
    return sorted(set(value) - allowed)


def _validate_session(session, label):
    if not isinstance(session, dict):
        return [f"{label} phải là đối tượng"]
    errors = []
    unknown = _unknown_keys(session, SESSION_KEYS)
    if unknown:
        errors.append(f"{label} chứa khóa lạ: {unknown}")
    if not isinstance(session.get("name"), str):
        errors.append(f"{label}.name phải là chuỗi")
    if not _is_int(session.get("periods")) or session["periods"] < 1:
        errors.append(f"{label}.periods phải là số nguyên ≥ 1")
    return errors


def _validate_day(day, index):
    label = f"frame.days[{index}]"
    if not isinstance(day, dict):
        return [f"{label} phải là đối tượng"]
    errors = []
    unknown = _unknown_keys(day, DAY_KEYS)
    if unknown:
        errors.append(f"{label} chứa khóa lạ: {unknown}")
    if not _is_int(day.get("day")) or not 2 <= day["day"] <= 8:
        errors.append(f"{label}.day phải là số từ 2 đến 8")
    sessions = day.get("sessions")
    if not isinstance(sessions, list) or not sessions:
        errors.append(f"{label}.sessions không được trống")
        return errors
    for session_index, session in enumerate(sessions):
        errors.extend(
            _validate_session(session, f"{label}.sessions[{session_index}]")
        )
    return errors


def _validate_frame(frame):
    if (
        not isinstance(frame, dict)
        or not isinstance(frame.get("days"), list)
        or not frame["days"]
    ):
        return ["thiếu frame.days (khung thời gian)"]
    errors = []
    unknown = _unknown_keys(frame, FRAME_KEYS)
    if unknown:
        errors.append(f"frame chứa khóa lạ: {unknown}")
    for index, day in enumerate(frame["days"]):
        errors.extend(_validate_day(day, index))
    return errors


def _validate_assignments(rows):
    if not isinstance(rows, list) or not rows:
        return ["thiếu assignments (danh sách phân công)"]
    errors = []
    seen_ids = set()
    for index, row in enumerate(rows):
        label = f"assignments[{index}]"
        if not isinstance(row, dict):
            errors.append(f"{label} phải là đối tượng")
            continue
        unknown = _unknown_keys(row, ASSIGNMENT_KEYS)
        if unknown:
            errors.append(f"{label} chứa khóa lạ: {unknown}")
        for field in ("teacher", "class", "subject"):
            if not isinstance(row.get(field), str) or not row[field].strip():
                errors.append(f"{label} thiếu '{field}' hợp lệ")
        if not _is_int(row.get("periods")) or row["periods"] < 1:
            errors.append(f"{label}.periods phải là số nguyên ≥ 1")
        row_id = row.get("id")
        if row_id is None:
            continue
        try:
            if row_id in seen_ids:
                errors.append(f"{label}: id trùng '{row_id}'")
            seen_ids.add(row_id)
        except TypeError:
            errors.append(f"{label}.id phải là giá trị đơn")
    return errors


def _validate_lesson_selector(selector, label):
    if selector is None:
        return []
    if not isinstance(selector, dict):
        return [f"{label} phải là đối tượng"]
    unknown = _unknown_keys(selector, LESSON_SELECTOR_KEYS)
    if unknown:
        return [f"{label} chứa khóa lạ: {unknown}"]
    return []


def _validate_slot_selector(selector, label):
    if selector is None:
        return []
    selectors = selector if isinstance(selector, list) else [selector]
    if not isinstance(selector, (dict, list)) or not all(
        isinstance(item, dict) for item in selectors
    ):
        return [f"{label} phải là đối tượng hoặc danh sách đối tượng"]
    errors = []
    for item in selectors:
        unknown = _unknown_keys(item, SLOT_SELECTOR_KEYS)
        if unknown:
            errors.append(f"{label} chứa khóa lạ: {unknown}")
    return errors


def _validate_requirement(requirement, index):
    label = f"requirements[{index}]"
    if not isinstance(requirement, dict):
        return [f"{label} phải là đối tượng"]
    requirement_id = requirement.get("id", "?")
    context = f"{label} ({requirement_id})"
    requirement_type = requirement.get("type")
    if requirement_type not in REQ_TYPE_KEYS:
        return [
            f"{context}: loại không hỗ trợ '{requirement_type}'. "
            f"Loại hỗ trợ: {sorted(REQ_TYPE_KEYS)}. Nếu yêu cầu không diễn đạt "
            "được bằng các loại này, hãy viết hàm trong constraints_extra.py "
            "(xem reference/problem-schema.md)."
        ]

    errors = []
    unknown = _unknown_keys(
        requirement, REQ_COMMON_KEYS | REQ_TYPE_KEYS[requirement_type]
    )
    if unknown:
        errors.append(f"{context}: khóa lạ {unknown}")
    for selector_key in ("selector", "selector_b", "exclude_adjacent"):
        if selector_key in requirement:
            errors.extend(
                _validate_lesson_selector(
                    requirement.get(selector_key), f"{label}.{selector_key}"
                )
            )
    errors.extend(
        _validate_slot_selector(requirement.get("slot_selector"), f"{label}.slot_selector")
    )

    rule = REQUIREMENT_VALUE_RULES.get(requirement_type)
    if rule is not None:
        field, predicate, message = rule
        if not predicate(requirement.get(field)):
            errors.append(f"{context}: {message}")
    return errors


def _validate_requirements(requirements):
    if requirements is None:
        return []
    if not isinstance(requirements, list):
        return ["requirements phải là danh sách"]
    errors = []
    for index, requirement in enumerate(requirements):
        errors.extend(_validate_requirement(requirement, index))
    return errors


def _validate_preferences(preferences):
    if preferences is None:
        return []
    if not isinstance(preferences, list):
        return ["preferences phải là danh sách"]
    errors = []
    for index, preference in enumerate(preferences):
        label = f"preferences[{index}]"
        if not isinstance(preference, dict):
            errors.append(f"{label} phải là đối tượng")
            continue
        unknown = _unknown_keys(preference, PREFERENCE_KEYS)
        if unknown:
            errors.append(f"{label}: khóa lạ {unknown}")
        errors.extend(
            _validate_lesson_selector(preference.get("selector"), f"{label}.selector")
        )
        errors.extend(
            _validate_slot_selector(
                preference.get("slot_selector"), f"{label}.slot_selector"
            )
        )
    return errors


def _validate_solver(solver):
    if solver is None:
        return []
    if not isinstance(solver, dict):
        return ["solver phải là đối tượng"]
    unknown = _unknown_keys(solver, SOLVER_KEYS)
    if unknown:
        return [f"solver chứa khóa lạ: {unknown}"]
    return []


def validate_problem(problem):
    """Return all schema errors grouped by the owning input section."""

    if not isinstance(problem, dict):
        return ["problem.json phải là một đối tượng JSON"]
    errors = []
    unknown_top = set(problem) - TOP_LEVEL_KEYS
    if unknown_top:
        errors.append(f"khóa lạ ở mức gốc: {sorted(unknown_top)}")
    errors.extend(_validate_frame(problem.get("frame")))
    errors.extend(_validate_assignments(problem.get("assignments")))
    errors.extend(_validate_requirements(problem.get("requirements")))
    errors.extend(_validate_preferences(problem.get("preferences")))
    errors.extend(_validate_solver(problem.get("solver")))
    return errors
