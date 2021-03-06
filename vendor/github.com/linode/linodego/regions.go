package linodego

import (
	"context"
	"fmt"
)

// Region represents a linode region object
type Region struct {
	ID           string          `json:"id"`
	Country      string          `json:"country"`
	Capabilities []string        `json:"capabilities"`
	Status       string          `json:"status"`
	Resolvers    RegionResolvers `json:"resolvers"`
}

// RegionResolvers contains the DNS resolvers of a region
type RegionResolvers struct {
	IPv4 string `json:"ipv4"`
	IPv6 string `json:"ipv6"`
}

// RegionsPagedResponse represents a linode API response for listing
type RegionsPagedResponse struct {
	*PageOptions
	Data []Region `json:"data"`
}

// endpoint gets the endpoint URL for Region
func (RegionsPagedResponse) endpoint(c *Client) string {
	endpoint, err := c.Regions.Endpoint()
	if err != nil {
		panic(err)
	}
	return endpoint
}

// appendData appends Regions when processing paginated Region responses
func (resp *RegionsPagedResponse) appendData(r *RegionsPagedResponse) {
	resp.Data = append(resp.Data, r.Data...)
}

// ListRegions lists Regions
func (c *Client) ListRegions(ctx context.Context, opts *ListOptions) ([]Region, error) {
	response := RegionsPagedResponse{}
	err := c.listHelper(ctx, &response, opts)
	if err != nil {
		return nil, err
	}
	return response.Data, nil
}

// GetRegion gets the template with the provided ID
func (c *Client) GetRegion(ctx context.Context, id string) (*Region, error) {
	e, err := c.Regions.Endpoint()
	if err != nil {
		return nil, err
	}
	e = fmt.Sprintf("%s/%s", e, id)
	r, err := coupleAPIErrors(c.R(ctx).SetResult(&Region{}).Get(e))
	if err != nil {
		return nil, err
	}
	return r.Result().(*Region), nil
}
