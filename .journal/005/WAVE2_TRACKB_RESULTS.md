---
title: Wave 2 track B execution results
plan: .journal/005/FUNCTIONAL_TEST_PLAN.md
commit tested: 59c268b
executed: 2026-08-16
operator: Main + four functional-tester agents (BImage, BRehearsal, BRuntime, BSbom)
---

# Wave 2, track B — container image and release rehearsal

## Verdict

**9 of 9 cases executed. 9 pass, 0 fail, 0 blocked.**

The packaging and image chain delivers on every promise it makes: the apk is
genuinely signed and apko refuses it without the key, the container binary
carries the real commit and build date, the image runs as uid 65532 with the
contractual entrypoint, apko emits inspectable SPDX documents, syft describes
the assembled image, the documented CI snippets produce a byte-identical
envelope, and the release rehearsal is green with no publishing call anywhere in
its 2,083-line log.

| Case | Result | Headline evidence |
|------|--------|-------------------|
| SUP-03 image-local build | pass | `loaded incusos-builder:dev (host arch arm64)`, exit 0, **39 s** |
| SUP-04 signing negative control | pass | apko exits 1: `no signature with known key (one of: [wolfi-signing.rsa.pub]) found in repository index` |
| SUP-05 stamped version/commit/date | pass | `incusos-builder dev (59c268b) built 2026-08-16T20:05:16Z` — byte-exact vs `git rev-parse --short HEAD` + vars file |
| SUP-06 nonroot + entrypoint | pass | `.Config.User` = `65532`; entrypoint `["/usr/bin/incusos-builder"]`; runtime uid proven kernel-side |
| SUP-07 no repo residue | pass | `git status --porcelain --untracked-files=all` empty; all 9 generated paths matched by an explicit `.gitignore` rule |
| SUP-08 apko SBOM | pass | `sbom-aarch64.spdx.json` (SPDX-2.3, 19 pkgs), `sbom-index.spdx.json` (SPDX-2.3, 3 pkgs) |
| SUP-09 syft describes the image | pass | 174 packages; the 4 apk packages match apko exactly |
| SUP-10 run-in-ci snippets | pass | `cmp` against the documented golden → byte-identical; exactly one JSON document on stdout |
| SUP-12 release rehearsal | pass | run 31969505600 `success`, 4/4 jobs, artifacts `["apk-amd64","apk-arm64"]`, 9 staged assets, zero publishing calls |

Deliberate deviation from PRE-07-W2: **`root:e2e` was not run.** It is track A's
live pre-screen and nothing in track B depends on it.

## Strongest evidence produced

**SUP-04 — the signing guarantee is real, not incidental.** apko fails at index
signature verification, the strictest possible point, before any package is
extracted. Three independent corroborations that no cached keyring was in play:
`/Users/josh/.apko` and `~/Library/Caches/apko` do not exist; the error
enumerates the entire trusted set as exactly `[wolfi-signing.rsa.pub]`; and no
output tar and no `:untrusted` tag were produced.

**SUP-05 — `--vars-file` reaches `ldflags`.** The container prints
`dev (59c268b) built 2026-08-16T20:05:16Z`; the tester synthesised the expected
string from `git rev-parse --short HEAD` and `.melange-vars.local.yaml` and
`diff`ed it — byte-exact. The `incus-os API:` line survives `-buildvcs=false`
plus `strip: -s -w`, proving Go build info is intact. The uncontainerised control
correctly shows `dev (none) built unknown`.

**SUP-06 — runtime uid proven, not merely asserted.** The image is shell-less and
has no `id` binary (verified: `stat /usr/bin/id: no such file or directory`), so
the tester substituted kernel-enforced permission probes with paired negative
controls rather than settling for the metadata read. The runtime **gid** remains
metadata-only; that ceiling is stated honestly rather than papered over.

**SUP-10 — stdout purity holds through the container.** The success envelope is
`cmp`-identical to the documented golden, `jq -s length` = 1 on every run, and
the empty-stdin failure exits 3 with `error.code` 3.

**SUP-12 — the fidelity boundary is intact.** Grep over the full run log:
`gh release upload`, `apko publish`, `cosign sign`, `docker push`, `ghcr.io`,
`crane push`, `docker login` — **0 hits each**. The only `cosign`/`attest`
matches are PATH dumps, mise install rows, echoed comments, and Dependabot branch
names. Nine staged assets confirmed by name. Wall time 7m03s against the plan's
"~7 min"; the prior rehearsal (31963560870) took 6m33s.

## New findings

| ID | Severity | Finding |
|----|----------|---------|
| **F-SUP-3** | cosmetic | The container smoke line reports `built 2026-08-16T11:57:26-07:00` while the binary smoke line reports `built 2026-08-16T18:57:26Z` — the same instant, rendered differently. `release-dry-run.yml:139` uses `git show -s --format=%cI`, which emits the committer's local offset, while GoReleaser stamps UTC. `release.yml` uses the same construction, so a **published image will carry a non-UTC build timestamp while the published binaries carry UTC**. |
| **F-SUP-4** | observability | Three rehearsal assertions (image architectures, uid 65532, SBOM package count) are silent on success. A future regression is distinguishable from a pass only by the step's exit code; the log carries no positive evidence. The plan's suggested greps match only the echoed script bodies. |
| **F-SBOM-1** | supply chain | **No file in this repo pins the syft version.** All four `Install Syft` steps pin the action SHA (`anchore/sbom-action/download-syft@e22c389… v0.24.0`) with no `syft-version:` input, so the attested SBOM's contents and schema can drift with the action's default. Local reproduction is syft v1.43.0 / schema 16.1.3, recovered from build info because the `go install`-ed binary self-reports `[not provided]`. |
| **F-SBOM-3** | informational | The apk purl is `pkg:apk/unknown/incusos-builder@0.1.1-r0?arch=aarch64` — namespace `unknown`, because the package comes from the `@local ./packages` repo. A downstream scanner keying on purl namespace will not match a Wolfi-namespaced advisory feed. |
| **F-SBOM-2** | plan | `sbom-index.spdx.json` names no `incusos-builder` package — only two layer digests and the source repo. Correct apko behaviour; SUP-08's expectation must be scoped to the per-arch document. |
| **F-SBOM-4** | positive | The index SBOM records `versionInfo` `59c268b1499bd1a5ce94d487f1bc3adf377bead3`, matching HEAD — a provenance anchor tying the image to a commit independently of the ldflags stamp. |
| **F-IMG-A** | informational | `RepoDigests` is `[]` on the locally loaded image. The image ID is a config digest, not the manifest digest cosign signs — a later case must not try to verify `incusos-builder:dev` by digest. |
| **F-IMG-B** | informational | The OCI label `org.opencontainers.image.created=2026-08-12T15:22:42Z` is four days before the build. This is apko's reproducible-build behaviour, not a stamping bug, and does not contradict SUP-05. |
| **F-IMG-C** | positive | `/usr/bin/incusos-builder` is `-rwxr-xr-x 0 0`, so the runtime uid cannot modify the binary. |
| **F-DOC-9 extension** | medium | Beyond accepting plain http, **`validate` ignores `--server` entirely** — the envelope is byte-identical with and without a bogus server. Strengthens the Wave 1 finding. |

## Plan corrections

