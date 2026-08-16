---
title: incusos-builder manual functional test plan
subject: componere/incusos-builder @ master 5337e7e
date: 2026-08-16
session: 005
status: draft for review
---

# incusos-builder — Manual Functional Test Plan (v1 release gate)

## 1. Purpose and scope

This plan is the last human gate before the first `incusos-builder` release. An
operator executes it by hand against real inputs — the live IncusOS update
server, real YAML configs, real multi-gigabyte artifacts, a real container
image, and the real GitHub release machinery — and at the end can state, with
recorded evidence, which promises the project keeps and which it does not.

"Release ready" here means all four of these, separately signed off:

1. **Every documented promise is observed.** Each user-visible claim in
   `README.md`, `docs/docs/**`, `SECURITY.md`, and the CLI's own help text maps
   to at least one executed test case with a recorded result.
2. **Artifacts are correct under third-party tools.** The seeded image, the
   seed tar, and the rescue media are verified with `dd`, `tar`, `shasum`,
   `bsdtar`, and macOS `hdiutil`/`diskutil` — not only with the project's own
   Go code, which is what the automated suites already do.
3. **The supply chain works or is honestly labeled.** Pre-tag: the image builds,
   runs nonroot, and the release rehearsal is green. Post-tag: signatures,
   attestations, SBOMs, and checksums verify with the exact commands the docs
   hand to consumers.
4. **The unproven path is named, not hidden.** IncusOS seed consumption and
   recovery acceptance have never been observed in any environment
   (`docs/notes/phase-5-boot-probe.md:44,49-54`). The plan either closes that
   gap on a real Linux host or records it as an accepted, owned risk.

### What this plan is not

- It is not a new automated suite. Every step is typed in a terminal or observed
  in a browser.
- It does not re-implement existing automated coverage. `moon run root:check`
  (testscripts under `internal/cli/testdata/script/`) and the opt-in
  `moon run root:e2e` (`INCUSOS_BUILDER_E2E=1`, `runInCI: false`, `moon.yml:97-107`)
  already cover exit codes, envelope shapes, and a live build. They are run once
  in preflight as a **pre-screen**: if they fail, stop — the manual cases will
  fail too. The manual cases exist because they use readers and hosts the Go
  suites cannot vouch for.
- It does not require every case to pass. Several cases are written to **falsify**
  a documented claim; those are listed in §5 and are decisions, not surprises.

### Execution waves

Cases are split by wall time into two waves that share nothing but the
repository and the operator's judgement. **Any case that takes more than five
minutes lives in Wave 2**, together with every cheap case that can only run
against a Wave 2 artifact. Wave 1 therefore runs start to finish on a laptop
with no multi-gigabyte downloads and no Docker.

| Wave | Content | Cases | Wall time | Disk | Detached? |
|------|---------|-------|-----------|------|-----------|
| **Wave 1** — fast lane | Every case ≤5 min: full CLI contract, config/SOPS, all usage-error and metadata-rejection paths, synthetic mirrors, docs site, repository/release-plumbing reads | 74 | 2.5–4 h | <500 MB | — |
| **Wave 2** — long lane | Every case >5 min plus its dependants: the live build chain, rescue media, the container image, the release rehearsal, the two full doc walkthroughs, and boot acceptance | 49 | 6–10 h local + 3–8 h on a Linux host | ~25 GB local, ~40 GB on the boot host | yes — own shell, own `$WORK`, own cache |
| **Wave P** — post-tag | Signature, attestation, SBOM, checksum, and asset verification against the first real release | 5 | ~1 h | ~1 GB | yes — after the tag, before publishing the draft |

**The detachment contract.** Wave 2 has no dependency on Wave 1 and Wave 1 has
no dependency on Wave 2. Each wave needs only PRE-01..PRE-06; Wave 2 then runs
its own bootstrap (PRE-07-W2). Nothing in Wave 1 reads an artifact Wave 2
produced, and nothing in Wave 2 reads state Wave 1 left behind. They may run on
different days, in different terminals, or on different machines, in either
order, or simultaneously — the only shared resource is network bandwidth to the
update server. Wave 2 track C additionally needs a host Wave 1 never touches.

Suggested schedule:

- **Wave 1 — half a day.** PRE-01..06, then CLI, CFG, the cheap ART/MED/SUP/DOC
  cases, BOOT-01. Produces every contract-level finding.
- **Wave 2 — one to two days, started whenever convenient.** Dispatch SUP-12
  (the GitHub rehearsal) first so it runs remotely while local work proceeds,
  then run track A, then track B; track C needs the Linux host.
- **Wave P — after tagging**, before the draft release is published.

Wave 2 is where the money is spent: 15 of its cases each exceed five minutes,
but they share one warmed cache, so the real bill is roughly one 415 MiB asset
download plus ~25 GB of transient disk, not 15 independent downloads.

---

## 2. Preflight

Every check below is a gate: a failure here invalidates downstream cases rather
than producing a finding.

### PRE-01 — Host, toolchain, and mise lock

```console
$ uname -m; sw_vers -productVersion
$ cd /Users/josh/code/componere/incusos-builder
$ git rev-parse --short HEAD          # expect 5337e7e (or record the tested commit)
$ git status --porcelain              # expect empty
$ mise install
$ mise ls --current
$ mise tasks
```

Expected: `arm64` on darwin. `mise ls --current` shows the nine pins from
`mise.toml:17-37`: go 1.26.4, python 3.14.3, uv 0.11.0, moon 2.3.5,
golangci-lint 2.12.2, mockery 3.7.3, melange 0.54.0, apko 1.2.19, cosign 3.1.1.
`mise tasks` prints exactly one row, `image-local`. `mise.toml:48-49` sets
`lockfile`/`locked`, so a platform without a locked URL fails closed.

Pass/Fail: [ ]

### PRE-02 — Environment hygiene

```console
$ env | grep '^INCUSOS_BUILDER_'      # expect no output
$ env | grep '^SOPS_'                 # expect no output
$ echo "CI=[${CI-unset}]"             # expect CI=[unset]
$ test -t 0 && test -t 1 && echo tty  # expect tty
$ whoami                              # must not be root
$ umask                               # record it; MED-16 needs a non-default umask
```

Any stray `INCUSOS_BUILDER_*` invalidates most CLI cases
(`internal/cli/root.go` `applyViperDefaults`). A non-empty `CI` silently disables
every prompt (`internal/cli/policy.go` `autoNoInput`), which would make CLI-17,
CLI-18, CLI-19, CFG-12 prove nothing. Running as root breaks CLI-09, which
depends on `permission denied` in a `chmod 555` directory.

Pass/Fail: [ ]

### PRE-03 — Third-party inspection tools

```console
$ which -a jq shasum bsdtar hdiutil diskutil od stat cmp curl python3 dd tr
$ command -v sops && sops --version --disable-version-check
$ command -v syft && syft --version
$ command -v sha256sum || echo 'sha256sum MISSING (expected on macOS; use shasum -a 256)'
$ docker info --format '{{.ServerVersion}} {{.OperatingSystem}} {{.Architecture}}'
$ gh auth status
```

Required: `jq`, `shasum`, `bsdtar` (libarchive — the Rock Ridge reader for
MED-12), `hdiutil`, `diskutil`, `od`, `stat`, `cmp`, `curl`, `python3`.
`sops` is **not** mise-pinned; without it, skip CFG-14 and rely on the committed
fixtures. `syft` is **not** mise-pinned (CI gets it from
`anchore/sbom-action/download-syft`, `release.yml:310-311`); without it, skip
SUP-09. A Docker-API daemon is **mandatory** for SUP-03..SUP-10 because melange
needs a Linux sandbox (`mise.toml:51-54`). `gh` must be authenticated with
`repo`, `workflow`, `read:packages`.

`isoinfo` and `xorriso` are absent on this host and are **not** required —
`bsdtar` is the independent ISO reader.

Pass/Fail: [ ]

### PRE-04 — Network and disk

```console
$ curl -sfI https://images.linuxcontainers.org/os/index.json | head -1
$ df -h /Users/josh
```

Expected: `HTTP/2 200`. Free space ≥ 25 GB for the ART + MED blocks run in one
session (one 415 MiB cached asset, several 3.2 GiB images, a 276 MB rescue
image, and a transient 2× peak during `--force` cases).

Pass/Fail: [ ]

### PRE-05 — Build the subject binary

Every case in this plan uses the compiled binary, never `go run` — `go run`
swallows the process exit code and emits an extra `exit status N` line
(finding F-DOC-1, §5).

```console
$ mise x -- moon run root:build
$ export REPO="$PWD"
$ export IOB="$PWD/bin/incusos-builder"
$ "$IOB" --version
$ git status --porcelain      # expect empty; bin/ is gitignored (.gitignore:36)
```

Expected: two stdout lines,
`incusos-builder dev (none) built unknown` and
`incus-os API: v0.0.0-20260815030500-0f5b8057f2fc`.
That second string must match `go.mod:14` and `internal/config/schema.go:17`;
it is embedded in the unknown-field errors asserted by CFG-08.

Use `mise x --` for every moon/golangci invocation: a global `moon` and a global
`golangci-lint 2.11.4` shadow the pinned versions.

Pass/Fail: [ ]

### PRE-06 — Automated pre-screen (shared, ≤5 min)

```console
$ mise x -- moon run root:check
```

Expected: exit 0. `root:check` runs format, lint, build, test, check-upstream,
docs:build (`moon.yml:110-116`). A failure here means stop; do not start either
wave. The live suite `root:e2e` is **not** run here — it downloads multi-gigabyte
assets and belongs to Wave 2's bootstrap (PRE-07-W2).

Pass/Fail: [ ]

### PRE-07-W1 — Wave 1 session scaffold

```console
$ export WORK=$(mktemp -d) && cd "$WORK"
$ export CACHE="$WORK/cache" && mkdir -p "$CACHE"
$ mkdir -p "$WORK/mirror"        # an empty but valid --server directory
$ printf 'version: 1\nimage:\n  type: raw\n  architecture: x86_64\n  channel: stable\n' > good.yaml
$ printf 'version: 1\nimage:\n  type: disk\n  architecture: x86_64\n' > bad.yaml
$ printf 'version: 1\nimage:\n  type: raw\n  architecture: x86_64\n  channel: stable\n  offline: true\nseeds:\n  applications:\n    applications:\n      - name: incus\n' > offline.yaml
$ echo "$WORK"
```

`good.yaml` and `bad.yaml` are the literal fixtures from
`internal/cli/testdata/script/exit_codes.txtar`; `offline.yaml` is from
`build_offline.txtar`. Record `$WORK` in the results log — every Wave 1 case
refers to it. This directory never exceeds a few hundred megabytes; nothing in
Wave 1 downloads an image.

Pass/Fail: [ ]

### PRE-07-W2 — Wave 2 bootstrap (separate shell, separate `$WORK`)

Run this **only** when starting Wave 2. It is deliberately independent of
PRE-07-W1: a different scratch tree, a different cache, and its own fixtures, so
the two waves cannot contaminate each other's evidence.

```console
$ export WORK2=$(mktemp -d) && cd "$WORK2"
$ export CACHE="$WORK2/cache" && mkdir -p "$CACHE"
$ cp "$WORK/good.yaml" "$WORK/bad.yaml" "$WORK/offline.yaml" . 2>/dev/null || {
    printf 'version: 1\nimage:\n  type: raw\n  architecture: x86_64\n  channel: stable\n' > good.yaml
    printf 'version: 1\nimage:\n  type: raw\n  architecture: x86_64\n  channel: stable\n  offline: true\nseeds:\n  applications:\n    applications:\n      - name: incus\n' > offline.yaml; }
$ df -h "$WORK2"                                   # need ≥25 GB free
$ INCUSOS_BUILDER_E2E=1 mise x -- moon run root:e2e
$ gh workflow run release-dry-run.yml --repo componere/incusos-builder --ref master   # SUP-12, runs remotely
```

Expected: ≥25 GB free. `root:e2e` exits 0 — it live-proves versions parsing, a
smallest-image build, the eleven-section seed round-trip, and an exact ISO/FAT32
rescue read-back; if it fails, the whole of Wave 2 track A will fail too, so stop
here. The rehearsal dispatch returns immediately; collect its result in SUP-12
while track A runs locally. The `cp` fallback exists so Wave 2 can start on a
machine that never ran Wave 1.

Pass/Fail: [ ]

---

## 3. Promise inventory

Each row is a user-visible claim, its source, and the cases that observe it.
Promises are grouped by the surface that owns them; a promise is "covered" only
when at least one non-optional case for it has a recorded result.

### CLI and automation contract

| Promise | Source | Cases |
|---|---|---|
| Bare invocation exits 0 and writes nothing | `docs/docs/reference/cli.md:26` | CLI-01 |
| `--version` prints two lines incl. the `incus-os API:` pin; unset metadata is `dev`/`none`/`unknown` | `cli.md:27-35` | CLI-01, PRE-05, SUP-05 |
| Registered commands are `build`, `validate`, `versions`, `init` | `cli.md:9-10` | CLI-01 (F-CLI-2) |
| Eight persistent flags with the documented defaults | `cli.md:43-52` | CLI-02 |
| Per-command flag sets are exactly as documented | `cli.md:74-79,129-131,148-151,171-173` | CLI-02 |
| Exit 2 = usage errors (11 enumerated triggers) | `automation.md:21` | CLI-03, CLI-04, CLI-05, CLI-15, CLI-18, CFG-11 |
| Exit 3 = invalid seed config | `automation.md:22` | CLI-06, CFG-01..CFG-10 |
| Exit 4 = SOPS decryption failure | `automation.md:23` | CLI-07, CFG-13..CFG-17 |
| Exit 5 = acquisition / version resolution | `automation.md:24` | CLI-08, ART-04, ART-13..ART-17 |
| Exit 6 = artifact write failure | `automation.md:25` | CLI-09 |
| One newline-terminated JSON document per `--json` invocation | `automation.md:31-33` | CLI-11, CLI-12, ART-06, SUP-10 |
| Error envelope `{"error":{"code","message"}}`, `code` == exit code | `automation.md:39-51` | CLI-11 |
| build / validate / versions / init envelopes carry the documented fields | `automation.md:67-149` | CLI-12, ART-02, ART-06, MED-07 |
| `-f -` reads config from stdin (plaintext and SOPS) | `automation.md:207` | CLI-14, CFG-13 |
| `-o -` streams image bytes, no summary; `init -o -` writes YAML | `automation.md:208-209` | ART-21, CLI-12 |
| `-` sentinel also matches paths that clean to `-` | `automation.md:203` | CLI-15 (expected to falsify) |
| Precedence: flag > `INCUSOS_BUILDER_*` > default; explicit `=false` beats env | `automation.md:151-169` | CLI-16, ART-11 |
| `CI`, `NO_COLOR`, `TERM=dumb`, `ACCESSIBLE`, `SOPS_AGE_KEY` behaviors | `automation.md:171-179` | CLI-17, CLI-19, CFG-15 |
| stdout/stderr split; `-q` suppresses only human success | `automation.md:183-191` | CLI-20 |
| `--progress auto` resolution; `--color auto` disabling rules | `automation.md:193-199` | CLI-17, CLI-20 |
| Overwrite prompt, `--force`, `.incusos-builder.bak`, both-or-neither publish | `cli.md:96-109` | CLI-18, ART-18, MED-17 |
| `init` refuses an existing path; cancel is exit 2 | `cli.md:175-177,190` | CLI-19, CFG-11 |
| `--verbose` logs resolved version, asset, output paths | `cli.md:117-118` | CLI-20 |

### Seed config and SOPS

| Promise | Source | Cases |
|---|---|---|
| `version: 1` required; other integers ask for a newer CLI | `configuration.md:160-169` | CFG-07 |
| Document keys are exactly `version`, `image`, `seeds` (+ non-schema `sops`) | `configuration.md:41-52` | CFG-08 |
| `image` accepts `type`, `architecture`, `channel`, `release`, `offline` only | `configuration.md:175-181` | CFG-01, CFG-08 |
| `type` ∈ {iso,raw}, `architecture` ∈ {x86_64,aarch64}, case-sensitive | `internal/config/validate.go:119-131` | CFG-01, CFG-08 |
| Empty/omitted channel becomes `stable`; channel/release are free text | `configuration.md:126-128` | CFG-01 |
| Exactly eleven `seeds.*` sections accepted | `configuration.md:211-223` | CFG-02, CFG-04, CFG-11 |
| Offline requires a non-empty `seeds.applications.applications` | `configuration.md:124,260-261` | CFG-03, MED-02 |
| Offline forces `seeds.update.check_frequency: never` | `configuration.md:106-107` | not observable via CLI (§6) |
| `sort_order` ∈ {"", smallest, largest}, case-insensitive | `configuration.md:122` | CFG-05 |
| `encryption_recovery_keys` rejected without echoing the secret | `configuration.md:741-753` | CFG-06 |
| Unknown keys rejected with a dotted path and the pin string | `configuration.md:75-96` | CFG-08 |
| Wrong types rejected with the literal sanitized to `<value>` | `configuration.md:59-60,90-91` | CFG-09 |
| A top-level `sops` key alone selects in-memory decryption | `configuration.md:48-52` | CFG-13, CFG-16 |
| Every post-selection failure is exit 4, never 3 | `configuration.md:156-158` | CFG-16 |
| Decrypted bytes never touch disk | `internal/config/doc.go:5` | CFG-13, CFG-18 |
| `validate` performs no network I/O | `sops-encryption.md:66-67` | CFG-18 |
| `init --no-input` output validates; lists all eleven sections commented | `cli.md:179-185` | CFG-11 |
| Interactive `init` answers produce a valid config | `internal/cli/init.go:259-291` | CFG-12 (expected to falsify for offline=yes) |

### Live server, cache, artifacts

| Promise | Source | Cases |
|---|---|---|
| `versions` table columns and channel/architecture filtering | `cli.md:144-159` | ART-01, ART-02 |
| Unknown channel is an empty list at exit 0 | `cli.md:146` | ART-03 |
| Default server; `http://` and non-directories are exit 2 | `cli.md:50,57-59` | CLI-05, ART-12 |
| `build` human summary field set and order | `cli.md:111-115` | ART-05 |
| `result.sha256` equals a second hash of the published file | `automation.md:94,97` | ART-05, ART-06, MED-07 |
| `.gz` output digest covers compressed stored bytes | `cli.md:92-94` | ART-10 |
| `seed-data` starts at byte 2148532224; drift is exit 5 | `internal/build/probe.go:18-22` | ART-08 |
| Seed region is an uncompressed tar, one YAML per section, mode 0600, writeSeed order | `seed-injection.md:56-63` | ART-08 |
| Splice rewrites only the tar range | `seed-injection.md:69-85` | ART-09 |
| Seed tar must fit the 100 MiB partition (else exit 3) | `configuration.md:133-146` | §6 (not manually reachable) |
| Cache layout `<dir>/sha256/<64hex>`, mode 0444, no residue | `cache.md:35-51` | ART-07 |
| Cache key is the metadata digest; reuse skips the fetch | `cache.md:53-78` | ART-06, ART-07 |
| `--cache-dir` / env precedence; empty is exit 5 | `cache.md:17-31` | ART-04, ART-11 |
| Low free space warns and continues | `cache.md:92-95` | ART-20 |
| A local `--server` directory builds through the same cache | `use-local-mirror.md:8-48` | ART-12 |
| Size/digest admission failure = exit 5, no cache residue | `cache.md:88-90` | ART-13 |
| Metadata allowlist (filename segments, 64-hex lowercase, 0 < size ≤ 8 GiB) | `cache.md:59-70` | ART-14 |
| Missing `index.json` / missing asset messages | `use-local-mirror.md:134-135` | ART-14 |
| Highest-version-in-channel default; exact pin honored | `configuration.md:126-128` | ART-15 |
| Unknown pin = exit 5 with the sorted available list | `use-local-mirror.md:146-152` | ART-16 |
| Cancelled fetch = exit 5; no partial artifact; temps cleaned | `recover-interrupted-build.md:53-56` | ART-17 |
| `--force` backup/restore semantics | `recover-interrupted-build.md:29-61` | ART-18, MED-17 |
| Offline three-way metadata binding (Filename+Sha256, Version match) | `use-local-mirror.md:129-132` | ART-19 |

### Rescue media and offline builds

| Promise | Source | Cases |
|---|---|---|
| Offline publishes two artifacts in one invocation | `build-offline-media.md:60-78` | MED-07, MED-11 |
| Default rescue name `<stem>.resources.<iso\|img>` follows `image.type` | `cli.md:87-90` | MED-06 |
| `--resources-output` overrides and is echoed as `result.resources_output` | `automation.md:87-88` | MED-07, MED-17 |
| Four rescue usage errors (online, `-`, non-distinct, offline `-o -`) | `cli.md:83-85` | MED-01, MED-03, MED-04, MED-05 |
| ISO is Rock Ridge only, label `RESCUE_DATA`, PVD-truncated | `internal/media/iso.go:14-101` | MED-12, MED-13 |
| Raw is GPT + one `Microsoft Basic Data` `RESCUE_DATA` FAT32 partition at 1 MiB | `build-offline-media.md:140` | MED-08 |
| Proportional sizing with a 256 MiB floor (275,901,440 B for small payloads) | `internal/media/fat.go:56-73` | MED-09 |
| Media tree is exactly `update/update.json`, `update/update.sjson`, `update/<arch>/<app>`; never `hotfix.sh.sig` | `build-offline-media.md:156-166` | MED-10, MED-12 |
| Media bytes are byte-exact copies of verified sources | `internal/media/rescue.go:253-271` | MED-10, MED-12, MED-15 |
| Payloads come from the content-addressed cache after digest admission | `cache.md:39-46` | MED-15 |

### Packaging, image, supply chain, release

| Promise | Source | Cases |
|---|---|---|
| `mise run image-local` builds the apk and loads `incusos-builder:dev` | `mise.toml:55-71` | SUP-03 |
| The apk is signed and apko trusts it only via the appended key | `melange.yaml`, `mise.toml:67` | SUP-04 |
| version/commit/date reach the binary via `--vars-file` ldflags | `melange.yaml:25-40` | SUP-05 |
| Image entrypoint is `/usr/bin/incusos-builder` | `apko.yaml:21-22` | SUP-06, SUP-10 |
| Image runs as uid/gid 65532 | `apko.yaml:24-34` | SUP-06 |
| Published image is a `linux/amd64,linux/arm64` index with four OCI annotations | `apko.yaml:36-44` | SUP-12 (rehearsal), SUP-22 (post-tag) |
| apko emits a per-build SBOM; a separate syft SPDX doc is attested | `apko.yaml:1-6`, `release.yml:409-420` | SUP-08, SUP-09, SUP-20 |
| Image digest is cosign keyless signed; the documented verify command works | `release.yml:404-407,477` | SUP-17, SUP-21 |
| Provenance is minted in the isolated `attest.yml`; `gh attestation verify` works | `attest.yml`, `release.yml:466` | SUP-17, SUP-19, SUP-20 |
| A release ships 9 assets (4 binaries, 4 SBOMs, checksums.txt), checksum-verified | `stage_ghd_release_assets.py:234-287` | SUP-11, SUP-12, SUP-22 |
| The release binary is smoke-tested before publication | `release.yml:125-149` | SUP-12, SUP-22 |
| Release Please opens a draft-producing PR bumping manifest, melange, apko | `release-please-config.json:10-16` | SUP-13 |
| Publication is a human decision on a draft release | `release.yml:438-481` | SUP-23 |
| `run-in-ci.md` snippets work against the real image | `run-in-ci.md` §2,§5 | SUP-10, DOC-13 |
| SECURITY.md's private reporting channel works | `SECURITY.md:11` | SUP-16 |
| README install methods behave as stated pre-release | `README.md:7,26-53` | SUP-18, DOC-02, DOC-03 |

### Documentation and boot acceptance

| Promise | Source | Cases |
|---|---|---|
| README source install and quickstart work verbatim | `README.md:13-24,55-67` | DOC-01, DOC-04 |
| CONTRIBUTING's local gate and task list are accurate | `CONTRIBUTING.md:32-59` | DOC-09 |
| The docs site builds `--strict` and serves; nav covers every page | `docs/moon.yml:42-60` | DOC-06, DOC-07, DOC-08 |
| The tutorial produces a seeded ISO with the documented outputs | `tutorials/first-seeded-iso.md` | DOC-10 |
| Each how-to guide is executable as written | `docs/docs/how-to/*.md` | DOC-11..DOC-16 |
| Boot acceptance requires four specific observations | `how-to/verify-boot-acceptance.md` | BOOT-02..BOOT-09 |
| A successful build is not boot acceptance | `explanation/trust-model.md:36-40` | BOOT-10 |

---

## 4. Test cases

Every case carries: its wave, the promises it observes, the source of those
promises, its cost, the literal commands, the observable expectation, and a
pass/fail box. Wave 1 commands assume the PRE-07-W1 shell (`$REPO`, `$IOB`,
`$WORK`, `$CACHE`); Wave 2 commands assume the PRE-07-W2 shell, where `$WORK`
means `$WORK2`. Cases marked **[falsifier]** are expected to fail as documented;
their value is the recorded evidence for the decision in §5.

Cases stay grouped by surface so the document reads as a reference. Run them by
wave, not by section order — §4.0 is the running order.

### 4.0 Wave assignment

**Wave 1 — fast lane (74 cases, none over 5 minutes).** Run in one sitting:

| Surface | Cases |
|---------|-------|
| CLI | CLI-01 .. CLI-20 (all 20) |
| Config / SOPS | CFG-01 .. CFG-19 (all 19) |
| Server metadata | ART-01, ART-02, ART-03, ART-04, ART-14, ART-15, ART-16 |
| Rescue usage errors | MED-01 .. MED-06 |
| Supply chain | SUP-01, SUP-02, SUP-11, SUP-13, SUP-14, SUP-15, SUP-16, SUP-17, SUP-18 |
| Documentation | DOC-02, DOC-03, DOC-05, DOC-06, DOC-07, DOC-08, DOC-09, DOC-11, DOC-13, DOC-16, DOC-17, DOC-18 |
| Boot | BOOT-01 (venue decision only) |

**Wave 2 — long lane (49 cases), three independent tracks.** Track A and track B
share only the machine; track C needs a different host entirely.

| Track | Cases, in order | Gate |
|-------|-----------------|------|
| **A — live build chain, artifacts, media, doc walkthroughs** | ART-05 → ART-06, ART-07, ART-08, ART-09, ART-10, ART-11 → ART-12 → ART-13 → ART-17, ART-18, ART-21 → ART-19 → ART-20 → MED-07 → MED-08, MED-09, MED-10, MED-16 → MED-11 → MED-12, MED-13, MED-15 → MED-14 → MED-17 → DOC-04, DOC-10, DOC-12, DOC-14, DOC-15 | PRE-07-W2 |
| **B — container image and release rehearsal** | SUP-12 (dispatch first, collect last) ‖ SUP-03 → SUP-04, SUP-05, SUP-06, SUP-07, SUP-08, SUP-09, SUP-10 | PRE-07-W2 + a Docker daemon |
| **C — boot acceptance** | BOOT-02 → BOOT-03 → BOOT-04 → BOOT-05 → BOOT-06 → BOOT-07 → BOOT-08 → BOOT-09 → BOOT-10 | BOOT-01 chose a real `x86_64` Linux Incus host |

