# KinD Clusters
resource "kind_cluster" "cluster" {
  name       = var.kind_cluster.name
  node_image = "kindest/node:v1.34.0" # Use recent stable image compatible with kind CLI

  containerd_config_patches = flatten([
    # Base registry config
    [
      <<-TOML
      [plugins."io.containerd.grpc.v1.cri".registry]
        config_path = "/etc/containerd/certs.d"
      TOML
    ],
    # Registry mirror config if registry is enabled
    var.kind_cluster.registry.enabled ? [
      <<-TOML
      [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:${local.cluster_registry.port}"]
        endpoint = ["http://${local.cluster_registry.name}:5000"]
      TOML
    ] : []
  ])

  feature_gates = var.kind_cluster.feature_gates

  # Control Plane Node
  node {
    role = "control-plane"
    labels = {
      name       = "control-plane"
      managed_by = "Terraform"
      cluster    = var.kind_cluster.name
    }

    kubeadm_config_patches = [
      <<-YAML
      kind: ClusterConfiguration
      apiServer:
        extraArgs:
          enable-admission-plugins: NodeRestriction,MutatingAdmissionWebhook,ValidatingAdmissionWebhook
      YAML
      ,
      <<-YAML
      kind: InitConfiguration
      nodeRegistration:
        kubeletExtraArgs:
          node-labels: "name=control-plane,ingress-ready=true"
      YAML
    ]

    # Standard port mappings (80/443)
    dynamic "extra_port_mappings" {
      for_each = var.kind_cluster.port_mappings ? toset(["enabled"]) : toset([])
      content {
        container_port = 80
        host_port      = 80
        listen_address = "127.0.0.1"
        protocol       = "TCP"
      }
    }

    dynamic "extra_port_mappings" {
      for_each = var.kind_cluster.port_mappings ? toset(["enabled"]) : toset([])
      content {
        container_port = 443
        host_port      = 443
        listen_address = "127.0.0.1"
        protocol       = "TCP"
      }
    }

    # Custom port mappings
    dynamic "extra_port_mappings" {
      for_each = var.kind_cluster.extra_port_mappings
      content {
        container_port = extra_port_mappings.value.container_port
        host_port      = extra_port_mappings.value.host_port
        listen_address = "127.0.0.1"
        protocol       = extra_port_mappings.value.protocol
      }
    }
  }

  # Worker Nodes
  dynamic "node" {
    for_each = range(var.kind_cluster.worker_nodes)
    content {
      kubeadm_config_patches = [
        <<-YAML
        kind: JoinConfiguration
        nodeRegistration:
          kubeletExtraArgs:
            node-labels: "name=${format("node%02d", node.key + 1)},ingress-ready=true"
        YAML
      ]
      labels = {
        name       = format("node%02d", node.key + 1)
        managed_by = "Terraform"
        cluster    = var.kind_cluster.name
      }
      role = "worker"
    }
  }

  runtime_config = {}
}

locals {
  # Generate registry configurations
  cluster_registry = merge(var.kind_cluster.registry, {
    name = coalesce(var.kind_cluster.registry.name, "${var.kind_cluster.name}-registry")
  })
}

# Docker Registry for clusters that need it
resource "shell_script" "kind_registry" {
  for_each = local.cluster_registry.enabled ? toset(["enabled"]) : toset([])

  # Triggers force recreation when these values change
  triggers = {
    registry_name = local.cluster_registry.name
    registry_port = local.cluster_registry.port
  }

  lifecycle_commands {
    create = <<-EOF
      #!/bin/bash
      set -e

      # Check if registry already exists
      if [ "$(docker inspect -f '{{.State.Running}}' "${local.cluster_registry.name}" 2>/dev/null || true)" != 'true' ]; then
        docker run \
          -d \
          --name "${local.cluster_registry.name}" \
          --hostname "${local.cluster_registry.name}" \
          --restart=always \
          -p "127.0.0.1:${local.cluster_registry.port}:5000" \
          --network bridge \
          registry:2
        echo '{"created": "true", "name": "${local.cluster_registry.name}", "port": "${local.cluster_registry.port}"}'
      else
        echo '{"created": "false", "name": "${local.cluster_registry.name}", "port": "${local.cluster_registry.port}", "reason": "already_exists"}'
      fi
    EOF

    read = <<-EOF
      #!/bin/bash
      if [ "$(docker inspect -f '{{.State.Running}}' "${local.cluster_registry.name}" 2>/dev/null || true)" = 'true' ]; then
        echo '{"running": "true", "name": "${local.cluster_registry.name}", "port": "${local.cluster_registry.port}"}'
      else
        echo '{"running": "false", "name": "${local.cluster_registry.name}", "port": "${local.cluster_registry.port}"}'
      fi
    EOF

    delete = <<-EOF
      #!/bin/bash
      docker stop "${local.cluster_registry.name}" 2>/dev/null || true
      docker rm "${local.cluster_registry.name}" 2>/dev/null || true
    EOF
  }
}

# Configure registry for cluster nodes
resource "shell_script" "configure_registry_nodes" {
  for_each = local.cluster_registry.enabled ? toset(["enabled"]) : toset([])

  # Triggers force recreation when these values change
  triggers = {
    registry_name = local.cluster_registry.name
    registry_port = local.cluster_registry.port
  }

  lifecycle_commands {
    create = <<-EOF
      #!/bin/bash
      set -e

      REGISTRY_DIR="/etc/containerd/certs.d/localhost:${local.cluster_registry.port}"

      # Configure each node
      for node in $(kind get nodes --name "${var.kind_cluster.name}"); do
        echo "Configuring registry on $node"
        docker exec "$node" mkdir -p "$REGISTRY_DIR"
        echo "[host.\"http://${local.cluster_registry.name}:5000\"]" | docker exec -i "$node" cp /dev/stdin "$REGISTRY_DIR/hosts.toml"
      done

      # Connect registry to cluster network if not connected
      if [ "$(docker inspect -f='{{json .NetworkSettings.Networks.kind}}' "${local.cluster_registry.name}")" = 'null' ]; then
        docker network connect "kind" "${local.cluster_registry.name}"
      fi

      echo '{"configured": "true", "cluster": "${var.kind_cluster.name}"}'
    EOF

    read = <<-EOF
      #!/bin/bash
      # Check if registry is connected to kind network
      if [ "$(docker inspect -f='{{json .NetworkSettings.Networks.kind}}' "${local.cluster_registry.name}" 2>/dev/null)" != 'null' ]; then
        echo '{"configured": "true", "cluster": "${var.kind_cluster.name}"}'
      else
        echo '{"configured": "false", "cluster": "${var.kind_cluster.name}"}'
      fi
    EOF

    delete = <<-EOF
      #!/bin/bash
      # No cleanup needed - registry config is inside the cluster nodes
      echo '{"deleted": "true"}'
    EOF
  }

  depends_on = [
    kind_cluster.cluster,
    shell_script.kind_registry,
  ]
}

# Create ConfigMap for local registry documentation
resource "shell_script" "local_registry_configmap" {
  for_each = local.cluster_registry.enabled ? toset(["enabled"]) : toset([])

  # Triggers force recreation when these values change
  triggers = {
    registry_name = local.cluster_registry.name
    registry_port = local.cluster_registry.port
  }

  lifecycle_commands {
    create = <<-EOF
      #!/bin/bash
      set -e

      export KUBECONFIG="${kind_cluster.cluster.kubeconfig_path}"

      cat <<YAML | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: local-registry-hosting
  namespace: kube-public
data:
  localRegistryHosting.v1: |
    host: "localhost:${local.cluster_registry.port}"
    help: "https://kind.sigs.k8s.io/docs/user/local-registry/"
YAML

      echo '{"created": "true", "name": "local-registry-hosting"}'
    EOF

    read = <<-EOF
      #!/bin/bash
      export KUBECONFIG="${kind_cluster.cluster.kubeconfig_path}"

      if kubectl get configmap local-registry-hosting -n kube-public &>/dev/null; then
        echo '{"exists": "true", "name": "local-registry-hosting"}'
      else
        echo '{"exists": "false"}'
      fi
    EOF

    delete = <<-EOF
      #!/bin/bash
      export KUBECONFIG="${kind_cluster.cluster.kubeconfig_path}"

      kubectl delete configmap local-registry-hosting -n kube-public 2>/dev/null || true
      echo '{"deleted": "true"}'
    EOF
  }

  depends_on = [
    kind_cluster.cluster,
    shell_script.configure_registry_nodes,
  ]
}

# Outputs
output "kind_cluster" {
  description = "KinD cluster information"
  value = {
    name            = kind_cluster.cluster.name
    kubeconfig_path = kind_cluster.cluster.kubeconfig_path
    endpoint        = kind_cluster.cluster.endpoint
    context         = "kind-${kind_cluster.cluster.name}"
  }
}

output "kind_registries" {
  description = "Docker registry information for KinD clusters"
  value = local.cluster_registry.enabled ? {
    name     = local.cluster_registry.name
    port     = local.cluster_registry.port
    endpoint = "localhost:${local.cluster_registry.port}"
  } : null
}
