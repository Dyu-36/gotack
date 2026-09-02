"""Built-in timetable requirement builders and self-checks.

Each requirement type owns its CP-SAT builder and schedule verifier in one
registry entry. Adding a type therefore cannot update one dispatch tree while
silently forgetting the other.
"""

from collections import defaultdict

from timetable_model import (
    DAY_LABEL,
    LESSON_SELECTOR_KEYS,
    REQ_TYPE_KEYS,
    match_slot_selector,
    normalize_selector,
    slot_label,
)


class BuildContext:
    def __init__(self, builder, requirement, index):
        self.builder = builder
        self.requirement = requirement
        self.requirement_id = requirement.get("id") or f"Y{index}"
        self.name = requirement.get("name") or self.requirement_id
        selector = normalize_selector(
            requirement.get("selector"),
            LESSON_SELECTOR_KEYS,
            f"yêu cầu {self.requirement_id}.selector",
        )
        self.rows = builder.rows_for(selector)
        self.literal = builder.new_lit(self.requirement_id)

    @property
    def model(self):
        return self.builder.model

    @property
    def frame(self):
        return self.builder.frame

    def slots(self):
        explicit = self.requirement.get("slots")
        if explicit is not None:
            return match_slot_selector(self.frame, None, explicit)
        return match_slot_selector(
            self.frame, self.requirement.get("slot_selector")
        )


class CheckContext:
    def __init__(self, builder, requirement_id, requirement, placed):
        self.builder = builder
        self.requirement_id = requirement_id
        self.requirement = requirement
        self.placed = placed
        self.rows = builder.rows_for(requirement.get("selector") or {})
        row_ids = {row["id"] for row in self.rows}
        self.mine = [item for item in placed if item["row_id"] in row_ids]

    @property
    def frame(self):
        return self.builder.frame

    def slots(self):
        explicit = self.requirement.get("slots")
        if explicit is not None:
            return set(match_slot_selector(self.frame, None, explicit))
        return set(
            match_slot_selector(
                self.frame, self.requirement.get("slot_selector")
            )
        )


def _build_pin(context):
    count = int(context.requirement.get("count", 1))
    slots = context.slots()
    for row in context.rows:
        variables = [
            context.builder.x[(row["id"], slot)] for slot in slots
        ]
        if count == 1 and len(variables) == 1:
            context.builder._add(variables[0] == 1, context.literal)
        elif count == 1 and len(variables) > 1:
            context.builder._add_exactly_one(variables, context.literal)
        else:
            context.builder._add(sum(variables) == count, context.literal)


def _check_pin(context):
    failures = []
    count = int(context.requirement.get("count", 1))
    slots = context.slots()
    for row in context.rows:
        actual = sum(
            1
            for item in context.placed
            if item["row_id"] == row["id"] and item["slot"] in slots
        )
        if actual != count:
            failures.append(
                f"{context.requirement_id}: {row['subject']}-{row['class']} "
                f"ghim {actual}/{count} tiết đúng chỗ"
            )
    return failures


def _build_slot_filter(context):
    slots = context.slots()
    if context.requirement["type"] == "allow_slots":
        allowed = set(slots)
        slots = [slot for slot in context.frame.slots if slot not in allowed]
    for row in context.rows:
        for slot in slots:
            context.builder._add(
                context.builder.x[(row["id"], slot)] == 0, context.literal
            )


def _check_slot_filter(context):
    failures = []
    if context.requirement["type"] == "forbid_slots":
        bad_slots = context.slots()
    else:
        bad_slots = set(context.frame.slots) - context.slots()
    for item in context.mine:
        if item["slot"] in bad_slots:
            failures.append(
                f"{context.requirement_id}: {item['subject']}-{item['class']} "
                "rơi vào slot không được phép "
                f"({slot_label(item['slot'])})"
            )
    return failures


def _group_key(per, item):
    if per == "class":
        return item["class"]
    if per == "teacher":
        return item["teacher"]
    raise ValueError("per phải là 'class' hoặc 'teacher'")


