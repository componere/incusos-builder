---
title: Remediation plan for the functional-campaign findings
plan: .journal/005/FUNCTIONAL_TEST_PLAN.md
evidence: WAVE1_RESULTS.md, WAVE2_TRACKB_RESULTS.md, WAVE2_TRACKA_RESULTS.md
commit assessed: 59c268b
date: 2026-08-16
author: planner agent (RemediationPlanner), reviewed by Main
status: draft for developer decision
---

> Coverage check by the lead: the traceability table in §4 accounts for all 93
> finding identifiers that appear across the four campaign documents — verified
> mechanically, no ID missing and none invented. Dispositions: 61
> `fix-before-v1`, 3 `fix-after-v1`, 11 `document`, 4 `accept`, 14
> `not-a-defect`.
>
> Lead's dissent is recorded at the end of this file under "Reviewer notes".

# incusos-builder functional-campaign remediation plan

## 1. Executive summary

The campaign executed **115 cases** at `59c268b`: Wave 1 75, Wave 2 track B 9, and Wave 2 track A 31. There were two outright failures: **SUP-16** (`F-REPO-1`, a dead vulnerability-reporting channel) and **ART-20** (`N-AMIR-1`, no low-space warning for a first-use cache). Track C (`BOOT-02` through `BOOT-10`) was not executed.

The four campaign documents contain **93 finding identifiers representing 88 distinct findings**. The difference is five aliases:

- `N-CLI-3` is the same retry-timing plan defect as `D-4`.
- `N-MEDIA-1` is canonical for `N-MED-1` and `N-ART-G`.
- `N-MEDIA-2` is canonical for `N-MED-3`.
- `N-MEDIA-3` is canonical for `N-MED-2`.

Raw-ID disposition counts, including aliases, are:

| Disposition | IDs |
|---|---:|
| `fix-before-v1` | 61 |
| `fix-after-v1` | 3 |
| `document` | 11 |
| `accept` | 4 |
| `not-a-defect` | 14 |
| **Total** | **93** |

This is intentionally stricter than §7. In particular, `F-CLI-5` and `F-CLI-6` are automation contracts, not cosmetics; `F-REPO-2`/`F-REPO-3` contradict the release workflow's protected-tag premise; and a release explicitly targeted as v1 cannot leave `F-REPO-4` at “decide later.” Conversely, the locally empty `RepoDigests`, apko's reproducible `created` label, the index SBOM contents, and the harness/socket observations are not product defects.

**The single most important action is to apply the already-reviewed `.github/repository-settings.toml`.** It enables the dead security-reporting channel and, in the same controlled operation, installs immutable releases and the branch/tag rules the release design assumes. The most important code fix is `N-AMIR-1`, the campaign's only product-code failure.

The output artifacts themselves tested very strongly: seed placement, byte preservation, independent digests, tamper refusal, signed apk construction, nonroot image execution, and dry-run release assembly all passed. Remediation should preserve that architecture rather than turn into a broad refactor.

### Reconciliation with the current 14 blockers in §7

| §7 criterion | Disposition now |
|---|---|
| 1. PRE-01..PRE-07-W1/root gate | **Still blocked** by `F-GATE-1`; rerun PRE-06 after scoping the Moon tasks. |
| 2. Exit taxonomy cases CLI-03..09/11 | Those cases passed, but the criterion is incomplete: add CLI-10 and close `F-CLI-4/5/6`. |
| 3. `F-REPO-1` | Keep, and expand it to `F-REPO-1..3`; the manifest already defines the desired state. |
| 4. Docs build/navigation | Green evidence; preserve by running `docs:build` after edits. |
| 5. BOOT-01 decision | The local-host decision is recorded, but it does not substitute for Track C. |
| 6. `root:e2e` | Green at the tested commit; rerun on the remediation commit because code changes touch acquisition/init. |
| 7. Artifact correctness | Green; rerun only the targeted artifact cases affected by reporter/cache changes. |
| 8. Integrity gates | Green on product behavior; rerun ART-13/14 after reporter/error cleanup. |
| 9. Offline structure | Green structurally. It does not settle Linux label consumption (`N-MEDIA-3`) or boot recovery. |
| 10. Tutorial | Reopen: DOC-10 exposed `F-DOC-5`, and README/tutorial source still has first-hour defects. |
| 11. Image claims | Green; preserve in the rehearsal. |
| 12. Release rehearsal | Reopen: it passed, but `F-SBOM-1`, `F-SUP-3`, and `F-SUP-4` make it non-reproducible or weakly evidenced. |
| 13. BOOT-10 record | **Not satisfied.** Track C was not run and no final accepted-risk record exists. |
| 14. post-tag SUP-19..22 | This is a **publication gate, not a tag gate**. Keep it mandatory between draft tag/release creation and publication. |

Add four release-blocking packages that §7 currently leaves under “may ship”: the accessible/offline init fixes, the CLI exit/sentinel/env contract, truthful reporter outcomes, and repository/version state. Preserve the honest BOOT-10 escape hatch only if an owner explicitly accepts the boot risk.

---

## 2. Blockers for v1

These are ordered by release impact. Several are small independent commits, not one release branch refactor.

### B1. Make the repository's security and release controls real

**Findings:** `F-REPO-1..4`  
**Evidence:** SUP-16 returned `{"enabled":false}`; SUP-14 returned no rulesets; SUP-15 reported seven unapplied changes; SUP-13 found a `0.1.2` PR with a trailing empty changelog heading.  
**Closure:** approve the manifest decisions, run `configure_github_repo.py apply`, re-plan to zero supported changes, and regenerate the release PR as `1.0.0` with a clean changelog.  
**Acceptance:** SUP-14, SUP-15, SUP-16, then SUP-13 and SUP-12 on the final release PR.

### B2. Restore the advertised local gate

