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

## 2026-08-16 17:35 — REPO-1 applied

Ran `configure_github_repo.py plan` then `apply` against componere/incusos-builder.
The plan reported **six** changes, not the seven the campaign recorded: "Create
GitHub Pages site" is gone because CI created the Pages site in the meantime.

Applied: general repository settings, immutable releases, private vulnerability
reporting, automated security fixes, managed branch ruleset "Default branch",
managed tag ruleset "Default tags". No warnings.

Verification:
- SUP-16 CLOSED — `private-vulnerability-reporting` is `{"enabled":true}`.
  SECURITY.md's reporting link works. This was the campaign's only outright
  failure and a §7 release blocker.
- SUP-14 CLOSED — both rulesets active (branch + tag).
- SUP-15 CLOSED — a second plan reports "No supported changes are required."
- PR #21 is still MERGEABLE/CLEAN with the four required contexts enforced
  (`ci`, `GitHub Pages`, `Binary Release Dry Run`, `Container Image Dry Run`).
  The two dry-run contexts report `SKIPPED` on ordinary PRs, which counts as
  passing, so the ruleset does not deadlock normal work.
- Merge methods are squash-only (merge and rebase both false), matching the
  repo's squash-merge policy.

Two facts worth recording accurately rather than repeating the plan's wording:

1. The tag ruleset targets `~ALL` tags, not just `v*`. Stricter than the plan
   described, and consistent with intent, but it means *any* hand-created tag
   now needs the admin bypass.
2. The release-please app bypass RESOLVED: the tag ruleset's bypass actors are
   RepositoryRole 5 (admin, always) and Integration 4551177 (always). The plan
   said to stop rather than weaken tag protection if this failed; it did not
   fail, so Release Please can still create tags.

Honest gap: `immutable_releases` is not exposed as a field on the repo REST
object, so I could not independently confirm it after the fact. The apply report
records the toggle as `current=false -> desired=true` with no warning, and the
re-plan shows no remaining delta, which is the strongest available evidence.
Enabling it now was low risk because no releases exist yet, so there is nothing
retroactive to freeze.

Nine manifest keys remain unsupported by the REST API and are manual follow-ups
(archive enrollment, automatic dependency submission, dependabot malware alerts,
alert-dismissal controls, grouped security updates, dependabot version updates,
code quality preview, commit comment visibility, PR creation permissions).

REPO-2 (release PR as 1.0.0) is still held for approval.

## 2026-08-16 18:20 — Track C execution options researched

Three researchers in parallel: ProviderScan (paid infra), EphemeralCI (CI/dev
environments), TechFloor (technical floor). Output at
`.journal/005/TRACK_C_OPTIONS.md`.

The important result is not a price. **BOOT-06 cannot pass as written**, on any
host. At pinned upstream 0f5b8057f2fc the installer does not wipe the seed from
the installer media: it copies partition 2 to the target and deletes install.*
from the TARGET (install.go:893-894 calling seed.CleanupPostInstall; seed.go:31-36
does the tar --delete). Source is opened O_RDONLY at install.go:1023, target
O_WRONLY at :1061. BOOT-06 hashes the SOURCE before and after and asserts the
digest changed, which can never happen. Corrected oracle: keep BOOT-05's capture
as an input baseline (rename source-seed.before.*), then after install detach the
target volume, attach it to a throwaway Linux VM, locate it by PARTLABEL
seed-data, and compare against the source baseline.

That also reinterprets the Phase 5 probe: target-disk growth was the right oracle
in principle, so zero growth means the installer never reached its write path,
not that the seed was ignored. Check the target for a GPT and seed-data partition
to tell "never started" from "ran but did not consume".

Second correction: BOOT-07 does not exercise N-MEDIA-3 at all. Track C uses
image.type raw, so recovery matches GPT PARTLABEL, never the NUL-padded ISO PVD.
Closing that risk needs an extra ISO run.

Cost floor: no physical TPM needed (Incus tpm device is swtpm; QEMU emulator
backend), so any working /dev/kvm qualifies. Cheapest credible paths: Semaphore
Cloud f1-standard-4 at $0 under its recurring credit with documented nested virt
and a first-party 3-hour attended SSH session with port forwarding; or Scaleway
EM-A610R-NVMe bare metal at about EUR 0.11-0.33 for the run. GitHub-hosted
runners do now expose /dev/kvm but only ~14 GB disk and unsupported generic
nesting, so not viable for a 50 GiB target.

