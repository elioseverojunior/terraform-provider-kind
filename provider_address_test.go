// SPDX-FileCopyrightText: 2026 Elio Severo Junior <elioseverojunior@gmail.com>
//
// SPDX-License-Identifier: MIT OR Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServeOpts_UsesTheRegistryAddress(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "registry.terraform.io/elioseverojunior/kind", serveOpts(false).Address)
}

func TestServeOpts_PropagatesDebugFlag(t *testing.T) {
	t.Parallel()

	assert.False(t, serveOpts(false).Debug)
	assert.True(t, serveOpts(true).Debug)
}

// The address the provider serves on must match the `source` practitioners copy
// out of the documentation. A mismatch is invisible until `terraform init`
// fails for someone else, so it is asserted here against the real files.
func TestProviderAddress_MatchesDocumentedSource(t *testing.T) {
	t.Parallel()

	// "registry.terraform.io/elioseverojunior/kind" -> "elioseverojunior/kind"
	parts := strings.Split(providerAddress, "/")
	require.Len(t, parts, 3, "provider address must be host/namespace/type")
	shortForm := parts[1] + "/" + parts[2]

	// Every file that tells a practitioner which source to write.
	sources := []string{
		filepath.Join("examples", "provider", "provider.tf"),
		filepath.Join("examples", "data-sources", "kind_clusters", "data-source.tf"),
		filepath.Join("README.md"),
	}

	for _, file := range sources {
		t.Run(file, func(t *testing.T) {
			t.Parallel()

			content, err := os.ReadFile(file)
			require.NoError(t, err, "documented source file must exist")

			assert.Contains(t, string(content), shortForm,
				"%s must reference the provider as %q", file, shortForm)
		})
	}
}