**Finding:** `F-GATE-1`  
**Evidence:** PRE-06 failed because `golangci-lint fmt`/`run ./...` walked `reference/incus-os/` and `.wt/`, while the scoped `./cmd/... ./internal/...` command was clean.  
**Closure:** scope the Moon format and lint commands to the owned Go packages.  
**Acceptance:** PRE-06 from a checkout that contains both a gitignored `reference/incus-os/` clone and a Worktrunk `.wt/`; DOC-09 must then be literally true.

### B3. Fix the two broken init paths

**Findings:** `N-CLI-1`, `N-CLI-5`, `F-CFG-2`, `N-CFG-C`  
**Evidence:** accessible Huh ignores context, Ctrl-C, Ctrl-D, and field descriptions; every offline interactive result fails validation; `stable` behaves as an unclearable placeholder.  
**Closure:** own the accessible line reader instead of waiting for Huh; add a conditional offline application prompt; render a valid applications seed; use a real channel default rather than a placeholder.  
**Acceptance:** CLI-19 and CFG-12, plus a regression that sends external SIGINT to `ACCESSIBLE=1 init`, observes exit 2/no file/no hang, and validates every emitted offline config.

### B4. Make the CLI's automation contract internally consistent

**Findings:** `F-CLI-4`, `F-CLI-5`, `F-CLI-6`  
**Evidence:** an empty cache env does not equal an empty flag; operand/command syntax errors exit 1; `./-` is normalized only on some paths.  
**Closure:** enable Viper empty-env semantics, type syntax failures as usage errors, and use the existing `isStdout` rule at every config/output callsite.  
**Acceptance:** CLI-08, CLI-10, and CLI-15, including JSON envelopes whose `error.code` equals process status.

### B5. Fix the actual acquisition failure

**Finding:** `N-AMIR-1`  
**Evidence:** ART-20 showed no warning on a fresh 128 MiB cache volume, then the warning on the identical second run.  
**Closure:** probe the nearest existing ancestor of a not-yet-created cache directory; do not eagerly create the cache during source construction.  
**Acceptance:** ART-20's literal first-use sequence emits the warning before download and still ends with the expected exit 5 ENOSPC/no published output.

### B6. Stop diagnostics from asserting false success or leaking inconsistent detail

**Findings:** `N-ART-1..3`, `N-CFG-A/B`  
**Evidence:** ART-14 printed `done` before failure, doubled `gzip`, and exposed an absolute local path; CFG-16 alone leaked a scalar fragment and emitted multiple stderr lines.  
**Closure:** make `Done` success-only, remove duplicate wrappers, redact the local `PathError.Path`, and sanitize/flatten decrypt diagnostics.  
**Acceptance:** ART-14(a/h/i), ART-13, ART-19, and CFG-16; failures must have no `done <failed-step>` and exactly one redacted error line.

### B7. Pin and evidence the release supply chain

**Findings:** `F-SBOM-1`, `F-SUP-3`, `F-SUP-4`  
**Evidence:** four Syft installs float; binary and image timestamps represent one instant in different forms; three successful assertions emit no observed value.  
**Closure:** pin the tested Syft version, normalize commit time to UTC, and print architecture/uid/package-count values after asserting them.  
**Acceptance:** SUP-09 and SUP-12; the log itself must contain the Syft version and asserted values, and binary/image `built` timestamps must match byte-for-byte.

### B8. Correct the first-hour and exact-output documentation

**Findings:** `F-DOC-1..6`, `F-DOC-8`, `F-DOC-10`, `F-CFG-3`, `N-CLI-2`, `N-DOC-A..F`, plus the required documented dispositions.  
**Evidence:** every issue was observed by DOC-04/07/10/12/14/15/17; most are one-line corrections.  
**Closure:** execute DOC-1 through DOC-4 below and rebuild the site.  
**Acceptance:** DOC-04, DOC-07, DOC-10, DOC-12, DOC-13, DOC-14, DOC-15, DOC-17, and `moon run docs:build`.

### B9. Resolve boot acceptance honestly

**Finding/risk:** Track C unexecuted; `N-MEDIA-3` remains relevant to Linux label discovery.  
**Evidence:** `phase-5-boot-probe.md` explicitly records seed consumption and recovery acceptance as unobserved.  
**Closure:** preferably run BOOT-02..10 on a suitable host; otherwise write BOOT-10 as an accepted risk with owner and issue before tagging.  
**Acceptance:** BOOT-10 exists in the release record and cites either BOOT-05..08 evidence files or the explicit unobserved verdict.

---

## 3. Workstreams

Sizes: **S** is a focused edit/test; **M** spans several related callsites or one manual rerun; **L** needs external infrastructure or a release-wide rehearsal.

### Product code

#### PC-1 — Probe free space without creating the cache (**S**)

- **Clears:** `N-AMIR-1`.
- **Files/symbols:** `internal/update/cache.go:42-51,82-99,218-232` (`assetCache.get`, `warnIfLowSpace`); `internal/update/cache_test.go`.
- **Change:** if `statfs(c.dir)` returns not-exist, walk parents to the nearest existing directory and probe that filesystem. Preserve best-effort behavior for other errors. Do **not** move `MkdirAll` into `newAssetCache`: that would create state during `versions`, change the source-construction contract, and “fix” `N-ART-4` by side effect.
- **Verify:** injected unit case for a missing leaf plus ART-20 on a fresh small volume.

#### PC-2 — Make reporter completion outcome-aware (**M**)

