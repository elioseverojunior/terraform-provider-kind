package main

import (
	"context"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"kind-provider/internal/provider"
)

var version = "0.1.0"

func main() {
	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/elioseverojunior/kind",
	}

	err := providerserver.Serve(context.Background(), provider.New(version), opts)
	if err != nil {
		log.Fatal(err)
	}
}
