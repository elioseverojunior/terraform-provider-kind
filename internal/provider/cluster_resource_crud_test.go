// SPDX-FileCopyrightText: 2026 Elio Severo Junior <elioseverojunior@gmail.com>
//
// SPDX-License-Identifier: MIT OR Apache-2.0

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClusterResource_Metadata(t *testing.T) {
	t.Parallel()

	resp := &resource.MetadataResponse{}
	NewClusterResource().Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "kind"}, resp)

	assert.Equal(t, "kind_cluster", resp.TypeName)
}

func TestClusterResource_ConfigureWithNilProviderDataIsNoop(t *testing.T) {
	t.Parallel()

	r := newClusterResourceT(t)
	resp := &resource.ConfigureResponse{}
	r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: nil}, resp)

	assert.False(t, resp.Diagnostics.HasError())
	assert.Nil(t, r.provider)
}

func TestClusterResource_ConfigureRejectsUnexpectedType(t *testing.T) {
	t.Parallel()

	r := newClusterResourceT(t)
	resp := &resource.ConfigureResponse{}
	r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: "not-a-provider"}, resp)

	require.True(t, resp.Diagnostics.HasError())
	assert.Contains(t, resp.Diagnostics.Errors()[0].Summary(), "Unexpected Resource Configure Type")
}

func TestClusterResource_ConfigureAcceptsClusterManager(t *testing.T) {
	t.Parallel()

	mgr := &fakeClusterManager{}
	r := newConfiguredResource(t, mgr)

	assert.NotNil(t, r.provider)
}

func TestClusterResource_CreateSuccess(t *testing.T) {
	t.Parallel()

	mgr := &fakeClusterManager{kubeconfig: validKubeconfig}
	r := newConfiguredResource(t, mgr)

	resp := &resource.CreateResponse{State: emptyResourceState(t)}
	r.Create(context.Background(), resource.CreateRequest{Plan: planWith(t, baseModel("demo"))}, resp)

	require.False(t, resp.Diagnostics.HasError(), "diagnostics: %v", resp.Diagnostics)
	assert.Equal(t, "demo", mgr.createdName)

	var got ClusterResourceModel
	require.False(t, resp.State.Get(context.Background(), &got).HasError())
	assert.Equal(t, "demo", got.ID.ValueString())
	assert.Equal(t, "https://127.0.0.1:6443", got.Endpoint.ValueString())
	assert.Equal(t, validKubeconfig, got.Kubeconfig.ValueString())
}

func TestClusterResource_CreateSurfacesProviderError(t *testing.T) {
	t.Parallel()

	mgr := &fakeClusterManager{createErr: errBoom}
	r := newConfiguredResource(t, mgr)

	resp := &resource.CreateResponse{State: emptyResourceState(t)}
	r.Create(context.Background(), resource.CreateRequest{Plan: planWith(t, baseModel("demo"))}, resp)

	require.True(t, resp.Diagnostics.HasError())
	assert.Equal(t, "Failed to create cluster", resp.Diagnostics.Errors()[0].Summary())
	assert.Contains(t, resp.Diagnostics.Errors()[0].Detail(), "boom")
}

// node_image is optional; when set it must be forwarded to kind as an extra
// create option rather than silently dropped.
func TestClusterResource_CreatePassesNodeImageOption(t *testing.T) {
	t.Parallel()

	mgr := &fakeClusterManager{kubeconfig: validKubeconfig}
	r := newConfiguredResource(t, mgr)

	model := baseModel("demo")
	model.NodeImage = types.StringValue("kindest/node:v1.34.0")

	resp := &resource.CreateResponse{State: emptyResourceState(t)}
	r.Create(context.Background(), resource.CreateRequest{Plan: planWith(t, model)}, resp)

	require.False(t, resp.Diagnostics.HasError(), "diagnostics: %v", resp.Diagnostics)

	withImage := mgr.createdOpts

	mgr2 := &fakeClusterManager{kubeconfig: validKubeconfig}
	r2 := newConfiguredResource(t, mgr2)
	resp2 := &resource.CreateResponse{State: emptyResourceState(t)}
	r2.Create(context.Background(), resource.CreateRequest{Plan: planWith(t, baseModel("demo"))}, resp2)
	require.False(t, resp2.Diagnostics.HasError())

	assert.Equal(t, mgr2.createdOpts+1, withImage, "node_image must add one create option")
}