`→` is a hard dependency (the left case produces an artifact or state the right
one reads); commas are siblings that may run in any order; `‖` runs concurrently.

**Wave P — post-tag (5 cases):** SUP-19, SUP-20, SUP-21, SUP-22, SUP-23, in that
order, against the first real release and before the draft is published.

#### The fifteen cases that exceed five minutes

Everything else in Wave 2 is a cheap inspection that merely needs a Wave 2
artifact. These are the ones that actually cost time:

| Case | Wall time | Disk | Downloads |
|------|-----------|------|-----------|
| MED-07 offline raw build | 10–20 min | ~4.5 GB | yes |
| MED-11 offline ISO build | 10–20 min | ~2 GB | yes (ISO asset) |
| MED-17 `--force` pair replacement | 5–15 min | ~7.5 GB peak | no |
| SUP-03 `mise run image-local` | 5–12 min | 1–3 GB in the Docker VM | yes (Wolfi) |
| SUP-12 release rehearsal | ~7 min (remote runner) | none local | n/a |
| DOC-01 fresh-clone source install | 3–8 min | ~1 GB | yes |
| DOC-04 README quickstart | 20–45 min | ~10 GB | yes |
| DOC-10 tutorial walkthrough | 20–45 min | ~10 GB | yes |
| DOC-12 offline-media how-to | reuses MED-07/11 | — | no |
| DOC-14 local-mirror how-to (populated half) | reuses ART-12 | — | no |
| DOC-15 interrupted-build how-to | reuses ART-18/MED-07 | — | no |
| BOOT-02 build release-gate media | 25–60 min | ~15 GB | yes |
| BOOT-05 install and watch console | 20–60 min | — | Linux host |
| BOOT-07 copy volume, detect `RESCUE_DATA` | 15–40 min | — | Linux host |
| PRE-07-W2 `root:e2e` bootstrap | 10–25 min | ~5 GB | yes |

ART-05 is 1–3 min on a fast link but sits in Wave 2 as track A's producer:
everything downstream reads its 3.2 GiB image and warmed cache, and its 415 MiB
download alone exceeds five minutes on a slower connection.

#### Cases split across waves

Four cases have a cheap part and an expensive part. Run the cheap part in
Wave 1 and record the deferred half against Wave 2:

| Case | Wave 1 portion | Wave 2 portion |
|------|----------------|----------------|
| CLI-20 | everything except the last command | the final `--verbose` build against the warm cache |
| DOC-11 | steps 1–4 and Verification (validate only) | optional step 5 `build -f config.enc.yaml` |
| DOC-13 | every `validate` snippet and the exit table | the `build` snippets and the container run (with SUP-10) |
| DOC-14 | the four documented failure modes | the populated-mirror build (with ART-12) |

### 4.1 CLI contract

#### CLI-01 — Bare invocation, `--help`, `--version`
- Promise: bare run is silent exit 0; two-line version banner; command list.
- Source: `docs/docs/reference/cli.md:9-10,26-35`; `internal/cli/root.go` `SetVersionTemplate`.
- Wave: **Wave 1**
- Cost: cheap.
- Commands:
  ```console
  $ "$IOB"; echo "rc=$?"
  $ "$IOB" 2>&1 | wc -c
  $ "$IOB" --version | wc -l
  $ "$IOB" -v
  $ "$IOB" --help
  ```
- Expected: `rc=0` and `0` bytes of output for the bare run. `--version` prints
  exactly 2 lines: `incusos-builder dev (none) built unknown` and
  `incus-os API: v0.0.0-20260815030500-0f5b8057f2fc`; `-v` is an undocumented
  cobra shorthand that prints the same (**F-CLI-1**). `--help` starts
  `Build seeded IncusOS installation media from a YAML config`, lists
  `Available Commands:` — which includes `completion` and `help` beyond the four
  documented ones (**F-CLI-2**) — and a `Flags:` block naming all eight
  persistent flags with `--cache-dir` defaulting to
  `/Users/<you>/Library/Caches/incusos-builder`.
- Pass/Fail: [ ]

#### CLI-02 — Flag inventory conformance sweep
- Promise: the registered flag set equals the reference tables, defaults included.
- Source: `internal/cli/root.go` `registerPersistentFlags`; each `new*Command`; `cli.md:43-52,74-79,129-131,148-151,171-173`.
- Wave: **Wave 1**
- Cost: cheap.
- Commands:
  ```console
  $ for c in "" build validate versions init; do echo "== $c"; "$IOB" $c --help 2>&1 | sed -n '/^Flags:/,/^$/p'; done
  $ grep -n 'cmd.Flags()\.' "$REPO"/internal/cli/{build,validate,versions,init}.go
  ```
- Expected: persistent — `color`, `progress`, `no-input`, `verbose`, `quiet`(`-q`),
  `server`, `cache-dir`, `json`. `build` — `config`(`-f`), `output`(`-o`),
  `resources-output`, `force`. `validate` — `config`(`-f`). `versions` —
  `channel` (default `stable`), `architecture` (default `aarch64` on this host,
  proving the `arm64`→`aarch64` mapping). `init` — `output`(`-o`, default
  `config.yaml`) and **no** `--force`. No flag in code is missing from the docs
  and none is documented that does not exist. Note `-f`/`-o` render their
  placeholder as `-` rather than `string` (**F-CLI-3**, cosmetic).
- Pass/Fail: [ ]

#### CLI-03 — Exit 2, global flag usage errors
- Promise: invalid `--color`/`--progress`, `--verbose` with `-q`, unknown flags, missing flag arguments.
- Source: `internal/cli/policy.go` `parseColor`/`parseProgress`; `internal/cli/exit.go:50-52`.
- Wave: **Wave 1**
- Cost: cheap.
- Commands:
  ```console
  $ "$IOB" --color purple validate -f good.yaml; echo "rc=$?"
  $ "$IOB" --progress sometimes versions; echo "rc=$?"
  $ "$IOB" --verbose -q validate -f good.yaml; echo "rc=$?"
  $ "$IOB" --nope; echo "rc=$?"
  $ "$IOB" versions --channel; echo "rc=$?"
  ```
- Expected: every case `rc=2`, empty stdout, exactly one stderr line:
  `usage error: invalid --color "purple" (want auto, always, or never)`;
  `usage error: invalid --progress "sometimes" (want auto, always, or never)`;
  `usage error: --verbose and -q are mutually exclusive`;
  `usage error: unknown flag: --nope`;
  `usage error: flag needs an argument: --channel`.
  No cobra usage block is reprinted.
- Pass/Fail: [ ]

#### CLI-04 — Exit 2, `build` and `init` usage errors
- Promise: the ten enumerated build/init usage triggers.
- Source: `internal/cli/build.go` `checkBuildFlags`; `internal/cli/publish.go` `resolvePaths`; `internal/cli/init.go`.
- Wave: **Wave 1**
- Cost: cheap (no network: all fail before acquisition).
- Commands:
  ```console
  $ "$IOB" build -o out.img --server "$WORK/mirror" --cache-dir "$CACHE"; echo "rc=$?"
  $ "$IOB" build -f good.yaml -o "" --server "$WORK/mirror" --cache-dir "$CACHE"; echo "rc=$?"
  $ "$IOB" build --json -f good.yaml -o - --server "$WORK/mirror" --cache-dir "$CACHE"; echo "rc=$?"
  $ "$IOB" build -f offline.yaml -o - --server "$WORK/mirror" --cache-dir "$CACHE"; echo "rc=$?"
  $ "$IOB" build -f good.yaml -o o.img --resources-output r.img --server "$WORK/mirror" --cache-dir "$CACHE"; echo "rc=$?"
  $ "$IOB" build -f offline.yaml -o o.img --resources-output - --server "$WORK/mirror" --cache-dir "$CACHE"; echo "rc=$?"
  $ "$IOB" build -f offline.yaml -o o.img --resources-output o.img --server "$WORK/mirror" --cache-dir "$CACHE"; echo "rc=$?"
  $ "$IOB" init --no-input -o cfgA.yaml; echo "rc=$?"
  $ "$IOB" init --no-input -o cfgA.yaml; echo "rc=$?"
  $ "$IOB" init --no-input -o ""; echo "rc=$?"
  ```
- Expected: `rc=2` for all but the first `init` (`rc=0`, stdout `wrote cfgA.yaml`).
  stderr lines in order: `usage error: -f/--config is required`;
  `usage error: -o/--output is required`;
  `usage error: --json cannot be combined with -o -` (this one also writes
  `{"error":{"code":2,"message":"usage error: --json cannot be combined with -o -"}}`
  to stdout); `usage error: offline builds cannot use -o -`;
  `usage error: --resources-output requires offline: true in the config`;
  `usage error: resources path cannot be -`;
  `usage error: image and resources paths must be distinct`;
  `usage error: refusing to overwrite existing file cfgA.yaml`;
  `usage error: output path is required`. No file is created by a failing case.
- Pass/Fail: [ ]

#### CLI-05 — Exit 2, `--server` classification
- Promise: plain `http://` and non-directory values are usage errors before any request.
- Source: `internal/cli/build.go` `selectImageSource`; `cli.md:57-59`; `use-local-mirror.md:96-113`.
- Wave: **Wave 1**
- Cost: cheap.
- Commands:
  ```console
  $ "$IOB" versions --server http://example.invalid/os; echo "rc=$?"
  $ "$IOB" build -f good.yaml -o out.img --server /definitely-not-a-mirror --cache-dir "$CACHE"; echo "rc=$?"
  $ "$IOB" versions --server "$WORK/good.yaml"; echo "rc=$?"
  ```
- Expected: all `rc=2`.
  `usage error: --server "http://example.invalid/os": plain http is not supported; use https or a local mirror directory`;
  `usage error: --server "/definitely-not-a-mirror" is neither an https URL nor an existing directory`;
  the same wording for the existing *file* (a file is not a directory). No HTTP
  request is made.
- Pass/Fail: [ ]

#### CLI-06 — Exit 3 through the real CLI
- Promise: invalid seed config on both `validate` and `build`, including an unreadable path.
- Source: `internal/errdefs/errors.go` `ErrConfig`; `internal/cli/exit.go:53-54`.
- Wave: **Wave 1**
- Cost: cheap.
- Commands:
  ```console
  $ "$IOB" validate -f bad.yaml; echo "rc=$?"
  $ "$IOB" build -f bad.yaml -o out.img --server "$WORK/mirror" --cache-dir "$CACHE"; echo "rc=$?"
  $ "$IOB" validate -f nope.yaml; echo "rc=$?"
  ```
- Expected: all `rc=3`, empty stdout.
  `invalid config: image.type: must be iso or raw` (twice);
  `invalid config: read config: open nope.yaml: no such file or directory`.
  No output file is produced by the `build` case.
- Pass/Fail: [ ]

#### CLI-07 — Exit 4 through the real CLI
- Promise: a top-level `sops` key selects decryption; failure is 4, never 3.
- Source: `internal/config/load.go` `Parse`; `internal/cli/exit.go:55-56`. Full matrix in CFG-13..CFG-17.
- Wave: **Wave 1**
- Cost: cheap.
- Commands:
  ```console
  $ "$IOB" validate -f "$REPO/internal/config/testdata/stray-sops.yaml"; echo "rc=$?"
  $ env -u SOPS_AGE_KEY -u SOPS_AGE_KEY_FILE -u SOPS_AGE_KEY_CMD "$IOB" validate -f "$REPO/internal/config/testdata/encrypted.yaml"; echo "rc=$?"
  $ env -u SOPS_AGE_KEY -u SOPS_AGE_KEY_FILE -u SOPS_AGE_KEY_CMD "$IOB" validate --json -f "$REPO/internal/config/testdata/encrypted.yaml" | jq -e '.error.code == 4'
  ```
- Expected: `rc=4` for both plain runs; stderr begins `decryption failed: `. The
  `--json` run prints `true` and exits 4.
- Pass/Fail: [ ]

#### CLI-08 — Exit 5 through the real CLI
- Promise: empty cache directory and unreachable/empty sources are acquisition failures.
- Source: `internal/update/cache.go:44-46`; `internal/update/local.go:136`; `internal/update/client.go` `getOnce`.
- Wave: **Wave 1**
- Cost: cheap; the HTTPS case takes ~1 min (three retry attempts).
- Commands:
  ```console
  $ "$IOB" versions --server "$WORK/mirror" --cache-dir ""; echo "rc=$?"
  $ "$IOB" versions --server "$WORK/mirror" --cache-dir "$CACHE"; echo "rc=$?"
  $ "$IOB" versions --server https://127.0.0.1:1/os --cache-dir "$CACHE"; echo "rc=$?"
  $ INCUSOS_BUILDER_CACHE_DIR= "$IOB" versions --server "$WORK/mirror"; echo "rc=$?"
  ```
- Expected: all `rc=5`. First: `acquisition failed: cache directory is required`.
  Second: `acquisition failed: open index.json: open <WORK>/mirror/index.json: no such file or directory`.
  Third: ends in a `connection refused` cause. Fourth is `rc=5` **for the wrong
  reason** — it fails on `index.json`, proving an empty `INCUSOS_BUILDER_CACHE_DIR`
  does not override the default the way `--cache-dir ""` does (**F-CLI-4**).
- Pass/Fail: [ ]

#### CLI-09 — Exit 6 through the real CLI
- Promise: an artifact write failure is exit 6 with the `output write failed:` prefix.
- Source: `internal/cli/publish.go` `createTemp`/`outputWrap`; `exit_codes.txtar:2-3`.
- Wave: **Wave 1**
- Cost: cheap (fails in `Begin`, before any download).
- Commands:
  ```console
  $ mkdir -p ro && chmod 555 ro
  $ "$IOB" build -f good.yaml -o ro/out.img --server "$WORK/mirror" --cache-dir "$CACHE"; echo "rc=$?"
  $ "$IOB" build --json -f good.yaml -o ro/out.img --force --server "$WORK/mirror" --cache-dir "$CACHE"; echo "rc=$?"
  $ chmod 755 ro; ls -a ro
  ```
- Expected: both `rc=6`; stderr matches
  `output write failed: create temp for ro/out.img: open ro/.out.img-<digits>.tmp: permission denied`.
  The `--json` run writes one envelope with `error.code == 6`. `ro/` is empty.
- Pass/Fail: [ ]

#### CLI-10 — Unknown commands and stray operands **[falsifier]**
- Promise: docs say every command takes no operands and usage errors are exit 2.
- Source: `cli.md:10,71,126,146,169`; `automation.md:21`; `internal/cli/exit.go:61-63`.
- Wave: **Wave 1**
- Cost: cheap.
- Commands:
  ```console
  $ "$IOB" frobnicate; echo "rc=$?"
  $ "$IOB" build extra; echo "rc=$?"
  $ "$IOB" validate x -f good.yaml; echo "rc=$?"
  $ "$IOB" versions x; echo "rc=$?"
  $ "$IOB" init x --no-input; echo "rc=$?"
  ```
- Expected: every case `rc=1` (not 2) with stderr
  `unknown command "<arg>" for "incusos-builder[ <command>]"`. Cobra's error
  wraps neither `ErrUsage` nor a pflag error, so it falls through to
  `exitInternal`. Record as **F-CLI-5**: an automation consumer branching on
  exit 2 will not see operand mistakes.
- Pass/Fail: [ ]

#### CLI-11 — Error envelope shape across codes 2, 3, 5, 6
- Promise: `{"error":{"code","message"}}`, one document, `code` == exit code, message also on stderr.
- Source: `automation.md:39-65`; `internal/cli/exit.go:31-43,79-96`.
- Wave: **Wave 1**
- Cost: cheap.
- Commands:
  ```console
  $ "$IOB" build --json -f good.yaml -o - --server "$WORK/mirror" --cache-dir "$CACHE" 2>/dev/null | jq -e 'keys == ["error"] and (.error|keys) == ["code","message"] and (.error.code|type) == "number" and .error.code == 2'
  $ "$IOB" validate --json -f bad.yaml 2>/dev/null | jq -e '.error.code == 3 and (.error.message | startswith("invalid config: "))'
  $ "$IOB" versions --json --server "$WORK/mirror" --cache-dir "" 2>/dev/null | jq -e '.error.code == 5 and .error.message == "acquisition failed: cache directory is required"'
  $ chmod 555 ro; "$IOB" build --json -f good.yaml -o ro/out.img --server "$WORK/mirror" --cache-dir "$CACHE" 2>/dev/null | jq -e '.error.code == 6'; chmod 755 ro
  $ "$IOB" validate --json -f bad.yaml 2>/dev/null | wc -l
  $ "$IOB" validate --json -f bad.yaml 2>&1 >/dev/null
  ```
- Expected: each `jq -e` prints `true`. `wc -l` prints `1` — exactly one
  newline-terminated document. The last command shows the error line is also
  reprinted on stderr under `--json`.
- Pass/Fail: [ ]

#### CLI-12 — Success envelopes for `validate`, `versions`, `init`, and `-q` behavior
- Promise: documented field sets; `-q` suppresses human output but not JSON or streamed bytes.
- Source: `automation.md:100-149,190-191`.
- Wave: **Wave 1**
- Cost: cheap (one live `index.json` fetch).
- Commands:
  ```console
  $ "$IOB" validate -f good.yaml; echo "rc=$?"
  $ "$IOB" validate -q -f good.yaml | wc -c
  $ "$IOB" validate --json -f good.yaml | jq -e '(.result|keys) == ["architecture","offline","type","valid"] and .result.valid == true and .result.type == "raw"'
  $ "$IOB" versions --json --cache-dir "$CACHE" | jq -e '(.result.versions[0]|keys) == ["architectures","channels","published_at","version"]'
  $ mkdir -p initwork && cd initwork
  $ "$IOB" init --json --no-input -o cfgB.yaml | jq -e '.result == {"output":"cfgB.yaml"}'
  $ "$IOB" init --no-input -o - | head -3
  $ "$IOB" init -q --no-input -o cfgC.yaml | wc -c; ls cfgC.yaml; cd "$WORK"
  ```
- Expected: `configuration valid` on stdout; `0` bytes under `-q`; every `jq -e`
  prints `true`. `init -o -` writes YAML with no `wrote` line; `init -q` writes
  0 bytes but still creates the file.
- Pass/Fail: [ ]

#### CLI-13 — `--json` on a flag-parse failure
- Promise: flag-parse errors are enveloped only when `--json` is on the command line.
- Source: `automation.md:41-42`; `internal/cli/root.go` `SetFlagErrorFunc`.
- Wave: **Wave 1**
- Cost: cheap.
- Commands:
  ```console
  $ "$IOB" --json --nope 2>/dev/null; echo "rc=$?"
  $ INCUSOS_BUILDER_JSON=1 "$IOB" --nope 2>/dev/null; echo "rc=$?"
  $ INCUSOS_BUILDER_JSON=1 "$IOB" --nope 2>&1 >/dev/null
  ```
- Expected: first prints
  `{"error":{"code":2,"message":"usage error: unknown flag: --nope"}}`, `rc=2`.
  Second prints nothing on stdout, `rc=2`; the message appears on stderr. This
  asymmetry is documented behavior (Viper is not initialized during flag parse),
  not a defect.
- Pass/Fail: [ ]

#### CLI-14 — `-f -` reads the config from stdin
- Promise: stdin configs, plaintext and SOPS-encrypted, on `validate` and `build`.
- Source: `automation.md:207`; `stdin_config.txtar`.
- Wave: **Wave 1**
- Cost: cheap.
- Commands:
  ```console
  $ printf 'version: 1\nimage:\n  type: iso\n  architecture: aarch64\n' | "$IOB" validate -f -; echo "rc=$?"
  $ SOPS_AGE_KEY="$(tail -n1 "$REPO/internal/config/testdata/age.key")" "$IOB" validate -f - < "$REPO/internal/config/testdata/encrypted.yaml"; echo "rc=$?"
  $ env -u SOPS_AGE_KEY -u SOPS_AGE_KEY_FILE -u SOPS_AGE_KEY_CMD "$IOB" validate -f - < "$REPO/internal/config/testdata/encrypted.yaml"; echo "rc=$?"
  $ printf 'version: 1\nimage:\n  type: disk\n  architecture: x86_64\n' | "$IOB" validate -f -; echo "rc=$?"
  ```
- Expected: `configuration valid` / `rc=0` for the first two; `rc=4` with
  `decryption failed: ` for the third; `rc=3` with
  `invalid config: image.type: must be iso or raw` for the fourth. No decrypted
  bytes appear on disk (`ls -a` shows nothing new).
- Pass/Fail: [ ]

#### CLI-15 — Stream sentinel and the "cleans to `-`" rule **[falsifier]**
- Promise: `automation.md:203` says `-` "or a path that cleans to `-`" is the sentinel for all uses.
- Source: `internal/cli/publish.go` `isStdout` (uses `filepath.Clean`) vs `internal/cli/init.go` and `build.go` `loadBuildSpec` (exact compare).
- Wave: **Wave 1**
- Cost: cheap.
- Commands:
  ```console
  $ mkdir -p dashwork && cd dashwork && cp ../good.yaml .
  $ "$IOB" build --json -f good.yaml -o ./- --server "$WORK/mirror" --cache-dir "$CACHE"; echo "rc=$?"
  $ "$IOB" init --json --no-input -o ./-; echo "rc=$?"
  $ ls -la
  $ printf 'version: 1\nimage:\n  type: iso\n  architecture: x86_64\n' | "$IOB" validate -f ./-; echo "rc=$?"
  $ cd "$WORK"
  ```
- Expected: `build -o ./-` honors the rule (`rc=2`, the `--json`/`-o -` envelope).
  `init --json -o ./-` does **not**: `rc=0`, `{"result":{"output":"./-"}}`, and a
  regular file named `-` is created. `validate -f ./-` opens that file instead of
  stdin. Record as **F-CLI-6**.
- Pass/Fail: [ ]

#### CLI-16 — Precedence: flag > env > default
- Promise: unpassed flag defaults cannot mask env; explicit `=false` beats a true env value.
- Source: `automation.md:151-169`; `internal/cli/root.go` `bindParsedFlags`.
- Wave: **Wave 1**
- Cost: cheap.
- Commands:
  ```console
  $ INCUSOS_BUILDER_SERVER=/env-bad-dir "$IOB" versions --cache-dir "$CACHE"; echo "rc=$?"
  $ INCUSOS_BUILDER_SERVER=/env-bad-dir "$IOB" versions --server /flag-bad-dir --cache-dir "$CACHE"; echo "rc=$?"
  $ INCUSOS_BUILDER_JSON=1 "$IOB" validate -f good.yaml
  $ INCUSOS_BUILDER_JSON=1 "$IOB" validate --json=false -f good.yaml
  $ INCUSOS_BUILDER_COLOR=purple "$IOB" validate -f good.yaml; echo "rc=$?"
  $ INCUSOS_BUILDER_COLOR=purple "$IOB" validate --color never -f good.yaml; echo "rc=$?"
  $ INCUSOS_BUILDER_PROGRESS=sometimes "$IOB" versions --cache-dir "$CACHE"; echo "rc=$?"
  ```
- Expected: the env-only server error quotes `"/env-bad-dir"`; adding the flag
  makes it quote `"/flag-bad-dir"`. `INCUSOS_BUILDER_JSON=1` alone yields the
  validate envelope; `--json=false` yields `configuration valid`. Invalid env
  values for `--color`/`--progress` are `rc=2` and an explicit valid flag clears
  them.
- Pass/Fail: [ ]

#### CLI-17 — `CI`, `NO_COLOR`, `TERM=dumb`, `ACCESSIBLE` (real TTY)
- Promise: auto no-input and color/prompt style selection.
- Source: `automation.md:171-179,193-199`; `internal/cli/policy.go` `autoNoInput`; `internal/ux/reporter.go` `colorAutoEnabled`.
- Wave: **Wave 1**
- Cost: cheap; **must be run in a real Terminal window, not a pipe**.
- Commands:
  ```console
  $ cp good.yaml victim.img
  $ CI=1 "$IOB" build -f good.yaml -o victim.img --no-input=false --server "$WORK/mirror" --cache-dir "$CACHE"; echo "rc=$?"
  $ NO_COLOR=1 "$IOB" versions --server "$WORK/mirror" --cache-dir "$CACHE"
  $ TERM=dumb "$IOB" versions --server "$WORK/mirror" --cache-dir "$CACHE"
  $ "$IOB" versions --server "$WORK/mirror" --cache-dir "$CACHE"
  ```
- Expected: with `CI=1` and `--no-input=false`, **no prompt**; immediate `rc=2`
  `usage error: refusing to overwrite victim.img; re-run with --force` — `CI`
  cannot be cleared by the flag. `NO_COLOR=1` and `TERM=dumb` produce plain
  `==> index` / `done index`; the unset run on a TTY produces styled `▸`/`✓`
  lines with ANSI sequences.
- Pass/Fail: [ ]

#### CLI-18 — Overwrite prompt on a real TTY: `y`, `n`, EOF, `--force`
- Promise: the prompt text, the accepted answers, and the non-interactive refusal.
- Source: `cli.md:96-109`; `internal/cli/build.go` `confirmPrompt`; `no_input.txtar`.
- Wave: **Wave 1**
- Cost: cheap (each run stops at exit 5 against the empty mirror right after the prompt).
- Commands (type the answers at the prompt):
  ```console
  $ "$IOB" build -f good.yaml -o victim.img --server "$WORK/mirror" --cache-dir "$CACHE"
  overwrite existing output? [y/N] n
  $ "$IOB" build -f good.yaml -o victim.img --server "$WORK/mirror" --cache-dir "$CACHE"
  overwrite existing output? [y/N] y
  $ "$IOB" build -f good.yaml -o victim.img --server "$WORK/mirror" --cache-dir "$CACHE" < /dev/null; echo "rc=$?"
  $ "$IOB" build -f good.yaml -o victim.img --force --server "$WORK/mirror" --cache-dir "$CACHE"; echo "rc=$?"
  $ "$IOB" build -f good.yaml -o victim.img --no-input --server "$WORK/mirror" --cache-dir "$CACHE"; echo "rc=$?"
  $ cmp good.yaml victim.img && echo "victim intact"
  ```
- Expected: `n` → `rc=2` `usage error: refusing to overwrite victim.img; re-run with --force`.
  `y` (or `yes`, any case) → the build proceeds and stops at `rc=5`
  `acquisition failed: open index.json: …`. `< /dev/null` → **no prompt** (stdin
  is not a TTY, auto no-input) and `rc=2`. `--force` and `--no-input` → no prompt,
  `rc=5` and `rc=2` respectively. `victim.img` is byte-identical throughout.
