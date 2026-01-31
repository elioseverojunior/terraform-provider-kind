package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"sigs.k8s.io/kind/pkg/cluster"
)

var _ provider.Provider = &KindProvider{}

type KindProvider struct {
	version         string
	clusterProvider *cluster.Provider
}

type KindProviderModel struct {
	Host types.String `tfsdk:"host"`
}

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
		Attributes: map[string]schema.Attribute{
			"host": schema.StringAttribute{
				Description: "Docker daemon endpoint (e.g., unix:///var/run/docker.sock or tcp://localhost:2375). Sets the DOCKER_HOST environment variable for kind operations.",
				Optional:    true,
			},
		},
	}
}

func (p *KindProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config KindProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !config.Host.IsNull() && config.Host.ValueString() != "" {
		os.Setenv("DOCKER_HOST", config.Host.ValueString())
	}

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
