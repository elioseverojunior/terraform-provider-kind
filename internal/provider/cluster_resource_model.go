package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type ClusterResourceModel struct {
	ID                              types.String    `tfsdk:"id"`
	Name                            types.String    `tfsdk:"name"`
	NodeImage                       types.String    `tfsdk:"node_image"`
	WaitForReady                    types.Int64     `tfsdk:"wait_for_ready"`
	Networking                      *NetworkingModel `tfsdk:"networking"`
	FeatureGates                    types.Map       `tfsdk:"feature_gates"`
	RuntimeConfig                   types.Map       `tfsdk:"runtime_config"`
	KubeadmConfigPatches            types.List      `tfsdk:"kubeadm_config_patches"`
	KubeadmConfigPatchesJSON6902    []PatchJSON6902Model `tfsdk:"kubeadm_config_patches_json6902"`
	ContainerdConfigPatches         types.List      `tfsdk:"containerd_config_patches"`
	ContainerdConfigPatchesJSON6902 types.List      `tfsdk:"containerd_config_patches_json6902"`
	Kubeconfig                      types.String    `tfsdk:"kubeconfig"`
	KubeconfigPath                  types.String    `tfsdk:"kubeconfig_path"`
	ClientCertificate               types.String    `tfsdk:"client_certificate"`
	ClientKey                       types.String    `tfsdk:"client_key"`
	ClusterCaCertificate            types.String    `tfsdk:"cluster_ca_certificate"`
	Endpoint                        types.String    `tfsdk:"endpoint"`
	Nodes                           []NodeModel     `tfsdk:"node"`
}

type NetworkingModel struct {
	IPFamily          types.String `tfsdk:"ip_family"`
	APIServerPort     types.Int64  `tfsdk:"api_server_port"`
	APIServerAddress  types.String `tfsdk:"api_server_address"`
	PodSubnet         types.String `tfsdk:"pod_subnet"`
	ServiceSubnet     types.String `tfsdk:"service_subnet"`
	DisableDefaultCNI types.Bool   `tfsdk:"disable_default_cni"`
	KubeProxyMode     types.String `tfsdk:"kube_proxy_mode"`
	DNSSearch         types.List   `tfsdk:"dns_search"`
}

type NodeModel struct {
	Role                         types.String         `tfsdk:"role"`
	Image                        types.String         `tfsdk:"image"`
	Labels                       types.Map            `tfsdk:"labels"`
	ExtraMounts                  []MountModel         `tfsdk:"extra_mounts"`
	ExtraPortMappings            []PortMappingModel   `tfsdk:"extra_port_mappings"`
	KubeadmConfigPatches         types.List           `tfsdk:"kubeadm_config_patches"`
	KubeadmConfigPatchesJSON6902 []PatchJSON6902Model `tfsdk:"kubeadm_config_patches_json6902"`
}

type MountModel struct {
	HostPath       types.String `tfsdk:"host_path"`
	ContainerPath  types.String `tfsdk:"container_path"`
	ReadOnly       types.Bool   `tfsdk:"read_only"`
	SelinuxRelabel types.Bool   `tfsdk:"selinux_relabel"`
	Propagation    types.String `tfsdk:"propagation"`
}

type PortMappingModel struct {
	ContainerPort types.Int64  `tfsdk:"container_port"`
	HostPort      types.Int64  `tfsdk:"host_port"`
	ListenAddress types.String `tfsdk:"listen_address"`
	Protocol      types.String `tfsdk:"protocol"`
}

type PatchJSON6902Model struct {
	Group   types.String `tfsdk:"group"`
	Version types.String `tfsdk:"version"`
	Kind    types.String `tfsdk:"kind"`
	Patch   types.String `tfsdk:"patch"`
}

type ClustersDataSourceModel struct {
	ID       types.String   `tfsdk:"id"`
	Clusters []types.String `tfsdk:"clusters"`
}