- Pass/Fail: [ ]

#### CLI-19 — `init` interactive form and cancel (real TTY)
- Promise: form fields, stderr rendering, `ACCESSIBLE` line mode, cancel is exit 2 with no file.
- Source: `cli.md:186-190`; `internal/cli/init.go` `newInitForm`, `runInitForm`.
- Wave: **Wave 1**
- Cost: cheap; interactive.
- Commands:
  ```console
  $ mkdir -p formwork && cd formwork
  $ "$IOB" init            # then press Ctrl-C
  $ echo "rc=$?"; ls
  $ "$IOB" init            # select ISO installer, aarch64, empty channel, no
  $ echo "rc=$?"; cat config.yaml
  $ ACCESSIBLE=1 "$IOB" init -o acc.yaml   # answer 1, 1, Enter, n
  $ cat acc.yaml; cd "$WORK"
  ```
- Expected: the form renders on **stderr** with group title
  `incusos-builder init` and fields `Image type`, `Architecture`, `Channel`
  (placeholder `stable`), `Offline install?`. Ctrl-C → `rc=2`
  `usage error: init cancelled`, **no file created**. The completed run →
  `wrote config.yaml`, with `type: iso`, `architecture: aarch64`,
  `channel: stable` (empty defaulted), `offline: false`, and **no** commented
  `#seeds:` block (that appears only under `--no-input`). `ACCESSIBLE=1`
  produces numbered `Enter a number between 1 and 2:` prompts. Piping stdin makes
  the form unreachable — auto no-input writes the commented example instead.
- Pass/Fail: [ ]

#### CLI-20 — Stream routing, `-q`, `--verbose`, progress gating
- Promise: stdout/stderr split; `-q` suppresses only human success; `--verbose` debug lines.
- Source: `automation.md:183-199`; `internal/cli/build.go` `logBuildPlan`; `internal/ux/plain.go`.
- Wave: **Wave 1** — except the final `--verbose` build, deferred to Wave 2 track A
- Cost: cheap, except the final `--verbose` build which reuses the ART-05 warm cache.
- Commands:
  ```console
  $ "$IOB" validate -f good.yaml 2>&1 >/dev/null | wc -c
  $ "$IOB" validate -f bad.yaml 2>/dev/null | wc -c; echo "rc=$?"
  $ "$IOB" versions --server "$WORK/mirror" --cache-dir "$CACHE" -q 2>&1 >/dev/null
  $ "$IOB" versions --server "$WORK/mirror" --cache-dir "$CACHE" --progress never 2>&1 >/dev/null
  $ "$IOB" validate --verbose -f good.yaml 2>&1 >/dev/null | wc -c
  $ "$IOB" build --verbose --json -f "$WORK/live11.yaml" -o v.img --cache-dir "$CACHE" --color never 2> v.err >/dev/null; grep -E 'resolved version|output paths' v.err
  ```
- Expected: `validate` success writes to stdout with 0 bytes on stderr; the
  failure writes 0 bytes to stdout. `-q` and `--progress never` still emit
  `==> index` / `done index` step headers on stderr — only percentage lines are
  gated (**F-CLI-7**). `validate --verbose` emits 0 bytes of stderr — `--verbose`
  has no observable effect outside a successful `build`, and no warn/error-level
  log statement exists in the tree (**F-CLI-8**). The final build writes two
  debug lines containing `resolved version` (with `version=`/`asset=`) and
  `output paths` (with `image=`/`resources=`).
- Pass/Fail: [ ]

---

### 4.2 Seed config and SOPS

#### CFG-01 — Minimal document, image matrix, channel default
- Promise: `version: 1` + `image.type` + `image.architecture` suffice; iso/raw × x86_64/aarch64; channel/release are free text.
- Source: `configuration.md:775-783,126-128`; `internal/build/spec.go:12-38`.
- Wave: **Wave 1**
- Cost: cheap.
- Commands:
  ```console
  $ printf 'version: 1\nimage:\n  type: iso\n  architecture: x86_64\n' > min.yaml
  $ "$IOB" validate -f min.yaml --color never; echo "exit=$?"
  $ for t in iso raw; do for a in x86_64 aarch64; do printf 'version: 1\nimage:\n  type: %s\n  architecture: %s\n' "$t" "$a" | "$IOB" validate -f - --json; done; done
  $ printf 'version: 1\nimage:\n  type: raw\n  architecture: aarch64\n  channel: daily\n  release: "202608102114"\n' | "$IOB" validate -f - --color never; echo "exit=$?"
  ```
- Expected: `configuration valid`, `exit=0`. Four envelopes, one per combination,
  e.g. `{"result":{"valid":true,"type":"iso","architecture":"x86_64","offline":false}}`.
  The free-text `channel: daily` document validates without any resolution attempt.
- Pass/Fail: [ ]

#### CFG-02 — Maximal config: all eleven seed sections populated
- Promise: eleven sections, every documented field real on the pin.
- Source: `configuration.md:869-1023` ("Populated eleven-section shape"); `internal/config/schema.go:55-78`.
- Wave: **Wave 1**
- Cost: cheap.
- Commands:
  ```console
  $ sed -n '873,1022p' "$REPO/docs/docs/reference/configuration.md" > maximal.yaml
  $ "$IOB" validate -f maximal.yaml --color never; echo "exit=$?"
  $ "$IOB" validate -f maximal.yaml --json
  ```
- Expected: `configuration valid`, `exit=0`, envelope
  `{"result":{"valid":true,"type":"raw","architecture":"aarch64","offline":false}}`.
  Because decoding is strict, exit 0 proves every key across `applications`,
  `incus`, `operations-center`, `migration-manager`, `install` (with
  `security`/`target`), `network` (with `dns`/`time`/`interfaces`/`routes`),
  `provider`, `services` (all eight service maps), `update` (with
  `maintenance_windows`), `kernel.console`, and `security.custom_ca_certs` is a
  real field on the pinned upstream. Re-check the line range if the doc changed.
- Pass/Fail: [ ]

#### CFG-03 — Offline requires a non-empty applications list
- Promise: `image.offline: true` without applications is exit 3, before any download.
- Source: `internal/config/validate.go:156-164`; `configuration.md:124,260-261`.
- Wave: **Wave 1**
- Cost: cheap.
- Commands:
  ```console
  $ "$IOB" validate -f offline.yaml --json; echo "exit=$?"
  $ printf 'version: 1\nimage:\n  type: iso\n  architecture: x86_64\n  offline: true\n' | "$IOB" validate -f - --color never; echo "exit=$?"
  $ printf 'version: 1\nimage:\n  type: iso\n  architecture: x86_64\n  offline: true\nseeds:\n  applications:\n    applications: []\n' | "$IOB" validate -f - --color never; echo "exit=$?"
  ```
- Expected: first `exit=0` with `"offline":true`. Second and third: stdout empty,
  stderr exactly
  `invalid config: seeds.applications: required when image.offline is true`,
  `exit=3` — an empty list is rejected identically to an absent section.
- Pass/Fail: [ ]

#### CFG-04 — `seeds` omitted, null, or empty
- Promise: `seeds` is optional and may be `{}` or all-empty sections.
- Source: `configuration.md:33-45,148-149,818-835`.
- Wave: **Wave 1**
- Cost: cheap.
- Commands:
  ```console
  $ printf 'version: 1\nimage:\n  type: iso\n  architecture: x86_64\nseeds:\n' | "$IOB" validate -f - --color never; echo "exit=$?"
  $ printf 'version: 1\nimage:\n  type: iso\n  architecture: x86_64\nseeds: {}\n' | "$IOB" validate -f - --color never; echo "exit=$?"
  $ sed -n '818,835p' "$REPO/docs/docs/reference/configuration.md" | "$IOB" validate -f - --color never; echo "exit=$?"
  ```
- Expected: all three `configuration valid`, `exit=0`. The third is the doc's
  "every accepted seed section present" example with all eleven as `{}`.
- Pass/Fail: [ ]

#### CFG-05 — `sort_order` enum, case-insensitive
- Promise: empty / `smallest` / `largest` accepted in any case; anything else exit 3.
- Source: `internal/config/validate.go:134-144`; `configuration.md:122`.
- Wave: **Wave 1**
- Cost: cheap.
- Commands:
  ```console
  $ for o in '""' smallest Smallest largest LARGEST medium; do printf 'version: 1\nimage:\n  type: iso\n  architecture: x86_64\nseeds:\n  install:\n    target:\n      sort_order: %s\n' "$o" | "$IOB" validate -f - --color never; echo "$o exit=$?"; done
  ```
- Expected: the first five print `configuration valid` / `exit=0`; `medium`
  prints exactly
  `invalid config: seeds.install.target.sort_order: must be empty, smallest, or largest`
  with `exit=3`.
- Pass/Fail: [ ]

#### CFG-06 — Recovery keys rejected without echoing the secret
- Promise: `seeds.security.encryption_recovery_keys` is refused and the value never appears in the error.
- Source: `internal/config/validate.go:147-153`; `configuration.md:741-753,1025-1035`.
- Wave: **Wave 1**
- Cost: cheap.
- Commands:
  ```console
  $ sed -n '1028,1035p' "$REPO/docs/docs/reference/configuration.md" > recovery.yaml
  $ "$IOB" validate -f recovery.yaml --color never 2> recovery.err; echo "exit=$?"
  $ cat recovery.err; grep -c 'super-secret-recovery-key' recovery.err
  $ sed -n '855,866p' "$REPO/docs/docs/reference/configuration.md" | "$IOB" validate -f - --color never; echo "exit=$?"
  ```
- Expected: `exit=3` and one stderr line:
  `invalid config: seeds.security.encryption_recovery_keys: it is not possible to set encryption recovery key(s) via the security seed; see https://linuxcontainers.org/incus-os/docs/main/reference/system/security/`.
  `grep -c` prints `0` — the secret is absent. The `custom_ca_certs` +
  `encryption_recovery_keys: []` document validates at `exit=0`.
- Pass/Fail: [ ]

#### CFG-07 — `version` required and pinned to 1
- Promise: missing version, unsupported version, and a document with no `image`.
- Source: `internal/config/validate.go:26-34`; `configuration.md:160-169`.
- Wave: **Wave 1**
- Cost: cheap.
- Commands:
  ```console
  $ printf 'image:\n  type: iso\n  architecture: x86_64\n' | "$IOB" validate -f - --color never; echo "exit=$?"
  $ printf 'version: 2\nimage:\n  type: iso\n  architecture: x86_64\n' | "$IOB" validate -f - --color never; echo "exit=$?"
  $ printf 'version: 1\n' | "$IOB" validate -f - --color never; echo "exit=$?"
  ```
- Expected: `invalid config: version: required`;
  `invalid config: version: unsupported schema version; a newer CLI is required`;
  `invalid config: image.type: must be iso or raw`. All `exit=3`.
- Pass/Fail: [ ]

#### CFG-08 — Strict decode: unknown keys carry a dotted path and the pin string
- Promise: unknown keys rejected with the upstream pin named; enums case-sensitive.
- Source: `internal/config/load.go:135-189`; `configuration.md:75-96`.
- Wave: **Wave 1**
- Cost: cheap.
- Commands:
  ```console
  $ printf 'version: 1\nimage:\n  type: iso\n  architecture: x86_64\nmystery: true\n' | "$IOB" validate -f - --color never; echo "exit=$?"
  $ printf 'version: 1\nimage:\n  type: iso\n  architecture: x86_64\n  flavor: desktop\n' | "$IOB" validate -f - --color never; echo "exit=$?"
  $ printf 'version: 1\nimage:\n  type: iso\n  architecture: x86_64\nseeds:\n  kernel:\n    blacklist_modules:\n      - foo\n' | "$IOB" validate -f - --color never; echo "exit=$?"
  $ printf 'version: 1\nimage:\n  type: ISO\n  architecture: x86_64\n' | "$IOB" validate -f - --color never; echo "exit=$?"
  ```
- Expected: all `exit=3`, stderr verbatim:
  `invalid config: mystery: unknown to incus-os v0.0.0-20260815030500-0f5b8057f2fc; a newer incusos-builder may accept this`;
  the same with `image.flavor`;
  the same with `seeds.kernel.blacklist_modules` (proving upstream
  `SystemKernelConfig` fields are not seed fields);
  `invalid config: image.type: must be iso or raw` for `ISO`. The pin string must
  equal the one from PRE-05.
- Pass/Fail: [ ]

#### CFG-09 — Wrong types rejected with sanitized literals
- Promise: scalar values are replaced by `<value>` so secrets never leak.
- Source: `internal/config/load.go:191-211`; `configuration.md:59-60,90-91`.
- Wave: **Wave 1**
- Cost: cheap.
- Commands:
  ```console
  $ printf 'version: one\nimage:\n  type: iso\n  architecture: x86_64\n' | "$IOB" validate -f - --color never; echo "exit=$?"
  $ printf 'version: 1\nimage:\n  type: iso\n  architecture: x86_64\n  offline: maybe\n' | "$IOB" validate -f - --color never; echo "exit=$?"
  $ printf 'version: 1\nimage:\n  type: iso\n  architecture: x86_64\nseeds: notamapping\n' | "$IOB" validate -f - --color never; echo "exit=$?"
  ```
- Expected: `exit=3` each, stderr
  `invalid config: cannot construct !!str <value> into int`,
  `… into bool`, `… into config.seeds`. None of `one`, `maybe`, `notamapping`
  appears anywhere. Note the third message exposes the Go type name
  `config.seeds` rather than a YAML path (**F-CFG-1**, undocumented wording).
- Pass/Fail: [ ]

#### CFG-10 — Malformed YAML, duplicate keys, empty document
- Promise: all parse failures are exit 3 with sanitized diagnostics.
- Source: `internal/config/load.go:95-110`; `configuration.md:58-60`.
- Wave: **Wave 1**
- Cost: cheap.
- Commands:
  ```console
  $ printf 'version: 1\nimage:\n\ttype: iso\n' | "$IOB" validate -f - --color never; echo "exit=$?"
  $ printf 'version: 1\nimage:\n  type: iso\n  architecture: x86_64\n  type: raw\n' | "$IOB" validate -f - --color never; echo "exit=$?"
  $ : | "$IOB" validate -f - --color never; echo "exit=$?"
  ```
- Expected: `exit=3` each:
  `invalid config: go-yaml load error in scanner (while scanning for the next token) at L3.C1: found character that cannot start any token`;
  `invalid config: yaml: construct errors: line 5: mapping key <value> already defined at line 3`;
  `invalid config: yaml: construct errors: <unknown position>: yaml: no documents in stream`.
- Pass/Fail: [ ]

#### CFG-11 — `init --no-input` round-trip and body
- Promise: the generated example validates and lists all eleven sections commented, in order.
- Source: `internal/cli/init.go:44-61,259-317`; `cli.md:179-185`.
- Wave: **Wave 1**
- Cost: cheap.
- Commands:
  ```console
  $ "$IOB" init --no-input -o gen.yaml --color never; echo "exit=$?"
  $ "$IOB" validate -f gen.yaml --color never; echo "exit=$?"
  $ "$IOB" init --no-input -o gen.yaml; echo "exit=$?"
  $ "$IOB" init --no-input -o - --color never | "$IOB" validate -f - --color never; echo "exit=$?"
  $ cat gen.yaml; grep -c '^#  ' gen.yaml
  ```
- Expected: `wrote gen.yaml`; `configuration valid`; the re-run is
  `usage error: refusing to overwrite existing file gen.yaml` at `exit=2`; the
  piped body validates. The file carries `version: 1`, `type: iso`,
  `architecture: x86_64`, `channel: stable`, `offline: false`, then `#seeds:`
  with exactly eleven commented sections in order: `applications`, `incus`,
  `install`, `migration-manager`, `network`, `operations-center`, `provider`,
  `services`, `update`, `kernel`, `security` — `grep -c` prints `11`.
- Pass/Fail: [ ]

#### CFG-12 — Interactive `init` output validity, including offline **[falsifier]**
- Promise: `internal/cli/init.go:259-291` claims the emitted body is a valid `config.Parse` input.
- Source: `internal/cli/init.go:171-229`; `internal/config/validate.go:156-164`.
- Wave: **Wave 1**
- Cost: minutes; real TTY required.
- Commands:
  ```console
  $ env -u CI "$IOB" init -o form.yaml        # raw, aarch64, channel "daily", offline no
  $ cat form.yaml; "$IOB" validate -f form.yaml --color never; echo "exit=$?"
  $ env -u CI "$IOB" init -o form-offline.yaml # iso, x86_64, empty channel, offline YES
  $ cat form-offline.yaml; "$IOB" validate -f form-offline.yaml --color never; echo "exit=$?"
  ```
- Expected: `form.yaml` is exactly `version: 1` / `image:` / `type: raw` /
  `architecture: aarch64` / `channel: daily` / `offline: false` and validates at
  `exit=0`. `form-offline.yaml` carries `offline: true` with **no** `seeds`
  block and therefore **fails**: `invalid config: seeds.applications: required
  when image.offline is true`, `exit=3`. Every offline=yes answer combination
  produces an invalid starter config. Record as **F-CFG-2** — a release decision:
  seed an `applications` entry in the interactive offline path, or qualify the
  promise in `internal/cli/init.go:260-261` and `cli.md:167-169`. Not covered by
  any existing test (`internal/cli/init_test.go:240-277` asserts the answers but
  never validates the rendered config).
- Pass/Fail: [ ]

#### CFG-13 — SOPS: decrypt the repo fixture from a file and from stdin
- Promise: a top-level `sops` key triggers in-memory decryption keyed by `SOPS_AGE_KEY`.
- Source: `internal/config/testdata/README`; `internal/config/load.go:29-57`; `sops-encryption.md` §§3–4.
- Wave: **Wave 1**
- Cost: cheap.
- Commands:
  ```console
  $ export SOPS_AGE_KEY="$(tail -n1 "$REPO/internal/config/testdata/age.key")"
  $ head -n1 "$REPO/internal/config/testdata/encrypted.yaml"; grep -c '^sops:' "$REPO/internal/config/testdata/encrypted.yaml"
  $ "$IOB" validate -f "$REPO/internal/config/testdata/encrypted.yaml" --color never; echo "exit=$?"
  $ "$IOB" validate -f - --color never < "$REPO/internal/config/testdata/encrypted.yaml"; echo "exit=$?"
  $ ls -a
  ```
- Expected: the key begins `AGE-SECRET-KEY-`; the fixture's first line begins
  `version: ENC[AES256_GCM,data:` and `grep -c` prints `1`. Both runs print
  `configuration valid`, `exit=0`, and `$WORK` gains no plaintext file. These are
  throwaway keys (`testdata/README:3-4`) — never reuse them for real secrets and
  do not export `SOPS_AGE_KEY` outside this session.
- Pass/Fail: [ ]

#### CFG-14 — SOPS: encrypt your own maximal config
- Promise: an operator-produced encrypted document behaves identically.
- Source: `internal/config/testdata/gen.go:43-54`; `sops-encryption.md` §§1–2.
- Wave: **Wave 1**
- Cost: cheap. Skip if `sops` is unavailable (PRE-03) and note the gap.
- Commands:
  ```console
  $ export AGE_RECIPIENT="$(cat "$REPO/internal/config/testdata/age.pub")"
  $ sops --age "$AGE_RECIPIENT" -e maximal.yaml > maximal.enc.yaml
  $ head -n1 maximal.enc.yaml; grep -c '^sops:' maximal.enc.yaml
  $ "$IOB" validate -f maximal.enc.yaml --json; echo "exit=$?"
  $ "$IOB" validate -f - --json < maximal.enc.yaml; echo "exit=$?"
  ```
- Expected: recipient
  `age10kg4k848vfdhvjjv04myq3rhmdhaamgpfgkg0pkq9ehu3eyf29ysfapsl8`; first line
  begins `version: ENC[AES256_GCM,data:`; `grep -c` prints `1`; both validations
  emit
  `{"result":{"valid":true,"type":"raw","architecture":"aarch64","offline":false}}`
  at `exit=0` — identical to the plaintext result in CFG-02.
- Pass/Fail: [ ]

#### CFG-15 — SOPS: no key source at all is exit 4
- Promise: absence of every key source fails at 4 with `decryption failed`.
- Source: `internal/config/load.go:37-41`; `sops-encryption.md:104-110`.
- Wave: **Wave 1**
- Cost: cheap.
- Commands:
  ```console
  $ mkdir -p "$WORK/emptyhome"
  $ env -u SOPS_AGE_KEY -u SOPS_AGE_KEY_FILE -u SOPS_AGE_KEY_CMD HOME="$WORK/emptyhome" GNUPGHOME="$WORK/emptyhome" "$IOB" validate -f maximal.enc.yaml --color never; echo "exit=$?"
  $ env SOPS_AGE_KEY= "$IOB" validate -f maximal.enc.yaml --color never; echo "exit=$?"
  $ env -u SOPS_AGE_KEY -u SOPS_AGE_KEY_FILE -u SOPS_AGE_KEY_CMD HOME="$WORK/emptyhome" GNUPGHOME="$WORK/emptyhome" "$IOB" validate -f - --color never < maximal.enc.yaml; echo "exit=$?"
  ```
- Expected: `exit=4` all three, stdout empty, stderr exactly
  `decryption failed: Error getting data key: 0 successful groups required, got 0`.
- Pass/Fail: [ ]

#### CFG-16 — Every decrypt-path failure is exit 4, never 3
- Promise: wrong key, MAC mismatch, malformed metadata, and a stray `sops` key on plaintext.
- Source: `internal/config/testdata/README:11-15`; `configuration.md:61-66,156-158`.
- Wave: **Wave 1**
- Cost: cheap.
- Commands:
  ```console
  $ cd "$REPO/internal/config/testdata"
  $ for f in encrypted-wrong-key.yaml encrypted-mac-mismatch.yaml malformed-sops.yaml stray-sops.yaml; do "$IOB" validate -f "$f" --color never; echo "$f exit=$?"; done
  $ cd "$WORK"
  $ printf 'version: 1\nimage:\n  type: iso\n  architecture: x86_64\nsops: true\n' | "$IOB" validate -f - --color never; echo "exit=$?"
  $ env SOPS_AGE_KEY= "$IOB" validate -f maximal.enc.yaml --json > d.json 2> d.txt; echo "exit=$?"; cat d.json d.txt
  ```
- Expected: `exit=4` for all five, never 3. Wrong key:
  `decryption failed: Error getting data key: 0 successful groups required, got 0`.
  MAC mismatch:
  `decryption failed: Failed to decrypt original mac: Could not decrypt with AES_GCM: cipher: message authentication failed`.
  Malformed: `decryption failed: No keys found in file`. Stray key and scalar
  `sops: true`: `decryption failed: Error unmarshalling input yaml: yaml: unmarshal errors:`
  naming `stores.Metadata` — presence of the key alone selects decryption. The
  `--json` run writes exactly
  `{"error":{"code":4,"message":"decryption failed: Error getting data key: 0 successful groups required, got 0"}}`.
- Pass/Fail: [ ]

#### CFG-17 — `SOPS_AGE_KEY_FILE` interaction **[falsifier]**
- Promise: `sops-encryption.md:36-38` says an empty `SOPS_AGE_KEY_FILE` makes SOPS ignore `SOPS_AGE_KEY`.
- Source: `sops-encryption.md:36-38`; `configuration.md:759-761`; `go.mod` `getsops/sops/v3 v3.11.0`.
- Wave: **Wave 1**
- Cost: cheap.
- Commands:
  ```console
  $ env SOPS_AGE_KEY_FILE= "$IOB" validate -f "$REPO/internal/config/testdata/encrypted.yaml" --color never; echo "exit=$?"
  $ env SOPS_AGE_KEY_FILE=/nonexistent/age.key "$IOB" validate -f "$REPO/internal/config/testdata/encrypted.yaml" --color never; echo "exit=$?"
  $ env SOPS_AGE_KEY= SOPS_AGE_KEY_FILE= "$IOB" validate -f "$REPO/internal/config/testdata/encrypted.yaml" --color never; echo "exit=$?"
  $ env SOPS_AGE_KEY= SOPS_AGE_KEY_FILE="$REPO/internal/config/testdata/age.key" "$IOB" validate -f "$REPO/internal/config/testdata/encrypted.yaml" --color never; echo "exit=$?"
  ```
- Expected: the first two succeed (`configuration valid`, `exit=0`) — a valid
  `SOPS_AGE_KEY` still works with an empty or bogus `SOPS_AGE_KEY_FILE`,
  contradicting the how-to. Third `exit=4`. Fourth `exit=0`
  (`SOPS_AGE_KEY_FILE` alone is a usable source). Record as **F-CFG-3**:
  downgrade the how-to's "unset it for this session" from a requirement to a
  hygiene note; `configuration.md:759-761` is literally true and can stay.
- Pass/Fail: [ ]

#### CFG-18 — `validate` performs no network I/O
- Promise: `validate` never fetches, even with a `--server` that cannot resolve.
- Source: `internal/cli/validate.go:36,46-48`; `sops-encryption.md:66-67`.
- Wave: **Wave 1**
- Cost: cheap (one control fetch).
- Commands:
  ```console
  $ "$IOB" validate -f maximal.yaml --server /definitely-not-a-mirror --color never; echo "exit=$?"
  $ "$IOB" validate -f maximal.yaml --server https://127.0.0.1:1/os --cache-dir /definitely/not/writable --color never; echo "exit=$?"
  $ "$IOB" build -f maximal.yaml -o out.img --server /definitely-not-a-mirror --color never; echo "exit=$?"
  $ printf '(version 1)\n(allow default)\n(deny network*)\n' > deny-net.sb
  $ "$IOB" versions --color never > /dev/null; echo "unsandboxed versions exit=$?"
  $ sandbox-exec -f deny-net.sb "$IOB" versions --color never; echo "sandboxed versions exit=$?"
  $ sandbox-exec -f deny-net.sb "$IOB" validate -f maximal.yaml --color never; echo "sandboxed validate exit=$?"
  $ SOPS_AGE_KEY="$(tail -n1 "$REPO/internal/config/testdata/age.key")" sandbox-exec -f deny-net.sb "$IOB" validate -f maximal.enc.yaml --color never; echo "sandboxed sops validate exit=$?"
  ```
- Expected: both `validate` runs with hostile `--server`/`--cache-dir` print
  `configuration valid` at `exit=0`, while the `build` control fails at `exit=2`
  with the `--server` usage error — the asymmetry is the evidence. Under
  `sandbox-exec` with network denied, `versions` fails at `exit=5`
  (`dial tcp: lookup images.linuxcontainers.org: no such host`), proving the
  sandbox works, while both plaintext and SOPS `validate` still succeed at
  `exit=0`.
- Pass/Fail: [ ]

