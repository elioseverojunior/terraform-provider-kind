---
page_title: "Using a KinD cluster with the Kubernetes and Helm providers"
subcategory: "Guides"
description: |-
  Create a cluster and deploy into it in a single Terraform configuration.
---

# Using a KinD cluster with the Kubernetes and Helm providers

Creating a cluster is rarely the goal on its own — you want workloads running in
it. This guide covers wiring `kind_cluster` into the `kubernetes` and `helm`
providers, and the two pitfalls that catch people out.

## Passing credentials

`kind_cluster` exports everything the downstream providers need. The three
certificate attributes are base64-encoded, exactly as they appear inside a
kubeconfig, so each must be decoded:

```terraform
resource "kind_cluster" "this" {
  name = "demo"

  node {
    role = "control-plane"
  }
}

provider "kubernetes" {
  host                   = kind_cluster.this.endpoint
  client_certificate     = base64decode(kind_cluster.this.client_certificate)
  client_key             = base64decode(kind_cluster.this.client_key)
  cluster_ca_certificate = base64decode(kind_cluster.this.cluster_ca_certificate)
}
```

Wiring the attributes directly, rather than pointing at `kubeconfig_path`, keeps
Terraform's dependency graph correct: the `kubernetes` provider is configured
from values that only exist once the cluster has been created.

## Pitfall 1: provider configuration cannot depend on a pending resource

Terraform configures providers before it builds most of the graph. When the
cluster and the workloads live in one configuration and one state, a **fresh**
`terraform apply` may fail with `Invalid provider configuration` or
`connection refused`, because the `kubernetes` provider is configured before
`kind_cluster.this` exists.

This is a Terraform-wide constraint, not specific to this provider. There are
two reliable ways around it.

**Split the apply into two targeted stages:**

```shell
terraform apply -target=kind_cluster.this
terraform apply
```

**Or split cluster and workloads into separate root modules**, with the
workload module reading the cluster's outputs through a remote state data
source. This is the more robust option for anything long-lived, because it also
lets you destroy and recreate workloads without touching the cluster.

## Pitfall 2: ingress needs both a label and a port mapping

An ingress controller in a KinD cluster reaches the host through
`extra_port_mappings`, and must be pinned to the node those mappings belong to:

```terraform
resource "kind_cluster" "this" {
  name = "demo"

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
  }
}
```

Then set the controller's node selector to match `ingress-ready = "true"` and
enable its host ports. Omitting either half produces a controller that starts
successfully but is unreachable from the host.

## Pitfall 3: a custom CNI leaves nodes NotReady

With `networking.disable_default_cni = true`, nodes stay `NotReady` until you
install a CNI — which cannot happen until after `terraform apply` returns. Set
`wait_for_nodes_ready = false` on the cluster, or the apply will fail on a
readiness timeout that is expected and harmless:

```terraform
resource "kind_cluster" "this" {
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

`kube_proxy_mode = "none"` is the right companion setting for CNIs such as
Cilium that replace kube-proxy entirely.

## A complete, runnable example

A working configuration covering all of the above, including an ingress-nginx
release installed with Helm, lives in
[`examples/complete/kubernetes-provider`](https://github.com/elioseverojunior/terraform-provider-kind/tree/main/examples/complete/kubernetes-provider).
