---
title: Wave 2 track A execution results
plan: .journal/005/FUNCTIONAL_TEST_PLAN.md
commit tested: 59c268b
executed: 2026-08-16
operator: Main + five functional-tester agents (AChain, AMirror, APublish, AMedia, ADocs)
---

# Wave 2, track A — live build chain, artifacts, rescue media, doc walkthroughs

## Verdict

**31 of 31 cases executed. 25 pass, 5 deviations, 1 fail, 0 blocked.**

The artifact contract holds completely. The seed tar lands at byte
2,148,532,224 with exactly eleven `0600` entries in `writeSeed` order; everything
outside the tar is byte-identical to the fetched image; every digest the tool
reports matches a second hash of the published file; the integrity gates refuse
tampered bytes and tampered metadata with the documented wording and leave no
cache residue; and rescue media is structurally exactly what IncusOS recovery
looks for on both raw and ISO.

The one failure is a real code defect found by executing the plan literally:
**the low-disk warning never fires on a first-use cache directory** — precisely
the case where a user is most likely to be on a too-small volume.

| Block | Cases | pass | deviation | fail |
|-------|-------|------|-----------|------|
| PRE-07-W2 gate (`root:e2e`) | 1 | 1 | 0 | 0 |
| AChain — ART-05..10 | 6 | 6 | 0 | 0 |
| AMirror — ART-12, 13, 19, 20 | 4 | 3 | 0 | 1 |
| APublish — ART-11, 17, 18, 21 | 4 | 4 | 0 | 0 |
| AMedia — MED-07..17 | 11 | 11 | 0 | 0 |
| ADocs — DOC-04, 10, 12, 14, 15 | 5 | 1 | 4 | 0 |
| **Total** | **31** | **25** | **5** | **1** |

`root:e2e` passed in 63 s (`ok github.com/componere/incusos-builder/internal/cli 58.777s`).

## Reference values produced

Reused across the whole track and reproduced independently by three agents:

| Value | Observed |
|-------|----------|
| Selected asset | `aarch64/IncusOS_202608102114.img.gz`, declared `435456672` B, digest `4ac7328b…fefb75` (smallest `image-raw`) |
| `seed_bytes` | `13312` (eleven-section config) |
| Spliced image | `sha256 3e1fbc204b7b015e2b074f53a79080151d4a49d629a380b6bc407deca961847c`, `3432026112` B |
| `.gz` stored digest | `eef7b2c0ac71289c675a7b7fbade8252a5b3da296d82f587de5e028c2c31bc8c`, `452121275` B |
| seed tar digest | `95ae3c08490cdcfe9e947f19cf562a8fdb4bca5db5e4c1f1e08d2d31f21ca6df` |

APublish reproduced the image digest three separate ways — a fresh-cache env-var
build, a post-interrupt re-run, and the `-o -` stdout stream — all
`3e1fbc20…1847c`. ADocs reproduced it a fourth time from a local mirror.

## The failure

### ART-20 — the low-space warning is silently skipped on a first-use cache directory

Plan expects `==> warning: cache free space below asset size` **before** the
download. On a 128 MiB RAM disk with 127,984 KiB free against a 435,456,672 B
asset, the literal plan sequence produced **no warning at all**:

```
==> acquire
==> download
done download
acquisition failed: write cache temp: write /Volumes/CACHESMALL/cache/.fetch-4166684964: no space left on device
exit=5
```

Re-running the *identical* command — the only difference being that run 1 had by
then created the cache directory — produced the warning correctly:

```
==> acquire
==> warning: cache free space below asset size
done warning: cache free space below asset size
==> download
```

