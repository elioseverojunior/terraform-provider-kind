<!--
SPDX-FileCopyrightText: 2026 Elio Severo Junior <elioseverojunior@gmail.com>

SPDX-License-Identifier: MIT OR Apache-2.0
-->

# Terraform Provider for KinD

[![Terraform Registry](https://img.shields.io/badge/registry-elioseverojunior%2Fkind-7B42BC?style=flat&logo=terraform)](https://registry.terraform.io/providers/elioseverojunior/kind/latest)
[![Go Reference](https://img.shields.io/badge/go-reference-00ADD8?style=flat&logo=go)](https://pkg.go.dev/github.com/elioseverojunior/terraform-provider-kind)
[![License](https://img.shields.io/badge/license-MIT%20OR%20Apache--2.0-blue.svg)](#license)

Manage [KinD](https://kind.sigs.k8s.io/) (Kubernetes in Docker) clusters with
Terraform. The provider embeds kind's own Go library rather than shelling out to
the `kind` binary, so create, refresh and destroy are driven by the same code
the CLI uses — and the `kind` binary does not need to be installed.

Built for ephemeral CI clusters, reproducible local development environments,
and integration tests that need a real API server.

## Contents

- [Requirements](#requirements)
- [Installation](#installation)
- [Quick start](#quick-start)
- [Common configurations](#common-configurations)
- [Using the cluster with other providers](#using-the-cluster-with-other-providers)
- [Importing an existing cluster](#importing-an-existing-cluster)
- [Troubleshooting](#troubleshooting)
- [Limitations](#limitations)
- [Documentation](#documentation)
- [Contributing](#contributing)
- [License](#license)

## Requirements

| Requirement | Notes |
| ----------- | ----- |
| [Docker](https://docs.docker.com/get-docker/) | Installed and **running**. Podman and nerdctl are auto-detected by kind but are not covered by this project's tests. |
| Terraform >= 1.0 | Or OpenTofu >= 1.6. |
| Memory | Each node is a container; budget roughly 2&nbsp;GB per node. |

The `kind` CLI is optional. It is handy for troubleshooting, since
`kind get clusters` and `kind export logs` inspect the very same clusters.

## Installation

### From the Terraform Registry

```terraform
terraform {
  required_providers {
    kind = {
      source  = "elioseverojunior/kind"
      version = ">= 0.0.3"
    }
  }
}

provider "kind" {}
```

### From source

```shell
git clone https://github.com/elioseverojunior/terraform-provider-kind.git
cd terraform-provider-kind
make install
```

This installs into `~/.terraform.d/plugins/registry.terraform.io/elioseverojunior/kind/<version>/<os>_<arch>/`,
where Terraform will find it without any registry lookup.

## Quick start

```terraform
resource "kind_cluster" "default" {
  name = "my-cluster"

  node {
    role = "control-plane"
  }

  node {
    role = "worker"
  }
}

output "kubeconfig_path" {
  value = kind_cluster.default.kubeconfig_path
}
```

```shell
terraform init
terraform apply

export KUBECONFIG=$(terraform output -raw kubeconfig_path)
kubectl get nodes
```

Omitting the `node` blocks entirely gives kind's default topology: one
control-plane node and one worker.

## Common configurations

### Pinning the Kubernetes version

```terraform
resource "kind_cluster" "versioned" {
  name       = "pinned"
  node_image = "kindest/node:v1.37.0"

  node {
    role = "control-plane"
  }
}
```

Node images are built for a specific kind release. Leave `node_image` unset to
use the digest-pinned default that ships with the bundled kind version — the
safest option. If you do pin, pick an image listed in the
[kind release notes](https://github.com/kubernetes-sigs/kind/releases) for that
release; an arbitrary Kubernetes tag may not have a matching node image.

### Ingress-ready cluster

An ingress controller needs both a labelled node and host port mappings on that
same node.

```terraform
resource "kind_cluster" "ingress" {
  name = "ingress"

  node {
    role = "control-plane"

    labels = {
      "ingress-ready" = "true"
    }

    extra_port_mappings {
      container_port = 80
      host_port      = 8080
      protocol       = "TCP"
    }

    extra_port_mappings {
      container_port = 443
      host_port      = 8443
      protocol       = "TCP"
    }
  }

  node {
    role = "worker"
  }
}
```

### Highly available control plane

```terraform
resource "kind_cluster" "ha" {
  name = "ha"

  node { role = "control-plane" }
  node { role = "control-plane" }
  node { role = "control-plane" }
  node { role = "worker" }
}
```

kind places an external load balancer container in front of the control planes
automatically.

### Custom CNI

With the default CNI disabled, nodes stay `NotReady` until you install a
replacement — which cannot happen before `terraform apply` returns. Turn the
readiness wait off, or the apply fails on an expected timeout.

```terraform
resource "kind_cluster" "cilium" {
  name                 = "cilium"
  wait_for_nodes_ready = false

  networking {
    disable_default_cni = true
    kube_proxy_mode     = "none"
  }

  node {
    role = "control-plane"
  }
}
```

`kube_proxy_mode = "none"` suits CNIs such as Cilium that replace kube-proxy
entirely. The other accepted modes are `iptables` (default), `ipvs` and
`nftables`.

### Local registry mirror

```terraform
resource "kind_cluster" "registry" {
  name = "with-registry"

  containerd_config_patches = [
    <<-TOML
    [plugins."io.containerd.grpc.v1.cri".registry]
      config_path = "/etc/containerd/certs.d"
    TOML
  ]

  node {
    role = "control-plane"
  }
}
```

A complete registry setup, including the registry container itself, is in
[`examples/complete/cicd-cluster`](examples/complete/cicd-cluster).

### Listing existing clusters

```terraform
data "kind_clusters" "all" {}

output "clusters" {
  value = data.kind_clusters.all.clusters
}
```

## Using the cluster with other providers

`kind_cluster` exports the credentials the `kubernetes` and `helm` providers
need. The certificate attributes are base64-encoded, as they are inside a
kubeconfig, so decode each one:

```terraform
provider "kubernetes" {
  host                   = kind_cluster.default.endpoint
  client_certificate     = base64decode(kind_cluster.default.client_certificate)
  client_key             = base64decode(kind_cluster.default.client_key)
  cluster_ca_certificate = base64decode(kind_cluster.default.cluster_ca_certificate)
}
```

> **A fresh apply needs two stages.** Terraform configures providers before the
> cluster exists, so the first `terraform apply` of a configuration that both
> creates the cluster and deploys into it will fail. Either run
> `terraform apply -target=kind_cluster.default` first, or keep the cluster and
> its workloads in separate root modules. This is a Terraform-wide constraint,
> not specific to this provider.

The [Kubernetes and Helm guide](https://registry.terraform.io/providers/elioseverojunior/kind/latest/docs/guides/kubernetes-and-helm)
covers this in full, and
[`examples/complete/kubernetes-provider`](examples/complete/kubernetes-provider)
is a runnable version.

## Importing an existing cluster

Clusters are imported by name, which is also the resource ID:

```shell
terraform import kind_cluster.default my-cluster
```

Run `kind get clusters` to see the available names. Only `name` is imported;
every other attribute is re-read from kind on the next refresh.

## Troubleshooting

### `Cannot connect to the Docker daemon`

Docker is not running, or is listening somewhere the provider is not looking.
Check with `docker info`. For Colima, Rancher Desktop, or a rootless daemon, set
the socket explicitly:

```terraform
provider "kind" {
  host = "unix:///Users/me/.colima/default/docker.sock"
}
```

### `timeout ... waiting for nodes to become ready`

The cluster came up but some nodes never reached `Ready`; the error names them.

```shell
kubectl --context kind-<cluster-name> describe node <node-name>
```

Usually this is too little memory allocated to Docker, or a cluster with
`disable_default_cni = true` and no CNI installed. Raise `wait_for_ready` on
slow machines, or set `wait_for_nodes_ready = false`.

### A cluster is replaced on every plan

Nearly every argument is `RequiresReplace`, because kind cannot reconfigure a
running cluster. Read the plan output to see which attribute changed — a
floating `node_image` tag is the usual culprit.

### `port is already allocated`

Another container or service holds a port named in `extra_port_mappings`. Choose
a different `host_port`, or set `networking.api_server_port = 0` to let the
kernel pick a free API server port.

### Leftover containers after an interrupted apply

```shell
kind delete clusters --all
```

## Limitations

- **Clusters are immutable.** kind cannot add, remove or reconfigure nodes on a
  running cluster, so changing the topology destroys and recreates it — along
  with everything deployed inside.
- **Local only.** Nodes are Docker containers on the machine running Terraform.
  There is no remote infrastructure to manage.
- **Not for production.** KinD clusters run development defaults and are not
  hardened.
- **Credentials land in state.** `kubeconfig`, `client_key` and the certificates
  are stored in Terraform state in cleartext. See [SECURITY.md](SECURITY.md).

## Documentation

The full, generated reference lives on the
[Terraform Registry](https://registry.terraform.io/providers/elioseverojunior/kind/latest/docs):

- [Provider configuration](docs/index.md)
- [`kind_cluster` resource](docs/resources/cluster.md)
- [`kind_clusters` data source](docs/data-sources/clusters.md)
- [Guide: Kubernetes and Helm](docs/guides/kubernetes-and-helm.md)

For the internals, see [ARCHITECTURE.md](ARCHITECTURE.md).

## Contributing

Contributions are welcome. [CONTRIBUTING.md](CONTRIBUTING.md) covers the
development loop, testing tiers and how the documentation is generated.

```shell
make test      # unit tests, no Docker needed
make lint      # must report 0 issues
make generate  # regenerate docs/ after a schema change
make testacc   # acceptance tests: real clusters, needs Docker
```

Report bugs and request features through
[GitHub issues](https://github.com/elioseverojunior/terraform-provider-kind/issues).
Security reports go through [SECURITY.md](SECURITY.md) instead.

## License

Dual-licensed under either of:

- MIT ([LICENSE](LICENSE))
- Apache License 2.0 ([LICENSE-APACHE](LICENSE-APACHE))

at your option. Unless you state otherwise, any contribution you intentionally
submit for inclusion in this work shall be dual-licensed as above, with no
additional terms or conditions.

## Related projects

- [KinD](https://kind.sigs.k8s.io/) — Kubernetes in Docker
- [terraform-plugin-framework](https://github.com/hashicorp/terraform-plugin-framework) — the provider SDK used here
