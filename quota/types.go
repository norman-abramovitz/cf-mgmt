package quota

import (
	"context"

	"github.com/fivetwenty-io/capi/v3/pkg/capi"
)

type CFSpaceQuotaClient interface {
	ListAll(ctx context.Context, params *capi.QueryParams) ([]*capi.SpaceQuotaV3, error)
	Update(ctx context.Context, guid string, r *capi.SpaceQuotaV3UpdateRequest) (*capi.SpaceQuotaV3, error)
	Create(ctx context.Context, r *capi.SpaceQuotaV3CreateRequest) (*capi.SpaceQuotaV3, error)
	Apply(ctx context.Context, guid string, spaceGUIDs []string) ([]string, error)
	Get(ctx context.Context, guid string) (*capi.SpaceQuotaV3, error)
}

type CFOrgQuotaClient interface {
	ListAll(ctx context.Context, params *capi.QueryParams) ([]*capi.OrganizationQuota, error)
	Update(ctx context.Context, guid string, r *capi.OrganizationQuotaUpdateRequest) (*capi.OrganizationQuota, error)
	Create(ctx context.Context, r *capi.OrganizationQuotaCreateRequest) (*capi.OrganizationQuota, error)
	Get(ctx context.Context, guid string) (*capi.OrganizationQuota, error)
	Apply(ctx context.Context, guid string, organizationGUIDs []string) ([]string, error)
}
