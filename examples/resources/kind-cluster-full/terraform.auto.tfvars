kind_cluster = {
  name          = "cicd"
  worker_nodes  = 1 # Reduced from 3 to conserve Docker resources
  port_mappings = true
  registry = {
    enabled = true
  }
}