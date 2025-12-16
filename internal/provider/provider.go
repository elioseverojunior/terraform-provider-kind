package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"sigs.k8s.io/kind/pkg/cluster"
)

var _ provider.Provider = &KindProvider{}

type KindProvider struct {
	version         string
	clusterProvider *cluster.Provider
}

type KindProviderModel struct{}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &KindProvider{
			version: version,
		}
	}
}

func (p *KindProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "kind"
	resp.Version = p.version
}

func (p *KindProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Terraform provider for KinD (Kubernetes in Docker) clusters.",
	}
}

func (p *KindProvider) Configure(_ context.Context, _ provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	p.clusterProvider = cluster.NewProvider()
	resp.ResourceData = p.clusterProvider
	resp.DataSourceData = p.clusterProvider
}

func (p *KindProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewClusterResource,
	}
}

func (p *KindProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewClustersDataSource,
	}
}
