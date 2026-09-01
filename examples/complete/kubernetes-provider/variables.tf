variable "cluster_name" {
  description = "Name of the KinD cluster. Must match ^[a-z0-9.-]+$."
  type        = string
  default     = "demo"
}

variable "node_image" {
  description = "Node image pinning the Kubernetes version. Leave empty to use the version bundled with kind."
  type        = string
  default     = "kindest/node:v1.37.0"
}

variable "http_port" {
  description = "Host port forwarded to the ingress controller's HTTP port."
  type        = number
  default     = 8080
}

variable "https_port" {
  description = "Host port forwarded to the ingress controller's HTTPS port."
  type        = number
  default     = 8443
}
