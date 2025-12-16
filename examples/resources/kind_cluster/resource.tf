# Basic cluster with control-plane and worker
resource "kind_cluster" "basic" {
  name = "basic-cluster"

  node {
    role = "control-plane"
  }

  node {
    role = "worker"
  }
}

# Cluster with specific Kubernetes version
resource "kind_cluster" "versioned" {
  name       = "versioned-cluster"
  node_image = "kindest/node:v1.31.2"

  node {
    role = "control-plane"
  }

  node {
    role = "worker"
  }
}

# Cluster with custom networking
resource "kind_cluster" "custom_network" {
  name = "custom-network-cluster"

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

# Ingress-ready cluster with port mappings
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

  node {
    role = "worker"
  }
}

# Cluster with containerd registry configuration
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

# HA cluster with multiple control planes
resource "kind_cluster" "ha" {
  name = "ha-cluster"

  node {
    role = "control-plane"
  }

  node {
    role = "control-plane"
  }

  node {
    role = "control-plane"
  }

  node {
    role = "worker"
  }

  node {
    role = "worker"
  }
}

# Cluster with feature gates and runtime config
resource "kind_cluster" "advanced" {
  name = "advanced-cluster"

  feature_gates = {
    "EphemeralContainers" = true
  }

  runtime_config = {
    "api/beta" = "true"
  }

  node {
    role = "control-plane"
  }

  node {
    role = "worker"
  }
}

# Cluster with volume mounts
resource "kind_cluster" "mounts" {
  name = "mounts-cluster"

  node {
    role = "control-plane"

    extra_mounts {
      host_path      = "/tmp/kind-data"
      container_path = "/data"
      read_only      = false
    }
  }

  node {
    role = "worker"
  }
}
