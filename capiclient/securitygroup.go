package capiclient

import (
	"context"

	"github.com/fivetwenty-io/capi/v3/pkg/capi"
)

// SecurityGroups adapts capi.SecurityGroupsClient to cf-mgmt's client shape.
type SecurityGroups struct {
	client capi.SecurityGroupsClient
}

func NewSecurityGroups(c capi.Client) *SecurityGroups {
	return &SecurityGroups{client: c.SecurityGroups()}
}

func (s *SecurityGroups) ListAll(ctx context.Context, params *capi.QueryParams) ([]*capi.SecurityGroup, error) {
	return listAll(ctx, func(ctx context.Context, p *capi.QueryParams) (*capi.ListResponse[capi.SecurityGroup], error) {
		return s.client.List(ctx, p)
	}, params)
}

func (s *SecurityGroups) ListRunningForSpaceAll(ctx context.Context, spaceGUID string) ([]*capi.SecurityGroup, error) {
	return listAll(ctx, func(ctx context.Context, p *capi.QueryParams) (*capi.ListResponse[capi.SecurityGroup], error) {
		return s.client.List(ctx, p, capi.WithSecurityGroupRunningSpaceGUIDs(spaceGUID))
	}, nil)
}

func (s *SecurityGroups) Get(ctx context.Context, guid string) (*capi.SecurityGroup, error) {
	return s.client.Get(ctx, guid)
}

func (s *SecurityGroups) Create(ctx context.Context, r *capi.SecurityGroupCreateRequest) (*capi.SecurityGroup, error) {
	return s.client.Create(ctx, r)
}

func (s *SecurityGroups) Update(ctx context.Context, guid string, r *capi.SecurityGroupUpdateRequest) (*capi.SecurityGroup, error) {
	return s.client.Update(ctx, guid, r)
}

func (s *SecurityGroups) BindRunningSecurityGroup(ctx context.Context, guid string, spaceGUIDs []string) ([]string, error) {
	rel, err := s.client.BindRunningSpaces(ctx, guid, spaceGUIDs)
	if err != nil {
		return nil, err
	}
	return relationshipGUIDs(rel), nil
}

func (s *SecurityGroups) BindStagingSecurityGroup(ctx context.Context, guid string, spaceGUIDs []string) ([]string, error) {
	rel, err := s.client.BindStagingSpaces(ctx, guid, spaceGUIDs)
	if err != nil {
		return nil, err
	}
	return relationshipGUIDs(rel), nil
}

func (s *SecurityGroups) UnBindRunningSecurityGroup(ctx context.Context, guid string, spaceGUID string) error {
	return s.client.UnbindRunningSpace(ctx, guid, spaceGUID)
}

func (s *SecurityGroups) UnBindStagingSecurityGroup(ctx context.Context, guid string, spaceGUID string) error {
	return s.client.UnbindStagingSpace(ctx, guid, spaceGUID)
}
