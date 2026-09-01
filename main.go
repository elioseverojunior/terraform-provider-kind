// SPDX-FileCopyrightText: 2026 Elio Severo Junior <elioseverojunior@gmail.com>
//
// SPDX-License-Identifier: MIT OR Apache-2.0

package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"kind-provider/internal/provider"
)

// version and commit are injected at release time by GoReleaser via
// -ldflags "-X main.version=... -X main.commit=...". The defaults are what a
// locally built binary reports.
var (
	version = "dev"
	commit  = "none"
)

// providerAddress is the fully qualified registry address Terraform uses to
// locate this provider. It must stay in step with the `source` practitioners
// write in required_providers; provider_address_test.go asserts that the
// documentation and examples agree with this constant.
const providerAddress = "registry.terraform.io/elioseverojunior/kind"

// serveOpts builds the plugin server options. Kept separate from main so the
// address, which is not otherwise verifiable until a practitioner's
// `terraform init` fails, can be asserted in tests.
func serveOpts(debug bool) providerserver.ServeOpts {
	return providerserver.ServeOpts{
		Address: providerAddress,
		Debug:   debug,
	}
}

func main() {
	// Terraform attaches a debugger to the provider process when started with
	// -debug; see https://developer.hashicorp.com/terraform/plugin/debugging
	var debug bool
	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	log.SetPrefix("terraform-provider-kind: ")

	if err := providerserver.Serve(context.Background(), provider.New(version), serveOpts(debug)); err != nil {
		log.Fatalf("failed to serve provider (version %s, commit %s): %v", version, commit, err)
	}
}
