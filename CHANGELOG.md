<!--
SPDX-FileCopyrightText: 2026 Elio Severo Junior <elioseverojunior@gmail.com>

SPDX-License-Identifier: MIT OR Apache-2.0
-->

# Changelog

All notable changes to this project are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and
this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Schema validation for enum-like arguments. `node.role`, `networking.ip_family`,
  `networking.kube_proxy_mode`, `extra_port_mappings.protocol` and
  `extra_mounts.propagation` are now checked at plan time, and `name` is checked
  against kind's own cluster-name rule. Invalid values previously reached kind
  and failed mid-apply with an opaque message.
- `none` documented and accepted as a `networking.kube_proxy_mode`, for CNIs
  such as Cilium that replace kube-proxy entirely. kind has always supported it;
  the provider did not document it.
- `-debug` flag for running the provider under a debugger.
- Registry guide covering the `kubernetes` and `helm` provider integration,
  including the two-stage apply required on a fresh configuration.
- Runnable examples: `examples/complete/kubernetes-provider` and
  `examples/complete/cicd-cluster`.
- Import documentation and example for `kind_cluster`.
- Unit and acceptance test suites; coverage of `internal/provider` is above 99%.
- `CONTRIBUTING.md`, `SECURITY.md`, `ARCHITECTURE.md` and issue templates.

### Changed

- The node-readiness timeout now names the nodes that were still not ready,
  instead of reporting only that a timeout elapsed.
- Provider documentation is generated from templates and now carries
  requirements, Docker host configuration and a troubleshooting section.
- Dependencies updated: Kubernetes libraries to v0.37.0,
  terraform-plugin-framework to v1.19.0, kind to v0.33.0.

### Fixed

- `docs/` was stale and omitted the `host` provider argument entirely.
- Licence files were unusable: `LICENSE-APACHE` was a truncated placeholder and
  `LICENSE` named a placeholder copyright holder. Both now carry the real texts,
  and `.copywrite.hcl` no longer contradicts them.
- `.goreleaser.yml` injected `-X main.commit`, but no such variable existed, so
  the value was silently discarded.
- Licence headers were being embedded inside the Terraform code samples on the
  published Registry pages.

## [0.0.3] and earlier

No changelog was kept for these releases. They introduced the `kind_cluster`
resource and the `kind_clusters` data source.