| ID | Correction |
|----|-----------|
| D-9 | SUP-03's cost is **~40 s**, not "5–12 min, 1–3 GB". The estimate was anchored on cold-CI numbers; a warm OrbStack VM builds it in 39 s. |
| D-10 | SUP-12's verification greps (`linux/(amd64\|arm64)`, `65532`) match only echoed script bodies. Replace with the step-conclusion assertion plus the observable proxies `Loaded image: incusos-builder:dry-run-amd64` / `-arm64`. |
| D-11 | SUP-08 should say the **per-arch** SBOM names `incusos-builder`; the index SBOM legitimately does not. |
| D-12 | SUP-09 cannot report a syft version from `--version` on a `go install`-ed binary; use `syft version -o json` plus the module build info. |

## Confirmations

- The apk-version / binary-stamp divergence (`incusos-builder-0.1.1-r0.apk` vs a
  binary stamped `dev`) is the documented deliberate behaviour, now confirmed
  **both locally and in CI** (`0.0.0-dryrun.31969505600.1`).
- F-SUP-1 (`moon.yml:37` phantom `.github/workflows.disabled/**`) is untouched by
  this run — the rehearsal does not go through moon, so it remains a static
  finding.
- Wave 1's F-CLI-7 does not bite here: `--version` and `--help` emit no step
  headers and wrote 0 bytes to stderr.

## Side effects left in place

Deliberately retained for any later case: `incusos-builder:dev` and
`incusos-builder:dev-arm64` (image `952969c4a012`, 45.2 MB) loaded in Docker;
`packages/aarch64/incusos-builder-0.1.1-r0.apk`, `image.tar`, `melange.rsa`,
`melange.rsa.pub`, `.melange-vars.local.yaml`, `sbom-aarch64.spdx.json`,
`sbom-index.spdx.json` in the repo root — all gitignored, all confirmed by
`git check-ignore -v`.

On GitHub: run 31969505600 with artifacts `apk-amd64` / `apk-arm64`
(`retention-days: 1`, self-expiring). The synthetic tag
`v0.0.0-dryrun.31969505600.1` existed only in the runner's ephemeral checkout and
was never pushed. Nothing published, signed, or applied.

All four testers verified `git status --porcelain` empty before and after.

---

# Appendix — verbatim slice reports

## Slice report: BImage (SUP-03, SUP-04, SUP-07)

All three cases executed with evidence. Final verification of end state is complete: image loaded, artifacts in place, tree clean.

## Slice: Wave 2 track B stage 1 — image build chain — 3 cases

**Repo:** `/Users/josh/code/componere/incusos-builder` @ `59c268b`, host arm64 (Apple M4 Max), Docker `29.4.0 OrbStack`.

### Results

| Case | Result | Observed evidence (verbatim, one line) | Deviation |
|---|---|---|---|
| SUP-03 | **PASS** | `loaded incusos-builder:dev (host arch arm64)` — exit 0; `packages/aarch64/incusos-builder-0.1.1-r0.apk` (14 236 709 B) + `APKINDEX.tar.gz` (964 B); `incusos-builder dev/dev-arm64 952969c4a012 45.2MB` | Runtime 39 s, not the 5–12 min budgeted |
| SUP-04 | **PASS** (enforcement proven) | `Error: locking config: resolving apk packages: for arch "arm64": error getting repository indexes: reading index ./packages/aarch64/APKINDEX.tar.gz: no signature with known key (one of: [wolfi-signing.rsa.pub]) found in repository index` — `exit=1` | none |
| SUP-07 | **PASS** | `git status --porcelain --untracked-files=all` → empty; all 9 generated paths matched by an explicit `.gitignore` rule | none |

### SUP-03 — detail

`mise run image-local` exited **0**. Chain observed in order: `melange keygen` (4096-bit) → melange **v0.54.0** docker-runner build → `wrote packages/aarch64/incusos-builder-0.1.1-r0.apk` → `signing index packages/aarch64/APKINDEX.tar.gz with key melange.rsa` / `appending signature RSA256 to index` → apko **v1.2.19** assembling `wolfi-baselayout`, `ca-certificates-bundle`, `incusos-builder (0.1.1-r0)`, `tzdata` → `Loaded image: incusos-builder:dev-arm64` → `loaded incusos-builder:dev (host arch arm64)`.

```
REPOSITORY        TAG         IMAGE ID       SIZE
incusos-builder   dev         952969c4a012   45.2MB
incusos-builder   dev-arm64   952969c4a012   45.2MB
```
Both tags point at the **same image ID** `952969c4a012`, as `docker tag` implies.

**Deliberate divergence confirmed, not a defect.** The apk filename carries `package.version` from `melange.yaml:10` → `incusos-builder-0.1.1-r0.apk`, and `provides: cmd:incusos-builder=0.1.1-r0`. The binary is stamped from the generated vars file:
```
version: "dev"
commit: "59c268b"
date: "2026-08-16T20:05:16Z"
```
`commit` equals `git rev-parse --short HEAD` (`59c268b`). This matches the known-findings list. In-container stamping proof is SUP-05's case; I did not run it.

Two non-blocking log observations, recorded but not raised as findings: melange emits `INFO melange version  with runner docker building [melange.yaml] at commit  for arches [arm64]` with **empty** version and commit fields on that one line (the next line correctly says `melange v0.54.0`), and `WARN git repository URL for build config not provided`. Both are melange-side cosmetics of an unpinned local build, not repo defects.

### SUP-04 — detail

Negative control, no `--keyring-append`:
```console
$ mise x -- apko build apko.yaml incusos-builder:untrusted /tmp/untrusted.tar --arch arm64; echo "exit=$?"
2026/08/16 13:06:17 WARN skipping signature .SIGN.RSA256.melange.rsa.pub due to missing keyfile: melange.rsa.pub
Error: locking config: resolving apk packages: for arch "arm64": error getting repository indexes: reading index ./packages/aarch64/APKINDEX.tar.gz: no signature with known key (one of: [wolfi-signing.rsa.pub]) found in repository index
exit=1
```
**Not a blocker — the guarantee holds.** Three independent corroborations that no cached or previously-trusted keyring was in play:
1. Pre-build sweep found **no apko cache at all**: `/Users/josh/.apko` and `~/Library/Caches/apko` both `No such file or directory`.
2. The error enumerates the entire trusted set as exactly `[wolfi-signing.rsa.pub]` — the only keyring entry in `apko.yaml:9-10`; the ephemeral melange key was absent, so the repo index was rejected outright.
3. No output was produced and nothing was loaded: `/tmp/untrusted.tar` → `No such file or directory`, and `docker image ls incusos-builder` still lists only `incusos-builder:dev` and `incusos-builder:dev-arm64` — no `:untrusted` tag.

Failure occurs at index-signature verification (before package extraction), which is the strictest possible point. The apk itself is genuinely signed — its first member is `.SIGN.RSA256.melange.rsa.pub`.

### SUP-07 — detail

`git status --porcelain --untracked-files=all` printed **nothing** both before and after the build. Every generated path resolves to an explicit ignore rule:

```
packages                                       -> .gitignore:25:/packages/	packages
packages/aarch64/incusos-builder-0.1.1-r0.apk  -> .gitignore:25:/packages/	packages/aarch64/incusos-builder-0.1.1-r0.apk
packages/aarch64/APKINDEX.tar.gz               -> .gitignore:25:/packages/	packages/aarch64/APKINDEX.tar.gz
image.tar                                      -> .gitignore:31:image.tar	image.tar
melange.rsa                                    -> .gitignore:27:melange*.rsa	melange.rsa
melange.rsa.pub                                -> .gitignore:28:melange*.rsa.pub	melange.rsa.pub
.melange-vars.local.yaml                       -> .gitignore:30:.melange-vars.local.yaml	.melange-vars.local.yaml
sbom-aarch64.spdx.json                         -> .gitignore:33:*.spdx.json	sbom-aarch64.spdx.json
sbom-index.spdx.json                           -> .gitignore:33:*.spdx.json	sbom-index.spdx.json
```
Sizes: `image.tar` 14 876 672 B, `melange.rsa` 3 243 B, `melange.rsa.pub` 800 B, `.melange-vars.local.yaml` 62 B, `sbom-aarch64.spdx.json` 19 250 B, `sbom-index.spdx.json` 4 250 B.

**No gitignore gap.** Notably the private signing key `melange.rsa` is ignored by `melange*.rsa` (`.gitignore:27`), so an ephemeral key cannot be committed by accident — the sharpest risk this case guards.

### New findings not already in the known list

**None.** No new defect surfaced in these three cases. Two items for the record, neither a defect:
- The apko SBOM filenames the plan says "the repo fixes nowhere" are, empirically, **`sbom-aarch64.spdx.json`** and **`sbom-index.spdx.json`**, written to the repo root. Handing this to whoever runs SUP-08.
- The plan's 5–12 min / 1–3 GB estimate for SUP-03 is pessimistic on this host: **39 s wall time**. Warm Go module cache and an M4 Max; the cost table could be relaxed but the estimate is not wrong for a cold machine.

Known findings touched: the apk-version-vs-`dev`-stamp divergence is **confirmed as deliberate** (evidence above). F-SUP-1 and F-SUP-2 were not in scope of these three cases and were not re-examined.

### Side-effect ledger

**Built:** the signed apk `packages/aarch64/incusos-builder-0.1.1-r0.apk`, the signed `APKINDEX.tar.gz`, the OCI tarball `image.tar`, two SPDX SBOMs, an ephemeral RSA keypair.

**Left on disk deliberately, for stage 2** — `packages/`, `image.tar`, `melange.rsa`, `melange.rsa.pub`, `.melange-vars.local.yaml`, `sbom-aarch64.spdx.json`, `sbom-index.spdx.json`.

**Left in Docker deliberately:** `incusos-builder:dev` and `incusos-builder:dev-arm64`, both image ID `952969c4a012`, 45.2 MB. Main was messaged the moment SUP-03 went green so stage 2 could begin.

**Stopped/removed:** only `/tmp/untrusted.tar`, pre-emptively `rm -f`'d *before* SUP-04 so its absence afterwards would be meaningful. apko never created it.

