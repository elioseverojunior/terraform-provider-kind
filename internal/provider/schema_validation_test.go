// SPDX-FileCopyrightText: 2026 Elio Severo Junior <elioseverojunior@gmail.com>
//
// SPDX-License-Identifier: MIT OR Apache-2.0

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clusterSchema returns the kind_cluster resource schema under test.
func clusterSchema(t *testing.T) schema.Schema {
	t.Helper()

	resp := &resource.SchemaResponse{}
	newClusterResourceT(t).Schema(context.Background(), resource.SchemaRequest{}, resp)
	require.False(t, resp.Diagnostics.HasError(), "schema produced diagnostics: %v", resp.Diagnostics)

	return resp.Schema
}

// nodeBlockAttr returns a string attribute declared on the node block.
func nodeBlockAttr(t *testing.T, name string) schema.StringAttribute {
	t.Helper()

	node, ok := clusterSchema(t).Blocks["node"].(schema.ListNestedBlock)
	require.True(t, ok, "node block should be a ListNestedBlock")

	attr, ok := node.NestedObject.Attributes[name].(schema.StringAttribute)
	require.True(t, ok, "node.%s should be a StringAttribute", name)

	return attr
}

// networkingAttr returns a string attribute declared on the networking block.
func networkingAttr(t *testing.T, name string) schema.StringAttribute {
	t.Helper()

	net, ok := clusterSchema(t).Blocks["networking"].(schema.SingleNestedBlock)
	require.True(t, ok, "networking block should be a SingleNestedBlock")

	attr, ok := net.Attributes[name].(schema.StringAttribute)
	require.True(t, ok, "networking.%s should be a StringAttribute", name)

	return attr
}

// runStringValidators feeds a value through every validator on an attribute and
// reports whether the value was rejected.
func runStringValidators(t *testing.T, validators []validator.String, value string) bool {
	t.Helper()

	req := validator.StringRequest{ConfigValue: types.StringValue(value)}
	resp := &validator.StringResponse{}
	for _, v := range validators {
		v.ValidateString(context.Background(), req, resp)
	}

	return resp.Diagnostics.HasError()
}

// A typo such as role = "workers" must be caught at plan time with a message
// naming the valid roles, rather than reaching kind and failing mid-create with
// an opaque error. The valid values also render into the generated docs.
func TestClusterSchema_NodeRoleRejectsUnknownValues(t *testing.T) {
	t.Parallel()

	role := nodeBlockAttr(t, "role")
	require.NotEmpty(t, role.Validators, "node.role must declare validators")

	assert.False(t, runStringValidators(t, role.Validators, "control-plane"), "control-plane must be accepted")
	assert.False(t, runStringValidators(t, role.Validators, "worker"), "worker must be accepted")
	assert.True(t, runStringValidators(t, role.Validators, "workers"), "workers must be rejected")
	assert.True(t, runStringValidators(t, role.Validators, ""), "empty role must be rejected")
}

func TestClusterSchema_NetworkingEnumsAreConstrained(t *testing.T) {
	t.Parallel()

	tests := []struct {
		attribute string
		valid     []string
		invalid   []string
	}{
		{attribute: "ip_family", valid: []string{"ipv4", "ipv6", "dual"}, invalid: []string{"ipv5", "IPv4"}},
		{attribute: "kube_proxy_mode", valid: []string{"iptables", "ipvs", "nftables", "none"}, invalid: []string{"eBPF"}},
	}

	for _, tt := range tests {
		t.Run(tt.attribute, func(t *testing.T) {
			t.Parallel()

			attr := networkingAttr(t, tt.attribute)
			require.NotEmpty(t, attr.Validators, "networking.%s must declare validators", tt.attribute)

			for _, v := range tt.valid {
				assert.False(t, runStringValidators(t, attr.Validators, v), "%q must be accepted", v)
			}
			for _, v := range tt.invalid {
				assert.True(t, runStringValidators(t, attr.Validators, v), "%q must be rejected", v)
			}
		})
	}
}

func TestClusterSchema_PortMappingAndMountEnumsAreConstrained(t *testing.T) {
	t.Parallel()

	node, ok := clusterSchema(t).Blocks["node"].(schema.ListNestedBlock)
	require.True(t, ok)

	portMappings, ok := node.NestedObject.Blocks["extra_port_mappings"].(schema.ListNestedBlock)
	require.True(t, ok)
	protocol, ok := portMappings.NestedObject.Attributes["protocol"].(schema.StringAttribute)
	require.True(t, ok)
	require.NotEmpty(t, protocol.Validators, "protocol must declare validators")
	assert.False(t, runStringValidators(t, protocol.Validators, "TCP"))
	assert.False(t, runStringValidators(t, protocol.Validators, "UDP"))
	assert.False(t, runStringValidators(t, protocol.Validators, "SCTP"))
	assert.True(t, runStringValidators(t, protocol.Validators, "tcp"), "protocol is case-sensitive in kind")

	mounts, ok := node.NestedObject.Blocks["extra_mounts"].(schema.ListNestedBlock)
	require.True(t, ok)
	propagation, ok := mounts.NestedObject.Attributes["propagation"].(schema.StringAttribute)
	require.True(t, ok)
	require.NotEmpty(t, propagation.Validators, "propagation must declare validators")
	assert.False(t, runStringValidators(t, propagation.Validators, "None"))
	assert.False(t, runStringValidators(t, propagation.Validators, "HostToContainer"))
	assert.False(t, runStringValidators(t, propagation.Validators, "Bidirectional"))
	assert.True(t, runStringValidators(t, propagation.Validators, "bidirectional"))
}

// The cluster name reaches Docker as a container-name component, so it must be
// constrained to what Docker and kind actually accept.
func TestClusterSchema_NameIsConstrained(t *testing.T) {
	t.Parallel()

	name, ok := clusterSchema(t).Attributes["name"].(schema.StringAttribute)
	require.True(t, ok)
	require.NotEmpty(t, name.Validators, "name must declare validators")

	assert.False(t, runStringValidators(t, name.Validators, "my-cluster"))
	assert.False(t, runStringValidators(t, name.Validators, "cluster1"))
	assert.True(t, runStringValidators(t, name.Validators, ""), "empty name must be rejected")
	assert.True(t, runStringValidators(t, name.Validators, "My_Cluster"), "underscores/uppercase must be rejected")
}
