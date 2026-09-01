// SPDX-FileCopyrightText: 2026 Elio Severo Junior <elioseverojunior@gmail.com>
//
// SPDX-License-Identifier: MIT OR Apache-2.0

package provider

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/kind/pkg/cluster"
)

// errBoom is the canonical failure returned by fakeClusterManager so tests can
// assert that the underlying error text reaches the practitioner's diagnostics.
var errBoom = errors.New("boom")

// fakeClusterManager is an in-memory clusterManager used to exercise the
// resource and data source without Docker or a real kind installation.
type fakeClusterManager struct {
	clusters   []string
	kubeconfig string

	createErr     error
	deleteErr     error
	listErr       error
	kubeConfigErr error

	createdName string
	createdOpts int
	deletedName string
	kubeCfgName string
}

func (f *fakeClusterManager) Create(name string, options ...cluster.CreateOption) error {
	f.createdName = name
	f.createdOpts = len(options)
	if f.createErr != nil {
		return f.createErr
	}
	f.clusters = append(f.clusters, name)
	return nil
}

func (f *fakeClusterManager) Delete(name, _ string) error {
	f.deletedName = name
	return f.deleteErr
}

func (f *fakeClusterManager) List() ([]string, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.clusters, nil
}

func (f *fakeClusterManager) KubeConfig(name string, _ bool) (string, error) {
	f.kubeCfgName = name
	if f.kubeConfigErr != nil {
		return "", f.kubeConfigErr
	}
	return f.kubeconfig, nil
}

// validKubeconfig is a minimally complete kubeconfig with the four fields the
// provider surfaces as computed attributes.
const validKubeconfig = `apiVersion: v1
kind: Config
clusters:
- name: kind-demo
  cluster:
    server: https://127.0.0.1:6443
    certificate-authority-data: Q0FEQVRB
users:
- name: kind-demo
  user:
    client-certificate-data: Q0VSVERBVEE=
    client-key-data: S0VZREFUQQ==
contexts:
- name: kind-demo
  context:
    cluster: kind-demo
    user: kind-demo
current-context: kind-demo
`

// newClusterResourceT constructs the concrete kind_cluster resource, failing the
// test rather than panicking if the constructor's type ever changes.
func newClusterResourceT(t *testing.T) *ClusterResource {
	t.Helper()

	r, ok := NewClusterResource().(*ClusterResource)
	require.True(t, ok, "NewClusterResource must return *ClusterResource")

	return r
}

// newClustersDataSourceT constructs the concrete kind_clusters data source.
func newClustersDataSourceT(t *testing.T) *ClustersDataSource {
	t.Helper()

	d, ok := NewClustersDataSource().(*ClustersDataSource)
	require.True(t, ok, "NewClustersDataSource must return *ClustersDataSource")

	return d
}

// resourceSchemaFor returns the kind_cluster schema for building plans/states.
func resourceSchemaFor(t *testing.T) tfsdk.Plan {
	t.Helper()
	return tfsdk.Plan{Schema: clusterSchema(t)}
}

// baseModel is a fully-populated ClusterResourceModel with every collection
// attribute explicitly null, suitable for round-tripping through the schema.
func baseModel(name string) ClusterResourceModel {
	return ClusterResourceModel{
		ID:                              types.StringValue(name),
		Name:                            types.StringValue(name),
		NodeImage:                       types.StringValue(""),
		WaitForReady:                    types.Int64Value(300),
		WaitForNodesReady:               types.BoolValue(false),
		FeatureGates:                    types.MapNull(types.BoolType),
		RuntimeConfig:                   types.MapNull(types.StringType),
		KubeadmConfigPatches:            types.ListNull(types.StringType),
		ContainerdConfigPatches:         types.ListNull(types.StringType),
		ContainerdConfigPatchesJSON6902: types.ListNull(types.StringType),
		Kubeconfig:                      types.StringNull(),
		KubeconfigPath:                  types.StringNull(),
		ClientCertificate:               types.StringNull(),
		ClientKey:                       types.StringNull(),
		ClusterCaCertificate:            types.StringNull(),
		Endpoint:                        types.StringNull(),
	}
}

// planWith returns a tfsdk.Plan carrying the supplied model.
func planWith(t *testing.T, model ClusterResourceModel) tfsdk.Plan {
	t.Helper()

	plan := resourceSchemaFor(t)
	require.False(t, plan.Set(context.Background(), &model).HasError())

	return plan
}

// stateWith returns a tfsdk.State carrying the supplied model.
func stateWith(t *testing.T, model ClusterResourceModel) tfsdk.State {
	t.Helper()

	state := tfsdk.State{Schema: clusterSchema(t)}
	require.False(t, state.Set(context.Background(), &model).HasError())

	return state
}

// emptyResourceState returns a null-but-typed state shaped by the resource
// schema. The framework always hands a resource a state whose Raw is a null
// object of the schema's type -- never a zero tftypes.Value -- and attribute
// writes such as ImportState's passthrough depend on that type being present.
func emptyResourceState(t *testing.T) tfsdk.State {
	t.Helper()

	s := clusterSchema(t)

	return tfsdk.State{
		Schema: s,
		Raw:    tftypes.NewValue(s.Type().TerraformType(context.Background()), nil),
	}
}

// dataSourceSchema returns the kind_clusters data source schema.
func dataSourceSchema(t *testing.T) dsschema.Schema {
	t.Helper()

	resp := &datasource.SchemaResponse{}
	newClustersDataSourceT(t).Schema(context.Background(), datasource.SchemaRequest{}, resp)
	require.False(t, resp.Diagnostics.HasError())

	return resp.Schema
}

// newConfiguredResource returns a kind_cluster resource wired to a fake manager
// through the real Configure path, so the injection seam is covered too.
func newConfiguredResource(t *testing.T, mgr *fakeClusterManager) *ClusterResource {
	t.Helper()

	r := newClusterResourceT(t)
	resp := &resource.ConfigureResponse{}
	r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: mgr}, resp)
	require.False(t, resp.Diagnostics.HasError(), "Configure diagnostics: %v", resp.Diagnostics)

	return r
}

// newConfiguredDataSource returns a kind_clusters data source wired to a fake.
func newConfiguredDataSource(t *testing.T, mgr *fakeClusterManager) *ClustersDataSource {
	t.Helper()

	d := newClustersDataSourceT(t)
	resp := &datasource.ConfigureResponse{}
	d.Configure(context.Background(), datasource.ConfigureRequest{ProviderData: mgr}, resp)
	require.False(t, resp.Diagnostics.HasError(), "Configure diagnostics: %v", resp.Diagnostics)

	return d
}