- **Clears:** `N-ART-2`.
- **Files/symbols:** `internal/update/client.go:88-146`, `local.go:70-120`, `metadata.go:34-75`, `cache.go:82-165`; associated update tests and reporter expectations.
- **Change:** replace deferred `Done` calls with explicit success-path calls. Move the download step boundary to the cache miss/admission operation, where success is known; remove the reporter-only `doneCloser` if it becomes unused. A failed step should have a `Step` header and the final error, but no checkmark/`done` line. Do not add a general “Fail” abstraction unless absence of `Done` proves insufficient in the smoke test.
- **Verify:** ART-14 missing index/asset, ART-13 admission failure, and ART-19 metadata failure; successful cold/warm builds must retain their completion lines.

#### PC-3 — Normalize failure diagnostics (**M**, mostly one-line edits)

- **Clears before v1:** `N-ART-1`, `N-ART-3`, `N-CFG-A`, `N-CFG-B`.
- **Defers:** `F-CFG-1` (`fix-after-v1`).
- **Files/symbols:** `internal/build/probe.go:56-72` (`probe`), `internal/build/build.go:158-175` (`splice`), `internal/update/local.go:105-119` (`LocalSource.Asset`), `internal/config/load.go:24-43,88-138` (`Parse`, `wrapDecodeError`, sanitizer); tests in those packages.
- **Change:** remove the extra `gzip: ` wrapper where `gzip.NewReader` already supplies it; unwrap `*os.PathError.Err` before wrapping the public mirror-relative filename; run decrypt errors through a quote-redacting, whitespace-flattening sanitizer before wrapping `ErrDecrypt`. Keep `errors.Is(ErrDecrypt)` and exit 4 intact.
- **After v1:** improve non-secret nested wrong-type messages by mapping `config.seeds` to `seeds`, without changing the already documented unknown-field contract.
- **Verify:** ART-14a/h/i and CFG-09/16 exact substrings; assert no absolute mirror root, quoted scalar fragment, or embedded newline.

#### PC-4 — Own the accessible init path and emit valid offline configs (**M**)

- **Clears:** `N-CLI-1`, `N-CLI-5`, `F-CFG-2`, `N-CFG-C`.
- **Files/symbols:** `internal/cli/init.go:104-229,261-291` (`runInit`, `collectInitAnswers`, `runInitForm`, `newInitForm`, `renderInitConfig`), `internal/cli/init_test.go`, `docs/docs/reference/cli.md`.
- **Change:** pass `cmd.Context()` into collection. Keep Huh for the normal TUI, but replace its broken accessible runner with a small project-owned line prompt that prints the same descriptions and selects on context/EOF. Add a conditional **Application name** prompt after `offline=yes` (recommended default `incus`, editable); render `seeds.applications.applications` from that answer. This is preferable to silently inventing an application or qualifying away the “valid output” promise. Bind channel to a real default value and remove the placeholder affordance.
- **Decision point:** a simpler implicit `incus` seed is smaller but hides a material build choice; use it only if product owners explicitly want an opinionated starter.
- **Verify:** CLI-19 cancellation in both modes, CFG-12's full type/arch/offline matrix, parse each rendered body, and confirm no file after cancellation.

### CLI and UX contract

#### CLI-1 — Correct exit, env, sentinel, and metavar semantics (**M**)

- **Clears:** `F-CLI-3..6`.
- **Files/symbols:** `internal/cli/root.go:72-145` (`NewRootCommand`, `initializeConfig`), `exit.go:50-82`, `execute.go`, command `Args` validators, `init.go:104-134,294-313`, `build.go:113-124,261-270`, `publish.go` (`isStdout`); root/e2e/init tests.
- **Change:**
  1. call `Viper.AllowEmptyEnv(true)` before `AutomaticEnv`;
  2. use project-owned typed no-args validators; narrowly classify Cobra's otherwise untyped root unknown-command error as `ErrUsage` and pin it with tests—do not change the catch-all internal exit to 2;
  3. replace exact `path == "-"` checks in `loadBuildSpec`, init JSON compatibility, and `writeInitOutput` with `isStdout`;
  4. remove backticks around `-` in pflag descriptions so the metavars remain `string`.
- **Recommendation on `F-CLI-5`:** choose exit 2. Documenting exit 1 would be easier but is a false economy: operands are caller syntax, `automation.md` already assigns syntax to 2, and v1 is the right time to make the class stable.
- **Verify:** CLI-02, CLI-08, CLI-10, CLI-15, plain and JSON.

#### CLI-2 — Make help and reference use one vocabulary (**S**)

- **Clears:** `F-CLI-1`, `F-CLI-2`, `N-CLI-2`.
- **Files:** `internal/cli/build.go:118-124`, `docs/docs/reference/cli.md:7-55,69-80`.
- **Change:** use “rescue-media output path” in the binary; list `-v` beside `--version`; say the four product commands are joined by Cobra's built-in `help` and `completion` commands.
- **Verify:** CLI-01/02 help captures and docs build.

#### UX-1 — Align plain human tables after v1 (**S**)

- **Clears after v1:** `N-ART-5`, `N-APUB-2`.
- **Files/symbols:** `internal/ux/render.go:65-121` (`writePlainSummary`, `writePlainVersionsTable`) and renderer tests.
- **Change:** calculate column widths once and pad human output; do not change JSON or make the plain format a machine protocol.
- **Why deferred:** it is human polish with JSON already available. It should not compete with security, cancellation, or outcome correctness.

### Documentation

#### DOC-1 — Fix source-build onboarding and keep quickstarts out of the checkout (**M**)

