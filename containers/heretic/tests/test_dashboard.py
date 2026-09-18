"""Unit tests for the runhug heretic dashboard (dashboard.py).

The app handlers are exercised directly (no live ASGI runner) so the suite
stays hermetic: FastAPI logic + template rendering are both covered.
"""

import importlib
import json
import os
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))


class _Req:
    def __init__(self, token=None, bearer=None):
        self.query_params = {"token": token} if token is not None else {}
        self.headers = {"authorization": f"Bearer {bearer}"} if bearer else {}


class BaseCase(unittest.TestCase):
    @classmethod
    def _load_dashboard(cls, token=None):
        for key in tuple(os.environ):
            if key == "RUNHUH_TOKEN":
                os.environ.pop(key)
        if token:
            os.environ["RUNHUH_TOKEN"] = token
        dash = importlib.import_module("dashboard")
        dash = importlib.reload(dash)
        return dash

    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self._state_dir = Path(self._tmp.name) / "runhug"
        self._state_dir.mkdir()
        self._dash = self._load_dashboard(token=None)
        self._dash.STATE_DIR = self._state_dir
        self._dash.STATUS_PATH = self._state_dir / "status.json"
        self._dash.LOG_PATH = self._state_dir / "training.log"
        self._write(
            {
                "model": "Org/Model",
                "status": "training",
                "trials_done": 1,
                "trials_total": 20,
                "best_refusals": 55,
                "refusals_total": 100,
                "best_kl": 0.0083,
                "error": "",
                "done": False,
            }
        )
        self._dash.LOG_PATH.write_text(
            "[runhug] dashboard started\nRunning trial 1 of 20\n* Refusals: 55 / 100\n"
        )

    def tearDown(self):
        self._tmp.cleanup()
        os.environ.pop("RUNHUH_TOKEN", None)

    def _write(self, data):
        self._dash.STATUS_PATH.write_text(json.dumps(data))


class TestHealth(BaseCase):
    def test_ok(self):
        self.assertEqual(self._dash.health(), {"ok": True})


class TestStatusJson(BaseCase):
    def test_returns_serialized_status(self):
        resp = self._dash.status_json(_Req())
        self.assertEqual(resp.status_code, 200)
        body = json.loads(resp.body)
        self.assertEqual(body["model"], "Org/Model")
        self.assertEqual(body["best_refusals"], 55)

    def test_missing_status_file_yields_empty(self):
        self._dash.STATUS_PATH.unlink()
        resp = self._dash.status_json(_Req())
        self.assertEqual(resp.status_code, 200)
        self.assertEqual(json.loads(resp.body), {})

    def test_corrupt_status_file_yields_empty(self):
        self._dash.STATUS_PATH.write_text("{not json")
        resp = self._dash.status_json(_Req())
        self.assertEqual(json.loads(resp.body), {})


class TestLogs(BaseCase):
    def test_slices_and_offsets(self):
        resp = self._dash.logs(_Req(), offset=0, n=2)
        body = json.loads(resp.body)
        self.assertEqual(body["offset"], 2)
        self.assertEqual(body["total"], 3)
        self.assertFalse(body["done"])
        self.assertEqual(body["lines"], ["[runhug] dashboard started", "Running trial 1 of 20"])

    def test_offset_bounds_clamped(self):
        resp = self._dash.logs(_Req(), offset=100, n=500)
        body = json.loads(resp.body)
        self.assertEqual(body["lines"], [])
        self.assertEqual(body["offset"], 3)
        self.assertEqual(body["total"], 3)

    def test_done_flag_from_status(self):
        self._write(
            {
                "model": "Org/Model",
                "status": "done",
                "trials_done": 20,
                "trials_total": 20,
                "best_refusals": 7,
                "best_kl": 0.0019,
                "error": "",
                "done": True,
            }
        )
        resp = self._dash.logs(_Req(), offset=0)
        self.assertTrue(json.loads(resp.body)["done"])

    def test_missing_log_file(self):
        self._dash.LOG_PATH.unlink()
        resp = self._dash.logs(_Req(), offset=0)
        body = json.loads(resp.body)
        self.assertEqual(body["lines"], [])
        self.assertEqual(body["total"], 0)


class TestIndex(BaseCase):
    def test_renders_dashboard_html(self):
        resp = self._dash.index(_Req())
        self.assertEqual(resp.status_code, 200)
        html = resp.body.decode()
        self.assertIn("<title>heretic", html)
        self.assertIn("Org/Model", html)


class TestAuth(BaseCase):
    def setUp(self):
        super().setUp()
        self._dash = self._load_dashboard(token="sekrit")
        self._dash.STATE_DIR = self._state_dir
        self._dash.STATUS_PATH = self._state_dir / "status.json"
        self._dash.LOG_PATH = self._state_dir / "training.log"

    def test_denies_without_token(self):
        self.assertEqual(self._dash.status_json(_Req()).status_code, 401)
        self.assertEqual(self._dash.logs(_Req(), offset=0).status_code, 401)
        self.assertEqual(self._dash.index(_Req()).status_code, 401)

    def test_query_token_allows(self):
        resp = self._dash.status_json(_Req(token="sekrit"))
        self.assertEqual(resp.status_code, 200)

    def test_bearer_header_allows(self):
        resp = self._dash.logs(_Req(bearer="sekrit"), offset=0)
        self.assertEqual(resp.status_code, 200)

    def test_wrong_token_denied(self):
        self.assertEqual(self._dash.status_json(_Req(token="nope")).status_code, 401)


if __name__ == "__main__":
    unittest.main()