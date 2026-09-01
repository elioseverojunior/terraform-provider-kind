// SPDX-FileCopyrightText: 2026 Elio Severo Junior <elioseverojunior@gmail.com>
//
// SPDX-License-Identifier: MIT OR Apache-2.0

package provider

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// regexpInvalidName matches the diagnostic the name validator emits.
var regexpInvalidName = regexp.MustCompile(`cluster names must match`)

// testAccProtoV6ProviderFactories wires the in-process provider into the
// acceptance test harness, so no provider binary has to be installed.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"kind": providerserver.NewProtocol6WithError(New("test")()),
}

// testAccPreCheck fails fast with an actionable message when the machine cannot
// run acceptance tests, rather than letting them fail deep inside kind.
//
// Acceptance tests create real clusters: they need Docker running and take
// minutes. They only run when TF_ACC is set, which is the standard Terraform
// provider convention and is what `make testacc` sets.
func testAccPreCheck(t *testing.T) {
	t.Helper()

	if os.Getenv("TF_ACC") == "" {
		t.Skip("acceptance tests skipped unless TF_ACC is set")
	}

	if _, err := exec.LookPath("docker"); err != nil {
		t.Fatal("acceptance tests require the docker CLI on PATH")
	}

	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Fatal("acceptance tests require a running Docker daemon")
	}
}

// testAccClusterConfig renders a minimal single-node cluster configuration.
func testAccClusterConfig(name string) string {
	return fmt.Sprintf(`
resource "kind_cluster" "test" {
  name = %[1]q

  node {
    role = "control-plane"
  }
}
`, name)
}

// TestAccClusterResource_lifecycle covers create, refresh and import against a
// real cluster.
func TestAccClusterResource_lifecycle(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccClusterConfig("tfacc-lifecycle"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("kind_cluster.test", "name", "tfacc-lifecycle"),
					resource.TestCheckResourceAttr("kind_cluster.test", "id", "tfacc-lifecycle"),
					resource.TestCheckResourceAttrSet("kind_cluster.test", "endpoint"),
					resource.TestCheckResourceAttrSet("kind_cluster.test", "kubeconfig"),
					resource.TestCheckResourceAttrSet("kind_cluster.test", "kubeconfig_path"),
					resource.TestCheckResourceAttrSet("kind_cluster.test", "client_certificate"),
					resource.TestCheckResourceAttrSet("kind_cluster.test", "cluster_ca_certificate"),
				),
			},
			{
				ResourceName:      "kind_cluster.test",
				ImportState:       true,
				ImportStateId:     "tfacc-lifecycle",
				ImportStateVerify: false, // kubeconfig is re-read, not stored by kind.
			},
		},
	})
}

// TestAccClusterResource_rejectsInvalidName proves the name validator runs at
// plan time, before anything reaches Docker.
func TestAccClusterResource_rejectsInvalidName(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccClusterConfig("Invalid_Name"),
				PlanOnly:    true,
				ExpectError: regexpInvalidName,
			},
		},
	})
}

// TestAccClustersDataSource_listsCreatedCluster checks the data source observes
// a cluster created in the same configuration.
func TestAccClustersDataSource_listsCreatedCluster(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccClusterConfig("tfacc-datasource") + `
data "kind_clusters" "all" {
  depends_on = [kind_cluster.test]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.kind_clusters.all", "id", "kind-clusters"),
					resource.TestCheckTypeSetElemAttr("data.kind_clusters.all", "clusters.*", "tfacc-datasource"),
				),
			},
		},
	})
}
