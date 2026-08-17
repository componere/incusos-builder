#!/usr/bin/env python3
"""Fail the build when upstream incus-os internals reach the compiled package closure.

incusos-builder imports the incus-osd API packages for their type declarations
only. This gate is a scoped deny assertion over the transitive build closure of
the root module: no package under the upstream module's `internal/` or `cmd/`
trees may be linked into our binary. It does not assert an absolute package
count, because the closure grows with our own dependencies.

Usage:
  python3 .github/scripts/check_upstream_closure.py
  python3 .github/scripts/check_upstream_closure.py --packages - < packages.txt
  python3 .github/scripts/check_upstream_closure.py --self-test

Exit codes: 0 clean closure, 1 denied package present, 2 the closure could not be
resolved (a `go list` failure, which is a broken tree rather than a policy breach).
"""

from __future__ import annotations

import argparse
import subprocess
import sys
from pathlib import Path

UPSTREAM_MODULE = "github.com/lxc/incus-os/incus-osd"

# Subtrees of the upstream module that must never be linked in. `internal/` is the
# daemon implementation (tailscale, tview, umoci, ...) and `cmd/` its executables;
# both are far outside the type surface we consume.
DENIED_SUBTREES = (
    f"{UPSTREAM_MODULE}/internal",
    f"{UPSTREAM_MODULE}/cmd",
)

# Package pattern whose build closure is inspected. `go list -deps` reports build
# imports only, matching what the shipped binary links.
LIST_PATTERN = "./..."


class ClosureError(RuntimeError):
    """Raised when the package closure cannot be resolved."""


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument(
        "--packages",
        type=Path,
        help="Read a newline-delimited package list instead of running `go list` ('-' reads stdin)",
    )
    parser.add_argument("--go", default="go", help="Go binary to invoke (default: go)")
    parser.add_argument(
        "--root",
        default=Path("."),
        type=Path,
        help="Module root to run `go list` in (default: the current directory)",
    )
    parser.add_argument(
        "--self-test",
        action="store_true",
        help="Exercise the deny rule against a synthetic closure and exit",
    )
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    if args.self_test:
        return self_test()

    try:
        if args.packages is not None:
            packages = read_packages(args.packages)
        else:
            packages = list_closure(go=args.go, root=args.root)
    except ClosureError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2

    offenders = denied_packages(packages)
    if offenders:
        print(
            f"error: {len(offenders)} denied upstream package(s) in the build closure:",
            file=sys.stderr,
        )
        for package in offenders:
            print(f"  {package}", file=sys.stderr)
        print(
            "hint: incus-osd must be imported for API types only",
            file=sys.stderr,
        )
        return 1

    upstream = upstream_packages(packages)
    print(f"upstream closure clean: {len(upstream)} incus-osd package(s), {len(packages)} non-stdlib total")
    for package in upstream:
        print(f"  {package}")
    return 0


def list_closure(*, go: str, root: Path) -> list[str]:
    """Return the non-stdlib build closure of LIST_PATTERN as import paths."""
    command = [go, "list", "-deps", "-f", "{{if not .Standard}}{{.ImportPath}}{{end}}", LIST_PATTERN]
    try:
        result = subprocess.run(
            command,
            cwd=root,
            capture_output=True,
            text=True,
            check=False,
        )
    except OSError as exc:
        raise ClosureError(f"could not run {' '.join(command)}: {exc}") from exc

    if result.returncode != 0:
        raise ClosureError(f"{' '.join(command)} failed with exit {result.returncode}:\n{result.stderr.strip()}")

    return parse_packages(result.stdout)


def read_packages(path: Path) -> list[str]:
    """Return the package list read from PATH, or stdin when PATH is '-'."""
    if str(path) == "-":
        return parse_packages(sys.stdin.read())
    try:
        return parse_packages(path.read_text(encoding="utf-8"))
    except OSError as exc:
        raise ClosureError(f"could not read package list {path}: {exc}") from exc


def parse_packages(raw: str) -> list[str]:
    """Return the non-empty, whitespace-stripped lines of RAW."""
    return [line.strip() for line in raw.splitlines() if line.strip()]


def denied_packages(packages: list[str]) -> list[str]:
    """Return the sorted, deduplicated packages that violate the deny rule."""
    return sorted({package for package in packages if is_denied(package)})


def is_denied(package: str) -> bool:
    """Report whether PACKAGE lives in a denied upstream subtree."""
    return any(package == subtree or package.startswith(f"{subtree}/") for subtree in DENIED_SUBTREES)


def upstream_packages(packages: list[str]) -> list[str]:
    """Return the sorted upstream incus-osd packages present in PACKAGES."""
    return sorted({p for p in packages if p == UPSTREAM_MODULE or p.startswith(f"{UPSTREAM_MODULE}/")})


def self_test() -> int:
    """Check the deny rule against a synthetic closure; return 0 when it holds."""
    allowed = [
        f"{UPSTREAM_MODULE}/api",
        f"{UPSTREAM_MODULE}/api/seed",
        f"{UPSTREAM_MODULE}/api/images",
        f"{UPSTREAM_MODULE}/api/customizer",
        "github.com/lxc/incus/v7/shared/api",
        "go.yaml.in/yaml/v4",
        # Near misses: same module, names that merely start with the denied words.
        f"{UPSTREAM_MODULE}/api/internals",
        f"{UPSTREAM_MODULE}/api/commands",
        # A denied-looking path from a different module.
        "github.com/componere/incusos-builder/internal/build",
    ]
    denied = [
        f"{UPSTREAM_MODULE}/internal",
        f"{UPSTREAM_MODULE}/internal/seed",
        f"{UPSTREAM_MODULE}/cmd/image-customizer",
    ]

    failures: list[str] = []

    unexpected = denied_packages(allowed)
    if unexpected:
        failures.append(f"allowed packages were rejected: {', '.join(unexpected)}")

    missed = sorted(set(denied) - set(denied_packages(allowed + denied)))
    if missed:
        failures.append(f"denied packages were accepted: {', '.join(missed)}")

    if failures:
        for failure in failures:
            print(f"error: self-test: {failure}", file=sys.stderr)
        return 1

    print(f"self-test ok: {len(allowed)} allowed, {len(denied)} denied package(s) classified correctly")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