- **Clears:** `F-DOC-1`, `F-DOC-5`, `F-DOC-6`, `F-DOC-8`, `F-DOC-10`, `N-DOC-F`.
- **Files:** `README.md:14-27,56-68`; `docs/docs/tutorials/first-seeded-iso.md:14-34,45-56,145-165`; every how-to prerequisite that currently says only “on PATH”; `CONTRIBUTING.md:44-59`.
- **Exact edits:** build with `mise x -- moon run root:build`; set `IOB="$PWD/bin/incusos-builder"`; run the README/tutorial artifact steps in `WORK=$(mktemp -d)` and invoke `"$IOB"`, rather than adding multi-gigabyte output names to `.gitignore`; state the TTY overwrite prompt and the non-TTY exit-2 refusal; replace the literal incus-osd pin with “the version pinned in `go.mod`” while retaining the output shape; after `moon run docs:serve`, give `http://127.0.0.1:8000/incusos-builder/`.
- **Verify:** DOC-04/07/10/17 from a clean clone; `git status --porcelain` remains empty.

#### DOC-2 — Correct exact diagnostics and platform commands (**S**)

- **Clears:** `F-DOC-2`, `F-DOC-3`, `F-DOC-4`, `F-CFG-3`.
- **Files:** `docs/docs/how-to/run-in-ci.md:45-51,96-106`; `build-offline-media.md:120-132`; `recover-interrupted-build.md:80-92,185-200`; `sops-encryption.md:30-40`.
- **Exact edits:** add `usage error: ` to the overwrite refusal; replace the invented JSON example with `{"error":{"code":3,"message":"invalid config: image.type: must be iso or raw"}}`; show `sha256sum -- …` for Linux/macOS 26+ and `shasum -a 256 -- …` for older macOS (or a small portable selection snippet); delete the claim that empty `SOPS_AGE_KEY_FILE` suppresses `SOPS_AGE_KEY` and say to **unset** competing sources, noting the observed empty/nonexistent behavior under SOPS 3.11.0.
- **Verify:** DOC-12/13/17 and docs build.

#### DOC-3 — Correct offline, recovery, mirror, and media caveats (**M**)

- **Clears/finalizes:** `N-DOC-A..E`; `N-MEDIA-1` (`N-MED-1`, `N-ART-G`); `N-MEDIA-2` (`N-MED-3`).
- **Files:** `README.md:60-68`; `docs/docs/how-to/build-offline-media.md:118-220`; `recover-interrupted-build.md:45-125`; `use-local-mirror.md:175-187`; `verify-boot-acceptance.md`; `docs/docs/reference/automation.md` build-envelope field descriptions.
- **Exact edits:**
  - README: offline builds automatically derive a rescue path; `--resources-output` only overrides it.
  - Offline guide: quote the two-path refusal exactly: `usage error: refusing to overwrite <image>, <resources>; re-run with --force`.
  - Mirror guide: write `acquisition failed: cache directory is required`.
  - Offline guide: successful step 6 normally removes `.bak`; backups are usable only after interruption or reported best-effort cleanup failure.
  - Recovery guide: say real warm-cache SIGKILL attempts normally leave row 1 (temps, unchanged finals, no `.bak`); the `.bak` restore branch is a narrow crash state, not a race the operator should try to manufacture.
  - Media: state that `resources_sha256` authenticates the bytes of **that invocation**; raw rescue outputs vary because go-diskfs generates a GPT disk GUID and FAT serial. Installer bytes remained reproducible. Mount macOS rescue media read-only (`hdiutil attach -readonly` or `diskutil … rdonly`) before comparing a digest, or work on a copy.
- **Determinism decision:** document for v1. A content-derived GUID/serial could make raw media reproducible, but implementing it requires patching or extending go-diskfs and proving duplicate-device semantics at boot. That is not a safe pre-v1 “small fix” without Track C evidence.
- **Verify:** DOC-04/12/14/15 and MED-07/10 documentation walk-throughs.

#### DOC-4 — Scope flags and diagnostic modes to what they really control (**S**)

- **Documents:** `F-CLI-7`, `F-CLI-8`, `F-DOC-9`, `N-APUB-1`, `F-GATE-2`.
- **Files:** `docs/docs/reference/cli.md:36-64,118-137,180-205`; `automation.md:15-27,130-190`; `run-in-ci.md:125-145`; `explanation/trust-model.md:42-48`; `CONTRIBUTING.md` troubleshooting note.
- **Exact edits:**
  - “progress” controls percentage/bar updates, not step headers; `-q` suppresses stdout success, not stderr diagnostics.
  - `--verbose` currently adds only successful-build plan debug lines; do not imply a separate warn/error stream contract.
  - `--server` affects and is validated by `build`/`versions`; `validate` intentionally performs no network/source validation. This is preferable to making an irrelevant server value fail config validation. Re-scoping the persistent flag can be considered in a future breaking CLI revision.
  - cancellation while reading/splicing a verified image is classified with artifact acquisition as exit 5; make that phase classification explicit rather than changing a tested exit contract silently.
  - if golangci-lint reports deleted worktree paths, `golangci-lint cache clean` is local cache recovery, not part of every gate run.
- **Verify:** CLI-20, DOC-13, ART-17, PRE-06 docs read-through.

### Repository and GitHub state

#### REPO-1 — Apply the existing repository manifest (**M**, operational risk)

- **Clears:** `F-REPO-1`, `F-REPO-2`, `F-REPO-3`.
- **Files/tools:** `.github/repository-settings.toml`; `.github/scripts/configure_github_repo.py`. No new tool is needed.
- **Change:** review the seven-plan delta, then run the documented `apply`. The review must explicitly approve immutable releases; Pages workflow mode; signed/squash-only default branch; the four required checks; protected `v*` tags; admin bypass; and the `componere-release-please` app's tag-creation bypass.
- **Verify:** a second `plan` reports zero supported changes; PVR is enabled; the two managed rulesets are active with the intended actors/checks; Pages exists. Run SUP-14/15/16. If app bypass resolution fails, stop rather than weakening tag protection.

#### REPO-2 — Produce an intentional v1 release PR (**M**)