#### CFG-19 — Every YAML example in the configuration reference behaves as documented
- Promise: the eight fenced examples in `configuration.md` are accurate.
- Source: `configuration.md:775-1035`.
- Wave: **Wave 1**
- Cost: cheap.
- Commands:
  ```console
  $ for r in 779,782 788,796 803,811 818,835 841,849 855,866 873,1022 1028,1035; do sed -n "${r}p" "$REPO/docs/docs/reference/configuration.md" | "$IOB" validate -f - --color never; echo "$r exit=$?"; done
  ```
- Expected: the first seven print `configuration valid` at `exit=0`; the last
  ("Rejected recovery keys") prints the `encryption_recovery_keys` refusal at
  `exit=3`. Any deviation means the reference drifted — re-check the line ranges
  before failing the case.
- Pass/Fail: [ ]

---

### 4.3 Live server, cache, and artifact correctness

Run **ART-05 first**: it warms `$CACHE`, and every later expensive case reuses
it. Keep one shared `--cache-dir` for the whole block.

#### ART-01 — `versions` human table against the real default server
- Promise: table columns, host-architecture default, live data.
- Source: `cli.md:144-159`; `internal/ux/render.go:117-124`.
- Wave: **Wave 1**
- Cost: cheap (~35 KB `index.json`).
- Commands:
  ```console
  $ env -u INCUSOS_BUILDER_SERVER "$IOB" versions --color never --progress never --cache-dir "$CACHE"; echo "exit=$?"
  ```
- Expected: `exit=0`; first stdout line exactly
  `Version  Channel  Architecture  Type` (two-space separator), then rows like
  `202608102114  stable  aarch64  raw`. The default architecture on this arm64
  host is `aarch64`. Nothing on stderr except the `==> index` / `done index`
  step lines.
- Pass/Fail: [ ]

#### ART-02 — `versions --json` envelope and architecture filtering
- Promise: documented entry fields; `channels` never null; no per-image `type` in the body.
- Source: `automation.md:113-139`; `internal/cli/versions.go:105-154`.
- Wave: **Wave 1**
- Cost: cheap.
- Commands:
  ```console
  $ "$IOB" versions --json --cache-dir "$CACHE" | tee versions-default.json | jq .
  $ "$IOB" versions --json --architecture x86_64 --cache-dir "$CACHE" | jq -r '.result.versions[].architectures | join(",")'
  $ "$IOB" versions --json --architecture '' --cache-dir "$CACHE" | jq -r '.result.versions[0].architectures | join(",")'
  $ "$IOB" versions --json --cache-dir "$CACHE" | jq -r '.result.versions[0] | keys | join(",")'
  ```
- Expected: one newline-terminated document per command. Entries have non-empty
  `version`, `channels` containing `stable`, an RFC 3339 `published_at`, and
  `architectures` `["aarch64"]` by default. `--architecture x86_64` prints
  `x86_64` for every release; `--architecture ''` prints both. The key list is
  exactly `architectures,channels,published_at,version`.
- Pass/Fail: [ ]

#### ART-03 — Unknown channel is an empty list, not an error
- Promise: `cli.md:146`.
- Source: `internal/cli/versions.go:108-131`.
- Wave: **Wave 1**
- Cost: cheap.
- Commands:
  ```console
  $ "$IOB" versions --json --channel nightly --cache-dir "$CACHE"; echo "exit=$?"
  $ "$IOB" versions --channel nightly --color never --cache-dir "$CACHE"; echo "exit=$?"
  ```
- Expected: `{"result":{"versions":[]}}` at `exit=0`; the human run prints only
  the header line at `exit=0`. No error envelope.
- Pass/Fail: [ ]

#### ART-04 — Empty cache directory is an acquisition failure
- Promise: `cache.md:27-28`.
- Source: `internal/update/cache.go:44-46`.
- Wave: **Wave 1**
- Cost: cheap (fails before any request).
- Commands:
  ```console
  $ "$IOB" versions --cache-dir ''; echo "exit=$?"
  $ "$IOB" versions --json --cache-dir ''
  ```
- Expected: `exit=5`; stderr exactly
  `acquisition failed: cache directory is required`; the `--json` run adds
  `{"error":{"code":5,"message":"acquisition failed: cache directory is required"}}`.
- Pass/Fail: [ ]

#### ART-05 — Real end-to-end build of the smallest live image
- Promise: the whole online build path plus the documented human summary.
- Source: `internal/cli/build.go:466-485`; `internal/cli/e2e_helpers_test.go:111-263`.
- Wave: **Wave 2**
- Cost: **expensive**: ~1–3 min wall, ~3.9 GB disk (≈415 MiB cached asset +
  3,432,026,112 B output). Re-select the smallest raw image first — the live
  index moves.
