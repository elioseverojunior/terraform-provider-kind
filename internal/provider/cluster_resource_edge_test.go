// SPDX-FileCopyrightText: 2026 Elio Severo Junior <elioseverojunior@gmail.com>
//
// SPDX-License-Identifier: MIT OR Apache-2.0

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mismatchedPlan returns a plan whose schema does not match
// ClusterResourceModel, forcing the framework's decode to fail. This is how the
// CRUD methods' "could not read plan/state" guards are reached.
func mismatchedPlan(t *testing.T) tfsdk.Plan {
	t.Helper()

	s := dataSourceSchema(t)

	return tfsdk.Plan{
		Schema: s,
		Raw:    tftypes.NewValue(s.Type().TerraformType(context.Background()), nil),
	}
}

// mismatchedState is the tfsdk.State counterpart of mismatchedPlan.
func mismatchedState(t *testing.T) tfsdk.State {
	t.Helper()

	s := dataSourceSchema(t)

	return tfsdk.State{
		Schema: s,
		Raw:    tftypes.NewValue(s.Type().TerraformType(context.Background()), nil),
	}
}

func TestClusterResource_CreateBailsOutOnUnreadablePlan(t *testing.T) {
	t.Parallel()

	r := newConfiguredResource(t, &fakeClusterManager{})

	resp := &resource.CreateResponse{State: emptyResourceState(t)}
	r.Create(context.Background(), resource.CreateRequest{Plan: mismatchedPlan(t)}, resp)

	require.True(t, resp.Diagnostics.HasError())
	assert.Empty(t, (&fakeClusterManager{}).createdName, "no cluster may be created from an unreadable plan")
}

func TestClusterResource_ReadBailsOutOnUnreadableState(t *testing.T) {
	t.Parallel()

	r := newConfiguredResource(t, &fakeClusterManager{})

	resp := &resource.ReadResponse{State: emptyResourceState(t)}
	r.Read(context.Background(), resource.ReadRequest{State: mismatchedState(t)}, resp)

	assert.True(t, resp.Diagnostics.HasError())
}

func TestClusterResource_UpdateBailsOutOnUnreadablePlan(t *testing.T) {
	t.Parallel()

	r := newConfiguredResource(t, &fakeClusterManager{})

	resp := &resource.UpdateResponse{State: emptyResourceState(t)}
	r.Update(context.Background(), resource.UpdateRequest{Plan: mismatchedPlan(t)}, resp)

	assert.True(t, resp.Diagnostics.HasError())
}

func TestClusterResource_DeleteBailsOutOnUnreadableState(t *testing.T) {
	t.Parallel()

	mgr := &fakeClusterManager{}
	r := newConfiguredResource(t, mgr)

	resp := &resource.DeleteResponse{State: emptyResourceState(t)}
	r.Delete(context.Background(), resource.DeleteRequest{State: mismatchedState(t)}, resp)

	require.True(t, resp.Diagnostics.HasError())
	assert.Empty(t, mgr.deletedName, "no cluster may be deleted from an unreadable state")
}

// If the cluster is created but its kubeconfig cannot be read, the apply must
// fail loudly rather than writing a half-populated resource to state.
func TestClusterResource_CreateFailsWhenKubeconfigUnavailable(t *testing.T) {
	t.Parallel()

	r := newConfiguredResource(t, &fakeClusterManager{kubeConfigErr: errBoom})

	resp := &resource.CreateResponse{State: emptyResourceState(t)}
	r.Create(context.Background(), resource.CreateRequest{Plan: planWith(t, baseModel("demo"))}, resp)

	require.True(t, resp.Diagnostics.HasError())
	assert.Equal(t, "Failed to get kubeconfig", resp.Diagnostics.Errors()[0].Summary())
}

func TestClusterResource_UpdateSurfacesKubeconfigError(t *testing.T) {
	t.Parallel()

	r := newConfiguredResource(t, &fakeClusterManager{kubeConfigErr: errBoom})

	resp := &resource.UpdateResponse{State: emptyResourceState(t)}
	r.Update(context.Background(), resource.UpdateRequest{Plan: planWith(t, baseModel("demo"))}, resp)

	require.True(t, resp.Diagnostics.HasError())
	assert.Equal(t, "Failed to get kubeconfig", resp.Diagnostics.Errors()[0].Summary())
}

func TestClusterResource_ReadSurfacesKubeconfigError(t *testing.T) {
	t.Parallel()

	r := newConfiguredResource(t, &fakeClusterManager{clusters: []string{"demo"}, kubeConfigErr: errBoom})

	resp := &resource.ReadResponse{State: emptyResourceState(t)}
	r.Read(context.Background(), resource.ReadRequest{State: stateWith(t, baseModel("demo"))}, resp)

	require.True(t, resp.Diagnostics.HasError())
	assert.Equal(t, "Failed to get kubeconfig", resp.Diagnostics.Errors()[0].Summary())
}

// With wait_for_nodes_ready enabled the apply must fail when the nodes never
// report Ready, instead of returning a cluster that is not usable yet. The
// kubeconfig here points at a port nothing is listening on, so the wait can
// only end in a timeout.
func TestClusterResource_CreateFailsWhenNodesNeverBecomeReady(t *testing.T) {
	t.Parallel()

	r := newConfiguredResource(t, &fakeClusterManager{kubeconfig: validKubeconfig})

	model := baseModel("demo")
	model.WaitForNodesReady = types.BoolValue(true)
	model.WaitForReady = types.Int64Value(1)

	resp := &resource.CreateResponse{State: emptyResourceState(t)}
	r.Create(context.Background(), resource.CreateRequest{Plan: planWith(t, model)}, resp)

	require.True(t, resp.Diagnostics.HasError())
	assert.Equal(t, "Failed waiting for nodes to be ready", resp.Diagnostics.Errors()[0].Summary())
}
