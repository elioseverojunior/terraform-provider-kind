// SPDX-FileCopyrightText: 2026 Elio Severo Junior <elioseverojunior@gmail.com>
//
// SPDX-License-Identifier: MIT OR Apache-2.0

package provider

import (
	"context"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// providerSchema returns the provider-level schema under test.
func providerSchema(t *testing.T) schema.Schema {
	t.Helper()

	resp := &provider.SchemaResponse{}
	New("test")().Schema(context.Background(), provider.SchemaRequest{}, resp)
	require.False(t, resp.Diagnostics.HasError())

	return resp.Schema
}

// providerConfig builds a tfsdk.Config carrying the given host value.
//
// tfsdk.Config is read-only, so the raw value is produced by round-tripping the
// model through a tfsdk.State that shares the same schema. That keeps this
// helper working unchanged if the provider schema gains attributes.
func providerConfig(t *testing.T, host types.String) tfsdk.Config {
	t.Helper()

	s := providerSchema(t)
	state := tfsdk.State{
		Schema: s,
		Raw:    tftypes.NewValue(s.Type().TerraformType(context.Background()), nil),
	}
	require.False(t, state.Set(context.Background(), &KindProviderModel{Host: host}).HasError())

	return tfsdk.Config{Schema: s, Raw: state.Raw}
}

func TestProvider_MetadataReportsTypeNameAndVersion(t *testing.T) {
	t.Parallel()

	resp := &provider.MetadataResponse{}
	New("1.2.3")().Metadata(context.Background(), provider.MetadataRequest{}, resp)

	assert.Equal(t, "kind", resp.TypeName)
	assert.Equal(t, "1.2.3", resp.Version)
}

func TestProvider_SchemaExposesOptionalHost(t *testing.T) {
	t.Parallel()

	host, ok := providerSchema(t).Attributes["host"].(schema.StringAttribute)
	require.True(t, ok, "host must be a StringAttribute")

	assert.True(t, host.Optional, "host must be optional")
	assert.False(t, host.Required, "host must not be required")
	assert.NotEmpty(t, host.Description, "host must be documented")
}

func TestProvider_RegistersClusterResourceAndClustersDataSource(t *testing.T) {
	t.Parallel()

	p := New("test")()

	resources := p.Resources(context.Background())
	require.Len(t, resources, 1)
	assert.IsType(t, &ClusterResource{}, resources[0]())

	dataSources := p.DataSources(context.Background())
	require.Len(t, dataSources, 1)
	assert.IsType(t, &ClustersDataSource{}, dataSources[0]())
}

// A configured host must reach kind through DOCKER_HOST, which is the only
// channel kind's Docker node provider reads.
func TestProvider_ConfigureExportsHostAsDockerHost(t *testing.T) {
	t.Setenv("DOCKER_HOST", "")

	resp := &provider.ConfigureResponse{}
	New("test")().Configure(
		context.Background(),
		provider.ConfigureRequest{Config: providerConfig(t, types.StringValue("tcp://127.0.0.1:2375"))},
		resp,
	)

	require.False(t, resp.Diagnostics.HasError(), "diagnostics: %v", resp.Diagnostics)
	assert.Equal(t, "tcp://127.0.0.1:2375", os.Getenv("DOCKER_HOST"))
	assert.NotNil(t, resp.ResourceData, "resource data must carry the cluster provider")
	assert.NotNil(t, resp.DataSourceData, "data source data must carry the cluster provider")
}

// Omitting host must leave an operator's existing DOCKER_HOST untouched.
func TestProvider_ConfigureLeavesDockerHostAloneWhenHostUnset(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix:///pre/existing.sock")

	tests := []struct {
		name string
		host types.String
	}{
		{name: "null host", host: types.StringNull()},
		{name: "empty host", host: types.StringValue("")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &provider.ConfigureResponse{}
			New("test")().Configure(
				context.Background(),
				provider.ConfigureRequest{Config: providerConfig(t, tt.host)},
				resp,
			)

			require.False(t, resp.Diagnostics.HasError())
			assert.Equal(t, "unix:///pre/existing.sock", os.Getenv("DOCKER_HOST"))
		})
	}
}

// The provider hands the same cluster manager to both resources and data
// sources; it must satisfy the interface they type-assert against.
func TestProvider_ConfigureSuppliesAClusterManager(t *testing.T) {
	t.Setenv("DOCKER_HOST", "")

	resp := &provider.ConfigureResponse{}
	New("test")().Configure(
		context.Background(),
		provider.ConfigureRequest{Config: providerConfig(t, types.StringNull())},
		resp,
	)

	require.False(t, resp.Diagnostics.HasError())

	_, ok := resp.ResourceData.(clusterManager)
	assert.True(t, ok, "ResourceData must satisfy clusterManager")

	_, ok = resp.DataSourceData.(clusterManager)
	assert.True(t, ok, "DataSourceData must satisfy clusterManager")
}

// If the framework hands over a config the provider model cannot decode, the
// provider must report it and leave resources unconfigured rather than
// continuing with a zero-valued config.
func TestProvider_ConfigureBailsOutOnUnreadableConfig(t *testing.T) {
	t.Parallel()

	// A schema that does not match KindProviderModel forces the decode to fail.
	mismatched := tfsdk.Config{
		Schema: dataSourceSchema(t),
		Raw: tftypes.NewValue(
			dataSourceSchema(t).Type().TerraformType(context.Background()),
			nil,
		),
	}

	resp := &provider.ConfigureResponse{}
	New("test")().Configure(context.Background(), provider.ConfigureRequest{Config: mismatched}, resp)

	require.True(t, resp.Diagnostics.HasError())
	assert.Nil(t, resp.ResourceData, "resources must not be configured from an unreadable config")
	assert.Nil(t, resp.DataSourceData, "data sources must not be configured from an unreadable config")
}