- Preconditions: ≥8 GB free; network.
- Commands:
  ```console
  $ curl -sS https://images.linuxcontainers.org/os/index.json -o index.json
  $ jq -r '[.updates[] | .version as $v | .files[] | select(.type=="image-raw") | {v:$v,a:.architecture,f:.filename,s:.size,d:.sha256}] | sort_by(.s)[0] | "\(.v) \(.a) \(.f) \(.s) \(.d)"' index.json
  $ export VER=<version from above> ARCH=<arch> DIGEST=<sha256>
  $ sed -n '113,262p' "$REPO/internal/cli/e2e_helpers_test.go" | sed '1s/.*fmt\.Sprintf(`//' | sed "s/architecture: %s/architecture: $ARCH/; s/release: %q/release: \"$VER\"/" > live11.yaml
  $ "$IOB" validate -f live11.yaml; echo "exit=$?"
  $ export OUT="$WORK/seeded.img"
  $ time "$IOB" build -f live11.yaml -o "$OUT" --cache-dir "$CACHE" --color never --progress always
  $ echo "exit=$?"; stat -f '%z %N' "$OUT"
  ```
- Expected: `configuration valid`, then `exit=0`. stderr shows, in order:
  `==> resolve`, `==> index`, `done index`, `done resolve`, `==> acquire`,
  `==> download`, `progress N% (done/total)` ending at 100%, `done download`,
  `done acquire`, `==> probe`, `done probe`, `==> seed`, `done seed`,
  `==> splice`, `done splice`. stdout is the `summary` block with rows
  `output`, `type`, `architecture`, `version`, `channel`, `seed_bytes`,
  `sha256`, two-space separated, and **no** `resources_*` rows (online config).
  `stat` reports the full decompressed image size. **Record `seed_bytes` as
  `$SEED` and the `sha256`** — ART-06/08/09/10 reuse them.
- Pass/Fail: [ ]

#### ART-06 — `--json` envelope, digest equality, cache reuse
- Promise: envelope fields; `result.sha256` == a second hash; a warm cache skips the download.
- Source: `automation.md:67-98`; `cache.md:72-78`; `internal/update/client.go:122-126`.
- Wave: **Wave 2**
- Cost: minutes; no download; ~3.2 GB written again (delete `$OUT` first if tight).
- Commands:
  ```console
  $ ls -l "$CACHE/sha256/"
  $ time "$IOB" build --json -f live11.yaml -o seeded2.img --cache-dir "$CACHE" --color never --progress always > build2.json
  $ echo "exit=$?"; cat build2.json
  $ jq -r '.result.sha256' build2.json; shasum -a 256 seeded2.img; shasum -a 256 "$OUT"
  ```
- Expected: `exit=0`. stderr shows `==> acquire` followed directly by
  `==> probe` — **no** `==> download` and no progress lines. stdout is one line
  with `output`, `type`, `architecture`, `version`, `channel`, `seed_bytes`,
  `sha256` and **no** `resources_*` keys. `.result.sha256` equals
  `shasum -a 256 seeded2.img`, which equals `$OUT`'s digest from ART-05.
- Pass/Fail: [ ]

#### ART-07 — Cache layout, content addressing, mode, no residue
- Promise: `<cache-dir>/sha256/<64hex>`, mode 0444, no `.fetch-*` temps.
- Source: `cache.md:35-57`; `internal/update/cache.go:19-27,153-161`.
- Wave: **Wave 2**
- Cost: cheap.
- Commands:
  ```console
  $ find "$CACHE" -maxdepth 2 | sort
  $ stat -f '%Sp %z %N' "$CACHE/sha256/$DIGEST"
  $ shasum -a 256 "$CACHE/sha256/$DIGEST"
  $ ls -a "$CACHE" | grep -c '^\.fetch-' || true
  ```
- Expected: only `$CACHE`, `$CACHE/sha256`, and one file per admitted blob.
  Mode `-r--r--r--`, size equal to the index-declared `size`, and the file's own
  SHA-256 equal to its filename. `sha256/` is `drwxr-xr-x`. No `.fetch-*`.
- Pass/Fail: [ ]

#### ART-08 — Extract the seed region at the documented byte offset
- Promise: `seed-data` at byte 2,148,532,224; uncompressed tar; eleven entries, mode 0600, writeSeed order.
- Source: `internal/build/probe.go:18-22`; `internal/seed/seed.go:44-78,99-120`; `seed-injection.md:26-28,56-63`.
- Wave: **Wave 2**
- Cost: cheap.
- Arithmetic (do not guess): offset `O = 2148532224`; `O/512 = 4196352`;
  `O/1048576 = 2049` exactly; tar length is always a multiple of 512, so
  `count = $SEED/512` at `bs=512`.
- Commands:
  ```console
  $ export SEED=<seed_bytes from ART-05>
  $ echo "offset=2148532224 lba512=$((2148532224/512)) mib=$((2148532224/1048576)) blocks=$((SEED/512))"
  $ dd if="$OUT" of=seed.tar bs=512 skip=4196352 count=$((SEED/512)) status=none
  $ wc -c seed.tar
  $ tar -tvf seed.tar
  $ tar -tf seed.tar | tr '\n' ' '
  ```
- Expected: `lba512=4196352 mib=2049`; `wc -c` equals `$SEED`; `$SEED` ≤
  104857600 (the 100 MiB partition). `tar -tvf` lists exactly eleven regular
  files, each line beginning `-rw-------  0 0` (mode 0600, uid/gid 0). `tar -tf`
  order is exactly `applications.yaml incus.yaml operations-center.yaml
  migration-manager.yaml install.yaml network.yaml provider.yaml services.yaml
  update.yaml kernel.yaml security.yaml`. Compare against the committed golden
  with `tar -tvf "$REPO/internal/seed/testdata/nine-section.golden.tar"`.
- Pass/Fail: [ ]

#### ART-09 — Splice invariant: everything outside the tar is untouched
- Promise: only `[offset, offset+len(tar))` is rewritten.
- Source: `seed-injection.md:69-85`; `internal/build/build.go:154-157`.
- Wave: **Wave 2**
- Cost: expensive: ~1–2 min, +3.2 GiB temporary disk.
- Commands:
  ```console
  $ time gunzip -c "$CACHE/sha256/$DIGEST" > stock.img
  $ stat -f '%z %N' stock.img "$OUT"
  $ cmp -n 2148532224 stock.img "$OUT"; echo "prefix_exit=$?"
  $ END=$((2148532224+SEED)); cmp -i $END:$END stock.img "$OUT"; echo "suffix_exit=$?"
  $ dd if=stock.img bs=512 skip=4196352 count=$((SEED/512)) status=none | shasum -a 256
  $ shasum -a 256 seed.tar
  ```
- Expected: both files are the same size. `prefix_exit=0` and `suffix_exit=0`
  with no output — the ESP before and the verity/OS partitions after the tar are
  byte-identical. The two `shasum` values **differ** (the stock image does not
  contain the seed tar).
- Pass/Fail: [ ]

#### ART-10 — `.gz` output: digest covers the compressed stored bytes
- Promise: `cli.md:92-94`, `automation.md:97-98`.
- Source: `internal/cli/build.go:389-400`.
- Wave: **Wave 2**
- Cost: expensive: ~1–2 min recompression, ~450 MiB; no download.
- Commands:
  ```console
  $ "$IOB" build --json -f live11.yaml -o seeded.img.gz --cache-dir "$CACHE" --color never --progress never > buildgz.json
  $ jq -r '.result.sha256' buildgz.json; shasum -a 256 seeded.img.gz
  $ gzip -t seeded.img.gz; echo "gzip_test_exit=$?"
  $ gunzip -c seeded.img.gz | wc -c
  $ gunzip -c seeded.img.gz | shasum -a 256; shasum -a 256 "$OUT"
  ```
- Expected: `result.sha256` equals the digest of the **compressed** file
  (gzip footer included), `gzip -t` exits 0, the decompressed length equals the
  ART-05 image size, and the decompressed digest equals `$OUT`'s.
- Pass/Fail: [ ]

#### ART-11 — `--cache-dir` override and env precedence
- Promise: `cache.md:17-25`; flag beats env beats the macOS default.
- Source: `internal/cli/root.go:166-209`.
- Wave: **Wave 2**
- Cost: minutes plus one extra ~415 MiB download for the fresh-cache half.
- Commands:
  ```console
  $ ls -d "$HOME/Library/Caches/incusos-builder" 2>/dev/null; echo "default_dir_exit=$?"
  $ export CACHE2="$WORK/cache-env" && mkdir -p "$CACHE2"
  $ INCUSOS_BUILDER_CACHE_DIR="$CACHE2" "$IOB" build --json -f live11.yaml -o seeded-env.img --color never --progress always 2>env.err >/dev/null
  $ grep -c '==> download' env.err; ls "$CACHE2/sha256/"
  $ INCUSOS_BUILDER_CACHE_DIR=/nonexistent-should-be-ignored "$IOB" build --json -f live11.yaml -o seeded-flag.img --cache-dir "$CACHE" --color never --progress always 2>flag.err >/dev/null
  $ grep -c '==> download' flag.err; ls -d /nonexistent-should-be-ignored 2>&1
  ```
- Expected: the env run downloads once and creates `$CACHE2/sha256/$DIGEST`; the
  flag run prints `0` download lines and never creates
  `/nonexistent-should-be-ignored`. Both exit 0.
- Pass/Fail: [ ]

#### ART-12 — Local mirror: build with `--server <directory>`, network off
- Promise: a directory mirror serves the same build through the same cache.
- Source: `use-local-mirror.md:23-76`; `internal/update/local.go:38-67`.
- Wave: **Wave 2**
- Cost: minutes; ~415 MiB mirror copy + 3.2 GiB output; no download.
- Commands:
  ```console
  $ export MIRROR="$WORK/mirror-full" && mkdir -p "$MIRROR/$VER/$ARCH"
  $ cp index.json "$MIRROR/index.json"
  $ cp "$CACHE/sha256/$DIGEST" "$MIRROR/$VER/$ARCH/IncusOS_$VER.img.gz"
  $ shasum -a 256 "$MIRROR/$VER/$ARCH/IncusOS_$VER.img.gz"; find "$MIRROR" | sort
  $ networksetup -setairportpower en0 off
  $ export MCACHE="$WORK/cache-mirror" && mkdir -p "$MCACHE"
  $ "$IOB" versions --server "$MIRROR" --cache-dir "$MCACHE" --architecture "$ARCH" --color never
  $ "$IOB" build --json -f live11.yaml -o mirror-seeded.img --server "$MIRROR" --cache-dir "$MCACHE" --color never --progress always; echo "exit=$?"
  $ networksetup -setairportpower en0 on
  ```
- Expected: the copied file hashes to `$DIGEST`. With Wi-Fi off, `versions`
  still prints the table and `build` exits 0 with the same `sha256` as ART-05.
  `$MCACHE/sha256/$DIGEST` is created mode 0444 — a local server still admits
  into the cache.
- Pass/Fail: [ ]

#### ART-13 — Tampered mirror byte is refused at admission, with no residue
- Promise: `asset failed size/digest admission; untrusted metadata; possible tampering`, exit 5.
- Source: `internal/update/cache.go:143-149`; `use-local-mirror.md:154-176`.
- Wave: **Wave 2**
- Cost: minutes (hashes 415 MiB locally).
- Preconditions: **a fresh cache directory is mandatory** — a warm cache
  short-circuits before the mirror file is read and the tamper would go
  undetected by design.
- Commands:
  ```console
  $ cp "$MIRROR/$VER/$ARCH/IncusOS_$VER.img.gz" good.img.gz
  $ printf '\xff' | dd of="$MIRROR/$VER/$ARCH/IncusOS_$VER.img.gz" bs=1 seek=1000000 count=1 conv=notrunc status=none
  $ shasum -a 256 "$MIRROR/$VER/$ARCH/IncusOS_$VER.img.gz"
  $ export TCACHE="$WORK/cache-tamper" && rm -rf "$TCACHE" && mkdir -p "$TCACHE"
  $ "$IOB" build --json -f live11.yaml -o tamper.img --server "$MIRROR" --cache-dir "$TCACHE" --color never --progress never; echo "exit=$?"
  $ find "$TCACHE" | sort; ls -a "$TCACHE"; ls tamper.img 2>&1
  $ cp good.img.gz "$MIRROR/$VER/$ARCH/IncusOS_$VER.img.gz"
  ```
- Expected: the tampered digest differs. `exit=5`; stderr exactly
  `acquisition failed: "<arch>/IncusOS_<ver>.img.gz": asset failed size/digest admission; untrusted metadata; possible tampering`;
  the JSON envelope carries the same message at code 5. `$TCACHE/sha256` exists
  but is **empty**, no `.fetch-*` temp remains, and `tamper.img` was never
  created. Note the local adapter makes exactly one attempt; the retry ladder is
  HTTPS-only.
- Pass/Fail: [ ]

#### ART-14 — Metadata-rejection matrix on a synthetic tiny mirror
- Promise: allowlist before any URL/filesystem use; missing index/asset messages.
- Source: `internal/update/validate.go:10-74`; `internal/update/local.go:117,136`; `cache.md:59-70`.
- Wave: **Wave 1**
- Cost: cheap (a few KB; every case fails before a large read).
- Commands:
  ```console
  $ export TINY="$WORK/tiny" && rm -rf "$TINY" && mkdir -p "$TINY/202601010000/aarch64"
  $ dd if=/dev/zero of="$TINY/202601010000/aarch64/IncusOS_202601010000.img.gz" bs=1024 count=1 status=none
  $ export TD=$(shasum -a 256 "$TINY/202601010000/aarch64/IncusOS_202601010000.img.gz" | cut -d' ' -f1)
  $ mk() { jq -n --arg d "$1" --argjson s "$2" --arg f "$3" '{format:"1.0",updates:[{format:"1.0",version:"202601010000",channels:["stable"],published_at:"2026-01-01T00:00:00Z",origin:"local",severity:"none",url:"",files:[{architecture:"aarch64",component:"os",filename:$f,sha256:$d,size:$s,type:"image-raw"}]}]}' > "$TINY/index.json"; }
  $ printf 'version: 1\nimage:\n  type: raw\n  architecture: aarch64\n  channel: stable\n' > tiny.yaml
  $ export TC="$WORK/cache-tiny"
  $ run() { rm -rf "$TC"; "$IOB" build -f tiny.yaml -o tiny.img --server "$TINY" --cache-dir "$TC" --color never --progress never; echo "exit=$?"; }
  $ mk "$TD" 1024 "aarch64/IncusOS_202601010000.img.gz"; run          # a) valid metadata
  $ mk "abc123" 1024 "aarch64/IncusOS_202601010000.img.gz"; run       # b) short sha256
  $ mk "$(echo "$TD" | tr 'a-f' 'A-F')" 1024 "aarch64/IncusOS_202601010000.img.gz"; run  # c) uppercase
  $ mk "$TD" 0 "aarch64/IncusOS_202601010000.img.gz"; run             # d) size 0
  $ mk "$TD" 8589934593 "aarch64/IncusOS_202601010000.img.gz"; run    # e) size > 8 GiB
  $ mk "$TD" 1024 "../evil.img.gz"; run                               # f) path traversal
  $ mk "$TD" 2048 "aarch64/IncusOS_202601010000.img.gz"; run          # g) declared size too large
  $ mk "$TD" 1024 "aarch64/absent.img.gz"; run                        # h) missing asset
  $ mv "$TINY/index.json" "$TINY/index.json.bak"; run; mv "$TINY/index.json.bak" "$TINY/index.json"  # i) missing index
  ```
- Expected: all `exit=5`.
  (a) `acquisition failed: gzip: …` — admission **passed** and the GPT probe
  rejected the fake payload; `$TC/sha256/$TD` exists mode 0444.
  (b)(c) `acquisition failed: sha256 "…" rejected; untrusted metadata; possible tampering`
  (64 lowercase hex only). (d) `size 0 rejected; …`.
  (e) `size 8589934593 rejected; …` (bound is 8 GiB = 8589934592).
  (f) `filename "../evil.img.gz" rejected; …`, and nothing is created outside
  `$TINY`. (g) the admission wording from ART-13.
  (h) `acquisition failed: open aarch64/absent.img.gz: …no such file or directory`.
  (i) `acquisition failed: open index.json: …no such file or directory`.
  For (b)–(f) the cache is never touched at all.
- Pass/Fail: [ ]

#### ART-15 — Pin resolution: highest-in-channel by default, exact pin when set
- Promise: `configuration.md:126-128`.
- Source: `internal/build/resolve.go:68-90`.
- Wave: **Wave 1**
- Cost: cheap (assets are deliberately absent; the error names the selected file).
- Commands:
  ```console
  $ export PIN="$WORK/pinmirror" && rm -rf "$PIN" && mkdir -p "$PIN"
  $ jq -n '{format:"1.0",updates:[
      {format:"1.0",version:"202601010000",channels:["stable"],published_at:"2026-01-01T00:00:00Z",origin:"local",severity:"none",url:"",files:[{architecture:"aarch64",component:"os",filename:"aarch64/old.img.gz",sha256:"'"$(printf 'a%.0s' $(seq 64))"'",size:1024,type:"image-raw"}]},
      {format:"1.0",version:"202602020000",channels:["stable"],published_at:"2026-02-02T00:00:00Z",origin:"local",severity:"none",url:"",files:[{architecture:"aarch64",component:"os",filename:"aarch64/new.img.gz",sha256:"'"$(printf 'b%.0s' $(seq 64))"'",size:1024,type:"image-raw"}]}]}' > "$PIN/index.json"
  $ "$IOB" versions --server "$PIN" --cache-dir "$WORK/c1" --architecture aarch64 --color never
  $ "$IOB" build -f tiny.yaml -o p.img --server "$PIN" --cache-dir "$WORK/c1" --color never --progress never; echo "exit=$?"
  $ sed 's/^  channel: stable/  channel: stable\n  release: "202601010000"/' tiny.yaml > pinned.yaml
  $ "$IOB" build -f pinned.yaml -o p.img --server "$PIN" --cache-dir "$WORK/c1" --color never --progress never; echo "exit=$?"
  ```
- Expected: `versions` lists both releases. The unpinned build fails at `exit=5`
  naming **`aarch64/new.img.gz`** (highest version string wins); the pinned build
  fails naming **`aarch64/old.img.gz`** (the pin overrode it). Nothing downloads.
- Pass/Fail: [ ]

#### ART-16 — A pin the live index does not carry
- Promise: `version not found: release "<pin>" not in channel "<chan>"; available: <sorted list>`, exit 5.
- Source: `internal/build/resolve.go:94-103,198-228`; `use-local-mirror.md:146-152`.
- Wave: **Wave 1**
- Cost: cheap (only `index.json` is fetched).
- Commands:
  ```console
  $ sed 's/^  channel: stable/  channel: stable\n  release: "199901010000"/' tiny.yaml > badpin.yaml
  $ "$IOB" build --json -f badpin.yaml -o np.img --cache-dir "$CACHE" --color never --progress never; echo "exit=$?"
  ```
- Expected: `exit=5`; stderr and `error.message` are exactly
  `version not found: release "199901010000" not in channel "stable"; available: <versions ascending>`.
  The list is sorted ascending and de-duplicated, not index order — compare it
  against ART-02's output.
- Pass/Fail: [ ]

#### ART-17 — Interrupt a build (SIGINT): exit code, no partial artifact, clean re-run
- Promise: cancelled fetch is exit 5; nothing partial at the final path; temps cleaned.
- Source: `cmd/incusos-builder/main.go:26`; `internal/cli/publish.go:246-253`; `recover-interrupted-build.md:53-56`.
- Wave: **Wave 2**
- Cost: minutes with a warm cache. Interrupt during **splice**, not download.
- Commands:
  ```console
  $ export IDIR="$WORK/interrupt" && rm -rf "$IDIR" && mkdir -p "$IDIR"
  $ "$IOB" build -f live11.yaml -o "$IDIR/out.img" --cache-dir "$CACHE" --color never --progress always &
  # once `==> splice` appears on stderr:
  $ kill -INT %1; wait %1; echo "exit=$?"
  $ ls -la "$IDIR"; find "$IDIR" -name '.out.img-*.tmp'
  $ ls -a "$CACHE" | grep '^\.fetch-' || echo "no fetch temps"
  $ ls -l "$CACHE/sha256/$DIGEST"
  $ "$IOB" build --json -f live11.yaml -o "$IDIR/out.img" --cache-dir "$CACHE" --color never --progress never | jq -r '.result.sha256'
  $ shasum -a 256 "$IDIR/out.img"
  ```
- Expected: the interrupted run exits **5** with
  `acquisition failed: context canceled`. `$IDIR` contains **no** `out.img` and
  **no** `.out.img-*.tmp` (the deferred `Abort()` removed it). No `.fetch-*` in
  `$CACHE`; the cache entry is still intact at mode 0444. The re-run exits 0 and
  its `sha256` matches both ART-05's digest and `shasum` of the new file.
- Pass/Fail: [ ]

#### ART-18 — `--force` backups and the documented restore drill
- Promise: refusal without `--force`; `.incusos-builder.bak` created and removed; documented recovery works.
- Source: `recover-interrupted-build.md:29-61,102-198`; `internal/cli/publish.go:489-585`.
- Wave: **Wave 2**
- Cost: minutes (two builds against a warm cache; delete between runs to bound disk).
- Commands:
  ```console
  $ export FDIR="$WORK/force" && rm -rf "$FDIR" && mkdir -p "$FDIR"
  $ "$IOB" build -f live11.yaml -o "$FDIR/out.img" --cache-dir "$CACHE" --color never --progress never
  $ shasum -a 256 "$FDIR/out.img" | tee "$FDIR/gen1.sha"
  $ "$IOB" build --no-input -f live11.yaml -o "$FDIR/out.img" --cache-dir "$CACHE" --color never --progress never; echo "exit=$?"
  $ "$IOB" build --force -f live11.yaml -o "$FDIR/out.img" --cache-dir "$CACHE" --color never --progress never; echo "exit=$?"
  $ ls -la "$FDIR"
  $ mv "$FDIR/out.img" "$FDIR/out.img.incusos-builder.bak"; : > "$FDIR/out.img"
  $ for p in "$FDIR/out.img" "$FDIR/out.img.incusos-builder.bak"; do [ -e "$p" ] && ls -l -- "$p" && shasum -a 256 -- "$p"; done
  $ if [ -e "$FDIR/out.img.incusos-builder.bak" ]; then if [ ! -e "$FDIR/out.img" ]; then mv -- "$FDIR/out.img.incusos-builder.bak" "$FDIR/out.img"; elif [ ! -s "$FDIR/out.img" ]; then rm -- "$FDIR/out.img"; mv -- "$FDIR/out.img.incusos-builder.bak" "$FDIR/out.img"; fi; fi
  $ ls -la "$FDIR"; shasum -a 256 "$FDIR/out.img"; cat "$FDIR/gen1.sha"
  ```
- Expected: the no-`--force` run exits 2 with
  `usage error: refusing to overwrite <…>/out.img; re-run with --force` and does
  not touch the file. The `--force` run exits 0 and afterwards **no**
  `*.incusos-builder.bak` and no `.out.img-*.tmp` remain. The drill's inventory
  shows a 0-byte claim plus a full-size `.bak`; the documented restore block
  (`recover-interrupted-build.md:137-146`, with `sha256sum` replaced by
  `shasum -a 256` on macOS) restores the digest recorded in `gen1.sha` and
  removes the `.bak`.
- Pass/Fail: [ ]

#### ART-19 — Offline three-way metadata binding (tamper test)
- Promise: `update.sjson` cleartext must list every selected Filename+Sha256; `update.json` Version must match.
- Source: `internal/update/metadata.go:105-159`; `use-local-mirror.md:129-132,178-181`.
- Wave: **Wave 2**
- Cost: **expensive**: ~2–4 min, ~4.2 GB. Metadata is validated after the image
  is acquired and spliced, so a fake image cannot shortcut it.
- Commands:
  ```console
  $ base=https://images.linuxcontainers.org/os/$VER
  $ curl -sS -o "$MIRROR/$VER/$ARCH/debug.raw.gz" "$base/$ARCH/debug.raw.gz"
  $ curl -sS -o "$MIRROR/$VER/update.json"  "$base/update.json"
  $ curl -sS -o "$MIRROR/$VER/update.sjson" "$base/update.sjson"
  $ shasum -a 256 "$MIRROR/$VER/$ARCH/debug.raw.gz"
  $ printf 'version: 1\nimage:\n  type: raw\n  architecture: %s\n  channel: stable\n  release: "%s"\n  offline: true\nseeds:\n  applications:\n    applications:\n      - name: debug\n' "$ARCH" "$VER" > offline-mirror.yaml
  $ "$IOB" build --json -f offline-mirror.yaml -o off.img --resources-output off.res.img --server "$MIRROR" --cache-dir "$MCACHE" --color never --progress never; echo "exit=$?"
  $ cp "$MIRROR/$VER/update.sjson" update.sjson.good
  $ sed "s#$ARCH/debug.raw.gz#$ARCH/debug-NOPE.raw.gz#" update.sjson.good > "$MIRROR/$VER/update.sjson"
  $ rm -f off.img off.res.img
  $ "$IOB" build -f offline-mirror.yaml -o off.img --resources-output off.res.img --server "$MIRROR" --cache-dir "$MCACHE" --color never --progress never; echo "exit=$?"
  $ cp update.sjson.good "$MIRROR/$VER/update.sjson"
  $ cp "$MIRROR/$VER/update.json" update.json.good
  $ sed "s/\"$VER\"/\"209901010000\"/" update.json.good > "$MIRROR/$VER/update.json"
  $ rm -f off.img off.res.img
  $ "$IOB" build -f offline-mirror.yaml -o off.img --resources-output off.res.img --server "$MIRROR" --cache-dir "$MCACHE" --color never --progress never; echo "exit=$?"
  $ cp update.json.good "$MIRROR/$VER/update.json"; ls off.img off.res.img 2>&1
  ```
- Expected: the baseline exits 0 with both digests in the envelope. Tamper 1
  exits 5 with
  `acquisition failed: update.sjson missing selected file "<arch>/debug.raw.gz" sha256 "<digest>"; untrusted metadata; possible tampering`.
  Tamper 2 exits 5 with
  `acquisition failed: update.json version "209901010000" != "<VER>"; untrusted metadata; possible tampering`.
  After each failure neither `off.img` nor `off.res.img` exists. Note the sjson
  signature itself is not verified — only the multipart structure and cleartext.
- Pass/Fail: [ ]

#### ART-20 — Low-free-space warning on the cache filesystem
- Promise: `cache.md:92-95` — a warning, never an error.
- Source: `internal/update/cache.go:218-232`.
- Wave: **Wave 2**
- Cost: minutes; a 128 MiB RAM disk; the download aborts partway.
- Commands:
  ```console
  $ DEV=$(hdiutil attach -nomount ram://262144) && echo "$DEV"
  $ diskutil erasevolume HFS+ CACHESMALL "$DEV"
  $ df -k /Volumes/CACHESMALL
  $ "$IOB" build -f live11.yaml -o small.img --cache-dir /Volumes/CACHESMALL/cache --color never --progress never; echo "exit=$?"
  $ diskutil eject /Volumes/CACHESMALL
  ```
- Expected: stderr contains the two literal lines
  `==> warning: cache free space below asset size` and
  `done warning: cache free space below asset size`, emitted **before** the
  download and not treated as an error. The build then fails when the volume
  fills: `exit=5`, `acquisition failed: write cache temp: …no space left on device`.
  No output file is published.
- Pass/Fail: [ ]

#### ART-21 — `build -o -` streams image bytes to stdout
- Promise: no summary, no rescue media, bytes only.
- Source: `automation.md:208`; `internal/cli/build.go` `streamBuild`; `build_stdout.txtar`.
- Wave: **Wave 2**
- Cost: expensive: ~3.2 GiB written to the redirect target; no download.
- Commands:
  ```console
  $ "$IOB" build -f live11.yaml -o - --cache-dir "$CACHE" --color never --progress never > streamed.img 2> stream.err; echo "exit=$?"
  $ shasum -a 256 streamed.img; shasum -a 256 "$OUT"
  $ file streamed.img; grep -c 'sha256' stream.err
  ```
- Expected: `exit=0`; `streamed.img` and `$OUT` have identical SHA-256;
  `file` reports a disk image, not text; no summary key appears on stdout and no
  JSON document is appended to the stream.
- Pass/Fail: [ ]

---

### 4.4 Rescue media and offline builds

#### MED-01 — Offline config refuses `-o -`
- Wave: **Wave 1**
- Promise / Source: `internal/cli/build.go:252-257`; `build_offline.txtar:5-6`. Cost: cheap.
- Commands: `"$IOB" build -f offline.yaml -o - --cache-dir "$CACHE"; echo "exit=$?"`
- Expected: stderr exactly `usage error: offline builds cannot use -o -`, `exit=2`, no files created.
- Pass/Fail: [ ]

#### MED-02 — Offline without applications is rejected before download
- Wave: **Wave 1**
- Promise / Source: `internal/config/validate.go:160-161`; `build-offline-media.md:47-58`. Cost: cheap.
- Commands:
  ```console
  $ sed '/applications:/,$d' offline.yaml > missing-apps.yaml
  $ "$IOB" build -f missing-apps.yaml -o out.img --cache-dir "$CACHE"; echo "exit=$?"
  ```
- Expected: `invalid config: seeds.applications: required when image.offline is true`, `exit=3`, no `out.img`.
- Pass/Fail: [ ]

#### MED-03 — `--resources-output` on an online config
- Wave: **Wave 1**
- Promise / Source: `internal/cli/publish.go:279-284`. Cost: cheap.
- Commands: `"$IOB" build -f good.yaml -o out.img --resources-output r.img --cache-dir "$CACHE"; echo "exit=$?"`
- Expected: `usage error: --resources-output requires offline: true in the config`, `exit=2`.
- Pass/Fail: [ ]

#### MED-04 — `--resources-output -`
- Wave: **Wave 1**
- Promise / Source: `internal/cli/publish.go:286-291`; `automation.md:211-212`. Cost: cheap.
- Commands: `"$IOB" build -f offline.yaml -o out.img --resources-output - --cache-dir "$CACHE"; echo "exit=$?"`
- Expected: `usage error: resources path cannot be -`, `exit=2`.
- Pass/Fail: [ ]

#### MED-05 — Image and rescue paths must be distinct after cleaning
- Wave: **Wave 1**
- Promise / Source: `internal/cli/publish.go:285-297`. Cost: cheap.
- Commands:
  ```console
  $ "$IOB" build -f offline.yaml -o out.img --resources-output out.img --cache-dir "$CACHE"; echo "exit=$?"
  $ "$IOB" build -f offline.yaml -o out.img --resources-output ./out.img --cache-dir "$CACHE"; echo "exit=$?"
  ```
- Expected: both `usage error: image and resources paths must be distinct`, `exit=2` (the second proves the check runs after `filepath.Clean`).
- Pass/Fail: [ ]

#### MED-06 — Default rescue-media naming matrix (no download)
- Promise: `<stem>.resources.<ext>` where `<stem>` drops only the last extension and `<ext>` follows `image.type`, not the `-o` suffix.
- Source: `internal/cli/publish.go:301-318`; `cli.md:87-90`.
- Wave: **Wave 1**
- Cost: cheap. The overwrite pre-check runs before any download and names the
  computed path, so pre-creating the expected name makes the refusal the oracle.
- Commands:
  ```console
  $ sed 's/type: raw/type: iso/' offline.yaml > offline-iso.yaml
  $ : > a.resources.img     && "$IOB" build -f offline.yaml     -o a.img    --cache-dir "$CACHE"; echo "exit=$?"
  $ : > b.resources.iso     && "$IOB" build -f offline-iso.yaml -o b.iso    --cache-dir "$CACHE"; echo "exit=$?"
  $ : > c.img.resources.img && "$IOB" build -f offline.yaml     -o c.img.gz --cache-dir "$CACHE"; echo "exit=$?"
  $ : > d.resources.img     && "$IOB" build -f offline.yaml     -o d        --cache-dir "$CACHE"; echo "exit=$?"
  $ : > e.resources.iso     && "$IOB" build -f offline-iso.yaml -o e.img    --cache-dir "$CACHE"; echo "exit=$?"
  ```
- Expected: each exits 2 with
  `usage error: refusing to overwrite <the pre-created path>; re-run with --force`
  naming exactly `a.resources.img`, `b.resources.iso`, `c.img.resources.img`,
  `d.resources.img`, `e.resources.iso`. The `e` line proves the extension
  follows `image.type`; the `c` line proves only the last extension is stripped.
  Every pre-created file is still 0 bytes.
- Pass/Fail: [ ]

#### MED-07 — Offline raw build publishes both artifacts with matching digests
- Promise: two artifacts, both digests in the envelope, both matching on disk.
- Source: `build-offline-media.md:60-133`; `internal/cli/publish.go:436-444`.
- Wave: **Wave 2**
- Cost: **expensive**: ~10–20 min, ~4.5 GB. Run under a non-default umask so
  MED-16 rides along.
- Commands:
  ```console
  $ ( umask 077; "$IOB" build --json -f offline.yaml -o "$WORK/seeded-off.img" --cache-dir "$CACHE" --color never --progress never ) > envelope.json
  $ echo "exit=$?"; cat envelope.json
  $ shasum -a 256 "$WORK/seeded-off.img" "$WORK/seeded-off.resources.img"
  $ stat -f '%z %Lp %N' "$WORK/seeded-off.img" "$WORK/seeded-off.resources.img"
  ```
- Expected: `exit=0`; one JSON line with `output`, `resources_output` (the
  default `seeded-off.resources.img`), `type`, `architecture`, `version`,
  `channel`, `seed_bytes`, and 64-hex `sha256` / `resources_sha256`. Both
  `shasum` values equal the envelope fields. `resources_sha256` is **not**
  `e3b0c442…b855` (the empty-input digest) and the rescue file is 275,901,440
  bytes — together these prove the publisher hashed the replaced inode. Record
  `result.version` as `$V`; MED-10/12/15 need it.
- Pass/Fail: [ ]

#### MED-08 — Raw media is GPT + one `RESCUE_DATA` partition at 1 MiB
- Promise: protective MBR, `EFI PART`, FirstLBA 2048, `Microsoft Basic Data`, FAT32.
- Source: `internal/media/fat.go:15-53`; `internal/media/rescue.go:38-43,63-69`.
- Wave: **Wave 2**
- Cost: cheap.
- Commands:
  ```console
  $ R="$WORK/seeded-off.resources.img"
  $ dd if=$R bs=1 skip=450 count=1 2>/dev/null | od -An -tx1        # protective MBR type
  $ dd if=$R bs=1 skip=512 count=8 2>/dev/null                      # GPT signature
  $ dd if=$R bs=1 skip=1056 count=8 2>/dev/null | od -An -tu8       # entry0 FirstLBA
  $ dd if=$R bs=1 skip=1064 count=8 2>/dev/null | od -An -tu8       # entry0 LastLBA
  $ dd if=$R bs=1 skip=1080 count=24 2>/dev/null | tr -d '\000'     # entry0 name
  $ dd if=$R bs=1 skip=1048587 count=2 2>/dev/null | od -An -tu2    # BPB_BytsPerSec
  $ dd if=$R bs=1 skip=1048658 count=8 2>/dev/null                  # BS_FilSysType
  $ hdiutil attach -imagekey diskimage-class=CRawDiskImage -nomount $R
  $ diskutil list /dev/diskN
  ```
- Expected: type byte `ee`; `EFI PART`; FirstLBA `2048`; LastLBA `536821`;
  partition name `RESCUE_DATA`; `BytsPerSec` `512`; filesystem type `FAT32   `.
  `diskutil list` shows `GUID_partition_scheme` with one `Microsoft Basic Data`
  partition named `RESCUE_DATA`. No `sudo` needed; do not use `gpt -r show`.
  Detach with `hdiutil detach /dev/diskN` when done with MED-10.
- Pass/Fail: [ ]

#### MED-09 — Raw media size is the exact proportional-with-floor value
- Promise: partition = `max(content+1 MiB, 256 MiB)` × 51/50, 512-aligned; disk = +2 MiB.
- Source: `internal/media/fat.go:56-73`; `internal/media/doc.go:27-35`.
- Wave: **Wave 2**
- Cost: cheap.
- Commands:
  ```console
  $ stat -f %z "$WORK/seeded-off.resources.img"
  $ dd if="$WORK/seeded-off.resources.img" bs=1 skip=1048576 count=512 2>/dev/null | od -An -tx1 -j510 -N2
  ```
- Expected: exactly `275901440` bytes for any payload ≤ 255 MiB, and boot-sector
  signature `55 aa` at partition start + 510. A different size means either the
  sizing constants changed or the payload exceeded 255 MiB — recompute before
  failing.
- Pass/Fail: [ ]

#### MED-10 — Raw media read-back with exact logical byte counts
- Promise: exactly three files, `aarch64/` prefix preserved, byte-identical to source, no `hotfix.sh.sig`.
- Source: `internal/media/rescue.go:253-271`; `internal/cli/e2e_helpers_test.go:500-527`.
- Wave: **Wave 2**
- Cost: cheap.
- Commands:
  ```console
  $ diskutil mount /dev/diskNs1            # mounts at /Volumes/RESCUE_DATA
  $ find /Volumes/RESCUE_DATA -type f | sort
  $ stat -f '%z %N' /Volumes/RESCUE_DATA/update/update.json /Volumes/RESCUE_DATA/update/update.sjson /Volumes/RESCUE_DATA/update/$ARCH/*.gz
  $ curl -sS -o src.update.json  https://images.linuxcontainers.org/os/$V/update.json
  $ curl -sS -o src.update.sjson https://images.linuxcontainers.org/os/$V/update.sjson
  $ cmp src.update.json  /Volumes/RESCUE_DATA/update/update.json
  $ cmp src.update.sjson /Volumes/RESCUE_DATA/update/update.sjson
  $ hdiutil detach /dev/diskN
  ```
- Expected: volume name `RESCUE_DATA` (what IncusOS recovery looks up). Exactly
  three files: `update/update.json`, `update/update.sjson`, and
  `update/<arch>/<app>.gz`. No `hotfix.sh.sig`. Both `cmp` calls are silent and
  exit 0. **Compare exact logical byte counts, never a whole-cluster read**:
  go-diskfs FAT `File.Read` can return final-cluster zero slack to `io.ReadAll`,
  so a padded read digests trailing zeros and looks like corruption on correct
  media. Use the mounted file (whose directory entry carries the logical size),
  not `dd` across clusters.
- Pass/Fail: [ ]

#### MED-11 — Offline ISO build publishes ISO rescue media
- Promise: `.iso` default naming, both digests.
- Source: `build-offline-media.md:135-154`; `internal/cli/publish.go:309-318`.
- Wave: **Wave 2**
- Cost: expensive: ~10–20 min, ~2 GB (application asset reused from `$CACHE`).
- Commands:
  ```console
  $ "$IOB" build --json -f offline-iso.yaml -o "$WORK/seeded.iso" --cache-dir "$CACHE" --color never --progress never > iso-envelope.json
  $ echo "exit=$?"; cat iso-envelope.json
  $ shasum -a 256 "$WORK/seeded.iso" "$WORK/seeded.resources.iso"
  ```
- Expected: `exit=0`; `resources_output` ends `seeded.resources.iso`;
  `"type":"iso"`; both digests match the envelope.
- Pass/Fail: [ ]

#### MED-12 — ISO carries Rock Ridge long names and exact bytes
- Promise: the intended file set with full case-sensitive names.
- Source: `internal/media/iso.go:44-58`; `docs/notes/spike-1b-rescue.md:121-141`.
- Wave: **Wave 2**
- Cost: cheap. `bsdtar` (libarchive) is the independent Rock Ridge reader.
- Commands:
  ```console
  $ bsdtar -tvf "$WORK/seeded.resources.iso"
  $ bsdtar -xOf "$WORK/seeded.resources.iso" update/update.json | wc -c
  $ bsdtar -xOf "$WORK/seeded.resources.iso" update/update.sjson | cmp - src.update.sjson
  $ bsdtar -xOf "$WORK/seeded.resources.iso" update/$ARCH/<app>.gz | shasum -a 256
  ```
- Expected: the listing shows exactly the three files with long names plus their
  directories, and no `hotfix.sh.sig`. `wc -c` equals the source `update.json`
  byte count; `cmp` is silent; the asset digest equals the `sha256` `index.json`
  declares for that file. Do not "verify" by `dd`-ing raw 2048-byte blocks.
- Pass/Fail: [ ]

#### MED-13 — ISO is 2048-aligned, PVD-truncated, labeled, and macOS-recognizable
- Promise: `internal/media/iso.go:66-101`.
- Wave: **Wave 2**
- Cost: cheap.
- Commands:
  ```console
  $ I="$WORK/seeded.resources.iso"; S=$(stat -f %z $I); echo "size=$S remainder=$((S % 2048))"
  $ dd if=$I bs=1 skip=32769 count=5 2>/dev/null                 # standard identifier
  $ dd if=$I bs=1 skip=32808 count=32 2>/dev/null | od -c | sed -n 1,3p
  $ P=$(dd if=$I bs=1 skip=32848 count=4 2>/dev/null | od -An -tu4 | tr -d ' '); echo "pvd_bytes=$((P * 2048))"
  $ hdiutil attach -nomount $I && hdiutil detach /dev/diskN
  ```
- Expected: `remainder=0`; identifier `CD001`; volume identifier `RESCUE_DATA`
  plus space padding; `pvd_bytes` equals `size` exactly; `hdiutil attach
  -nomount` succeeds and does **not** print `image not recognized`. If you mount
  it instead, macOS shows 8.3 fallback names because macOS implements no Rock
  Ridge — cosmetic, not a failure; MED-12 is the authoritative name check.
- Pass/Fail: [ ]

#### MED-14 — Bad rescue input is refused before publishing
- Promise: nothing partial is left at either final path.
- Source: `internal/update/metadata.go:117-133`; `internal/media/rescue.go:127-170`.
- Wave: **Wave 2**
- Cost: minutes.
- Commands:
  ```console
  $ M="$WORK/badmirror"; mkdir -p "$M/$V/$ARCH"
  $ cp index.json "$M/index.json"; cp src.update.json "$M/$V/update.json"; : > "$M/$V/update.sjson"
  $ cp "$CACHE/sha256/<app-digest>"   "$M/$V/$ARCH/<app>.gz"
  $ cp "$CACHE/sha256/<image-digest>" "$M/$V/$ARCH/IncusOS_$V.img.gz"
  $ "$IOB" build -f offline-mirror.yaml -o neg.img --server "$M" --cache-dir "$(mktemp -d)"; echo "exit=$?"
  $ ls neg.img neg.resources.img 2>&1
  ```
- Expected: `exit=5` with an `acquisition failed:` line naming `update.sjson`
  (the metadata validator rejects the non-`multipart/signed` document before the
  media writer's own guard). Neither `neg.img` nor `neg.resources.img` exists.
- Pass/Fail: [ ]

#### MED-15 — Payload provenance: media bytes trace to `index.json` and the cache
- Promise: `cache.md:39-46,76-86`; media assets are the verified cached blobs.
- Wave: **Wave 2**
- Cost: cheap.
- Commands:
  ```console
  $ jq -r --arg v "$V" '.updates[] | select(.version==$v) | .files[] | "\(.type) \(.filename) \(.size) \(.sha256)"' index.json
  $ ls -l "$CACHE/sha256/"; shasum -a 256 "$CACHE"/sha256/*
  ```
- Expected: for the selected application, `index.json`'s `sha256` equals the
  cache filename, the `shasum` of that entry, and the digest read off the media
  in MED-10/MED-12; `index.json`'s `size` equals the media file's logical size.
  Cache entries are mode `-r--r--r--`. `update.json`/`update.sjson` are not
  index files — they are verbatim HTTP bodies, already `cmp`ed in MED-10/12.
- Pass/Fail: [ ]

#### MED-16 — Rescue inode replacement is visible in the published file mode
- Promise: the publisher reopens by path after `WriteRescue` replaces the inode.
- Source: `internal/media/rescue.go:172-185`; `internal/cli/publish.go:402-444`.
- Wave: **Wave 2**
- Cost: cheap (piggybacks on MED-07's `umask 077` run).
- Commands: `stat -f '%Lp %N' "$WORK/seeded-off.img" "$WORK/seeded-off.resources.img"`
- Expected: the installer is `644` (explicitly chmodded to `claimMode`) while the
  rescue file is `600` (0666 masked by umask 077, because `diskfs.Create` made a
  new inode). Two different modes from one run is the operator-visible proof. If
  the modes are equal, re-run MED-07 under `umask 077` before failing. Had the
  publisher kept the pre-call fd, `resources_sha256` would be the zero-byte
  digest and the file would be 0 bytes.
- Pass/Fail: [ ]

#### MED-17 — `--force` replaces the artifact pair
- Promise: refusal names both finals; `--force` replaces both and cleans backups.
- Source: `build-offline-media.md:185-227`; `internal/cli/publish.go:493-513`.
- Wave: **Wave 2**
- Cost: expensive: ~5–15 min warm, peak ~7.5 GB (both previous finals are renamed aside).
- Commands:
  ```console
  $ "$IOB" build -f offline.yaml -o "$WORK/seeded-off.img" --cache-dir "$CACHE"; echo "exit=$?"
  $ "$IOB" build --json --force -f offline.yaml -o "$WORK/seeded-off.img" --resources-output "$WORK/rescue-data.img" --cache-dir "$CACHE" > env2.json; echo "exit=$?"
  $ ls -l "$WORK" | sed -n 1,20p
  $ shasum -a 256 "$WORK/seeded-off.img" "$WORK/rescue-data.img"; cat env2.json
  ```
- Expected: the first run exits 2 with
  `usage error: refusing to overwrite <…>/seeded-off.img, <…>/seeded-off.resources.img; re-run with --force`
  (both paths, image first) and leaves both files unchanged. The second exits 0,
  writes the flag-named rescue file, and its envelope digests match `shasum`.
  Any leftover `*.incusos-builder.bak` is harmless and renameable back.
- Pass/Fail: [ ]

---

### 4.5 Packaging, container image, supply chain, release

SUP-01..SUP-18 are runnable now. SUP-19..SUP-23 are **post-tag only** and are
mandatory on the first real release, before the draft is published.

#### SUP-01 — Toolchain preflight and pinned supply-chain CLIs
- Promise: every tool comes from mise with lockfile integrity.
- Source: `mise.toml:17-49`; `mise.lock`.
- Wave: **Wave 1**
- Cost: cheap (a cold `mise install` adds minutes).
- Commands:
  ```console
  $ mise install && mise ls --current && mise tasks && mise x -- cosign version
  ```
- Expected: the nine pins resolve (go 1.26.4, python 3.14.3, melange 0.54.0,
  apko 1.2.19, cosign 3.1.1, moon 2.3.5, golangci-lint 2.12.2, mockery 3.7.3,
  uv 0.11.0). `mise tasks` prints exactly one row: `image-local  Build the apko
  image for the host arch and load it into Docker as incusos-builder:dev`.
  `cosign version` reports `GitVersion: v3.1.1`.
- Pass/Fail: [ ]

#### SUP-02 — Docker prerequisite for melange's Linux sandbox
- Wave: **Wave 1**
- Promise / Source: `mise.toml:51-54`. Cost: cheap.
- Commands: `docker info --format '{{.ServerVersion}} {{.OperatingSystem}} {{.Architecture}}'`
- Expected: one line naming a reachable server (e.g. `29.4.0 OrbStack aarch64`).
  `Cannot connect to the Docker daemon` means SUP-03..SUP-10 cannot run.
- Pass/Fail: [ ]

#### SUP-03 — `mise run image-local` builds the signed apk and loads the image
- Promise: the documented local image path works on macOS arm64.
- Source: `mise.toml:55-71`.
- Wave: **Wave 2**
- Cost: **expensive**: ~5–12 min first run, ~1–3 GB inside the Docker VM.
- Commands:
  ```console
  $ mise run image-local
  $ ls packages/*/ && docker image ls incusos-builder
  ```
- Expected: exit 0 with last line `loaded incusos-builder:dev (host arch arm64)`.
  `packages/aarch64/` holds `incusos-builder-<pkgver>-r0.apk` plus melange's
  `APKINDEX.tar.gz`. Docker shows `incusos-builder:dev-arm64` and
  `incusos-builder:dev`. Note the deliberate divergence: the apk filename carries
  `package.version` while the binary is stamped `dev` locally (SUP-05).
- Pass/Fail: [ ]

#### SUP-04 — apk signature is actually enforced (negative control)
- Promise: apko trusts the local apk only via the appended public key.
- Source: `mise.toml:67` (`--keyring-append ./melange.rsa.pub`).
- Wave: **Wave 2**
- Cost: minutes; reuses SUP-03's apk.
- Commands: `mise x -- apko build apko.yaml incusos-builder:untrusted /tmp/untrusted.tar --arch arm64; echo "exit=$?"`
- Expected: **non-zero exit** — apko cannot install the `@local` apk without the
  keyring. Record the literal error. If this succeeds, the signing guarantee is
  not enforced: **release blocker**.
- Pass/Fail: [ ]

#### SUP-05 — Stamped version/commit/date match the source revision
- Promise: `--vars-file` ldflags reach the binary; Go build info survives stripping.
- Source: `mise.toml:62-64`; `melange.yaml:25-40`; `internal/cli/pin.go:20-26`.
- Wave: **Wave 2**
- Cost: cheap.
- Commands:
  ```console
  $ git rev-parse --short HEAD; cat .melange-vars.local.yaml
  $ docker run --rm incusos-builder:dev --version
  ```
- Expected: two lines; the first is
  `incusos-builder dev (<short sha>) built <RFC3339 UTC>` where the commit equals
  `git rev-parse --short HEAD` and the date equals `.melange-vars.local.yaml`;
  the second is `incus-os API: v0.0.0-20260815030500-0f5b8057f2fc`, proving
  `-buildvcs=false` + `strip: -s -w` did not destroy build info. `0.0.0`/`none`/
  `unknown` in the **container** means `--vars-file` did not reach `ldflags`:
  **release blocker**.
- Pass/Fail: [ ]

#### SUP-06 — Image runs as nonroot 65532 with the contractual entrypoint
- Wave: **Wave 2**
- Promise / Source: `apko.yaml:21-34`; asserted in CI at `release.yml:396-402`. Cost: cheap.
- Commands:
  ```console
  $ docker image inspect incusos-builder:dev --format '{{.Config.User}}'
  $ docker image inspect incusos-builder:dev --format '{{json .Config.Entrypoint}}'
  $ docker run --rm --entrypoint /usr/bin/incusos-builder incusos-builder:dev --help
  ```
- Expected: exactly `65532`; entrypoint contains `/usr/bin/incusos-builder`;
  `--help` exits 0.
- Pass/Fail: [ ]

#### SUP-07 — The local image build leaves no repo residue
- Wave: **Wave 2**
- Promise / Source: `.gitignore:23-33`; `mise.toml:59-64`. Cost: cheap.
- Commands: `git status --porcelain; ls melange.rsa melange.rsa.pub image.tar .melange-vars.local.yaml *.spdx.json 2>/dev/null`
- Expected: `git status --porcelain` prints nothing; every generated path is
  gitignored. Any untracked path here is a gitignore gap.
- Pass/Fail: [ ]

#### SUP-08 — apko per-build SBOM exists and is inspectable
- Wave: **Wave 2**
- Promise / Source: `apko.yaml:1-6`; apko's `--sbom` defaults true. Cost: cheap.
- Commands: `ls -l *.spdx.json && jq -r '.spdxVersion, (.packages | length)' <one of them>`
- Expected: at least one SPDX JSON next to `image.tar` (the task passes no
  `--sbom-path`, so they land in the repo root); `spdxVersion` present and a
  package count > 0. Record the literal filenames — the repo fixes them nowhere.
- Pass/Fail: [ ]

#### SUP-09 — syft can describe the local image (stand-in for the attested SBOM)
- Wave: **Wave 2**
- Promise / Source: `release.yml:409-412`; rehearsed at `release-dry-run.yml:281-289`. Cost: minutes.
- Commands:
  ```console
  $ syft --version
  $ syft docker:incusos-builder:dev -o spdx-json=/tmp/image.spdx.json
  $ jq -e '.packages | length > 0' /tmp/image.spdx.json
  ```
- Expected: `true` — the same assertion the rehearsal makes. The package list
  includes `incusos-builder` plus the Wolfi base packages. Skip and record if
  `syft` is unavailable (it is not mise-pinned).
- Pass/Fail: [ ]

#### SUP-10 — `run-in-ci.md` snippets against the real container image
- Promise: one JSON document to stdout, `-f -` stdin, documented exit codes, through the image.
- Source: `run-in-ci.md` §2,§5; `automation.md:103`.
- Wave: **Wave 2**
- Cost: cheap.
- Commands:
  ```console
  $ docker run --rm incusos-builder:dev init --no-input -o - > /tmp/config.yaml; cat /tmp/config.yaml
  $ docker run --rm -i incusos-builder:dev validate --json -f - --color never < /tmp/config.yaml; echo "exit=$?"
  $ docker run --rm -i incusos-builder:dev validate --json -f - --color never < /dev/null; echo "exit=$?"
  ```
- Expected: `init -o -` writes YAML with no `wrote` line. The first `validate`
  prints exactly
  `{"result":{"valid":true,"type":"iso","architecture":"x86_64","offline":false}}`
  at `exit=0`, character for character as documented. The empty-stdin run prints
  one error envelope on stdout, reprints on stderr, and exits with the same
  integer as `error.code` — record which. Any progress or log line polluting
  stdout breaks the CI contract.
- Pass/Fail: [ ]

#### SUP-11 — ghd asset staging and checksum logic (stand-in for the 9-asset promise)
- Wave: **Wave 1**
- Promise / Source: `.github/scripts/stage_ghd_release_assets.py:17-23,234-287` and its unit suite. Cost: cheap.
- Commands:
  ```console
  $ mise x -- python3 -m unittest discover -s .github/scripts -p 'test_stage_ghd_release_assets.py' -v
  $ mise x -- python3 .github/scripts/stage_ghd_release_assets.py --tag v9.9.9; echo "exit=$?"
  ```
- Expected: the unittest run ends `OK`. The bare script run fails non-zero with
  `error: GITHUB_REPOSITORY must be set` — the release-time guard exists and the
  script refuses to invent a repository. GoReleaser is not mise-pinned, so
  `dist/artifacts.json` cannot be produced locally; SUP-12 covers the real path.
- Pass/Fail: [ ]

#### SUP-12 — Non-publishing release rehearsal
- Promise: the rehearsal exercises every release path short of publication.
- Source: `.github/workflows/release-dry-run.yml` (jobs at `:30,:106,:175`).
- Wave: **Wave 2**
- Cost: minutes (~7 min of runner time). Prior green rehearsal: run 31963560870.
- Commands:
  ```console
  $ gh workflow run release-dry-run.yml --repo componere/incusos-builder --ref master
  $ gh run list --repo componere/incusos-builder --workflow release-dry-run.yml --limit 1
  $ gh run watch <RUN_ID> --repo componere/incusos-builder
  $ gh run view <RUN_ID> --repo componere/incusos-builder --json conclusion,jobs --jq '{conclusion, jobs:[.jobs[]|{name,conclusion}]}'
  $ gh api /repos/componere/incusos-builder/actions/runs/<RUN_ID>/artifacts --jq '[.artifacts[].name]'
  $ gh run view <RUN_ID> --repo componere/incusos-builder --log --job <BINARY_JOB_ID> | grep -E 'release-assets|checksums.txt|incusos-builder 0.0.0-dryrun'
  $ gh run view <RUN_ID> --repo componere/incusos-builder --log --job <CONTAINER_JOB_ID> | grep -E 'linux/(amd64|arm64)|65532|incusos-builder 0.0.0-dryrun'
  ```
- Expected: `conclusion: success` with exactly four jobs — `Binary Release Dry
  Run`, `Melange Build Dry Run (amd64)`, `Melange Build Dry Run (arm64)`,
  `Container Image Dry Run` — all `success`. Artifacts are exactly
  `["apk-amd64","apk-arm64"]`. The binary job log shows nine
  `dist/release-assets/...` paths (4 binaries, 4 `.sbom.json`, `checksums.txt`)
  and the smoke line `incusos-builder 0.0.0-dryrun.<run>.<attempt> …`. The
  container job asserts both platforms, the version smoke line, `65532`, and the
  syft SBOM. No `gh release upload`, `apko publish`, `cosign sign`, or attestation
  step appears — that is the declared fidelity boundary.
- Pass/Fail: [ ]

#### SUP-13 — Release Please PR is correct before merge
- Promise: version bump reaches manifest + melange + apko; changelog filtered; draft-producing.
- Source: `release-please-config.json:1-24`; `.github/workflows/release-please.yml:7-12`.
- Wave: **Wave 1**
- Cost: cheap.
- Commands:
  ```console
  $ gh pr list --repo componere/incusos-builder --search 'head:release-please--' --json number,title
  $ gh pr view <N> --repo componere/incusos-builder --json title,headRefName,files --jq '{title,headRefName,files:[.files[].path]}'
  $ gh pr diff <N> --repo componere/incusos-builder
  $ gh pr checks <N> --repo componere/incusos-builder
  ```
- Expected: exactly four changed files — `.release-please-manifest.json`,
  `CHANGELOG.md`, `apko.yaml`, `melange.yaml` — with the same version in the
  manifest, `melange.yaml` `version:` (x-release-please marker), and
  `apko.yaml` `org.opencontainers.image.version`. The changelog section lists
  only `feat`/`fix` entries (`docs`/`chore` are hidden). `gh pr checks` reports
  the four contexts from `.github/repository-settings.toml:82-87` (`ci`,
  `GitHub Pages`, `Binary Release Dry Run`, `Container Image Dry Run`) — the two
  dry-run jobs actually execute on a `release-please--` branch.
  **Two decisions this case forces** (both true of the currently open PR #10,
  `chore(master): release 0.1.2`):
  1. The proposed version is **0.1.2, not 1.0.0** — `bump-minor-pre-major` +
     `bump-patch-for-minor-pre-major` keep the project pre-1.0 until a deliberate
     `Release-As:`/`!` bump. Merging as-is does not produce v1.
  2. The rendered `CHANGELOG.md` ends with a stray empty `## Changelog` heading
     (an artifact of the single-line seed file). Fix or accept before merge — it
     becomes the permanent changelog of the first release.
- Pass/Fail: [ ]

#### SUP-14 — Release Please credentials and tag-protection prerequisites
- Wave: **Wave 1**
- Promise / Source: `release-please.yml:1-5,44-45`; `.github/repository-settings.toml:89-104`. Cost: cheap.
- Commands:
  ```console
  $ gh variable list --repo componere/incusos-builder
  $ gh secret list --repo componere/incusos-builder
  $ gh api /repos/componere/incusos-builder/rulesets
  ```
- Expected: variable `COMPONERE_RELEASE_APP_CLIENT_ID` and secret
  `COMPONERE_RELEASE_APP_PRIVATE_KEY` both present. **Observed 2026-08-16:**
  `rulesets` returns `[]` — the `Default tags` ruleset does not exist, so tag
  creation is unprotected and the workflow header's "protected `v*` tags"
  premise is not yet true. Decide: apply the settings (SUP-15) or accept an
  unprotected first tag.
- Pass/Fail: [ ]

#### SUP-15 — Repository settings plan (read-only) reports the unapplied delta
- Wave: **Wave 1**
- Promise / Source: `.github/repository-settings.toml`; `configure_github_repo.py` plan mode is GET-only. Cost: cheap.
- Commands: `mise x -- uv run .github/scripts/configure_github_repo.py plan --repo componere/incusos-builder; echo "exit=$?"`
- Expected: exit 0, output beginning `Planned changes:` then
  `Unsupported or manual follow-ups:`. Observed 2026-08-16 — seven planned
  changes: general repository settings, immutable releases, private
  vulnerability reporting, automated security fixes, create GitHub Pages site,
  create branch ruleset `Default branch`, create tag ruleset `Default tags`.
  **Do not run `apply` as part of this case.** The release decision is whether
  v1 ships with immutable releases, rulesets, and required status checks
  unenforced.
- Pass/Fail: [ ]

#### SUP-16 — SECURITY.md's reporting channel actually works
- Wave: **Wave 1**
- Promise / Source: `SECURITY.md:11`; `.github/repository-settings.toml:38`. Cost: cheap.
- Commands:
  ```console
  $ gh api /repos/componere/incusos-builder/private-vulnerability-reporting
  $ open https://github.com/componere/incusos-builder/security/advisories/new
  ```
- Expected: `{"enabled":true}` and a working advisory form. **Observed
  2026-08-16: `{"enabled":false}`** — the link SECURITY.md gives reporters is
  not usable. Blocker for that promise; fixed by applying the settings or via
  the Settings UI before v1.
- Pass/Fail: [ ]

#### SUP-17 — Consumer verification tooling and syntax (pre-tag negative control)
- Promise: the exact verify commands the release summary hands to consumers are valid.
- Source: `release.yml:466,476-477`; `attest.yml:9-10`; `ghd.toml:3-4`.
- Wave: **Wave 1**
- Cost: cheap.
- Commands:
  ```console
  $ gh attestation verify --help | grep -E -- '--signer-workflow|--source-ref|--deny-self-hosted-runners|oci://'
  $ mise x -- cosign verify --help | grep -E -- '--certificate-identity-regexp|--certificate-oidc-issuer'
  $ printf 'x' > /tmp/sup-probe.bin && gh attestation verify /tmp/sup-probe.bin --repo componere/incusos-builder; echo "exit=$?"
  ```
- Expected: every flag spelled in the release summary exists in the installed
  tools, and `oci://<image-uri>` is an accepted subject form. The probe
  verification **must fail** with `HTTP 404: Not Found (… /attestations/sha256:…)`
  and exit 1. That 404 is the honest baseline: it proves the verification path
  reaches GitHub's API and that nothing is attested yet, so a later success in
  SUP-19/20 cannot be a false positive.
- Pass/Fail: [ ]

#### SUP-18 — README install sections match reality pre-tag
- Wave: **Wave 1**
- Promise / Source: `README.md:7,26-53`. Cost: cheap.
- Commands:
  ```console
  $ git tag; gh release list --repo componere/incusos-builder
  $ gh api '/orgs/componere/packages?package_type=container' --jq '[.[].name]'
  $ docker pull ghcr.io/componere/incusos-builder:v0.1.1; echo "exit=$?"
  ```
- Expected: no tags, empty release list, `[]` packages, and a failing pull —
  exactly what `README.md:7,53` claims. After the first release, re-run and
  confirm the pull/`--version` block works and the Status paragraph is rewritten.
- Pass/Fail: [ ]

> **SUP-19..SUP-23 are POST-TAG ONLY.** Execute them against the first real
> release before publishing the draft. Pre-tag substitutes are SUP-12 and SUP-17.

#### SUP-19 — POST-TAG: binary provenance verifies against the isolated signer workflow
- Wave: **Wave P**
- Promise / Source: `release.yml:466`; `attest.yml:1-15,62-66`. Cost: minutes.
- Commands:
  ```console
  $ TAG=v<X.Y.Z>; VERSION="${TAG#v}"
  $ gh release download "$TAG" --repo componere/incusos-builder --pattern "incusos-builder_${VERSION}_darwin_arm64" --dir /tmp/rel
  $ gh attestation verify "/tmp/rel/incusos-builder_${VERSION}_darwin_arm64" --repo componere/incusos-builder --signer-workflow componere/incusos-builder/.github/workflows/attest.yml --source-ref "refs/tags/$TAG" --deny-self-hosted-runners; echo "exit=$?"
  ```
- Expected: exit 0 with a success summary naming the verified subject. Provenance
  is attested over `checksums.txt`, so verification of an individual binary must
  resolve through that checksum subject; if it does not, record the exact
  failure — it invalidates the consumer command printed in the release summary.
  `--deny-self-hosted-runners` must not cause a failure.
- Pass/Fail: [ ]

#### SUP-20 — POST-TAG: image provenance and attestation inventory
- Wave: **Wave P**
- Promise / Source: `release.yml:414-420,432-436`; `attest.yml:79-85`. Cost: minutes.
- Commands:
  ```console
  $ DIGEST=sha256:<digest from the run's Release Inspection Summary>
  $ gh attestation verify "oci://ghcr.io/componere/incusos-builder@$DIGEST" --repo componere/incusos-builder --signer-workflow componere/incusos-builder/.github/workflows/attest.yml --source-ref "refs/tags/$TAG" --deny-self-hosted-runners; echo "exit=$?"
  $ gh api "/repos/componere/incusos-builder/attestations/$DIGEST" --jq '[.attestations[] | .bundle.dsseEnvelope.payload | @base64d | fromjson | .predicateType]'
  ```
- Expected: provenance verification exits 0. The listing returns **two**
  predicate types for the digest — SLSA provenance from `attest.yml` and the
  SPDX SBOM from `actions/attest-sbom`. Record the literal strings rather than
  asserting them; the repo declares the SBOM predicate type nowhere.
- Pass/Fail: [ ]

#### SUP-21 — POST-TAG: cosign keyless signature on the image digest
- Wave: **Wave P**
- Promise / Source: `release.yml:404-407,477`. Cost: minutes.
- Commands:
  ```console
  $ mise x -- cosign verify "ghcr.io/componere/incusos-builder@$DIGEST" \
      --certificate-identity-regexp "^https://github.com/componere/incusos-builder/.github/workflows/release.yml@.*" \
      --certificate-oidc-issuer https://token.actions.githubusercontent.com; echo "exit=$?"
  ```
- Expected: exit 0, `Verification for ghcr.io/componere/incusos-builder@sha256:… --`
  and a payload whose `critical.image.docker-manifest-digest` equals `$DIGEST`.
  Note the identity is `release.yml` (which runs `cosign sign`), **not**
  `attest.yml` — the two verification identities differ deliberately and both
  must be checked.
- Pass/Fail: [ ]

#### SUP-22 — POST-TAG: assets, checksums, multi-arch, nonroot, annotations
- Wave: **Wave P**
- Promise / Source: `stage_ghd_release_assets.py:234-266`; `release.yml:355-402`; `apko.yaml:36-44`. Cost: minutes, ~1 GB.
- Commands:
  ```console
  $ gh release view "$TAG" --repo componere/incusos-builder --json isDraft,assets --jq '{isDraft, assets:[.assets[].name]}'
  $ gh release download "$TAG" --repo componere/incusos-builder --dir /tmp/rel-all
  $ cd /tmp/rel-all && shasum -a 256 -c checksums.txt
  $ docker buildx imagetools inspect "ghcr.io/componere/incusos-builder@$DIGEST" --format '{{json .Manifest}}' | jq '[.manifests[] | "\(.platform.os)/\(.platform.architecture)"] | unique'
  $ docker buildx imagetools inspect "ghcr.io/componere/incusos-builder@$DIGEST" --raw | jq '.annotations, .manifests[].annotations'
  $ docker run --rm "ghcr.io/componere/incusos-builder:$TAG" --version
  $ docker image inspect "ghcr.io/componere/incusos-builder:$TAG" --format '{{.Config.User}}'
  ```
- Expected: `isDraft: true` (nobody has published yet) and exactly nine assets:
  four `incusos-builder_<version>_{darwin,linux}_{amd64,arm64}`, four matching
  `.sbom.json`, and `checksums.txt`. `shasum -a 256 -c` prints `: OK` for each.
  Platforms are `["linux/amd64","linux/arm64"]`. The raw index carries the four
  annotations with `org.opencontainers.image.version` equal to `${TAG#v}` — if it
  shows the previous version, release-please's `extra-files` bump did not land
  even though the binary stamp is correct (different mechanisms). `--version`
  begins `incusos-builder ${TAG#v} (` with the full 40-char commit;
  `.Config.User` is `65532`.
- Pass/Fail: [ ]

#### SUP-23 — POST-TAG: ghd install path and the human publish decision
- Wave: **Wave P**
- Promise / Source: `README.md:26-42`; `ghd.toml`; `release.yml:438-481`. Cost: minutes.
- Commands:
  ```console
  $ gh run view <RELEASE_RUN_ID> --repo componere/incusos-builder --json jobs --jq '[.jobs[]|{name,conclusion}]'
  $ ghd download "componere/incusos-builder/incusos-builder@${TAG#v}" --output "$(mktemp -d)"
  $ gh release view "$TAG" --repo componere/incusos-builder --web
  ```
- Expected: every release job reports `success` and the run's step summary
  contains the four verification commands with the real tag/digest substituted.
  `ghd download` succeeds and its provenance check resolves against
  `signer_workflow = componere/incusos-builder/.github/workflows/attest.yml`.
  Only then publish the draft by hand. Note the image is already public at
  `ghcr.io/componere/incusos-builder:$TAG` — GHCR has no draft state.
- Pass/Fail: [ ]

---

### 4.6 Documentation accuracy

Every DOC case uses `./bin/incusos-builder`, never `go run` — see F-DOC-1.

#### DOC-01 — README "From source" install, in a fresh clone
- Wave: **Wave 2**
- Promise / Source: `README.md:13-24`. Cost: minutes (~3–8 min cold).
- Commands:
  ```console
  $ cd "$(mktemp -d)" && git clone https://github.com/componere/incusos-builder.git && cd incusos-builder
  $ mise install
  $ go run ./cmd/incusos-builder --version
  $ mise x -- moon run root:build && ls -l bin/incusos-builder
  ```
- Expected: all exit 0; the version banner matches PRE-05; `bin/incusos-builder`
  exists and is executable. Record the Go version actually used — `mise.toml:19`
  pins 1.26.4 and `mise.toml:42` sets `GOTOOLCHAIN=local`, so a different Go is a
  failure.
- Pass/Fail: [ ]

#### DOC-02 — README "From a GitHub release" (ghd) method
- Wave: **Wave 1**
- Promise / Source: `README.md:26-42`; `ghd.toml`. Cost: cheap.
- Commands:
  ```console
  $ gh release list --repo componere/incusos-builder
  $ git ls-remote --tags https://github.com/componere/incusos-builder.git
  $ ghd install componere/incusos-builder/incusos-builder --store-dir "$HOME/.local/share/ghd/store" --bin-dir "$HOME/.local/bin"
  ```
- Expected: the first two print nothing, confirming `README.md:7,42`. The `ghd`
  invocation fails; record its literal message. Then confirm from `ghd.toml` that
  an asset pattern exists for this platform (`incusos-builder_${version}_darwin_arm64`)
  and `[[packages.binaries]] path = "incusos-builder"`. A missing pattern means
  the README documents an install path this host could not use even after a
  release.
- Pass/Fail: [ ]

#### DOC-03 — README "Container image" method
- Wave: **Wave 1**
- Promise / Source: `README.md:44-53`. Cost: cheap.
- Commands: `docker pull ghcr.io/componere/incusos-builder:v0.0.0`
- Expected: fails with a registry error, consistent with "No image has been
  published yet". Record the literal message. The future-tense claim is
  internally consistent: `release.yml:29` sets the image name and `apko.yaml:21-22`
  sets the entrypoint, so `docker run --rm <image> --version` will work as
  documented. SUP-03/SUP-05/SUP-06 are the live substitute; do not duplicate here.
- Pass/Fail: [ ]

#### DOC-04 — README quickstart, literally
- Wave: **Wave 2**
- Promise / Source: `README.md:55-67`. Cost: expensive: ~20–45 min, ~10 GB.
- Commands:
  ```console
  $ go run ./cmd/incusos-builder init --no-input
  $ go run ./cmd/incusos-builder validate -f config.yaml
  $ go run ./cmd/incusos-builder build -f config.yaml -o incusos.iso
  ```
- Expected: `wrote config.yaml`; `configuration valid`; a `summary` block naming
  `output  incusos.iso`. `config.yaml` carries `iso`, `x86_64`, `channel: stable`,
  `offline: false` with all `seeds` commented out. Because there are no seeds,
  expect `seed_bytes` to be small or `0`; the tutorial says a meaningful
  `seed_bytes` requires `seeds.applications`. `incusos.iso` exists and no
  `.resources.*` file is written. If disk or time is tight, reuse ART/MED
  artifacts and record the substitution.
- Pass/Fail: [ ]

#### DOC-05 — README capability and status statements
- Wave: **Wave 1**
- Promise / Source: `README.md:3,7,9,24,65,67,69-71`. Cost: cheap.
- Commands:
  ```console
  $ "$IOB" --help | grep -- '--server'
  $ gh release list --repo componere/incusos-builder
  $ curl -sI https://incusos-customizer.linuxcontainers.org/ui/ | head -1
  ```
- Expected: the `--server` default is `https://images.linuxcontainers.org/os`,
  matching README and `cli.md:50`. The upstream customizer URL named in
  `README.md:3` still resolves. `README.md:71`'s relative link to
  `docs/docs/index.md` resolves in the GitHub UI at this commit, and
  `SECURITY.md` says only the latest published release is supported.
- Pass/Fail: [ ]

#### DOC-06 — Docs site builds under `--strict`
- Wave: **Wave 1**
- Promise / Source: `docs/moon.yml:42-50`; `docs/mkdocs.yml:9`. Cost: minutes.
- Commands: `mise x -- moon run docs:build && ls docs/build/index.html`
- Expected: exit 0 with no MkDocs WARNING lines; `docs/build/index.html` exists.
  Any broken internal link across the 15 pages fails here — that is this case's
  whole value.
- Pass/Fail: [ ]

#### DOC-07 — Docs site serves and every nav entry renders
- Wave: **Wave 1**
- Promise / Source: `docs/moon.yml:52-60`; `docs/mkdocs.yml:20-72`. Cost: minutes.
- Commands: `mise x -- moon run docs:serve` then browse `http://127.0.0.1:8000/`.
- Expected: MkDocs binds `127.0.0.1:8000`. Click all 15 nav entries: each renders
  with its front-matter title, search returns hits, the light/dark toggle works,
  and the edit action points at `edit/master/docs/docs/`. Record any 404 or
  unstyled page.
- Pass/Fail: [ ]

#### DOC-08 — Nav covers every page (orphan sweep)
- Wave: **Wave 1**
- Promise / Source: `docs/mkdocs.yml:7,36-55`. Cost: cheap.
- Commands:
  ```console
  $ diff <(find docs/docs -name '*.md' | sed 's|docs/docs/||' | sort) <(grep -oE '[a-z0-9/-]+\.md' docs/mkdocs.yml | sort)
  ```
- Expected: empty diff — 15 files, 15 nav entries (pre-audited clean at 5337e7e).
  Do **not** rely on `--strict` for this: MkDocs logs nav-omitted files at INFO,
  so DOC-06 passing does not imply nav coverage. The six `docs/notes/*.md` files
  sit outside `docs_dir` deliberately and are repository-only.
- Pass/Fail: [ ]

#### DOC-09 — CONTRIBUTING local setup and task list
- Wave: **Wave 1**
- Promise / Source: `CONTRIBUTING.md:32-59`; `moon.yml:43-123`. Cost: minutes.
- Commands: `mise x -- moon run root:check` and `mise x -- moon query tasks`
- Expected: `root:check` exits 0 with exactly the six dependencies claimed —
  `root:format`, `root:lint`, `root:build`, `root:test`, `root:check-upstream`,
  `docs:build`. Every command listed in `CONTRIBUTING.md:45-55` resolves to a
  real task. `root:e2e` does **not** run as part of `root:check`.
- Pass/Fail: [ ]

#### DOC-10 — Tutorial walkthrough, literal, top to bottom
- Wave: **Wave 2**
- Promise / Source: `docs/docs/tutorials/first-seeded-iso.md`. Cost: expensive: ~20–45 min, ~10 GB.
- Commands:
  ```console
  $ "$IOB" --version
  $ "$IOB" init --no-input -o config.yaml
  $ "$IOB" init --no-input -o config.yaml; echo "exit=$?"      # expect the :76 refusal
  $ cat > config.yaml <<'YAML'
  version: 1
  image:
    type: iso
    architecture: x86_64
    channel: stable
  seeds:
    applications:
      applications:
        - name: incus
  YAML
  $ diff config.yaml "$REPO/internal/config/testdata/valid.yaml"
  $ "$IOB" validate -f config.yaml --color never
  $ printf 'version: 1\nimage:\n  type: disk\n  architecture: x86_64\n' | "$IOB" validate -f - --color never; echo "exit=$?"
  $ "$IOB" build -f config.yaml -o seeded.iso --color never
  $ ls -l seeded.iso
  $ "$IOB" build -f config.yaml -o seeded.iso --color never    # step 6 overwrite claim
  ```
- Expected: the two `--version` lines match `:52-57`; `wrote config.yaml`; the
  second `init` prints `usage error: refusing to overwrite existing file
  config.yaml` at exit 2; `diff` is empty (the step-3 YAML is byte-identical to
  the committed fixture); `configuration valid`; the `disk` config prints
  `invalid config: image.type: must be iso or raw` at exit 3; the build prints a
  `summary` block with labels in the documented order, `seed_bytes` > 0, and a
  64-hex `sha256`; `seeded.iso` exists with no `seeded.resources.*`. The final
  re-run at a TTY **prompts** `overwrite existing output? [y/N] ` rather than
  refusing outright — that divergence from `:157-158` is **F-DOC-5** and must be
  recorded, not waved through.
- Success criterion: one `seeded.iso` plus a recorded `sha256` matching the
  summary, and explicit acknowledgment that this is **not** boot acceptance.
- Pass/Fail: [ ]

#### DOC-11 — How-to: SOPS encryption, literal walkthrough
- Wave: **Wave 1** — optional step 5 (`build -f config.enc.yaml`) deferred to Wave 2 track A
- Promise / Source: `docs/docs/how-to/sops-encryption.md`. Cost: minutes.
- Commands: follow the guide's steps 1–4 and its Verification section; the
  equivalent commands are CFG-13..CFG-17 with the guide's own config.
- Expected: `config.enc.yaml` carries `ENC[...]` values and a top-level `sops`
  key; both keyed validations print `configuration valid` at exit 0; the
  cleared-key run exits 4 with `decryption failed`; a plaintext document with a
  bogus `sops:` key also exits 4. Note **F-CFG-3**: the guide's claim that an
  empty `SOPS_AGE_KEY_FILE` overrides `SOPS_AGE_KEY` does not hold.
- Success criterion: an encrypted config that validates from a path and from
  stdin, a recorded exit-4 failure, and confirmation that no decrypted bytes
  reached disk.
- Pass/Fail: [ ]

#### DOC-12 — How-to: build offline media, literal walkthrough
- Wave: **Wave 2**
- Promise / Source: `docs/docs/how-to/build-offline-media.md`. Cost: expensive (reuse MED-07/MED-11 artifacts).
- Commands: follow the guide verbatim, substituting `shasum -a 256` for
  `sha256sum` (**F-DOC-2**).
- Expected: the missing-applications config exits 3; `-o -` exits 2; the
  successful build writes one envelope with all nine fields and the default
  `seeded.resources.img` name; both on-disk digests match the envelope; the
  re-run without `--force` refuses at exit 2 leaving both files unchanged. The
  rescue-tree claim (`:156-166`) is verified in MED-10/MED-12 — cross-reference
  rather than repeat.
- Pass/Fail: [ ]

#### DOC-13 — How-to: run in CI, literal walkthrough
- Wave: **Wave 1** — the `build` snippets and the container run (SUP-10) deferred to Wave 2
- Promise / Source: `docs/docs/how-to/run-in-ci.md`. Cost: cheap (reuse artifacts for the build steps).
- Commands: run every snippet in §§2–6 against `./bin/incusos-builder` and, for
  §5, against the container image (SUP-10).
- Expected: the success envelope is byte-identical to `:89`; exactly one stdout
  document per invocation; diagnostics only on stderr; `--json` with `-o -` exits
  2; `--verbose` with `-q` exits 2; the `INCUSOS_BUILDER_JSON` pair proves the
  precedence table; the `http://` server exits 2. The unknown-field message will
  **not** read `invalid config: field seeds.install` as printed at `:102` —
  that is **F-DOC-3**. Cross-check every row of the exit table `:163-172`
  against `internal/cli/exit.go:14-64` (pre-audited exact).
- Pass/Fail: [ ]

#### DOC-14 — How-to: use a local mirror, literal walkthrough
- Wave: **Wave 1** for the four documented failure modes; **Wave 2 track A** for the populated-mirror build (needs ART-12)
- Promise / Source: `docs/docs/how-to/use-local-mirror.md`. Cost: minutes (negatives) / expensive (populated mirror — reuse ART-12).
- Expected: the two documented usage errors reproduce verbatim at exit 2; a
  regular file as `--server` is also exit 2; an empty mirror directory fails at
  exit 5 with `open index.json`; a populated mirror lists versions and builds; an
  unknown channel returns `[]` at exit 0. The digest-tamper case is ART-13 —
  cross-reference.
- Pass/Fail: [ ]

#### DOC-15 — How-to: recover an interrupted `--force` build
- Wave: **Wave 2**
- Promise / Source: `docs/docs/how-to/recover-interrupted-build.md`. Cost: minutes if ART-18/MED-07 artifacts are reused.
- Commands: follow the guide's inventory → classify → restore → verify sequence,
  substituting `shasum -a 256` (**F-DOC-2**), against a `SIGKILL`-ed build.
- Expected: the post-kill directory state matches one row of the decision table
  at `:112-119`; backups are named `<path>.incusos-builder.bak`; temps are
  `.<base>-<digits>.tmp` in the destination directory; a 0-byte final is an
  `O_CREAT|O_EXCL` claim, not an artifact. After the restore, the digest equals
  the one recorded from the `.bak`, the `.bak` is gone, and no temps remain.
  Also confirm the scope claims: `-o -` creates no `.bak` and `init` has no
  `--force`.
- Pass/Fail: [ ]

#### DOC-16 — How-to: verify boot acceptance, read-through (no boot)
- Wave: **Wave 1**
- Promise / Source: `docs/docs/how-to/verify-boot-acceptance.md`. Cost: cheap.
- Commands:
  ```console
  $ printf 'version: 1\nimage:\n  type: raw\n  architecture: x86_64\n  offline: true\nseeds:\n  applications:\n    applications:\n      - name: incus\n  install:\n    target:\n      min_size: 50GiB\n' | "$IOB" validate -f - --color never; echo "exit=$?"
  $ for c in cp losetup lsblk blockdev sha256sum incus remote-viewer; do printf '%s: ' "$c"; command -v "$c" || echo MISSING; done
  ```
- Expected: the guide's own step-1 config validates (`configuration valid`,
  exit 0). The tool sweep shows `losetup`, `lsblk`, `blockdev`, `sha256sum`,
  `incus`, `remote-viewer` MISSING on macOS and `cp` without
  `--reflink`/`--sparse` — the guide is GNU/Linux-only by construction and says
  so (**F-DOC-7**, consistent, not a defect). Also confirm by reading: boot
  priorities installer 30 > target 20 > dummy root 10; the vTPM is added before
  first start; `security.secureboot=false` is still UEFI, not legacy BIOS;
  recovery attaches the copied volume to the **same** VM; and the guide refuses
  invented success strings.