def _build_per_day_limit(context):
    requirement = context.requirement
    per = requirement.get("per", "class")
    try:
        groups = sorted({_group_key(per, row) for row in context.rows})
    except ValueError as error:
        raise ValueError(f"yêu cầu {context.requirement_id}: {error}") from error
    for group in groups:
        rows = [row for row in context.rows if _group_key(per, row) == group]
        for day in context.frame.day_sessions:
            variables = [
                context.builder.x[(row["id"], slot)]
                for row in rows
                for slot in context.frame.slots_by_day[day]
            ]
            if requirement.get("max") is not None:
                maximum = int(requirement["max"])
                if maximum == 1:
                    context.builder._add_at_most_one(variables, context.literal)
                else:
                    context.builder._add(sum(variables) <= maximum, context.literal)
            if requirement.get("min") is not None:
                context.builder._add(
                    sum(variables) >= int(requirement["min"]), context.literal
                )
            if requirement.get("exactly") is not None:
                exact = int(requirement["exactly"])
                if exact == 1:
                    context.builder._add_exactly_one(variables, context.literal)
                else:
                    context.builder._add(sum(variables) == exact, context.literal)


def _check_per_day_limit(context):
    failures = []
    requirement = context.requirement
    per = requirement.get("per", "class")
    counts = defaultdict(int)
    for item in context.mine:
        counts[(_group_key(per, item), item["slot"][0])] += 1
    for (group, day), count in sorted(counts.items()):
        if requirement.get("max") is not None and count > int(requirement["max"]):
            failures.append(
                f"{context.requirement_id}: {group} có {count} tiết/ngày "
                f"{DAY_LABEL.get(day, day)} (tối đa {requirement['max']})"
            )
        if requirement.get("min") is not None and count < int(requirement["min"]):
            failures.append(
                f"{context.requirement_id}: {group} chỉ {count} tiết/ngày "
                f"{DAY_LABEL.get(day, day)} (tối thiểu {requirement['min']})"
            )
        if requirement.get("exactly") is not None and count != int(
            requirement["exactly"]
        ):
            failures.append(
                f"{context.requirement_id}: {group} có {count} tiết/ngày "
                f"{DAY_LABEL.get(day, day)} (yêu cầu đúng {requirement['exactly']})"
            )
    return failures


def _build_spread_days(context):
    requirement = context.requirement
    counting = set(context.slots())
    for class_id in context.builder.class_ids(context.rows):
        rows = [row for row in context.rows if row["class"] == class_id]
        day_used = []
        for day in context.frame.day_sessions:
            slots = [
                slot
                for slot in context.frame.slots_by_day[day]
                if slot in counting
            ]
            terms = [
                context.builder.x[(row["id"], slot)]
                for row in rows
                for slot in slots
            ]
            if not terms:
                continue
            used = context.model.new_bool_var(
                f"du_{context.requirement_id}_{class_id}_{day}"
            )
            context.model.add_max_equality(used, terms)
            day_used.append(used)
        total = sum(day_used)
        if requirement.get("min_days") is not None:
            context.builder._add(
                total >= int(requirement["min_days"]), context.literal
            )
        if requirement.get("exactly_days") is not None:
            context.builder._add(
                total == int(requirement["exactly_days"]), context.literal
            )


def _check_spread_days(context):
    failures = []
    requirement = context.requirement
    counting = context.slots()
    for class_id in context.builder.class_ids(context.rows):
        days = {
            item["slot"][0]
            for item in context.mine
            if item["class"] == class_id and item["slot"] in counting
        }
        if requirement.get("min_days") is not None and len(days) < int(
            requirement["min_days"]
        ):
            failures.append(
                f"{context.requirement_id}: {class_id} chỉ trải {len(days)} ngày "
                f"(tối thiểu {requirement['min_days']})"
            )
        if requirement.get("exactly_days") is not None and len(days) != int(
            requirement["exactly_days"]
        ):
            failures.append(
                f"{context.requirement_id}: {class_id} trải {len(days)} ngày "
                f"(yêu cầu đúng {requirement['exactly_days']})"
            )
    return failures


