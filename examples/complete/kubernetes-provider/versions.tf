terraform {
  required_version = ">= 1.0"

  required_providers {
    kind = {
      source  = "elioseverojunior/kind"
      version = ">= 0.0.3"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = ">= 2.0"
    }
    helm = {
      source  = "hashicorp/helm"
      version = ">= 2.0"
    }
  }
}
