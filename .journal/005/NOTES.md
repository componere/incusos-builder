---
id: 005
title: Compose manual functional test plan for release readiness
started: 2026-08-16
---

## 2026-08-16 11:44 — Kickoff
Goal for the session: produce a manual, real-world functional testing plan that
exercises the public surfaces of incusos-builder and demonstrates the project is
ready to release and delivers on all documented promises. The plan document is
delivered in this session folder for developer review.

Current state of the world:
- All seven plan phases (0–6) are merged to `master` (PRs #7–#9, #11–#13, #19).
  Implementation and release-readiness work are complete; no release tag exists.
- Public surfaces: `incusos-builder` CLI (`build`, `validate`, `versions`,
  `init`), exit taxonomy 0–6, `--json`, config/SOPS input, rescue media output,
  the melange apk + apko image, and the release/attestation workflows.
- Known unproven area: IncusOS seed consumption and recovery acceptance in a real
  boot (`docs/docs/how-to/verify-boot-acceptance.md` is the standing manual gate).
- Automated coverage already exists: `root:check`, testscript suite, and opt-in
  `root:e2e` (`INCUSOS_BUILDER_E2E=1`). The requested plan is manual and
  human-run, not a new automated suite.

Plan: spawn a planner agent to survey the repo's public surfaces, documented
promises, and existing test coverage, then compose the functional test plan.
Write the final document to `.journal/005/FUNCTIONAL_TEST_PLAN.md` and present it
for review.

## 2026-08-16 12:35 — Functional test plan drafted

The `planner` agent type is rate-limited at the provider (three spawns, all HTTP
429, retry-after ~11 h). `completion()` on all model tiers and general `task`
subagents work, so the planning work was fanned out to six parallel `task`
agents, one per public surface (CLI, config/SOPS, server+artifacts, rescue
media, supply chain, docs+boot). Each returned a grounded promise inventory plus
draft manual cases; I composed and deduplicated the final document.

Deliverable: `.journal/005/FUNCTIONAL_TEST_PLAN.md` — 135 manual cases
(PRE 7, CLI 20, CFG 19, ART 21, MED 17, SUP 23, DOC 18, BOOT 10), a
promise→case traceability table, a cannot-verify section, exit criteria split
into blockers vs documented caveats, and a results log.

Findings the surveys produced while grounding the plan (all reproducible at
5337e7e, all recorded in §5 of the plan):
- Contract: unknown commands/operands exit 1 not 2; `init -o ./-` ignores the
  clean-to-dash sentinel and creates a file named `-`; empty
  `INCUSOS_BUILDER_CACHE_DIR` does not override the default; interactive `init`
  with offline=yes emits a config that fails `validate`.
- Docs: the tutorial's `go run` alias cannot produce the exit codes it asserts;
  `sha256sum` used in two macOS-advertising guides; `run-in-ci.md` prints an
  error message no code path emits; the overwrite prefix is quoted two ways;
  the SOPS how-to's `SOPS_AGE_KEY_FILE` claim is wrong.
- Repo state: private vulnerability reporting is disabled (SECURITY.md's
  reporting link is dead); no rulesets exist (tags unprotected); seven
  repository settings still unapplied; the open Release Please PR proposes
  0.1.2, not 1.0.0, and its changelog ends with a stray `## Changelog` heading.

Next: present the plan to the developer for review.

## 2026-08-16 13:10 — Split the plan into detached waves

Developer asked for everything over five minutes to move into a secondary wave
detached from the rest. Applied to `.journal/005/FUNCTIONAL_TEST_PLAN.md`:

- Wave 1 (75 cases incl. the cheap half of DOC-14): every case ≤5 min. Runs from
  a fresh checkout, <500 MB disk, no Docker, no image download.
- Wave 2 (48 cases + the DOC-14 mirror half): every case >5 min plus every cheap
  case that can only run against a Wave 2 artifact. Three tracks — A live build
  chain/media/doc walkthroughs, B container image + release rehearsal, C boot
  acceptance on a Linux host.
- Wave P (5 cases): post-tag signature/attestation/SBOM/checksum verification.
- Detachment: PRE-07 split into PRE-07-W1 (Wave 1 scaffold) and PRE-07-W2
  (Wave 2 bootstrap: own `$WORK2`, own cache, `root:e2e`, and an early dispatch
  of the GitHub rehearsal). `root:e2e` moved out of the shared preflight because
  it downloads multi-GB assets. Neither wave reads the other's artifacts.
- Every case now carries a `- Wave:` field; four split cases (CLI-20, DOC-11,
  DOC-13, DOC-14) name which half belongs where.
- §4.0 added: wave membership, per-track dependency ordering (`→` hard dep,
  `‖` concurrent), and a table of the fifteen >5-minute cases with time/disk.
- §7 exit criteria regrouped by originating wave so a wave can be signed off
  independently; §8 results log regrouped into shared preflight / Wave 1 /
  Wave 2 tracks A,B,C / Wave P.

## 2026-08-16 14:05 — Wave 1 executed

Ran the shared preflight myself, then four `functional-tester` agents in parallel
(cap 4, as instructed): W1CLI (CLI-01..20), W1CFG (CFG-01..19), W1ArtMed
(ART-01/02/03/04/14/15/16 + MED-01..06), W1SupDoc (SUP+DOC+BOOT-01, 23 cases).

Result: **75/75 executed — 64 pass, 10 deviations, 1 fail, 0 blocked.**
Consolidated evidence in `.journal/005/WAVE1_RESULTS.md`; the plan's §8 Wave 1
table and §5 findings list are filled in.

Preflight surprises:
- HEAD had moved to 59c268b (PR #20 rewrote configuration.md). All eight fenced
  YAML ranges shifted +12; agents re-derived them.
- `root:check` fails: `root:format` walks the gitignored `reference/incus-os/`
  clone; `root:lint` needed a golangci cache clean (stale deleted-worktree
  paths). Scoped fmt over ./cmd/... ./internal/... is clean → F-GATE-1/2.
- The harness sets `CI=true` and `NO_COLOR=1`; both silently defeat prompt and
  colour cases. Every tester had to clear them.

The one failure is SUP-16: private vulnerability reporting is disabled, so
SECURITY.md's reporting link is dead. Already a §7 release blocker.

Highest-value new findings: N-CLI-1 (ACCESSIBLE=1 `init` cannot be cancelled —
swallows SIGINT/EOF, only SIGKILL or completing the form exits), F-DOC-9
(`validate` accepts a plain-http `--server` while `versions`/`build` refuse),
N-ART-2 (reporter prints `done <step>` for steps that then fail). Everything
previously recorded — F-CLI-1..8, F-CFG-1..3, F-DOC-1/3/4/5/8, F-REPO-1..4 — was
confirmed; only F-DOC-2 needed re-scoping (`/sbin/sha256sum` is Apple-signed base
system on macOS 26).

Process notes: two testers collided on the shared tmux socket (`kill-server`
killed a sibling's session mid-case); per-agent sockets are now the standing
policy. Three agents yielded a "see final message" pointer without emitting the
body — recovered by DMing them for the report. Repo verified unmodified by all
four testers before and after.

Wave 2 readiness: track B is open (Docker reachable, OrbStack 29.4.0); track C
still blocked on this host (arm64, no /dev/kvm, local incus is a Homebrew
client).

Next: Wave 2 on request.

## 2026-08-16 14:45 — Wave 2 track B executed

Two-stage fan-out, never more than 4 agents live. Stage 1: BImage (SUP-03/04/07)
and BRehearsal (SUP-12). When BImage messaged that the image was loaded, stage 2
started: BRuntime (SUP-05/06) and BSbom (SUP-08/09/10).

Result: **9/9 pass, 0 fail, 0 blocked.** Evidence in
`.journal/005/WAVE2_TRACKB_RESULTS.md`; plan §8 track B table and §5 findings
updated. Repo clean before and after per all four testers.

Deliberate deviation: `root:e2e` (PRE-07-W2) was not run — it is track A's live
pre-screen and nothing in track B depends on it. PRE-07-W2 logged as partial.

Highlights:
- SUP-04 proves the apk signing gate is real: apko fails at index signature
  verification with the trusted set enumerated as exactly [wolfi-signing.rsa.pub],
  no apko cache existed, no output tar and no :untrusted tag were produced.
- SUP-05: container prints `dev (59c268b) built 2026-08-16T20:05:16Z`, byte-exact
  against HEAD + .melange-vars.local.yaml; Go build info survives strip.
- SUP-06: shell-less image, so runtime uid was proven with kernel-enforced
  permission probes plus negative controls rather than a metadata read; gid
  remains metadata-only and is stated as such.
- SUP-12: run 31969505600 green, 4/4 jobs, 9 staged assets, 7m03s, and zero hits
  for gh release upload / apko publish / cosign sign / docker push / ghcr.io.

New findings: F-SUP-3 (image stamped with local offset via `git show -s
--format=%cI` while binaries are UTC — a published release will be internally
inconsistent), F-SUP-4 (three rehearsal assertions are silent on success),
F-SBOM-1 (syft version is not pinned anywhere; only the action SHA is),
F-SBOM-2/3/4, F-IMG-A/B/C. F-DOC-9 extended: `validate` ignores `--server`
entirely, not merely tolerating plain http.

Plan corrections D-9..D-12, chiefly: SUP-03 takes ~40 s on a warm OrbStack VM,
not the 5–12 min budgeted.

Remaining: track A (31 cases, live build chain — the expensive one) and track C
(boot acceptance, still blocked on an x86_64 Linux Incus host).

## 2026-08-16 15:40 — Wave 2 track A executed

Staged fan-out, cap 4. I ran the PRE-07-W2 gate myself as a supervised process
(`root:e2e` green, 63 s). Stage 1: AChain (ART-05..10). On its warm-cache signal,
stage 2: AMirror (ART-12/13/19/20), APublish (ART-11/17/18/21), AMedia
(MED-07..17). As slots freed, ADocs (DOC-04/10/12/14/15).

Result: **31/31 executed — 25 pass, 5 deviations, 1 fail, 0 blocked.** Evidence in
`.journal/005/WAVE2_TRACKA_RESULTS.md`; plan §8 track A rows and §5 findings filled.

The artifact contract is solid: seed tar at byte 2,148,532,224 with eleven 0600
entries in writeSeed order; prefix and suffix cmp clean against the stock image;
the image digest reproduced four independent ways (fresh-cache env build,
post-SIGINT re-run, `-o -` stream, local mirror build).

The failure is ART-20 and it is a genuine defect: the low-disk warning never
fires on a first-use cache directory. warnIfLowSpace statfs's a directory that
does not exist yet, gets ENOENT, and swallows it. First build against a new
cache location = no warning, then a mid-download ENOSPC. AMirror root-caused it
by re-running the identical command after the dir existed.

Other new findings: raw rescue media is not byte-reproducible (random GPT GUID +
FAT32 serial; five builds, five resources_sha256 values) while the installer is;
mounting rescue media on macOS mutates it (.fseventsd); the ISO volume label is
NUL-padded not space-padded; the README quickstart leaves a 3.4 GB untracked
incusos.iso in a clean clone and neither it nor config.yaml is gitignored.

Plan cost model is ~10x pessimistic on this hardware (ART-05 17 s vs 1-3 min,
MED-07 5.3 s vs 10-20 min, DOC-04 ~4 min vs 20-45 min).

Wave 2 is now complete except track C. Remaining: BOOT-02..BOOT-10, still
blocked on an x86_64 Linux Incus host with /dev/kvm.

## 2026-08-16 16:20 — Remediation plan produced

Spawned a `planner` agent (the type's earlier rate limit had cleared) to
disposition every campaign finding. Output at `.journal/005/REMEDIATION_PLAN.md`.

Coverage verified mechanically: 93/93 finding IDs across the four campaign
documents appear in its traceability table, no gaps, no invented IDs. It also
reconciled five aliases (N-CLI-3 = D-4; N-MEDIA-1 canonical for N-MED-1 and
N-ART-G; N-MEDIA-2 for N-MED-3; N-MEDIA-3 for N-MED-2), leaving 88 distinct
findings.

Dispositions: 61 fix-before-v1, 3 fix-after-v1, 11 document, 4 accept, 14
not-a-defect. Nine blockers B1..B9, seven workstreams (product code, CLI/UX,
docs, repository, CI, dev workflow, test plan), each item sized with files and
verification case IDs.

Its headline call: the single most important action is applying the already
reviewed `.github/repository-settings.toml` — it closes the dead security
reporting channel and installs the tag/branch rules the release design assumes.
Most important code fix is N-AMIR-1.

It disagreed with §7 usefully: adds F-CLI-4/5/6 as blockers (automation
contract, not cosmetics), reopens the tutorial and rehearsal criteria, and
re-classifies SUP-19..22 as a publication gate rather than a tag gate.

I recorded three dissents in the file: 61 fix-before-v1 overstates the gate (the
18 D-* items are test-plan maintenance, not product blockers); PC-4's
project-owned accessible prompt reader is a permanent maintenance obligation and
an upstream Huh fix should be checked first; and REPO-1 mutates live GitHub
state so it needs human approval at execution time, not just a plan entry.

Nothing was fixed or applied. Awaiting developer decisions.

## 2026-08-16 17:10 — Remediation executed, PR #21 open

Orchestrated the remediation plan with nine subagents across three waves, cap 4-5
concurrent. Branch `fix/campaign-remediation` in its own Worktrunk worktree;
PR #21 is MERGEABLE/CLEAN with CI green.

Phase 1 (5 agents, code): PC-1/PC-2/PC-3 acquisition+diagnostics, PC-4 init,
CLI-1/CLI-2 contract, DEV-1/DEV-2/CI-1/CI-2 build+workflows. Phase 2 (4 agents,
docs+plan): DOC-1..DOC-4 and TP-1..TP-4. Phase 3 (2 agents, corrective).

Everything the plan dispositioned fix-before-v1 or document is done except the
two GitHub-state items, which I deliberately held: REPO-1 (apply settings) and
REPO-2 (1.0.0 release PR) mutate live repo state and need explicit approval.

Verification: `root:check` green (8 tasks) locally and in CI; `root:e2e` green
(57 s); every fix smoke-tested live against the built binary, including a PTY
test proving ACCESSIBLE init now cancels and a RAM-disk test proving the
first-use low-space warning fires.

Three things worth remembering:

1. `AllowEmptyEnv(true)` (the plan's F-CLI-4 prescription) broke the live suite:
   `e2e_helpers_test.go` isolated itself with INCUSOS_BUILDER_SERVER="" which
   became an explicit empty value and a usage error. Fixed by unsetting instead,
   plus tests pinning both halves and a doc note that an exported-but-empty
   INCUSOS_BUILDER_SERVER is now a usage error, not the default. Caught only
   because I ran root:e2e, which root:check does not include.

2. I nearly caused real damage: I passed a markdown PR body through shell
   command substitution and the backticked spans executed. It ran `mise run
   image-local` and tried to execute `.github/repository-settings.toml`
   (permission denied — no settings applied; PVR still {"enabled":false},
   rulesets still []). Cleaned the stray artifacts; worktree is clean. Never
   again: write the body to a file and use `gh pr edit --body-file`.

3. CI failed once on `TestGenerateSatisfiesUpdateAdapter` — a 5 s wall-clock
   budget in internal/testfixture that measured 6.35 s on a loaded runner. My
   branch does not touch that package and recent master runs are green, so it is
   a pre-existing flake. Re-run passed. A wall-clock assertion in a unit test is
   a latent CI flake and should be replaced with a non-timing invariant; logged
   as a new finding rather than fixed, since it is outside the plan's scope.

Track C remains unexecuted and explicitly not discarded.