def _build_same_day_adjacent(context):
    for class_id in context.builder.class_ids(context.rows):
        rows = [row for row in context.rows if row["class"] == class_id]
        for day_slots in context.frame.slots_by_day.values():
            for first_index, first in enumerate(day_slots):
                for second in day_slots[first_index + 1 :]:
                    if context.frame.adjacent(first, second):
                        continue
                    variables = [
                        context.builder.x[(row["id"], first)] for row in rows
                    ] + [context.builder.x[(row["id"], second)] for row in rows]
                    context.builder._add_at_most_one(variables, context.literal)


def _check_same_day_adjacent(context):
    failures = []
    by_class_day = defaultdict(list)
    for item in context.mine:
        by_class_day[(item["class"], item["slot"][0])].append(item["slot"])
    for (class_id, day), slots in by_class_day.items():
        for first_index, first in enumerate(slots):
            for second in slots[first_index + 1 :]:
                if not context.frame.adjacent(first, second):
                    failures.append(
                        f"{context.requirement_id}: {class_id} "
                        f"{DAY_LABEL.get(day, day)} có 2 tiết không liền kề cùng buổi"
                    )
    return failures


def _build_shared_days(context):
    minimum = int(context.requirement.get("min_days", 1))
    for first_class, second_class in context.requirement.get("pairs") or []:
        if first_class == second_class:
            continue
        shared = []
        for day in context.frame.day_sessions:
            slots = context.frame.slots_by_day[day]
            first_terms = [
                context.builder.x[(row["id"], slot)]
                for row in context.rows
                if row["class"] == first_class
                for slot in slots
            ]
            second_terms = [
                context.builder.x[(row["id"], slot)]
                for row in context.rows
                if row["class"] == second_class
                for slot in slots
            ]
            if not first_terms or not second_terms:
                continue
            first_used = context.model.new_bool_var(
                f"sh_{context.requirement_id}_{first_class}_{day}"
            )
            second_used = context.model.new_bool_var(
                f"sh_{context.requirement_id}_{second_class}_{day}"
            )
            context.model.add_max_equality(first_used, first_terms)
            context.model.add_max_equality(second_used, second_terms)
            both = context.model.new_bool_var(
                f"both_{context.requirement_id}_{first_class}_{second_class}_{day}"
            )
            context.model.add_bool_or([first_used.Not(), second_used.Not(), both])
            context.model.add_implication(both, first_used)
            context.model.add_implication(both, second_used)
            shared.append(both)
        context.builder._add(sum(shared) >= minimum, context.literal)


def _check_shared_days(context):
    failures = []
    minimum = int(context.requirement.get("min_days", 1))
    for first_class, second_class in context.requirement.get("pairs") or []:
        first_days = {
            item["slot"][0]
            for item in context.mine
            if item["class"] == first_class
        }
        second_days = {
            item["slot"][0]
            for item in context.mine
            if item["class"] == second_class
        }
        shared = first_days & second_days
        if len(shared) < minimum:
            failures.append(
                f"{context.requirement_id}: {first_class}/{second_class} chỉ trùng "
                f"{len(shared)} ngày (tối thiểu {minimum})"
            )
    return failures


