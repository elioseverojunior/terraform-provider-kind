# Provision a KinD cluster and immediately manage workloads inside it with the
# kubernetes and helm providers.
#
# The kind_cluster resource exports the credentials these providers need, so no
# kubeconfig file has to exist on disk and nothing has to be exported into the
# shell environment first.

resource "kind_cluster" "this" {
  name       = var.cluster_name
  node_image = var.node_image

  node {
    role = "control-plane"

    # Required for an ingress controller to bind the host ports below.
    labels = {
      "ingress-ready" = "true"
    }

    extra_port_mappings {
      container_port = 80
      host_port      = var.http_port
      protocol       = "TCP"
    }

    extra_port_mappings {
      container_port = 443
      host_port      = var.https_port
      protocol       = "TCP"
    }
  }

  node {
    role = "worker"
  }
}

# The credentials are base64-encoded, exactly as they appear in a kubeconfig,
# so each one is decoded before being handed to the kubernetes provider.
provider "kubernetes" {
  host                   = kind_cluster.this.endpoint
  client_certificate     = base64decode(kind_cluster.this.client_certificate)
  client_key             = base64decode(kind_cluster.this.client_key)
  cluster_ca_certificate = base64decode(kind_cluster.this.cluster_ca_certificate)
}

provider "helm" {
  kubernetes {
    host                   = kind_cluster.this.endpoint
    client_certificate     = base64decode(kind_cluster.this.client_certificate)
    client_key             = base64decode(kind_cluster.this.client_key)
    cluster_ca_certificate = base64decode(kind_cluster.this.cluster_ca_certificate)
  }
}

resource "kubernetes_namespace" "demo" {
  metadata {
    name = "demo"
  }
}

resource "helm_release" "ingress_nginx" {
  name             = "ingress-nginx"
  repository       = "https://kubernetes.github.io/ingress-nginx"
  chart            = "ingress-nginx"
  namespace        = "ingress-nginx"
  create_namespace = true

  # Pin the controller to the labelled control-plane node so the host port
  # mappings declared on the cluster actually reach it.
  set {
    name  = "controller.nodeSelector.ingress-ready"
    value = "true"
  }

  set {
    name  = "controller.hostPort.enabled"
    value = "true"
  }
}
