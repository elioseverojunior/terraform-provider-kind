package provider

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/kind/pkg/apis/config/v1alpha4"
	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/yaml"
)

var (
	_ resource.Resource                = &ClusterResource{}
	_ resource.ResourceWithImportState = &ClusterResource{}
)

type ClusterResource struct {
	provider *cluster.Provider
}

func NewClusterResource() resource.Resource {
	return &ClusterResource{}
}

// cleanupStaleLockFile removes stale kubeconfig lock files that may be left over
// from interrupted operations. Only removes locks older than 60 seconds.
func cleanupStaleLockFile() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}

	lockFile := filepath.Join(homeDir, ".kube", "config.lock")
	info, err := os.Stat(lockFile)
	if err != nil {
		return // Lock file doesn't exist
	}

	// Only remove if older than 60 seconds (stale)
	if time.Since(info.ModTime()) > 60*time.Second {
		os.Remove(lockFile)
	}
}

// waitForAllNodesReady waits for all nodes in the cluster to be in Ready state.
// It uses the kubeconfig to connect to the cluster and polls node status.
func waitForAllNodesReady(ctx context.Context, kubeconfigContent string, timeout time.Duration) error {
	// Create a temporary kubeconfig file for the client
	tmpFile, err := os.CreateTemp("", "kubeconfig-*.yaml")
	if err != nil {
		return fmt.Errorf("failed to create temp kubeconfig: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(kubeconfigContent); err != nil {
		return fmt.Errorf("failed to write kubeconfig: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close kubeconfig file: %w", err)
	}

	// Build kubernetes client from kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", tmpFile.Name())
	if err != nil {
		return fmt.Errorf("failed to build kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	// Poll until all nodes are ready or timeout
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	timeoutCh := time.After(timeout)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeoutCh:
			return fmt.Errorf("timeout waiting for nodes to be ready after %v", timeout)
		case <-ticker.C:
			nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
			if err != nil {
				// Cluster might not be fully ready yet, continue polling
				continue
			}

			if len(nodes.Items) == 0 {
				// No nodes yet, continue polling
				continue
			}

			allReady := true
			notReadyNodes := []string{}
			for _, node := range nodes.Items {
				nodeReady := false
				for _, condition := range node.Status.Conditions {
					if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
						nodeReady = true
						break
					}
				}
				if !nodeReady {
					allReady = false
					notReadyNodes = append(notReadyNodes, node.Name)
				}
			}

			if allReady {
				return nil
			}
			// Continue polling - some nodes are not ready yet
		}
	}
}

func (r *ClusterResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster"
}