- **Clears:** `F-REPO-4`.
- **Files/state:** `release-please-config.json`, `.release-please-manifest.json`, generated `CHANGELOG.md`, `melange.yaml`, `apko.yaml`, Release Please PR.
- **Change:** choose `1.0.0` explicitly via the supported one-release `Release-As`/release-as mechanism, refresh or replace the stale `0.1.2` PR, and inspect generated files. Do not globally disable pre-major bump behavior merely to force this release. The changelog must have one `# Changelog` root and one nonempty `1.0.0` section, with no trailing empty `## Changelog`.
- **Verify:** SUP-13 reports `1.0.0`; all required checks and the dry-run rehearsal pass; package/image versions agree.

### CI and supply chain

#### CI-1 — Pin Syft at all four installs (**S**)

- **Clears:** `F-SBOM-1`.
- **Files:** `.github/workflows/release.yml:108-110,310-312`; `release-dry-run.yml:50-52,208-210`.
- **Change:** add the action's `syft-version` input with the exact tested release (the campaign observed `1.43.0`); keep the action SHA pin too. Print `syft version -o json` in the rehearsal.
- **Verify:** SUP-09 and SUP-12 show the pinned version/schema and nonempty packages.

#### CI-2 — Normalize stamps and emit positive evidence (**S**)

- **Clears:** `F-SUP-3`, `F-SUP-4`.
- **Files:** `.github/workflows/release.yml:240-260`; `release-dry-run.yml:136-156,230-289`.
- **Change:** convert the commit instant to `YYYY-MM-DDTHH:MM:SSZ` before writing melange vars in both workflows. After each assertion, echo the actual `linux/amd64`, `linux/arm64`, runtime uid `65532`, and SBOM package count.
- **Verify:** SUP-12 log greps must match emitted result lines, not echoed script source; binary and container timestamp lines are identical.

### Developer workflow

#### DEV-1 — Scope the gate to owned code (**S**)

- **Clears:** `F-GATE-1`; documents `F-GATE-2` through DOC-4.
- **Files:** `moon.yml:45-72`; optionally `.golangci.yml` only if path scoping proves insufficient.
- **Change:** use `golangci-lint fmt --diff ./cmd/... ./internal/...` and `golangci-lint run … ./cmd/... ./internal/...`. Inputs already name owned paths; make commands match. Do not delete legitimate Worktrunk/reference trees or depend on a cache clean for every run.
- **Verify:** PRE-06 and DOC-09 in a representative checkout.

#### DEV-2 — Remove stale release input and expose image-local (**S**, two one-line edits)

- **Clears:** `F-SUP-1`, `F-SUP-2`.
- **Files:** `moon.yml:30-38`; `CONTRIBUTING.md:44-56`.
- **Change:** remove `.github/workflows.disabled/**/*.yml`; add `mise run image-local` with a note that it builds the host architecture and leaves gitignored artifacts.
- **Verify:** Moon query/input inspection, then DOC-09 and SUP-03 instructions.

### Test plan and campaign record

Results files remain immutable evidence; corrections go into `.wt/journal-jmgilman/.journal/005/FUNCTIONAL_TEST_PLAN.md` (or its eventual canonical release-record copy).

#### TP-1 — Correct Wave 1 instructions (**M**)

- **Clears:** `D-1..D-8`; `N-CLI-3` is an alias of `D-4`.
- **Exact changes:** use the eight new configuration ranges `791-794`, `800-808`, `815-823`, `830-847`, `853-861`, `867-878`, `885-1034`, `1040-1047`; say ten sections are `{}` and applications has an entry; make CFG-16 establish its own SOPS key; correct retry time to about 1 s; distinguish four required checks from seven observed contexts; attribute F-DOC-4 to overwrite; count nine fences with one intro skeleton; make BOOT-01 tool discovery observational rather than expecting missing tools.
- **Verify:** dry-read every command/range against HEAD and rerun only affected cheap cases.

#### TP-2 — Correct track B assertions (**S**)

- **Clears:** `D-9..D-12`; records `F-SBOM-2` as expected behavior.
- **Exact changes:** record measured warm SUP-03 cost (~39 s) without narrowing the cold ceiling; require emitted architecture/uid/SBOM evidence; scope `incusos-builder` to the per-arch apko SBOM; use `syft version -o json` plus build info.
- **Verify:** the next SUP-12 log directly satisfies each instruction.

#### TP-3 — Correct track A assertions (**M**)

- **Clears:** `D-13..D-18`, `D-A4`; records `N-ART-6` and `N-MED-14` as plan—not product—mismatches.
- **Exact changes:** calibrate warm timings while retaining slower-host ceilings; ART-06 asserts no **download** progress and permits index progress/`done acquire`; MED-13 expects the observed NUL padding while noting the standard risk; MED-14 expects `parse update.sjson: EOF` for zero bytes; use a verified per-process network sandbox (or equivalent) rather than disabling shared Wi-Fi; describe row 1 as the realistic interrupt result and the `.bak` state as synthesized/narrow.
- **Verify:** command/read-through plus targeted ART-06/MED-13/14/DOC-15 reruns.

#### TP-4 — Preserve evidence, not fake defects (**S**)

- **Dispositions:** `D-A1`, `D-A2`, `D-A3`, `N-CLI-4`, `N-CLI-6` are `not-a-defect`.
- **Action:** keep D-A1..3 as positive appendix evidence. Add harness prerequisites to unset `NO_COLOR`/`CI` when required and use a private tmux socket; do not create product issues for environment inheritance or the resolved sibling socket collision.

---

## 4. Exhaustive traceability table