def _build_pair_days_disjoint(context):
    selector = normalize_selector(
        context.requirement.get("selector_b"),
        LESSON_SELECTOR_KEYS,
        f"yêu cầu {context.requirement_id}.selector_b",
    )
    other_rows = context.builder.rows_for(selector)
    shared_classes = sorted(
        set(context.builder.class_ids(context.rows))
        & set(context.builder.class_ids(other_rows))
    )
    for class_id in shared_classes:
        first_rows = [row for row in context.rows if row["class"] == class_id]
        second_rows = [row for row in other_rows if row["class"] == class_id]
        for day, slots in context.frame.slots_by_day.items():
            first_pair = context.model.new_bool_var(
                f"pda_{context.requirement_id}_{class_id}_{day}"
            )
            second_pair = context.model.new_bool_var(
                f"pdb_{context.requirement_id}_{class_id}_{day}"
            )
            first_total = sum(
                context.builder.x[(row["id"], slot)]
                for row in first_rows
                for slot in slots
            )
            second_total = sum(
                context.builder.x[(row["id"], slot)]
                for row in second_rows
                for slot in slots
            )
            context.model.add(first_total <= 1).only_enforce_if(first_pair.Not())
            context.model.add(first_total >= 2).only_enforce_if(first_pair)
            context.model.add(second_total <= 1).only_enforce_if(second_pair.Not())
            context.model.add(second_total >= 2).only_enforce_if(second_pair)
            context.builder._add_bool_or(
                [first_pair.Not(), second_pair.Not()], context.literal
            )


def _check_pair_days_disjoint(context):
    failures = []
    other_rows = context.builder.rows_for(
        context.requirement.get("selector_b") or {}
    )
    other_ids = {row["id"] for row in other_rows}
    first_counts = defaultdict(int)
    second_counts = defaultdict(int)
    for item in context.mine:
        first_counts[(item["class"], item["slot"][0])] += 1
    for item in context.placed:
        if item["row_id"] in other_ids:
            second_counts[(item["class"], item["slot"][0])] += 1
    for key in sorted(set(first_counts) & set(second_counts)):
        if first_counts[key] >= 2 and second_counts[key] >= 2:
            failures.append(
                f"{context.requirement_id}: {key[0]} ngày "
                f"{DAY_LABEL.get(key[1], key[1])} có tiết đôi của cả hai nhóm"
            )
    return failures


def _build_no_k_consecutive(context):
    per = context.requirement.get("per", "class")
    consecutive = int(context.requirement["k"])
    try:
        groups = sorted({_group_key(per, row) for row in context.rows})
    except ValueError as error:
        raise ValueError(f"yêu cầu {context.requirement_id}: {error}") from error
    for group in groups:
        rows = [row for row in context.rows if _group_key(per, row) == group]
        for slots in context.frame.slots_by_day_session.values():
            ordered = sorted(slots, key=lambda slot: slot[2])
            for start in range(len(ordered) - consecutive + 1):
                window = ordered[start : start + consecutive]
                variables = [
                    context.builder.x[(row["id"], slot)]
                    for row in rows
                    for slot in window
                ]
                context.builder._add(
                    sum(variables) <= consecutive - 1, context.literal
                )


def _check_no_k_consecutive(context):
    failures = []
    per = context.requirement.get("per", "class")
    consecutive = int(context.requirement["k"])
    by_group = defaultdict(set)
    for item in context.mine:
        by_group[
            (_group_key(per, item), item["slot"][0], item["slot"][1])
        ].add(item["slot"][2])
    for (group, day, session), periods in sorted(by_group.items()):
        run = 1
        ordered = sorted(periods)
        for first, second in zip(ordered, ordered[1:]):
            if second == first + 1:
                run += 1
                if run >= consecutive:
                    failures.append(
                        f"{context.requirement_id}: {group} dạy {consecutive} tiết liền "
                        f"tại {DAY_LABEL.get(day, day)}-{session}"
                    )
                    run = 1
            else:
                run = 1
    return failures


def _build_min_total_in_slots(context):
    total = sum(
        context.builder.x[(row["id"], slot)]
        for row in context.rows
        for slot in context.slots()
    )
    context.builder._add(
        total >= int(context.requirement["min"]), context.literal
    )


def _check_min_total_in_slots(context):
    actual = sum(1 for item in context.mine if item["slot"] in context.slots())
    minimum = int(context.requirement["min"])
    if actual >= minimum:
        return []
    return [
        f"{context.requirement_id}: chỉ có {actual} tiết trong cửa sổ yêu cầu "
        f"(tối thiểu {minimum})"
    ]


