// SPDX-FileCopyrightText: 2026 Elio Severo Junior <elioseverojunior@gmail.com>
//
// SPDX-License-Identifier: MIT OR Apache-2.0

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// emptyDataSourceState returns a null-but-typed state for the data source.
func emptyDataSourceState(t *testing.T) tfsdk.State {
	t.Helper()

	s := dataSourceSchema(t)

	return tfsdk.State{
		Schema: s,
		Raw:    tftypes.NewValue(s.Type().TerraformType(context.Background()), nil),
	}
}

func TestClustersDataSource_Metadata(t *testing.T) {
	t.Parallel()

	resp := &datasource.MetadataResponse{}
	NewClustersDataSource().Metadata(context.Background(), datasource.MetadataRequest{ProviderTypeName: "kind"}, resp)

	assert.Equal(t, "kind_clusters", resp.TypeName)
}

func TestClustersDataSource_SchemaIsFullyComputed(t *testing.T) {
	t.Parallel()

	s := dataSourceSchema(t)

	id, ok := s.Attributes["id"].(schema.StringAttribute)
	require.True(t, ok)
	assert.True(t, id.Computed)

	clusters, ok := s.Attributes["clusters"].(schema.ListAttribute)
	require.True(t, ok)
	assert.True(t, clusters.Computed)
	assert.NotEmpty(t, clusters.Description)
}

func TestClustersDataSource_ConfigureWithNilProviderDataIsNoop(t *testing.T) {
	t.Parallel()

	d := newClustersDataSourceT(t)
	resp := &datasource.ConfigureResponse{}
	d.Configure(context.Background(), datasource.ConfigureRequest{ProviderData: nil}, resp)

	assert.False(t, resp.Diagnostics.HasError())
	assert.Nil(t, d.provider)
}

func TestClustersDataSource_ConfigureRejectsUnexpectedType(t *testing.T) {
	t.Parallel()

	d := newClustersDataSourceT(t)
	resp := &datasource.ConfigureResponse{}
	d.Configure(context.Background(), datasource.ConfigureRequest{ProviderData: 42}, resp)

	require.True(t, resp.Diagnostics.HasError())
	assert.Contains(t, resp.Diagnostics.Errors()[0].Summary(), "Unexpected Data Source Configure Type")
}

func TestClustersDataSource_ReadReturnsClusterNames(t *testing.T) {
	t.Parallel()

	d := newConfiguredDataSource(t, &fakeClusterManager{clusters: []string{"alpha", "beta"}})

	resp := &datasource.ReadResponse{State: emptyDataSourceState(t)}
	d.Read(context.Background(), datasource.ReadRequest{}, resp)

	require.False(t, resp.Diagnostics.HasError(), "diagnostics: %v", resp.Diagnostics)

	var got ClustersDataSourceModel
	require.False(t, resp.State.Get(context.Background(), &got).HasError())
	assert.Equal(t, "kind-clusters", got.ID.ValueString())
	require.Len(t, got.Clusters, 2)
	assert.Equal(t, "alpha", got.Clusters[0].ValueString())
	assert.Equal(t, "beta", got.Clusters[1].ValueString())
}

// With no clusters the data source must return an empty list rather than null,
// so that length() and for_each keep working in practitioner configurations.
func TestClustersDataSource_ReadReturnsEmptyListWhenNoClusters(t *testing.T) {
	t.Parallel()

	d := newConfiguredDataSource(t, &fakeClusterManager{})

	resp := &datasource.ReadResponse{State: emptyDataSourceState(t)}
	d.Read(context.Background(), datasource.ReadRequest{}, resp)

	require.False(t, resp.Diagnostics.HasError())

	var got ClustersDataSourceModel
	require.False(t, resp.State.Get(context.Background(), &got).HasError())
	assert.Empty(t, got.Clusters)
}

func TestClustersDataSource_ReadSurfacesListError(t *testing.T) {
	t.Parallel()

	d := newConfiguredDataSource(t, &fakeClusterManager{listErr: errBoom})

	resp := &datasource.ReadResponse{State: emptyDataSourceState(t)}
	d.Read(context.Background(), datasource.ReadRequest{}, resp)

	require.True(t, resp.Diagnostics.HasError())
	assert.Equal(t, "Failed to list clusters", resp.Diagnostics.Errors()[0].Summary())
	assert.Contains(t, resp.Diagnostics.Errors()[0].Detail(), "boom")
}
