terraform {
  required_providers {
    kind = {
      source = "registry.terraform.io/elioseverojunior/kind"
    }
  }
}

provider "kind" {}

# Data source to list all existing KinD clusters
data "kind_clusters" "all" {}

output "existing_clusters" {
  description = "List of all KinD clusters currently running"
  value       = data.kind_clusters.all.clusters
}

output "cluster_count" {
  description = "Number of KinD clusters"
  value       = length(data.kind_clusters.all.clusters)
}

# Example: Check if a specific cluster exists
output "has_my_cluster" {
  description = "Whether 'my-cluster' exists"
  value       = contains(data.kind_clusters.all.clusters, "my-cluster")
}