- Pass/Fail: [ ]

#### DOC-17 — Pre-audit regression sweep (confirm the §5 findings)
- Promise: the findings below are real and reproducible at this commit.
- Wave: **Wave 1**
- Cost: cheap.
- Commands:
  ```console
  $ printf 'version: 1\nimage:\n  type: disk\n  architecture: x86_64\n' | go run ./cmd/incusos-builder validate -f - --color never; echo "go_run_exit=$?"
  $ printf 'version: 1\nimage:\n  type: disk\n  architecture: x86_64\n' | "$IOB" validate -f - --color never; echo "binary_exit=$?"
  $ printf 'version: 1\nimage:\n  type: iso\n  architecture: x86_64\nseeds:\n  bogus: {}\n' | "$IOB" validate --json -f - --color never
  $ "$IOB" versions --server http://example.invalid/os; echo "exit=$?"
  $ command -v sha256sum || echo 'sha256sum MISSING'
  $ "$IOB" init --no-input -o -
  ```
- Expected: `go_run_exit=1` with a stray `exit status 3` line versus
  `binary_exit=3` → **F-DOC-1**. The unknown-field envelope reads
  `{"error":{"code":3,"message":"invalid config: seeds.bogus: unknown to incus-os v0.0.0-20260815030500-0f5b8057f2fc; a newer incusos-builder may accept this"}}`,
  not the string printed at `run-in-ci.md:102` → **F-DOC-3**. The `--server`
  error carries the `usage error: ` prefix that `run-in-ci.md:49` omits →
  **F-DOC-4**. `sha256sum MISSING` → **F-DOC-2**. `init --no-input -o -` shows
  all eleven commented seed keys in the documented order. `--version` matches
  the tutorial and `go.mod:14` → **F-DOC-8** tripwire green. For **F-DOC-5**,
  re-run `build` over an existing `-o` from a real terminal and record whether
  the prompt or the refusal appears.
