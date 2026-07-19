package securitygroup

import (
	"context"

	"github.com/fivetwenty-io/capi/v3/pkg/capi"
)

type Manager interface {
	ListNonDefaultSecurityGroups() (map[string]*capi.SecurityGroup, error)
	ListDefaultSecurityGroups() (map[string]*capi.SecurityGroup, error)
	ListSpaceSecurityGroups(spaceGUID string) (map[string]string, error)
	GetSecurityGroupRules(sgGUID string) ([]byte, error)
	CreateApplicationSecurityGroups() error
	CreateGlobalSecurityGroups() error
	AssignDefaultSecurityGroups() error
}

type CFSecurityGroupClient interface {
	ListAll(ctx context.Context, params *capi.QueryParams) ([]*capi.SecurityGroup, error)
	Create(ctx context.Context, r *capi.SecurityGroupCreateRequest) (*capi.SecurityGroup, error)
	Update(ctx context.Context, guid string, r *capi.SecurityGroupUpdateRequest) (*capi.SecurityGroup, error)
	BindRunningSecurityGroup(ctx context.Context, guid string, spaceGUIDs []string) ([]string, error)
	BindStagingSecurityGroup(ctx context.Context, guid string, spaceGUIDs []string) ([]string, error)
	UnBindRunningSecurityGroup(ctx context.Context, guid string, spaceGUID string) error
	UnBindStagingSecurityGroup(ctx context.Context, guid string, spaceGUID string) error
	Get(ctx context.Context, guid string) (*capi.SecurityGroup, error)
	ListRunningForSpaceAll(ctx context.Context, spaceGUID string) ([]*capi.SecurityGroup, error)
}
