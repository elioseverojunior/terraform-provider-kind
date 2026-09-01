output "endpoint" {
  description = "Kubernetes API server endpoint."
  value       = kind_cluster.this.endpoint
}

output "kubeconfig_path" {
  description = "Path to the kubeconfig kind wrote for this cluster."
  value       = kind_cluster.this.kubeconfig_path
}

output "kubectl" {
  description = "Command to talk to the cluster with kubectl."
  value       = "KUBECONFIG=${kind_cluster.this.kubeconfig_path} kubectl get nodes"
}
