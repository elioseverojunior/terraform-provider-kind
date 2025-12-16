# Kind Provider

This is a Terraform IaC Provider

## Requirements

I want to implement using the natively KinD CLI GoLang Package a Terraform IaC Provider

1. https://github.com/kubernetes-sigs/kind
2. https://github.com/docker/go-sdk
3. https://github.com/hashicorp/terraform-plugin-go and https://developer.hashicorp.com/terraform/plugin/sdkv2/guides/v1-upgrade-guide or https://github.com/hashicorp/terraform-provider-scaffolding-framework

## Implementation

### Initial Codebase

1. Create a cluster (Create a cluster only with 1 control-plane and 1 worker)
2. Modify a cluster (Adding new worker node)
3. Destroy the cluster (Destroy the cluster)