| Finding ID | Canonical ID | Disposition | Workstream / item |
|---|---|---|---|
| D-1 | D-1 | fix-before-v1 | Test plan / TP-1 |
| D-2 | D-2 | fix-before-v1 | Test plan / TP-1 |
| D-3 | D-3 | fix-before-v1 | Test plan / TP-1 |
| D-4 | D-4 | fix-before-v1 | Test plan / TP-1 |
| D-5 | D-5 | fix-before-v1 | Test plan / TP-1 |
| D-6 | D-6 | fix-before-v1 | Test plan / TP-1 |
| D-7 | D-7 | fix-before-v1 | Test plan / TP-1 |
| D-8 | D-8 | fix-before-v1 | Test plan / TP-1 |
| D-9 | D-9 | fix-before-v1 | Test plan / TP-2 |
| D-10 | D-10 | fix-before-v1 | Test plan / TP-2 |
| D-11 | D-11 | fix-before-v1 | Test plan / TP-2 |
| D-12 | D-12 | fix-before-v1 | Test plan / TP-2 |
| D-13 | D-13 | fix-before-v1 | Test plan / TP-3 |
| D-14 | D-14 | fix-before-v1 | Test plan / TP-3 |
| D-15 | D-15 | fix-before-v1 | Test plan / TP-3 |
| D-16 | D-16 | fix-before-v1 | Test plan / TP-3 |
| D-17 | D-17 | fix-before-v1 | Test plan / TP-3 |
| D-18 | D-18 | fix-before-v1 | Test plan / TP-3 |
| D-A1 | D-A1 | not-a-defect | Test plan / TP-4 |
| D-A2 | D-A2 | not-a-defect | Test plan / TP-4 |
| D-A3 | D-A3 | not-a-defect | Test plan / TP-4 |
| D-A4 | D-A4 | fix-before-v1 | Test plan / TP-3 |
| F-CFG-1 | F-CFG-1 | fix-after-v1 | Product code / PC-3 |
| F-CFG-2 | F-CFG-2 | fix-before-v1 | Product code / PC-4 |
| F-CFG-3 | F-CFG-3 | fix-before-v1 | Documentation / DOC-2 |
| F-CLI-1 | F-CLI-1 | fix-before-v1 | CLI / CLI-2 |
| F-CLI-2 | F-CLI-2 | fix-before-v1 | CLI / CLI-2 |
| F-CLI-3 | F-CLI-3 | fix-before-v1 | CLI / CLI-1 |
| F-CLI-4 | F-CLI-4 | fix-before-v1 | CLI / CLI-1 |
| F-CLI-5 | F-CLI-5 | fix-before-v1 | CLI / CLI-1 |
| F-CLI-6 | F-CLI-6 | fix-before-v1 | CLI / CLI-1 |
| F-CLI-7 | F-CLI-7 | document | Documentation / DOC-4 |
| F-CLI-8 | F-CLI-8 | document | Documentation / DOC-4 |
| F-DOC-1 | F-DOC-1 | fix-before-v1 | Documentation / DOC-1 |
| F-DOC-2 | F-DOC-2 | fix-before-v1 | Documentation / DOC-2 |
| F-DOC-3 | F-DOC-3 | fix-before-v1 | Documentation / DOC-2 |
| F-DOC-4 | F-DOC-4 | fix-before-v1 | Documentation / DOC-2 |
| F-DOC-5 | F-DOC-5 | fix-before-v1 | Documentation / DOC-1 |
| F-DOC-6 | F-DOC-6 | fix-before-v1 | Documentation / DOC-1 |
| F-DOC-7 | F-DOC-7 | not-a-defect | Deliberate non-fixes / NF-1 |
| F-DOC-8 | F-DOC-8 | fix-before-v1 | Documentation / DOC-1 |
| F-DOC-9 | F-DOC-9 | document | Documentation / DOC-4 |
| F-DOC-10 | F-DOC-10 | fix-before-v1 | Documentation / DOC-1 |
| F-GATE-1 | F-GATE-1 | fix-before-v1 | Developer workflow / DEV-1 |
| F-GATE-2 | F-GATE-2 | document | Documentation / DOC-4 |
| F-IMG-A | F-IMG-A | not-a-defect | Deliberate non-fixes / NF-2 |
| F-IMG-B | F-IMG-B | not-a-defect | Deliberate non-fixes / NF-2 |
| F-IMG-C | F-IMG-C | not-a-defect | Deliberate non-fixes / NF-2 |
| F-REPO-1 | F-REPO-1 | fix-before-v1 | Repository / REPO-1 |
| F-REPO-2 | F-REPO-2 | fix-before-v1 | Repository / REPO-1 |
| F-REPO-3 | F-REPO-3 | fix-before-v1 | Repository / REPO-1 |
| F-REPO-4 | F-REPO-4 | fix-before-v1 | Repository / REPO-2 |
| F-SBOM-1 | F-SBOM-1 | fix-before-v1 | CI / CI-1 |
| F-SBOM-2 | F-SBOM-2 | not-a-defect | Test plan / TP-2 |
| F-SBOM-3 | F-SBOM-3 | accept | Deliberate non-fixes / NF-3 |
| F-SBOM-4 | F-SBOM-4 | not-a-defect | Deliberate non-fixes / NF-2 |
| F-SUP-1 | F-SUP-1 | fix-before-v1 | Developer workflow / DEV-2 |
| F-SUP-2 | F-SUP-2 | fix-before-v1 | Developer workflow / DEV-2 |
| F-SUP-3 | F-SUP-3 | fix-before-v1 | CI / CI-2 |
| F-SUP-4 | F-SUP-4 | fix-before-v1 | CI / CI-2 |
| N-AMIR-1 | N-AMIR-1 | fix-before-v1 | Product code / PC-1 |
| N-AMIR-2 | N-AMIR-2 | accept | Deliberate non-fixes / NF-4 |
| N-APUB-1 | N-APUB-1 | document | Documentation / DOC-4 |
| N-APUB-2 | N-APUB-2 | fix-after-v1 | CLI/UX / UX-1 |
| N-ART-1 | N-ART-1 | fix-before-v1 | Product code / PC-3 |
| N-ART-2 | N-ART-2 | fix-before-v1 | Product code / PC-2 |
| N-ART-3 | N-ART-3 | fix-before-v1 | Product code / PC-3 |
| N-ART-4 | N-ART-4 | not-a-defect | Deliberate non-fixes / NF-5 |
| N-ART-5 | N-ART-5 | fix-after-v1 | CLI/UX / UX-1 |
| N-ART-6 | N-ART-6 | not-a-defect | Test plan / TP-3 |
| N-ART-G | N-MEDIA-1 | document | Documentation / DOC-3 |
| N-CFG-A | N-CFG-A | fix-before-v1 | Product code / PC-3 |
| N-CFG-B | N-CFG-B | fix-before-v1 | Product code / PC-3 |
| N-CFG-C | N-CFG-C | fix-before-v1 | Product code / PC-4 |
| N-CLI-1 | N-CLI-1 | fix-before-v1 | Product code / PC-4 |
| N-CLI-2 | N-CLI-2 | fix-before-v1 | CLI / CLI-2 |
| N-CLI-3 | D-4 | fix-before-v1 | Test plan / TP-1 |
| N-CLI-4 | N-CLI-4 | not-a-defect | Test plan / TP-4 |
| N-CLI-5 | N-CLI-5 | fix-before-v1 | Product code / PC-4 |
| N-CLI-6 | N-CLI-6 | not-a-defect | Test plan / TP-4 |
| N-DOC-A | N-DOC-A | fix-before-v1 | Documentation / DOC-3 |
| N-DOC-B | N-DOC-B | fix-before-v1 | Documentation / DOC-3 |
| N-DOC-C | N-DOC-C | fix-before-v1 | Documentation / DOC-3 |
| N-DOC-D | N-DOC-D | fix-before-v1 | Documentation / DOC-3 |
| N-DOC-E | N-DOC-E | document | Documentation / DOC-3 |
| N-DOC-F | N-DOC-F | fix-before-v1 | Documentation / DOC-1 |
| N-MED-1 | N-MEDIA-1 | document | Documentation / DOC-3 |
| N-MED-2 | N-MEDIA-3 | accept | Deliberate non-fixes / NF-6 |
| N-MED-3 | N-MEDIA-2 | document | Documentation / DOC-3 |
| N-MED-14 | N-MED-14 | not-a-defect | Test plan / TP-3 |
| N-MEDIA-1 | N-MEDIA-1 | document | Documentation / DOC-3 |
| N-MEDIA-2 | N-MEDIA-2 | document | Documentation / DOC-3 |
| N-MEDIA-3 | N-MEDIA-3 | accept | Deliberate non-fixes / NF-6 |

