// SPDX-FileCopyrightText: 2026 Elio Severo Junior <elioseverojunior@gmail.com>
//
// SPDX-License-Identifier: MIT OR Apache-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/kind/pkg/apis/config/v1alpha4"
)

// strList builds a types.List of strings for table-driven test fixtures.
func strList(values ...string) types.List {
	elems := make([]attr.Value, len(values))
	for i, v := range values {
		elems[i] = types.StringValue(v)
	}
	return types.ListValueMust(types.StringType, elems)
}

// strMap builds a types.Map of strings for table-driven test fixtures.
func strMap(kv map[string]string) types.Map {
	elems := make(map[string]attr.Value, len(kv))
	for k, v := range kv {
		elems[k] = types.StringValue(v)
	}
	return types.MapValueMust(types.StringType, elems)
}

// boolMap builds a types.Map of bools for table-driven test fixtures.
func boolMap(kv map[string]bool) types.Map {
	elems := make(map[string]attr.Value, len(kv))
	for k, v := range kv {
		elems[k] = types.BoolValue(v)
	}
	return types.MapValueMust(types.BoolType, elems)
}

// nullModel returns a ClusterResourceModel with every collection attribute
// explicitly null, matching what the framework hands us when a practitioner
// omits the corresponding argument.
func nullModel(name string) *ClusterResourceModel {
	return &ClusterResourceModel{
		Name:                            types.StringValue(name),
		FeatureGates:                    types.MapNull(types.BoolType),
		RuntimeConfig:                   types.MapNull(types.StringType),
		KubeadmConfigPatches:            types.ListNull(types.StringType),
		ContainerdConfigPatches:         types.ListNull(types.StringType),
		ContainerdConfigPatchesJSON6902: types.ListNull(types.StringType),
	}
}

func TestBuildClusterConfig_TypeMetaAndName(t *testing.T) {
	t.Parallel()

	// Dots and hyphens are both legal in kind cluster names.
	const name = "team.ci-2"

	r := &ClusterResource{}
	cfg := r.buildClusterConfig(nullModel(name))

	require.NotNil(t, cfg)
	assert.Equal(t, "Cluster", cfg.Kind)
	assert.Equal(t, "kind.x-k8s.io/v1alpha4", cfg.APIVersion)
	assert.Equal(t, name, cfg.Name)
}

// A cluster with no node blocks must fall back to kind's documented default
// topology of one control-plane plus one worker. The README and the resource
// schema both promise this, so it is pinned by a test.
func TestBuildClusterConfig_DefaultTopologyIsOneControlPlaneOneWorker(t *testing.T) {
	t.Parallel()

	r := &ClusterResource{}
	cfg := r.buildClusterConfig(nullModel("demo"))

	require.Len(t, cfg.Nodes, 2)
	assert.Equal(t, v1alpha4.ControlPlaneRole, cfg.Nodes[0].Role)
	assert.Equal(t, v1alpha4.WorkerRole, cfg.Nodes[1].Role)
}

func TestBuildClusterConfig_NullCollectionsProduceEmptyConfig(t *testing.T) {
	t.Parallel()

	r := &ClusterResource{}
	cfg := r.buildClusterConfig(nullModel("demo"))

	assert.Nil(t, cfg.FeatureGates)
	assert.Nil(t, cfg.RuntimeConfig)
	assert.Nil(t, cfg.KubeadmConfigPatches)
	assert.Nil(t, cfg.KubeadmConfigPatchesJSON6902)
	assert.Nil(t, cfg.ContainerdConfigPatches)
	assert.Nil(t, cfg.ContainerdConfigPatchesJSON6902)
	assert.Equal(t, v1alpha4.Networking{}, cfg.Networking)
}

func TestBuildClusterConfig_FeatureGatesAndRuntimeConfig(t *testing.T) {
	t.Parallel()

	data := nullModel("demo")
	data.FeatureGates = boolMap(map[string]bool{"EphemeralContainers": true, "WindowsGMSA": false})
	data.RuntimeConfig = strMap(map[string]string{"api/beta": "true"})

	r := &ClusterResource{}
	cfg := r.buildClusterConfig(data)

	assert.Equal(t, map[string]bool{"EphemeralContainers": true, "WindowsGMSA": false}, cfg.FeatureGates)
	assert.Equal(t, map[string]string{"api/beta": "true"}, cfg.RuntimeConfig)
}

