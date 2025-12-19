variable "kind_cluster" {
  description = "KinD cluster configuration"
  type = object({
    enabled       = optional(bool, true)
    name          = string
    worker_nodes  = optional(number, 2)
    port_mappings = optional(bool, true) # Map ports 80/443 to host
    registry = optional(object({
      enabled = optional(bool, false)
      name    = optional(string, null)
      port    = optional(number, 5001)
    }), {})
    extra_port_mappings = optional(list(object({
      container_port = number
      host_port      = number
      protocol       = optional(string, "TCP")
    })), [])
    feature_gates = optional(map(bool), {})
  })
  nullable = false
}
