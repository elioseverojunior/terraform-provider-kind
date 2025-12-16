# Terraform Provider for KinD (Kubernetes in Docker)

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Terraform](https://img.shields.io/badge/Terraform-1.0+-7B42BC?style=flat&logo=terraform)](https://www.terraform.io/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A Terraform provider for managing [KinD](https://kind.sigs.k8s.io/) (Kubernetes in Docker) clusters. This provider wraps the native KinD Go library to provide full infrastructure-as-code support for local Kubernetes development clusters.

## Features

- Full KinD v1alpha4 configuration support
- Multi-node clusters (control-plane and workers)
- High Availability (HA) clusters with multiple control planes
- Custom networking configuration (IP family, subnets, proxy mode)
- Ingress-ready clusters with port mappings
- Containerd and kubeadm configuration patches
- Feature gates and runtime configuration
- Volume mounts and SELinux support
- Automatic kubeconfig export

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.22 (for building)
- [Docker](https://docs.docker.com/get-docker/) (running)
- [KinD](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) CLI (optional, for debugging)

## Installation

### From Source

```bash
git clone https://github.com/elioseverojunior/terraform-provider-kind.git
cd terraform-provider-kind
make install
```

### Manual Installation

```bash
go build -o terraform-provider-kind
mkdir -p ~/.terraform.d/plugins/registry.terraform.io/elioseverojunior/kind/0.1.0/$(go env GOOS)_$(go env GOARCH)
cp terraform-provider-kind ~/.terraform.d/plugins/registry.terraform.io/elioseverojunior/kind/0.1.0/$(go env GOOS)_$(go env GOARCH)/
```

## Quick Start

```hcl
terraform {
  required_providers {
    kind = {
      source = "elioseverojunior/kind"
    }
  }
}

provider "kind" {}

resource "kind_cluster" "default" {
  name = "my-cluster"

  node {
    role = "control-plane"
  }

  node {
    role = "worker"
  }
}

output "kubeconfig" {
  value     = kind_cluster.default.kubeconfig
  sensitive = true
}

output "endpoint" {
  value = kind_cluster.default.endpoint
}
```

```bash
terraform init
terraform apply

# Use the cluster
export KUBECONFIG=$(terraform output -raw kubeconfig_path)
kubectl get nodes
```

## Examples

### Specific Kubernetes Version

```hcl
resource "kind_cluster" "versioned" {
  name       = "k8s-131"
  node_image = "kindest/node:v1.31.2"

  node {
    role = "control-plane"
  }

  node {
    role = "worker"
  }
}
```

### Ingress-Ready Cluster

```hcl
resource "kind_cluster" "ingress" {
  name = "ingress-cluster"

  node {
    role = "control-plane"

    labels = {
      "ingress-ready" = "true"
    }

    kubeadm_config_patches = [
      <<-YAML
      kind: InitConfiguration
      nodeRegistration:
        kubeletExtraArgs:
          node-labels: "ingress-ready=true"
      YAML
    ]

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

### Custom Networking

```hcl
resource "kind_cluster" "custom" {
  name = "custom-network"

  networking {
    ip_family           = "ipv4"
    api_server_port     = 6443
    api_server_address  = "127.0.0.1"
    pod_subnet          = "10.244.0.0/16"
    service_subnet      = "10.96.0.0/12"
    disable_default_cni = false
    kube_proxy_mode     = "iptables"
  }

  node {
    role = "control-plane"
  }

  node {
    role = "worker"
  }
}
```

### HA Cluster (Multiple Control Planes)

```hcl
resource "kind_cluster" "ha" {
  name = "ha-cluster"

  node { role = "control-plane" }
  node { role = "control-plane" }
  node { role = "control-plane" }
  node { role = "worker" }
  node { role = "worker" }
}
```

### Private Registry Support

```hcl
resource "kind_cluster" "registry" {
  name = "registry-cluster"

  containerd_config_patches = [
    <<-TOML
    [plugins."io.containerd.grpc.v1.cri".registry]
      config_path = "/etc/containerd/certs.d"
    TOML
  ]

  node {
    role = "control-plane"
  }

  node {
    role = "worker"
  }
}
```

### Feature Gates and Runtime Config

```hcl
resource "kind_cluster" "advanced" {
  name = "advanced-cluster"

  feature_gates = {
    "EphemeralContainers" = true
    "WindowsGMSA"         = false
  }

  runtime_config = {
    "api/beta"  = "true"
    "api/alpha" = "true"
  }

  node {
    role = "control-plane"
  }

  node {
    role = "worker"
  }
}
```

## Resources

### kind_cluster

Manages a KinD cluster lifecycle.

#### Arguments

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | string | Yes | Cluster name |
| `node_image` | string | No | Default node image (e.g., `kindest/node:v1.31.2`) |
| `wait_for_ready` | number | No | Seconds to wait for control plane (default: 300) |
| `networking` | block | No | Networking configuration |
| `feature_gates` | map(bool) | No | Kubernetes feature gates |
| `runtime_config` | map(string) | No | API server runtime config |
| `kubeadm_config_patches` | list(string) | No | Kubeadm YAML merge patches |
| `containerd_config_patches` | list(string) | No | Containerd TOML patches |
| `node` | block | No | Node configuration (defaults to 1 CP + 1 worker) |

#### Attributes (Computed)

| Name | Description |
|------|-------------|
| `kubeconfig` | Kubeconfig content (sensitive) |
| `kubeconfig_path` | Path to kubeconfig file |
| `endpoint` | API server endpoint |
| `client_certificate` | Client certificate (base64, sensitive) |
| `client_key` | Client key (base64, sensitive) |
| `cluster_ca_certificate` | CA certificate (base64, sensitive) |

## Data Sources

### kind_clusters

Lists all existing KinD clusters.

```hcl
data "kind_clusters" "all" {}

output "clusters" {
  value = data.kind_clusters.all.clusters
}
```

## Development

```bash
# Build
make build

# Install locally
make install

# Run tests
make test

# Run acceptance tests (creates real clusters)
make testacc

# Generate documentation
make docs

# Format code
make fmt

# Lint
make lint
```

## Documentation

Full documentation is available in the [docs](./docs) directory:

- [Provider Configuration](./docs/index.md)
- [kind_cluster Resource](./docs/resources/cluster.md)
- [kind_clusters Data Source](./docs/data-sources/clusters.md)

## Available KinD Images

| Kubernetes Version | Image |
|--------------------|-------|
| v1.32.0 | `kindest/node:v1.32.0` |
| v1.31.2 | `kindest/node:v1.31.2` |
| v1.30.6 | `kindest/node:v1.30.6` |
| v1.29.10 | `kindest/node:v1.29.10` |

See all available images: https://hub.docker.com/r/kindest/node/tags

## Limitations

- **Node modifications require cluster recreation**: KinD doesn't support adding/removing nodes to existing clusters. Any change to node configuration triggers cluster destruction and recreation.
- **Local clusters only**: This provider manages local Docker-based clusters, not remote infrastructure.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License - see [LICENSE](LICENSE) for details.

## Related Projects

- [KinD](https://kind.sigs.k8s.io/) - Kubernetes in Docker
- [Terraform](https://www.terraform.io/) - Infrastructure as Code
- [terraform-plugin-framework](https://github.com/hashicorp/terraform-plugin-framework) - Terraform Provider SDK
