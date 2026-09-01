// SPDX-FileCopyrightText: 2026 Elio Severo Junior <elioseverojunior@gmail.com>
//
// SPDX-License-Identifier: MIT OR Apache-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/kind/pkg/apis/config/v1alpha4"
)

// nullNode returns a NodeModel with collection attributes null, as the
// framework supplies them when the practitioner omits those arguments.
func nullNode(role string) *NodeModel {
	return &NodeModel{
		Role:                 types.StringValue(role),
		Labels:               types.MapNull(types.StringType),
		KubeadmConfigPatches: types.ListNull(types.StringType),
	}
}

func TestBuildNodeConfig_RoleMapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		role string
		want v1alpha4.NodeRole
	}{
		{name: "control-plane", role: "control-plane", want: v1alpha4.ControlPlaneRole},
		{name: "worker", role: "worker", want: v1alpha4.WorkerRole},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := (&ClusterResource{}).buildNodeConfig(nullNode(tt.role))
			assert.Equal(t, tt.want, got.Role)
		})
	}
}

func TestBuildNodeConfig_ImageOverride(t *testing.T) {
	t.Parallel()

	node := nullNode("worker")
	node.Image = types.StringValue("kindest/node:v1.34.0")

	got := (&ClusterResource{}).buildNodeConfig(node)
	assert.Equal(t, "kindest/node:v1.34.0", got.Image)
}

func TestBuildNodeConfig_EmptyImageIsIgnored(t *testing.T) {
	t.Parallel()

	node := nullNode("worker")
	node.Image = types.StringValue("")

	got := (&ClusterResource{}).buildNodeConfig(node)
	assert.Empty(t, got.Image)
}

func TestBuildNodeConfig_Labels(t *testing.T) {
	t.Parallel()

	node := nullNode("control-plane")
	node.Labels = strMap(map[string]string{"ingress-ready": "true"})

	got := (&ClusterResource{}).buildNodeConfig(node)
	assert.Equal(t, map[string]string{"ingress-ready": "true"}, got.Labels)
}

func TestBuildNodeConfig_KubeadmPatches(t *testing.T) {
	t.Parallel()

	node := nullNode("control-plane")
	node.KubeadmConfigPatches = strList("kind: InitConfiguration")
	node.KubeadmConfigPatchesJSON6902 = []PatchJSON6902Model{{
		Group:   types.StringValue("kubeadm.k8s.io"),
		Version: types.StringValue("v1beta3"),
		Kind:    types.StringValue("InitConfiguration"),
		Patch:   types.StringValue(`[{"op":"add"}]`),
	}}

	got := (&ClusterResource{}).buildNodeConfig(node)

	assert.Equal(t, []string{"kind: InitConfiguration"}, got.KubeadmConfigPatches)
	require.Len(t, got.KubeadmConfigPatchesJSON6902, 1)
	assert.Equal(t, "InitConfiguration", got.KubeadmConfigPatchesJSON6902[0].Kind)
}

func TestBuildNodeConfig_ExtraMounts(t *testing.T) {
	t.Parallel()

	node := nullNode("worker")
	node.ExtraMounts = []MountModel{{
		HostPath:       types.StringValue("/tmp/data"),
		ContainerPath:  types.StringValue("/data"),
		ReadOnly:       types.BoolValue(true),
		SelinuxRelabel: types.BoolValue(true),
		Propagation:    types.StringValue("HostToContainer"),
	}}

	got := (&ClusterResource{}).buildNodeConfig(node)

	require.Len(t, got.ExtraMounts, 1)
	assert.Equal(t, v1alpha4.Mount{
		HostPath:       "/tmp/data",
		ContainerPath:  "/data",
		Readonly:       true,
		SelinuxRelabel: true,
		Propagation:    v1alpha4.MountPropagationHostToContainer,
	}, got.ExtraMounts[0])
}

func TestBuildNodeConfig_ExtraMountsNullPropagation(t *testing.T) {
	t.Parallel()

	node := nullNode("worker")
	node.ExtraMounts = []MountModel{{
		HostPath:      types.StringValue("/tmp/data"),
		ContainerPath: types.StringValue("/data"),
		Propagation:   types.StringNull(),
	}}

	got := (&ClusterResource{}).buildNodeConfig(node)

	require.Len(t, got.ExtraMounts, 1)
	assert.Equal(t, v1alpha4.MountPropagation(""), got.ExtraMounts[0].Propagation)
	assert.False(t, got.ExtraMounts[0].Readonly)
}

func TestBuildNodeConfig_ExtraPortMappings(t *testing.T) {
	t.Parallel()

	node := nullNode("control-plane")
	node.ExtraPortMappings = []PortMappingModel{{
		ContainerPort: types.Int64Value(80),
		HostPort:      types.Int64Value(8080),
		ListenAddress: types.StringValue("0.0.0.0"),
		Protocol:      types.StringValue("TCP"),
	}}

	got := (&ClusterResource{}).buildNodeConfig(node)

	require.Len(t, got.ExtraPortMappings, 1)
	assert.Equal(t, v1alpha4.PortMapping{
		ContainerPort: 80,
		HostPort:      8080,
		ListenAddress: "0.0.0.0",
		Protocol:      v1alpha4.PortMappingProtocolTCP,
	}, got.ExtraPortMappings[0])
}

func TestBuildNodeConfig_ExtraPortMappingsOptionalFieldsNull(t *testing.T) {
	t.Parallel()

	node := nullNode("control-plane")
	node.ExtraPortMappings = []PortMappingModel{{
		ContainerPort: types.Int64Value(443),
		HostPort:      types.Int64Value(8443),
		ListenAddress: types.StringNull(),
		Protocol:      types.StringNull(),
	}}

	got := (&ClusterResource{}).buildNodeConfig(node)

	require.Len(t, got.ExtraPortMappings, 1)
	assert.Empty(t, got.ExtraPortMappings[0].ListenAddress)
	assert.Equal(t, v1alpha4.PortMappingProtocol(""), got.ExtraPortMappings[0].Protocol)
}