def _class_slot(context):
    slot = context.requirement["slot"]
    return int(slot["day"]), str(slot["session"]), int(slot["period"])


def _build_class_slot_used(context):
    slot = _class_slot(context)
    if slot not in context.frame.slot_set:
        raise ValueError(
            f"yêu cầu {context.requirement_id}: slot không tồn tại: "
            f"{context.requirement['slot']}"
        )
    allowed_ids = {row["id"] for row in context.rows}
    for class_id in context.requirement["classes"]:
        rows = [
            row
            for row in context.builder.rows
            if row["class"] == class_id and row["id"] in allowed_ids
        ]
        if not rows:
            raise ValueError(
                f"yêu cầu {context.requirement_id}: lớp {class_id} không có "
                "phân công nào khớp selector"
            )
        variables = [context.builder.x[(row["id"], slot)] for row in rows]
        if len(variables) == 1:
            context.builder._add(variables[0] == 1, context.literal)
        else:
            context.builder._add_exactly_one(variables, context.literal)


def _check_class_slot_used(context):
    failures = []
    slot = _class_slot(context)
    allowed_ids = {row["id"] for row in context.rows}
    for class_id in context.requirement["classes"]:
        actual = sum(
            1
            for item in context.placed
            if item["class"] == class_id
            and item["slot"] == slot
            and item["row_id"] in allowed_ids
        )
        if actual != 1:
            failures.append(
                f"{context.requirement_id}: lớp {class_id} có {actual} tiết "
                f"(yêu cầu 1) tại {slot_label(slot)}"
            )
    return failures


def _build_resource(context):
    requirement = context.requirement
    resource_name = requirement["resource"]
    allowed = set(context.slots())
    if not allowed:
        raise ValueError(
            f"yêu cầu {context.requirement_id}: cửa sổ tài nguyên rỗng"
        )
    capacity = int(requirement.get("capacity", 1))
    usage = {}
    per_slot = defaultdict(list)
    for row in context.rows:
        for slot in context.frame.slots:
            if slot not in allowed:
                continue
            variable = context.model.new_bool_var(
                f"res_{resource_name}_{row['id']}_{slot[0]}_{slot[1]}_{slot[2]}"
            )
            usage[(row["id"], slot)] = variable
            context.builder._add(
                variable <= context.builder.x[(row["id"], slot)],
                context.literal,
            )
            per_slot[slot].append(variable)

    for class_id in context.builder.class_ids(context.rows):
        rows = [row for row in context.rows if row["class"] == class_id]
        if requirement.get("per_class_count") is not None:
            expected = int(requirement["per_class_count"])
        else:
            expected = sum(int(row["periods"]) for row in rows)
        variables = [
            usage[(row["id"], slot)]
            for row in rows
            for slot in context.frame.slots
            if (row["id"], slot) in usage
        ]
        if not variables:
            raise ValueError(
                f"yêu cầu {context.requirement_id}: lớp {class_id} không có slot "
                "nào trong cửa sổ tài nguyên"
            )
        if expected == 1:
            context.builder._add_exactly_one(variables, context.literal)
        else:
            context.builder._add(sum(variables) == expected, context.literal)
    for variables in per_slot.values():
        if capacity == 1:
            context.builder._add_at_most_one(variables, context.literal)
        else:
            context.builder._add(sum(variables) <= capacity, context.literal)

    excluded = requirement.get("exclude_adjacent")
    if excluded:
        selector = normalize_selector(
            excluded,
            LESSON_SELECTOR_KEYS,
            f"yêu cầu {context.requirement_id}.exclude_adjacent",
        )
        excluded_rows = context.builder.rows_for(selector)
        for (row_id, slot), variable in usage.items():
            row = context.builder.row_by_id[row_id]
            matching = [
                candidate
                for candidate in excluded_rows
                if candidate["class"] == row["class"]
            ]
            for neighbor in context.frame.neighbors(slot):
                if matching:
                    context.builder._add_at_most_one(
                        [variable]
                        + [
                            context.builder.x[(candidate["id"], neighbor)]
                            for candidate in matching
                        ],
                        context.literal,
                    )
    context.builder.resource_labels[resource_name] = usage