Pre-staging: build on the M4 first and transfer only ~3.76 GB (the plan's 15 GB
is a workspace budget, not the transfer pair). Watch the disk caveat: 22 GB holds
only with sparse copy and a thin pool; on thick provisioning budget ~120 GiB.

Next decision for the developer: apply the BOOT-05/06 checklist correction, then
pick Semaphore (free) or Scaleway (~EUR 0.35).

## 2026-08-16 19:05 — BOOT oracle corrected; Semaphore blocked on GitHub authorization

Checklist corrections landed (three parallel agents):
- `docs/docs/how-to/verify-boot-acceptance.md` on `fix/campaign-remediation`
  (commit 9bb06cb, pushed, PR #21): BOOT-05 reframed as an input baseline with
  `source-seed.before.*`; BOOT-06 rewritten around the target-side oracle
  (detach target volume -> throwaway Linux VM -> locate by PARTLABEL=seed-data ->
  assert source-contains / target-lacks / digests-differ); BOOT-07 annotated that
  the raw config matches the GPT partlabel and never exercises the NUL-padded ISO
  identifier, with an optional ISO variant; evidence inventory renamed.
- `.journal/005/FUNCTIONAL_TEST_PLAN.md`: same corrections plus new finding
  **F-DOC-11** (BOOT-06 unpassable as written, cited to install.go:893-894 and
  seed.go:31-36) and corrected Track C costs (~3.76 GB transfer; ~22 GB only with
  sparse copy + thin pool; ~120 GiB on thick provisioning).
- `chore/semaphore-preflight` branch (commit 6abf9c1, pushed): disposable
  `.semaphore/semaphore.yml` preflight — 8 fail-fast checks ending in a real
  `-enable-kvm` guest boot using the host kernel as payload (zero download).
  Deliberately uses `-enable-kvm` not `-accel kvm:tcg`, so it cannot manufacture
  a pass via software emulation.

Verified independently against primary docs:
- `f1-standard-4` is 4 vCPU / 16 GB / 65 GB Intel x86_64 and pairs with
  `ubuntu2404` (docs.semaphore.io/reference/machine-types).
- Nested virtualization IS officially supported: "Linux-based machines support
  nested virtualization" (docs.semaphore.io/reference/os-ubuntu). Note their own
  snippet says `grep -cw vmx /proc/cpuinfo` "should be 0", which is backwards —
  a count of 0 means absent. Our preflight asserts presence, which is correct.

**Blocked:** the Semaphore account has not authorized GitHub. Both integrations
fail at `sem init`:
- `github_token`: `Token for not found.`
- `github_app`: "It looks like you haven't authorized Semaphore with GitHub,
  please visit https://docs.semaphoreci.com/using-semaphore/connect-github"
Projects are mandatory — `sem create job` and `sem create workflow` both refuse
without one, so there is no CLI-only path around it. Authorizing a third-party
app against the componere org is a security-significant grant I will not perform
on the developer's behalf.

Confirmed zero collateral: the failed init created **no** webhook and **no**
deploy key (repo reports 0 of each). Nothing to clean up.

Pipeline YAML validated offline: parses, correct schema version, correct machine
type and image, 56 commands, manual-only parameter gate that fails closed.

## 2026-08-16 19:40 — Semaphore preflight PASSED; Track C has a free host

Developer connected GitHub; Semaphore auto-created project
`incusos-builder-eins` (id e3c87503) bound to the repo via github_app.

First action was defensive: the project shipped with `run_on: [tags, branches,
draft_pull_requests]` and an EMPTY whitelist, i.e. it would build every branch
and every draft PR — including master and PR #21, none of which carry
`.semaphore/semaphore.yml`, producing "pipeline file not found" noise on the
product repo. Scoped it with `sem apply` to `run_on: [branches]` and
`whitelist.branches: [chore/semaphore-preflight]` before triggering anything.

Preflight pipeline 9c8bb158, job 6c20e0e3: **state done, result passed.**

Observed on `f1-standard-4` / `ubuntu2404`:
- arch x86_64; CPU 13th Gen Intel Core i5-13500 (newer than the i5-12500 the
  machine-types doc advertises); 4 vCPU; 16 GB RAM
- virt-flag `vmx`; `kvm_intel` and `kvm` modules loaded
- `/dev/kvm` readable AND writable by the unprivileged `semaphore` user
- 52 GB free on the working filesystem (need 25)
- passwordless sudo resolves to root
- QEMU 8.2.2, swtpm 0.7.3, OVMF at /usr/share/OVMF/OVMF_CODE_4M.fd
- **real KVM guest booted**: under `-enable-kvm` the guest printed
  `Hypervisor detected: KVM` and `smpboot: CPU0: 13th Gen Intel Core i5-13500`,
  then the expected `Kernel panic ... unable to mount root fs` (no rootfs was
  supplied — that panic is the success condition), qemu exit 0
- Incus 7.3 client and server installed from Zabbly; daemon activating

That is a genuine nested-KVM host with a software TPM available, for $0 under
the recurring credit. Track C's blocking constraint is solved.

Remaining gaps before an actual gate run, in order:
1. `incus admin init` with a `default` pool and `incusbr0` — not yet exercised.
2. The attended console: `sem debug job --duration 3h` plus port forwarding for
   noVNC/SPICE. Untested; it is the piece that decides whether the four required
   observations can actually be watched.
3. Transfer the ~3.76 GB artifact pair built on the M4.
4. Disk: 52 GB free is enough only with a sparse/thin target volume. A
   thick-provisioned 50 GiB Incus volume will not fit alongside the installer.
   Must use a thin/COW pool (dir or btrfs/zfs sparse) — verify before the run.

Cleanup owed: delete or keep-scoped the Semaphore project when done.

## 2026-08-16 20:15 — Track C venue proven end to end; runbook written

Second preflight block passed (pipeline 5e4dfbe6, job c82f95e5). Semaphore runs
the exact Track C topology nested: incus admin init on a sparse `dir` pool,
`incusbr0` at 10.193.173.1/24, a VM with UEFI and a vTPM added before first
start, `/dev/tpm0` + `/dev/tpmrm0` inside the guest with tpm_version_major=2,
`/sys/firmware/efi` present, 913 MB actual usage for an 8 GiB volume (sparse
confirmed), 50 GB free afterwards.

The important discovery is that **no graphical console is needed**. At the
pinned commit, `internal/tui/tui.go:67-71` adds `/dev/ttyS0` to the TUI's output
devices when `/dev/virtio-ports/org.linuxcontainers.incus` exists — i.e. when
running as an Incus VM. So `incus start $VM --console` inside the SSH debug
session shows the installer. No SPICE, no noVNC, no port forwarding, no X.

Two consequences:
- The old note "the shipped UKIs configure no console" is refuted for the Incus
  venue. It was true of the plain QEMU/OVMF spike, which has no virtio port —
  which very likely explains why the Phase 5.2 probe could not distinguish a
  failed install from a silent one.
- This argues for running the gate under Incus specifically rather than a plain
  QEMU equivalent; the documented checklist and the observable console coincide
  only in the Incus case.

Caught a defect in the researcher's drafted runbook: its seed-consumption step
reproduced the OLD source-side oracle (hash SOURCE before/after, assert
changed), because it read `verify-boot-acceptance.md` from master rather than
the corrected version in PR #21. Rewrote that step to the target-side oracle
before committing anything.

Runbook at `.journal/005/TRACK_C_RUNBOOK.md`, with preflight evidence, the
corrected oracle, and residual risks (ephemeral agent dies with the SSH session;
no published debug-duration maximum; serial mirroring is Incus-specific and
should be re-checked on pin bumps).

Cost: ~$2.70 of the $15 recurring credit for a three-hour session. Effectively
free.

Not yet done: the gate itself has not been run. Everything now needed is the
artifact build plus one attended session.

## 2026-08-16 21:35 — TRACK C RAN, AND SEED CONSUMPTION WAS OBSERVED

PR #21 merged as 64cf0ee. Built the x86_64 offline pair locally in 35 s
(installer 3,432,026,112 B sha 2f6b26c0..., rescue 329,374,720 B sha 6ed42a34...,
seed_bytes 4096; seed tar at byte 2,148,532,224 contains applications.yaml,
install.yaml, update.yaml).

Then decided against the attended SSH route. `sem debug job` allocates an
ephemeral agent that dies with the SSH session — a three-hour run with an
unrecoverable failure mode. Wrote the gate as an unattended pipeline instead
(`.semaphore/trackc-gate.yml`, 963 lines, 125 commands) that builds artifacts on
the box, runs the corrected oracle, and archives console evidence. A captured
console log is better evidence than a human watching, and the assertion strings
are all traceable to pinned upstream source.

First run failed in 30 s: the pipeline exported `MISE_PIN`, which collides with
mise's own reserved boolean `Settings::pin`. Renamed to `GATE_MISE_VERSION`.

Second run: pipeline 4c5cc805, job a8b16331 — **PASSED through stage 9/9** in
about 15 minutes.

### The result that matters

**O2, seed consumption, is OBSERVED — the first time anywhere in this project.**

    source seed  /dev/loop0p2  8537e356...  entries: applications.yaml install.yaml update.yaml
    target seed  /dev/sdb2     84a75f5c...  entries: applications.yaml update.yaml
    source after install:      8537e356...  (unchanged, exactly as predicted)

install.yaml is present in the source and absent from the target; the digests
differ; applications.yaml survives on the target as a positive control proving
the partition is intact rather than blank. That is precisely the corrected
oracle, and it also confirms the F-DOC-11 analysis: the source is untouched, so
the old source-side assertion could never have passed.

### The other three observations

- O1 install completion: `IncusOS was successfully installed` followed by
  `Please remove the install media to complete the installation`, observed on
  serial 45 s after start (install.go:388-390).
- O3 `Recovery partition detected` (recovery.go:50).
- O4: `Update metadata detected, verifying signature` (recovery.go:180) then
  `Processing validated update metadata` (recovery.go:212), which is only
  reachable after util.VerifySMIME succeeds; then `Recovery actions completed`
  (recovery.go:89) and `System is starting up` (main.go:657). `Recovery failed:`
  ABSENT.

The pipeline deliberately reports **INCOMPLETE-PENDING-ADJUDICATION**: it marks
O4 as ACCEPTED-NOT-ADJUDICATED rather than silently claiming a gate pass,
because the guide forbids inventing a success criterion. A human must read
gate-result.txt and adjudicate O4 before this is recorded as a formal pass.
That restraint is correct and I am not overriding it.

Also of note: the Phase 5 probe's negative result is now fully explained. Its
oracle (target growth) was right in principle; the installer simply never
reached its write path there, and its console was blind because plain QEMU has
no org.linuxcontainers.incus virtio port, so the TUI never mirrored to ttyS0.

Cost: about $0.25 of the $15 recurring credit.

Outstanding: adjudicate O4; pull the artifact bundle (the `sem` CLI has no
`artifact` subcommand — fetch from the job page or use the API); then record
BOOT-10 and update the release record. `N-MEDIA-3` (ISO label) still untested.

## 2026-08-16 22:30 — O4 adjudicated; gate recorded; scaffolding removed

Developer adjudicated O4, making Track C a formal **pass**: O1, O2, O3 observed
and asserted against pinned upstream strings, O4 accepted by the pipeline and
adjudicated by a human.

Recorded:
- `.journal/005/BOOT_10_RECORD.md` — the release-record artifact BOOT-10
  demands. Verdict, venue, pipeline/job ids, builder commit, upstream pin,
  per-observation evidence, and the honest limits.
- `.journal/005/TRACK_C_GATE_LOG.txt` — the raw 2,478-line job log, 361 GATE
  evidence lines, kept as primary evidence. The Semaphore v1alpha API does not
  expose job artifacts, so the log is the durable record.
- `FUNCTIONAL_TEST_PLAN.md` — §1, §6, §7, §8 updated. BOOT-02..BOOT-09 marked
  pass with a note wherever the unattended pipeline differed in *mechanism* from
  the attended procedure; blocker 13 satisfied; seed consumption and recovery
  acceptance moved from cannot-verify to observed; N-MEDIA-3 kept open.
- PR #24 merged (e02dd1e): product docs now state the observed result.
  `trust-model.md` corrected, `verify-boot-acceptance.md` records the successful
  run and replaces VGA/SPICE with the Incus serial console, and the Phase 5.2
  probe note is superseded — findings unaltered — with the explanation of why it
  was blind.

Verified the record rather than trusting it: the builder commit it cites
(c08cef30) is exactly the sha the job logged as its input.

Cleanup done: Semaphore project deleted; repo left with 0 webhooks and 0 deploy
keys; `chore/semaphore-preflight` branch and worktree removed locally and on the
remote; `docs/boot-acceptance-observed` merged and removed. Total Semaphore
spend across preflight and gate: well under $1 of the recurring credit.

Open items for the developer:
1. **REPO-2** — the release PR is still 0.1.2, not 1.0.0. Needs the deliberate
   Release-As decision. This is now the main thing between here and a tag.
2. Three Dependabot alerts surfaced by enabling security features: high
   pymdown-extensions ReDoS, medium pymdown-extensions path traversal (both
   docs-only pip deps), low cloudflare/circl in go deps.
3. N-MEDIA-3 still needs an optional ISO-media gate run to close.
4. Deferred by plan: F-CFG-1, N-ART-5, N-APUB-2.

## 2026-08-17 02:20 - Release version baseline corrected to 0.1.0

Developer rejected 1.0.0 (not ready to promise a stable API) and asked why the
release PR proposed 0.1.2. Root cause: `.release-please-manifest.json` carried
`0.1.1` from the initial template commit (424e764) - it is
`meigma/template-go`'s own release state, copied in wholesale. This repo has
zero tags and zero releases, so the number was extrapolated from releases that
never happened.

**I got the fix wrong on the first pass and the developer's pushback caught it.**
I claimed manifest-`0.0.0` plus `bump-patch-for-minor-pre-major: true` would
yield 0.0.1. Wrong: `manifest.ts` special-cases `0.0.0` out of the manifest
backfill, so no `latestRelease` exists, so `base.ts buildNewVersion()` never
calls the versioning strategy at all - it returns `initialReleaseVersion()`,
which defaults to `Version.parse('1.0.0')`. My "fix" would have produced exactly
the version that was rejected. I had read `determineReleaseType` correctly but
never checked whether it was the code that would run.

Landed (PR #26, 2abe534):
- manifest `0.1.1` -> `0.0.0`
- `initial-version: "0.1.0"` added - makes the first release stated, not derived
- `bump-patch-for-minor-pre-major` -> `false` (post-first-release cadence:
  features move the minor, not the patch)
- `bump-minor-pre-major` stays `true` - breaking changes bump the minor, so the
  project stays in 0.x and never silently promises a stable API
- melange.yaml / apko.yaml markers -> `0.0.0`

Then PR #29 (6369865) closed the second half of F-REPO-4: `CHANGELOG.md` shipped
as a `# Changelog` stub, and release-please's updater runs `adjustHeaders()`
over any pre-existing content when the file has no version heading, demoting the
H1 and re-appending it as a stray `## Changelog`. Emptying the seed takes the
clean branch. Confirmed gone in the regenerated PR.

Verified empirically, not by analysis: PR #10 regenerated as
**`chore(master): release 0.1.0`** with 0.1.0 in all four files and a clean
changelog.

Also observed: master moved twice under me (#27 dropped the flaky wall-clock
assertion in internal/testfixture; #25 trimmed comments repo-wide). #26's first
CI run failed on that pre-existing flake before I rebased.

Next: PR #10 is MERGEABLE/BLOCKED pending review. Merging it tags v0.1.0 and
triggers the real publish pipeline - the first non-rehearsal run.

## 2026-08-17 02:40 - Dependabot backlog cleared as one batch

Eight open Dependabot PRs, all raised against an older master and all
`UNKNOWN` mergeable (unrebased). Merging them serially would have cost eight
rebase-and-rerun cycles, since each merge invalidates the next. Applied them
together on one branch instead: PR #30, merged as `2e14c9f`.

Security (now **0 open Dependabot alerts**):
- `pymdown-extensions` 10.21.3 -> 11.0.1 - high (b64 path traversal) + medium
  (caret/tilde ReDoS). Transitive via `mkdocs-material`, so it lands in
  `docs/uv.lock`, not `docs/pyproject.toml`.
- `github.com/cloudflare/circl` 1.6.1 -> 1.6.3 - low, `// indirect` in go.mod.

Action pins - every SHA verified against the upstream tag through the GitHub
API rather than trusted from Dependabot's trailing comment (all five matched):
cache 6.0.0->6.1.0, attest-build-provenance 4.1.1->4.2.2, setup-go 6.5.0->7.0.0,
login-action 4.2.0->4.6.0, goreleaser-action 7.2.2->7.2.3.

**Verification gap found and closed.** `release-dry-run.yml` gates its jobs on
`workflow_dispatch || startsWith(github.head_ref, 'release-please--')`, so on an
ordinary PR the release contexts report `skipping` - which counts as passing.
Plain CI green would therefore not have proven the `setup-go` major bump on the
release path at all. Dispatched the dry run against the branch: run
31988148676, all four jobs success, and the logs confirm it downloaded
`setup-go@b7ad1dad31e0` (v7.0.0) and ran `goreleaser-action@f06c13b6b1a9`
(v7.2.3) rather than anything cached.

Residual, stated rather than papered over: `actions/attest-build-provenance`
cannot be exercised before a real release - `attest.yml` is `workflow_call`
only and is invoked solely by `release.yml`; the dry run just asserts the file
exists. Since no release has ever been cut, 4.1.1 had no more real-world proof
than 4.2.2, so no known-good was given up.

`pymdown-extensions` v11 is a major under `mkdocs-material`; `moon run
docs:build` is green. Its Material "MkDocs 2.0" banner is an upstream advisory
that also appears on master - checked, not assumed.

Dependabot auto-closed only #1 on its own scan; closed the remaining seven
explicitly against #30 with branch deletion, rather than leaving stale PRs to
drift.

State: the only open PR is #10, `chore(master): release 0.1.0`. Merging it tags
v0.1.0 and runs the first non-rehearsal publish.

## 2026-08-17 03:15 - v0.1.0 built and verified; release still a draft

Merged #10 (`03a84f5`). Release Please created tag `v0.1.0` and a **draft**
GitHub Release, and `release.yml` ran on the tag push: run 31989614368, all
eight jobs success - including `attest-binaries / Attest` and
`attest-image / Attest`, neither of which had ever executed before.

Then merged #31 (`065b9e8`), the README/SECURITY.md release pass.

Artifacts: four platform binaries + four SBOMs + `checksums.txt` on the draft
release; multi-arch image (linux/amd64 + linux/arm64) at
`ghcr.io/componere/incusos-builder:v0.1.0`, index digest
`sha256:e3bfe74884acbbd707bccca58e89d66ec542c63f365d0f5f73af45a6284b37de`.

**Verified as a consumer, with falsifiers, not by trusting the green run.**
This gh build (2.97.0) is silent on success, so a bare exit 0 proves nothing on
its own; each check was paired with a deliberate negative:

- `gh attestation verify` on the image index -> exit 0. Falsifier with
  `--source-ref refs/tags/v9.9.9` -> exit 1, `expected SourceRepositoryRef to
  be refs/tags/v9.9.9, got refs/tags/v0.1.0`. The failure text independently
  confirms the attested ref.
- `gh attestation verify` on `incusos-builder_0.1.0_darwin_arm64` -> exit 0;
  its sha256 matches `checksums.txt` exactly.
- `cosign verify` on the image -> exit 0, with transparency-log existence
  verified offline and the certificate chained to a trusted CA. Falsifier with
  a foreign identity regex -> exit 1, and the error revealed both real signing
  identities: `.github/workflows/attest.yml@refs/tags/v0.1.0` and
  `.github/workflows/release.yml@refs/tags/v0.1.0`.

**Not done, deliberately:** the GitHub Release is still a draft. `release.yml`
gates publication on human inspection ("Publish or reject the draft release
manually after inspection"), so I did not publish it. Consequence worth
knowing: the container image is already public - GHCR has no draft state - while
the binaries are not fetchable by anyone unauthenticated. The README merged in
#31 documents the `ghd` install path, so that path does not work for the public
until the draft is published. Window is open now and closes on the developer's
call.

Also open: `N-MEDIA-3`, and the `fix-after-v1` deferrals (F-CFG-1, N-ART-5,
N-APUB-2).

## 2026-08-17 03:25 — Close

Session 005 closed. Nothing was left unlanded: all seven pull requests raised in
this session are merged and `master` is fast-forwarded to `065b9e8`.

Merged: #21 (campaign remediation), #24 (first observed boot acceptance), #26
(release version baseline), #29 (changelog seed), #30 (dependency batch), #31
(README and SECURITY.md release pass), and #10 (`chore(master): release 0.1.0`).

Hand-off state:

- `v0.1.0` is tagged, built, signed, attested, and consumer-verified with
  falsifiers. Its **GitHub Release is still a draft** — publish with
  `gh release edit v0.1.0 --draft=false`. The container image at
  `ghcr.io/componere/incusos-builder:v0.1.0` is already public, so until the
  draft is published the README's `ghd` path does not work for the public.
- Dependabot alerts are at zero; `.github/repository-settings.toml` is applied.
- Still open: `N-MEDIA-3` (untested NUL-padded ISO label), and the
  `fix-after-v1` deferrals F-CFG-1, N-ART-5, N-APUB-2.

`SUMMARY.md` is the postmortem; `TECH_NOTES.md` gained the durable pieces —
release versioning mechanics, the corrected boot oracle, the draft/GHCR timing
split, and the verification falsifier rule.

Session 006 was running concurrently and was not touched by this closeout.
