package main

import (
	"context"
	"flag"
	"log"

	"github.com/everadaptive/terraform-provider-purelymail/internal/provider"
	tfprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggable provider instances")
	flag.Parse()

	err := providerserver.Serve(context.Background(), func() tfprovider.Provider {
		return provider.New()
	}, providerserver.ServeOpts{
		Address: "registry.terraform.io/everadaptive/purelymail",
		Debug:   debug,
	})
	if err != nil {
		log.Fatal(err)
	}
}
