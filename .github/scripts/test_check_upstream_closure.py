from __future__ import annotations

import contextlib
import importlib.util
import io
import tempfile
import unittest
from pathlib import Path


SCRIPT_PATH = Path(__file__).with_name("check_upstream_closure.py")
SPEC = importlib.util.spec_from_file_location("check_upstream_closure", SCRIPT_PATH)
assert SPEC is not None
assert SPEC.loader is not None
check_upstream_closure = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(check_upstream_closure)


UPSTREAM = check_upstream_closure.UPSTREAM_MODULE


@contextlib.contextmanager
def captured_output():
    stdout, stderr = io.StringIO(), io.StringIO()
    with contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
        yield stdout, stderr


class DenyRuleTest(unittest.TestCase):
    def test_accepts_upstream_api_packages(self) -> None:
        allowed = [
            UPSTREAM,
            f"{UPSTREAM}/api",
            f"{UPSTREAM}/api/seed",
            f"{UPSTREAM}/api/images",
            f"{UPSTREAM}/api/customizer",
            "github.com/lxc/incus/v7/shared/api",
            "go.yaml.in/yaml/v4",
        ]

        self.assertEqual(check_upstream_closure.denied_packages(allowed), [])

    def test_rejects_upstream_internal_and_cmd_packages(self) -> None:
        packages = [
            f"{UPSTREAM}/api/seed",
            f"{UPSTREAM}/internal",
            f"{UPSTREAM}/internal/seed",
            f"{UPSTREAM}/cmd/image-customizer",
        ]

        self.assertEqual(
            check_upstream_closure.denied_packages(packages),
            [
                f"{UPSTREAM}/cmd/image-customizer",
                f"{UPSTREAM}/internal",
                f"{UPSTREAM}/internal/seed",
            ],
        )

    def test_denies_by_path_segment_not_substring(self) -> None:
        near_misses = [
            f"{UPSTREAM}/api/internals",
            f"{UPSTREAM}/api/commands",
            f"{UPSTREAM}/apiinternal",
            "github.com/componere/incusos-builder/internal/build",
            "github.com/componere/incusos-builder/cmd/incusos-builder",
        ]

        for package in near_misses:
            with self.subTest(package=package):
                self.assertFalse(check_upstream_closure.is_denied(package))


class MainTest(unittest.TestCase):
    def test_clean_package_list_exits_zero(self) -> None:
        listing = f"{UPSTREAM}/api\n{UPSTREAM}/api/seed\ngo.yaml.in/yaml/v4\n"
        path = self.write_listing(listing)

        with captured_output() as (stdout, _):
            code = check_upstream_closure.main(["--packages", str(path)])

        self.assertEqual(code, 0)
        self.assertIn("upstream closure clean: 2 incus-osd package(s)", stdout.getvalue())

    def test_denied_package_exits_one_and_names_offenders(self) -> None:
        listing = f"{UPSTREAM}/api/seed\n{UPSTREAM}/internal/systemd\n"
        path = self.write_listing(listing)

        with captured_output() as (_, stderr):
            code = check_upstream_closure.main(["--packages", str(path)])

        self.assertEqual(code, 1)
        self.assertIn(f"{UPSTREAM}/internal/systemd", stderr.getvalue())

    def test_unresolvable_closure_exits_two(self) -> None:
        with captured_output() as (_, stderr):
            code = check_upstream_closure.main(["--packages", str(self.tmp_path() / "missing.txt")])

        self.assertEqual(code, 2)
        self.assertIn("could not read package list", stderr.getvalue())

    def test_self_test_passes(self) -> None:
        with captured_output() as (stdout, _):
            code = check_upstream_closure.main(["--self-test"])

        self.assertEqual(code, 0)
        self.assertIn("self-test ok", stdout.getvalue())

    def tmp_path(self) -> Path:
        if not hasattr(self, "_tmp"):
            self._tmp = tempfile.TemporaryDirectory()
            self.addCleanup(self._tmp.cleanup)
        return Path(self._tmp.name)

    def write_listing(self, content: str) -> Path:
        path = self.tmp_path() / "packages.txt"
        path.write_text(content, encoding="utf-8")
        return path


if __name__ == "__main__":
    unittest.main()
