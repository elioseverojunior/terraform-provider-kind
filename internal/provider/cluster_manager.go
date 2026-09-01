// SPDX-FileCopyrightText: 2026 Elio Severo Junior <elioseverojunior@gmail.com>
//
// SPDX-License-Identifier: MIT OR Apache-2.0

package provider

import "sigs.k8s.io/kind/pkg/cluster"

// clusterManager is the slice of sigs.k8s.io/kind's *cluster.Provider that this
// Terraform provider actually calls.
//
// Depending on this interface rather than the concrete type keeps the resource
// and data source exercisable without Docker or a kind installation: production
// code is handed a real *cluster.Provider, tests are handed an in-memory fake.
// Only the four methods used are listed, so implementers are not forced to
// satisfy the whole of kind's surface.
type clusterManager interface {
	// Create provisions a new cluster with the supplied kind create options.
	Create(name string, options ...cluster.CreateOption) error

	// Delete tears down a cluster. The second argument is an explicit
	// kubeconfig path; empty means "use the default resolution".
	Delete(name, explicitKubeconfigPath string) error

	// List returns the names of all clusters kind knows about.
	List() ([]string, error)

	// KubeConfig returns the kubeconfig for a cluster. When internal is true the
	// returned config addresses the API server from inside the Docker network.
	KubeConfig(name string, internal bool) (string, error)
}

// Compile-time proof that kind's provider satisfies the interface, so a
// signature change upstream is caught at build time rather than at runtime.
var _ clusterManager = (*cluster.Provider)(nil)