- Pass/Fail: [ ]

#### DOC-18 — Explanation-page claims: routing, not duplication
- Promise: no explanation claim is silently dropped.
- Wave: **Wave 1**
- Cost: cheap; no assertions here — file each claim under its owning prefix:
  - `--server` classification, `http://` rejection, redirect-downgrade refusal — `trust-model.md:42-58` → CLI-05, §6
  - filename allowlist, 64-hex digests, 8 GiB size bound, admission, cache re-hash — `trust-model.md:70-96` → ART-13, ART-14
  - unknown JSON fields ignored, trailing data rejected, 1 MiB / 64 MiB caps — `trust-model.md:98-102,152-154` → ART-14, §6
  - `update.sjson` multipart/signed, Version match, Filename+Sha256 binding — `trust-model.md:104-134` → ART-19, MED-14
  - `seed-data` at byte 2148532224 — `seed-injection.md:24-40` → ART-08
  - uncompressed tar, mode 0600, eleven sections, size fit — `seed-injection.md:56-67` → ART-08, CFG-02
  - unknown-field error embeds the pin string — `upstream-version-coupling.md:99-101` → CFG-08
  - `RESCUE_DATA` lookup by partlabel or label; verbatim sjson staging — `seed-injection.md:123-128` → MED-08, MED-10
  - `root:check-upstream` denies `incus-osd/internal` and `.../cmd` — `upstream-version-coupling.md:78-97` → DOC-09
  - the 2026-08-15 live-server measurements, explicitly "measurements, not invariants" → ART, **not** a gate
  - "a successful build is not evidence that a host will install or recover" — `trust-model.md:36-40` → BOOT-10
- Pass/Fail: [ ]

---

### 4.7 Boot acceptance

This is the one surface that has never passed anywhere. Treat BOOT-01 as a
release-blocking decision, not a formality.

#### BOOT-01 — Decide the execution venue
- Promise / Source: `verify-boot-acceptance.md:8,20-30`; `docs/notes/phase-5-boot-probe.md:1-6,58-63`.
- Wave: **Wave 1**
- Cost: cheap (the decision); the chosen path is expensive.
- Commands:
  ```console
  $ uname -m; command -v incus || echo 'incus MISSING'; ls /dev/kvm 2>&1
  ```
- Expected: on this host `arm64`, `incus MISSING`, no `/dev/kvm` — the
  checklist's first prerequisite ("an `x86_64` Linux Incus host with `/dev/kvm`")
  is not satisfiable locally. Record one option and its verdict:
  - **Remote `x86_64` Linux host with Incus** — the only documented path.
    **Viable and required for a real pass.** Needs nested-virt/KVM, pool
    `default`, network `incusbr0`, `sudo` for `losetup`, and a SPICE client.
    BOOT-02..BOOT-09 run there.
  - **QEMU (local or GitHub-hosted, as in the Phase 5.2 probe)** —
    **known-insufficient.** `phase-5-boot-probe.md:25-37` records exactly that
    topology and classified it negative: Secure Boot enrolled, seed consumption
    not observed. On arm64 with no KVM it is strictly worse.
  - **Defer to post-tag** — **explicit risk acceptance, not a pass.** The
    checklist runs "before every release tag until a CI boot gate succeeds".
    Deferring means tagging with the product's core end-to-end claim unobserved;
    it must be written into the release record with a named owner.
- Pass/Fail: [ ]

#### BOOT-02 — Build the release-gate media
- Wave: **Wave 2**
- Promise / Source: `verify-boot-acceptance.md:31-63`. Cost: expensive: ~25–60 min, ~15 GB. The build host may be macOS.
- Commands:
  ```console
  $ cat > release-gate.yaml <<'YAML'
  version: 1
  image:
    type: raw
    architecture: x86_64
    offline: true
  seeds:
    applications:
      applications:
        - name: incus
    install:
      target:
        min_size: 50GiB
  YAML
  $ "$IOB" validate -f release-gate.yaml --color never
  $ "$IOB" build --json -f release-gate.yaml -o /absolute/path/seeded-x86_64.raw --resources-output /absolute/path/rescue-data.raw
  ```
- Expected: `configuration valid`; one JSON envelope; record `result.sha256` and
  `result.resources_sha256`; both files exist at the absolute paths. The
  `min_size: 50GiB` selector must match **only** the 50 GiB volume created in
  BOOT-04 (the dummy root is 4 GiB).
- Pass/Fail: [ ]

#### BOOT-03 — Map the raw files to block devices and record hashes
- Wave: **Wave 2**
- Promise / Source: `verify-boot-acceptance.md:65-103`. Cost: minutes; ~7 GB on the Linux host.
- Commands: run the guide's script at `:72-103` verbatim.
- Expected: the `uname -m = x86_64` and `test -b /dev/kvm` assertions pass; two
  loop devices are reported; `artifact-sha256.txt` shows `$SOURCE_RAW` and
  `$SOURCE_WORK` with equal digests, `$SOURCE_RAW` equal to BOOT-02's
  `result.sha256`, and `$RESCUE_RAW` equal to `result.resources_sha256`. The
  originally built file stays unmodified.
- Pass/Fail: [ ]

#### BOOT-04 — Create the profile, install target, and VM
- Wave: **Wave 2**
- Promise / Source: `verify-boot-acceptance.md:105-141`. Cost: minutes.
- Commands: run `:109-132` verbatim.
- Expected: `incus config show "$VM" --expanded` shows `security.secureboot=false`,
  `limits.cpu=2`, `limits.memory=4GiB`, a `vtpm` device, `install-media` on the
  source loop device (`io.bus=virtio-scsi`, `readonly=false`, `boot.priority=30`),
  `install-target` on the 50 GiB volume (`boot.priority=20`), and the dummy 4 GiB
  root (`boot.priority=10`, `virtio-scsi`). Every disk is `virtio-scsi`, never
  `virtio-blk`. The TPM was added before the first start.
- Pass/Fail: [ ]

#### BOOT-05 — Observation 1: record the seed, then install
- Wave: **Wave 2**
- Promise / Source: `verify-boot-acceptance.md:143-171`. Cost: expensive: ~20–60 min interactive.
- Commands: run `:148-167` verbatim — locate `seed-data` via
  `lsblk -nrpo NAME,PARTLABEL`, capture `seed-partition.before.sha256` and
  `seed.before.list`, grep for `install.(json|ya?ml)`, then `incus start` and
  watch `incus console --type=vga` plus `incus console --show-log | tee install-serial.log`.
- Expected: `SEED_PART` is non-empty and the pre-boot tar listing contains an
  `install.*` entry (the baseline BOOT-06 depends on). Secure Boot enrollment may
  run before the installer UI. The installer must **report completion on VGA or
  serial** — network traffic is explicitly not success.
- Pass/Fail: [ ]

#### BOOT-06 — Observation 2: prove the installer seed was consumed **(never yet observed)**
- Promise / Source: `verify-boot-acceptance.md:173-195`.
- Wave: **Wave 2**
- Cost: minutes, after BOOT-05.
- Commands: run `:175-189` verbatim — stop the VM, remove `install-media` and
  `install-target`, `blockdev --flushbufs`, re-read the seed partition into
  `seed-partition.after.sha256` / `seed.after.list`, then assert
  `test "$BEFORE" != "$AFTER"` and
  `! grep -Eq '(^|/)install\.(json|ya?ml)$' seed.after.list`.
- Expected: **both** assertions pass. Stop the whole gate if either fails.
  **This step has never succeeded anywhere.** `phase-5-boot-probe.md:44` records
  `seed_consumption_observed: false`; `:43` records that the blank target disk
  did not grow; `:46-48` records that source-overlay growth and three guest
  network frames were the only positive signals and neither is seed consumption.
  Do not accept a changed file size, cache growth, or network traffic as a
  substitute.
- Pass/Fail: [ ]

#### BOOT-07 — Observation 3: copy the installed volume and detect `RESCUE_DATA`
- Wave: **Wave 2**
- Promise / Source: `verify-boot-acceptance.md:197-234`. Cost: expensive: ~15–40 min.
- Commands: run `:199-211` then `:220-229` verbatim.
- Expected: `rescue-block-layout.txt` shows a FAT or ISO data partition whose
  `LABEL`/`PARTLABEL` is `RESCUE_DATA`; the installed target boots; and there is
  console evidence that IncusOS **detected** `RESCUE_DATA`. The copy attaches to
  the original VM because a cloned VM is not promised to keep the enrolled UEFI
  NVRAM and vTPM identity.
- Pass/Fail: [ ]

#### BOOT-08 — Observation 4: recovery payload accepted and applied **(never yet observed)**
- Wave: **Wave 2**
- Promise / Source: `verify-boot-acceptance.md:236-246`. Cost: minutes after BOOT-07.
- Commands: observe `recovery-serial.log` and the VGA console; then query the
  booted system for the post-recovery OS or application version.
- Expected: evidence that the signed recovery payload was **accepted and
  applied** — the expected post-boot version or effect. The guide forbids
  matching an invented log line because upstream publishes no stable success
  string. Recovery reads `hotfix.sh.sig` at the root then signed updates under
  `update/`; the builder writes no `hotfix.sh.sig`, so acceptance must come from
  the `update/` path. **Never observed:** `phase-5-boot-probe.md:49-54` records
  recovery as "not reachable" with `rescue_data_detected`,
  `update_sjson_acceptance`, `update_json_acceptance` all false.
- Pass/Fail: [ ]

#### BOOT-09 — Archive evidence, then clean up
- Wave: **Wave 2**
- Promise / Source: `verify-boot-acceptance.md:248-277`. Cost: minutes.
- Expected: the release record holds all ten artifacts listed at `:252-259`
  (`incus-version.txt`, `install-config.yaml`, `recovery-config.yaml`,
  `artifact-sha256.txt`, `seed-partition.before.sha256`, `seed.before.list`,
  `seed-partition.after.sha256`, `seed.after.list`, `install-serial.log`,
  `recovery-serial.log`, `rescue-block-layout.txt`). Cleanup order holds: stop
  before delete, custom volumes deleted explicitly, both loop devices detached,
  `$SOURCE_WORK` deleted **only after** archiving. A missing evidence item fails
  the gate on its own.
- Pass/Fail: [ ]

#### BOOT-10 — Record the release verdict honestly
- Promise / Source: `phase-5-boot-probe.md`; `trust-model.md:36-40`; `first-seeded-iso.md:160-170`.
- Wave: **Wave 2**
- Cost: cheap; this is a written record attached to the release.
- Expected: the record states, in these terms:
  1. Which of BOOT-05..BOOT-08 were **observed**, each with the evidence filename.
  2. That before this attempt, seed consumption had **never** been observed in
     any environment, and `RESCUE_DATA` detection and signed recovery acceptance
     were likewise unobserved.
  3. That a green DOC-01..DOC-18 run proves only that the documentation is
     accurate and the builder emits the media it claims — it says nothing about
     whether IncusOS consumes the seed.
  4. If BOOT-06 was not observed: that the release ships with its central
     end-to-end claim unproven, named as an accepted risk with an owner and a
     follow-up.
- Pass/Fail: [ ]

---

## 5. Known findings to confirm or fix

These were found while grounding this plan against the source at `5337e7e`.
Each is reproducible; the cases named re-confirm them. Every one is a decision
the release owner must make before tagging — fix it, document it, or accept it.

### Contract-level (an automation consumer can be misled)

| ID | Finding | Evidence | Case |
|----|---------|----------|------|
| F-CLI-5 | Unknown commands and stray operands exit **1**, not 2. Cobra's `unknown command` error wraps neither `ErrUsage` nor a pflag error, so `exitCode` falls to `default: exitInternal` (`internal/cli/exit.go:61-63`). `automation.md:21` places "other usage errors" at 2 and `cli.md` says every command takes no operands. | `incusos-builder frobnicate` → rc 1; `incusos-builder build extra` → rc 1 | CLI-10 |
| F-CLI-6 | The "cleans to `-`" sentinel rule (`automation.md:203`) holds only for `build -o` / `--resources-output`. `init -o ./-` creates a real file named `-` and `--json` does not reject it; `-f ./-` opens a file named `-` instead of stdin. | `init --json --no-input -o ./-` → exit 0, `{"result":{"output":"./-"}}` | CLI-15 |
| F-CLI-4 | `INCUSOS_BUILDER_CACHE_DIR=` (empty) does not override the default — Viper ignores empty env values — while `--cache-dir ""` does. `automation.md:163` presents flag and env as equivalent. | empty env var fails on `index.json`, not on `cache directory is required` | CLI-08 |
| F-CFG-2 | Interactive `init` with **"Offline install? yes"** writes a config that fails `validate` (`invalid config: seeds.applications: required when image.offline is true`). `internal/cli/init.go:260-261` promises the emitted body is a valid `config.Parse` input; every offline=yes answer combination breaks it. Not covered by any test — `internal/cli/init_test.go:240-277` asserts the answers but never validates the rendered config. | all four type × arch combinations with offline=true fail; all four with offline=false pass | CFG-12 |

### Documentation defects

| ID | Finding | Evidence | Case |
|----|---------|----------|------|
| F-DOC-1 | The tutorial aliases `incusos-builder` to `go run ./cmd/incusos-builder` (`first-seeded-iso.md:30`) and then asserts exit codes 2 and 3 (`:76`, `:114`). `go run` exits 1 and injects a stray `exit status 3` line. Fix: build the binary in the prerequisites, or state that `go run` does not propagate exit codes. | side-by-side run: `go run` → rc 1; `bin/incusos-builder` → rc 3 | DOC-17 |
| F-DOC-2 | `sha256sum` does not exist on stock macOS but appears in two guides that advertise macOS support (`build-offline-media.md:127`, `recover-interrupted-build.md:87,194`). | `/usr/bin/sha256sum` absent; `/usr/bin/shasum` present | DOC-17 |
| F-DOC-3 | `run-in-ci.md:101-104` shows `{"error":{"code":3,"message":"invalid config: field seeds.install"}}`. No code path emits `invalid config: field <path>`; real messages are `invalid config: <field path>: <reason>`. | observed unknown-field envelope names `seeds.bogus: unknown to incus-os v0.0.0-…` | DOC-13, DOC-17 |
| F-DOC-4 | The same refusal is quoted two ways: `run-in-ci.md:48-50` omits the `usage error: ` prefix that `usagef` always prepends; `build-offline-media.md:196-199` includes it. | live `--server` error carries the prefix | DOC-17 |
| F-DOC-5 | The tutorial (`:157-158`) describes only the non-interactive refusal on re-build, but at a TTY the tool prompts `overwrite existing output? [y/N] `. | `cli.md:98-101`; `internal/cli/build.go:42` | DOC-10 |
| F-DOC-6 | Every how-to prerequisite says "`incusos-builder` on `PATH`", which is unreachable pre-release; combined with F-DOC-1 the alias choice is load-bearing. | `README.md:7` | DOC-11..DOC-15 |
| F-DOC-8 | The tutorial hard-codes the upstream pin `v0.0.0-20260815030500-0f5b8057f2fc` (`:54`). Exact today; any pin bump staleses it silently. Treat as a release-checklist re-read. | matches `go.mod:14` | DOC-17 |
| F-CFG-3 | `sops-encryption.md:36-38` claims an empty `SOPS_AGE_KEY_FILE` makes SOPS ignore `SOPS_AGE_KEY`. Observed: `SOPS_AGE_KEY` still succeeds with an empty or nonexistent `SOPS_AGE_KEY_FILE`. `configuration.md:759-761` is literally true and can stay. | four-way matrix under sops/v3 v3.11.0 | CFG-17 |

### Cosmetic or advisory

| ID | Finding | Case |
|----|---------|------|
| F-CLI-1 | `-v` works as a `--version` shorthand (cobra auto-adds it) but appears nowhere in the reference. | CLI-01 |
| F-CLI-2 | `completion` and `help` are registered commands, but `cli.md:9-10` says the registered commands are the four documented ones. | CLI-01 |
| F-CLI-3 | `-f`/`-o` render their value placeholder as `-` rather than `string` because the usage strings backtick-quote `` `-` ``. | CLI-02 |
| F-CLI-7 | `--progress never` and `-q` do not suppress reporter step headers (`==> index`, `done index`); only percentage lines are gated, though `internal/ux/reporter.go` documents `ProgressModeNever` as "suppresses progress". | CLI-20 |
| F-CLI-8 | `--verbose` has no observable effect outside a successful `build`; the only log statements in the tree are two `Debug` calls in `logBuildPlan`, yet `automation.md:188` implies warn/error output exists. | CLI-20 |
| F-CFG-1 | Wrong-type diagnostics for nested mappings leak a Go type name (`cannot construct !!str <value> into config.seeds`) instead of a YAML path. No secret leak; undocumented wording. | CFG-09 |
| F-SUP-1 | `moon.yml:37` declares the `releaseConfig` input `'.github/workflows.disabled/**/*.yml'`; that directory does not exist. | SUP-12 |
| F-SUP-2 | `CONTRIBUTING.md:44-56` never mentions `mise run image-local`, so the container path has no discoverable entry point for a new contributor. | DOC-09 |

### Found during Wave 1 execution (2026-08-16, commit 59c268b)

Full evidence in `WAVE1_RESULTS.md`. 75/75 Wave 1 cases executed: 64 pass,
10 deviations, 1 fail.

