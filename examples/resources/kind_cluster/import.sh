# KinD clusters are imported by their cluster name, which is also the resource ID.
# Run `kind get clusters` to list the names available on this machine.
terraform import kind_cluster.default my-cluster
