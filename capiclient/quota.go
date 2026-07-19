// Package capiclient adapts the fivetwenty-io/capi client to the narrow
// per-resource interfaces cf-mgmt's managers consume: it aggregates paged
// List calls into ListAll and keeps the []string apply shape.
package capiclient

import (
	"context"

	"github.com/fivetwenty-io/capi/v3/pkg/capi"
)

const perPage = 500

// listAll walks every page of a fw-capi List call and returns pointers into
// the aggregated result set.
func listAll[T any](ctx context.Context, list func(context.Context, *capi.QueryParams) (*capi.ListResponse[T], error), params *capi.QueryParams) ([]*T, error) {
	if params == nil {
		params = capi.NewQueryParams()
	}
	params.WithPerPage(perPage)
	var results []*T
	for page := 1; ; page++ {
		resp, err := list(ctx, params.WithPage(page))
		if err != nil {
			return nil, err
		}
		for i := range resp.Resources {
			results = append(results, &resp.Resources[i])
		}
		if page >= resp.Pagination.TotalPages {
			return results, nil
		}
	}
}

func relationshipGUIDs(rel *capi.ToManyRelationship) []string {
	if rel == nil {
		return nil
	}
	guids := make([]string, 0, len(rel.Data))
	for _, d := range rel.Data {
		guids = append(guids, d.GUID)
	}
	return guids
}

// SpaceQuotas adapts capi.SpaceQuotasClient to cf-mgmt's quota client shape.
type SpaceQuotas struct {
	client capi.SpaceQuotasClient
}

func NewSpaceQuotas(c capi.Client) *SpaceQuotas {
	return &SpaceQuotas{client: c.SpaceQuotas()}
}

func (s *SpaceQuotas) ListAll(ctx context.Context, params *capi.QueryParams) ([]*capi.SpaceQuotaV3, error) {
	return listAll(ctx, func(ctx context.Context, p *capi.QueryParams) (*capi.ListResponse[capi.SpaceQuotaV3], error) {
		return s.client.List(ctx, p)
	}, params)
}

func (s *SpaceQuotas) Get(ctx context.Context, guid string) (*capi.SpaceQuotaV3, error) {
	return s.client.Get(ctx, guid)
}

func (s *SpaceQuotas) Create(ctx context.Context, r *capi.SpaceQuotaV3CreateRequest) (*capi.SpaceQuotaV3, error) {
	return s.client.Create(ctx, r)
}

func (s *SpaceQuotas) Update(ctx context.Context, guid string, r *capi.SpaceQuotaV3UpdateRequest) (*capi.SpaceQuotaV3, error) {
	return s.client.Update(ctx, guid, r)
}

func (s *SpaceQuotas) Apply(ctx context.Context, guid string, spaceGUIDs []string) ([]string, error) {
	rel, err := s.client.ApplyToSpaces(ctx, guid, spaceGUIDs)
	if err != nil {
		return nil, err
	}
	return relationshipGUIDs(rel), nil
}

// OrgQuotas adapts capi.OrganizationQuotasClient to cf-mgmt's quota client shape.
type OrgQuotas struct {
	client capi.OrganizationQuotasClient
}

func NewOrgQuotas(c capi.Client) *OrgQuotas {
	return &OrgQuotas{client: c.OrganizationQuotas()}
}

func (o *OrgQuotas) ListAll(ctx context.Context, params *capi.QueryParams) ([]*capi.OrganizationQuota, error) {
	return listAll(ctx, func(ctx context.Context, p *capi.QueryParams) (*capi.ListResponse[capi.OrganizationQuota], error) {
		return o.client.List(ctx, p)
	}, params)
}

func (o *OrgQuotas) Get(ctx context.Context, guid string) (*capi.OrganizationQuota, error) {
	return o.client.Get(ctx, guid)
}

func (o *OrgQuotas) Create(ctx context.Context, r *capi.OrganizationQuotaCreateRequest) (*capi.OrganizationQuota, error) {
	return o.client.Create(ctx, r)
}

func (o *OrgQuotas) Update(ctx context.Context, guid string, r *capi.OrganizationQuotaUpdateRequest) (*capi.OrganizationQuota, error) {
	return o.client.Update(ctx, guid, r)
}

func (o *OrgQuotas) Apply(ctx context.Context, guid string, organizationGUIDs []string) ([]string, error) {
	rel, err := o.client.ApplyToOrganizations(ctx, guid, organizationGUIDs)
	if err != nil {
		return nil, err
	}
	return relationshipGUIDs(rel), nil
}