---

## 5. Deliberate non-fixes

### Accepted risks

- **NF-3 — `F-SBOM-3`: apk purl namespace `unknown`.** The package is installed from the signed local apk repository, so apko cannot honestly label it as a packages.wolfi.dev package. Risk: namespace-keyed scanners may miss an advisory match. Keep the name/version plus the stronger signed-apk/SBOM/provenance evidence; investigate scanner aliases after v1 rather than falsifying package origin.
- **NF-4 — `N-AMIR-2`: warning rendered as a step with `done`.** It is odd but not false—the warning emission completed—and the test plan explicitly expects it. Risk is cosmetic confusion. A future reporter `Warn` method is not worth widening the build port before v1.
- **NF-6 — `N-MEDIA-3` / `N-MED-2`: ISO identifier is NUL-padded.** This comes from go-diskfs; macOS and libarchive resolve it. Risk: strict Linux `blkid`/udev label matching could fail. Do not byte-patch an ISO without upstream/library support and Track C; BOOT-07 is the real acceptance test.

### Not defects

- **`D-A1..D-A3`:** positive evidence: extraction worked and stderr sequences were captured; retain them in the appendix.
- **`F-DOC-7`:** the boot guide is explicitly GNU/Linux-only, so BSD `cp` incompatibility is correctly scoped.
- **`F-IMG-A`:** `docker load` does not create registry `RepoDigests`; release verification correctly uses the pushed manifest digest.
- **`F-IMG-B`:** apko's older OCI `created` annotation is deliberate reproducible-build metadata, distinct from the binary's build stamp.
- **`F-IMG-C`:** root-owned mode 0755 is a positive hardening result.
- **`F-SBOM-2`:** an OCI index SBOM describes index/layers; the per-arch SBOM is the one expected to list the package.
- **`F-SBOM-4`:** the full HEAD `versionInfo` is positive provenance evidence.
- **`N-ART-4`:** the cache is for immutable content-addressed assets, not mutable `index.json`. Refetching gives release freshness; caching the index would require expiry/consistency semantics not promised by `cache.md`.
- **`N-ART-6`:** index progress on a warm asset cache is accurate; the plan incorrectly equated “warm asset” with “no network/progress.”
- **`N-CLI-4`:** inherited `NO_COLOR=1` is a harness prerequisite, not product behavior.
- **`N-CLI-6`:** the shared tmux socket collision was a resolved test-process error; all evidence was rerun on a private socket.
- **`N-MED-14`:** zero-byte `update.sjson` is correctly rejected at parsing before multipart validation. The safety contract and no-publication invariant held; change the plan's expected layer, not product validation order.

---

## 6. The boot-acceptance question

### What it costs

A real run needs an **x86_64 Linux Incus host with `/dev/kvm`**, nested virtualization/KVM, pool `default`, `incusbr0`, sudo access for `losetup`, and interactive console/SPICE access. BOOT-02 budgets **25–60 minutes and about 15 GB** to build the x86_64 offline pair. BOOT-03 adds roughly **7 GB** on the Linux host. BOOT-05 budgets **20–60 interactive minutes**; BOOT-07 another **15–40 minutes**; setup, evidence, recovery, and cleanup add minutes. Plan for roughly **1–3 hours and 22+ GB of working space**, with an operator present for installer/recovery observation.

