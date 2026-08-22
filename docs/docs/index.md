---
title: incusos-builder Docs
slug: /
description: Build seeded IncusOS installation media from a YAML config
---

# incusos-builder

incusos-builder builds seeded IncusOS installation media from a YAML
seed config. It is a local CLI alternative to the IncusOS web
customizer at https://incusos-customizer.linuxcontainers.org/ui/.

To learn the workflow, [build your first seeded ISO](tutorials/first-seeded-iso.md).

When you already know the tool:

- Task guides: [install incusos-builder](how-to/install.md),
  [encrypt secrets with SOPS](how-to/sops-encryption.md),
  [build offline media](how-to/build-offline-media.md),
  [run in CI](how-to/run-in-ci.md),
  [use a local mirror](how-to/use-local-mirror.md),
  [recover an interrupted build](how-to/recover-interrupted-build.md),
  and [verify boot acceptance](how-to/verify-boot-acceptance.md)
- Reference: [configuration](reference/configuration.md),
  [CLI](reference/cli.md),
  [automation](reference/automation.md),
  and [cache](reference/cache.md)
- Background: [trust model](explanation/trust-model.md),
  [seed injection](explanation/seed-injection.md),
  and [upstream version coupling](explanation/upstream-version-coupling.md)
