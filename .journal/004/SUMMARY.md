---
id: 004
title: Complete Phase 6 documentation and release readiness
date: 2026-08-16
status: complete
repos_touched: [incusos-builder]
related_sessions: [001, 002, 003]
---

## Goal
Complete the final implementation-plan phase: user and repository documentation, release-pipeline rehearsal and hardening, deferred-work tracking, review, verification, and a merge-ready PR. Do not cut or publish a release during the session.

## Outcome
Goal met. Phase 6 was implemented, reviewed, verified, and squash-merged as PR #19. All seven plan phases are now on `master`; the local default branch was fast-forwarded and the Phase 6 Worktrunk worktree was removed. No tag, release, artifact publication, or repository-settings apply occurred.

## Key Decisions
- Target Phase 6 rather than the initially requested Phase 5 -> Phase 5 was already complete in PR #13; the developer confirmed the correction.
- License under `Apache-2.0 OR MIT` -> the developer selected dual permissive licensing, aligned with Apache-2.0 upstream while retaining MIT reuse terms.
- Keep release verification non-publishing -> a faithful `workflow_dispatch` rehearsal exercised binaries, native per-arch apks, and the multi-arch image without creating tags, releases, registry pushes, signatures, or attestations.
- Keep the manual boot gate -> the Phase 5 probe never proved seed consumption or recovery acceptance, so documentation continues to require the Incus checklist before a release tag.
- Validate but do not apply repository settings -> the settings plan is ready, but applying remote policy outside the reviewed PR was intentionally excluded.

## Changes
- `docs/docs/` and `docs/mkdocs.yml` - added and navigated tutorial, how-to, reference, and explanation documentation; retained the manual boot-acceptance gate.
- `README.md`, `CONTRIBUTING.md`, `SECURITY.md` - replaced template/pre-release placeholders with current installation, contributor, support, and security guidance.
- `LICENSE-APACHE`, `LICENSE-MIT` - added the selected dual license.
- `.github/workflows/release-dry-run.yml` - rehearses the real ghd staging script, four binary/SBOM pairs, native amd64 and arm64 melange apks, a multi-arch apko image, image SBOM generation, nonroot execution, and the real `--version` entrypoint.
- `.github/workflows/release.yml`, `.github/workflows/attest.yml` - removed the invalid `--message` smoke test, inspect published images by digest, serialize push/dispatch for the same tag, and correctly propagate reusable-workflow attestation permissions.
- `.github/workflows/release-please.yml`, `release-please-config.json` - migrated the GitHub App input to `client-id`, confirmed changelog/version-marker behavior, and removed the dead dependency changelog section.
- `.github/repository-settings.toml` - added the GitHub Pages required check and aligned declared Dependabot version updates with repository configuration.
- GitHub repository variables - provisioned `COMPONERE_RELEASE_APP_CLIENT_ID` from 1Password and removed the obsolete `COMPONERE_RELEASE_APP_ID` after merge.
- GitHub issues #14–#18 - recorded update tarball, HTTP Range resume, pin cadence, cache eviction, and signed-index / `--update-ca` follow-ups.
- PR #19 - squash-merged Phase 6 as commit `5337e7eb0f533f4c072be401cd3ed7a2bc541d38`.

## Open Threads
- No release was cut. Review the next Release Please PR and run the manual boot-acceptance checklist before merging a release/tagging.
- `.github/repository-settings.toml` remains unapplied; run the plan command and review unsupported/manual controls before applying it.
- Seed consumption, `RESCUE_DATA` detection, and signed recovery-metadata acceptance remain unobserved end to end.
- Follow-up product work is tracked in issues #14–#18.

## Lessons
- Permissions requested by a reusable workflow must also be granted by every caller; adding `artifact-metadata: write` only to the callee makes the workflow invalid.
- Release concurrency must normalize `github.ref_name` and dispatch input to the same bare tag; `github.ref` separates tag-push and manual-dispatch runs for the same release.
- A release smoke test must exercise an actual CLI contract before publication. The removed `--message` flag would have failed only after the image had already been pushed.
- Go's default cache directory is platform-specific: Linux honors `XDG_CACHE_HOME`, while macOS uses `$HOME/Library/Caches`.

## References
- PR #19: https://github.com/componere/incusos-builder/pull/19
- Release rehearsal: https://github.com/componere/incusos-builder/actions/runs/31963560870
- Follow-up issues: https://github.com/componere/incusos-builder/issues/14 through https://github.com/componere/incusos-builder/issues/18
- Manual boot gate: `docs/docs/how-to/verify-boot-acceptance.md`
- Original plan: `.journal/001/PLAN.md`
- Phase 5 summary: `.journal/003/SUMMARY.md`