func TestBuildClusterConfig_AllPatchFlavours(t *testing.T) {
	t.Parallel()

	data := nullModel("demo")
	data.KubeadmConfigPatches = strList("kind: InitConfiguration")
	data.ContainerdConfigPatches = strList("[plugins]")
	data.ContainerdConfigPatchesJSON6902 = strList(`[{"op":"add"}]`)
	data.KubeadmConfigPatchesJSON6902 = []PatchJSON6902Model{{
		Group:   types.StringValue("kubeadm.k8s.io"),
		Version: types.StringValue("v1beta3"),
		Kind:    types.StringValue("ClusterConfiguration"),
		Patch:   types.StringValue(`[{"op":"add","path":"/x","value":"y"}]`),
	}}

	r := &ClusterResource{}
	cfg := r.buildClusterConfig(data)

	assert.Equal(t, []string{"kind: InitConfiguration"}, cfg.KubeadmConfigPatches)
	assert.Equal(t, []string{"[plugins]"}, cfg.ContainerdConfigPatches)
	assert.Equal(t, []string{`[{"op":"add"}]`}, cfg.ContainerdConfigPatchesJSON6902)
	require.Len(t, cfg.KubeadmConfigPatchesJSON6902, 1)
	assert.Equal(t, v1alpha4.PatchJSON6902{
		Group:   "kubeadm.k8s.io",
		Version: "v1beta3",
		Kind:    "ClusterConfiguration",
		Patch:   `[{"op":"add","path":"/x","value":"y"}]`,
	}, cfg.KubeadmConfigPatchesJSON6902[0])
}

// Null elements inside an otherwise populated list must be dropped rather than
// forwarded to kind as empty strings, which kind would reject at parse time.
func TestBuildClusterConfig_NullElementsAreSkipped(t *testing.T) {
	t.Parallel()

	data := nullModel("demo")
	data.KubeadmConfigPatches = types.ListValueMust(types.StringType, []attr.Value{
		types.StringValue("keep"),
		types.StringNull(),
	})
	data.FeatureGates = types.MapValueMust(types.BoolType, map[string]attr.Value{
		"Keep": types.BoolValue(true),
		"Drop": types.BoolNull(),
	})

	r := &ClusterResource{}
	cfg := r.buildClusterConfig(data)

	assert.Equal(t, []string{"keep"}, cfg.KubeadmConfigPatches)
	assert.Equal(t, map[string]bool{"Keep": true}, cfg.FeatureGates)
}

func TestBuildClusterConfig_ExplicitNodesReplaceDefaults(t *testing.T) {
	t.Parallel()

	data := nullModel("demo")
	data.Nodes = []NodeModel{
		{Role: types.StringValue("control-plane"), Labels: types.MapNull(types.StringType), KubeadmConfigPatches: types.ListNull(types.StringType)},
		{Role: types.StringValue("worker"), Labels: types.MapNull(types.StringType), KubeadmConfigPatches: types.ListNull(types.StringType)},
		{Role: types.StringValue("worker"), Labels: types.MapNull(types.StringType), KubeadmConfigPatches: types.ListNull(types.StringType)},
	}

	r := &ClusterResource{}
	cfg := r.buildClusterConfig(data)

	require.Len(t, cfg.Nodes, 3)
	assert.Equal(t, v1alpha4.ControlPlaneRole, cfg.Nodes[0].Role)
	assert.Equal(t, v1alpha4.WorkerRole, cfg.Nodes[1].Role)
	assert.Equal(t, v1alpha4.WorkerRole, cfg.Nodes[2].Role)
}