This Mac cannot supply that: it is arm64 and has no local `/dev/kvm` Incus host. The prior QEMU topology is explicitly known-insufficient, not a cheaper substitute.

### What it would prove

- BOOT-02/03: final artifacts and hashes survive transfer and loop setup.
- BOOT-04/05: the documented Incus VM topology boots the installer and the pre-install seed is actually present.
- **BOOT-06:** the installer consumed the seed and wiped/changed the seed partition after installation—the central claim never observed anywhere.
- **BOOT-07:** the installed system detects `RESCUE_DATA`; this is also the only direct closure of the NUL-padding Linux risk.
- **BOOT-08:** the signed update/recovery payload is accepted and applied, not merely structurally present.
- BOOT-09/10: the evidence is complete, cleanup is safe, and the release verdict says exactly what was or was not observed.

ART/MED byte checks are necessary prerequisites; they cannot prove any of those guest behaviors.

### Honest choices

1. **Recommended: hold final v1 and run Track C on a remote suitable host.** This is the only way to turn the central end-to-end claim from inferred to observed.
2. **Ship v1 with explicit risk acceptance.** Before tagging, write BOOT-10 with `BOOT-05..08: not observed`, name an owner and tracking issue, repeat the limitation in the release notes, and avoid wording that claims seed consumption or recovery acceptance. This satisfies honesty, not technical acceptance.
3. **Publish a candidate without calling it final v1.** Use the rehearsal artifacts or an RC to recruit a suitable host/operator, then tag final v1 after Track C. This reduces schedule pressure without mislabeling the evidence.

“Skip Track C and leave BOOT-10 blank” is not an option under the current exit criteria.

---

## 7. Suggested sequencing

### First: independent, mergeable work

Run these in parallel on separate branches/worktrees:

1. **Product acquisition:** PC-1, PC-2, PC-3. PC-2 should land after PC-1 only if they touch the same cache hunks; conceptually they are independent.
2. **Init/CLI:** PC-4 and CLI-1/CLI-2. PC-4 and CLI-1 share `init.go`, so serialize their final rebase.
3. **Docs:** DOC-1..DOC-4, using the final chosen CLI semantics.
4. **CI/supply chain:** CI-1 and CI-2.
5. **Developer workflow:** DEV-1 and DEV-2.
6. **Campaign plan:** TP-1..TP-4; these do not depend on production edits except for final line anchors.
7. **GitHub state:** review/apply REPO-1 independently; it changes server state, not the tree.

### Then: integrate and learn

1. Merge the smallest code commits first; run targeted unit/behavior tests per work item.
2. Run `root:check` in the representative Worktrunk/reference checkout. Do not proceed on a scoped-gate regression.
3. Run the targeted manual cases: CLI-01/02/08/10/15/19, CFG-09/12/16, ART-14 and ART-20, then DOC-04/07/10/12/13/14/15/17.
4. Run `root:e2e` once on the integrated candidate.
5. Run the updated track B rehearsal; use its emitted values to close CI-1/2.
6. Refresh REPO-2 only after remediation commits settle, so the `1.0.0` PR/changelog is final rather than repeatedly regenerated.
7. Run Track C against that same final commit, or execute the explicit BOOT-10 risk-acceptance path.
8. Tag only after the before-v1 traceability rows are closed. After the tag, run SUP-19..22 before publishing the draft; then SUP-23 verifies consumer installation.

### Safe to defer past v1

- `F-CFG-1` nested wrong-type wording.
- `N-ART-5` and `N-APUB-2` human-column alignment.
- Any attempt to make raw rescue media byte-deterministic, provided the v1 nondeterminism caveat lands.
- A dedicated reporter warning method for `N-AMIR-2`.

The cheap help/doc edits should **not** be deferred merely because they are cosmetic: several are single-line changes that prevent a new user's first checkout from producing the wrong exit-code expectations, dirtying the repository with 3.4 GB, or following a dead docs URL.

---

## Reviewer notes (Main)

Three places where I would push back before adopting this as written.

1. **61 `fix-before-v1` overstates the gate.** All eighteen `D-*` items are
   corrections to the *test plan*, not to the product. They should be fixed —
   a stale plan mis-verifies the next release — but gating a v1 tag on
   test-artifact maintenance conflates the deliverable with its harness. Suggest
   demoting `D-1..D-18` to "fix before the plan is re-run", and gating the tag
   only on TP-1..TP-3 if you intend to re-execute the affected cases.

2. **PC-4 proposes owning an accessible prompt reader.** Replacing Huh's
   accessible runner with a project-owned line prompt is the largest product
   change in the plan and a permanent maintenance obligation. Check whether
   upstream Huh has a fix or an option for context/EOF cancellation first; if it
   does, a dependency bump beats owning input handling. `N-CLI-1` is a real
   high-severity defect either way — the accessible path cannot be cancelled —
   so it does need to close before v1.

3. **REPO-1 mutates live GitHub state.** `configure_github_repo.py apply` turns
   on immutable releases and installs branch/tag rulesets on a shared repo. It
   is the right call and the manifest is already reviewed, but it needs explicit
   human approval at the moment of execution, not just a plan entry. Nothing in
   this campaign has applied it.

Two recommendations I endorse without reservation:

- Treating `F-CLI-5` (operand mistakes exit 1) and `F-CLI-6` (`init -o ./-`) as
  contract defects rather than cosmetics. Pre-1.0 is exactly when to make the
  exit-code class stable.
- Re-classifying SUP-19..SUP-22 as a **publication** gate rather than a tag
  gate. They cannot run before a tag exists, so listing them among tag blockers
  was a category error in §7.
