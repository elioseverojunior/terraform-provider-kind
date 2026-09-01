// SPDX-FileCopyrightText: 2026 Elio Severo Junior <elioseverojunior@gmail.com>
//
// SPDX-License-Identifier: MIT OR Apache-2.0

package provider

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPopulateComputedValues_ExtractsEveryConnectionAttribute(t *testing.T) {
	t.Parallel()

	r := &ClusterResource{provider: &fakeClusterManager{kubeconfig: validKubeconfig}}
	data := baseModel("demo")
	var diags diag.Diagnostics

	r.populateComputedValues(&data, &diags)

	require.False(t, diags.HasError(), "diagnostics: %v", diags)
	assert.Equal(t, "demo", data.ID.ValueString())
	assert.Equal(t, validKubeconfig, data.Kubeconfig.ValueString())
	assert.Equal(t, "https://127.0.0.1:6443", data.Endpoint.ValueString())
	assert.Equal(t, "Q0FEQVRB", data.ClusterCaCertificate.ValueString())
	assert.Equal(t, "Q0VSVERBVEE=", data.ClientCertificate.ValueString())
	assert.Equal(t, "S0VZREFUQQ==", data.ClientKey.ValueString())
}

// The kubeconfig path is derived, not read from kind, so it is pinned here.
func TestPopulateComputedValues_DerivesKubeconfigPath(t *testing.T) {
	t.Parallel()

	r := &ClusterResource{provider: &fakeClusterManager{kubeconfig: validKubeconfig}}
	data := baseModel("demo")
	var diags diag.Diagnostics

	r.populateComputedValues(&data, &diags)
	require.False(t, diags.HasError())

	home, err := os.UserHomeDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, ".kube", "kind", "kind-demo"), data.KubeconfigPath.ValueString())
}

func TestPopulateComputedValues_SurfacesKubeconfigError(t *testing.T) {
	t.Parallel()

	r := &ClusterResource{provider: &fakeClusterManager{kubeConfigErr: errBoom}}
	data := baseModel("demo")
	var diags diag.Diagnostics

	r.populateComputedValues(&data, &diags)

	require.True(t, diags.HasError())
	assert.Equal(t, "Failed to get kubeconfig", diags.Errors()[0].Summary())
}

func TestPopulateComputedValues_SurfacesUnparseableKubeconfig(t *testing.T) {
	t.Parallel()

	r := &ClusterResource{provider: &fakeClusterManager{kubeconfig: "\tnot: [valid yaml"}}
	data := baseModel("demo")
	var diags diag.Diagnostics

	r.populateComputedValues(&data, &diags)

	require.True(t, diags.HasError())
	assert.Equal(t, "Failed to parse kubeconfig", diags.Errors()[0].Summary())
}

// A kubeconfig missing the clusters/users stanzas must leave the computed
// attributes as empty strings. They are Computed and non-nullable, so leaving
// them null would make Terraform fail with "provider produced inconsistent
// result after apply".
func TestPopulateComputedValues_MissingStanzasBecomeEmptyStrings(t *testing.T) {
	t.Parallel()

	r := &ClusterResource{provider: &fakeClusterManager{kubeconfig: "apiVersion: v1\nkind: Config\n"}}
	data := baseModel("demo")
	var diags diag.Diagnostics

	r.populateComputedValues(&data, &diags)

	require.False(t, diags.HasError(), "diagnostics: %v", diags)
	assert.Equal(t, "", data.Endpoint.ValueString())
	assert.Equal(t, "", data.ClusterCaCertificate.ValueString())
	assert.Equal(t, "", data.ClientCertificate.ValueString())
	assert.Equal(t, "", data.ClientKey.ValueString())
	assert.False(t, data.Endpoint.IsNull(), "computed attributes must never stay null")
}

func TestPopulateComputedValues_PartialKubeconfigStanzas(t *testing.T) {
	t.Parallel()

	// Cluster stanza present but with no certificate-authority-data, and a user
	// stanza with only a client certificate.
	kubeconfig := `apiVersion: v1
kind: Config
clusters:
- name: kind-demo
  cluster:
    server: https://127.0.0.1:7443
users:
- name: kind-demo
  user:
    client-certificate-data: T05MWQ==
`

	r := &ClusterResource{provider: &fakeClusterManager{kubeconfig: kubeconfig}}
	data := baseModel("demo")
	var diags diag.Diagnostics

	r.populateComputedValues(&data, &diags)

	require.False(t, diags.HasError())
	assert.Equal(t, "https://127.0.0.1:7443", data.Endpoint.ValueString())
	assert.Equal(t, "T05MWQ==", data.ClientCertificate.ValueString())
	assert.Equal(t, "", data.ClusterCaCertificate.ValueString())
	assert.Equal(t, "", data.ClientKey.ValueString())
}

// Malformed stanzas (wrong shape rather than invalid YAML) must not panic.
func TestPopulateComputedValues_TolerateWronglyShapedStanzas(t *testing.T) {
	t.Parallel()

	kubeconfig := `apiVersion: v1
kind: Config
clusters: "not-a-list"
users:
- "not-a-map"
`

	r := &ClusterResource{provider: &fakeClusterManager{kubeconfig: kubeconfig}}
	data := baseModel("demo")
	var diags diag.Diagnostics

	require.NotPanics(t, func() { r.populateComputedValues(&data, &diags) })
	require.False(t, diags.HasError())
	assert.Equal(t, "", data.Endpoint.ValueString())
}

func TestPopulateComputedValues_SurfacesHomeDirError(t *testing.T) {
	// t.Setenv forbids t.Parallel, which is what we want here: HOME is global.
	t.Setenv("HOME", "")

	r := &ClusterResource{provider: &fakeClusterManager{kubeconfig: validKubeconfig}}
	data := baseModel("demo")
	var diags diag.Diagnostics

	r.populateComputedValues(&data, &diags)

	require.True(t, diags.HasError())
	assert.Equal(t, "Failed to get home directory", diags.Errors()[0].Summary())
}
