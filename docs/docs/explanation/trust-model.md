---
title: About the trust model
description: Why incusos-builder treats HTTPS, hash binding, and S/MIME structure as build-time checks, and leaves signature authentication to the booted system
---

# About the trust model

incusos-builder is a media factory. It downloads an IncusOS image, writes a
seed config into the `seed-data` partition, and can stage rescue media. It
is not the IncusOS update verifier. The interesting question is therefore
not "is the builder a root of trust?" but "which failures must it refuse
before it emits media, and which checks only the booted system can make?"

Those two layers exist because they solve different problems. A truncated
index, a plaintext redirect, or a cache blob that does not match its
declared digest would otherwise produce an installer that only fails after
you boot it. Signature authentication of `update.sjson` belongs on the
booted system: that system has the Incus OS update CA, and recovery
verifies the signature over the exact bytes on the rescue media.

## Two trust boundaries

Build-time checks answer "did we fetch and stage the bytes we intended?"
They run in the HTTPS client, the content-addressed cache, and the
metadata parser. They wrap as fetch failures. They do not prove that Linux
Containers signed the release.

Boot-time checks answer "does IncusOS accept this media?" Secure Boot
enrollment, dm-verity of the OS partitions, seed consumption from the
`seed-data` partition, and S/MIME authentication of rescue `update.sjson`
all happen after the builder has exited. The production update CA is not
in this checkout. The builder therefore cannot, and does not, claim that
step.

The split is a tradeoff. It keeps the compiled closure type-only — see
[About upstream version coupling](upstream-version-coupling.md) — instead
of linking `incus-osd` recovery code. It also means a successful build is
not evidence that a host will install or recover. The manual
[boot-acceptance checklist](../how-to/verify-boot-acceptance.md) is the
procedure for that observation.

## Why HTTPS, and why a redirect still has to be HTTPS

`build` and `versions` use `--server` to select an `https://` URL or an
existing local directory. For those commands, a plain `http://` value is
rejected as a bad flag before any adapter runs. `NewHTTPSSource` also
refuses a non-https base URL, so a caller that bypasses the flag still
cannot construct the network source.

`validate` intentionally performs no network or update-source
validation. It ignores `--server` and validates only the seed config.
`init` also does not use the flag.

Every GET then checks the *final* response URL. A redirect to `http://`
is refused. The comment in the client states the failure that matters: a
downgrade would yield a plaintext `index.json` an attacker can fill with
chosen digests. Asset admission would then faithfully cache those chosen
bytes.

That is transport integrity to the host you named, plus a closed redirect
hole. It is not publisher authentication. The default `http.Client` uses
the process's system CA set and Go's default TLS. There is no pinned
update-server certificate and no TLS version floor in this tree.

A local mirror directory is the other first-class source. It skips TLS
because there is no network. It still runs the same size and digest
admission and the same metadata structure checks. Trust then sits in
whoever populated the directory.

`UpdateFull.URL` on the live index is a relative path such as
`/202608102114`. Neither this client nor the upstream customizer treats
that field as a new host. A future absolute `https://…` override is
possible in the type and unused on the server that was probed.

## Filename, size, and SHA-256

The update index is unsigned JSON. Integrity of a downloaded file
therefore cannot come from "the server said so." It comes from treating
each `UpdateFile` as an untrusted triple and refusing to use any field as
a URL or cache path until it is well-formed:

- `Filename` is a relative path whose `/` segments each match
  `[A-Za-z0-9._-]+`. Empty segments, `.`, `..`, a leading `/`, and `\`
  are rejected.
- `Sha256` is exactly 64 lowercase hex characters. That string is also
  the cache path component under `<cache>/sha256/`.
- `Size` must satisfy `0 < Size ≤ 8 GiB` before any download.

Admission then binds the body to both `Size` and `Sha256`: exactly
`Size` bytes then EOF, and a SHA-256 that matches. A mismatch is not
retried as a checksum success; the fetch layer gives one clean
re-download and then fails with the admission wording. Reuse of a cache
entry re-hashes the file. Handles always open the immutable cache object,
never the original download or a later-changed mirror file.

That is why Filename and Size appear next to the digest. Filename is how
the server names the object and how rescue media restages it. Size is
bound at cache admission so a short or overlong body cannot be treated
as the named object. SHA-256 is the content bind. Admission therefore
pairs Size with Sha256; selected-file checks on `update.sjson` pair
Filename with Sha256. Neither pair is a signature.

JSON decoding of `index.json`, `update.json`, and the sjson payload
ignores unknown fields on purpose. Integrity is the size cap, the digest,
and selected-file binding. `DisallowUnknownFields` would couple every
extra server key to a builder rebuild. Trailing data after the first JSON
value is still rejected.

## What `update.sjson` is, and what parsing it does not do

Rescue recovery trusts `update.sjson`, not `update.json`. The unsigned
twin exists so humans and tools can read the same file list. Both
documents sit next to the assets at `/<version>/`, not in the index
`files` array, so they carry no index digest. The adapter validates them
structurally and returns the HTTP (or file) bodies verbatim. Recovery
verifies a signature over those exact bytes; re-serializing would break
that.

Structural validation requires:

- `update.json` decodes as `apiimages.Update` with `Version` equal to the
  selected release.
- `update.sjson` is a `multipart/signed` S/MIME message.
- Its first part decodes as `apiimages.Update` with the same `Version`.
- Every selected asset appears in that payload's `Files` with an equal
  `Filename` and `Sha256`.

An empty `UpdateSJSON` is refused before rescue media is written.
Upstream recovery treats media without that document as a silent no-op.

This is not PKCS#7 authentication. The parser reads the clear-text part
and checks JSON and file binding. It does not require a chain to the
Incus OS update CA. Test fixtures use a throwaway self-signed envelope
for the same reason: MIME shape and binding, not recovery-grade
signatures.

`update.json` is only version-checked. Extra files may exist in the
sjson payload; the bind is "every selected file is present," not "the
lists are identical."

## Observations from the live server and boot spikes

These are measurements, not invariants.

On 2026-08-15, `https://images.linuxcontainers.org/os` served
`index.json` at 35 672 bytes and, for release `202608102114`,
`update.json` at 11 859 bytes and `update.sjson` at 14 268 bytes. The
sjson envelope was `multipart/signed` with
`protocol=application/x-pkcs7-signature` and `micalg=sha-256`. The
detached PKCS#7 carried two certificates in the Incus OS update hierarchy
and a SHA-256 signed-data digest. After trimming, the clear-text payload
was one byte shorter than `update.json` (trailing newline on the unsigned
twin). The three `[]UpdateFile` lists — index entry, `update.json`, sjson
payload — agreed positionally on `(Filename, Sha256, Size)` for all 55
files.

Those sizes are why each metadata document is capped at 1 MiB (~70×
headroom at the measured 14 KiB) and `index.json` at 64 MiB. The caps
are DoS bounds, not authenticity.

Earlier boot experiments on the spliced `202608102114` image reached
UEFI → systemd-boot → UKI → kernel → systemd PID 1. Secure Boot is a
hard IncusOS gate: firmware that cannot report Secure Boot state stops
with `IncusOS will only boot on UEFI systems`. A setup-mode firmware
with `secure-boot-enroll force` enrolled the shipped keys and continued.
The QEMU spikes and Phase 5.2 Linux diagnostic did not observe seed
consumption or run recovery.

On 2026-08-17, Track C met all four boot-acceptance observations on a
Semaphore Cloud `f1-standard-4` machine with nested KVM (pipeline
`4c5cc805`, job `a8b16331`):

- **O1, install completion:** serial output contained `IncusOS was
  successfully installed` followed by `Please remove the install media
  to complete the installation` 45 seconds after start
  (`internal/install/install.go:388-390`).
- **O2, seed consumption:** source partition `/dev/loop0p2` contained
  `applications.yaml`, `install.yaml`, and `update.yaml`; target
  partition `/dev/sdb2` retained `applications.yaml` and `update.yaml`
  but not `install.yaml`. The source and target digests differed
  (`8537e356…` and `84a75f5c…`), while the source digest was unchanged
  after installation. The surviving application seed is the positive
  control that the target archive was readable and not blank.
- **O3, rescue-media detection:** serial output contained `Recovery
  partition detected` (`internal/recovery/recovery.go:50`).
- **O4, recovery-payload acceptance (human-adjudicated):** serial output
  progressed from `Update metadata detected, verifying signature` to
  `Processing validated update metadata`, which follows successful
  `util.VerifySMIME` verification
  (`internal/recovery/recovery.go:180-212`). It then contained `Recovery
  actions completed` and `System is starting up`; `Recovery failed:`
  was absent.

This is one observation on one venue, for IncusOS release
`202608102114` and upstream API pin
`v0.0.0-20260815030500-0f5b8057f2fc`. It is not a guarantee for later
pins. The run used raw media, so it did not exercise the NUL-padded ISO
volume identifier; `N-MEDIA-3` remains open.

## What this model refuses to claim

HTTPS plus hash admission means the builder cached the bytes the index
named. It does not mean the index is a signed Incus OS update, that the
OS partitions will verify under dm-verity, or that a host will consume
the seed config. Those are boot-boundary properties. Confusing the two
layers produces media that looks finished and fails only when someone
tries to install.