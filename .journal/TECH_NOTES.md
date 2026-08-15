# Technical Notes

- Use hexagonal architecture at all times. Keep business logic isolated from CLI, filesystem, network, storage, and other external adapters.
- Prefer functional testing before calling any feature complete. Unit tests are useful, but they do not prove the tool works the way the design intends.
- Take an agile approach to development. Avoid waterfall: underspecify when useful, prototype early, learn from the result, and refine from working behavior.
- Authoritative design docs: `.journal/001/ARCHITECTURE.md` (hexagonal design, ports, exit taxonomy, invariants) and `.journal/001/PLAN.md` (7-phase implementation plan). Start there before any implementation work.
- Upstream reference: shallow clone of `lxc/incus-os` at `reference/incus-os/` (gitignored). The web customizer is `incus-osd/cmd/image-customizer/`; seed/API types under `incus-osd/api/`.
- Seeding mechanic: uncompressed tar spliced at fixed byte offset 2,148,532,224 (2049 MiB) into the decompressed base image; `seed-data` partition is 100 MiB — tar size is a hard pre-publication invariant.
- Base images come from the update-server index (default https://images.linuxcontainers.org/os), SHA-256-verified assets; `update.sjson` (S/MIME-signed) is the metadata recovery actually trusts for offline media.
- Web API injects only 9 seed sections; incus-osd consumes 11 — `kernel` and `security` seeds are CLI-exclusive. Upstream fatally rejects non-empty `encryption_recovery_keys` in the security seed; config validation must reject it up front.
- SOPS: any top-level `sops` key in the YAML selects in-memory decryption via `github.com/getsops/sops/v3/decrypt`; all decrypt failures map to one exit code (4).
