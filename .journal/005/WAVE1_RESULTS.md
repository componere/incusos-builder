---
title: Wave 1 execution results
plan: .journal/005/FUNCTIONAL_TEST_PLAN.md
commit tested: 59c268b (docs(reference): describe every configuration property, #20)
executed: 2026-08-16
operator: Main + four functional-tester agents (W1CLI, W1CFG, W1ArtMed, W1SupDoc)
---

# Wave 1 — execution results

## Verdict

**75 of 75 cases executed. 1 fail, 10 deviations, 64 pass. No case blocked.**

The CLI contract, the config/SOPS schema, the metadata-rejection gates, and the
rescue usage errors all behave exactly as documented — every quoted error
sentence came back byte-exact. The single failure is repository state, not code:
`SECURITY.md` points reporters at a private-vulnerability-reporting channel that
is disabled.

| Slice | Cases | pass | deviation | fail |
|-------|-------|------|-----------|------|
| CLI-01..20 | 20 | 18 | 2 | 0 |
| CFG-01..19 | 19 | 19 | 0 | 0 |
| ART (7) + MED (6) | 13 | 12 | 1 | 0 |
| SUP (9) + DOC (13) + BOOT-01 | 23 | 15 | 7 | 1 |
| **Total** | **75** | **64** | **10** | **1** |

Both deliberate falsifiers fired as designed: CFG-12 (interactive `init` with
offline=yes emits an invalid config) and CFG-17 (the SOPS how-to's
`SOPS_AGE_KEY_FILE` claim is false).

## Shared preflight (run by the lead)

| Gate | Result | Note |
|------|--------|------|
| PRE-01 host/toolchain | pass | darwin arm64, macOS 26.4, nine mise pins resolve |
| PRE-02 environment hygiene | **deviation** | `CI=true` **and** `NO_COLOR=1` are set in this harness. Both silently defeat prompt and colour cases; every tester had to `env -u CI` and unset `NO_COLOR`. |
| PRE-03 tools | pass | all required tools present; `sops` 3.13.1, `syft` present, `/sbin/sha256sum` present |
| PRE-04 network/disk | pass | HTTP 200 from the update server; 237 GB free |
| PRE-05 build binary | pass | `incusos-builder dev (none) built unknown` / `incus-os API: v0.0.0-20260815030500-0f5b8057f2fc` |
| PRE-06 pre-screen | **deviation** | `root:lint` passes only after `golangci-lint cache clean`; `root:format` fails because it walks the gitignored `reference/incus-os/` clone. Scoped `golangci-lint fmt --diff ./cmd/... ./internal/...` is clean. → F-GATE-1, F-GATE-2 |

**The tested commit is `59c268b`, not the `5337e7e` the plan was written
against.** PR #20 rewrote `docs/docs/reference/configuration.md`, shifting all
eight fenced-YAML examples by exactly +12 lines. Every affected case was
re-derived; see D-1 in the CFG slice.

## The one failure

**SUP-16 — `SECURITY.md`'s reporting channel is dead.**
`GET /repos/componere/incusos-builder/private-vulnerability-reporting` →
`{"enabled":false}`. The advisory URL in `SECURITY.md:11` redirects an
unauthenticated reporter to a login page. This is release blocker #3 in §7 of
the plan and is fixed by enabling the setting (or by rewriting SECURITY.md).

## New findings — code defects

| ID | Severity | Finding |
|----|----------|---------|
| **N-CLI-1** | high | `ACCESSIBLE=1 init` cannot be cancelled. The accessible prompt path leaves the terminal in raw mode with ISIG off: Ctrl-C echoes a literal `^C`, Ctrl-D echoes `^D`, and an external `kill -INT` is ignored. Keystrokes typed meanwhile are consumed as form answers. The only exits are completing the form or SIGKILL. Contradicts `cli.md:186-190`'s cancel promise. The normal TUI path cancels correctly. |
| **F-DOC-9** | medium | `validate` silently accepts a plain-HTTP `--server`: `validate --server http://example.invalid/os` → `configuration valid`, exit 0, while `versions` and `build` correctly refuse with exit 2. `run-in-ci.md` §4 states the rule unconditionally. Either scope the doc or apply the check where the flag is accepted. |
| **N-ART-2** | medium | The step reporter prints `done index` / `done download` for steps that then immediately fail. In ART-14(h) `done download` is emitted although the asset was never opened; in (i) `done index` is emitted although `index.json` does not exist. stderr asserts a step succeeded and then reports the same step failing. |
| **N-ART-1** | cosmetic | Doubled error prefix: `acquisition failed: gzip: gzip: invalid header`. |
| **N-ART-3** | cosmetic | Local-mirror open errors double the verb and leak the resolved absolute path: `open aarch64/absent.img.gz: open /var/folders/.../absent.img.gz: no such file or directory`. |
| **N-CLI-5** | cosmetic | `ACCESSIBLE=1` drops every field description, so the "default stable" hint is invisible precisely to the users who need it most. |
| **N-CFG-A** | advisory | Sanitization is inconsistent between paths: the decrypt path passes go-yaml's message through unfiltered, echoing a 7-character fragment of the offending scalar (`cannot unmarshal !!str \`not-a-s...\``), where every other path renders `<value>`. |
| **N-CFG-B** | advisory | Decrypt-path errors are the only multi-line stderr messages; line-wise stderr greps will truncate them. |
| **N-ART-4** | advisory | `versions` never writes to `--cache-dir`; `index.json` is refetched on every invocation. Not a stated promise, but it affects Wave 2 timing assumptions and `cache.md`. |
| **N-ART-5** | cosmetic | The `versions` "table" is two-space-joined text; 12-character version values push every column out of alignment with the header. |
| **N-CFG-C** | cosmetic | Interactive `init` shows `stable` as a Channel *placeholder*; backspace/`C-u` leave it unchanged, which reads as an unclearable prefilled field. Behavior is correct; only the affordance is ambiguous. |
| **N-CLI-2** | cosmetic | `--resources-output` description drift: `cli.md:77` says "Rescue-media output path", the binary says "resources-media output path". |

## New findings — documentation

- **F-DOC-10** — the docs site is **not** served at `/`. `mkdocs.yml:3` `site_url`
  mounts the dev server under `/incusos-builder/`; `http://127.0.0.1:8000/`
  returns `302 Found`. Re-swept at the correct path: **15/15 pages 200 OK**, all
  titles correct, edit links present, search index 200 with 169 documents.
- **F-DOC-2 must be re-scoped.** `/sbin/sha256sum` exists on this host and is an
  Apple-signed base-system binary (`Identifier=com.apple.sha224sum`,
  `Platform identifier=26`, hard-linked into the `/sbin/*sum` family). The
  finding as written ("does not exist on stock macOS") is **false for macOS 26**;
  it still holds for macOS ≤ 15. `/usr/bin/sha256sum` is indeed absent.

## Findings confirmed (no re-litigation needed)

All previously recorded findings were reproduced against the real binary:

- **F-CLI-1..F-CLI-8** — all eight confirmed, none refuted. Notably F-CLI-5
  (operand mistakes exit 1, not 2) reproduced on all five forms, and F-CLI-6
  produced a 689-byte regular file literally named `-`, disambiguated by feeding
  *invalid* YAML on stdin and watching `validate -f ./-` report the file's
  contents instead.
- **F-CFG-1, F-CFG-2, F-CFG-3** — confirmed. F-CFG-2 was driven through a real
  PTY: the offline=yes form emits a config with no `seeds` block that `validate`
  rejects at exit 3.
- **F-DOC-1, F-DOC-3, F-DOC-4, F-DOC-5** — confirmed with side-by-side output.
  **F-DOC-8** tripwire green (pin matches `go.mod:14`).
- **F-REPO-1..F-REPO-4** — all four confirmed with the API response quoted;
  F-REPO-3 is exactly seven planned changes, F-REPO-4 is PR #10 proposing
  **0.1.2** with `CHANGELOG.md` ending in a stray `## Changelog`.

## Plan defects found while executing

| ID | Defect |
|----|--------|
| D-1 | All eight `configuration.md` line ranges are stale at `59c268b` (+12 each). Every example survived PR #20 with an identical caption and line count. New ranges: 791-794, 800-808, 815-823, 830-847, 853-861, 867-878, 885-1034, 1040-1047. |
| D-2 | CFG-04's prose says the "every accepted seed section present" example is "all eleven as `{}`". At HEAD, ten are `{}` and `applications` carries a real entry. The asserted outcome still holds. |
| D-3 | CFG-16 silently depends on CFG-13 having exported `SOPS_AGE_KEY` in the same shell. With the key genuinely unset, the MAC-mismatch fixture fails earlier with a different (still exit 4) message. |
| D-4 | CLI-08's cost line claims the HTTPS retry ladder takes "~1 min". Measured: **1 second** — `retryAttempts = 3`, `retryBase = 100ms`, backoff capped at `retryBase * 2^3`. |
| D-5 | SUP-13 expects four check contexts; `gh pr checks` reports seven (the four required ones plus `Deploy GitHub Pages` skipping and two Melange matrix legs). Under-specified, not a repo problem. |
| D-6 | DOC-17's prose attributes F-DOC-4 to "the `--server` error"; `run-in-ci.md:49` is the *overwrite* refusal. The §5 table states it correctly. |
| D-7 | The awk extractor finds **nine** fenced YAML blocks, not eight — `34-38` is the intro skeleton, not an Example. |
| D-8 | DOC-16 and BOOT-01 expect `incus` and `sha256sum` to be MISSING on macOS; both are present here. Verdicts unchanged (a Homebrew client is not an `x86_64` Linux Incus host). |

## Environment notes for Wave 2

- **Docker is reachable** — `29.4.0 OrbStack aarch64`. Wave 2 track B (SUP-03..10) is **not** blocked.
- **Wave 2 track C is still blocked on this host** — arm64, no `/dev/kvm`, and the
  local `incus` is a Homebrew client. BOOT-01's verdict stands: a remote
  `x86_64` Linux Incus host is required, QEMU is known-insufficient, deferral is
  risk acceptance and not a pass.
- **Live index at execution time** (reusable by Wave 2, saves a fetch): releases
  `202608021451`, `202608072311`, `202608102114`; every release carries channels
  `["testing","stable"]` and architectures `aarch64` + `x86_64`, each publishing
  `raw` and `iso`.
- **Harness hygiene is load-bearing.** `CI=true` and `NO_COLOR=1` are both set by
  default here. Any Wave 2 case touching prompts or colour must clear them, and
  interactive cases must run on a private tmux socket (`-L <agent>`); a shared
  socket was destroyed mid-run by a sibling's `kill-server`.

---

# Appendix — verbatim slice reports

## Slice report: W1CLI (CLI-01..20)

## Slice: CLI — 20 cases

`$WORK` = `/var/folders/gv/kllfch6d5l9dq4676hq4260m0000gn/T/tmp.m0bkGgXb5N` · binary `/Users/josh/code/componere/incusos-builder/bin/incusos-builder` · all non-TTY runs prefixed `env -u CI`; `INCUSOS_BUILDER_*`/`SOPS_*` unset (verified `env | grep -c` = 0).

### Results

| Case | Result | Observed evidence (verbatim, one line) | Deviation |
|------|--------|----------------------------------------|-----------|
| CLI-01 | pass | bare run `rc=0`, `0` bytes; `--version` = 2 lines `incusos-builder dev (none) built unknown` / `incus-os API: v0.0.0-20260815030500-0f5b8057f2fc`; `-v` identical; help lists `completion`/`help`; `--cache-dir` default `"/Users/josh/Library/Caches/incusos-builder"` | — |
| CLI-02 | pass | `-f, --config -   path to config YAML (- reads stdin)` / `-o, --output -` / `--architecture string   architecture to list (default "aarch64")`; `init` has no `--force` | F-CLI-3 confirmed; new doc-wording mismatch (N-CLI-2) |
| CLI-03 | pass | `usage error: invalid --color "purple" (want auto, always, or never)` … `usage error: flag needs an argument: --channel`, all `rc=2`, stdout empty, no cobra usage block | — |
| CLI-04 | pass | all ten exact strings reproduced, e.g. `usage error: --resources-output requires offline: true in the config`; `--json`+`-o -` also emits `{"error":{"code":2,"message":"usage error: --json cannot be combined with -o -"}}`; only `cfgA.yaml` created | — |
| CLI-05 | pass | `usage error: --server "http://example.invalid/os": plain http is not supported; use https or a local mirror directory`; both non-dir forms give `… is neither an https URL nor an existing directory`, `rc=2` | — |
| CLI-06 | pass | `invalid config: image.type: must be iso or raw` (×2, `rc=3`); `invalid config: read config: open nope.yaml: no such file or directory`; `ls: cannot access 'out.img': No such file or directory` | — |
| CLI-07 | pass | `decryption failed: Error getting data key: 0 successful groups required, got 0` `rc=4`; stray-sops → `decryption failed: Error unmarshalling input yaml: …into stores.Metadata` `rc=4`; `--json` jq → `true`, `rc=4` | — |
| CLI-08 | deviation | `acquisition failed: cache directory is required` (rc=5); `acquisition failed: Get "https://127.0.0.1:1/os/index.json": dial tcp 127.0.0.1:1: connect: connection refused` — **elapsed=1s**, not ~1 min | plan cost defect (N-CLI-3); F-CLI-4 confirmed |
| CLI-09 | pass | `output write failed: create temp for ro/out.img: open ro/.out.img-2536629952.tmp: permission denied` rc=6; `--json` envelope `"code":6`; `ro/` empty | — |
| CLI-10 | pass | `unknown command "frobnicate" for "incusos-builder"` → `rc=1` (all five) | F-CLI-5 confirmed as written |
| CLI-11 | pass | all four `jq -e` printed `true`; `wc -l` = `1`; stderr repeat `invalid config: image.type: must be iso or raw` | — |
| CLI-12 | pass | `configuration valid`; `-q` → `0`; `{"result":{"valid":true,"type":"raw","architecture":"x86_64","offline":false}}`; versions jq `true`; `init -o -` has 0 `^wrote` lines; `init -q` 0 bytes + `cfgC.yaml` exists | — |
| CLI-13 | pass | `{"error":{"code":2,"message":"usage error: unknown flag: --nope"}}` rc=2; env-only → stdout `0` bytes, rc=2, stderr `usage error: unknown flag: --nope` | — |
| CLI-14 | pass | `configuration valid` rc=0 (plain + `SOPS_AGE_KEY`); `decryption failed: …` rc=4; `invalid config: image.type: must be iso or raw` rc=3; `ls -a` diff → `no new files` | — |
| CLI-15 | pass | `build -o ./-` → rc=2 envelope; `init --json -o ./-` → `{"result":{"output":"./-"}}` rc=0 and `-rw-r--r--  1 josh staff 689 … -`; `validate -f ./-` with **invalid** stdin → `{"result":{"valid":true,"type":"iso",…}}` rc=0 | F-CLI-6 confirmed |
| CLI-16 | pass | env-only quotes `"/env-bad-dir"`, flag quotes `"/flag-bad-dir"`; `INCUSOS_BUILDER_JSON=1` → envelope, `--json=false` → `configuration valid`; color/progress env rc=2, cleared by explicit flag | — |
| CLI-17 | pass | `CI=1 … --no-input=false` → `usage error: refusing to overwrite victim.img; re-run with --force` rc=2, no prompt; `NO_COLOR=1`/`TERM=dumb` → `==> index`/`done index`; clean TTY → `^[[1m^[[38;5;99mM-bM-^VM-8 index` / `^[[38;5;36mM-bM-^\M-^S index` (▸/✓ + ANSI) | harness gotcha N-CLI-4 |
| CLI-18 | pass | `overwrite existing output? [y/N] n` → rc=2 refuse; `y`/`YES` → `▸ resolve` … rc=5; `< /dev/null` → no prompt rc=2; `--force` rc=5; `--no-input` rc=2; `victim intact`, sha256 `7723f7ed…09b7` unchanged; stderr file `overwrite·existing·output?·[y/N]·` proves stderr routing | — |
| CLI-19 | pass | form on stderr, title `incusos-builder init`, fields Image type/Architecture/Channel/Offline install?; Ctrl-C → `usage error: init cancelled` rc=2, **no file**; completed run `wrote config.yaml` → `type: iso`/`architecture: aarch64`/`channel: stable`/`offline: false`, `grep -c "#seeds:"` = `0`; `ACCESSIBLE=1` → `Enter a number between 1 and 2:` | new defect N-CLI-1 in ACCESSIBLE mode |
| CLI-20 | deviation | stderr bytes `0` on validate success; stdout `0` on failure rc=3; `-q` and `--progress never` still print `==> index`/`done index`; `validate --verbose` stderr = `0` bytes | final `--verbose build` deferred to Wave 2 (per instruction) |

### Failures and deviations, in detail

**CLI-08 — plan defect (cost estimate falsified).**
Plan (`FUNCTIONAL_TEST_PLAN.md:623`, Cost line): *"cheap; the HTTPS case takes ~1 min (three retry attempts)"*. Measured: `elapsed=1s rc=5`. Root cause is in the code, not the run: `internal/update/client.go:26-28` sets `retryAttempts = 3`, `retryBase = 100 * time.Millisecond`, and `jitteredBackoff` (`client.go:316-323`) caps at `retryBase * 2^min(attempt,3)` — the whole ladder is under one second. All four *behavioral* expectations of CLI-08 passed verbatim, including `acquisition failed: cache directory is required` and the `connection refused` cause. The `INCUSOS_BUILDER_CACHE_DIR=` run failed on `open index.json: open $WORK/mirror/index.json: no such file or directory` — i.e. it used the default cache dir — so **F-CLI-4 is confirmed**.

**CLI-20 — deferred step (instructed).**
The final command `"$IOB" build --verbose --json -f "$WORK/live11.yaml" -o v.img …` was not run: it belongs to Wave 2 track A and its fixture `live11.yaml` does not exist in the PRE-07-W1 scaffold. Every other step ran; the promises they cover all held, and both anchored findings are confirmed: **F-CLI-7** (`-q` and `--progress never` still emit `==> index` / `done index` on stderr — only percentage lines are gated) and **F-CLI-8** (`validate --verbose` emits exactly `0` bytes of stderr; `grep` for `slog.(Warn|Error)`/`logger.(Warn|Error)` across `internal/` and `cmd/` returns **no matches**).

**F-CLI-1..F-CLI-8 — all eight confirmed, none refuted.**
- F-CLI-1: `-v` prints the two-line banner; `docs/docs/reference/cli.md` contains no `` `-v` `` (grep: no matches). Doc defect.
- F-CLI-2: help lists `completion` and `help`; neither appears in `cli.md` (grep: no matches). Doc defect.
- F-CLI-3: help renders `-f, --config -` and `-o, --output -` because `build.go:119-120` / `init.go:100` backquote the `-` in the usage string, which pflag treats as the placeholder name. Cosmetic code defect.
- F-CLI-4: see CLI-08 above. Code defect.
- F-CLI-5: all five operand mistakes exit **1**, e.g. `unknown command "extra" for "incusos-builder build"`. Code defect for exit-code-branching automation; the docs' "exit 2 for usage errors" (`automation.md:21`) is untrue for this class.
- F-CLI-6: `init --json --no-input -o ./-` → `rc=0`, `{"result":{"output":"./-"}}`, and a 689-byte regular file literally named `-`. I disambiguated `validate -f ./-` by feeding *invalid* YAML on stdin (`type: disk`): output was `{"result":{"valid":true,"type":"iso","architecture":"x86_64","offline":false}}` — proving the file was read, not stdin. Code defect (inconsistent sentinel).
- F-CLI-7, F-CLI-8: see CLI-20 above.

### New findings not already in the known list

**N-CLI-1 (code defect, severity: high) — `ACCESSIBLE=1 init` cannot be cancelled; it swallows SIGINT and EOF.**
In the normal TUI form, Ctrl-C behaves as promised (`usage error: init cancelled`, rc=2, no file). Under `ACCESSIBLE=1` the terminal is left in raw mode with ISIG off and huh's accessible reader handles neither: `tmux send-keys C-c` echoed a literal `^C` and the process stayed alive; `C-d` echoed `^D` and it stayed alive; an **external `kill -INT 58067`** also had no effect (`pane_current_command` remained `incusos-builder`). Any keystrokes typed meanwhile are consumed as form answers — my shell line was eaten and produced `Enter a number between 1 and 2: Invalid: must be a number between 1 and 2` in the captured stderr. The only exits are completing the form or SIGKILL. Contradicts `cli.md:186-190`'s cancel promise for the accessible path.

**N-CLI-2 (doc defect, cosmetic) — `--resources-output` description drift.**
`docs/docs/reference/cli.md:77` documents *"Rescue-media output path (offline builds)"*; the binary's help and `internal/cli/build.go:121` say *"resources-media output path (offline builds)"*.

**N-CLI-3 (plan defect) — the retry-ladder timing claim.** See CLI-08. 1s measured against "~1 min" documented; any Wave-2 case budgeting time for retries is over-provisioned.

**N-CLI-4 (harness gotcha, affects other testers) — `NO_COLOR=1` is set in this agent environment.**
My first TTY attempt at CLI-17's unset-env run produced plain `==> index`/`done index` and looked like a failure. `echo $NO_COLOR` in the pane returned `1` — inherited from the harness, not from the case. After `unset NO_COLOR` the same command produced `^[[1m^[[38;5;99m▸ index` / `^[[38;5;36m✓ index`. Anyone testing `--color auto` must unset `NO_COLOR` alongside `CI`, or the case silently "passes" as plain.

**N-CLI-5 (code defect, cosmetic) — accessible mode drops field descriptions.**
The TUI form shows `Update-server channel; default stable.` and `Also build RESCUE_DATA resources media.`; `ACCESSIBLE=1` renders bare `Channel` and `Offline install? [y/N]` with no description, so the "default stable" hint is invisible to the users who most need it.

**N-CLI-6 (process, already resolved) — tmux socket collision.** My first PTY session on the shared `-L claude-agent` socket was destroyed mid-CLI-17 by a sibling's `kill-server` (W1CFG acknowledged). Re-run on `-L w1cli`; no evidence was lost, all CLI-17/18/19 results above come from the private socket.

### Repo cleanliness

```
$ git -C /Users/josh/code/componere/incusos-builder status --porcelain    # before
(empty, rc=0)
$ git -C /Users/josh/code/componere/incusos-builder status --porcelain    # after
(empty, rc=0)
```
All artifacts were confined to `$WORK` (plus `/tmp/w1_prompt.err`, `/tmp/w1_form.err`). No repo file was read-modified; the only repo paths touched were read-only fixtures under `internal/config/testdata/`.

### Time spent / cases blocked and why

≈40 minutes wall clock, of which ~18 were the three PTY cases (including one full restart after the socket collision). **Zero cases blocked.** One case partially deferred: CLI-20's final `--verbose build` (Wave 2 track A, requires an image download and a fixture that does not exist in the Wave 1 scaffold) — recorded as `deviation`, not `skipped`, because every other step of CLI-20 ran and both of its anchored findings were confirmed. Every documented exit code was reached through the real binary: **2** (CLI-03/04/05/13/15/17/18/19), **3** (CLI-06/14/20), **4** (CLI-07/14), **5** (CLI-08/11/18), **6** (CLI-09/11) — plus the undocumented **1** from CLI-10.

## Slice report: W1CFG (CFG-01..19)

## Slice: Config and SOPS — 19 cases

### Results

| Case | Result | Observed evidence (verbatim, one line) | Deviation |
|------|--------|----------------------------------------|-----------|
| CFG-01 | pass | `configuration valid` / `exit=0`; matrix e.g. `{"result":{"valid":true,"type":"raw","architecture":"aarch64","offline":false}}` | — |
| CFG-02 | pass | `{"result":{"valid":true,"type":"raw","architecture":"aarch64","offline":false}}` at `exit=0` from `sed -n '885,1034p'` | plan range 873-1022 stale |
| CFG-03 | pass | `invalid config: seeds.applications: required when image.offline is true` / `exit=3` (both absent and `applications: []`) | — |
| CFG-04 | pass | `configuration valid` / `exit=0` × 3 (null, `{}`, doc 830-847) | plan range 818-835 stale; doc example is no longer "all eleven as `{}`" |
| CFG-05 | pass | `invalid config: seeds.install.target.sort_order: must be empty, smallest, or largest` / `medium exit=3`; other five `exit=0` | — |
| CFG-06 | pass | `invalid config: seeds.security.encryption_recovery_keys: it is not possible to set encryption recovery key(s) via the security seed; see https://linuxcontainers.org/incus-os/docs/main/reference/system/security/`; `grep -c` → `0` | plan ranges 1028-1035 and 855-866 stale |
| CFG-07 | pass | `invalid config: version: required` / `… unsupported schema version; a newer CLI is required` / `invalid config: image.type: must be iso or raw`, all `exit=3` | — |
| CFG-08 | pass | `invalid config: seeds.kernel.blacklist_modules: unknown to incus-os v0.0.0-20260815030500-0f5b8057f2fc; a newer incusos-builder may accept this` | — |
| CFG-09 | pass | `invalid config: cannot construct !!str <value> into config.seeds`; leak grep → `0`,`0`,`0` | confirms F-CFG-1 |
| CFG-10 | pass | `invalid config: go-yaml load error in scanner (while scanning for the next token) at L3.C1: found character that cannot start any token` (+2 others byte-exact) | — |
| CFG-11 | pass | `usage error: refusing to overwrite existing file gen.yaml` / `exit=2`; `grep -c '^#  '` → `11`, order exact | — |
| CFG-12 | pass (falsifier fires) | `invalid config: seeds.applications: required when image.offline is true` / `exit=3` for the offline=yes form output | confirms F-CFG-2 |
| CFG-13 | pass | `version: ENC[AES256_GCM,data:Qg==,iv:bJONFAF4fy/CQ8/XpMkkdgIo9cBJNytwrEN3Bt+M058=,tag:KbK6s5BPn4DEXNSedvQo9g==,type:int]`; both runs `configuration valid` / `exit=0`; `ls -a` → `.` `..` only | — |
| CFG-14 | pass | recipient `age10kg4k848vfdhvjjv04myq3rhmdhaamgpfgkg0pkq9ehu3eyf29ysfapsl8`; `{"result":{"valid":true,"type":"raw","architecture":"aarch64","offline":false}}` `exit=0` file and stdin | sops is 3.13.1, not the 3.11.0 the plan names |
| CFG-15 | pass | `decryption failed: Error getting data key: 0 successful groups required, got 0` / `exit=4` × 3, stdout empty | — |
| CFG-16 | pass | five failures all `exit=4`; `{"error":{"code":4,"message":"decryption failed: Error getting data key: 0 successful groups required, got 0"}}` | plan omits that CFG-13's `SOPS_AGE_KEY` export is a precondition for the MAC-mismatch string |
| CFG-17 | pass (falsifier fires) | `SOPS_AGE_KEY_FILE=` + valid `SOPS_AGE_KEY` → `configuration valid` / `exit=0` | confirms F-CFG-3 |
| CFG-18 | pass | `acquisition failed: Get "https://images.linuxcontainers.org/os/index.json": dial tcp: lookup images.linuxcontainers.org: no such host` / `sandboxed versions exit=5`, both sandboxed validates `exit=0` | — |
| CFG-19 | pass | seven `configuration valid` `exit=0`, eighth the recovery-key refusal at `exit=3` | all eight plan ranges stale |

19 pass, 0 fail, 0 blocked. Every quoted sentence in §4.2 came back byte-exact.

### Failures and deviations, in detail

No code defect found in this slice. Everything non-pass-shaped is either a stale plan reference or a confirmation of an already-known finding.

**D-1 — All eight `configuration.md` fenced-YAML ranges in the plan are stale (plan defect, caused by PR #20).**
Extractor used: `awk '/^```yaml$/{s=NR} /^```$/{if(s){print s+1"-"NR-1; s=0}}' docs/docs/reference/configuration.md`.
It reports **nine** yaml fences; the first, `34-38`, is the document-shape skeleton in the intro, not one of the eight Examples. The eight Examples ranges I used, with the plan's stale value:

| # | Caption in `## Examples` | Plan range | Range I used | Δ |
|---|---|---|---|---|
| 1 | Minimal accepted document | 779-782 | **791-794** | +12 |
| 2 | Checked-in valid fixture | 788-796 | **800-808** | +12 |
| 3 | Offline document | 803-811 | **815-823** | +12 |
| 4 | Every accepted seed section present | 818-835 | **830-847** | +12 |
| 5 | CLI kernel extension | 841-849 | **853-861** | +12 |
| 6 | CLI security extension, empty recovery list | 855-866 | **867-878** | +12 |
| 7 | Populated eleven-section shape | 873-1022 | **885-1034** | +12 |
| 8 | Rejected recovery keys | 1028-1035 | **1040-1047** | +12 |

Every example survived PR #20 with an identical line count and an identical caption; the page simply shifted by 12 lines. No example was deleted or reshaped. CFG-02 used 885-1034, CFG-04 used 830-847, CFG-06 used 1040-1047 and 867-878, CFG-19 used all eight.

**D-2 — CFG-04's prose expectation no longer matches the doc (plan defect).**
The plan says the third document is "the doc's *every accepted seed section present* example with all eleven as `{}`". At 59c268b that example is not all-`{}`: `applications` carries a real entry.
```
seeds:
  applications:
    applications:
      - name: incus
  install: {}
  incus: {}
  …
```
The asserted outcome (`configuration valid`, `exit=0`) still holds, so the case passes; only the description is stale. Ten sections are `{}`, one is populated.

**D-3 — CFG-16 depends on an unstated environment precondition (plan defect).**
The plan's CFG-16 command block never sets `SOPS_AGE_KEY`; it relies on CFG-13 having exported it earlier in the same shell. With the key genuinely unset, `encrypted-mac-mismatch.yaml` does **not** produce the asserted MAC string — it fails earlier:
```
decryption failed: Error getting data key: 0 successful groups required, got 0
```
`exit=4` either way, so the case's real promise (never 3) holds. With `SOPS_AGE_KEY` exported, the asserted string is byte-exact:
```
decryption failed: Failed to decrypt original mac: Could not decrypt with AES_GCM: cipher: message authentication failed
```
The plan should state the export explicitly rather than leaning on shell carry-over.

**D-4 — CFG-14 host `sops` is 3.13.1, not the 3.11.0 named in the case source (environmental).**
`sops 3.13.1`. The plan's `CFG-17` source line names the *linked library* (`go.mod` `getsops/sops/v3 v3.11.0`), which is the version that actually governs decryption; the CLI version only governs encryption. Both directions interoperated, so no impact — recorded so the evidence is reproducible.

**Known findings confirmed, not rediscovered:**
- **F-CFG-1 confirmed.** `internal/config/load.go:191-211`. `seeds: notamapping` → `invalid config: cannot construct !!str <value> into config.seeds` — a Go type name where a YAML path belongs. No secret leaks: `grep -cE 'one|maybe|notamapping'` returned `0` for all three CFG-09 documents. Doc/wording defect, cosmetic.
- **F-CFG-2 confirmed** (CFG-12, real PTY, `CI` unset, tmux `-L claude-agent` session `w1cfg`, 200×50). Answers ISO / x86_64 / channel left empty / **offline yes** produced:
  ```
  version: 1
  image:
    type: iso
    architecture: x86_64
    channel: stable
    offline: true
  ```
  with **no** `seeds` block, and `validate` rejected it:
  ```
  invalid config: seeds.applications: required when image.offline is true
  exit=3
  ```
  The promise is `internal/cli/init.go:260-261`, that the emitted body is a valid `config.Parse` input. **Code defect.** The control run (raw / aarch64 / channel `daily` / offline **no**) emitted exactly `version: 1` / `type: raw` / `architecture: aarch64` / `channel: daily` / `offline: false` and validated at `exit=0`, so the defect is specific to the offline=yes branch.
- **F-CFG-3 confirmed** (CFG-17, four-way matrix, promise at `docs/docs/how-to/sops-encryption.md:36-38`: *"An empty `SOPS_AGE_KEY_FILE` makes SOPS open path `""` instead of using `SOPS_AGE_KEY`."*):

  | `SOPS_AGE_KEY` | `SOPS_AGE_KEY_FILE` | Observed | Exit |
  |---|---|---|---|
  | valid | `` (empty) | `configuration valid` | 0 |
  | valid | `/nonexistent/age.key` | `configuration valid` | 0 |
  | `` (empty) | `` (empty) | `decryption failed: Error getting data key: 0 successful groups required, got 0` | 4 |
  | `` (empty) | real `testdata/age.key` | `configuration valid` | 0 |

  Three of four succeeded. An empty or bogus `SOPS_AGE_KEY_FILE` does **not** suppress a valid `SOPS_AGE_KEY`; the how-to's sentence is false as written. `SOPS_AGE_KEY_FILE` alone is a usable source. **Doc defect** — downgrade to a hygiene note.

**CFG-18 sandbox control validated before trusting the result.** Unsandboxed `versions` exited 0; the same command under `sandbox-exec -f deny-net.sb` exited **5** with `dial tcp: lookup images.linuxcontainers.org: no such host`, so `(deny network*)` demonstrably bites. Both `validate` runs then succeeded under the same sandbox — plaintext and SOPS-encrypted — at `exit=0`. The `build` asymmetry control behaved as promised: `usage error: --server "/definitely-not-a-mirror" is neither an https URL nor an existing directory`, `exit=2`, while `validate` with the identical hostile `--server` printed `configuration valid` at `exit=0`.

### New findings not already in the known list

**N-CFG-A — `stray-sops.yaml` and a scalar `sops:` key echo a truncated fragment of the offending value.** Everywhere else the loader sanitizes literals to `<value>` (CFG-09, `configuration.md:59-60,90-91`), but the decrypt path passes the go-yaml message through unfiltered:
```
decryption failed: Error unmarshalling input yaml: yaml: unmarshal errors:
  line 5: cannot unmarshal !!str `not-a-s...` into stores.Metadata
```
and, for `sops: true`, `` cannot unmarshal !!bool `true` into stores.Metadata ``. The truncation to 7 characters is go-yaml's, not the CLI's — it is not a deliberate redaction, and a longer scalar would still expose its first 7 bytes. The value under a `sops:` key is metadata rather than a secret, so the practical exposure is low, but the sanitization promise is inconsistent between the two paths. Advisory.

**N-CFG-B — decrypt-path errors are the only multi-line stderr messages in the slice.** Every other `invalid config:` / `decryption failed:` message is exactly one line (CFG-06 verified with `wc -l` → `1`). The two `stores.Metadata` cases emit two. Anything grepping stderr line-wise will truncate them. Advisory; no documented single-line promise exists.

**N-CFG-C — interactive `init`'s Channel field shows `stable` as a placeholder, not a value.** Backspace and `C-u` leave the display unchanged, which reads as an unclearable prefilled field. Submitting empty yields `channel: stable` in the output anyway (the documented empty→stable default), so behavior is correct; only the affordance is ambiguous. Cosmetic, TTY-only.

**Non-finding, recorded to close it out:** the `--json` decrypt failure writes the envelope to stdout *and* reprints the human error on stderr. `automation.md:183-191` documents exactly that (*"Error reprint after every failure (including `--json`)"*), so it is correct, not a leak of a second document.

### Repo cleanliness

```
$ git -C /Users/josh/code/componere/incusos-builder status --porcelain      # before
                                                                            # (empty)
$ git -C .../incusos-builder rev-parse HEAD
59c268b1499bd1a5ce94d487f1bc3adf377bead3
```
```
$ git -C /Users/josh/code/componere/incusos-builder status --porcelain      # after
                                                                            # (empty)
$ git -C .../incusos-builder status --porcelain internal/config/testdata/
                                                                            # (empty)
$ git -C .../incusos-builder rev-parse HEAD
59c268b1499bd1a5ce94d487f1bc3adf377bead3
```
Both empty, HEAD unchanged. `internal/config/testdata/` was read only — the four failure fixtures were validated in place with no writes, and `age.key` / `age.pub` were read via `tail`/`cat` only. All scratch output lived in `/tmp/w1cfg` (plus `/tmp/w1cfg/cfg13` for CFG-13's leak check, which ended with `ls -a` showing only `.` and `..`, proving no plaintext was written).

### Time spent / cases blocked and why

Roughly 18 minutes wall clock. No case blocked, none skipped, none deferred to Wave 2. CFG-12 was the only case needing a PTY and consumed about 7 minutes across its two interactive runs; the remaining 18 cases are sub-second each.

One process note: I ran `tmux -L claude-agent kill-server` after CFG-12, which killed W1CLI's concurrent session on the shared socket. I acknowledged it to W1CLI and broadcast the correction; per-agent sockets (`-L w1cfg`) are the standing policy now. My own evidence was captured before the teardown and is unaffected, but W1CLI had to re-run on `-L w1cli`.

## Slice report: W1ArtMed (ART + MED)

## Slice: server metadata and rescue usage errors — 13 cases

`$WORK` = `/var/folders/gv/kllfch6d5l9dq4676hq4260m0000gn/T/tmp.NC4sWQqmks`
Binary: `/Users/josh/code/componere/incusos-builder/bin/incusos-builder` (HEAD 59c268b)
tmux: not used — no case in this slice is interactive. Socket `-L w1artmed` was never created (verified: `error connecting to /private/tmp/tmux-501/w1artmed (No such file or directory)`), so the shared-socket kill-server incident could not have affected any evidence here.

### Results

| Case | Result | Observed evidence (verbatim, one line) | Deviation |
|------|--------|----------------------------------------|-----------|
| ART-01 | pass | stdout L1 `Version  Channel  Architecture  Type`, L2 `202608102114  stable  aarch64  raw`; stderr only `==> index` / `done index`; `exit=0` | — |
| ART-02 | pass | `.result.versions[0]\|keys` → `architectures,channels,published_at,version`; default `architectures` `["aarch64"]`; one newline-terminated doc each | `published_at` carries nanoseconds (`2026-08-10T22:30:02.88333048Z`) — still RFC 3339 |
| ART-03 | pass | `{"result":{"versions":[]}}` `exit=0`; human run prints only `Version  Channel  Architecture  Type` `exit=0` | — |
| ART-04 | pass | stderr `acquisition failed: cache directory is required`, `exit=5`; `--json` stdout `{"error":{"code":5,"message":"acquisition failed: cache directory is required"}}` | stderr reprint under `--json` is documented at `automation.md:188` |
| ART-14a | deviation | `acquisition failed: gzip: gzip: invalid header`, `exit=5`; `$TC/sha256/5f70bf…c6ef` present `-r--r--r--` 1024 B | error string doubles the `gzip:` prefix |
| ART-14b | pass | `acquisition failed: sha256 "abc123" rejected; untrusted metadata; possible tampering`; `$TC` never created | — |
| ART-14c | pass | `acquisition failed: sha256 "5F70BF18A086007016E948B04AED3B82103A36BEA41755B6CDDFAF10ACE3C6EF" rejected; untrusted metadata; possible tampering`; `$TC` never created | — |
| ART-14d | pass | `acquisition failed: size 0 rejected; untrusted metadata; possible tampering`; `$TC` never created | — |
| ART-14e | pass | `acquisition failed: size 8589934593 rejected; untrusted metadata; possible tampering`; `$TC` never created | — |
| ART-14f | pass | `acquisition failed: filename "../evil.img.gz" rejected; untrusted metadata; possible tampering`; no `$WORK/evil.img.gz`; `$TC` never created | — |
| ART-14g | deviation | `acquisition failed: "aarch64/IncusOS_202601010000.img.gz": asset failed size/digest admission; untrusted metadata; possible tampering`; `$TC/sha256/` exists and is empty | stderr printed `done download` immediately before the failure |
| ART-14h | deviation | `acquisition failed: open aarch64/absent.img.gz: open /…/tiny/202601010000/aarch64/absent.img.gz: no such file or directory`; `$TC` never created | doubled `open` verb + absolute mirror path leaked; `done download` printed before failure |
| ART-14i | deviation | `acquisition failed: open index.json: open /…/tiny/index.json: no such file or directory`; `$TC` never created | `done index` printed before the index open failed |
| ART-15 | pass | unpinned → `acquisition failed: open aarch64/new.img.gz: …`; pinned → `acquisition failed: open aarch64/old.img.gz: …`; both `exit=5`; `$WORK/c1` never created | — |
| ART-16 | pass | `{"error":{"code":5,"message":"version not found: release \"199901010000\" not in channel \"stable\"; available: 202608021451, 202608072311, 202608102114"}}` `exit=5` | — |
| MED-01 | pass | `usage error: offline builds cannot use -o -` `exit=2` | — |
| MED-02 | pass | `invalid config: seeds.applications: required when image.offline is true` `exit=3`; no `out.img` | — |
| MED-03 | pass | `usage error: --resources-output requires offline: true in the config` `exit=2` | — |
| MED-04 | pass | `usage error: resources path cannot be -` `exit=2` | — |
| MED-05 | pass | both forms → `usage error: image and resources paths must be distinct` `exit=2` | — |
| MED-06 | pass | five refusals naming `a.resources.img`, `b.resources.iso`, `c.img.resources.img`, `d.resources.img`, `e.resources.iso`; all `exit=2`; all five files still `0` bytes | — |

ART-14 is one case with nine sub-cases; the slice total remains 13 cases (ART-01/02/03/04/14/15/16 + MED-01..06).

### Failures and deviations, in detail

**ART-14a — doubled error prefix.** Plan (`FUNCTIONAL_TEST_PLAN.md:1546-1547`) expected `acquisition failed: gzip: …`. Actual bytes: `acquisition failed: gzip: gzip: invalid header`. The `gzip:` token appears twice because the wrapper adds a `gzip:` prefix to an error whose text already begins `gzip: invalid header`. Everything else in (a) matched: admission passed, the GPT probe rejected the fake payload, `$TC/sha256/$TD` exists at mode `0444`, 1024 bytes, and `tiny.img` was never created. **Code defect (cosmetic, error-wrapping duplication)** in the probe error path.

**ART-14g/h/i — `done <step>` printed for steps that then fail.** Plan expectation was only about the message text, so the messages themselves pass. But the step reporter emits the completion line before the failure is known:
- (g) `done download` then `acquisition failed: "aarch64/IncusOS_202601010000.img.gz": asset failed size/digest admission; …`
- (h) `done download` then `acquisition failed: open aarch64/absent.img.gz: …` — the asset was never opened, so `done download` is false.
- (i) `done index` then `acquisition failed: open index.json: … no such file or directory` — `index.json` did not exist, so `done index` is false.
A human reading stderr sees a step reported complete and then the same step failing. **Code defect (UX / progress reporting).** Promise context: `internal/update/local.go:117,136` per plan `:1524`.

**ART-14h/i — doubled `open` verb and absolute path leak.** Actual: `acquisition failed: open aarch64/absent.img.gz: open /var/folders/gv/…/tiny/202601010000/aarch64/absent.img.gz: no such file or directory`. The plan's expectation `open aarch64/absent.img.gz: …no such file or directory` (`:1552`) used an ellipsis so it technically matches, but the message says `open` twice and exposes the fully-resolved local mirror path. Same shape for (i). **Code defect (cosmetic).**

**ART-02 — `published_at` precision.** Plan (`:1289`) said "an RFC 3339 `published_at`". Observed `2026-08-10T22:30:02.88333048Z` — 8-digit fractional seconds. RFC 3339 permits arbitrary fractional precision, so this is a **pass**, recorded only because a strict `YYYY-MM-DDTHH:MM:SSZ` consumer would break.

**ART-04 — stderr duplication under `--json`.** Both the JSON envelope on stdout and a plain `acquisition failed: cache directory is required` on stderr. Verified against `docs/docs/reference/automation.md:188` — "stderr | Error reprint after every failure (including `--json`)". Documented, so **pass**, not a deviation.

No case in this slice failed outright, and no case was blocked.

### Live index contents observed today (for Wave 2 reuse)

`https://images.linuxcontainers.org/os/index.json`, fetched 2026-08-16. Saved verbatim at `$WORK/versions-default.json`:

```json
{"result":{"versions":[{"version":"202608102114","channels":["testing","stable"],"published_at":"2026-08-10T22:30:02.88333048Z","architectures":["aarch64"]},{"version":"202608072311","channels":["testing","stable"],"published_at":"2026-08-08T00:30:03.00378509Z","architectures":["aarch64"]},{"version":"202608021451","channels":["testing","stable"],"published_at":"2026-08-04T18:04:19.35641104Z","architectures":["aarch64"]}]}}
```

- **Releases (3, ascending):** `202608021451`, `202608072311`, `202608102114`. This is exactly the `available:` list ART-16 printed — cross-check confirmed, sorted ascending and de-duplicated.
- **Channels:** every release carries both `testing` and `stable` (`[.result.versions[].channels[]]|unique` → `stable,testing`). `--channel testing` returns all three; `--channel nightly` returns an empty list at exit 0.
- **Architectures:** `aarch64` and `x86_64` for all three (`--architecture ''` → `aarch64,x86_64` for every entry; `--architecture x86_64` → `x86_64` ×3).
- **Types:** `raw` and `iso` for each release (six rows in the default human table).
- **Default architecture on this arm64 host resolves to `aarch64`** — confirmed: the unfiltered `versions` run emitted only `aarch64` rows and only `["aarch64"]` in JSON, while `--architecture ''` emitted both.

### New findings not already in the known list

- **N-ART-1** — `acquisition failed: gzip: gzip: invalid header`: the `gzip:` prefix is applied twice when the probe rejects a non-gzip payload.
- **N-ART-2** — the step reporter prints `done index` / `done download` for steps that immediately fail (ART-14 g, h, i). The completion line is emitted before the outcome is known, so stderr asserts a step succeeded and then reports it failing.
- **N-ART-3** — local-mirror open errors double the verb and leak the absolute resolved path: `open aarch64/absent.img.gz: open /abs/path/…: no such file or directory`.
- **N-ART-4** — `versions` never writes to `--cache-dir`. `$WORK/c1` (ART-15) and `$CACHE` (ART-01/02/03/16) were still non-existent or 0 bytes after every metadata-only run, so the index is not cached across invocations; each `versions` call refetches. Not a stated promise, but relevant to Wave 2 timing assumptions and to `cache.md`.
- **N-ART-5** — the `versions` human table is not column-aligned: the header `Version  Channel  Architecture  Type` is joined with two literal spaces, and 12-character version values push every column out of alignment with the header. It reads as delimited text, not a table, despite `render.go` being described as a table renderer.

All already-known findings relevant to this slice were confirmed in passing: F-CLI-7 (`--progress never` still emitted `==> index` / `done index`) reproduced on every single command in this slice.

### Repo cleanliness

```
$ git -C /Users/josh/code/componere/incusos-builder status --porcelain   # before
(no output)
$ git -C /Users/josh/code/componere/incusos-builder status --porcelain   # after
(no output)
```

Identical and empty (re-verified after the interruption). All work was done under `$WORK` = `/var/folders/gv/kllfch6d5l9dq4676hq4260m0000gn/T/tmp.NC4sWQqmks`; nothing was written inside the repository.

### No image asset downloaded

`du -sh $WORK/cache` → `0B`; `du -sh $WORK` → `108K` for the whole scratch tree. The only bytes ever admitted to any cache were the 1 KiB synthetic `/dev/zero` payload in the throwaway `$WORK/cache-tiny` (ART-14a), which is removed at the start of every `run()`. No case entered a download of a real asset; nothing was aborted for misclassification.

### Time spent / cases blocked and why

~6 minutes wall for all 13 cases; the slowest single command was ART-02 at 2.4 s (four live `index.json` fetches). No case was blocked, skipped, or exceeded its 5-minute budget. No TTY/tmux work was required — every case in this slice is non-interactive, and `CI=true` has no bearing on any of them since none reaches a prompt (MED-06's overwrite refusal is the `--force` pre-check, not the interactive prompt path).

## Slice report: W1SupDoc (SUP + DOC + BOOT-01)

## Slice: supply chain reads, documentation, boot venue — 23 cases

### Results

| Case | Result | Observed evidence (verbatim, one line) | Deviation |
|------|--------|----------------------------------------|-----------|
| SUP-01 | pass | `image-local  Build the apko image for the host arch and load it into Docker as incusos-builder:dev` + `GitVersion:    v3.1.1` | none — 9 pins resolve, 1 task row |
| SUP-02 | pass | `29.4.0 OrbStack aarch64` (exit=0) | none — Docker reachable, Wave 2 track B is **open** |
| SUP-11 | pass | `Ran 6 tests in 0.020s` / `OK`; guard: `error: GITHUB_REPOSITORY must be set` exit=1 | none |
| SUP-13 | deviation | `{"files":[".release-please-manifest.json","CHANGELOG.md","apko.yaml","melange.yaml"],...,"title":"chore(master): release 0.1.2"}` | `gh pr checks` shows **7** contexts, not the 4 the plan names |
| SUP-14 | deviation | vars: `COMPONERE_RELEASE_APP_CLIENT_ID	Iv23liJNXKWTgfZwQ3Kn`; secrets: `COMPONERE_RELEASE_APP_PRIVATE_KEY`; `gh api .../rulesets` → `[]` | credentials present; **tag protection absent** — F-REPO-2 confirmed |
| SUP-15 | pass | `Planned changes:` … 7 bullets … `Unsupported or manual follow-ups:` … `exit=0` | none — F-REPO-3 confirmed exactly |
| SUP-16 | fail | `gh api /repos/componere/incusos-builder/private-vulnerability-reporting` → `{"enabled":false}` | F-REPO-1 confirmed — SECURITY.md's channel is dead |
| SUP-17 | pass | `Error: HTTP 404: Not Found (https://api.github.com/repos/componere/incusos-builder/attestations/sha256:2d7116…)` exit=1 | none — the 404 is the expected pass |
| SUP-18 | pass | `git tag` → (empty); `gh release list` → (empty); packages → `[]`; `docker pull …:v0.1.1` → `Error response from daemon: manifest unknown` exit=1 | none |
| DOC-02 | pass | `ghd install …` → `no eligible stable release found for componere/incusos-builder/incusos-builder on darwin/arm64` exit=1 | none — `darwin_arm64` pattern + `path = "incusos-builder"` present in `ghd.toml` |
| DOC-03 | pass | `docker pull ghcr.io/componere/incusos-builder:v0.0.0` → `Error response from daemon: manifest unknown` exit=1 | none |
| DOC-05 | pass | `--server string      update server URL or local mirror directory (default "https://images.linuxcontainers.org/os")`; `HTTP/1.1 200 OK` | none |
| DOC-06 | pass | `INFO    -  Documentation built in 0.28 seconds` / `exit=0`; `docs/build/index.html` 23679 bytes | none — no MkDocs `WARNING` lines |
| DOC-07 | deviation | all 15 nav pages `200 OK` with expected titles, `edit_link=True` on every one, `search_index.json` 200 with 169 docs | site is served at **`/incusos-builder/`**, not `/`; the plan's `http://127.0.0.1:8000/` returns `302 Found` → `Location: /incusos-builder/` |
| DOC-08 | pass | `diff` produced no output, `diff_exit=0`; 15 files / 15 nav entries | none |
| DOC-09 | deviation | `root:check` deps = `root:format root:lint root:build root:test root:check-upstream docs:build`; all 11 CONTRIBUTING commands resolve; `root:e2e` not a dep | CONTRIBUTING's "`root:check` is the full local gate" is **false** in a TECH_NOTES checkout (F-GATE-1) |
| DOC-11 | pass | steps 3+4 both `configuration valid` exit=0; cleared key → `decryption failed: Error getting data key: 0 successful groups required, got 0` exit=4 | F-CFG-3 **confirmed** (see below) |
| DOC-13 | deviation | `{"result":{"valid":true,"type":"iso","architecture":"x86_64","offline":false}}` byte-identical to `:89`; all 6 exit codes match `exit.go` | `validate --server http://…` returns `configuration valid` exit **0**, contradicting §4's unconditional "A plain `http://` URL is exit `2`" — **new finding** |
| DOC-14 (Wave 1 half) | pass | `usage error: --server "http://example.invalid/os": plain http is not supported; use https or a local mirror directory` exit=2 | none; populated-mirror build deferred to Wave 2 |
| DOC-16 | deviation | step-1 config → `configuration valid` exit=0; `losetup: MISSING lsblk: MISSING blockdev: MISSING remote-viewer: MISSING` | `sha256sum` and `incus` are **present** on this host, not MISSING as the plan expects |
| DOC-17 | deviation | `go_run_exit=1` w/ stray `exit status 3` vs `binary_exit=3`; eleven seed keys emitted in order | F-DOC-2 **refuted for macOS 26** (see below) |
| DOC-18 | pass | all 11 mapping rows resolve at HEAD (e.g. `trust-model.md:36-40` → "a successful build is not evidence that a host will install or recover") | none |
| BOOT-01 | pass (decision recorded) | `arm64` / `/opt/homebrew/bin/incus` / `ls: cannot access '/dev/kvm': No such file or directory` | `incus` present (Homebrew **client**), plan expected MISSING — verdict unchanged |

---

### Failures and deviations, in detail

**SUP-16 — fail. Code/config defect (repository state).**
Plan expects `{"enabled":true}` at `SECURITY.md:11`. Actual API response, verbatim:
```
{"enabled":false}
```
The unauthenticated advisory form redirects to login (`200 https://github.com/login?return_to=…/security/advisories/new`), and `gh api /repos/componere/incusos-builder/security-advisories` returns `[]`. Promise: `SECURITY.md:11` — "Report vulnerabilities privately with [GitHub private vulnerability reporting](…/security/advisories/new)". That link is not usable by an outside reporter. Release blocker for the stated security promise. Fixed by SUP-15's `apply` (never run here).

**SUP-13 — deviation. Plan defect.**
Plan: "`gh pr checks` reports the four contexts from `.github/repository-settings.toml:82-87`". Observed seven rows:
```
Container Image Dry Run	pass	14s
Deploy GitHub Pages	skipping	0
Binary Release Dry Run	pass	6m34s
GitHub Pages	pass	14s
Melange Build Dry Run (amd64)	pass	1m45s
Melange Build Dry Run (arm64)	pass	1m7s
ci	pass	1m40s
```
All four required contexts are present and green; three extra contexts (`Deploy GitHub Pages` skipping, two `Melange Build Dry Run` matrix legs) are additive. The plan's expectation is under-specified, not the repo. Everything else in SUP-13 is exact: four changed files, `0.1.1 → 0.1.2` in the manifest, `melange.yaml version:`, and `apko.yaml org.opencontainers.image.version`, all three carrying the `# x-release-please-version` marker; the changelog section contains only `### Features` entries (no `docs`/`chore`).

**SUP-14 — deviation. Repository-state defect, not a code defect.**
`gh api /repos/componere/incusos-builder/rulesets` verbatim:
```
[]
```
The `Default tags` ruleset does not exist, so `v*` tag creation is unprotected and `release-please.yml:1-5`'s "protected `v*` tags" premise is not yet true. Credentials themselves are fine.

**DOC-07 — deviation. Plan/doc defect (harmless but reproducible).**
`docs/mkdocs.yml:3` sets `site_url: https://componere.github.io/incusos-builder/`, so `mkdocs serve` mounts the site under that path. Verbatim:
```
HTTP/1.0 302 Found
Location: /incusos-builder/
```
Following the plan's instruction literally (`browse http://127.0.0.1:8000/`) yields a redirect and, if not followed, 404s on every nav path. Re-swept at `http://127.0.0.1:8000/incusos-builder/`: **15/15 pages returned 200**, each `<title>` matched its front-matter `title`, each page carried an `edit/master/docs/docs/<path>` link, the search index served 200 with 169 documents, and the light/dark palette toggle markup is present (6 `data-md-color-scheme`/`__palette` occurrences on the index). No 404 and no unstyled page. `docs/moon.yml:52` — the promise itself is correct; the plan's URL is not.

**DOC-09 — deviation. Doc defect, folded with F-GATE-1.**
`moon query tasks` confirms `root:check` declares exactly the six claimed deps, `root:e2e` is not among them, and every command in `CONTRIBUTING.md:45-55` resolves to a real task (`root:format`, `root:lint`, `root:build`, `root:test`, `root:check-upstream`, `root:mocks`, `docs:build`, `docs:serve`). But `CONTRIBUTING.md:57` — "`moon run root:check` is the full local gate" — is **false** in a checkout that follows TECH_NOTES: per the lead's F-GATE-1, `root:format`/`root:lint` walk the gitignored `reference/incus-os/` clone and `.wt/` worktrees, so `root:check` cannot pass, while scoped `golangci-lint fmt --diff ./cmd/... ./internal/...` is clean. A new contributor following CONTRIBUTING verbatim hits a red gate on a clean tree. `root:check` was not re-run here, per instruction.

**DOC-13 — deviation. NEW doc/code defect.**
`run-in-ci.md` §4 states unconditionally: "`--server` must be an `https://` URL or an existing directory. A plain `http://` URL is exit `2`." Observed:
```
$ validate -f seed.yaml --server http://example.invalid/os --color never
configuration valid
exit=0
```
versus
```
$ versions --server http://example.invalid/os --color never
usage error: --server "http://example.invalid/os": plain http is not supported; use https or a local mirror directory
exit=2
$ build -f seed.yaml -o out.iso --server http://example.invalid/os --color never
usage error: --server "http://example.invalid/os": plain http is not supported; use https or a local mirror directory
exit=2
```
`validate` accepts a plain-HTTP `--server` silently. This is consistent with `use-local-mirror.md:73` ("`--server` applies to `build` and `versions`") but directly contradicts `run-in-ci.md` §4, which is the page a CI author reads. Either a code defect (validation not applied where the flag is accepted) or a doc defect (§4 must scope the rule). Everything else in DOC-13's Wave 1 portion is exact: success envelope byte-identical to `:89`, exactly one stdout line, `--json` + `-o -` → `usage error: --json cannot be combined with -o -` exit 2, `--verbose` + `-q` → `usage error: --verbose and -q are mutually exclusive` exit 2, `INCUSOS_BUILDER_JSON=true` produces the envelope and `--json=false` beats it, `-f -` works, error text appears on both stdout (envelope) and stderr (reprint) with stdout carrying only the JSON. Exit table `:163-172` cross-checked row-for-row against `internal/cli/exit.go:14-30` — all six codes and their sentinels match exactly. `build` snippets and the container run deferred to Wave 2.

**DOC-16 — deviation. Host-environment divergence from the plan, not a doc defect.**
Guide's step-1 config validates: `configuration valid` exit=0. Tool sweep verbatim:
```
cp: /bin/cp
losetup: MISSING
lsblk: MISSING
blockdev: MISSING
sha256sum: sha256sum
incus: /opt/homebrew/bin/incus
remote-viewer: MISSING
```
The plan expected `sha256sum` and `incus` MISSING; both resolve here. `cp --reflink=auto` → `cp: illegal option -- -` (BSD cp, as F-DOC-7 predicts). All five read-through assertions confirmed: boot priorities installer `30` (`:128`) > target `20` (`:130`) > dummy root `10` (`:122`); vTPM added at `:125` with the explicit "Add the software TPM before the first start; Incus cannot hot-plug a TPM into a VM" (`:139-140`); "`security.secureboot=false` … The VM still uses UEFI. This is not legacy BIOS." (`:135-137`); "Attach the copy to the same VM." (`:215`); and "Official docs do not publish a stable success string; do not match an invented log line." (`:241-243`). Guide is GNU/Linux-only by construction and says so — F-DOC-7 stands as consistent, not a defect.

**BOOT-01 — decision recorded. Checklist cannot run on this host.**
```
arm64
/opt/homebrew/bin/incus
ls: cannot access '/dev/kvm': No such file or directory
```
`verify-boot-acceptance.md:22-24` requires "An `x86_64` Linux Incus host with `/dev/kvm`, a managed storage pool, and a managed network." This host is `arm64` Darwin with no `/dev/kvm`; the `incus` present is a Homebrew **client** binary, not a Linux Incus server. **I state plainly: BOOT-02..BOOT-09 cannot execute on this host.** Verdicts on the three documented options:
1. **Remote `x86_64` Linux host with Incus** — **viable and required for a real pass.** The only documented path. Needs nested-virt/KVM, pool `default`, network `incusbr0`, `sudo` for `losetup`, and a SPICE client (`remote-viewer` is MISSING locally and would be needed there).
2. **QEMU (local or GitHub-hosted, as in the Phase 5.2 probe)** — **known-insufficient.** `phase-5-boot-probe.md:25-37` already recorded that topology as negative (Secure Boot enrolled, seed consumption not observed). On arm64 with no KVM it is strictly worse.
3. **Defer to post-tag** — **explicit risk acceptance, not a pass.** The checklist is meant to run before every release tag; deferring means tagging with the product's core end-to-end claim unobserved, and must be written into the release record with a named owner.

---

### Confirm / refute verdicts

**Repository findings**

| Finding | Verdict | Actual API response, quoted |
|---|---|---|
| F-REPO-1 private vulnerability reporting disabled | **CONFIRMED** | `GET /repos/componere/incusos-builder/private-vulnerability-reporting` → `{"enabled":false}` |
| F-REPO-2 rulesets empty | **CONFIRMED** | `GET /repos/componere/incusos-builder/rulesets` → `[]` |
| F-REPO-3 seven unapplied settings | **CONFIRMED, exactly seven** | `Update general repository settings` / `Update immutable releases` / `Update private vulnerability reporting` / `Update automated security fixes` / `Create GitHub Pages site` / `Create managed branch ruleset 'Default branch'` / `Create managed tag ruleset 'Default tags'` — followed by 9 `Unsupported or manual follow-ups:` entries, `exit=0` |
| F-REPO-4 release PR proposes 0.1.2 + stray `## Changelog` | **CONFIRMED on both counts** | PR list → `[{"number":10,"title":"chore(master): release 0.1.2"}]`; `git show FETCH_HEAD:CHANGELOG.md \| tail -3` → `* phase 4 CLI surface and output publisher (…)` / `` / `## Changelog` |

`plan` mode only. **No `apply` was run.** Nothing was mutated on GitHub; the only write was a local `git fetch` of the release-please branch into `FETCH_HEAD` (does not touch the working tree).

**Documentation findings (DOC-17 regression sweep)**

| Finding | Verdict | Literal side-by-side |
|---|---|---|
| F-DOC-1 `go run` swallows the exit code | **CONFIRMED** | `go run` → `invalid config: image.type: must be iso or raw` + stray `exit status 3` line, `go_run_exit=1`. Binary → `invalid config: image.type: must be iso or raw`, `binary_exit=3`. Same message, different rc, extra line. |
| F-DOC-2 `sha256sum` absent on stock macOS | **REFUTED for macOS 26; still stands for macOS ≤ 15** | `/usr/bin/sha256sum` → `No such file or directory` (so the finding's literal evidence column is correct), **but** `command -v sha256sum` → `sha256sum` resolving to `/sbin/sha256sum`, `sha256sum (Darwin) 1.0`. Provenance: `-rwxr-xr-x 5 root wheel 136576 Mar 19 22:25`, hard-linked to the `/sbin/{md5,sha1,sha224,sha256,sha384,sha512}[sum]` family (two shared inodes `…828`/`…830`), and `codesign -dv` → `Identifier=com.apple.sha224sum`, `Platform identifier=26`. **Verdict: this is an Apple-signed base-system binary, not a user install — `sha256sum` exists on a clean macOS 26 install.** The finding's premise ("does not exist on stock macOS") is false as of macOS 26 and should be re-scoped to "absent on macOS 15 and earlier"; the guides' `shasum -a 256` advice becomes unnecessary-but-harmless there. |
| F-DOC-3 fabricated unknown-field message | **CONFIRMED** | `run-in-ci.md:102` promises `{"error":{"code":3,"message":"invalid config: field seeds.install"}}`. Actual: `{"error":{"code":3,"message":"invalid config: seeds.bogus: unknown to incus-os v0.0.0-20260815030500-0f5b8057f2fc; a newer incusos-builder may accept this"}}`, exit 3. No code path emits `invalid config: field <path>`. |
| F-DOC-4 same refusal quoted two ways | **CONFIRMED** | `run-in-ci.md:49` → `refusing to overwrite out.img; re-run with --force`. `build-offline-media.md:198` → `usage error: refusing to overwrite /absolute/path/seeded.img; re-run with --force`. Live: `usage error: refusing to overwrite out.img; re-run with --force`, exit 2. **Plan defect note:** DOC-17 describes F-DOC-4 as being about "the `--server` error"; `run-in-ci.md:49` is the *overwrite* refusal. The §5 finding table (`:2867`) states it correctly. The underlying defect is real either way; only the DOC-17 prose mislabels it. |
| F-DOC-5 TTY prompts instead of refusing | **CONFIRMED** | `first-seeded-iso.md:157-158` says only "`build` will not replace `seeded.iso` if that path already exists. Re-run with `--force`, or choose a new `-o` path." At a real PTY (tmux socket `-L w1supdoc`, `CI` unset) the tool printed `overwrite existing output? [y/N] ` and waited. Answering `n` produced `usage error: refusing to overwrite out.img; re-run with --force`, `EXIT=2`. The prompt is undocumented in the tutorial. |
| F-DOC-8 tripwire | **GREEN** | `--version` → `incus-os API: v0.0.0-20260815030500-0f5b8057f2fc`; `go.mod:14` → `github.com/lxc/incus-os/incus-osd v0.0.0-20260815030500-0f5b8057f2fc`. Identical. |
| F-CFG-3 (touched by DOC-11) | **CONFIRMED** | `sops-encryption.md:36-38` claims "An empty `SOPS_AGE_KEY_FILE` makes SOPS open path `""` instead of using `SOPS_AGE_KEY`." Observed: `env SOPS_AGE_KEY_FILE= incusos-builder validate -f config.enc.yaml --color never` → `configuration valid`, `exit=0`. The claim is wrong. |

DOC-11 additionally confirmed: `config.enc.yaml` carries 26 `ENC[…]` values and exactly one top-level `sops:` key; validation from a path and from stdin both `configuration valid` exit 0; a plaintext file with a bogus `sops:` key → `decryption failed: parsing time "" as "2006-01-02T15:04:05Z07:00": cannot parse "" as "2006"` exit 4; and the only files left on disk were `config.enc.yaml`, `config.yaml`, `key.txt`, `plain-bogus.yaml` — **no decrypted bytes reached disk**. Step 5 (`build -f config.enc.yaml`) deferred to Wave 2 as instructed.

`init --no-input -o -` emitted all eleven commented seed keys in the documented order: `applications, incus, install, migration-manager, network, operations-center, provider, services, update, kernel, security` — matching `seed-injection.md:59-62` ("The nine web-customizer sections … `kernel.yaml` and `security.yaml` follow").

---

### New findings not already in the known list

1. **F-DOC-9 (new, doc or code) — `validate` silently accepts a plain-HTTP `--server`.** `run-in-ci.md` §4 states the `http://` → exit 2 rule unconditionally, but `validate --server http://example.invalid/os` returns `configuration valid` at exit 0 while `versions` and `build` correctly return `usage error: --server "http://example.invalid/os": plain http is not supported; use https or a local mirror directory` at exit 2. Either scope the doc sentence or apply the check where the flag is accepted.
2. **F-DOC-10 (new, doc) — the docs site is not served at `/`.** `mkdocs.yml:3` `site_url` forces the dev server to mount at `/incusos-builder/`; `http://127.0.0.1:8000/` returns `302 Found → /incusos-builder/`. `docs/moon.yml:52` and any instruction saying "browse `http://127.0.0.1:8000/`" should say so, or `mkdocs serve` should be passed a flag that neutralizes the base path.
3. **F-DOC-2 must be re-scoped (see table above).** `/sbin/sha256sum` is an Apple-signed base-system binary on macOS 26; the finding as written ("does not exist on stock macOS") is false for current macOS.
4. **Plan defect — DOC-17's F-DOC-4 prose mislabels the message.** DOC-17 (`:2647-2649`) attributes the missing `usage error: ` prefix to "the `--server` error"; `run-in-ci.md:49` is the overwrite refusal. The §5 table at `:2867` is correct.
5. **Plan defect — `awk` finds nine fenced YAML blocks in `configuration.md`, not eight.** Using the prescribed re-derivation at 59c268b: `34-38`, `791-794`, `800-808`, `815-823`, `830-847`, `853-861`, `867-878`, `885-1034`, `1040-1047`. The lead's list omits the block at `34-38`. (Recorded for the CFG slice; not load-bearing for mine.)
6. **`incus` is present on this host** (`/opt/homebrew/bin/incus`, Homebrew client). BOOT-01 and DOC-16 both assume `incus MISSING`. The verdicts are unaffected — a macOS client is not an `x86_64` Linux Incus host — but the plan's expectation string is wrong.
7. **Docker is reachable** (`29.4.0 OrbStack aarch64`). Wave 2 track B (SUP-03..SUP-10) is **not** blocked on this host.

---

### Repo cleanliness

```
$ git -C /Users/josh/code/componere/incusos-builder status --porcelain   # BEFORE
                                                                          # (no output)
$ git -C /Users/josh/code/componere/incusos-builder rev-parse HEAD
59c268b1499bd1a5ce94d487f1bc3adf377bead3

$ git -C /Users/josh/code/componere/incusos-builder status --porcelain   # AFTER
                                                                          # (no output)
$ git -C /Users/josh/code/componere/incusos-builder rev-parse HEAD
59c268b1499bd1a5ce94d487f1bc3adf377bead3
```
Both empty and identical. `docs/build/` (written by DOC-06) is gitignored. All scratch work lived in `/tmp/w1supdoc/{sops,ci,mir}`.

**Side-effect ledger:** docs-serve process **stopped** — `hub op:"stop" name:"docsite"` returned `Stopped docsite: exited exit=0 uptime=29.7s restarts=0`. tmux session killed on my private socket only — `tmux -L w1supdoc kill-session -t fdoc5` → `no server running on /private/tmp/tmux-501/w1supdoc` (session had already terminated); no other socket touched. No settings applied (`plan` only). No container built — `docker images | grep -i incusos` → `no incusos images`; both `docker pull` attempts failed with `manifest unknown`. No image downloaded; no `build` run reached the network (every build invocation terminated at a usage error, a mirror error, or the overwrite prompt).

---

### Time spent / cases blocked

Approximately 32 minutes of wall-clock tool time across 23 cases; no case exceeded 5 minutes (longest: DOC-07 at ~90 s including server start, sweep, and shutdown). **Zero cases blocked.** The one PTY-dependent case (DOC-17's F-DOC-5 probe) ran successfully on private tmux socket `-L w1supdoc` — note the shared `wait-for-text.sh` helper hard-codes `-L claude-agent` and therefore timed out against a private socket; I polled `capture-pane` directly instead. Wave 2 deferrals honored: DOC-11 step 5, DOC-13 `build` snippets + container run, DOC-14 populated-mirror build. Not run, per instruction: SUP-03..SUP-10, SUP-12, SUP-19..23, DOC-01/04/10/12/15, BOOT-02..BOOT-10, and `root:check`.
