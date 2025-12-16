terraform {
  required_providers {
    kind = {
      source = "elioseverojunior/kind"
    }
  }
}

provider "kind" {}

# List all existing KinD clusters
data "kind_clusters" "all" {}

output "existing_clusters" {
  description = "List of all KinD clusters"
  value       = data.kind_clusters.all.clusters
}

output "cluster_count" {
  description = "Number of KinD clusters"
  value       = length(data.kind_clusters.all.clusters)
}
