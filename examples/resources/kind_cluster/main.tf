terraform {
  required_providers {
    kind = {
      source = "registry.terraform.io/elioseverojunior/kind"
    }
  }
}

provider "kind" {}

# Simple cluster with 1 control-plane and 1 worker
resource "kind_cluster" "default" {
  name       = "my-cluster"
  node_image = "kindest/node:v1.34.2"

  node {
    role = "control-plane"
  }

  node {
    role = "worker"
  }
}

# Cluster with ingress-ready configuration
resource "kind_cluster" "ingress" {
  name       = "ingress-cluster"
  node_image = "kindest/node:v1.34.2"

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

  node {
    role = "worker"
  }
}

output "default_kubeconfig" {
  value     = kind_cluster.default.kubeconfig
  sensitive = true
}

output "default_endpoint" {
  value = kind_cluster.default.endpoint
}

output "ingress_endpoint" {
  value = kind_cluster.ingress.endpoint
}
