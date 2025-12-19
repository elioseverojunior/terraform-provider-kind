#!/usr/bin/env bash

set -o errexit

INPUT_CLUSTER=${1:-kind-dev}
CLUSTER=${INPUT_CLUSTER/kind-/}
CUSTER_STATUS=""

# 1. Create registry container unless it already exists
REGISTRY_NAME="${CLUSTER}-registry"
REGISTRY_PORT="5001"
if [ "$(docker inspect -f '{{.State.Running}}' "${REGISTRY_NAME}" 2>/dev/null || true)" != 'true' ]; then
  docker run \
    -d \
    --name "${REGISTRY_NAME}" \
    --hostname "${REGISTRY_NAME}" \
    --restart=always \
    -p "127.0.0.1:${REGISTRY_PORT}:5000" \
    --network bridge \
    registry:2
  echo "Docker registry is configured"
else
  echo "Docker registry is already configured"
fi

# 2. Create kind cluster with containerd registry config dir enabled
# TODO: kind will eventually enable this by default and this patch will
# be unnecessary.
#
# See:
# https://github.com/kubernetes-sigs/kind/issues/2875
# https://github.com/containerd/containerd/blob/main/docs/cri/config.md#registry-configuration
# See: https://github.com/containerd/containerd/blob/main/docs/hosts.md
echo -e "Checking KinD Cluster ${CLUSTER}..."
if [[ -z "$(kind get clusters | grep -e "${CLUSTER}")" ]];
then
  echo -e "Creating KinD Cluster ${CLUSTER}..."
  cat <<EOF | kind create cluster --name ${CLUSTER} --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
containerdConfigPatches:
  - |
    [plugins."io.containerd.grpc.v1.cri".registry]
      config_path = "/etc/containerd/certs.d"
nodes:
  - role: control-plane
    kubeadmConfigPatches:
      - |
        ---
        kind: ClusterConfiguration
        apiServer:
          extraArgs:
            enable-admission-plugins: NodeRestriction,MutatingAdmissionWebhook,ValidatingAdmissionWebhook
        ---
        kind: InitConfiguration
        nodeRegistration:
          kubeletExtraArgs:
            node-labels: "name=control-plane,ingress-ready=true"
    extraPortMappings:
      - containerPort: 80
        hostPort: 80
        protocol: TCP
      - containerPort: 443
        hostPort: 443
        protocol: TCP
    labels:
      name: control-plane
  - role: worker
    kubeadmConfigPatches:
      - |
        kind: JoinConfiguration
        nodeRegistration:
          kubeletExtraArgs:
            node-labels: "name=node01"
    labels:
      name: node01
  - role: worker
    kubeadmConfigPatches:
      - |
        kind: JoinConfiguration
        nodeRegistration:
          kubeletExtraArgs:
            node-labels: "name=node02"
    labels:
      name: node02
  - role: worker
    kubeadmConfigPatches:
      - |
        kind: JoinConfiguration
        nodeRegistration:
          kubeletExtraArgs:
            node-labels: "name=node03"
    labels:
      name: node03

EOF
  echo -e "KinD Cluster ${CLUSTER} has been created."
  CUSTER_STATUS=$?
else
  echo -e "KinD Cluster ${CLUSTER} already has been created."
  kubectl config use-context ${INPUT_CLUSTER}
  CUSTER_STATUS=0
fi

if [[ ${CUSTER_STATUS} == 1 ]];
then
  echo "Error"
  exit -1
fi

# 3. Add the registry config to the nodes
#
# This is necessary because localhost resolves to loopback addresses that are
# network-namespace local.
# In other words: localhost in the container is not localhost on the host.
#
# We want a consistent name that works from both ends, so we tell containerd to
# alias localhost:${REGISTRY_PORT} to the registry container when pulling images
REGISTRY_DIR="/etc/containerd/certs.d/localhost:${REGISTRY_PORT}"
for node in $(kind get nodes --name ${CLUSTER}); do
  echo "Patching ${node}"
  docker exec "${node}" mkdir -p "${REGISTRY_DIR}"
  cat <<EOF | docker exec -i "${node}" cp /dev/stdin "${REGISTRY_DIR}/hosts.toml"
[host."http://${REGISTRY_NAME}:5000"]
EOF
done

# 4. Connect the registry to the cluster network if not already connected
# This allows kind to bootstrap the network but ensures they're on the same network
if [ "$(docker inspect -f='{{json .NetworkSettings.Networks.kind}}' "${REGISTRY_NAME}")" = 'null' ]; then
  docker network connect "kind" "${REGISTRY_NAME}"
fi

# 5. Document the local registry
# https://github.com/kubernetes/enhancements/tree/master/keps/sig-cluster-lifecycle/generic/1755-communicating-a-local-registry
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: local-registry-hosting
  namespace: kube-public
data:
  localRegistryHosting.v1: |
    host: "localhost:${REGISTRY_PORT}"
    help: "https://kind.sigs.k8s.io/docs/user/local-registry/"
EOF