func (r *ClusterResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a KinD (Kubernetes in Docker) cluster.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Cluster identifier (same as name).",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the cluster.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"node_image": schema.StringAttribute{
				Description: "The node image to use for the cluster nodes. Applies to all nodes unless overridden per node.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"wait_for_ready": schema.Int64Attribute{
				Description: "Time in seconds to wait for the control plane to be ready. Default is 300 (5 minutes).",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(300),
			},
			"wait_for_nodes_ready": schema.BoolAttribute{
				Description: "Wait for all nodes (including workers) to be in Ready state after cluster creation. Uses the wait_for_ready timeout. Default is true.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"feature_gates": schema.MapAttribute{
				Description: "Kubernetes feature gates to enable/disable. Map of feature gate name to boolean.",
				Optional:    true,
				ElementType: types.BoolType,
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.RequiresReplace(),
				},
			},
			"runtime_config": schema.MapAttribute{
				Description: "Runtime configuration for kube-apiserver (--runtime-config flags). Used to enable alpha APIs.",
				Optional:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.RequiresReplace(),
				},
			},
			"kubeadm_config_patches": schema.ListAttribute{
				Description: "Kubeadm config patches (RFC 7386 merge patches) applied to all nodes.",
				Optional:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"containerd_config_patches": schema.ListAttribute{
				Description: "Containerd config patches (TOML format) applied to all nodes.",
				Optional:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"containerd_config_patches_json6902": schema.ListAttribute{
				Description: "Containerd config patches (RFC 6902 JSON patches) applied to all nodes.",
				Optional:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"kubeconfig": schema.StringAttribute{
				Description: "The kubeconfig content for connecting to the cluster.",
				Computed:    true,
				Sensitive:   true,
			},
			"kubeconfig_path": schema.StringAttribute{
				Description: "The path to the kubeconfig file.",
				Computed:    true,
			},
			"client_certificate": schema.StringAttribute{
				Description: "Base64 encoded client certificate for TLS authentication.",
				Computed:    true,
				Sensitive:   true,
			},
			"client_key": schema.StringAttribute{
				Description: "Base64 encoded client key for TLS authentication.",
				Computed:    true,
				Sensitive:   true,
			},
			"cluster_ca_certificate": schema.StringAttribute{
				Description: "Base64 encoded cluster CA certificate.",
				Computed:    true,
				Sensitive:   true,
			},
			"endpoint": schema.StringAttribute{
				Description: "The Kubernetes API server endpoint.",
				Computed:    true,
			},
		},
		Blocks: map[string]schema.Block{
			"networking": schema.SingleNestedBlock{
				Description: "Cluster networking configuration.",
				Attributes: map[string]schema.Attribute{
					"ip_family": schema.StringAttribute{
						Description: "IP family for the cluster: ipv4, ipv6, or dual.",
						Optional:    true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
						},
					},
					"api_server_port": schema.Int64Attribute{
						Description: "Port for the API server on the host. 0 for random, -1 for backend selection.",
						Optional:    true,
						PlanModifiers: []planmodifier.Int64{
							int64planmodifier.RequiresReplace(),
						},
					},
					"api_server_address": schema.StringAttribute{
						Description: "Address to bind the API server on the host. Defaults to 127.0.0.1.",
						Optional:    true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
						},
					},
					"pod_subnet": schema.StringAttribute{
						Description: "CIDR for pod IPs. Example: 10.244.0.0/16.",
						Optional:    true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
						},
					},
					"service_subnet": schema.StringAttribute{
						Description: "CIDR for service IPs. Example: 10.96.0.0/12.",
						Optional:    true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
						},
					},
					"disable_default_cni": schema.BoolAttribute{
						Description: "Disable the default CNI (kindnet). Set to true to install a custom CNI.",
						Optional:    true,
						PlanModifiers: []planmodifier.Bool{
							boolplanmodifier.RequiresReplace(),
						},
					},
					"kube_proxy_mode": schema.StringAttribute{
						Description: "Kube-proxy mode: iptables, ipvs, or nftables.",
						Optional:    true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
						},
					},
					"dns_search": schema.ListAttribute{
						Description: "DNS search domains for nodes.",
						Optional:    true,
						ElementType: types.StringType,
						PlanModifiers: []planmodifier.List{
							listplanmodifier.RequiresReplace(),
						},
					},
				},
			},
			"kubeadm_config_patches_json6902": schema.ListNestedBlock{
				Description: "Kubeadm config patches (RFC 6902 JSON patches) applied to all nodes.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"group": schema.StringAttribute{
							Description: "API group of the target resource.",
							Required:    true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplace(),
							},
						},
						"version": schema.StringAttribute{
							Description: "API version of the target resource.",
							Required:    true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplace(),
							},
						},
						"kind": schema.StringAttribute{
							Description: "Kind of the target resource.",
							Required:    true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplace(),
							},
						},
						"patch": schema.StringAttribute{
							Description: "JSON patch content (RFC 6902 format).",
							Required:    true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplace(),
							},
						},
					},
				},
			},
			"node": schema.ListNestedBlock{
				Description: "Node configuration. If not specified, creates 1 control-plane and 1 worker. Changes trigger cluster recreation.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"role": schema.StringAttribute{
							Description: "Node role: control-plane or worker.",
							Required:    true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplace(),
							},
						},
						"image": schema.StringAttribute{
							Description: "Node image. Overrides cluster-level node_image.",
							Optional:    true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplace(),
							},
						},
						"labels": schema.MapAttribute{
							Description: "Kubernetes labels for the node.",
							Optional:    true,
							ElementType: types.StringType,
							PlanModifiers: []planmodifier.Map{
								mapplanmodifier.RequiresReplace(),
							},
						},
						"kubeadm_config_patches": schema.ListAttribute{
							Description: "Kubeadm config patches for this node (RFC 7386 merge patches).",
							Optional:    true,
							ElementType: types.StringType,
							PlanModifiers: []planmodifier.List{
								listplanmodifier.RequiresReplace(),
							},
						},
					},
					Blocks: map[string]schema.Block{
						"extra_mounts": schema.ListNestedBlock{
							Description: "Additional volume mounts for the node.",
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"host_path": schema.StringAttribute{
										Description: "Path on the host.",
										Required:    true,
										PlanModifiers: []planmodifier.String{
											stringplanmodifier.RequiresReplace(),
										},
									},
									"container_path": schema.StringAttribute{
										Description: "Path in the container.",
										Required:    true,
										PlanModifiers: []planmodifier.String{
											stringplanmodifier.RequiresReplace(),
										},
									},
									"read_only": schema.BoolAttribute{
										Description: "Read-only mount.",
										Optional:    true,
										PlanModifiers: []planmodifier.Bool{
											boolplanmodifier.RequiresReplace(),
										},
									},
									"selinux_relabel": schema.BoolAttribute{
										Description: "Enable SELinux relabeling.",
										Optional:    true,
										PlanModifiers: []planmodifier.Bool{
											boolplanmodifier.RequiresReplace(),
										},
									},
									"propagation": schema.StringAttribute{
										Description: "Mount propagation: None, HostToContainer, or Bidirectional.",
										Optional:    true,
										PlanModifiers: []planmodifier.String{
											stringplanmodifier.RequiresReplace(),
										},
									},
								},
							},
						},
						"extra_port_mappings": schema.ListNestedBlock{
							Description: "Port mappings from host to container.",
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"container_port": schema.Int64Attribute{
										Description: "Port in the container.",
										Required:    true,
										PlanModifiers: []planmodifier.Int64{
											int64planmodifier.RequiresReplace(),
										},
									},
									"host_port": schema.Int64Attribute{
										Description: "Port on the host.",
										Required:    true,
										PlanModifiers: []planmodifier.Int64{
											int64planmodifier.RequiresReplace(),
										},
									},
									"listen_address": schema.StringAttribute{
										Description: "Host bind address. Defaults to 127.0.0.1.",
										Optional:    true,
										PlanModifiers: []planmodifier.String{
											stringplanmodifier.RequiresReplace(),
										},
									},
									"protocol": schema.StringAttribute{
										Description: "Protocol: TCP, UDP, or SCTP.",
										Optional:    true,
										PlanModifiers: []planmodifier.String{
											stringplanmodifier.RequiresReplace(),
										},
									},
								},
							},
						},
						"kubeadm_config_patches_json6902": schema.ListNestedBlock{
							Description: "Kubeadm config patches for this node (RFC 6902 JSON patches).",
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"group": schema.StringAttribute{
										Description: "API group.",
										Required:    true,
										PlanModifiers: []planmodifier.String{
											stringplanmodifier.RequiresReplace(),
										},
									},
									"version": schema.StringAttribute{
										Description: "API version.",
										Required:    true,
										PlanModifiers: []planmodifier.String{
											stringplanmodifier.RequiresReplace(),
										},
									},
									"kind": schema.StringAttribute{
										Description: "Resource kind.",
										Required:    true,
										PlanModifiers: []planmodifier.String{
											stringplanmodifier.RequiresReplace(),
										},
									},
									"patch": schema.StringAttribute{
										Description: "JSON patch content.",
										Required:    true,
										PlanModifiers: []planmodifier.String{
											stringplanmodifier.RequiresReplace(),
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *ClusterResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	provider, ok := req.ProviderData.(*cluster.Provider)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *cluster.Provider, got: %T", req.ProviderData),
		)
		return
	}

	r.provider = provider
}