func TestClusterResource_ReadRefreshesExistingCluster(t *testing.T) {
	t.Parallel()

	mgr := &fakeClusterManager{clusters: []string{"demo"}, kubeconfig: validKubeconfig}
	r := newConfiguredResource(t, mgr)

	resp := &resource.ReadResponse{State: emptyResourceState(t)}
	r.Read(context.Background(), resource.ReadRequest{State: stateWith(t, baseModel("demo"))}, resp)

	require.False(t, resp.Diagnostics.HasError(), "diagnostics: %v", resp.Diagnostics)

	var got ClusterResourceModel
	require.False(t, resp.State.Get(context.Background(), &got).HasError())
	assert.Equal(t, "https://127.0.0.1:6443", got.Endpoint.ValueString())
}

// A cluster deleted outside Terraform must be removed from state so the next
// plan recreates it, rather than failing the refresh.
func TestClusterResource_ReadRemovesVanishedClusterFromState(t *testing.T) {
	t.Parallel()

	mgr := &fakeClusterManager{clusters: []string{"other"}}
	r := newConfiguredResource(t, mgr)

	resp := &resource.ReadResponse{State: stateWith(t, baseModel("demo"))}
	r.Read(context.Background(), resource.ReadRequest{State: stateWith(t, baseModel("demo"))}, resp)

	require.False(t, resp.Diagnostics.HasError())
	assert.True(t, resp.State.Raw.IsNull(), "state must be removed when the cluster is gone")
}

func TestClusterResource_ReadSurfacesListError(t *testing.T) {
	t.Parallel()

	mgr := &fakeClusterManager{listErr: errBoom}
	r := newConfiguredResource(t, mgr)

	resp := &resource.ReadResponse{State: emptyResourceState(t)}
	r.Read(context.Background(), resource.ReadRequest{State: stateWith(t, baseModel("demo"))}, resp)

	require.True(t, resp.Diagnostics.HasError())
	assert.Equal(t, "Failed to list clusters", resp.Diagnostics.Errors()[0].Summary())
}

func TestClusterResource_UpdateRepopulatesComputedValues(t *testing.T) {
	t.Parallel()

	mgr := &fakeClusterManager{clusters: []string{"demo"}, kubeconfig: validKubeconfig}
	r := newConfiguredResource(t, mgr)

	resp := &resource.UpdateResponse{State: emptyResourceState(t)}
	r.Update(context.Background(), resource.UpdateRequest{
		Plan:  planWith(t, baseModel("demo")),
		State: stateWith(t, baseModel("demo")),
	}, resp)

	require.False(t, resp.Diagnostics.HasError(), "diagnostics: %v", resp.Diagnostics)

	var got ClusterResourceModel
	require.False(t, resp.State.Get(context.Background(), &got).HasError())
	assert.Equal(t, "https://127.0.0.1:6443", got.Endpoint.ValueString())
}

func TestClusterResource_DeleteSuccess(t *testing.T) {
	t.Parallel()

	// A dotted, hyphenated name exercises the full character set kind allows.
	const name = "team.ci-2"

	mgr := &fakeClusterManager{clusters: []string{name}}
	r := newConfiguredResource(t, mgr)

	resp := &resource.DeleteResponse{State: stateWith(t, baseModel(name))}
	r.Delete(context.Background(), resource.DeleteRequest{State: stateWith(t, baseModel(name))}, resp)

	require.False(t, resp.Diagnostics.HasError())
	assert.Equal(t, name, mgr.deletedName)
}

func TestClusterResource_DeleteSurfacesProviderError(t *testing.T) {
	t.Parallel()

	mgr := &fakeClusterManager{deleteErr: errBoom}
	r := newConfiguredResource(t, mgr)

	resp := &resource.DeleteResponse{State: stateWith(t, baseModel("demo"))}
	r.Delete(context.Background(), resource.DeleteRequest{State: stateWith(t, baseModel("demo"))}, resp)

	require.True(t, resp.Diagnostics.HasError())
	assert.Equal(t, "Failed to delete cluster", resp.Diagnostics.Errors()[0].Summary())
}

// Importing is keyed on the cluster name, which is also the resource ID.
func TestClusterResource_ImportStateSetsName(t *testing.T) {
	t.Parallel()

	resp := &resource.ImportStateResponse{State: emptyResourceState(t)}
	newClusterResourceT(t).ImportState(
		context.Background(),
		resource.ImportStateRequest{ID: "demo"},
		resp,
	)

	require.False(t, resp.Diagnostics.HasError(), "diagnostics: %v", resp.Diagnostics)

	var name types.String
	require.False(t, resp.State.GetAttribute(context.Background(), tfsdkPath("name"), &name).HasError())
	assert.Equal(t, "demo", name.ValueString())
}

// tfsdkPath keeps the import-state assertion readable.
func tfsdkPath(attr string) path.Path { return path.Root(attr) }