def _check_resource(context):
    failures = []
    requirement = context.requirement
    resource_name = requirement["resource"]
    allowed = context.slots()
    resource_items = [
        item for item in context.mine if resource_name in item["labels"]
    ]
    per_slot = defaultdict(int)
    for item in resource_items:
        per_slot[item["slot"]] += 1
        if item["slot"] not in allowed:
            failures.append(
                f"{context.requirement_id}: tiết thực hành {item['class']} ngoài "
                f"cửa sổ ({slot_label(item['slot'])})"
            )
    capacity = int(requirement.get("capacity", 1))
    for slot, count in per_slot.items():
        if count > capacity:
            failures.append(
                f"{context.requirement_id}: {count} lớp cùng dùng tài nguyên "
                f"tại {slot_label(slot)}"
            )
    for class_id in context.builder.class_ids(context.rows):
        if requirement.get("per_class_count") is not None:
            expected = int(requirement["per_class_count"])
        else:
            expected = sum(
                int(row["periods"])
                for row in context.rows
                if row["class"] == class_id
            )
        actual = sum(
            1 for item in resource_items if item["class"] == class_id
        )
        if actual != expected:
            failures.append(
                f"{context.requirement_id}: {class_id} có {actual}/{expected} tiết "
                f"dùng tài nguyên {resource_name}"
            )

    excluded = requirement.get("exclude_adjacent")
    if excluded:
        excluded_ids = {
            row["id"] for row in context.builder.rows_for(excluded)
        }
        for item in resource_items:
            for neighbor in context.frame.neighbors(item["slot"]):
                for other in context.placed:
                    if (
                        other["row_id"] in excluded_ids
                        and other["slot"] == neighbor
                        and other["class"] == item["class"]
                    ):
                        failures.append(
                            f"{context.requirement_id}: tiết thực hành "
                            f"{item['class']} kề {other['subject']} tại "
                            f"{slot_label(neighbor)}"
                        )
    return failures


REQUIREMENT_HANDLERS = {
    "pin": (_build_pin, _check_pin),
    "forbid_slots": (_build_slot_filter, _check_slot_filter),
    "allow_slots": (_build_slot_filter, _check_slot_filter),
    "per_day_limit": (_build_per_day_limit, _check_per_day_limit),
    "spread_days": (_build_spread_days, _check_spread_days),
    "same_day_adjacent": (_build_same_day_adjacent, _check_same_day_adjacent),
    "shared_days": (_build_shared_days, _check_shared_days),
    "pair_days_disjoint": (_build_pair_days_disjoint, _check_pair_days_disjoint),
    "no_k_consecutive": (_build_no_k_consecutive, _check_no_k_consecutive),
    "min_total_in_slots": (_build_min_total_in_slots, _check_min_total_in_slots),
    "class_slot_used": (_build_class_slot_used, _check_class_slot_used),
    "resource": (_build_resource, _check_resource),
}

if set(REQUIREMENT_HANDLERS) != set(REQ_TYPE_KEYS):
    raise RuntimeError("requirement schema and handlers are out of sync")


def add_requirement(builder, requirement, index):
    context = BuildContext(builder, requirement, index)
    build, _ = REQUIREMENT_HANDLERS[requirement["type"]]
    build(context)
    builder.requirements.append(
        (context.requirement_id, context.name, requirement)
    )


def check_requirement(builder, requirement_id, requirement, placed):
    context = CheckContext(builder, requirement_id, requirement, placed)
    _, check = REQUIREMENT_HANDLERS[requirement["type"]]
    return check(context)