func TestBuildNetworkingConfig(t *testing.T) {
	t.Parallel()

	nullNet := func() *NetworkingModel {
		return &NetworkingModel{DNSSearch: types.ListNull(types.StringType)}
	}

	tests := []struct {
		name   string
		mutate func(*NetworkingModel)
		expect func(*testing.T, v1alpha4.Networking)
	}{
		{
			name:   "all null yields zero value",
			mutate: func(*NetworkingModel) {},
			expect: func(t *testing.T, n v1alpha4.Networking) {
				assert.Equal(t, v1alpha4.Networking{}, n)
			},
		},
		{
			name:   "ip family",
			mutate: func(n *NetworkingModel) { n.IPFamily = types.StringValue("dual") },
			expect: func(t *testing.T, n v1alpha4.Networking) {
				assert.Equal(t, v1alpha4.DualStackFamily, n.IPFamily)
			},
		},
		{
			// An empty string is treated as "unset" so a practitioner writing
			// ip_family = "" does not hand kind an invalid family.
			name:   "empty ip family is ignored",
			mutate: func(n *NetworkingModel) { n.IPFamily = types.StringValue("") },
			expect: func(t *testing.T, n v1alpha4.Networking) {
				assert.Equal(t, v1alpha4.ClusterIPFamily(""), n.IPFamily)
			},
		},
		{
			name:   "api server port",
			mutate: func(n *NetworkingModel) { n.APIServerPort = types.Int64Value(6443) },
			expect: func(t *testing.T, n v1alpha4.Networking) {
				assert.Equal(t, int32(6443), n.APIServerPort)
			},
		},
		{
			name:   "api server address",
			mutate: func(n *NetworkingModel) { n.APIServerAddress = types.StringValue("0.0.0.0") },
			expect: func(t *testing.T, n v1alpha4.Networking) {
				assert.Equal(t, "0.0.0.0", n.APIServerAddress)
			},
		},
		{
			name:   "pod subnet",
			mutate: func(n *NetworkingModel) { n.PodSubnet = types.StringValue("10.244.0.0/16") },
			expect: func(t *testing.T, n v1alpha4.Networking) {
				assert.Equal(t, "10.244.0.0/16", n.PodSubnet)
			},
		},
		{
			name:   "service subnet",
			mutate: func(n *NetworkingModel) { n.ServiceSubnet = types.StringValue("10.96.0.0/12") },
			expect: func(t *testing.T, n v1alpha4.Networking) {
				assert.Equal(t, "10.96.0.0/12", n.ServiceSubnet)
			},
		},
		{
			name:   "disable default cni",
			mutate: func(n *NetworkingModel) { n.DisableDefaultCNI = types.BoolValue(true) },
			expect: func(t *testing.T, n v1alpha4.Networking) {
				assert.True(t, n.DisableDefaultCNI)
			},
		},
		{
			name:   "kube proxy mode",
			mutate: func(n *NetworkingModel) { n.KubeProxyMode = types.StringValue("ipvs") },
			expect: func(t *testing.T, n v1alpha4.Networking) {
				assert.Equal(t, v1alpha4.IPVSProxyMode, n.KubeProxyMode)
			},
		},
		{
			name:   "dns search",
			mutate: func(n *NetworkingModel) { n.DNSSearch = strList("example.com", "svc.local") },
			expect: func(t *testing.T, n v1alpha4.Networking) {
				require.NotNil(t, n.DNSSearch)
				assert.Equal(t, []string{"example.com", "svc.local"}, *n.DNSSearch)
			},
		},
		{
			// An explicitly empty dns_search list must stay nil so kind keeps
			// its own default rather than receiving an empty override.
			name:   "empty dns search stays nil",
			mutate: func(n *NetworkingModel) { n.DNSSearch = strList() },
			expect: func(t *testing.T, n v1alpha4.Networking) {
				assert.Nil(t, n.DNSSearch)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			net := nullNet()
			tt.mutate(net)
			tt.expect(t, (&ClusterResource{}).buildNetworkingConfig(net))
		})
	}
}

func TestBuildClusterConfig_NetworkingBlockIsWiredIn(t *testing.T) {
	t.Parallel()

	data := nullModel("demo")
	data.Networking = &NetworkingModel{
		PodSubnet: types.StringValue("10.1.0.0/16"),
		DNSSearch: types.ListNull(types.StringType),
	}

	cfg := (&ClusterResource{}).buildClusterConfig(data)

	assert.Equal(t, "10.1.0.0/16", cfg.Networking.PodSubnet)
}