func (r *ClusterResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Clean up any stale lock files from previous interrupted operations
	cleanupStaleLockFile()

	var data ClusterResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	clusterName := data.Name.ValueString()

	cfg := r.buildClusterConfig(&data)

	createOpts := []cluster.CreateOption{
		cluster.CreateWithV1Alpha4Config(cfg),
		cluster.CreateWithWaitForReady(time.Duration(data.WaitForReady.ValueInt64()) * time.Second),
		cluster.CreateWithDisplayUsage(false),
		cluster.CreateWithDisplaySalutation(false),
	}

	if !data.NodeImage.IsNull() && data.NodeImage.ValueString() != "" {
		createOpts = append(createOpts, cluster.CreateWithNodeImage(data.NodeImage.ValueString()))
	}

	err := r.provider.Create(clusterName, createOpts...)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create cluster", err.Error())
		return
	}

	r.populateComputedValues(&data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	// Wait for all nodes to be ready if enabled
	if !data.WaitForNodesReady.IsNull() && data.WaitForNodesReady.ValueBool() {
		timeout := time.Duration(data.WaitForReady.ValueInt64()) * time.Second
		if err := waitForAllNodesReady(ctx, data.Kubeconfig.ValueString(), timeout); err != nil {
			resp.Diagnostics.AddError("Failed waiting for nodes to be ready", err.Error())
			return
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ClusterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ClusterResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	clusterName := data.Name.ValueString()

	clusters, err := r.provider.List()
	if err != nil {
		resp.Diagnostics.AddError("Failed to list clusters", err.Error())
		return
	}

	found := false
	for _, c := range clusters {
		if c == clusterName {
			found = true
			break
		}
	}

	if !found {
		resp.State.RemoveResource(ctx)
		return
	}

	r.populateComputedValues(&data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ClusterResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ClusterResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Populate computed values from the existing cluster
	r.populateComputedValues(&data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ClusterResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Clean up any stale lock files from previous interrupted operations
	cleanupStaleLockFile()

	var data ClusterResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	clusterName := data.Name.ValueString()

	err := r.provider.Delete(clusterName, "")
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete cluster", err.Error())
		return
	}
}

func (r *ClusterResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}

func (r *ClusterResource) buildClusterConfig(data *ClusterResourceModel) *v1alpha4.Cluster {
	cfg := &v1alpha4.Cluster{
		TypeMeta: v1alpha4.TypeMeta{
			Kind:       "Cluster",
			APIVersion: "kind.x-k8s.io/v1alpha4",
		},
		Name: data.Name.ValueString(),
	}

	// Networking configuration
	if data.Networking != nil {
		cfg.Networking = r.buildNetworkingConfig(data.Networking)
	}

	// Feature gates
	if !data.FeatureGates.IsNull() && len(data.FeatureGates.Elements()) > 0 {
		featureGates := make(map[string]bool)
		for k, v := range data.FeatureGates.Elements() {
			if boolVal, ok := v.(types.Bool); ok && !boolVal.IsNull() {
				featureGates[k] = boolVal.ValueBool()
			}
		}
		cfg.FeatureGates = featureGates
	}

	// Runtime config
	if !data.RuntimeConfig.IsNull() && len(data.RuntimeConfig.Elements()) > 0 {
		runtimeConfig := make(map[string]string)
		for k, v := range data.RuntimeConfig.Elements() {
			if strVal, ok := v.(types.String); ok && !strVal.IsNull() {
				runtimeConfig[k] = strVal.ValueString()
			}
		}
		cfg.RuntimeConfig = runtimeConfig
	}

	// Kubeadm config patches (merge patches)
	if !data.KubeadmConfigPatches.IsNull() && len(data.KubeadmConfigPatches.Elements()) > 0 {
		patches := make([]string, 0, len(data.KubeadmConfigPatches.Elements()))
		for _, elem := range data.KubeadmConfigPatches.Elements() {
			if strVal, ok := elem.(types.String); ok && !strVal.IsNull() {
				patches = append(patches, strVal.ValueString())
			}
		}
		cfg.KubeadmConfigPatches = patches
	}

	// Kubeadm config patches (JSON6902)
	if len(data.KubeadmConfigPatchesJSON6902) > 0 {
		patches := make([]v1alpha4.PatchJSON6902, len(data.KubeadmConfigPatchesJSON6902))
		for i, p := range data.KubeadmConfigPatchesJSON6902 {
			patches[i] = v1alpha4.PatchJSON6902{
				Group:   p.Group.ValueString(),
				Version: p.Version.ValueString(),
				Kind:    p.Kind.ValueString(),
				Patch:   p.Patch.ValueString(),
			}
		}
		cfg.KubeadmConfigPatchesJSON6902 = patches
	}

	// Containerd config patches (TOML)
	if !data.ContainerdConfigPatches.IsNull() && len(data.ContainerdConfigPatches.Elements()) > 0 {
		patches := make([]string, 0, len(data.ContainerdConfigPatches.Elements()))
		for _, elem := range data.ContainerdConfigPatches.Elements() {
			if strVal, ok := elem.(types.String); ok && !strVal.IsNull() {
				patches = append(patches, strVal.ValueString())
			}
		}
		cfg.ContainerdConfigPatches = patches
	}

	// Containerd config patches (JSON6902)
	if !data.ContainerdConfigPatchesJSON6902.IsNull() && len(data.ContainerdConfigPatchesJSON6902.Elements()) > 0 {
		patches := make([]string, 0, len(data.ContainerdConfigPatchesJSON6902.Elements()))
		for _, elem := range data.ContainerdConfigPatchesJSON6902.Elements() {
			if strVal, ok := elem.(types.String); ok && !strVal.IsNull() {
				patches = append(patches, strVal.ValueString())
			}
		}
		cfg.ContainerdConfigPatchesJSON6902 = patches
	}

	// Nodes
	if len(data.Nodes) > 0 {
		cfg.Nodes = make([]v1alpha4.Node, len(data.Nodes))
		for i, node := range data.Nodes {
			cfg.Nodes[i] = r.buildNodeConfig(&node)
		}
	} else {
		cfg.Nodes = []v1alpha4.Node{
			{Role: v1alpha4.ControlPlaneRole},
			{Role: v1alpha4.WorkerRole},
		}
	}

	return cfg
}

func (r *ClusterResource) buildNetworkingConfig(net *NetworkingModel) v1alpha4.Networking {
	networking := v1alpha4.Networking{}

	if !net.IPFamily.IsNull() && net.IPFamily.ValueString() != "" {
		networking.IPFamily = v1alpha4.ClusterIPFamily(net.IPFamily.ValueString())
	}

	if !net.APIServerPort.IsNull() {
		networking.APIServerPort = int32(net.APIServerPort.ValueInt64())
	}

	if !net.APIServerAddress.IsNull() && net.APIServerAddress.ValueString() != "" {
		networking.APIServerAddress = net.APIServerAddress.ValueString()
	}

	if !net.PodSubnet.IsNull() && net.PodSubnet.ValueString() != "" {
		networking.PodSubnet = net.PodSubnet.ValueString()
	}

	if !net.ServiceSubnet.IsNull() && net.ServiceSubnet.ValueString() != "" {
		networking.ServiceSubnet = net.ServiceSubnet.ValueString()
	}

	if !net.DisableDefaultCNI.IsNull() {
		networking.DisableDefaultCNI = net.DisableDefaultCNI.ValueBool()
	}

	if !net.KubeProxyMode.IsNull() && net.KubeProxyMode.ValueString() != "" {
		networking.KubeProxyMode = v1alpha4.ProxyMode(net.KubeProxyMode.ValueString())
	}

	if !net.DNSSearch.IsNull() && len(net.DNSSearch.Elements()) > 0 {
		dnsSearch := make([]string, 0, len(net.DNSSearch.Elements()))
		for _, elem := range net.DNSSearch.Elements() {
			if strVal, ok := elem.(types.String); ok && !strVal.IsNull() {
				dnsSearch = append(dnsSearch, strVal.ValueString())
			}
		}
		networking.DNSSearch = &dnsSearch
	}

	return networking
}

func (r *ClusterResource) buildNodeConfig(node *NodeModel) v1alpha4.Node {
	n := v1alpha4.Node{}

	if !node.Role.IsNull() {
		switch node.Role.ValueString() {
		case "control-plane":
			n.Role = v1alpha4.ControlPlaneRole
		case "worker":
			n.Role = v1alpha4.WorkerRole
		}
	}

	if !node.Image.IsNull() && node.Image.ValueString() != "" {
		n.Image = node.Image.ValueString()
	}

	if !node.Labels.IsNull() {
		labels := make(map[string]string)
		for k, v := range node.Labels.Elements() {
			if strVal, ok := v.(types.String); ok {
				labels[k] = strVal.ValueString()
			}
		}
		n.Labels = labels
	}

	// Kubeadm config patches (merge patches) for this node
	if !node.KubeadmConfigPatches.IsNull() && len(node.KubeadmConfigPatches.Elements()) > 0 {
		patches := make([]string, 0, len(node.KubeadmConfigPatches.Elements()))
		for _, elem := range node.KubeadmConfigPatches.Elements() {
			if strVal, ok := elem.(types.String); ok && !strVal.IsNull() {
				patches = append(patches, strVal.ValueString())
			}
		}
		n.KubeadmConfigPatches = patches
	}

	// Kubeadm config patches (JSON6902) for this node
	if len(node.KubeadmConfigPatchesJSON6902) > 0 {
		patches := make([]v1alpha4.PatchJSON6902, len(node.KubeadmConfigPatchesJSON6902))
		for i, p := range node.KubeadmConfigPatchesJSON6902 {
			patches[i] = v1alpha4.PatchJSON6902{
				Group:   p.Group.ValueString(),
				Version: p.Version.ValueString(),
				Kind:    p.Kind.ValueString(),
				Patch:   p.Patch.ValueString(),
			}
		}
		n.KubeadmConfigPatchesJSON6902 = patches
	}

	// Extra mounts
	if len(node.ExtraMounts) > 0 {
		n.ExtraMounts = make([]v1alpha4.Mount, len(node.ExtraMounts))
		for i, mount := range node.ExtraMounts {
			n.ExtraMounts[i] = v1alpha4.Mount{
				HostPath:       mount.HostPath.ValueString(),
				ContainerPath:  mount.ContainerPath.ValueString(),
				Readonly:       mount.ReadOnly.ValueBool(),
				SelinuxRelabel: mount.SelinuxRelabel.ValueBool(),
			}
			if !mount.Propagation.IsNull() {
				n.ExtraMounts[i].Propagation = v1alpha4.MountPropagation(mount.Propagation.ValueString())
			}
		}
	}

	// Extra port mappings
	if len(node.ExtraPortMappings) > 0 {
		n.ExtraPortMappings = make([]v1alpha4.PortMapping, len(node.ExtraPortMappings))
		for i, pm := range node.ExtraPortMappings {
			n.ExtraPortMappings[i] = v1alpha4.PortMapping{
				ContainerPort: int32(pm.ContainerPort.ValueInt64()),
				HostPort:      int32(pm.HostPort.ValueInt64()),
			}
			if !pm.ListenAddress.IsNull() {
				n.ExtraPortMappings[i].ListenAddress = pm.ListenAddress.ValueString()
			}
			if !pm.Protocol.IsNull() {
				n.ExtraPortMappings[i].Protocol = v1alpha4.PortMappingProtocol(pm.Protocol.ValueString())
			}
		}
	}

	return n
}

func (r *ClusterResource) populateComputedValues(data *ClusterResourceModel, diagnostics *diag.Diagnostics) {
	clusterName := data.Name.ValueString()

	data.ID = types.StringValue(clusterName)

	kubeconfig, err := r.provider.KubeConfig(clusterName, false)
	if err != nil {
		diagnostics.AddError("Failed to get kubeconfig", err.Error())
		return
	}
	data.Kubeconfig = types.StringValue(kubeconfig)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		diagnostics.AddError("Failed to get home directory", err.Error())
		return
	}
	kubeconfigPath := filepath.Join(homeDir, ".kube", "kind", "kind-"+clusterName)
	data.KubeconfigPath = types.StringValue(kubeconfigPath)

	var kubeconfigData map[string]interface{}
	if err := yaml.Unmarshal([]byte(kubeconfig), &kubeconfigData); err != nil {
		diagnostics.AddError("Failed to parse kubeconfig", err.Error())
		return
	}

	if clusters, ok := kubeconfigData["clusters"].([]interface{}); ok && len(clusters) > 0 {
		if clusterData, ok := clusters[0].(map[string]interface{}); ok {
			if clusterInfo, ok := clusterData["cluster"].(map[string]interface{}); ok {
				if server, ok := clusterInfo["server"].(string); ok {
					data.Endpoint = types.StringValue(server)
				}
				if caData, ok := clusterInfo["certificate-authority-data"].(string); ok {
					data.ClusterCaCertificate = types.StringValue(caData)
				}
			}
		}
	}

	if users, ok := kubeconfigData["users"].([]interface{}); ok && len(users) > 0 {
		if userData, ok := users[0].(map[string]interface{}); ok {
			if userInfo, ok := userData["user"].(map[string]interface{}); ok {
				if certData, ok := userInfo["client-certificate-data"].(string); ok {
					data.ClientCertificate = types.StringValue(certData)
				}
				if keyData, ok := userInfo["client-key-data"].(string); ok {
					data.ClientKey = types.StringValue(keyData)
				}
			}
		}
	}

	if data.Endpoint.IsNull() {
		data.Endpoint = types.StringValue("")
	}
	if data.ClusterCaCertificate.IsNull() {
		data.ClusterCaCertificate = types.StringValue("")
	}
	if data.ClientCertificate.IsNull() {
		data.ClientCertificate = types.StringValue("")
	}
	if data.ClientKey.IsNull() {
		data.ClientKey = types.StringValue("")
	}
}
