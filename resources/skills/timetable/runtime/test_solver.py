from __future__ import annotations

import importlib.util
import io
import json
import threading
import time
import unittest
from contextlib import redirect_stdout
from pathlib import Path

SOLVER_PATH = Path(__file__).with_name("solver.py")
SPEC = importlib.util.spec_from_file_location("timetable_solver", SOLVER_PATH)
assert SPEC and SPEC.loader
solver = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(solver)


class TimetableSolverTests(unittest.TestCase):
    def base_problem(self) -> dict:
        return {
            "slots": [
                {"day": "T2", "session": "S", "period": 1},
                {"day": "T2", "session": "S", "period": 2},
                {"day": "T3", "session": "S", "period": 1},
                {"day": "T3", "session": "S", "period": 2},
            ],
            "requirements": [
                {"id": "7A-toan", "class": "7A", "subject": "Toan", "teacher": "Minh", "periods": 2},
                {"id": "7A-van", "class": "7A", "subject": "Van", "teacher": "Linh", "periods": 2},
                {"id": "7B-toan", "class": "7B", "subject": "Toan", "teacher": "Minh", "periods": 2},
            ],
            "hard_constraints": [],
            "soft_constraints": [
                {"id": "linh-prefer-t2", "type": "teacher_preferred_days", "description": "Linh uu tien T2", "teacher": "Linh", "days": ["T2"], "weight": 3}
            ],
        }

    def test_optimal_and_post_validation(self) -> None:
        result = solver.solve(self.base_problem(), Path(__file__))
        self.assertEqual("OPTIMAL", result["status"])
        self.assertTrue(result["hard_constraints_satisfied"])
        self.assertEqual([], solver.validate_schedule(result["schedule"], *solver.normalize(self.base_problem())[:2], self.base_problem()["hard_constraints"]))

    def test_feasible_when_search_stops_after_first_incumbent(self) -> None:
        problem = self.base_problem()
        problem["soft_constraints"] = [
            {"id": "minh-prefer-t2", "type": "teacher_preferred_days", "description": "Minh uu tien T2", "teacher": "Minh", "days": ["T2"], "weight": 1},
            {"id": "linh-prefer-t2", "type": "teacher_preferred_days", "description": "Linh uu tien T2", "teacher": "Linh", "days": ["T2"], "weight": 1},
        ]
        result = solver.solve(problem, Path(__file__), stop_after_first_solution=True)
        self.assertEqual("FEASIBLE", result["status"])
        self.assertTrue(result["hard_constraints_satisfied"])

    def test_infeasible_returns_business_conflict(self) -> None:
        problem = self.base_problem()
        problem["hard_constraints"] = [
            {"id": "minh-only-t2", "type": "teacher_allowed_days", "description": "Minh chi day T2", "teacher": "Minh", "days": ["T2"]}
        ]
        result = solver.solve(problem, Path(__file__))
        self.assertEqual("INFEASIBLE", result["status"])
        self.assertTrue(result["hard_conflicts"])
        self.assertNotIn("literal", json.dumps(result, ensure_ascii=False).lower())

    def test_conflicting_soft_constraints_do_not_make_model_infeasible(self) -> None:
        problem = self.base_problem()
        problem["soft_constraints"] = [
            {"id": "prefer-t2", "type": "teacher_preferred_days", "description": "Minh uu tien T2", "teacher": "Minh", "days": ["T2"], "weight": 1},
            {"id": "avoid-t2", "type": "teacher_avoid_days", "description": "Minh tranh T2", "teacher": "Minh", "days": ["T2"], "weight": 1},
        ]
        result = solver.solve(problem, Path(__file__))
        self.assertIn(result["status"], {"OPTIMAL", "FEASIBLE"})
        self.assertTrue(result["hard_constraints_satisfied"])

    def test_heartbeat_is_independent_of_solution_callback(self) -> None:
        old = solver.HEARTBEAT_SECONDS
        solver.HEARTBEAT_SECONDS = 0.03
        hb = solver.Heartbeat(90)
        output = io.StringIO()
        try:
            with redirect_stdout(output):
                thread = threading.Thread(target=hb.run)
                thread.start()
                time.sleep(0.08)
                hb.done.set()
                thread.join(timeout=1)
        finally:
            solver.HEARTBEAT_SECONDS = old
        self.assertIn('"type":"heartbeat"', output.getvalue())
        self.assertIn('"solutions":0', output.getvalue())


if __name__ == "__main__":
    unittest.main()