**Did NOT do:** no `docker rmi`, no deletion of any build artifact, no edit to any tracked file, no `cosign sign`, no publish, no tag push, no `apply` of the repository-settings script, no SUP-19..SUP-23. Did not run SUP-05/06/08/09/10/12 (other agents' cases) nor tracks A/C. `root:e2e` (PRE-07-W2) was **deliberately not run** per the batch brief — it is track A's pre-screen and nothing here depends on it; recorded as a deviation, not a skip.

### Repo cleanliness

```console
# BEFORE
$ git status --porcelain
(empty)
$ git rev-parse --short HEAD
59c268b

# AFTER (--untracked-files=all, the stricter form)
$ git status --porcelain --untracked-files=all
(empty)
```
Empty before and after. Working tree unmodified; all 14 residue entries appear only under `--ignored=matching` as `!!`.

### Time spent / cases blocked

~6 minutes total (build 39 s, negative control 0.6 s, remainder inspection). **Zero cases blocked**; 3 of 3 executed and passed.

## Slice report: BRehearsal (SUP-12)

## Slice: Wave 2 track B, stage 1 — SUP-12 release rehearsal — 1 case

**Run URL: https://github.com/componere/incusos-builder/actions/runs/31969505600** (`workflow_dispatch`, `headBranch: master`, commit `59c268b1499bd1a5ce94d487f1bc3adf377bead3`, created `2026-08-16T20:05:06Z`, `conclusion: success`). Distinguished from PR-branch runs by `event: workflow_dispatch` + `headBranch: master`; the three prior runs (31965826888, 31965303175, 31964005234) are all `event: pull_request` with `conclusion: skipped`.

### Results

| Case | Result | Observed evidence (verbatim, one line) | Deviation |
|---|---|---|---|
| SUP-12 — run conclusion | **PASS** | `{"conclusion":"success", ... "status":"completed"}` | none |
| SUP-12 — exactly four jobs, all success | **PASS** | `Binary Release Dry Run`/`Melange Build Dry Run (amd64)`/`Melange Build Dry Run (arm64)`/`Container Image Dry Run`, each `"conclusion":"success"` | none |
| SUP-12 — artifact list | **PASS** | `["apk-amd64","apk-arm64"]` | none |
| SUP-12 — nine staged assets | **PASS** | 9 `dist/release-assets/` lines (count verified `= 9`); see list below | none |
| SUP-12 — binary version smoke line | **PASS** | `incusos-builder 0.0.0-dryrun.31969505600.1 (59c268b1499bd1a5ce94d487f1bc3adf377bead3) built 2026-08-16T18:57:26Z` | none |
| SUP-12 — container asserts both platforms | **PASS (assertion), DEVIATION (evidence form)** | `Loaded image: incusos-builder:dry-run-amd64` / `Loaded image: incusos-builder:dry-run-arm64`; the `Verify image architectures` step emitted **no stdout** and concluded success | plan's grep `linux/(amd64\|arm64)` matches nothing |
| SUP-12 — nonroot uid 65532 | **PASS (assertion), DEVIATION (evidence form)** | `Smoke test dry-run image` step succeeded; the string `65532` appears in the log **only** as the echoed script body, never as emitted output | plan's grep `65532` matches only the script echo |
| SUP-12 — container version smoke line | **PASS** | `incusos-builder 0.0.0-dryrun.31969505600.1 (59c268b1499bd1a5ce94d487f1bc3adf377bead3) built 2026-08-16T11:57:26-07:00` | timestamp rendered with local offset, not `Z` — see F-SUP-3 |
| SUP-12 — syft SBOM, non-zero packages | **PASS (assertion), DEVIATION (evidence form)** | `Generate image SBOM (no attestation)` succeeded under `set -euo pipefail` with `jq -e '.packages \| length > 0' image.spdx.json > /dev/null`; syft/jq emitted no stdout, so no package count is observable | count not quotable from the log |
| SUP-12 — no publishing call | **PASS** | see absence table below — all zero | none |

**Overall verdict: SUP-12 PASS.** Every substantive assertion in the plan holds. Three of them are proven only by step exit status because the workflow's checks are silent-on-success; the plan's suggested `grep` commands do not surface them.

#### The nine staged assets, verbatim
```
dist/release-assets/checksums.txt
dist/release-assets/incusos-builder_0.0.0-dryrun.31969505600.1_darwin_amd64
dist/release-assets/incusos-builder_0.0.0-dryrun.31969505600.1_darwin_amd64.sbom.json
dist/release-assets/incusos-builder_0.0.0-dryrun.31969505600.1_darwin_arm64
dist/release-assets/incusos-builder_0.0.0-dryrun.31969505600.1_darwin_arm64.sbom.json
dist/release-assets/incusos-builder_0.0.0-dryrun.31969505600.1_linux_amd64
dist/release-assets/incusos-builder_0.0.0-dryrun.31969505600.1_linux_amd64.sbom.json
dist/release-assets/incusos-builder_0.0.0-dryrun.31969505600.1_linux_arm64
dist/release-assets/incusos-builder_0.0.0-dryrun.31969505600.1_linux_arm64.sbom.json
```
4 binaries + 4 `.sbom.json` + `checksums.txt` = 9, exactly as promised. The step also passed its `test -f .github/workflows/attest.yml` guard.

#### Fidelity boundary — no publishing call occurred

Grep over the complete 2083-line run log (`gh run view 31969505600 --log`):

| Pattern | Hits | Interpretation |
|---|---|---|
| `gh release upload` | 0 | absent |
| `apko publish` | 0 | absent |
| `cosign sign` | 0 | absent |
| `docker push` | 0 | absent |
| `ghcr.io` | 0 | absent |
| `crane push` | 0 | absent |
| `docker login` / `registry` / `gh release` / `release upload` | 0 | absent |
| `imagetools` | 1 | **not a call** — the echoed source comment `# inspect\`; a dry run cannot push, so assert the loaded images here.` |
| `attest-sbom` | 1 | **not a call** — the echoed comment `# The release attests this SBOM with actions/attest-sbom.` |
| `attest-build-provenance` | 4 | **not a call** — four `git fetch` lines, one per job: `* [new branch] dependabot/github_actions/actions/attest-build-provenance-4.2.2 -> origin/...` |
| `cosign` (all 23 hits) | 23 | **none is an invocation** — 20 are `PATH:` env dumps, 3 are `aqua:sigstore/cosign 3.1.1 ... mise.toml` (mise install table) plus GoReleaser's own `cosign not found in PATH, skipping signature verification` (that is goreleaser-action verifying *its own* download, not signing anything) |

Positive corroboration from GoReleaser itself:
```
[command]/opt/hostedtoolcache/goreleaser-action/2.17.1/x64/goreleaser release --clean --skip=publish
  • skipping announce and publish...
  • pipe skipped or partially skipped              reason=release is disabled
  • release succeeded after 6m23s
```

### Failures and deviations, in detail

1. **Deviation (plan text, not product): three assertions are unobservable in the log.** The plan expects the container job log to *show* `linux/amd64`, `linux/arm64`, `65532`, and a syft SBOM. It does not. `Verify image architectures` loops over `docker image inspect` and only `echo`s on mismatch; the uid check only `echo`s on mismatch; `syft`/`jq -e` write to a file and `/dev/null`. All three run under `set -euo pipefail` in a job that concluded `success`, so the assertions genuinely held — but the plan's verification command `grep -E 'linux/(amd64|arm64)|65532|incusos-builder 0.0.0-dryrun'` returns only the smoke line. **Recommended plan correction:** state that these are proven by step conclusion, or ask for the workflow to echo the observed values. Nearest observable proxies that *are* in the log: `Loaded image: incusos-builder:dry-run-amd64` and `Loaded image: incusos-builder:dry-run-arm64`.

2. **Deviation (timing estimate):** the plan says "~7 min". Measured wall time is **7m03s** — the estimate is accurate and needs no correction. Per-job breakdown below shows the binary job is the entire critical path (`go test ./...` before-hook 2m2s + build 4m14s), while the whole container chain finishes in 2m10s.

3. **Not a deviation, confirmed as expected:** the apk installed by apko is `incusos-builder (0.1.1-r0)` — `melange.yaml` `package.version` — while the binary inside it reports `0.0.0-dryrun.31969505600.1`. This is the known deliberate divergence, now confirmed to hold in CI as well as locally.

### New findings not already in the known list

- **F-SUP-3 (cosmetic, new).** The container smoke line reports `built 2026-08-16T11:57:26-07:00` whereas the binary smoke line reports `built 2026-08-16T18:57:26Z` — the same instant, rendered differently. Cause: `release-dry-run.yml:139` sets `date=$(git show -s --format=%cI HEAD)`, which emits the committer's local UTC offset, while GoReleaser stamps UTC `Z`. The real `release.yml` uses the same `%cI` construction, so a published image will carry a non-UTC build timestamp while the published binaries carry UTC. Harmless but inconsistent; `%cI` → `%cd --date=format-local:%Y-%m-%dT%H:%M:%SZ` with `TZ=UTC` would align them.
- **F-SUP-4 (observability, new).** The three silent assertions in §"Failures and deviations" item 1 mean a future regression in the arch/uid/SBOM checks is only distinguishable from a pass by the step's exit code — the log carries no positive evidence. This is the same class of gap the plan itself flags elsewhere; worth an `echo` of the observed value in each check.
- No refutation of any known finding. F-SUP-1 (`moon.yml:37` phantom `.github/workflows.disabled/**`) is untouched by this run — the rehearsal does not go through moon, so SUP-12 neither confirms nor refutes it; it remains a static finding.

### Per-job wall time, and comparison to the prior rehearsal

| Job | This run (31969505600) | Prior run (31963560870) | Δ |
|---|---|---|---|
| Binary Release Dry Run | 6m59s (20:05:09 → 20:12:08) | 6m29s (18:05:38 → 18:12:07) | +30s |
| Melange Build Dry Run (amd64) | 1m46s (20:05:09 → 20:06:55) | 1m36s (18:05:38 → 18:07:14) | +10s |
| Melange Build Dry Run (arm64) | 1m13s (20:05:11 → 20:06:24) | 1m17s (18:05:41 → 18:06:58) | −4s |
| Container Image Dry Run | 19s (20:06:57 → 20:07:16) | 26s (18:07:16 → 18:07:42) | −7s |
| **Run wall clock** | **7m03s** (20:05:06 → 20:12:09) | **6m33s** (18:05:35 → 18:12:08) | +30s |

Prior run was `workflow_dispatch` on `phase/6-release-readiness`; this one on `master`. Both `success`, both four jobs, timings within noise. The plan's "~7 min (remote runner)" stands. Internal binary-job breakdown from GoReleaser: `hook=go test ./...` took `2m2s`, `building binaries` took `4m14s`, `release succeeded after 6m23s`.

### Side-effect ledger

- **Built:** nothing locally. One remote GitHub Actions run (31969505600) on `master`.
- **Left on disk:** `/tmp/sup12-watch.txt` (watch transcript) and `/tmp/sup12-full.log` (2083-line full run log) — both outside the repo, retained as evidence.
- **Left on GitHub:** two artifacts `apk-amd64`, `apk-arm64` with `retention-days: 1`; they expire on their own. The synthetic tag `v0.0.0-dryrun.31969505600.1` exists only in the runner's ephemeral checkout (`git tag -f` with `persist-credentials: false`, never pushed) — `gh api` shows no such tag on the remote path and nothing was pushed.
- **Stopped:** nothing; the run completed on its own. No job was re-run — **zero re-runs, first attempt (`run_attempt: 1`) succeeded outright.**
- **Did NOT do:** dispatched no other workflow; no `apply` on repository-settings; nothing published; no tag pushed; no `cosign sign`; touched no SUP-19..SUP-23 case; ran no track A (ART/MED/DOC) or track C (BOOT) case; did not run `root:e2e` (PRE-07-W2 — deliberate deviation, it is track A's pre-screen and SUP-12 has no dependency on it); modified no repository file.

### Repo cleanliness

```console
$ git status --porcelain      # before, 2026-08-16T20:05:01Z
(empty; exit 0)
$ git status --porcelain      # after
(empty; exit 0)
$ git rev-parse --short HEAD
59c268b
```
Both empty. SUP-12 is a purely remote case and wrote nothing into the working tree.

### Time spent / cases blocked

Wall time end to end ≈ 9 minutes (dispatch 20:05:06, run complete 20:12:09, evidence collection ~1 min). `gh run watch` blocked for 408s. **Zero cases blocked.** SUP-12 is the only case in this stage-1 assignment and it is recorded as **PASS** with the two evidence-form deviations and two new findings noted above.

## Slice report: BRuntime (SUP-05, SUP-06)

## Slice: Wave 2 / Track B / Stage 2 — runtime stamping & nonroot posture — 2 cases

### Results

| Case | Result | Observed evidence (verbatim, one line) | Deviation |
|---|---|---|---|
| SUP-05 — stamped version/commit/date | **PASS** | `incusos-builder dev (59c268b) built 2026-08-16T20:05:16Z` / `incus-os API: v0.0.0-20260815030500-0f5b8057f2fc` — byte-exact vs. constructed expectation (`diff` clean, `BYTE-EXACT-MATCH`), exit 0 | none |
| SUP-06 — nonroot 65532 + entrypoint contract | **PASS** | `65532` / `["/usr/bin/incusos-builder"]` / `--help` exit 0, stderr 0 bytes; runtime uid proven kernel-side (see below) | none |

---

### SUP-05 — verbatim evidence

```console
$ git rev-parse --short HEAD
59c268b

$ cat .melange-vars.local.yaml
version: "dev"
commit: "59c268b"
date: "2026-08-16T20:05:16Z"

$ docker run --rm incusos-builder:dev --version
incusos-builder dev (59c268b) built 2026-08-16T20:05:16Z
incus-os API: v0.0.0-20260815030500-0f5b8057f2fc
EXIT:0
```

Byte-for-byte comparison (not eyeballed). Expectation was synthesized from `git rev-parse --short HEAD` and the `date:` value parsed out of `.melange-vars.local.yaml`, then `diff`ed against captured stdout:

```console
$ diff /tmp/bruntime/expected.txt /tmp/bruntime/ver.out && echo "BYTE-EXACT-MATCH"
BYTE-EXACT-MATCH
$ wc -c /tmp/bruntime/ver.out
     106 /tmp/bruntime/ver.out
$ cat /tmp/bruntime/ver.err
[end stderr]          # 0 bytes — both lines are on stdout
```

Component-by-component:

| Field | Source of truth | Container value | Match |
|---|---|---|---|
| `version` | `.melange-vars.local.yaml` `version: "dev"` | `dev` | ✅ |
| `commit` | `git rev-parse --short HEAD` = `59c268b` | `(59c268b)` | ✅ byte-for-byte |
| `date` | `.melange-vars.local.yaml` `date: "2026-08-16T20:05:16Z"` | `2026-08-16T20:05:16Z` | ✅ byte-for-byte, RFC 3339 UTC |
| incus-os pin | `internal/cli/pin.go` via `debug.BuildInfo` | `v0.0.0-20260815030500-0f5b8057f2fc` | ✅ |

**No `0.0.0`, `none`, or `unknown` appears in the container output.** The melange `vars:` defaults (`melange.yaml:24-27`: `version: "0.0.0"`, `commit: "none"`, `date: "unknown"`) were fully overridden, so `--vars-file .melange-vars.local.yaml` (`mise.toml:62-66`) did reach the `go/build` `ldflags` at `melange.yaml:38`. **This is not a release blocker.**

Second line proves `extra-args: "-mod=readonly -buildvcs=false"` plus `strip: "-s -w"` did **not** destroy Go build info: the pin is read from `debug.ReadBuildInfo().Deps`, which survives both. Had stripping killed it, `internal/cli/pin.go`'s fallback would have printed `unknown`.

**Uncontainerised control** (`go run`, no ldflags — the correct un-stamped shape):

```console
$ mise x -- go run ./cmd/incusos-builder --version
incusos-builder dev (none) built unknown
incus-os API: v0.0.0-20260815030500-0f5b8057f2fc
EXIT:0
```

The contrast is exactly the intended one: `main.commit`/`main.date` keep their compile-time defaults out of band (`none`/`unknown`), while the *container* binary carries the real values. The `incus-os API:` line is identical in both, since it comes from module graph metadata, not from `-X`.

---

### SUP-06 — verbatim evidence

```console
$ docker image inspect incusos-builder:dev --format '{{.Config.User}}'
65532

$ docker image inspect incusos-builder:dev --format '{{json .Config.Entrypoint}}'
["/usr/bin/incusos-builder"]

$ docker run --rm --entrypoint /usr/bin/incusos-builder incusos-builder:dev --help
EXIT:0
   stderr: 0 bytes
   stdout (first 12 lines):
Build seeded IncusOS installation media from a YAML config

Usage:
  incusos-builder [flags]
  incusos-builder [command]

Available Commands:
  build       Build seeded IncusOS installation media from a YAML config
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  init        Write a starter config.yaml
  validate    Validate a build configuration without fetching images
```

`.Config.User` is exactly `65532` (string compare, no whitespace) — matches `apko.yaml:34` `run-as: 65532` and satisfies the CI assertion at `.github/workflows/release.yml:400-402`. Entrypoint is a one-element array containing `/usr/bin/incusos-builder`, matching `apko.yaml:21-22`. Root help lands on **stdout**, stderr empty, exit 0.

#### Runtime uid: proven, not merely asserted

The image is distroless-style. There is no `id` and no shell — confirmed, not assumed:

```console
$ docker run --rm --entrypoint /usr/bin/id incusos-builder:dev
docker: Error response from daemon: failed to create task for container: failed to create shim task: OCI runtime create failed: runc create failed: unable to start container process: error during container init: exec: "/usr/bin/id": stat /usr/bin/id: no such file or directory
```
Identical `no such file or directory` failures for `/bin/id`, `/bin/sh`, `/busybox`, `/bin/busybox` (all exit 127).

So I proved the effective uid **through kernel permission enforcement**, using the entrypoint binary's own `init -o <path>` write, with paired negative controls. Ground truth from the image filesystem (`docker export | tar tvf`, throwaway container, removed):

```
drwxr-xr-x  0 0      0           0 Apr 17 12:17 home/
drwx------  0 65532  65532       0 Dec 31  1969 home/nonroot/
-rwxr-xr-x  0 0      0    43122814 Dec 31  1969 usr/bin/incusos-builder
```

`/home/nonroot` is mode **0700**, owner **65532:65532** — writable by uid 65532 or uid 0 and by nobody else.

| Probe | Command | Observed | Inference |
|---|---|---|---|
| A | `docker run --rm incusos-builder:dev init -o /uid-probe.yaml` | `write /uid-probe.yaml: open /uid-probe.yaml: permission denied` — EXIT:1 | default user is **not uid 0** |
| A-ctl | `docker run --rm --user 0 … init -o /uid-probe.yaml` | `wrote /uid-probe.yaml` — EXIT:0 | denial in A is uid-driven, not a read-only rootfs |
| C | `docker run --rm incusos-builder:dev init -o /home/nonroot/probe.yaml` | `wrote /home/nonroot/probe.yaml` — EXIT:0 | default user **owns** a 0700 dir owned by 65532 |
| C-ctl | `docker run --rm --user 65533 … init -o /home/nonroot/probe.yaml` | `write /home/nonroot/probe.yaml: open /home/nonroot/probe.yaml: permission denied` — EXIT:1 | a neighbouring uid is refused; C is not "any nonroot uid" |

A + A-ctl rule out root; C + C-ctl pin the writer to the owner of a 0700 directory owned by 65532. Together the effective runtime uid **is 65532**, enforced by the kernel, not just declared in OCI config.

Corroborating signal (HOME is derived from the passwd entry of the run-as user):

```console
$ docker run --rm incusos-builder:dev init --help | grep -- --cache-dir
      --cache-dir string   content-addressed download cache directory (default "/home/nonroot/.cache/incusos-builder")
$ docker run --rm --user 0 incusos-builder:dev init --help | grep -- --cache-dir
      --cache-dir string   content-addressed download cache directory (default "/root/.cache/incusos-builder")
```

**Honest limitation:** I could not print a literal numeric `uid=65532 gid=65532` line, because no `id`, shell, or any second executable exists in the image. The *gid* is therefore weaker than the uid: `/home/nonroot` is `drwx------` (group has no bits), so no probe distinguishes gid 65532 from any other gid. The gid claim rests on `apko.yaml:26-33` and `.Config.User` alone. The **uid** claim does not — it is proven behaviourally above.

The macOS bind-mount ownership probe was **inconclusive and is not counted as evidence**: `docker run -v /tmp/bruntime/probe:/work … init -o /work/config.yaml` wrote the file, but the host saw `uid=501 gid=0` because OrbStack's VirtioFS remaps bind-mount ownership to the host user. Reported for completeness only.

---

### Image identity

| Property | Value |
|---|---|
| Image ID (config digest) | `sha256:952969c4a0126846b96e08afb2f24dbed38e2414da3a3df0a0adec71ac6a9dcf` |
| Size | `45215604` bytes (45.2 MB) |
| Platform | `arm64/linux` |
| `RepoTags` | `["incusos-builder:dev","incusos-builder:dev-arm64"]` |
| `RepoDigests` | `[]` |
| Env | `PATH=/usr/local/sbin:/usr/local/bin:/usr/bin:/usr/sbin:/sbin:/bin`, `SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt` |
| Labels | `created=2026-08-12T15:22:42Z`, `title=incusos-builder`, `description=Build seeded IncusOS installation media from a YAML config`, `source=https://github.com/componere/incusos-builder`, `version=0.1.1` |

`incusos-builder:dev` and `incusos-builder:dev-arm64` **share one image ID** — `docker images` reports `952969c4a012 45.2MB` for both, and both appear in a single `RepoTags` array on one inspect record. `docker tag` in `mise.toml:69` aliased, it did not copy.

**No registry digest exists.** `RepoDigests` is empty because the image was `docker load`ed from `image.tar` and never pushed; a `sha256:…` manifest digest only materializes on push. The config digest above is the only stable identifier available locally. SUP-17/SUP-21 (cosign against a pushed digest) cannot be satisfied from this artifact.

---

### Failures and deviations, in detail

None. Both cases match the plan's stated expectations exactly, including the byte-level commit and date comparison.

---

### New findings not already in the known list

1. **F-IMG-A (informational, no action).** `RepoDigests` is `[]` for the locally loaded image — expected for `docker load`, but worth recording so a later case does not try to cosign-verify `incusos-builder:dev` by digest. The image ID is a *config* digest and is not the manifest digest cosign would sign.
2. **F-IMG-B (informational, positive).** OCI label `org.opencontainers.image.created=2026-08-12T15:22:42Z` is **not** the build time — the build ran at `2026-08-16T20:05:16Z` per `.melange-vars.local.yaml`. This is apko's deterministic/reproducible SOURCE_DATE_EPOCH behaviour, not a stamping bug, and it does not contradict SUP-05 (which reads the binary's `-X main.date`, correct). Flagging only because the two timestamps sit four days apart and a reader comparing them could mistake it for a defect. `apko.yaml:36-44` declares no `created` annotation, so this value is apko-derived.
3. **F-IMG-C (informational, positive posture).** `/usr/bin/incusos-builder` is `-rwxr-xr-x 0 0` (root-owned, 43,122,814 bytes) and therefore not writable by the runtime uid — the binary cannot self-modify. Strengthens the nonroot claim.
4. **Gid unverifiable at runtime** (see limitation above) — not a defect, an evidence ceiling of a shell-less image. If a stronger gid guarantee is ever wanted, adding a group-writable, group-65532-owned directory would make it probeable; I am **not** recommending that change, just recording why the gid assertion is metadata-only.

Neither known finding applied here: F-DOC-9 concerns `validate --server` (SUP-10, not my scope), and F-CLI-7 concerns `==> step` headers — `--version` and `--help` emit no step headers, and both wrote 0 bytes to stderr, so stdout purity was clean in both my cases.

---

### Side-effect ledger

| Action | Effect | Reverted? |
|---|---|---|
| `docker run --rm …` × 14 | ephemeral containers, all auto-removed | n/a — `--rm` |
| `docker create` + `docker export` + `docker rm -f` | one throwaway container `d5956dfe8852…` created to read filesystem ownership | **yes**, removed (`throwaway container removed: d5956dfe8852…`) |
| `mise x -- go run ./cmd/incusos-builder --version` | populated Go build cache under `$HOME`; no repo write | n/a |
| Scratch files under `/tmp/bruntime/` (`ver.out`, `ver.err`, `expected.txt`, `help.out`, `help.err`, `probe/config.yaml`) | outside the repo | left in place, harmless |
| Files written **inside** containers (`/uid-probe.yaml`, `/home/nonroot/probe.yaml`, `/work/config.yaml`) | destroyed with the containers | n/a |

**Nothing rebuilt, nothing deleted.** No `docker rmi`, no `rm` of `packages/`, `image.tar`, `melange.rsa*`, `.melange-vars.local.yaml`, or any `*.spdx.json`. `mise run image-local` was not re-run. Both `incusos-builder:dev` and `incusos-builder:dev-arm64` remain loaded and unchanged (verified after all probes — same ID `952969c4a012`). No publish, sign, push, or repository-settings invocation. `BImage` and `BRehearsal` state untouched.

---

### Repo cleanliness

```console
# before
$ git status --porcelain
(no output)

# after
$ git status --porcelain
(no output)
```

Both empty. No tracked repository file was read-modified; all reads were read-only, all scratch output went to `/tmp/bruntime/`.

---

### Time spent / cases blocked and why

Roughly 5 minutes wall clock; ~20 commands. **Zero cases blocked.** Both SUP-05 and SUP-06 executed fully against the live image.

One sub-assertion of SUP-06 could not be proven to the letter — a literal `uid=65532 gid=65532` printout — because the image contains no `id` and no shell, which is itself the intended posture. Per the assignment's fallback instruction I did not stop at metadata: I substituted kernel-enforced permission probes with paired negative controls, which establish the runtime **uid** as 65532 more strongly than a metadata read. The runtime **gid** remains proven only by `.Config.User` (`65532`), `apko.yaml:26-33`, and the CI assertion at `.github/workflows/release.yml:396-402`.

## Slice report: BSbom (SUP-08, SUP-09, SUP-10)

## Slice: Wave 2 / track B stage 2 — SBOM + CI-snippet verification — 3 cases

### Results

| Case | Result | Observed evidence (verbatim, one line) | Deviation |
|---|---|---|---|
| SUP-08 — apko per-build SBOM | **PASS** | `-rw-r--r-- 1 josh staff 19250 Aug 16 13:05 sbom-aarch64.spdx.json` / `-rw-r--r-- 1 josh staff 4250 Aug 16 13:05 sbom-index.spdx.json`; `SPDX-2.3` + `19` and `SPDX-2.3` + `3` | Index SBOM's 3 packages do **not** include a literal `incusos-builder` (see D-1) |
| SUP-09 — syft describes the image | **PASS** | `syft [not provided]`; `jq -e '.packages \| length > 0'` → `true`, count `174`; apk set = `ca-certificates-bundle 20260413-r1`, `incusos-builder 0.1.1-r0`, `tzdata 2026c-r0`, `wolfi-baselayout 20230201-r29` | syft self-reports `[not provided]` as its version (see D-2) |
| SUP-10 — `run-in-ci.md` snippets through the image | **PASS** | `cmp golden.txt v1.out` → `cmp-exit=0 (0 = byte-identical)`; empty-stdin run `exit=3` with `"code":3`; every stdout `jq -s length` = `1` | None |

---

### Failures and deviations, in detail

**No case failed.** Two deviations from the plan's literal wording:

**D-1 — `sbom-index.spdx.json` does not name `incusos-builder`.**
The plan says "names `incusos-builder` among its packages". That holds for the arch SBOM but not the index SBOM. Verbatim package list of `sbom-index.spdx.json` (3 packages):

```
PKG sha256:19dc7bb5aee8938689ce60142920c79507c0826586e8a5b1332cf5677b0932bc sha256:19dc…
PKG sha256:bd1ee3167106652d1ee54515afd1c374263fdab2871a152b3c22aa8c5984a449 sha256:bd1e…
PKG github.com/componere/incusos-builder.git 59c268b1499bd1a5ce94d487f1bc3adf377bead3
```

This is correct apko behavior — the index SBOM describes the image index (manifest digests + source repo), not its contents; the per-arch SBOM carries the package inventory. The `versionInfo` on the third entry is `59c268b1499bd1a5ce94d487f1bc3adf377bead3`, which matches HEAD `59c268b` exactly — an independent confirmation that the loaded image was built from this tree. Not a defect; the plan's expectation should be scoped to the per-arch document.

`sbom-aarch64.spdx.json` does name it:

```
PKG incusos-builder 0.1.1-r0 pkg:apk/unknown/incusos-builder@0.1.1-r0?arch=aarch64
```

Full 19-name list: 2 sha256 layer/image entries, `wolfi`, then `wolfi-baselayout` ×3 + `wolfi-baselayout.yaml`, `ca-certificates-bundle` ×3 + `ca-certificates.yaml` + `alpine/ca-certificates`, `incusos-builder` + `melange.yaml`, `tzdata` ×3 + `tzdata.yaml` + `tz`. All four `apko.yaml:16-19` packages are present. Both documents parse under `jq` (`jq-exit=0`) and declare `spdxVersion` `SPDX-2.3`. Both sit next to `image.tar` (all three stamped `Aug 16 13:05`), confirming the "no `--sbom-path` ⇒ repo root" claim.

**D-2 — syft's self-reported version is `[not provided]` (version skew is unmeasurable from `--version` alone).**
Verbatim:

```
$ syft --version
syft [not provided]
$ syft version -o json
{ "application": "syft", "buildDate": "[not provided]", "compiler": "gc",
  "gitCommit": "[not provided]", "gitDescription": "[not provided]",
  "goVersion": "go1.26.2", "platform": "darwin/arm64",
  "schemaVersion": "16.1.3", "version": "[not provided]" }
```

The local binary is `/Users/josh/.go/bin/syft`, installed via `go install` without release ldflags, so every version field is unstamped. I recovered the real version from the build info: `mod github.com/anchore/syft v1.43.0`, SPDX schema `16.1.3`, built with `go1.26.2`.

CI does not pin a syft version either. `release.yml:109`, `release.yml:311`, `release-dry-run.yml:51`, `release-dry-run.yml:209` all use `anchore/sbom-action/download-syft@e22c389904149dbc22b58101806040fa8d37a610 # v0.24.0` with **no `with: syft-version:` input**, so CI gets whatever syft version that action release defaults to. The action SHA is pinned; the tool it installs is not. Local `v1.43.0` versus the action's default is therefore an unpinned, floating skew on both ends — the SBOM's package inventory and schema version can drift between a developer's machine and CI without any file in this repo changing. Worth recording as a supply-chain observation; the `syft-version` input on `download-syft` would close it.

**Count discrepancy, SUP-09 vs SUP-08 (174 vs 19) — explained, not a defect.**
The apko SBOM (19) is a *package-manager* view: it enumerates apk packages plus their build-definition files. Syft (174) additionally catalogs the Go module graph embedded in `/usr/bin/incusos-builder` by the `go-module-binary` cataloger — `charm.land/bubbletea/v2 v2.0.2`, `cloud.google.com/go/kms v1.25.0`, the whole AWS/Azure/GCP SOPS KMS dependency set, and so on. The 4 apk packages syft reports are exactly the 4 apko reports; the remaining ~170 are Go modules apko has no visibility into. The two documents are complementary, which is why the release attests the syft one.

Bonus cross-check from the syft output — three `incusos-builder`-matching entries:

```
github.com/componere/incusos-builder	UNKNOWN
incusos-builder	0.1.1-r0
incusos-builder	dev
```

The last line is the Go main-module version stamp of the *containerised* binary, reported as `dev`. This is consistent with the known local-build divergence (apk filename carries `package.version` `0.1.1`; the binary is stamped from `--vars-file`). SUP-05 owns the authoritative stamp check via `version --json`; I did not re-run it.

**SUP-10 — full evidence.**

`init --no-input -o -` (exit 0): 28 lines on stdout, **0 bytes on stderr**, no `wrote` line (`grep -c wrote` → `0`, `grep-exit=1`). Output begins `# Generated by incusos-builder init --no-input.` and contains the live `version: 1` / `image:` block plus the commented `#seeds:` catalogue. Note that `--no-input` was passed explicitly *and* would have been auto-on regardless — `docker run` here is non-TTY, so the prompt-disable condition in `run-in-ci.md` §1 is satisfied twice over; the snippet is honest but not load-bearing in this environment.

Happy-path validate (exit 0), stdout = 79 bytes / 1 line:

```
{"result":{"valid":true,"type":"iso","architecture":"x86_64","offline":false}}
```

Byte-exactness against the doc golden is *proven*, not eyeballed: I wrote `run-in-ci.md:89` / `automation.md:103` verbatim to `golden.txt` and ran `cmp`:

```
cmp-exit=0 (0 = byte-identical)
8a79b5f11dacb71f7fae7ff24c7a8970  golden.txt
8a79b5f11dacb71f7fae7ff24c7a8970  v1.out
```

stderr was 0 bytes. Trailing newline present (79 bytes = 78 chars + `\n`), matching "terminated by a newline" at `run-in-ci.md:64`.

Empty-stdin validate, **exit=3**, stdout 132 bytes / 1 line:

```
{"error":{"code":3,"message":"invalid config: yaml: construct errors: \u003cunknown position\u003e: yaml: no documents in stream"}}
```

stderr 89 bytes / 1 line, the message reprinted un-escaped:

```
invalid config: yaml: construct errors: <unknown position>: yaml: no documents in stream
```

`error.code` is `3` and the process exit is `3` — the `automation.md:39-51` identity holds through the image. `3` is the documented "Invalid seed config" code (`run-in-ci.md` §6 table). Note the stdout envelope carries `\u003c`/`\u003e` for `<`/`>` (Go's default HTML-escaping JSON encoder) while stderr carries the raw characters; a consumer diffing the two strings byte-for-byte would see a difference. That is correct JSON and correct behavior — parse stdout, don't scrape stderr, exactly as the doc instructs.

**Stdout purity proven, not assumed.** For both validate runs: `wc -l` = 1, `jq -e .` exits 0, and `jq -s 'length'` = **1**, which is the strong form — `jq -e .` alone would happily accept two concatenated documents, `jq -s length` would report `2`. Both reported `docs=1`. No progress line, no log line, no `==> step` header reached stdout in any container invocation.

Following up on the F-CLI-7 concern: `==> step` headers never appeared at all here, on either stream. Non-`--json` validate emits `configuration valid` on stdout with 0 bytes on stderr; `--json -q` emits the identical 79-byte envelope with 0 bytes on stderr. `validate` performs no staged work, so it has no step headers to emit — F-CLI-7 remains a `build`-path concern and does not threaten the SUP-10 stdout contract.

**F-DOC-9 through the container — confirmed identical.**

```
$ docker run --rm -i incusos-builder:dev validate --json -f - --color never --server http://example.invalid/os < config.yaml
exit=0
{"result":{"valid":true,"type":"iso","architecture":"x86_64","offline":false}}
```

Contrast, same image, same flag value:

```
$ docker run --rm incusos-builder:dev versions --json --color never --server http://example.invalid/os
exit=2
{"error":{"code":2,"message":"usage error: --server \"http://example.invalid/os\": plain http is not supported; use https or a local mirror directory"}}
```

The containerised binary reproduces Wave 1's F-DOC-9 exactly: `validate` silently accepts a plain-http `--server` and returns the success envelope at exit 0, while `versions` rejects the same value with exit 2 and the "plain http is not supported" usage error. Expected — it is the same binary — and it confirms the finding is in the CLI's flag-validation wiring, not in any host-specific path. Also note `validate`'s envelope is unchanged by the bogus `--server`, i.e. the flag is parsed and then wholly ignored on that command; a CI job that typos its mirror URL gets a green `validate` and discovers the problem only at `build`.

### New findings not already in the known list

- **F-SBOM-1 (low, supply chain).** Neither `release.yml` nor `release-dry-run.yml` pins the syft *version*; all four `Install Syft` steps pin only the action SHA (`anchore/sbom-action/download-syft@e22c389…` v0.24.0) with no `syft-version` input. The attested SBOM's contents and schema version can therefore drift with the action's default. Local reproduction is v1.43.0 / schema 16.1.3.
- **F-SBOM-2 (informational).** The apko index SBOM (`sbom-index.spdx.json`) lists no `incusos-builder` package — only two layer digests and `github.com/componere/incusos-builder.git`. Any tooling or doc that asserts "the SBOM names the binary" must target `sbom-*.spdx.json` per-arch, not the index. The test plan's SUP-08 expectation should be narrowed accordingly.
- **F-SBOM-3 (informational).** The apk purl in the apko SBOM is `pkg:apk/unknown/incusos-builder@0.1.1-r0?arch=aarch64` — namespace `unknown`, because the package comes from the `@local ./packages` repository rather than an identified distro. Harmless locally; if a downstream scanner keys on purl namespace, the locally-built package will not match a Wolfi-namespaced advisory feed.
- **F-SBOM-4 (informational, positive).** `sbom-index.spdx.json` records `versionInfo` `59c268b1499bd1a5ce94d487f1bc3adf377bead3`, matching HEAD `59c268b`. This is a usable provenance anchor tying the image to a commit independently of the binary's ldflags stamp.
- **F-CLI-x note (extends F-DOC-9).** Beyond accepting plain http, `validate` ignores `--server` entirely — the returned envelope is byte-identical with and without a bogus server. Recorded here because it strengthens the existing F-DOC-9 write-up rather than opening a new one.

### Side-effect ledger

Nothing was rebuilt, deleted, published, signed, or pushed.

- **Read-only** against the repo: `sbom-aarch64.spdx.json`, `sbom-index.spdx.json`, `image.tar` (stat only), `apko.yaml`, `docs/docs/how-to/run-in-ci.md`, `docs/docs/reference/automation.md`, both release workflows, the test plan.
- **Docker:** 7 × `docker run --rm` against `incusos-builder:dev` (init ×1, validate ×5, versions ×1). All `--rm`, all exited; no containers left behind. No `docker rmi`, no `docker build`, no `docker pull`. Both tags `incusos-builder:dev` and `incusos-builder:dev-arm64` (image `952969c4a012`, 45.2MB) remain loaded and untouched for `BImage` / `BRehearsal`.
- **syft:** one read-only `docker:` scan; wrote only to `/tmp/bsbom/`.
- **Scratch files, all under `/tmp/bsbom/`:** `image.spdx.json`, `syft.err`, `init.out`, `init.err`, `config.yaml`, `v1.out`, `v1.err`, `v2.out`, `v2.err`, `s1.out`, `s1.err`, `s2.out`, `s2.err`, `h.out`, `h.err`, `q.out`, `q.err`, `golden.txt`. Nothing written inside the repository. Note that the plan's SUP-09/SUP-10 command text writes to bare `/tmp/image.spdx.json` and `/tmp/config.yaml`; I redirected everything under `/tmp/bsbom/` to avoid colliding with concurrently running siblings.
- No network egress beyond the `docker run` attempts to `http://example.invalid/os`, which the CLI rejected (or ignored) before any request was made.

### Repo cleanliness

```
git status --porcelain   (before)
<no output>

git status --porcelain   (after)
<no output>
```

Both empty; exit 0 in both cases. No tracked file was modified. Build artifacts (`packages/`, `image.tar`, `melange.rsa`, `melange.rsa.pub`, `.melange-vars.local.yaml`, `sbom-aarch64.spdx.json`, `sbom-index.spdx.json`) remain in place and gitignored, exactly as handed over.

### Time spent / cases blocked and why

Roughly 6 minutes of wall time; no case blocked, none skipped. SUP-09's documented escape hatch ("skip and record if syft is unavailable") was not needed — syft was present at `/Users/josh/.go/bin/syft`, though unstamped, which is itself the finding recorded as D-2.

**Verdict: SUP-08 PASS, SUP-09 PASS, SUP-10 PASS.** The three headline promises hold through the real image: apko emits inspectable per-build SPDX-2.3 documents in the repo root, syft describes the assembled image with a non-empty package set including the binary and all four Wolfi base packages, and the `run-in-ci.md` success envelope is byte-identical to the documented golden with provably exactly one JSON document on stdout and the error code equal to the exit code.
