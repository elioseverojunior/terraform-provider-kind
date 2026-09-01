<!--
SPDX-FileCopyrightText: 2026 Elio Severo Junior <elioseverojunior@gmail.com>

SPDX-License-Identifier: MIT OR Apache-2.0
-->

# Security Policy

## Supported versions

This project is pre-1.0. Security fixes are released against the latest
published version only. Upgrade before reporting an issue against an older tag.

## Reporting a vulnerability

**Do not open a public issue for a security problem.**

Report it through
[GitHub private vulnerability reporting](https://github.com/elioseverojunior/terraform-provider-kind/security/advisories/new),
or by email to <elioseverojunior@gmail.com>.

Please include the provider version, the Terraform version, a description of the
impact, and the smallest configuration that reproduces it. You should get an
acknowledgement within a week.

## Scope

This provider creates local Docker containers and reads the kubeconfig kind
generates. Findings that are in scope include credential exposure through
Terraform state or logs, command or configuration injection through resource
arguments, and privilege escalation beyond what the invoking user already holds
over their own Docker daemon.

The following are **out of scope**, because they are properties of the tools
being wrapped rather than of this provider:

- KinD clusters are not hardened and are not meant for production. They run
  Kubernetes components with development defaults.
- Anyone who can reach the Docker daemon already has effective root on the host.
  This provider does not, and cannot, add a boundary there.
- Vulnerabilities in `kindest/node` images. Report those to
  [kubernetes-sigs/kind](https://github.com/kubernetes-sigs/kind).

## Handling credentials

`kind_cluster` exports `kubeconfig`, `client_certificate`, `client_key` and
`cluster_ca_certificate`. All four are marked sensitive, so Terraform redacts
them from plan and apply output.

They are still **written to Terraform state in cleartext**. State containing
these values grants full administrative access to the cluster. Treat the state
file accordingly: keep it out of version control, and use a backend with
encryption at rest if it is stored remotely.