| ID | Severity | Finding | Case |
|----|----------|---------|------|
| N-CLI-1 | high | `ACCESSIBLE=1 init` cannot be cancelled: raw mode with ISIG off swallows Ctrl-C, Ctrl-D, and an external `kill -INT`; stray keystrokes are eaten as form answers. Only completing the form or SIGKILL exits. Contradicts `cli.md:186-190`. | CLI-19 |
| F-DOC-9 | medium | `validate` silently accepts a plain-HTTP `--server` (exit 0) while `versions`/`build` refuse with exit 2. `run-in-ci.md` §4 states the rule unconditionally. | DOC-13 |
| N-ART-2 | medium | The step reporter emits `done index` / `done download` for steps that then fail; stderr asserts success and then reports the same step failing. | ART-14 |
| N-ART-1 | cosmetic | Doubled prefix: `acquisition failed: gzip: gzip: invalid header`. | ART-14a |
| N-ART-3 | cosmetic | Local-mirror open errors double the verb and leak the resolved absolute path. | ART-14h/i |
| N-CLI-5 | cosmetic | `ACCESSIBLE=1` drops every field description, hiding the "default stable" hint. | CLI-19 |
| N-CLI-2 | cosmetic | `--resources-output` help text differs between `cli.md:77` and the binary. | CLI-02 |
| N-CFG-A | advisory | The decrypt path passes go-yaml messages through unfiltered, echoing a 7-char fragment of the offending scalar where every other path renders `<value>`. | CFG-16 |
| N-CFG-B | advisory | Decrypt-path errors are the only multi-line stderr messages. | CFG-16 |
| N-ART-4 | advisory | `versions` never writes to `--cache-dir`; `index.json` is refetched every invocation. | ART-01..03 |
| N-ART-5 | cosmetic | The `versions` table is two-space-joined text and misaligns with its header. | ART-01 |
| N-CFG-C | cosmetic | Interactive `init` Channel placeholder reads as an unclearable prefilled value. | CFG-12 |
| F-DOC-10 | doc | The docs site is served at `/incusos-builder/`, not `/`; `http://127.0.0.1:8000/` returns 302. All 15 pages are otherwise 200 OK. | DOC-07 |
| F-DOC-2 (revised) | doc | **Re-scope, not a defect on macOS 26.** `/sbin/sha256sum` is an Apple-signed base-system binary here (`com.apple.sha224sum`, platform 26). The claim "does not exist on stock macOS" is false for macOS 26, true for ≤ 15. `/usr/bin/sha256sum` is absent. | DOC-17 |
| F-GATE-1 | high (dev workflow) | `root:format`/`root:lint` walk the gitignored `reference/incus-os/` clone and `.wt/` worktrees, so `moon run root:check` — which `CONTRIBUTING.md:57` calls the full local gate — cannot pass in a checkout that follows TECH_NOTES or uses the mandated Worktrunk flow. Scoped `golangci-lint fmt --diff ./cmd/... ./internal/...` is clean. | PRE-06, DOC-09 |
| F-GATE-2 | advisory | A stale golangci-lint cache reports diagnostics for deleted worktree paths; `golangci-lint cache clean` clears it. | PRE-06 |

### Found during Wave 2 track B execution (2026-08-16, commit 59c268b)

Full evidence in `WAVE2_TRACKB_RESULTS.md`. 9/9 track B cases executed, all pass.

| ID | Severity | Finding | Case |
|----|----------|---------|------|
| F-SUP-3 | cosmetic | The rehearsal stamps the **image** with a local-offset timestamp (`built 2026-08-16T11:57:26-07:00`) while the **binaries** carry UTC (`…T18:57:26Z`). `release-dry-run.yml:139` and `release.yml` both use `git show -s --format=%cI`. A published release will be internally inconsistent. | SUP-12 |
| F-SUP-4 | observability | Three rehearsal assertions (architectures, uid 65532, SBOM count) are silent on success; only the step exit code distinguishes a pass from a regression. | SUP-12 |
| F-SBOM-1 | supply chain | No file pins the **syft version**: all four `Install Syft` steps pin the action SHA but pass no `syft-version:`, so the attested SBOM can drift with the action's default. | SUP-09 |
| F-SBOM-3 | informational | The apk purl namespace is `unknown` (`pkg:apk/unknown/incusos-builder@0.1.1-r0`), so scanners keying on namespace will not match Wolfi advisories. | SUP-08 |
| F-SBOM-2 | plan | The apko **index** SBOM legitimately names no `incusos-builder`; only the per-arch document does. | SUP-08 |
| F-SBOM-4 | positive | The index SBOM's `versionInfo` is the full HEAD commit — a provenance anchor independent of the ldflags stamp. | SUP-08 |
| F-IMG-A | informational | `RepoDigests` is `[]` on a `docker load`-ed image; do not attempt digest-based cosign verification against `incusos-builder:dev`. | SUP-06 |
| F-IMG-B | informational | `org.opencontainers.image.created` is four days before the build — apko reproducibility, not a stamping bug. | SUP-06 |
| F-IMG-C | positive | The binary is root-owned `0755`, so the runtime uid cannot modify it. | SUP-06 |
| F-DOC-9 (extended) | medium | `validate` does not merely accept a plain-http `--server`; it **ignores `--server` entirely** — the envelope is byte-identical with and without a bogus server. | SUP-10 |

Track B plan corrections D-9..D-12 (SUP-03 costs ~40 s not 5–12 min; SUP-12's
greps match only echoed script bodies; SUP-08's expectation must target the
per-arch SBOM; SUP-09 cannot read a version from a `go install`-ed syft) are
detailed in `WAVE2_TRACKB_RESULTS.md`.

Plan defects found while executing (D-1..D-8) are listed in `WAVE1_RESULTS.md`;
the load-bearing one is **D-1**: all eight `configuration.md` line ranges shifted
+12 at commit 59c268b and must be re-derived, not trusted.

### Repository state that blocks documented promises (observed 2026-08-16)

| ID | Finding | Case |
|----|---------|------|
| F-REPO-1 | `gh api …/private-vulnerability-reporting` returns `{"enabled":false}` — the reporting link in `SECURITY.md:11` is dead. | SUP-16 |
| F-REPO-2 | `gh api …/rulesets` returns `[]` — no branch or tag protection, no required status checks. The release-please workflow's "protected `v*` tags" premise is not yet true. | SUP-14 |
| F-REPO-3 | `.github/repository-settings.toml` has seven unapplied planned changes, including immutable releases and the Pages site. | SUP-15 |
| F-REPO-4 | The open Release Please PR proposes **0.1.2**, not 1.0.0 (`bump-minor-pre-major` + `bump-patch-for-minor-pre-major`), and its rendered `CHANGELOG.md` ends with a stray empty `## Changelog` heading. | SUP-13 |

---

## 6. Out of scope and cannot verify

Recorded honestly, with the substitute evidence that exists and the risk that
remains.

| Claim | Why it cannot be proven here | Substitute / deferred risk |
|---|---|---|
| IncusOS consumes the spliced seed at boot | Never observed in any environment; `phase-5-boot-probe.md:44` records `seed_consumption_observed: false`. macOS arm64 has no KVM and no Incus. | BOOT-06 on a real Linux host is the only proof. ART-08/09 prove the bytes are correct and correctly placed; that is necessary, not sufficient. **This is the largest deferred risk in the release.** |
| `RESCUE_DATA` detection and signed recovery acceptance | Unreachable because recovery never ran (`phase-5-boot-probe.md:49-54`). | MED-08..MED-13 prove the media is structurally what upstream `recovery.go` looks for. BOOT-07/08 are the real gate. |
| Secure Boot enrollment, vTPM identity, dm-verity on this commit's media | Needs `x86_64` UEFI in setup mode plus a TPM. | The Phase 5 probe observed enrollment once under OVMF secboot; not re-observed for this commit. |
| Bare-metal install | No doc claims it; the checklist only ever describes an Incus VM. | Out of scope — note it in the release record rather than as a pass. |
| Real `version`/`commit`/`date` on the `--version` first line of a **released** binary | Only GoReleaser or melange injects them. | SUP-05 proves the melange path locally; re-run CLI-01 against the published binary post-tag (SUP-22). |
| `incus-os API: unknown` fallback | A normally built binary always records the dep in `debug.BuildInfo`. | `internal/cli/pin_test.go` covers nil info, empty deps, and the test-binary case. Cosmetic. |
| Exit 1 from a failed stderr write | macOS has no `/dev/full`; `2>&-` still yields the normal code. | Unit coverage with an erroring writer. Negligible risk. |
| HTTPS-specific integrity behavior: redirect-downgrade rejection, 5xx/transport retry with backoff, "exactly one clean re-download after a mismatch" | The live server will not serve a downgrade or corrupt bytes on demand, and there is no fault-injection flag. | ART-13 proves the same admission gate through the local adapter (identical `assetCache.admit` path); `internal/update/client_test.go` covers the HTTPS cases with `httptest`. The retry ladder is never exercised manually. |
| The 8 GiB asset ceiling with a real oversized file | No published file approaches it. | ART-14(e) proves the metadata gate rejects the declared size before any request. |
| GPT-drift rejection (`seed-data starts at byte X, expected 2148532224`) | Every published image matches; triggering it needs a hand-forged multi-GB gzip. | ART-05's success **is** the positive assertion that the probe found the offset; the negative branch has unit coverage. |
| Seed-tar-exceeds-partition (exit 3 at splice) | Requires a >100 MiB seed, i.e. an artificial config with enormous blobs plus a full acquisition. | Unit coverage. Residual risk: a maximal config with large certificate blobs would fail only at build time, never at `validate`. |
| Offline `seeds.update.check_frequency: never` rewrite and per-section `version: "1"` defaulting | `validate`'s output surface is only `valid`/`type`/`architecture`/`offline`, so the rewritten values are invisible to the CLI. | `internal/config/validate_test.go:352-385`; or read `update.yaml` out of an offline artifact's seed partition (extends ART-08). |
| Byte-equality of the seed tar against an upstream-built `image-customizer` binary | The oracle is a vendored `writeSeed` copy at the pinned commit, not an upstream binary. | Committed goldens plus the pin-bump revalidation list in `upstream-version-coupling.md`. |
| Rock Ridge as Linux resolves it | macOS implements no Rock Ridge; a mounted ISO shows 8.3 names. | `bsdtar` (libarchive) is an independent RR reader — MED-12. A Linux `mount -o loop` is the only stronger evidence. |
| FAT16 divergence | The builder never emits FAT16 — a deliberate, documented divergence from upstream. | Nothing to test. |
| `amd64` container execution locally | `image-local` builds only the host arch. | SUP-12 asserts both loaded images in CI; SUP-22 asserts the published index. |
| Multi-arch OCI **index** locally | `apko build` writes per-arch tars; only `apko publish` creates the index. | SUP-22 post-tag. |
| SLSA L3 isolation as a property | That provenance is unforgeable because the OIDC token is minted inside `attest.yml` is architectural; observable only as "the verified signer workflow is `attest.yml`". | SUP-19/SUP-20. L3 is not hermeticity — the build job still computes the digests it passes in. |
| `cosign verify` / `gh attestation verify` success paths | Nothing is signed or attested yet. | SUP-17's 404 is the honest baseline; SUP-19..21 are mandatory on the first tag — that is the *first ever* execution of `cosign sign` and the attest actions. |
| `checksums.txt` produced locally | GoReleaser is not mise-pinned. | SUP-11 (unit suite) + SUP-12 (rehearsal runs the identical script on real GoReleaser output). |
| `ghd install` / `docker pull` succeeding | No tag, release, or image exists. | Assert the documented failure now (DOC-02, DOC-03, SUP-18); re-run post-tag (SUP-23). Neither install method has ever been exercised end to end. |
| `Security Scan` verdict at tag time | Schedule-driven and dependent on the floating Wolfi base, so a pass today is not a pass at tag time. | Dispatch `security-scan.yml` once immediately before tagging. |
| Colour rendering fidelity, full-screen Huh TUI theming | Visual judgment, not an assertable string. | CLI-17/CLI-19 compare styled vs plain output; a human eyeball is adequate. |

---

## 7. Exit criteria

### Release blockers — all must be green

Each blocker names the wave that produces it, so a wave can be signed off on its
own. Waves 1 and 2 may complete in either order; both must be green before a tag.

**From Wave 1**

1. **PRE-01..PRE-06 and PRE-07-W1** pass, including `moon run root:check`.
2. **Exit taxonomy reachable through the real CLI:** CLI-03..CLI-09 (codes 2–6)
   and CLI-11 (envelope shape) pass.
3. **F-REPO-1 is fixed** (SUP-16): private vulnerability reporting is enabled, or
   `SECURITY.md` is rewritten to give a channel that works. Shipping a security
   policy with a dead reporting link is not acceptable.
4. **Documentation builds and its nav is complete:** DOC-06 and DOC-08 pass.
5. **BOOT-01 is decided and recorded.**

**From Wave 2, track A**

6. **PRE-07-W2** passes, including `INCUSOS_BUILDER_E2E=1 moon run root:e2e`.
7. **Artifact correctness under third-party tools:** ART-05, ART-06, ART-08,
   ART-09 pass — the seed tar is at byte 2,148,532,224 with the eleven expected
   entries at mode 0600, and everything outside the tar is byte-identical to the
   fetched image.
8. **Integrity gates hold:** ART-13 and ART-14 pass — tampered bytes and hostile
   metadata are refused with the documented wording and leave no cache residue.
   (ART-14 is Wave 1; ART-13 is Wave 2.)
9. **Offline media is structurally correct:** MED-07 through MED-13 pass on both
   raw and ISO, with byte-exact payload read-back.
10. **The tutorial executes as written:** DOC-10 passes.

**From Wave 2, track B**

11. **The image is what it claims:** SUP-03, SUP-04, SUP-05, SUP-06 pass —
    signed apk, correct stamping, nonroot 65532, contractual entrypoint.
12. **The release rehearsal is green:** SUP-12 passes on the tested commit.

**From Wave 2, track C**

13. **BOOT-10** is written into the release record, whatever the verdict.

**From Wave P**

14. **Post-tag, before publishing the draft:** SUP-19, SUP-20, SUP-21, SUP-22
    all pass. If any fails, delete/unpublish and fix — the consumer verification
    commands in the release summary must actually work.

### May ship with a documented caveat

- **BOOT-06 unobserved.** Only if BOOT-10 records it as an accepted risk with a
  named owner and a follow-up. This is the honest status quo, not a pass.
- **F-CLI-5** (operands exit 1) and **F-CLI-6** (`init -o ./-`) — contract
  wrinkles. Either fix the code or amend `automation.md`; do not leave the docs
  asserting behavior the binary does not have.
- **F-CFG-2** (interactive offline `init` emits an invalid config) — fix by
  seeding an `applications` entry, or qualify the promise. Cheap to fix; prefer
  fixing.
- **F-DOC-1..F-DOC-5** — documentation defects. All are small edits; a release
  whose tutorial asserts an exit code its own alias cannot produce is a poor
  first impression.
- **F-REPO-2, F-REPO-3** (rulesets and settings unapplied) — acceptable if the
  release record states that tags and the default branch are unprotected for v1.
- **F-REPO-4** (0.1.2 vs 1.0.0, stray changelog heading) — a versioning
  decision, not a defect. Decide deliberately before merging the release PR.
- Cosmetic findings **F-CLI-1, -2, -3, -7, -8**, **F-CFG-1**, **F-SUP-1, -2** —
  file as issues.

### Sign-off statement

The release is ready when the operator can write, and support with the evidence
recorded in §8:

> At commit `<sha>`, every documented user-facing promise of incusos-builder was
> executed by hand and observed, except those listed in §6. Artifacts were
> verified with tools outside the project. The supply chain verifies with the
> exact commands the documentation gives consumers. Boot acceptance was
> `<observed | not observed>`; if not observed, it is an accepted risk owned by
> `<name>` and tracked in `<issue>`.

---

## 8. Results log

Fill in one row per case. `Result` ∈ {pass, fail, skipped, N/A}. `Evidence` is a
path, a run URL, a digest, or a transcript filename — not a description. The two
waves are logged separately so a Wave 1 sign-off stands on its own even if Wave 2
runs days later.

### Shared preflight

| Case | Result | Evidence | Notes |
|------|--------|----------|-------|
| PRE-01 | pass | WAVE1_RESULTS.md |  |
| PRE-02 | deviation | WAVE1_RESULTS.md | CI=true and NO_COLOR=1 set in this harness |
| PRE-03 | pass | WAVE1_RESULTS.md | sops 3.13.1, syft present, /sbin/sha256sum present |
| PRE-04 | pass | WAVE1_RESULTS.md | HTTP 200, 237 GB free |
| PRE-05 | pass | WAVE1_RESULTS.md | dev (none) built unknown; pin matches go.mod |
| PRE-06 | deviation | WAVE1_RESULTS.md | root:format walks gitignored reference/ (F-GATE-1); scoped fmt clean |

### Wave 1 — fast lane

| Case | Result | Evidence | Notes |
|------|--------|----------|-------|
| PRE-07-W1 | pass | WAVE1_RESULTS.md | per-tester scratch dirs under /tmp |
| CLI-01 | pass | WAVE1_RESULTS.md |  |
| CLI-02 | pass | WAVE1_RESULTS.md |  |
| CLI-03 | pass | WAVE1_RESULTS.md |  |
| CLI-04 | pass | WAVE1_RESULTS.md |  |
| CLI-05 | pass | WAVE1_RESULTS.md |  |
| CLI-06 | pass | WAVE1_RESULTS.md |  |
| CLI-07 | pass | WAVE1_RESULTS.md |  |
| CLI-08 | deviation | WAVE1_RESULTS.md | retry ladder measured 1s, not ~1 min (D-4); F-CLI-4 confirmed |
| CLI-09 | pass | WAVE1_RESULTS.md |  |
| CLI-10 | pass | WAVE1_RESULTS.md |  |
| CLI-11 | pass | WAVE1_RESULTS.md |  |
| CLI-12 | pass | WAVE1_RESULTS.md |  |
| CLI-13 | pass | WAVE1_RESULTS.md |  |
| CLI-14 | pass | WAVE1_RESULTS.md |  |
| CLI-15 | pass | WAVE1_RESULTS.md |  |
| CLI-16 | pass | WAVE1_RESULTS.md |  |
| CLI-17 | pass | WAVE1_RESULTS.md |  |
| CLI-18 | pass | WAVE1_RESULTS.md |  |
| CLI-19 | pass | WAVE1_RESULTS.md |  |
| CLI-20 | deviation | WAVE1_RESULTS.md | final --verbose build deferred to Wave 2; F-CLI-7/F-CLI-8 confirmed |
| CFG-01 | pass | WAVE1_RESULTS.md |  |
| CFG-02 | pass | WAVE1_RESULTS.md |  |
| CFG-03 | pass | WAVE1_RESULTS.md |  |
| CFG-04 | pass | WAVE1_RESULTS.md |  |
| CFG-05 | pass | WAVE1_RESULTS.md |  |
| CFG-06 | pass | WAVE1_RESULTS.md |  |
| CFG-07 | pass | WAVE1_RESULTS.md |  |
| CFG-08 | pass | WAVE1_RESULTS.md |  |
| CFG-09 | pass | WAVE1_RESULTS.md |  |
| CFG-10 | pass | WAVE1_RESULTS.md |  |
| CFG-11 | pass | WAVE1_RESULTS.md |  |
| CFG-12 | pass | WAVE1_RESULTS.md | falsifier fired: offline=yes config fails validate (F-CFG-2) |
| CFG-13 | pass | WAVE1_RESULTS.md |  |
| CFG-14 | pass | WAVE1_RESULTS.md |  |
| CFG-15 | pass | WAVE1_RESULTS.md |  |
| CFG-16 | pass | WAVE1_RESULTS.md |  |
| CFG-17 | pass | WAVE1_RESULTS.md | falsifier fired: 3 of 4 key combinations succeed (F-CFG-3) |
| CFG-18 | pass | WAVE1_RESULTS.md |  |
| CFG-19 | pass | WAVE1_RESULTS.md |  |
| ART-01 | pass | WAVE1_RESULTS.md |  |
| ART-02 | pass | WAVE1_RESULTS.md |  |
| ART-03 | pass | WAVE1_RESULTS.md |  |
| ART-04 | pass | WAVE1_RESULTS.md |  |
| ART-14 | deviation | WAVE1_RESULTS.md | 9/9 sub-cases correct; N-ART-1/2/3 wording+reporter defects |
| ART-15 | pass | WAVE1_RESULTS.md |  |
| ART-16 | pass | WAVE1_RESULTS.md |  |
| MED-01 | pass | WAVE1_RESULTS.md |  |
| MED-02 | pass | WAVE1_RESULTS.md |  |
| MED-03 | pass | WAVE1_RESULTS.md |  |
| MED-04 | pass | WAVE1_RESULTS.md |  |
| MED-05 | pass | WAVE1_RESULTS.md |  |
| MED-06 | pass | WAVE1_RESULTS.md |  |
| SUP-01 | pass | WAVE1_RESULTS.md |  |
| SUP-02 | pass | WAVE1_RESULTS.md |  |
| SUP-11 | pass | WAVE1_RESULTS.md |  |
| SUP-13 | deviation | WAVE1_RESULTS.md | 7 check contexts, not 4 (D-5); F-REPO-4 confirmed |
| SUP-14 | deviation | WAVE1_RESULTS.md | credentials present; rulesets [] (F-REPO-2) |
| SUP-15 | pass | WAVE1_RESULTS.md |  |
| SUP-16 | fail | WAVE1_RESULTS.md | private-vulnerability-reporting = {"enabled":false} (F-REPO-1) |
| SUP-17 | pass | WAVE1_RESULTS.md |  |
| SUP-18 | pass | WAVE1_RESULTS.md |  |
| DOC-02 | pass | WAVE1_RESULTS.md |  |
| DOC-03 | pass | WAVE1_RESULTS.md |  |
| DOC-05 | pass | WAVE1_RESULTS.md |  |
| DOC-06 | pass | WAVE1_RESULTS.md |  |
| DOC-07 | deviation | WAVE1_RESULTS.md | site served at /incusos-builder/, not / (F-DOC-10); 15/15 pages 200 |
| DOC-08 | pass | WAVE1_RESULTS.md |  |
| DOC-09 | deviation | WAVE1_RESULTS.md | CONTRIBUTING's 'full local gate' claim false (F-GATE-1) |
| DOC-11 | pass | WAVE1_RESULTS.md |  |
| DOC-13 | deviation | WAVE1_RESULTS.md | validate accepts http:// --server (F-DOC-9); Wave 2 build steps deferred |
| DOC-16 | deviation | WAVE1_RESULTS.md | incus + sha256sum present, plan expected MISSING (D-8) |
| DOC-17 | deviation | WAVE1_RESULTS.md | F-DOC-1/3/4/5 confirmed, F-DOC-8 green, F-DOC-2 refuted for macOS 26 |
| DOC-18 | pass | WAVE1_RESULTS.md |  |
| BOOT-01 | pass | WAVE1_RESULTS.md | venue decision recorded: remote x86_64 Linux Incus host required |

### Wave 2 — track A (live build chain, media, doc walkthroughs)

| Case | Result | Evidence | Notes |
|------|--------|----------|-------|
| PRE-07-W2 | partial | WAVE2_TRACKB_RESULTS.md | track B portion only; root:e2e deferred to track A |
| ART-05 |  |  |  |
| ART-06 |  |  |  |
| ART-07 |  |  |  |
| ART-08 |  |  |  |
| ART-09 |  |  |  |
| ART-10 |  |  |  |
| ART-11 |  |  |  |
| ART-12 |  |  |  |
| ART-13 |  |  |  |
| ART-17 |  |  |  |
| ART-18 |  |  |  |
| ART-19 |  |  |  |
| ART-20 |  |  |  |
| ART-21 |  |  |  |
| MED-07 |  |  |  |
| MED-08 |  |  |  |
| MED-09 |  |  |  |
| MED-10 |  |  |  |
| MED-11 |  |  |  |
| MED-12 |  |  |  |
| MED-13 |  |  |  |
| MED-14 |  |  |  |
| MED-15 |  |  |  |
| MED-16 |  |  |  |
| MED-17 |  |  |  |
| DOC-01 |  |  |  |
| DOC-04 |  |  |  |
| DOC-10 |  |  |  |
| DOC-12 |  |  |  |
| DOC-14 | pass | WAVE1_RESULTS.md | Wave 1 half only; mirror build deferred to Wave 2 |
| DOC-15 |  |  |  |

### Wave 2 — track B (container image, release rehearsal)

| Case | Result | Evidence | Notes |
|------|--------|----------|-------|
| SUP-03 | pass | WAVE2_TRACKB_RESULTS.md | built in 39 s, not 5-12 min (D-9); both tags loaded, image 952969c4a012 |
| SUP-04 | pass | WAVE2_TRACKB_RESULTS.md | apko exits 1 at index signature verification; no apko cache existed |
| SUP-05 | pass | WAVE2_TRACKB_RESULTS.md | dev (59c268b) built 2026-08-16T20:05:16Z — byte-exact vs HEAD + vars file |
| SUP-06 | pass | WAVE2_TRACKB_RESULTS.md | User=65532, entrypoint /usr/bin/incusos-builder; runtime uid proven kernel-side, gid metadata-only |
| SUP-07 | pass | WAVE2_TRACKB_RESULTS.md | status --untracked-files=all empty; 9 paths each matched by an explicit .gitignore rule |
| SUP-08 | pass | WAVE2_TRACKB_RESULTS.md | sbom-aarch64.spdx.json 19 pkgs / sbom-index.spdx.json 3 pkgs, both SPDX-2.3 (F-SBOM-2) |
| SUP-09 | pass | WAVE2_TRACKB_RESULTS.md | syft v1.43.0 (self-reports [not provided]); 174 pkgs; 4 apk pkgs match apko (F-SBOM-1) |
| SUP-10 | pass | WAVE2_TRACKB_RESULTS.md | envelope cmp-identical to golden; exactly one JSON doc on stdout; empty stdin exit 3 |
| SUP-12 | pass | WAVE2_TRACKB_RESULTS.md | run 31969505600 success, 4/4 jobs, 9 assets, 0 publishing calls, 7m03s |

### Wave 2 — track C (boot acceptance, Linux host)

| Case | Result | Evidence | Notes |
|------|--------|----------|-------|
| BOOT-02 |  |  |  |
| BOOT-03 |  |  |  |
| BOOT-04 |  |  |  |
| BOOT-05 |  |  |  |
| BOOT-06 |  |  |  |
| BOOT-07 |  |  |  |
| BOOT-08 |  |  |  |
| BOOT-09 |  |  |  |
| BOOT-10 |  |  |  |

### Wave P — post-tag

| Case | Result | Evidence | Notes |
|------|--------|----------|-------|
| SUP-19 |  |  |  |
| SUP-20 |  |  |  |
| SUP-21 |  |  |  |
| SUP-22 |  |  |  |
| SUP-23 |  |  |  |

### Sign-off

| Field | Value |
|-------|-------|
| Commit tested |  |
| Operator |  |
| Start / end date |  |
| Blockers open at sign-off |  |
| Boot acceptance verdict |  |
| Accepted risks (owner, issue) |  |
| Decision | ship / hold |
