---
title: How to encrypt a seed config with age and SOPS
description: Encrypt a seed config with age-backed SOPS and have incusos-builder decrypt it
---

# How to encrypt a seed config with age and SOPS

Encrypt a seed config with age-backed SOPS so `incusos-builder`
decrypts it in memory from a file or from stdin.

## Prerequisites

- `incusos-builder` on `PATH`, or the same source checkout as
  [Build your first seeded ISO](../tutorials/first-seeded-iso.md)
  (`go run ./cmd/incusos-builder`).
- The `sops` CLI. The repository's fixture generator runs
  `sops --age <recipient> -e`.
- An age identity. This guide needs the secret key in `SOPS_AGE_KEY`
  and the matching recipient for `sops --age`.
- A valid plaintext seed config, for example `config.yaml` from the
  first-ISO tutorial.

Do not commit the age secret key. Do not put it in the seed config.
`incusos-builder` never writes decrypted bytes to the filesystem, and
validation errors name field paths, not secret values.

## 1. Export the age secret key

`incusos-builder` decrypts through SOPS. Put the identity's
`AGE-SECRET-KEY-` line in `SOPS_AGE_KEY`:

```bash
export SOPS_AGE_KEY='AGE-SECRET-KEY-...'
```

If `SOPS_AGE_KEY_FILE` or `SOPS_AGE_KEY_CMD` is set in the environment,
unset it for this session. An empty `SOPS_AGE_KEY_FILE` makes SOPS open
path `""` instead of using `SOPS_AGE_KEY`.

## 2. Encrypt the seed config

```bash
sops --age "$AGE_RECIPIENT" -e config.yaml > config.enc.yaml
```

`$AGE_RECIPIENT` is the public recipient for the key in
`SOPS_AGE_KEY`. The output is YAML whose values are `ENC[...]` blobs
and whose document has a top-level `sops` key.

`incusos-builder` selects the encrypted path when that top-level `sops`
key is present. It does not inspect the `sops` block first. A leftover
`sops:` key on an otherwise plaintext file is treated as encrypted.

## 3. Validate the encrypted file

```bash
incusos-builder validate -f config.enc.yaml --color never
```

Expected stdout:

```text
configuration valid
```

Exit status is `0`. `validate` decrypts in memory, then runs the same
schema checks as for plaintext. It does not fetch an image.

## 4. Validate from stdin

`-f -` reads the seed config from stdin, including SOPS-encrypted
bytes:

```bash
incusos-builder validate -f - --color never < config.enc.yaml
```

Expected stdout is again `configuration valid`.

## 5. Build from the encrypted file

```bash
incusos-builder build -f config.enc.yaml -o seeded.iso --color never
```

This is the same `build` as for plaintext. Decryption happens while
loading `-f`, before the image is fetched. You can also pipe the
encrypted document:

```bash
incusos-builder build -f - -o seeded.iso --color never < config.enc.yaml
```

## Verification

With `SOPS_AGE_KEY` set, both of these exit `0` and print
`configuration valid`:

```bash
incusos-builder validate -f config.enc.yaml --color never
incusos-builder validate -f - --color never < config.enc.yaml
```

Clear the key and repeat the file path:

```bash
env SOPS_AGE_KEY= incusos-builder validate -f config.enc.yaml
```

The process exits `4`. stderr contains `decryption failed`.

## Troubleshooting

### Exit 4, `decryption failed`

The document had a top-level `sops` key and decryption failed. The
same wording covers a missing `SOPS_AGE_KEY`, a key that does not
match the recipient, malformed `sops` metadata, and a MAC mismatch.

Fix the key or the encrypted file. Do not expect a schema (exit `3`)
message until decryption succeeds.

### Exit 3 after a successful decrypt

Decryption worked and the plaintext failed validation. stderr names a
field path such as `image.type` or `version`. Fix the plaintext, then
encrypt again.

### A plaintext file started failing with exit 4

The file has a top-level `sops` key. Remove it, or encrypt the
document properly. Presence of the key alone selects decryption.

## Related

- [Build your first seeded ISO](../tutorials/first-seeded-iso.md)
