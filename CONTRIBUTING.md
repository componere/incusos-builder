# Contributing

Thanks for contributing to incusos-builder.

Report vulnerabilities through [SECURITY.md](SECURITY.md). Do not use public issues, pull requests, or other public channels for security reports.

## Questions and bugs

Use [GitHub issues](https://github.com/componere/incusos-builder/issues) for questions, non-security bugs, and feature proposals. For a large change, open an issue before writing the patch.

A useful bug report includes:

- version, commit, or how you built the binary
- steps to reproduce
- expected behavior
- actual behavior
- logs or a minimal seed config when they help

## Pull requests

Keep each pull request focused on one problem.

1. Add or update tests when behavior changes.
2. Update `docs/` in the same pull request when user-facing behavior changes.
3. Use a Conventional Commit subject, such as `feat: add config loader` or `fix: handle empty input`.
4. Run `moon run root:check` before requesting review.

GitHub squash-merges pull requests. The pull request title becomes the commit on the default branch, so make that title the final Conventional Commit subject.

This repository does not require a CLA or a `Signed-off-by` trailer.

## Local setup

Install [mise](https://mise.jdx.dev/), then provision the pinned toolchain and run the project gate:

```sh
mise install
moon run root:check
```

`mise install` reads `mise.toml` and `mise.lock` and fails closed if a tool has no locked URL for the current platform. moon runs those binaries from `PATH`; it does not manage language toolchains.

Useful commands:

```sh
moon run root:format
moon run root:lint
moon run root:build
moon run root:test
moon run root:check-upstream   # incus-osd must stay a type-only dependency
moon run root:mocks            # regenerate the mockery mocks in .mockery.yml
moon run docs:build
moon run docs:serve
go run ./cmd/incusos-builder --version
```

`moon run root:check` is the full local gate: format, lint, build, test, the upstream-closure check, and the docs build. CI runs `moon ci`.

`moon run root:e2e` is opt-in. It hits the live update server and is not part of `root:check`.

## Commits and releases

[Release Please](https://github.com/googleapis/release-please) reads Conventional Commit subjects to open release pull requests and update `CHANGELOG.md`. Use `feat` and `fix` only for user-visible changes. Use `docs`, `chore`, `ci`, or `test` for work that should not start a release.

Do not create release tags by hand. Do not publish artifacts from a development checkout.
