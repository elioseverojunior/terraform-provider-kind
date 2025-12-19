package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"sigs.k8s.io/kind/pkg/cluster"
)

var _ datasource.DataSource = &ClustersDataSource{}

type ClustersDataSource struct {
	provider *cluster.Provider
}

func NewClustersDataSource() datasource.DataSource {
	return &ClustersDataSource{}
}

func (d *ClustersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_clusters"
}

func (d *ClustersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List all KinD clusters.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Data source identifier.",
				Computed:    true,
			},
			"clusters": schema.ListAttribute{
				Description: "List of cluster names.",
				Computed:    true,
				ElementType: types.StringType,
			},
		},
	}
}

func (d *ClustersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	provider, ok := req.ProviderData.(*cluster.Provider)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *cluster.Provider, got: %T", req.ProviderData),
		)
		return
	}

	d.provider = provider
}

func (d *ClustersDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	clusters, err := d.provider.List()
	if err != nil {
		resp.Diagnostics.AddError("Failed to list clusters", err.Error())
		return
	}

	data := ClustersDataSourceModel{
		ID:       types.StringValue("kind-clusters"),
		Clusters: make([]types.String, len(clusters)),
	}

	for i, c := range clusters {
		data.Clusters[i] = types.StringValue(c)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
