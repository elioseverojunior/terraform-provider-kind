terraform {
  required_version = ">= 1.0.0"

  required_providers {
    kind = {
      source  = "elioseverojunior/kind"
      version = ">= 0.1.0"
    }
    shell = {
      source  = "scottwinkler/shell"
      version = ">= 1.7.0"
    }
  }
}