Root cause, read from source: `warnIfLowSpace` (`internal/update/cache.go:220-232`)
calls `freeBytes(c.dir)` and does `if err != nil { return }`. On a first-use cache
the `statfs` hits a directory that does not exist yet, returns ENOENT, and the
warning is swallowed. It is invoked at `cache.go:91` and `client.go:127`, both
**before** the directory is created (`cache.go:42-51` documents "dir is created on
first admission").

Impact: the first build against any new cache location gets no advance warning
and discovers the problem as a mid-download ENOSPC. Every other ART-20
expectation held once the warning fired — both lines literal, before the
download, not treated as an error, run continues, exit 5 with
`no space left on device`, no output published.

## New findings — code and product

| ID | Severity | Finding |
|----|----------|---------|
| **N-AMIR-1** | medium | The low-disk warning never fires on a first-use cache directory (see above). |
| **N-MEDIA-1** | medium | **Raw rescue media is not byte-reproducible.** Five builds of one identical offline config produced the same installer digest every time and **five different** `resources_sha256` values. Cause: a random GPT disk GUID plus FAT32 volume serial. Nothing in the docs says the rescue artifact is non-deterministic, and `result.resources_sha256` reads like a reproducible property. |
| **N-MEDIA-2** | advisory | **Mounting rescue media on macOS mutates it.** macOS wrote `.fseventsd/fseventsd-uuid` into the FAT volume at mount, changing its digest. Any verification procedure must mount read-only (`hdiutil attach -readonly`) or expect the digest to change; never compare a post-mount digest to the envelope value. |
| **N-MEDIA-3** | cosmetic | The ISO volume identifier is **NUL**-padded, not space-padded as ISO 9660 §8.4.4 specifies. Emitted by go-diskfs. `hdiutil` and `bsdtar` both resolve the label anyway; the residual risk is a strict Linux `blkid`/`udev` label match, which BOOT-07 gates. |
| **N-DOC-F** | medium (UX) | The README quickstart **dirties a clean clone**: `config.yaml` and a 3.4 GB `incusos.iso` are written into the checkout root and neither is in `.gitignore`. Verified: `git status --porcelain` showed `?? config.yaml` and `?? incusos.iso`. |

## New findings — documentation

| ID | Finding |
|----|---------|
| **N-DOC-A** | `README.md:65` says "pass `--resources-output`" for rescue media. Offline builds write rescue media with **no flag**; the flag only overrides the path, and passing it on a non-offline config is exit 2. |
| **N-DOC-B** | `build-offline-media.md:198` quotes a one-path refusal; the real offline refusal names **both** finals, comma-separated, image first. Breaks exact-match assertions. |
| **N-DOC-C** | `use-local-mirror.md:183` omits the `acquisition failed: ` prefix on the empty-`--cache-dir` error. Same class as F-DOC-4. |
| **N-DOC-D** | `build-offline-media.md:215-219` presents leftover `.bak` files as a usable previous generation. Publish step 6 deletes them; four successful `--force` runs left none. `recover-interrupted-build.md:52` says this correctly; the offline guide never does. |
| **N-DOC-E** | The `--force` `.bak` window is not externally observable on a warm cache — publish steps 2→6 finish faster than a `pgrep`+`kill -9` round-trip. `recover-interrupted-build.md` §4's restore path is real but essentially unreachable by interrupting a cache-warm build; the realistic post-kill state is row 1 (temps, no `.bak`). |
| **N-MED-14** | MED-14's rejection comes from the JSON parser (`acquisition failed: parse update.sjson: EOF`), not the `multipart/signed` validator the plan names. The case's real promise — refused before publishing, nothing partial at either path — holds exactly. |

## Deviations recorded on passing cases

- **ART-05** wall time **17.0 s**, not the plan's 1–3 min. **ART-10** recompression
  **2.88 s**, not 1–2 min. **MED-07** 5.3 s and **MED-11** 17.4 s against a warm
  cache, not 10–20 min each. The plan's cost model is anchored on cold-CI numbers
  and is roughly an order of magnitude pessimistic on this hardware.
- **ART-06** — the plan says a cache hit shows `==> acquire` "followed directly by"
  `==> probe` with "no progress lines". Observed: `done acquire` sits between them,
  and one `progress 100% (35672/35672)` line survives because the **index** is
  fetched on every run regardless of cache state. The cache-hit proof itself holds:
  no `==> download`, all 59 asset progress lines gone, 4.17 s vs 17.0 s.
- **DOC-04** — the README quickstart produces `seed_bytes  1024`: an empty seed tar,
  because the generated starter config has every `seeds` section commented out. The
  README makes no claim either way, but a first-run user gets a "seeded" image with
  no seed and nothing says so.
- **DOC-15** — three real `SIGKILL` attempts all landed in **row 1** of the decision
  table (`recover-interrupted-build.md:112-119`): temps present, both finals
  unchanged, **no `.bak`**. The restore path was validated against a synthesized
  row-3 state, labelled as synthesized.
- **DOC-12** — the exit-6 "output appeared during the build" race
  (`build-offline-media.md:216-221`) was **not** exercised; it needs a filesystem
  race. Reported untested, not assumed passing.

## Confirmations

- **F-DOC-1 reproduced on the README itself** (DOC-04): documented exit 2 arrives
  as exit 1 through `go run`, plus a spurious `exit status 2` stderr line. Since
  `README.md:7` says no release exists, `go run` is the default first experience.
- **F-DOC-5 confirmed at a real PTY** (DOC-10) with raw bytes captured, including
  the trailing space: `overwrite existing output? [y/N] `. Both `n` (exit 2, file
  untouched) and `y` (exit 0, file replaced) branches exercised.
- **F-DOC-2 revised again**: `sha256sum` works **verbatim** on this host for every
  doc that uses it, and agrees digit-for-digit with `shasum -a 256`.
- **N-ART-2 (the `done <step>` before failure)** re-observed. **F-CLI-7** re-confirmed
  on every ART/MED command.
- The three-way offline metadata binding works exactly as documented: tampering the
  filename inside `update.sjson` and the version inside `update.json` each produce
  the documented `untrusted metadata; possible tampering` refusal at exit 5, with
  neither artifact published.

## Track A plan corrections

| ID | Correction |
|----|-----------|
| D-13 | Cost estimates are ~10× pessimistic on this hardware (ART-05 17 s, ART-10 2.9 s, MED-07 5.3 s, MED-11 17.4 s, DOC-04 ~4 min against the plan's 20–45 min). |
| D-14 | ART-06's "no progress lines / directly followed by" wording is wrong; the index progress line always survives and `done acquire` intervenes. |
| D-15 | MED-13 expects a space-padded ISO volume identifier; go-diskfs emits NUL padding. |
| D-16 | MED-14 expects the `multipart/signed` validator's message; a zero-byte file dies earlier at `parse update.sjson: EOF`. |
| D-17 | ART-12's "turn off Wi-Fi" instruction is unusable on a shared machine; `sandbox-exec -f deny-net.sb` with a verified control is the better isolation proof. |
| D-18 | DOC-15 should say the realistic post-kill state is row 1 and that the `.bak` window is effectively unobservable on a warm cache. |

## Side effects

Shared cache `/tmp/wave2a-cache` grew to 5 blobs / 1.7 GB (the aarch64 raw image,
the x86_64 ISO pulled by DOC-04, and `aarch64/incus.raw.gz` pulled by DOC-12).
Peak scratch across all agents was ~25 GB; every agent removed its own large
artifacts. The RAM disk from ART-20 was ejected. All `hdiutil` attachments were
detached. DOC-04's two untracked repo-root artifacts were deleted. Every agent
verified `git status --porcelain` empty before and after, at HEAD `59c268b`.

---

# Appendix — verbatim slice reports

## Slice report: AChain (ART-05..ART-10)

All six cases executed. Report below.

## Slice: Wave 2 track A stage 1 — live producer chain — 6 cases

**Selected asset (re-derived today, not assumed):** `VER=202608102114` `ARCH=aarch64` `DIGEST=4ac7328bbac7e2445048294c83de23a913dd6696f1c2c291494e486e65fefb75`, file `aarch64/IncusOS_202608102114.img.gz`, index-declared size `435456672`.

**Headline values every sibling needs:**
- **`$SEED = 13312`**
- **image `sha256` = `3e1fbc204b7b015e2b074f53a79080151d4a49d629a380b6bc407deca961847c`**, size `3432026112`
- `.gz` compressed digest = `eef7b2c0ac71289c675a7b7fbade8252a5b3da296d82f587de5e028c2c31bc8c`, size `452121275`
- `seed.tar` digest = `95ae3c08490cdcfe9e947f19cf562a8fdb4bca5db5e4c1f1e08d2d31f21ca6df`

### Results

| Case | Result | Observed evidence (verbatim, one line) | Deviation |
|---|---|---|---|
| ART-05 | **PASS** | `exit=0` … `stat` → `3432026112 /tmp/wave2a-AChain/seeded.img`; summary `seed_bytes  13312` / `sha256  3e1fbc204b7b015e2b074f53a79080151d4a49d629a380b6bc407deca961847c` | Wall **17.0 s**, not the plan's 1–3 min; stderr carries an undocumented `progress 100% (35672/35672)` line inside the `index` step |
| ART-06 | **PASS** | `.result.sha256` = `shasum seeded2.img` = `shasum seeded.img` = `3e1fbc…847c`; no `==> download`; 4.17 s vs 17.0 s | `==> acquire` is followed by `done acquire` then `==> probe` (plan says "directly"); one `progress 100% (35672/35672)` index line survives despite the plan's "no progress lines" |
| ART-07 | **PASS** | `-r--r--r-- 435456672 /tmp/wave2a-cache/sha256/4ac7328b…fb75`; self-hash `4ac7328b…fb75  …/4ac7328b…fb75`; `.fetch-*` count `0` | none |
| ART-08 | **PASS** | `offset=2148532224 lba512=4196352 mib=2049 blocks=26`; `13312 seed.tar`; eleven `-rw-------  0 0` lines; order exact | none |
| ART-09 | **PASS** | `prefix_exit=0`, `END=2148545536`, `suffix_exit=0`, both silent; stock region `7784ef4f…97a7` ≠ seed.tar `95ae3c08…a6df` | none |
| ART-10 | **PASS** | `eef7b2c0…bc8c` (json) = `eef7b2c0…bc8c  seeded.img.gz`; `gzip_test_exit=0`; `3432026112`; decompressed `3e1fbc20…847c` = `seeded.img` | Recompression took **2.88 s** (`user 7.706s`, parallel), not the plan's 1–2 min |

### Failures and deviations, in detail

**No failures.** Six passes. Deviations, all timing- or reporter-shaped, none affecting artifact correctness:

**D-A1 — the plan's `sed` extraction is sound; no hand-fix was needed.** The pipeline
```
sed -n '113,262p' internal/cli/e2e_helpers_test.go | sed '1s/.*fmt\.Sprintf(`//' | sed "s/architecture: %s/architecture: $ARCH/; s/release: %q/release: \"$VER\"/"
```
produced a 150-line `live11.yaml` beginning `version: 1` / `image:` / `  type: raw` / `  architecture: aarch64` / `  channel: stable` / `  release: "202608102114"` and ending on the `-----END CERTIFICATE-----` of `security.custom_ca_certs`. `grep -c '^  [a-z-]*:$'` returns `11` — the eleven-section shape is intact. `validate -f live11.yaml` printed `configuration valid`, `exit=0`. **This part of the plan is not defective.**

**D-A2 — ART-05 ordered stderr step lines (verbatim, `grep -E '^(==>|done )'`):**
```
==> resolve
==> index
done index
done resolve
==> acquire
==> download
progress 0% (3831/435456672) … progress 100% (435456672/435456672)
done download
done acquire
==> probe
done probe
==> seed
done seed
==> splice
done splice
```
Exactly the plan's expected order. 60 `progress` lines total, the last download one `progress 100% (435456672/435456672)` immediately preceding `done download`. Full stdout summary block:
```
summary
output  /tmp/wave2a-AChain/seeded.img
type  raw
architecture  aarch64
version  202608102114
channel  stable
seed_bytes  13312
sha256  3e1fbc204b7b015e2b074f53a79080151d4a49d629a380b6bc407deca961847c
```
Rows and order match `cli.md:111-115`; two-space separated; **no `resources_*` rows** — correct for an online config.

**D-A3 — ART-06 warm-cache stderr (verbatim, complete):**
```
==> resolve
==> index
progress 100% (35672/35672)
done index
done resolve
==> acquire
done acquire
==> probe
done probe
==> seed
done seed
==> splice
done splice
```
The cache-hit proof holds: `==> download` is absent and all 59 download progress lines are gone. Two textual mismatches against the plan's wording: (a) `done acquire` sits between `==> acquire` and `==> probe`, so "followed directly by" is loose rather than literal; (b) "no progress lines" is false — the index fetch still emits `progress 100% (35672/35672)` because the index itself is downloaded on every run regardless of cache state. Envelope is one newline-terminated line, keys in order `output,type,architecture,version,channel,seed_bytes,sha256`, zero `resources*` keys.

**D-A4 — every wall-time estimate in this slice is an order of magnitude high** on this machine/link. ART-05 17.0 s (plan 1–3 min); ART-09 `gunzip` of 415 MiB → 3.2 GiB in 2.15 s (plan 1–2 min); ART-10 recompression 2.88 s (plan 1–2 min). Same family as the existing **D-4** finding about the retry ladder. The plan's Wave-2 cost model is calibrated for a much slower host.

### New findings not already in the known list

- **N-ART-6 — the `index` step emits a progress line even when the asset cache is warm.** `progress 100% (35672/35672)` appears in every build's stderr (ART-05, ART-06) whenever `--progress always` is set. The plan's ART-06 expectation "no progress lines" is therefore unsatisfiable as written; the correct assertion is "no *download* progress lines". Cosmetic, but it will false-fail any automated check that greps for `^progress`.
- **F-CLI-7 reconfirmed under `--progress never`** (ART-10): the run still printed the full `==> resolve … done splice` header sequence to stderr with zero progress lines. Consistent with the known finding; nothing new beyond the confirmation.
- **Golden cross-check is stronger than the plan implies.** `internal/seed/testdata/nine-section.golden.tar` lists nine entries whose names, modes (`-rw-------`), uid/gid (`0 0`) and **exact byte sizes** (71, 226, 152, 152, 259, 450, 82, 1226, 222) are identical to the first nine entries of the live `seed.tar`; the live tar adds `kernel.yaml` (67) and `security.yaml` (134). The shared prefix being byte-size-identical is independent evidence that `writeSeed` serialization did not drift.

### Artifacts and measurements handed to sibling agents

All under `/tmp/wave2a-AChain/`, retained:

| Path | Size | Purpose |
|---|---|---|
| `live11.yaml` | 3236 | Validated eleven-section online raw config, `aarch64` / `202608102114`. Reuse verbatim. |
| `seeded.img` | 3432026112 | The ART-05 primary image, `sha256 3e1fbc20…847c`. |
| `seeded.img.gz` | 452121275 | ART-10 compressed output, `sha256 eef7b2c0…bc8c`. |
| `seed.tar` | 13312 | Extracted seed region, `sha256 95ae3c08…a6df`. |
| `index.json` | 35672 | The index snapshot the selection was derived from. |
| `art05.stderr` / `art05.stdout` / `art06.stderr` / `artgz.stderr` / `build2.json` / `buildgz.json` | small | Raw captures backing every quote above. |

**Shared cache `/tmp/wave2a-cache` is warm and untouched since:** one entry, `sha256/4ac7328bbac7e2445048294c83de23a913dd6696f1c2c291494e486e65fefb75`, `-r--r--r--`, `435456672` B. `find` shows exactly `/tmp/wave2a-cache`, `/tmp/wave2a-cache/sha256`, and that one file — nothing else. Both directories are `drwxr-xr-x`.

Constants for downstream cases: `$SEED=13312`, `blocks=26` at `bs=512`, `skip=4196352`, `END=2148545536`.

### Side-effect ledger

- Created `/tmp/wave2a-AChain/` (my scratch dir only).
- Created `/tmp/wave2a-cache/sha256/4ac7328b…fb75` — the intended warm-cache side effect, first and only download of the 415 MiB asset.
- Wrote then **deleted** `seeded2.img` (3.2 GiB, ART-06) and `stock.img` (3.2 GiB, ART-09) to reclaim disk, after their digests/`cmp` results were captured.
- Network: one `curl` of `index.json`, plus the builder's own index+asset fetches (4 builds × index, 1 × asset).
- Did not touch Docker, did not run `root:e2e`, did not run any track B or C case, did not delete or modify the shared cache.
- Free space `/tmp`: 235 Gi before → **231 Gi** after.

### Repo cleanliness

`git status --porcelain` — **empty before, empty after** (both runs exit 0, no output). `HEAD` unchanged at `59c268b1499bd1a5ce94d487f1bc3adf377bead3` at both checkpoints. No tracked file was read-modified; the repository was used read-only (`e2e_helpers_test.go` extraction, `nine-section.golden.tar` listing, `bin/incusos-builder` execution).

### Time spent / cases blocked and why

**Zero cases blocked.** Total slice wall time ≈ 1 min 20 s of command execution. Per-case measured: ART-05 17.04 s · ART-06 4.17 s build + 12.0 s of verification hashing · ART-07 0.81 s · ART-08 0.09 s · ART-09 2.15 s decompress + 2.06 s `cmp`/hash · ART-10 2.88 s build + 16.5 s of decompress-and-hash verification. Nothing approached the 30-minute ceiling.

## Slice report: AMirror (ART-12, 13, 19, 20)

## Slice: Local mirror, tamper detection, offline binding, low disk — 4 cases

### Results

| Case | Result | Observed evidence (verbatim, one line) | Deviation |
|---|---|---|---|
| ART-12 | **PASS** | `{"result":{"output":"mirror-seeded.img","type":"raw","architecture":"aarch64","version":"202608102114","channel":"stable","seed_bytes":13312,"sha256":"3e1fbc204b7b015e2b074f53a79080151d4a49d629a380b6bc407deca961847c"}}` at `exit=0`; cache entry `-r--r--r-- 444 435456672 cache-mirror/sha256/4ac7328b…fefb75` | Isolation proven with `sandbox-exec -f deny-net.sb` instead of `networksetup -setairportpower en0 off` (directed substitution) |
| ART-13 | **PASS** | `acquisition failed: "aarch64/IncusOS_202608102114.img.gz": asset failed size/digest admission; untrusted metadata; possible tampering` at `exit=5`; `$TCACHE/sha256` present and empty; `ls: cannot access 'tamper.img': No such file or directory` | none |
| ART-19 | **PASS** | baseline `exit=0` publishing `"sha256":"41e94d547af58bbf94da0fdc05e21258fb4ebbff49e4304085f29890d0cc0b0a","resources_sha256":"7f3efc300996fc19cd32a151682271af039f91b0b09f50fd55571e39ea215906"`; tamper 1 `acquisition failed: update.sjson missing selected file "aarch64/debug.raw.gz" sha256 "5f9a568a0156db6f3bd8262bf1570ccc1a7270df366fb92f182f36cfdf92d6e1"; untrusted metadata; possible tampering`; tamper 2 `acquisition failed: update.json version "209901010000" != "202608102114"; untrusted metadata; possible tampering` | none |
| ART-20 | **FAIL as written / PASS after root-cause isolation** | Plan's literal sequence: **no warning lines at all**, only `==> acquire` → `==> download` → `acquisition failed: write cache temp: write /Volumes/CACHESMALL/cache/.fetch-4166684964: no space left on device`, `exit=5`. Re-run with the cache dir already existing: `==> warning: cache free space below asset size` / `done warning: cache free space below asset size` | **Yes — new finding N-AMIR-1.** The documented warning is silently skipped on a first-use cache directory |

---

### Failures and deviations, in detail

#### ART-20 — the documented warning does not fire on a fresh cache directory

Plan's exact command sequence, on a 128 MiB RAM disk with 127984 KiB free vs a 435456672 B asset:

```
$ hdiutil attach -nomount ram://262144      → /dev/disk5
$ diskutil erasevolume HFS+ CACHESMALL /dev/disk5
  Initialized /dev/rdisk5 as a 128 MB case-insensitive HFS Plus volume
$ df -k /Volumes/CACHESMALL
  /dev/disk5      131072 3088    127984     3%  … /Volumes/CACHESMALL
$ "$IOB" build -f live11.yaml -o small.img --cache-dir /Volumes/CACHESMALL/cache --color never --progress never
==> resolve
==> index
done index
done resolve
==> acquire
==> download
done download
acquisition failed: write cache temp: write /Volumes/CACHESMALL/cache/.fetch-4166684964: no space left on device
exit=5
```

Neither `==> warning: cache free space below asset size` nor `done warning: cache free space below asset size` appears. The exit code and ENOSPC cause match the plan; the warning assertion fails.

Root cause isolated by re-running the *identical* command a second time, the only difference being that run 1 had by then created `/Volumes/CACHESMALL/cache`:

```
$ ls -ld /Volumes/CACHESMALL/cache
drwx------ 3 josh staff 102 Aug 16 13:27 /Volumes/CACHESMALL/cache
$ "$IOB" build -f live11.yaml -o small.img --cache-dir /Volumes/CACHESMALL/cache --color never --progress never
==> resolve
==> index
done index
done resolve
==> acquire
==> warning: cache free space below asset size
done warning: cache free space below asset size
==> download
done download
acquisition failed: write cache temp: write /Volumes/CACHESMALL/cache/.fetch-4195862009: no space left on device
exit=5
```

Mechanism, read from source: `internal/update/cache.go:42-51` documents "dir is created on first admission", and `warnIfLowSpace` (`cache.go:220-232`) does `free, err := c.freeBytes(c.dir)` then `if err != nil { return }`. On a first-use cache the `statfs` hits a directory that does not exist yet, returns ENOENT, and the warning is swallowed. `warnIfLowSpace` is invoked at `cache.go:91` and `client.go:127` — both before the directory is created.

Once the warning does fire, every other plan expectation holds: both lines are literal, they appear **before** `==> download`, the build continues past them (not treated as an error), and it then fails `exit=5` with `no space left on device`. No output file was published in either run.

Practical impact: the very first build against any new cache location — the common case, and precisely the case where a user is most likely on a too-small volume — gets no advance warning and instead discovers the problem as a mid-download ENOSPC.

#### ART-12 — network isolation substitution

Per instruction, host Wi-Fi was left up. Isolation used `sandbox-exec` with:

```
(version 1)
(allow default)
(deny network*)
```

Two controls, both run under that exact profile:

```
$ sandbox-exec -f deny-net.sb /usr/bin/curl -sS -o /dev/null https://images.linuxcontainers.org/os/index.json
curl: (6) Could not resolve host: images.linuxcontainers.org
curl_exit=6

$ sandbox-exec -f deny-net.sb "$IOB" versions --cache-dir /tmp/wave2a-cache --architecture aarch64 --color never
==> index
done index
acquisition failed: Get "https://images.linuxcontainers.org/os/index.json": dial tcp: lookup images.linuxcontainers.org: no such host
control2_exit=5
```

The second control is the strong one: the same binary, same sandbox, same invocation shape as the case under test, genuinely cannot reach the network. Every ART-12/13/19 build below ran inside that sandbox.

#### ART-12 — evidence

```
$ shasum -a 256 "$MIRROR/$VER/$ARCH/IncusOS_$VER.img.gz"
4ac7328bbac7e2445048294c83de23a913dd6696f1c2c291494e486e65fefb75  …/IncusOS_202608102114.img.gz
$ find "$MIRROR" | sort
/tmp/wave2a-AMirror/mirror-full
/tmp/wave2a-AMirror/mirror-full/202608102114
/tmp/wave2a-AMirror/mirror-full/202608102114/aarch64
/tmp/wave2a-AMirror/mirror-full/202608102114/aarch64/IncusOS_202608102114.img.gz
/tmp/wave2a-AMirror/mirror-full/index.json

$ sandbox-exec -f deny-net.sb "$IOB" versions --server "$MIRROR" --cache-dir "$MCACHE" --architecture aarch64 --color never
==> index
done index
Version  Channel  Architecture  Type
202608102114  stable  aarch64  raw
202608102114  stable  aarch64  iso
202608072311  stable  aarch64  raw
202608072311  stable  aarch64  iso
202608021451  stable  aarch64  raw
202608021451  stable  aarch64  iso
versions_exit=0

$ find cache-mirror | sort          # immediately after versions
cache-mirror                         ← still empty: N-ART-4 confirmed

$ sandbox-exec -f deny-net.sb "$IOB" build --json -f live11.yaml -o mirror-seeded.img --server "$MIRROR" --cache-dir "$MCACHE" --color never --progress always
exit=0   (real 0m3.660s)
stdout: {"result":{…"seed_bytes":13312,"sha256":"3e1fbc204b7b015e2b074f53a79080151d4a49d629a380b6bc407deca961847c"}}
stderr: ==> resolve / ==> index / progress 100% (35672/35672) / done index / done resolve /
        ==> acquire / ==> download / progress 0% (1048576/435456672) / progress 91% (396361728/435456672) /
        progress 100% (435456672/435456672) / done download / done acquire /
        ==> probe / done probe / ==> seed / done seed / ==> splice / done splice

$ ls -l mirror-seeded.img ; shasum -a 256 mirror-seeded.img
-rw-r--r-- 1 josh wheel 3432026112 … mirror-seeded.img
3e1fbc204b7b015e2b074f53a79080151d4a49d629a380b6bc407deca961847c  mirror-seeded.img
```

Digest and size are byte-identical to ART-05's reference (`3e1fbc20…1847c`, 3432026112 B), `seed_bytes` matches at 13312, and the mirror cache admitted the blob:

```
$ stat -f '%Sp %p %z %N' cache-mirror/sha256/*
-r--r--r-- 444 435456672 cache-mirror/sha256/4ac7328bbac7e2445048294c83de23a913dd6696f1c2c291494e486e65fefb75
```

Mode 0444 as promised.

#### ART-13 — evidence

```
$ printf '\xff' | dd of="$MIRROR/$VER/$ARCH/IncusOS_$VER.img.gz" bs=1 seek=1000000 count=1 conv=notrunc status=none
$ shasum -a 256 "$MIRROR/$VER/$ARCH/IncusOS_$VER.img.gz"
65afddf23d2e3513b1f26b3211702f6bf77e785e6699d533c945ebb02ef41ed0  …   ← differs from 4ac7328b…

$ rm -rf "$TCACHE" && mkdir -p "$TCACHE"          ← fresh cache, mandatory
$ sandbox-exec -f deny-net.sb "$IOB" build --json -f live11.yaml -o tamper.img --server "$MIRROR" --cache-dir "$TCACHE" --color never --progress never
exit=5
stdout: {"error":{"code":5,"message":"acquisition failed: \"aarch64/IncusOS_202608102114.img.gz\": asset failed size/digest admission; untrusted metadata; possible tampering"}}
stderr: ==> resolve / ==> index / done index / done resolve / ==> acquire / ==> download / done download /
        acquisition failed: "aarch64/IncusOS_202608102114.img.gz": asset failed size/digest admission; untrusted metadata; possible tampering
```

Wording is character-exact against the plan, and the JSON envelope carries the same message at `code: 5`.

Residue proof:

```
$ find "$TCACHE" | sort
/tmp/wave2a-AMirror/cache-tamper
/tmp/wave2a-AMirror/cache-tamper/sha256
$ ls -a "$TCACHE"        → .  ..  sha256
$ ls -a "$TCACHE/sha256" → .  ..                  ← empty, no .fetch-* anywhere
$ ls tamper.img
ls: cannot access 'tamper.img': No such file or directory
```

Exactly one attempt was made (single `==> download`), consistent with the note that the retry ladder is HTTPS-only. Good blob restored and re-verified at `4ac7328b…fefb75`.

#### ART-19 — evidence

Smallest `application` asset for `aarch64` confirmed from the live index as `aarch64/debug.raw.gz`, 5529210 B — the plan's choice is correct. Fetched into the mirror alongside `update.json` (11859 B) and `update.sjson` (14268 B); `shasum` of the asset `5f9a568a0156db6f3bd8262bf1570ccc1a7270df366fb92f182f36cfdf92d6e1`.

Baseline (offline, under the deny-net sandbox), `exit=0`:

```
==> resolve / ==> index / done index / done resolve / ==> acquire / done acquire /
==> probe / done probe / ==> seed / done seed / ==> splice / done splice /
==> rescue / ==> download / done download / ==> metadata / done metadata / done rescue
{"result":{"output":"off.img","resources_output":"off.res.img","type":"raw","architecture":"aarch64","version":"202608102114","channel":"stable","seed_bytes":3072,"sha256":"41e94d547af58bbf94da0fdc05e21258fb4ebbff49e4304085f29890d0cc0b0a","resources_sha256":"7f3efc300996fc19cd32a151682271af039f91b0b09f50fd55571e39ea215906"}}
```

Both digests independently re-verified with `shasum -a 256`:
```
41e94d547af58bbf94da0fdc05e21258fb4ebbff49e4304085f29890d0cc0b0a  off.img          (3432026112 B)
7f3efc300996fc19cd32a151682271af039f91b0b09f50fd55571e39ea215906  off.res.img      (275901440 B)
```

Tamper 1 (`update.sjson` cleartext filename rewritten to `debug-NOPE.raw.gz`; tamper confirmed by `cmp`: *"differ: char 2203, line 9"*), `exit=5`:
```
acquisition failed: update.sjson missing selected file "aarch64/debug.raw.gz" sha256 "5f9a568a0156db6f3bd8262bf1570ccc1a7270df366fb92f182f36cfdf92d6e1"; untrusted metadata; possible tampering
ls: cannot access 'off.img': No such file or directory
ls: cannot access 'off.res.img': No such file or directory
```

Tamper 2 (`update.json` `.version` rewritten; `jq -r '.version'` → `209901010000`; exactly one quoted occurrence in the file so the sed is unambiguous), `exit=5`:
```
acquisition failed: update.json version "209901010000" != "202608102114"; untrusted metadata; possible tampering
ls: cannot access 'off.img': No such file or directory
ls: cannot access 'off.res.img': No such file or directory
```

Both messages are character-exact against the plan. Neither artifact exists after either failure — and note the failure comes *after* the ~3.2 GB splice completes, so the binding check genuinely cannot be shortcut. Both good copies restored and re-verified with `cmp` (byte-identical).

---

### New findings not already in the known list

**N-AMIR-1 — the low-free-space warning never fires on a first-use cache directory (ART-20).** `assetCache.warnIfLowSpace` (`internal/update/cache.go:220-232`) calls `statfs` on `c.dir` and swallows any error, but the cache directory is only created on first admission (`cache.go:42-51`). On a brand-new `--cache-dir` the `statfs` returns ENOENT and the warning is dropped. Demonstrated by two identical invocations differing only in whether the directory pre-existed: run 1 emitted no warning, run 2 emitted both documented lines. This silently disables the warning in exactly the scenario it exists for — a user pointing a first build at a too-small volume.

**N-AMIR-2 (minor, observational) — `done` is printed for the low-space warning step.** `warnIfLowSpace` calls `reporter.Step` then immediately `reporter.Done`, producing `done warning: cache free space below asset size`. The plan documents this as expected, so it is not a defect against spec, but it is the same shape as N-ART-2 and reads oddly: a warning is not a step that completes.

**Confirmations of existing findings (not new):**
- **N-ART-2** confirmed three more times: `done index` then index fetch fails (sandbox control 2); `done download` then admission fails (ART-13); `done metadata` then metadata binding fails (ART-19 tamper 1 and 2).
- **N-ART-4** confirmed: `find cache-mirror` immediately after a successful `versions --server <dir>` shows the directory still empty; nothing was written.
- **N-ART-5** confirmed: the `versions` table columns are single-space separated and do not align under the header.
- **F-CLI-7** confirmed: `--progress never` still printed `==> resolve`, `==> index`, `==> acquire`, `==> download`, `==> rescue`, `==> metadata` headers in ART-13, ART-19 and ART-20.
- **N-ART-3** not triggered — no local-mirror open error arose in this slice.

---

### Side-effect ledger

| Item | State |
|---|---|
| `/tmp/wave2a-AMirror/` | my scratch dir; 421 MB retained (mirror-full: 435 MB img.gz + 5.5 MB debug.raw.gz + index/update metadata, plus captured stdout/stderr logs and `deny-net.sb`) |
| `mirror-seeded.img` (3.2 GB) | deleted after digest recorded |
| `off.img` (3.2 GB), `off.res.img` (276 MB) | deleted after digests recorded |
| `good.img.gz` (435 MB backup), `cache-mirror/` (435 MB), `cache-tamper/` | deleted after restore verified |
| `$MIRROR/.../IncusOS_$VER.img.gz` | tampered then restored; re-verified `4ac7328b…fefb75` |
| `$MIRROR/$VER/update.sjson` | tampered then restored; `cmp` byte-identical to `update.sjson.good` |
| `$MIRROR/$VER/update.json` | tampered then restored; `cmp` byte-identical to `update.json.good` |
| RAM disk `/dev/disk5` (CACHESMALL) | **ejected** — `Disk /Volumes/CACHESMALL ejected`, `eject_exit=0` |
| RAM disk `/dev/disk4` (dangling, from a first `hdiutil attach` whose output parsing failed) | **detached** — `"disk4" ejected.`, `detach4_exit=0` |
| `/Volumes/` after cleanup | `Macintosh HD` only; `hdiutil info` shows no ram:// images |
| `/tmp/wave2a-cache` | never written by me, never deleted; intact (3 entries) |
| `/tmp/wave2a-AChain/live11.yaml`, `seeded.img` | read only; both present and unmodified |
| Host Wi-Fi | never touched; `networksetup` never invoked |
| Network fetches | `index.json` (35 KB), `debug.raw.gz` (5.5 MB), `update.json`, `update.sjson`, plus ~128 MB aborted download in each of two ART-20 runs |
| Docker | not touched |
| tmux | not needed; no PTY case in this slice |

### Repo cleanliness

```
$ git status --porcelain          (before)   → empty
$ git rev-parse HEAD              (before)   → 59c268b1499bd1a5ce94d487f1bc3adf377bead3
$ git status --porcelain          (after)    → empty
$ git rev-parse HEAD              (after)    → 59c268b1499bd1a5ce94d487f1bc3adf377bead3
```

No tracked file was created, modified, or deleted. All work happened in `/tmp/wave2a-AMirror`.

### Time spent / cases blocked and why

Roughly 7 minutes wall for the whole slice (13:22 → 13:29). No case was blocked and none approached the 30-minute ceiling. Individual build timings: ART-12 mirror build 3.66 s; ART-13 rejection 0.27 s; ART-19 baseline 3.91 s, tamper 1 3.64 s, tamper 2 3.58 s; ART-20 5.00 s and 4.85 s. The dominant costs were the 435 MB mirror copy and the independent `shasum` verifications of the 3.2 GB outputs (~6–7 s each).

One extra run beyond the plan was performed deliberately: the second ART-20 invocation, to isolate N-AMIR-1 rather than merely report the warning as missing.

## Slice report: APublish (ART-11, 17, 18, 21)

## Slice: Wave 2 track A — cache precedence, interrupt, force/restore, stdout stream — 4 cases

All four cases executed end-to-end against the live server with the shared warm cache `/tmp/wave2a-cache`, config `/tmp/wave2a-AChain/live11.yaml` (read-only, unmodified), binary `bin/incusos-builder` at HEAD `59c268b`.

### Results

| Case | Result | Observed evidence (verbatim, one line) | Deviation |
|---|---|---|---|
| ART-11 | **PASS** | env run: `exit=0`, `grep -c '==> download' env.err` → `1`, `ls $CACHE2/sha256/` → `4ac7328bbac7e2445048294c83de23a913dd6696f1c2c291494e486e65fefb75`; flag run: `exit=0`, download count `0`, `ls: cannot access '/nonexistent-should-be-ignored': No such file or directory` | none |
| ART-17 | **PASS** | `exit=5` after `kill -INT`, stderr last line `acquisition failed: context canceled`, `$IDIR` empty (`total 0`), `no fetch temps`, cache entry `-r--r--r-- … 435456672`; re-run `sha256` = `3e1fbc20…1847c` | none (F-CLI-7 re-confirmed) |
| ART-18 | **PASS** | no-force: `exit=2`, `usage error: refusing to overwrite /tmp/wave2a-APublish/force/out.img; re-run with --force`; `--force`: `exit=0`, `no .bak and no .tmp present`; restore drill returns `3e1fbc20…1847c` matching `gen1.sha` | none |
| ART-21 | **PASS** | `exit=0`; `shasum -a 256 streamed.img` → `3e1fbc204b7b015e2b074f53a79080151d4a49d629a380b6bc407deca961847c`; `file` → `DOS/MBR boot sector; partition 1 : ID=0xee…`; `grep -c 'sha256' stream.err` → `0` | none |

---

### Detail per case

#### ART-11 — `--cache-dir` override and env precedence

Pre-state, recorded before anything ran:

```
$ ls -d "$HOME/Library/Caches/incusos-builder" 2>/dev/null; echo "default_dir_exit=$?"
default_dir_exit=2
```
The macOS default cache directory **did not exist** before the slice, and still does not exist after it (`ls: cannot access '/Users/josh/Library/Caches/incusos-builder': No such file or directory`). Nothing in this slice created it.

Env half (fresh `cache-env`, 17.2 s wall, one real ~415 MiB download):

```
real	0m17.245s
exit=0
--- grep -c '==> download' env.err:
1
--- ls $CACHE2/sha256/:
4ac7328bbac7e2445048294c83de23a913dd6696f1c2c291494e486e65fefb75
-r--r--r-- 1 josh wheel 435456672 …/cache-env/sha256/4ac7328b…fefb75
{"result":{"output":"seeded-env.img","type":"raw","architecture":"aarch64","version":"202608102114","channel":"stable","seed_bytes":13312,"sha256":"3e1fbc204b7b015e2b074f53a79080151d4a49d629a380b6bc407deca961847c"}}
```
On-disk verification of that image: `3e1fbc204b7b015e2b074f53a79080151d4a49d629a380b6bc407deca961847c  seeded-env.img` — equals ART-05's reference. The fresh cache entry landed at mode `0444` and exactly `435456672` B, matching the declared asset size.

Flag-beats-env half:

```
$ ls -d /nonexistent-should-be-ignored 2>&1 ; echo pre_exit=$?
ls: cannot access '/nonexistent-should-be-ignored': No such file or directory
pre_exit=2
exit=0
--- grep -c '==> download' flag.err:
0
--- ls -d /nonexistent-should-be-ignored:
ls: cannot access '/nonexistent-should-be-ignored': No such file or directory
post_exit=2
```
Full stderr of the flag run (no `download` phase at all):
```
==> resolve
==> index
progress 100% (35672/35672)
done index
done resolve
==> acquire
done acquire
==> probe
done probe
==> seed
done seed
==> splice
done splice
```
Flag beats env, env beats default, the bogus env path was never created. Both runs exit 0.

#### ART-17 — SIGINT during splice

The signal was delivered **deterministically**, never by hand timing: the build ran in background, a polling loop watched the stderr file for the literal `==> splice`, and `kill -INT` fired on the recorded PID.

```
bgpid=87371
iterations=80          # ~0.4 s of 5 ms polls; splice header had appeared
sent SIGINT to 87371
exit=5
```

Post-interrupt state, all six checks:

```
--- int.err tail:
done probe
==> seed
done seed
==> splice
acquisition failed: context canceled
--- last line verbatim (cat -A):
acquisition failed: context canceled$
--- int.out:            (empty — nothing on stdout)
--- ls -la $IDIR:
total 0
drwx------ 2 josh wheel  64 Aug 16 13:27 .
--- find "$IDIR" -name '.out.img-*.tmp':    (no output)
--- fetch temps in cache:
no fetch temps
--- cache entry:
-r--r--r-- 1 josh wheel 435456672 Aug 16 13:22 /tmp/wave2a-cache/sha256/4ac7328b…fefb75
```
No `out.img`, no `.out.img-*.tmp`, no `.fetch-*`, cache entry intact at mode 0444 and exactly 435456672 B.

Clean re-run to completion:
```
real	0m3.817s
exit=0
jq -r '.result.sha256'  → 3e1fbc204b7b015e2b074f53a79080151d4a49d629a380b6bc407deca961847c
shasum -a 256           → 3e1fbc204b7b015e2b074f53a79080151d4a49d629a380b6bc407deca961847c  …/out.img
-rw-r--r-- 1 josh wheel 3432026112 …/out.img
```
Digest equals ART-05's reference; size equals the reference `3432026112` B.

#### ART-18 — `--force` and the documented restore drill

Baseline build (`gen1.sha` = `3e1fbc20…1847c`, matching ART-05).

Refusal, with `--no-input`:
```
exit=2
--- stdout:  (empty)
--- stderr verbatim:
usage error: refusing to overwrite /tmp/wave2a-APublish/force/out.img; re-run with --force
```
The message names the full path, exit is 2, and the file was provably untouched — identical digest and identical `mtime size` before and after (`1786912080 3432026112` both times). No `==>` step headers appeared, so the refusal precedes the pipeline entirely.

`--force`:
```
exit=0
--- backups/tmps:
no .bak and no .tmp present
shasum → 3e1fbc204b7b015e2b074f53a79080151d4a49d629a380b6bc407deca961847c
```
`ls -la` after the forced run shows only `gen1.sha` and `out.img`; the temporary backup was created and removed within the run.

Drill, following `recover-interrupted-build.md` §2 → §3 → §4 with `shasum -a 256` substituted for `sha256sum` (F-DOC-2). Staged a hand-made `.bak` plus a 0-byte claim, then inventoried:
```
=== step 2 inventory ===
-rw------- 1 josh wheel 0 …/out.img
e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855  …/out.img
-rw-r--r-- 1 josh wheel 3432026112 …/out.img.incusos-builder.bak
3e1fbc204b7b015e2b074f53a79080151d4a49d629a380b6bc407deca961847c  …/out.img.incusos-builder.bak
=== temps ===
no temps
```
`e3b0c442…b855` is the empty-string digest, confirming the claim really is 0-byte. This matches decision-table row *"Image final present / Image `.bak` present"* with the §3 caveat that a 0-byte final is a claim, not media — the `elif [ ! -s ]` branch of §4 is the correct one.

Guard and restore block verbatim from the guide:
```
$ pgrep -fl incusos-builder || echo "no incusos-builder running"
no incusos-builder running
$ if [ -e "$IMAGE.incusos-builder.bak" ]; then if [ ! -e "$IMAGE" ]; then mv -- …; elif [ ! -s "$IMAGE" ]; then rm -- "$IMAGE"; mv -- "$IMAGE.incusos-builder.bak" "$IMAGE"; fi; fi
restore_block_exit=0
```
Verification per the guide's "Verification" section:
```
-rw-r--r--  1 josh wheel 3432026112 …/out.img
shasum → 3e1fbc204b7b015e2b074f53a79080151d4a49d629a380b6bc407deca961847c  …/out.img
gen1.sha 3e1fbc204b7b015e2b074f53a79080151d4a49d629a380b6bc407deca961847c  …/out.img
ls …/out.img.incusos-builder.bak → No such file or directory
```
Restored digest equals the pre-drill digest; the `.bak` is gone. Drill works as documented.

#### ART-21 — `build -o -`

```
real	0m3.765s
exit=0
-rw------- 1 josh wheel 3432026112 streamed.img
shasum → 3e1fbc204b7b015e2b074f53a79080151d4a49d629a380b6bc407deca961847c  streamed.img
```
Byte count is **exactly** the reference `3432026112` and the digest matches ART-05 — which alone proves no trailing JSON was appended.

```
--- file:
streamed.img: DOS/MBR boot sector; partition 1 : ID=0xee, start-CHS (0x0,0,2), end-CHS (0x3ff,255,63), startsector 1, 6703175 sectors, extended partition table (last)
```
A GPT-protective-MBR disk image, not text. Explicit purity checks: `strings` over the first and last 4 KiB matching `sha256|summary|seed_bytes|architecture|"result"|{` returned `none in head` / `none in tail`; the final 64 bytes are all `0x00`.

stderr was captured separately and contained only the step reporter — no summary, no digest:
```
==> resolve
==> index
done index
done resolve
==> acquire
done acquire
==> probe
done probe
==> seed
done seed
==> splice
done splice
```
`grep -c 'sha256' stream.err` → `0`.

---

### Failures and deviations, in detail

None. All four cases matched their §4.3 expectations, including exact error strings.

Confirmations of known findings encountered incidentally:
- **F-CLI-7 confirmed again.** Every `--progress never` run still printed the full `==> resolve / ==> index / … / ==> splice` header set on stderr (ART-17 re-run, both ART-18 builds, ART-21). Note this is what made ART-21's stderr non-empty; it does not contaminate stdout.
- **N-ART-2 not reproduced for `splice`.** The interrupted run printed `==> splice` and then the error with **no** `done splice`. So the "done before failure" defect does not extend to a context-cancelled splice; the reporter suppressed the completion line correctly here.

### New findings not already in the known list

1. **N-APUB-1 — a SIGINT during `splice` is reported as `acquisition failed`.** The run had already printed `done acquire` and `done seed` and was inside `==> splice` when the signal arrived, yet the sole error line is `acquisition failed: context canceled`. The exit code (5, "acquisition / version resolution") and the message both misattribute a splice-phase cancellation to acquisition. The test plan predicts this text, so it is expected-as-specified, but the phase label contradicts the reporter output immediately above it and would mislead an operator triaging a cancelled build. Severity: low (cosmetic/diagnostic).
2. **N-APUB-2 — the human `build` summary table uses the same two-space column separator implicated in N-ART-5.** Observed on stdout: `output  /tmp/…/out.img`, `seed_bytes  13312`, `sha256  3e1fbc…` under a bare `summary` heading, with no padding to align the value column. This is the `build` summary, not the `versions` table, so N-ART-5's misalignment is not confined to `versions`.
3. **Positive, worth recording:** the no-`--force` refusal is a genuine pre-flight — it emitted no step headers and completed before any network or splice work, so a wrong `-o` path costs nothing.

### Side-effect ledger

Created and deleted by me:
- `/tmp/wave2a-APublish/` (my scratch) — created, then removed entirely at the end. Confirmed gone.
- `/tmp/wave2a-APublish/cache-env/` — throwaway fresh cache for ART-11's env half, one ~415 MiB download, deleted.
- Four 3 432 026 112 B images written and deleted after their digests were recorded: `seeded-env.img`, `seeded-flag.img`, `interrupt/out.img`, `force/out.img`, plus `streamed.img`. Peak concurrent usage one image at a time; free space never dropped below 223 GiB.

Not touched:
- `/tmp/wave2a-cache` — **not deleted, not modified.** Still holds `4ac7328b…fefb75` at `-r--r--r--` / 435456672 B with its original `13:22` mtime, alongside two entries other agents added (`5f9a568a…`, `d949e6cd…`).
- `/tmp/wave2a-AChain/` — read `live11.yaml` only; directory listing at the end still shows all 13 files including `seeded.img` and `seeded.img.gz`.
- `$HOME/Library/Caches/incusos-builder` — absent before, absent after.
- `/nonexistent-should-be-ignored` — never created.
- Docker, track B/C artifacts — untouched.

One incidental misstep, contained and reported: my first interrupt attempt backgrounded the whole `cd && … &` chain, so `$!` and `$IDIR` never reached the foreground shell; the build ran to completion uninterrupted and a later `rm -rf "$IDIR"` expanded to `rm -rf ""`, which failed harmlessly (`mkdir: cannot create directory ''`). No file outside my scratch was at risk. I re-established the variables explicitly and re-ran the interrupt cleanly; the reported ART-17 result comes only from that clean run.

### Repo cleanliness

```
# before
$ git status --porcelain | cat
(no output)
$ git rev-parse HEAD
59c268b1499bd1a5ce94d487f1bc3adf377bead3

# after
$ git status --porcelain | wc -l
0
$ git rev-parse HEAD
59c268b1499bd1a5ce94d487f1bc3adf377bead3
```
Both empty. No tracked file was modified; every write went to `/tmp/wave2a-APublish`.

### Time spent / cases blocked and why

Roughly 12 minutes of wall clock for the slice; nothing blocked, nothing near the 30-minute cap.

| Case | Dominant cost |
|---|---|
| ART-11 | 17.2 s cold env build (real download) + 3.8 s warm flag build + 6.2 s verification hash |
| ART-17 | 0.6 s to interrupt + 3.8 s re-run + hashing |
| ART-18 | three warm builds ≈ 3.8 s each + four 3.4 GB hashes ≈ 6 s each |
| ART-21 | 3.8 s stream + 6 s hash |

One wasted ~76 s on the failed first interrupt attempt described above.

## Slice report: AMedia (MED-07..MED-17)

All eleven cases executed. Report follows.

## Slice: Rescue media, raw and ISO (MED-07 … MED-17) — 11 cases

### Results

| Case | Result | Observed evidence (verbatim, one line) | Deviation |
|---|---|---|---|
| MED-07 offline raw build, both digests | **PASS** | `{"result":{"output":"/tmp/wave2a-AMedia/seeded-off.img","resources_output":"/tmp/wave2a-AMedia/seeded-off.resources.img","type":"raw","architecture":"aarch64","version":"202608102114","channel":"stable","seed_bytes":3072,"sha256":"41e94d547af58bbf94da0fdc05e21258fb4ebbff49e4304085f29890d0cc0b0a","resources_sha256":"c392bbcd40a03010daee0d41516fb11b052d9f36092828ab687fb25d071f2f58"}}` — `exit=0` | none (build took 5.3 s, not 10–20 min: warm cache) |
| MED-08 GPT + `RESCUE_DATA` at 1 MiB | **PASS** | `ee` / `EFI PART` / `2048` / `536821` / `RESCUE_DATA` / `512` / `46 41 54 33 32 20 20 20`; `1: Microsoft Basic Data RESCUE_DATA 273.8 MB disk4s1` | none |
| MED-09 proportional-with-floor size | **PASS** | `275901440` and boot signature `55  aa` at partition start + 510 | none |
| MED-10 raw read-back, exact logical bytes | **PASS** | `11859 … update.json`, `14268 … update.sjson`, `5529210 … aarch64/debug.raw.gz`; both `cmp exit=0`; `hotfix check: 0` | `.fseventsd/fseventsd-uuid` appears in `find` — created by macOS at mount (mtime 13:25:34 = mount time), not builder-written |
| MED-11 offline ISO build | **PASS** | `{"result":{…"resources_output":"/tmp/wave2a-AMedia/seeded.resources.iso","type":"iso",…"sha256":"62c0379b4b359bbd39d8e24af0749806fae79119a7937162fc7e48e9b810665e","resources_sha256":"1841cb4f70e15b4784f232aa4f7964c731c4a7c55093a53d960637923e1f8586"}}` — `exit=0`, both `shasum` equal | none (17.4 s, not 10–20 min) |
| MED-12 Rock Ridge names + exact bytes | **PASS** | `-rw------- 1 501 20 5529210 Aug 16 13:26 update/aarch64/debug.raw.gz` (+ `update/update.json`, `update/update.sjson`, 3 dirs, no `hotfix`); `wc -c` = `11859`; `cmp exit=0`; digest `5f9a568a…d6e1` | none |
| MED-13 2048-aligned, PVD-truncated, labeled | **PASS w/ deviation** | `size=5605376 remainder=0`; `CD001`; `pvd_blocks=2737 pvd_bytes=5605376`; `hdiutil attach -nomount` → `/dev/disk4`, `attach exit=0` | volume identifier is `R E S C U E _ D A T A \0 \0 …` — **NUL** padding, plan (and ISO 9660) expect **space** padding |
| MED-14 bad rescue input refused | **PASS** | `acquisition failed: parse update.sjson: EOF` — `exit=5`; `ls: cannot access 'neg.img': No such file or directory` (same for `neg.resources.img`) | error text is `parse update.sjson: EOF`, i.e. the parser, not the `multipart/signed` validator the plan names |
| MED-15 payload provenance | **PASS** | `-r--r--r-- 1 josh wheel 5529210 … /tmp/wave2a-cache/sha256/5f9a568a0156db6f3bd8262bf1570ccc1a7270df366fb92f182f36cfdf92d6e1`; index: `application aarch64/debug.raw.gz 5529210 5f9a568a…d6e1` | none |
| MED-16 inode-replacement mode split | **PASS** | `3432026112 644 seeded-off.img` / `275901440 600 seeded-off.resources.img` | none |
| MED-17 `--force` pair replacement | **PASS** | `usage error: refusing to overwrite /tmp/wave2a-AMedia/seeded-off.img, /tmp/wave2a-AMedia/seeded-off.resources.img; re-run with --force` — `exit=2`; force run `exit=0`, envelope digests == `shasum`, no `*.incusos-builder.bak` left | none |

Setup: `$IOB=/Users/josh/code/componere/incusos-builder/bin/incusos-builder`, `$WORK=/tmp/wave2a-AMedia`, `$CACHE=/tmp/wave2a-cache`, `$V=202608102114`, `$ARCH=aarch64`. Config `offline.yaml` pins `release: "202608102114"`, `architecture: aarch64`, `offline: true`, one application `debug` (`aarch64/debug.raw.gz`, 5,529,210 B — chosen so the payload stays ≤ 255 MiB and MED-09's 256 MiB floor governs; `incus` at 272 MB would have broken the floor).

### Failures and deviations, in detail

**No case failed.** Four deviations from the plan's wording:

1. **MED-13 — volume identifier is NUL-padded, not space-padded.** Plan expects "`RESCUE_DATA` plus space padding". Observed at byte 32808:
   ```
   0000000    R   E   S   C   U   E   _   D   A   T   A  \0  \0  \0  \0  \0
   0000020   \0  \0  \0  \0  \0  \0  \0  \0  \0  \0  \0  \0  \0  \0  \0  \0
   ```
   ISO 9660 §8.4.4 specifies d-characters padded with `0x20`. The padding is emitted by go-diskfs, not by `internal/media/iso.go` (which only sizes and truncates — `truncateToPVD` at `iso.go:66-101`). Both readers exercised here resolve the label anyway: `hdiutil attach -nomount` succeeded and `bsdtar` read the tree. Cosmetic to macOS; a strict Linux `blkid`/`udev` label match is the residual risk, and that is exactly what BOOT-07 gates. Not a pass-blocker; recorded so the plan's expected string can be corrected.

2. **MED-14 — the rejection comes from the JSON parser, not the metadata validator.** The plan predicts a rejection by "the metadata validator … before the media writer's own guard" for a non-`multipart/signed` document. A zero-byte `update.sjson` never reaches that check; it dies earlier at `parse update.sjson: EOF`. The case's actual promise — *refused before publishing, nothing partial at either final path* — holds exactly: `exit=5`, both `neg.img` and `neg.resources.img` absent, no stray temporaries in `$WORK`. The plan's predicted error text should be relaxed, or the case should truncate to a valid-JSON-but-unsigned document to reach the validator.

3. **MED-10 — `find` returns a fourth path.** `find /Volumes/RESCUE_DATA -type f` lists `/Volumes/RESCUE_DATA/.fseventsd/fseventsd-uuid` alongside the three intended files. It is macOS's own FSEvents stub, written by the mount: its mtime is `2026-08-16 13:25:34`, i.e. the moment I mounted, while the builder's files carry `2026-08-16 20:24:58`. The builder wrote exactly three files. The plan's `find … | sort` command will show this on every macOS run and should say so.

4. **Cost estimates are far off with a warm cache.** MED-07 `real 0m5.277s` (plan: 10–20 min), MED-11 `real 0m17.398s` (plan: 10–20 min), MED-17 force run `real 0m4.503s` (plan: 5–15 min). Only the 5.5 MB application and the 436 MB ISO asset were fetched; the 435 MB raw image came from `/tmp/wave2a-cache`.

**Confirmed known findings.** N-ART-2 reproduced verbatim in MED-14: the reporter printed `done metadata` and then the run failed with `acquisition failed: parse update.sjson: EOF`. F-CLI-7 reproduced in MED-07 and MED-11: `--progress never --color never` still emitted `==> resolve`, `==> index`, `==> acquire`, `==> probe`, `==> seed`, `==> splice`, `==> rescue`, `==> metadata` on stderr.

### New findings not already in the known list

**N-MED-1 — raw rescue media is not byte-reproducible across builds.** Two builds from the identical config and identical cached payloads produced different rescue digests: MED-07 `c392bbcd40a03010daee0d41516fb11b052d9f36092828ab687fb25d071f2f58` vs MED-17's `rescue-data.img` `36af4056488fbf880d2059ef1c5eefd448c7b72ac941b775a7b4cdbb6c82a754`. Cause identified, not guessed: FAT32 `BS_VolID` differs (`dc f2 7e 3c` vs `cd bf bc ad`) and so does the GPT disk GUID (`cmp -l` reports differences beginning at byte 569, which is offset 56 of the GPT header = `DiskGUID`). Total differing bytes across the 275,901,440-byte images: **557** — all header/serial, no payload divergence. No document claims reproducible rescue media, so this is not a broken promise; it does mean an operator must not verify a rebuild by comparing against a previously recorded rescue digest, and `build-offline-media.md` §3 ("Record the published hashes") reads as if they could.

**N-MED-2 — ISO volume identifier NUL padding** (detail in deviation 1 above).

**N-MED-3 — mounting the raw rescue media read-write mutates it.** After `diskutil mount`, `seeded-off.resources.img` hashed `4877f1991cf1e9538122d5a8a2ee0e502da4ff3f6e66e6a026db9d78992c3b7d` instead of the envelope's `c392bbcd…`. This is macOS writing `.fseventsd`, not a builder defect, but MED-10's procedure silently invalidates the artifact MED-07 just certified and that DOC-12/DOC-15 are told to reuse. MED-10 should mount read-only (`diskutil mount -mountOptions rdonly`) or operate on a copy.

### Side-effect ledger

- Scratch dir `/tmp/wave2a-AMedia` created; nothing written anywhere else except the shared cache.
- Shared cache `/tmp/wave2a-cache/sha256/` gained two entries by normal builder operation, both mode `-r--r--r--`: `5f9a568a…d6e1` (5,529,210 B, application) and `d949e6cd…4243` (436,472,054 B, ISO image). Pre-existing `4ac7328b…fb75` untouched. Cache not deleted.
- `hdiutil` attachments: `/dev/disk4` attached twice (raw image for MED-08/10, ISO for MED-13). **Both detached** — `"disk4" ejected.` each time; final `hdiutil info` → `no attachments remain`.
- Volume `/Volumes/RESCUE_DATA` mounted once and unmounted with the detach. macOS wrote `.fseventsd/fseventsd-uuid` into it (N-MED-3).
- Deleted after recording digests: `badmirror/` (441 MB tampered mirror) and the throwaway fresh cache `/tmp/wave2a-AMedia-freshcache.6Omx` used for MED-14. No sibling scratch dir touched; `/tmp/wave2a-AChain/live11.yaml` and `seeded.img` read-only (in fact never opened — I built my own minimal configs, since MED-10 needs exactly one application).
- Retained for reuse, 6.4 GB total: `seeded-off.img`, `seeded-off.resources.img` (mount-mutated), `rescue-data.img` (pristine), `seeded.iso`, `seeded.resources.iso`, both envelopes, `src.update.json`, `src.update.sjson`, `index.json`, configs. Peak disk ~7.3 GB, under the 15 GB budget. Two `hub` messages sent to `Main` announcing the paths and the digest correction.

### Repo cleanliness

`git status --porcelain` before: empty. After: empty. `git rev-parse HEAD` = `59c268b1499bd1a5ce94d487f1bc3adf377bead3` before and after. No tracked file was read-modified or written; the only repo access was reading `internal/media/iso.go`, `docs/docs/how-to/build-offline-media.md`, the plan, and executing `bin/incusos-builder`.

### Time spent / cases blocked and why

Roughly 14 minutes wall for the whole slice; no case approached the 30-minute stop rule and **none blocked**. Measured build times: MED-07 `5.277s`, MED-11 `17.398s`, MED-14 `3.60s`, MED-17 refusal `13.28s` / force `4.503s`. The remaining cases are structural probes costing under a second each. The whole slice ran far under budget because `/tmp/wave2a-cache` supplied the 435 MB raw image and, after MED-11, the 436 MB ISO image.

## Slice report: ADocs (DOC-04, 10, 12, 14, 15)

All five cases executed. Report follows.

## Slice: Documentation walkthroughs (Wave 2 track A) — 5 cases

### Results

| Case | Result | Observed evidence (verbatim, one line) | Deviation |
|---|---|---|---|
| DOC-04 | **deviation** | `summary` / `output  incusos.iso` … `seed_bytes  1024` / `sha256  3eab1d8296c9003f4398c581c201cc9025e6697adedd7f701554a1993e103b0e` | README quickstart runs, but `go run` returns exit 1 for a tool exit 2 (F-DOC-1); `README.md:65` misstates `--resources-output`; seed is an empty 1024-byte tar |
| DOC-10 | **deviation** | `overwrite existing output? [y/N] ` captured at a real PTY, then `PTYy_exit=0` after `y` | **F-DOC-5 confirmed**: `first-seeded-iso.md:157-158` documents only refusal; a TTY prompts |
| DOC-12 | **deviation** | `{"result":{…"resources_output":"/tmp/wave2a-ADocs/doc12/seeded.resources.img"…"resources_sha256":"aa6e677129bb2a01242cdd36c7bcee9e44747c728ebf4fd0003884abb042129e"}}` (9 fields) | `:198` refusal quotes one path; the tool lists both, comma-separated |
| DOC-14 | **pass** | `"sha256":"3e1fbc204b7b015e2b074f53a79080151d4a49d629a380b6bc407deca961847c"` from `--server /tmp/wave2a-ADocs/doc14/mirror` — **equals ART-05** | `:183` omits the `acquisition failed: ` prefix |
| DOC-15 | **deviation** | post-`SIGKILL`: `.seeded.img-1040580127.tmp` (2391801856 B) + `.seeded.resources.img-14530683.tmp` (0 B), both finals unchanged, **no `.bak`** | Observed **row 1** of `:114`; the `.bak` window proved unreachable by a SIGKILL race |

---

### Documentation divergences, per guide

#### `README.md`

- **`README.md:60-62` — quickstart uses `go run`, which destroys exit codes (F-DOC-1, reproduced).**
  Same failure, two invocations:
  ```
  === go run rebuild (should refuse) ===
  usage error: refusing to overwrite incusos.iso; re-run with --force
  exit status 2
  GORUN_exit=1

  === binary rebuild (should refuse) ===
  usage error: refusing to overwrite incusos.iso; re-run with --force
  BIN_exit=2
  ```
  `go run` also injects an extra stderr line `exit status 2` that is not tool output. Every documented exit code (2/3/5/6) is unreachable through the three commands the README prints. The success path is unaffected (`exit=0` both ways) and the artifact is byte-identical: `3eab1d82…3b0e` from both `go run` and `./bin/incusos-builder`.

- **`README.md:65` — "Offline seed configs also produce rescue media; pass `--resources-output` for that path" is nearly right and misleading.** Rescue media is produced with *no* flag at all; `--resources-output` only *overrides* the default location. Observed with the flag absent:
  ```
  "resources_output":"/tmp/wave2a-ADocs/doc12/seeded.resources.img"
  ```
  A reader of `:65` reasonably concludes rescue media requires the flag. It does not — and passing it on a non-offline config is a hard usage error (exit 2).

- **`README.md:55-62` — no `--cache-dir`, no expectation set for a ~580 MiB first-run download.** The literal build downloads the `x86_64` ISO asset into `$HOME/Library/Caches/incusos-builder` with no size or time note. `first-seeded-iso.md:120-124` does explain this; the README does not.

- **`README.md:62` writes `incusos.iso` (3 433 074 688 B) into the user's checkout root.** Neither `config.yaml` nor `incusos.iso` is in `.gitignore`, so the documented quickstart leaves a clean clone with two untracked files, one of them 3.4 GB. Verified: `git status --porcelain` after the literal run showed `?? config.yaml` and `?? incusos.iso`.

- **Accurate as printed:** `wrote config.yaml`; `configuration valid`; the generated file carries `type: iso`, `architecture: x86_64`, `channel: stable`, `offline: false` with all eleven `seeds` sections commented; no `.resources.*` companion appears.

#### `docs/docs/tutorials/first-seeded-iso.md`

- **`:157-158` — F-DOC-5, confirmed at a real PTY.** The tutorial states flatly: "`build` will not replace `seeded.iso` if that path already exists. Re-run with `--force`, or choose a new `-o` path." At a genuine TTY (tmux `-L adocs`, `CI`/`NO_COLOR` unset) the tool instead prompts. Raw PTY bytes, note the **trailing space**:
  ```
  overwrite existing output? [y/N] usage error: refusing to overwrite seeded.iso; re-run with --force
  ```
  Answering `N`:
  ```
  overwrite existing output? [y/N] n
  usage error: refusing to overwrite /tmp/wave2a-ADocs/doc10/seeded.iso; re-run with --force
  PTY_exit=2
  ```
  Answering `y` — the branch the tutorial says does not exist:
  ```
  overwrite existing output? [y/N] y
  ==> resolve
  …
  sha256  f122c4f6c50c7785d27cc5fe8e03e84898b5ace5646c502137c62b85f0e6e8a0
  PTYy_exit=0
  ```
  The file was replaced, digest unchanged, and no `.bak` survived. The non-interactive path matches the tutorial exactly (`exit=2`, file unchanged) — so a reader in CI is fine and a reader at a terminal is told the wrong thing. `build-offline-media.md:186-192` documents the prompt correctly; the tutorial is the outlier.

- **`:30` — "In the commands below, `incusos-builder` means `go run ./cmd/incusos-builder`."** Inherits F-DOC-1 in full: `:114`'s exit `3` and `:76`'s exit `2` are both unobservable if the reader follows `:30` literally. I ran the tutorial with the absolute binary (recorded deviation) precisely so the documented codes could be checked; they are all correct *for the binary*.

- **Everything else in this tutorial is exact.** Verified line by line:
  - `:52-57` + `:59` — `incusos-builder dev (none) built unknown` / `incus-os API: v0.0.0-20260815030500-0f5b8057f2fc`, identical from `go run` and the binary. F-DOC-8's pin is still current.
  - `:70` — `wrote config.yaml`.
  - `:76` — `usage error: refusing to overwrite existing file config.yaml`, `exit=2`; `init --help | grep -i force` returns nothing.
  - `:83-93` — the printed YAML block, extracted byte-for-byte from the page, `diff`s clean against `internal/config/testdata/valid.yaml` (`diff_exit=0`).
  - `:104` — `configuration valid`.
  - `:114-116` — `invalid config: image.type: must be iso or raw`, `exit=3`.
  - `:129-140` — stdout genuinely starts with `summary` (I checked with `2>/dev/null`; the `==>` progress lines are stderr), labels in the documented order, `seed_bytes  2048` > 0, 64-hex `sha256`.
  - `:145-149` — `seeded.iso` exists, `ls seeded.resources.*` → `No such file or directory`.

#### `docs/docs/how-to/build-offline-media.md`

- **`:196-199` — the refusal quote is one path short.** Guide prints:
  ```
  usage error: refusing to overwrite /absolute/path/seeded.img; re-run with --force
  ```
  Observed on an offline build where both finals exist:
  ```
  usage error: refusing to overwrite /tmp/wave2a-ADocs/doc12/seeded.img, /tmp/wave2a-ADocs/doc12/seeded.resources.img; re-run with --force
  ```
  Both paths, comma-space separated. Distinct from F-DOC-4 (which is about the `usage error: ` prefix — that prefix *is* present here and the guide has it right). Exit 2, both files byte-unchanged, confirmed by digest.

- **`:215-219` — "`--force` moves each existing final aside to `<path>.incusos-builder.bak` … Leftover `.incusos-builder.bak` files are harmless; rename them back onto the final paths if you need the previous artifacts."** Reads as an offer of a rollback generation. It is not, in the normal case: publish step 6 best-effort deletes them, and across four successful `--force` runs **zero** `.bak` files survived. `recover-interrupted-build.md:52` states the deletion; this guide never does, so a reader here plans a recovery strategy on files that will not be there.

- **`:127` — `sha256sum --` works verbatim on this host.** F-DOC-2 (revised) holds: `/sbin/sha256sum` is present on macOS 26 and on `PATH`.
  ```
  efda64af4e7dfaf731b99ce0c14fd92cec970b77fda0c5cf7393b593f1dc68fc  /tmp/wave2a-ADocs/doc12/seeded.img
  aa6e677129bb2a01242cdd36c7bcee9e44747c728ebf4fd0003884abb042129e  /tmp/wave2a-ADocs/doc12/seeded.resources.img
  ```
  `shasum -a 256` produces identical digests. Both agree with the envelope.

- **`:13` — "`incusos-builder` on `PATH`" (F-DOC-6).** Unreachable pre-release; every command in this guide had to be re-pointed at an absolute binary path.

- **`:102-103` — `--json` with `-o -` emits *two* things, the guide quotes one.** Observed: the envelope on stdout **and** the plain message on stderr.
  ```
  {"error":{"code":2,"message":"usage error: --json cannot be combined with -o -"}}
  usage error: --json cannot be combined with -o -
  ```
  Correct behavior for a machine reader; the guide's single-line quote understates it.

- **Verified exact, no divergence:**
  - `:50-58` — `invalid config: seeds.applications: required when image.offline is true`, `exit=3`.
  - `:94-100` — `usage error: offline builds cannot use -o -`, `exit=2`.
  - `:105-109` — `usage error: --resources-output requires offline: true in the config`, `exit=2`.
  - `:90-91` — "The two paths must be distinct after cleaning. `-` is rejected": `usage error: image and resources paths must be distinct` and `usage error: resources path cannot be -`, both exit 2.
  - `:75-79` — the `<stem>.resources.<ext>` rule, including the trap that `<ext>` follows `image.type` and **not** the `-o` suffix. Proved with a deliberate mismatch: `-o trap.iso` on a `type: raw` config →
    ```
    "output":"/tmp/wave2a-ADocs/doc12/trap.iso","resources_output":"/tmp/wave2a-ADocs/doc12/trap.resources.img"
    ```
    and the `iso` half cross-referenced from AMedia's envelope (`seeded.iso` → `seeded.resources.iso`).
  - `:113-121` — the envelope carries exactly nine fields, all present.
  - `:125-129` — the human summary prints the same fields in the documented order (`output`, `resources_output`, `type`, `architecture`, `version`, `channel`, `seed_bytes`, `sha256`, `resources_sha256`); `-q` suppresses it (stdout empty, stderr progress unaffected).
  - `:156-166` — the rescue tree. Cross-referenced from AMedia (MED-10/MED-12), not re-mounted: raw media holds `update/update.json` (11859 B), `update/update.sjson` (14268 B), `update/aarch64/debug.raw.gz` (5529210 B); ISO media the same three under Rock Ridge; `hotfix*` count 0 on both. The per-arch prefix is preserved as documented.
  - `:216-221` — the exit-6 "output appeared during the build" case is a genuine race and was **not** exercised; reported untested.

#### `docs/docs/how-to/use-local-mirror.md`

- **`:183` — "An empty `--cache-dir` fails with `cache directory is required`."** Actual first line:
  ```
  acquisition failed: cache directory is required
  ```
  Exit 5. The `acquisition failed: ` prefix is missing from the doc — same class as F-DOC-4, and it defeats an exact-match grep in a CI assertion.

- **`:13` — "`incusos-builder` on `PATH`" (F-DOC-6).** Same as above.

- **Verified exact, no divergence:**
  - `:20-46` — I built the mirror from that layout literally (`mirror/index.json`, `mirror/202608102114/aarch64/IncusOS_202608102114.img.gz`, `mirror/202608102114/update.json`, `mirror/202608102114/update.sjson`), copying the blob out of `/tmp/wave2a-cache` after confirming `4ac7328bbac7e2445048294c83de23a913dd6696f1c2c291494e486e65fefb75`.
  - `:62` — "Relative paths are resolved to an absolute directory": `--server doc14/mirror` from a different cwd listed all six rows, exit 0.
  - `:64-70` — `versions` against the directory:
    ```
    Version  Channel  Architecture  Type
    202608102114  stable  aarch64  raw
    202608102114  stable  aarch64  iso
    …
    ```
  - `:72-78` — `build --json … --server <dir>` (the case this slice existed to close):
    ```
    {"result":{"output":"/tmp/wave2a-ADocs/doc14/seeded.img","type":"raw","architecture":"aarch64","version":"202608102114","channel":"stable","seed_bytes":13312,"sha256":"3e1fbc204b7b015e2b074f53a79080151d4a49d629a380b6bc407deca961847c"}}
    ```
    On-disk `shasum -a 256` agrees, size 3 432 026 112. **Equal to ART-05's reference digest and its `seed_bytes 13312`** — a local mirror and the live HTTPS server produce the identical artifact.
  - `:185-186` — unknown channel: `{"result":{"versions":[]}}`, exit **0**.
  - `:188-190` — bad pin: `version not found: release "199901010000" not in channel "stable"; available: 202608021451, 202608072311, 202608102114`, exit 5.
  - `:198-205` — the cache-admission claim: the local adapter copied the blob to `cache/sha256/4ac7328b…fb75`, mode `-r--r--r--`.
  - The four negative `--server` modes (`http://`, missing path, regular file, empty mirror dir) are **not repeated** — proven in Wave 1, cited per instruction.

#### `docs/docs/how-to/recover-interrupted-build.md`

- **`:112-119` — the observed row was row 1** (`:114`: *present | absent | present or n/a | absent → No leftover `--force` backup. Nothing to restore*). Post-`SIGKILL` inventory, from the guide's own `:95-107` loop run verbatim:
  ```
  -rw-r--r-- 1 josh wheel 3432026112 Aug 16 13:36 /tmp/wave2a-ADocs/doc15/seeded.img
  efda64af4e7dfaf731b99ce0c14fd92cec970b77fda0c5cf7393b593f1dc68fc  /tmp/wave2a-ADocs/doc15/seeded.img
  -rw------- 1 josh wheel 281619456 Aug 16 13:36 /tmp/wave2a-ADocs/doc15/seeded.resources.img
  9cf1ab7cade5eeb8d9f5b011a5786ec77ea06ed43b1c940f3436981c856999e3  /tmp/wave2a-ADocs/doc15/seeded.resources.img
  ```
  plus exactly the two temps the guide predicts, in the documented `.<base>-<digits>.tmp` shape and in the destination directory:
  ```
  -rw-r--r-- 1 josh wheel 2391801856 Aug 16 13:40 .seeded.img-1040580127.tmp
  -rw-r--r-- 1 josh wheel          0 Aug 16 13:39 .seeded.resources.img-14530683.tmp
  ```
  **The guide's classification instructions were sufficient.** §2's loop printed the state, §3's row 1 matched on first read, §4's conditional correctly declined to act (`no .bak -> nothing to restore`), §5 removed the two named temps, §6 found nothing to discard. Final verification: both finals at their original digests, `ls .*.tmp *.bak` → `No such file or directory` for both. **No `.tmp` and no `.bak` left behind.**

- **New finding — the `.bak` window is effectively unreachable by an external race (see "New findings").** Three SIGKILL attempts; none landed between publish steps 2 and 6. In the closest attempt my watcher *did* observe a `.bak`, but the `pgrep`+`kill` round-trip was slower than steps 3–6, and the build printed its full summary before dying. So the restore path in §4 could not be reached by interrupting a warm-cache build.

- **§4's restore block is correct when the state does arise.** I exercised it against a *synthesized* row-3 state (image final renamed aside by hand — clearly labelled, not an observed interrupt), running the guide's `:138-146` block verbatim:
  ```
  efda64af4e7dfaf731b99ce0c14fd92cec970b77fda0c5cf7393b593f1dc68fc  /tmp/wave2a-ADocs/doc15/seeded.img.incusos-builder.bak
  restored: final was absent
  efda64af4e7dfaf731b99ce0c14fd92cec970b77fda0c5cf7393b593f1dc68fc  /tmp/wave2a-ADocs/doc15/seeded.img
  ```
  Verification section satisfied: final exists, `.bak` gone, digest equals the one recorded from the `.bak`.

- **Both scope claims at `:14-15` confirmed.**
  - `-o -` creates no `.bak`: streamed a full raw image to stdout (`71d479f48bb1…6a9b`, exit 0), then `ls -la | grep -E 'bak|tmp'` → `no .bak, no .tmp after -o - : CONFIRMED`.
  - `init` has no `--force`: `usage error: unknown flag: --force`, real exit **2**; `init --help` lists only `-h/--help` and `-o/--output`.

- **`:87`, `:193` — `sha256sum` works verbatim here** (F-DOC-2 revised); `shasum -a 256` returns identical digests. Recorded as evidence, not as a substitution that was forced.

- **Backup naming exact:** `<path>.incusos-builder.bak`, e.g. `seeded.img.incusos-builder.bak`.

---

### Failures and deviations, in detail

1. **F-DOC-1 reproduced end-to-end on the README (DOC-04).** Documented exit 2 arrives as exit 1 through `go run`, plus a spurious `exit status 2` stderr line. The README's three commands are the first thing a new user runs and the only install path that works today (`:7` says there is no release), so this is the default experience, not an edge case.
2. **F-DOC-5 confirmed at a real PTY (DOC-10).** `first-seeded-iso.md:157-158` is wrong at a terminal. Prompt bytes captured raw with trailing space; both `n` (exit 2, file untouched) and `y` (exit 0, file replaced) branches exercised.
3. **Offline refusal message lists two paths, the guide quotes one** (`build-offline-media.md:198`).
4. **`use-local-mirror.md:183` drops the `acquisition failed: ` prefix.**
5. **`README.md:65` implies `--resources-output` is required for rescue media.** It is not.
6. **`build-offline-media.md:215-219` implies `.bak` files persist after a successful `--force`.** They do not; step 6 deletes them. Observed across four successful `--force` runs.
7. **Deviations I introduced, all deliberate and recorded:**
   - Added `--cache-dir /tmp/wave2a-cache` to the README build (`README.md:62` has no such flag) to reuse the warm cache. Everything else in DOC-04 ran byte-literally from the repo root.
   - Ran DOC-10 with the absolute binary rather than `first-seeded-iso.md:30`'s `go run`, so the documented exit codes were observable; the `go run` form was still checked for `--version` and matched.
   - DOC-12's §1 config is the guide's own block with `x86_64` → `aarch64` (one `sed`), to reuse the warm cache. Everything else — `incus` application, `raw`, `stable`, `offline: true` — is the guide's text unmodified. The claims under test (naming, envelope shape, digest agreement, refusal) are architecture-independent.
   - DOC-12's exit-6 "output appeared during the build" case (`:216-221`) was **not** exercised: it needs a filesystem race. Reported untested, not assumed passing.
   - DOC-15's restore path was reached via a **synthesized** row-3 state after three real SIGKILL attempts all landed in row 1. Labelled as synthesized wherever cited.

---

### New findings not already in the known list

- **N-DOC-A (doc, low) — `README.md:65` overstates `--resources-output`.** Offline builds write rescue media with no flag; the flag only overrides the path. Passing it on a non-offline config is exit 2.
- **N-DOC-B (doc, low) — `build-offline-media.md:198` quotes a one-path refusal; the offline refusal names both finals.** Breaks exact-match assertions.
- **N-DOC-C (doc, low) — `use-local-mirror.md:183` omits the `acquisition failed: ` prefix** on the empty-`--cache-dir` error (exit 5). Same class as F-DOC-4.
- **N-DOC-D (doc, low) — `build-offline-media.md:215-219` presents leftover `.bak` files as a usable previous generation.** Publish step 6 deletes them; four successful `--force` runs left none. `recover-interrupted-build.md:52` says this correctly; the offline guide never does.
- **N-DOC-E (advisory) — the `--force` `.bak` window is not externally observable on a warm cache.** Publish steps 2→6 complete faster than a `pgrep`+`kill -9` round-trip. On my closest attempt the watcher saw a `.bak`, but the process finished and printed its summary before the signal landed. Practical consequence: `recover-interrupted-build.md` §4's restore path is real but essentially unreachable by interrupting a cache-warm build; the realistic post-kill state is row 1 with temps. Worth a note in the guide so an operator does not go hunting for `.bak` files that were never created.
- **N-DOC-F (advisory) — the README quickstart dirties a clean clone with a 3.4 GB untracked file.** `config.yaml` and `incusos.iso` are not in `.gitignore`.
- **N-ART-G (advisory, confirms sibling finding) — raw rescue media is not byte-reproducible; the installer is.** Five builds of the identical offline config produced `sha256 efda64af…68fc` every time for the installer, and five *different* `resources_sha256` values (`aa6e6771…`, `9cf1ab7c…`, `ce28911e…`, `1a6b1c58…`, `8264d1f6…`, `51b0599a…`). Consistent with the random GPT disk GUID + FAT32 volume serial already reported.
- **F-DOC-2 (revised) corroborated independently.** `sha256sum` is present and works verbatim on this host for every doc that uses it (`build-offline-media.md:127`, `recover-interrupted-build.md:87,193`); `shasum -a 256` agrees digit-for-digit.

---

### Side-effect ledger

**Created and removed by me**
- `/Users/josh/code/componere/incusos-builder/config.yaml`, `/incusos.iso` (3 433 074 688 B) — DOC-04's literal quickstart writes into the checkout root. Both untracked, both deleted; `git status --porcelain` empty afterward.
- `/tmp/wave2a-ADocs/doc10/` — `seeded.iso` `f122c4f6c50c7785d27cc5fe8e03e84898b5ace5646c502137c62b85f0e6e8a0`, 3 433 074 688 B. Removed.
- `/tmp/wave2a-ADocs/doc12/` — installer `efda64af4e7dfaf731b99ce0c14fd92cec970b77fda0c5cf7393b593f1dc68fc` ×4 (`seeded.img`, `hs.img`, `hq.img`, `trap.iso`), rescue media `aa6e6771…`, `9cf1ab7c…`, `ce28911e…`, `1a6b1c58…`. Removed (~14 GB).
- `/tmp/wave2a-ADocs/doc14/` — mirror tree and `seeded.img` `3e1fbc204b7b015e2b074f53a79080151d4a49d629a380b6bc407deca961847c`, plus its private cache. Removed.
- `/tmp/wave2a-ADocs/doc15/` — the interrupt bed. Removed.
- Peak scratch 25 GB, now 8 KB. Free space on `/` unchanged at ~224 GB.

**Shared cache — additive only, nothing removed**
- `/tmp/wave2a-cache/sha256/` grew 3 → 5 blobs (839 M → 1.7 G):
  - `57a2228848e3b7de026704ad7a378852f35b24f958f8097cdec220e7167ee2b5` (609 259 296 B) — the `x86_64` ISO asset pulled by DOC-04's `==> download`.
  - `9ee5a9c165434250afc8466a41e39fc1d186179e08b262c9bc890f9becb03fa5` (272 966 533 B) — `aarch64/incus.raw.gz`, pulled by DOC-12's offline build.
  - The three pre-existing blobs are untouched; `4ac7328b…fb75` re-verified by digest before I copied it into the DOC-14 mirror.

**Siblings' artifacts**
- `/tmp/wave2a-AMedia/` read only (`index.json`, `src.update.json`, `src.update.sjson` copied out; `iso-envelope.json` read). Nothing mounted, nothing modified — 16 entries before and after.
- `/tmp/wave2a-AChain/live11.yaml` read only.
- `/tmp/wave2a-AMirror/` listed only; I built my own mirror rather than reuse theirs, per Main's warning about the ART-13 tamper/restore.
- Docker untouched.

**Processes**
- `tmux -L adocs` — my own socket; sessions `f5`, `f5y` created and `kill-server`'d at the end. No shared socket touched.
- Four `incusos-builder` processes killed with `-9`, all mine, all confirmed gone via `pgrep`.

---

### Repo cleanliness

```
$ git status --porcelain     # before
(empty)
$ git rev-parse HEAD
59c268b1499bd1a5ce94d487f1bc3adf377bead3

$ git status --porcelain     # after
(empty)
$ git rev-parse HEAD
59c268b1499bd1a5ce94d487f1bc3adf377bead3
```

No tracked file was modified. `bin/incusos-builder` untouched (same size and mtime, `12:34`). The only repo-root writes were DOC-04's two untracked artifacts, both removed.

---

### Time spent / cases blocked and why

| Case | Elapsed | Notes |
|---|---|---|
| DOC-04 | ~4 min | Far under the plan's 20–45 min: the `x86_64` ISO asset came down in one 23 s build against the shared cache. |
| DOC-10 | ~3 min | Includes two real-PTY runs and a `script(1)` raw-byte capture. |
| DOC-12 | ~4 min | Seven builds; rescue-tree cross-referenced from AMedia instead of mounting, per instruction. |
| DOC-14 | ~2 min | Mirror assembled by copy; build 3.8 s. |
| DOC-15 | ~9 min | Longest, because of three SIGKILL race attempts plus pre-state resets between them. |

**Zero cases blocked.** Everything in scope executed. Two items are reported as *untested* rather than passing: `build-offline-media.md:216-221`'s exit-6 race, and DOC-15's `.bak` restore *as an observed interrupt state* (validated against a synthesized state instead, and labelled as such).

**Boot-acceptance boundary, stated explicitly.** What DOC-10 leaves the operator holding is one file: `seeded.iso`, `sha256 f122c4f6c50c7785d27cc5fe8e03e84898b5ace5646c502137c62b85f0e6e8a0`, 3 433 074 688 B, `x86_64`, IncusOS `202608102114`, channel `stable`, `seed_bytes 2048`, built at `/tmp/wave2a-ADocs/doc10/seeded.iso` and since deleted. That digest was confirmed identical on disk and in the summary, and survived an overwrite round-trip unchanged. **This is not boot acceptance.** Nothing here shows that IncusOS booted, that it consumed the `incus` application seed, or that the installer seed was wiped after install. Per `first-seeded-iso.md:174-186` and `explanation/trust-model.md:36-40`, that evidence comes only from the manual procedure in `how-to/verify-boot-acceptance.md` on an `x86_64` Linux Incus host — track C, not this slice.
