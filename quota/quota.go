package quota

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/cloudfoundry-community/go-cfclient/v3/resource"
	"github.com/fivetwenty-io/capi/v3/pkg/capi"
	"github.com/pkg/errors"
	"github.com/vmwarepivotallabs/cf-mgmt/config"
	"github.com/vmwarepivotallabs/cf-mgmt/organizationreader"
	"github.com/vmwarepivotallabs/cf-mgmt/space"
	"github.com/xchapter7x/lo"
)

// NewManager -
func NewManager(
	spaceQuotaClient CFSpaceQuotaClient,
	orgQuotaClient CFOrgQuotaClient,
	spaceMgr space.Manager,
	orgReader organizationreader.Reader,
	cfg config.Reader, peek bool) *Manager {
	return &Manager{
		Cfg:              cfg,
		SpaceQuoteClient: spaceQuotaClient,
		OrgQuoteClient:   orgQuotaClient,
		SpaceMgr:         spaceMgr,
		OrgReader:        orgReader,
		Peek:             peek,
	}
}

// Manager -
type Manager struct {
	Cfg              config.Reader
	SpaceQuoteClient CFSpaceQuotaClient
	OrgQuoteClient   CFOrgQuotaClient
	SpaceMgr         space.Manager
	OrgReader        organizationreader.Reader
	Peek             bool
	SpaceQuotas      map[string]map[string]*capi.SpaceQuotaV3
}

// CreateSpaceQuotas -
func (m *Manager) CreateSpaceQuotas() error {
	m.SpaceQuotas = nil
	spaceConfigs, err := m.Cfg.GetSpaceConfigs()
	if err != nil {
		return err
	}

	for _, input := range spaceConfigs {
		if input.NamedQuota != "" && input.EnableSpaceQuota {
			return fmt.Errorf("cannot have named quota %s and enable-space-quota for org/space %s/%s", input.NamedQuota, input.Org, input.Space)
		}
		if input.NamedQuota != "" || input.EnableSpaceQuota {
			space, err := m.SpaceMgr.FindSpace(input.Org, input.Space)
			if err != nil {
				return errors.Wrap(err, "Finding spaces")
			}
			quotas, err := m.ListAllSpaceQuotasForOrg(space.Relationships.Organization.Data.GUID)
			if err != nil {
				return errors.Wrap(err, "ListAllSpaceQuotasForOrg")
			}

			orgQuotas, err := m.ListAllOrgQuotas()
			if err != nil {
				return err
			}
			if input.NamedQuota != "" {
				spaceQuotas, err := m.Cfg.GetSpaceQuotas(input.Org)
				if err != nil {
					return err
				}

				for _, spaceQuotaConfig := range spaceQuotas {
					err = m.createSpaceQuota(spaceQuotaConfig, space, quotas, orgQuotas)
					if err != nil {
						return err
					}
				}
			} else {
				if input.EnableSpaceQuota {
					quotaDef := input.GetQuota()
					err = m.createSpaceQuota(quotaDef, space, quotas, orgQuotas)
					if err != nil {
						return err
					}
					input.NamedQuota = input.Space
				}
			}
			spaceQuota := quotas[input.NamedQuota]

			if m.Peek != true {
				if spaceQuota != nil && (space.Relationships.Quota == nil || space.Relationships.Quota.Data == nil || space.Relationships.Quota.Data.GUID != spaceQuota.GUID) {
					if err = m.AssignQuotaToSpace(space, spaceQuota); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func (m *Manager) createSpaceQuota(input config.SpaceQuota, space *resource.Space, quotas map[string]*capi.SpaceQuotaV3, orgQuotas map[string]*capi.OrganizationQuota) error {

	org, err := m.OrgReader.FindOrg(input.Org)
	if err != nil {
		return err
	}

	quota := &capi.SpaceQuotaV3CreateRequest{
		Name: input.Name,
		Relationships: capi.SpaceQuotaRelationships{
			Organization: capi.Relationship{Data: &capi.RelationshipData{GUID: org.GUID}},
		},
	}
	quota.Apps = &capi.SpaceQuotaApps{}
	quota.Services = &capi.SpaceQuotaServices{}
	quota.Routes = &capi.SpaceQuotaRoutes{}

	instanceMemoryLimit, err := config.ToMegabytes(input.InstanceMemoryLimit)
	if err != nil {
		return err
	}

	totalRoutes, err := config.ToInteger(input.TotalRoutes)
	if err != nil {
		return err
	}

	totalServices, err := config.ToInteger(input.TotalServices)
	if err != nil {
		return err
	}

	totalReservedRoutePorts, err := config.ToInteger(input.TotalReservedRoutePorts)
	if err != nil {
		return err
	}

	totalServiceKeys, err := config.ToInteger(input.TotalServiceKeys)
	if err != nil {
		return err
	}

	appInstanceLimit, err := config.ToInteger(input.AppInstanceLimit)
	if err != nil {
		return err
	}

	appTaskLimit, err := config.ToInteger(input.AppTaskLimit)
	if err != nil {
		return err
	}

	memoryLimit, err := config.ToMegabytes(input.MemoryLimit)
	if err != nil {
		return err
	}

	if input.IsUnlimitedMemory() {
		org, err := m.OrgReader.FindOrg(input.Org)
		if err != nil {
			return err
		}
		for _, orgQuota := range orgQuotas {
			if org.Relationships.Quota.Data.GUID == orgQuota.GUID {
				if orgQuota.Apps.TotalMemoryInMB == nil {
					memoryLimit = nil
				} else {
					memoryLimit = orgQuota.Apps.TotalMemoryInMB
				}
			}
		}
	}

	logRateLimit, err := config.ToInteger(input.LogRateLimitBytesPerSecond)
	if err != nil {
		return err
	}

	quota.Apps.TotalInstances = appInstanceLimit
	quota.Apps.PerAppTasks = appTaskLimit
	quota.Apps.TotalMemoryInMB = memoryLimit
	quota.Apps.PerProcessMemoryInMB = instanceMemoryLimit
	quota.Apps.LogRateLimitInBytesPerSecond = logRateLimit
	quota.Routes.TotalReservedPorts = totalReservedRoutePorts
	quota.Routes.TotalRoutes = totalRoutes
	quota.Services.PaidServicesAllowed = &input.PaidServicePlansAllowed
	quota.Services.TotalServiceInstances = totalServices
	quota.Services.TotalServiceKeys = totalServiceKeys

	if spaceQuota, ok := quotas[input.Name]; ok {
		if m.hasSpaceQuotaChanged(spaceQuota, quota) {
			if err := m.UpdateSpaceQuota(spaceQuota.GUID, quota); err != nil {
				return err
			}
		}
	} else {
		// create the quota already related to its space (the org relationship
		// comes from NewSpaceQuotaCreate) so a separate apply call is not needed
		quota.Relationships.Spaces = &capi.ToManyRelationship{Data: []capi.RelationshipData{{GUID: space.GUID}}}
		createdQuota, err := m.CreateSpaceQuota(quota)
		if err != nil {
			return err
		}
		space.Relationships.Quota = &resource.ToOneRelationship{Data: &resource.Relationship{GUID: createdQuota.GUID}}
		quotas[input.Name] = createdQuota
	}
	return nil
}

func (m *Manager) hasSpaceQuotaChanged(quota *capi.SpaceQuotaV3, newQuota *capi.SpaceQuotaV3CreateRequest) bool {
	if !reflect.DeepEqual(quota.Apps, newQuota.Apps) {
		m.debugCompareOutput("Apps Quota has changed from %s to %s", quota.Apps, newQuota.Apps)
		return true
	}
	if !reflect.DeepEqual(quota.Routes, newQuota.Routes) {
		m.debugCompareOutput("Routes Quota has changed from %s to %s", quota.Routes, newQuota.Routes)
		return true
	}
	if !reflect.DeepEqual(quota.Services, newQuota.Services) {
		m.debugCompareOutput("Services Quota has changed from %s to %s", quota.Services, newQuota.Services)
		return true
	}
	return false
}

func (m *Manager) debugCompareOutput(msg string, a interface{}, b interface{}) {
	aOutput, _ := json.Marshal(a)
	bOutput, _ := json.Marshal(b)
	lo.G.Debugf(msg, string(aOutput), string(bOutput))
}

func (m *Manager) ListAllSpaceQuotasForOrg(orgGUID string) (map[string]*capi.SpaceQuotaV3, error) {
	if m.Peek && strings.Contains(orgGUID, "dry-run-org-guid") {
		return make(map[string]*capi.SpaceQuotaV3), nil
	}
	if m.SpaceQuotas == nil {
		spaceQuotas, err := m.SpaceQuoteClient.ListAll(context.Background(), nil)
		if err != nil {
			return nil, err
		}
		spaceQuotaMap := make(map[string]map[string]*capi.SpaceQuotaV3)
		for _, spaceQuota := range spaceQuotas {
			orgGUID := spaceQuota.Relationships.Organization.Data.GUID
			if orgSpaceQuotaMap, ok := spaceQuotaMap[orgGUID]; ok {
				orgSpaceQuotaMap[spaceQuota.Name] = spaceQuota
			} else {
				orgSpaceQuotaMap := make(map[string]*capi.SpaceQuotaV3)
				orgSpaceQuotaMap[spaceQuota.Name] = spaceQuota
				spaceQuotaMap[orgGUID] = orgSpaceQuotaMap
			}
		}
		m.SpaceQuotas = spaceQuotaMap
	}
	spaceQuotas := m.SpaceQuotas[orgGUID]
	if spaceQuotas == nil {
		spaceQuotas = make(map[string]*capi.SpaceQuotaV3)
	}
	lo.G.Debug("Total space quotas returned :", len(spaceQuotas))
	return spaceQuotas, nil
}

func (m *Manager) UpdateSpaceQuota(quotaGUID string, quota *capi.SpaceQuotaV3CreateRequest) error {
	if m.Peek {
		lo.G.Infof("[dry-run]: update space quota %s", quota.Name)
		return nil
	}
	lo.G.Infof("Updating space quota %s", quota.Name)
	// the update request type carries no relationships, so an update can
	// never clobber the quota's org/space assignments
	update := &capi.SpaceQuotaV3UpdateRequest{
		Name:     &quota.Name,
		Apps:     quota.Apps,
		Services: quota.Services,
		Routes:   quota.Routes,
	}
	_, err := m.SpaceQuoteClient.Update(context.Background(), quotaGUID, update)
	return err
}

func (m *Manager) AssignQuotaToSpace(space *resource.Space, quota *capi.SpaceQuotaV3) error {
	if m.Peek {
		lo.G.Infof("[dry-run]: assigning quota %s to space %s", quota.Name, space.Name)
		return nil
	}
	lo.G.Infof("Assigning quota %s to %s", quota.Name, space.Name)
	_, err := m.SpaceQuoteClient.Apply(context.Background(), quota.GUID, []string{space.GUID})
	return err
}

func (m *Manager) CreateSpaceQuota(quota *capi.SpaceQuotaV3CreateRequest) (*capi.SpaceQuotaV3, error) {
	if m.Peek {
		lo.G.Infof("[dry-run]: creating quota %s", quota.Name)
		return &capi.SpaceQuotaV3{Name: "dry-run-quota", Resource: capi.Resource{GUID: "dry-run-guid"}}, nil
	}
	lo.G.Infof("Creating quota %s", quota.Name)
	spaceQuota, err := m.SpaceQuoteClient.Create(context.Background(), quota)
	if err != nil {
		return nil, err
	}
	return spaceQuota, nil
}

// CreateOrgQuotas -
func (m *Manager) CreateOrgQuotas() error {
	quotas, err := m.ListAllOrgQuotas()
	if err != nil {
		return err
	}

	orgQuotas, err := m.Cfg.GetOrgQuotas()
	if err != nil {
		return err
	}
	for _, orgQuotaConfig := range orgQuotas {
		err = m.createOrgQuota(orgQuotaConfig, quotas)
		if err != nil {
			return err
		}
	}
	orgs, err := m.Cfg.GetOrgConfigs()
	if err != nil {
		return err
	}

	for _, input := range orgs {
		if input.NamedQuota != "" && input.EnableOrgQuota {
			return fmt.Errorf("cannot have named quota %s and enable-org-quota for org %s", input.NamedQuota, input.Org)
		}
		if input.EnableOrgQuota || input.NamedQuota != "" {
			org, err := m.OrgReader.FindOrg(input.Org)
			if err != nil {
				return err
			}
			if input.EnableOrgQuota {
				quotaDef := input.GetQuota()
				err = m.createOrgQuota(quotaDef, quotas)
				if err != nil {
					return err
				}
				input.NamedQuota = input.Org
			}
			orgQuota := quotas[input.NamedQuota]
			if orgQuota != nil && (org.Relationships.Quota.Data == nil || org.Relationships.Quota.Data.GUID != orgQuota.GUID) {
				if err = m.AssignQuotaToOrg(org, orgQuota); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (m *Manager) createOrgQuota(input config.OrgQuota, quotas map[string]*capi.OrganizationQuota) error {

	quota := &capi.OrganizationQuotaCreateRequest{
		Name:     input.Name,
		Apps:     &capi.OrganizationQuotaApps{},
		Services: &capi.OrganizationQuotaServices{},
		Routes:   &capi.OrganizationQuotaRoutes{},
		Domains:  &capi.OrganizationQuotaDomains{},
	}
	memoryLimit, err := config.ToMegabytes(input.MemoryLimit)
	if err != nil {
		return err
	}

	instanceMemoryLimit, err := config.ToMegabytes(input.InstanceMemoryLimit)
	if err != nil {
		return err
	}

	totalRoutes, err := config.ToInteger(input.TotalRoutes)
	if err != nil {
		return err
	}

	totalServices, err := config.ToInteger(input.TotalServices)
	if err != nil {
		return err
	}

	totalReservedRoutePorts, err := config.ToInteger(input.TotalReservedRoutePorts)
	if err != nil {
		return err
	}

	totalServiceKeys, err := config.ToInteger(input.TotalServiceKeys)
	if err != nil {
		return err
	}

	appInstanceLimit, err := config.ToInteger(input.AppInstanceLimit)
	if err != nil {
		return err
	}

	appTaskLimit, err := config.ToInteger(input.AppTaskLimit)
	if err != nil {
		return err
	}

	totalPrivateDomains, err := config.ToInteger(input.TotalPrivateDomains)
	if err != nil {
		return err
	}

	logRateLimit, err := config.ToInteger(input.LogRateLimitBytesPerSecond)
	if err != nil {
		return err
	}

	quota.Apps.TotalInstances = appInstanceLimit
	quota.Apps.PerAppTasks = appTaskLimit
	quota.Apps.TotalMemoryInMB = memoryLimit
	quota.Apps.PerProcessMemoryInMB = instanceMemoryLimit
	quota.Apps.LogRateLimitInBytesPerSecond = logRateLimit
	quota.Routes.TotalReservedPorts = totalReservedRoutePorts
	quota.Routes.TotalRoutes = totalRoutes
	quota.Services.PaidServicesAllowed = &input.PaidServicePlansAllowed
	quota.Services.TotalServiceInstances = totalServices
	quota.Services.TotalServiceKeys = totalServiceKeys
	quota.Domains.TotalDomains = totalPrivateDomains

	if orgQuota, ok := quotas[input.Name]; ok {
		if m.hasOrgQuotaChanged(orgQuota, quota) {
			if err = m.UpdateOrgQuota(orgQuota.GUID, quota); err != nil {
				return err
			}
		}
	} else {
		createdQuota, err := m.CreateOrgQuota(quota)
		if err != nil {
			return err
		}
		quotas[input.Name] = createdQuota
	}

	return nil
}

func (m *Manager) hasOrgQuotaChanged(quota *capi.OrganizationQuota, newQuota *capi.OrganizationQuotaCreateRequest) bool {
	if !reflect.DeepEqual(quota.Apps, newQuota.Apps) {
		m.debugCompareOutput("Apps Quota has changed from %s to %s", quota.Apps, newQuota.Apps)
		return true
	}
	if !reflect.DeepEqual(quota.Routes, newQuota.Routes) {
		m.debugCompareOutput("Routes Quota has changed from %s to %s", quota.Routes, newQuota.Routes)
		return true
	}
	if !reflect.DeepEqual(quota.Services, newQuota.Services) {
		m.debugCompareOutput("Services Quota has changed from %s to %s", quota.Services, newQuota.Services)
		return true
	}
	if !reflect.DeepEqual(quota.Domains, newQuota.Domains) {
		m.debugCompareOutput("Domains Quota has changed from %s to %s", quota.Domains, newQuota.Domains)
		return true
	}
	return false
}

func (m *Manager) ListAllOrgQuotas() (map[string]*capi.OrganizationQuota, error) {
	quotas := make(map[string]*capi.OrganizationQuota)
	orgQutotas, err := m.OrgQuoteClient.ListAll(context.Background(), nil)
	if err != nil {
		return nil, err
	}
	lo.G.Debug("Total org quotas returned :", len(orgQutotas))
	for _, quota := range orgQutotas {
		quotas[quota.Name] = quota
	}
	return quotas, nil
}

func (m *Manager) CreateOrgQuota(quota *capi.OrganizationQuotaCreateRequest) (*capi.OrganizationQuota, error) {
	if m.Peek {
		lo.G.Infof("[dry-run]: create org quota %s", quota.Name)
		return &capi.OrganizationQuota{Name: "dry-run-quota", Resource: capi.Resource{GUID: "dry-run-quota-guid"}}, nil
	}

	lo.G.Infof("Creating org quota %s", quota.Name)
	orgQuota, err := m.OrgQuoteClient.Create(context.Background(), quota)
	if err != nil {
		return nil, err
	}
	return orgQuota, nil
}

func (m *Manager) UpdateOrgQuota(quotaGUID string, quota *capi.OrganizationQuotaCreateRequest) error {
	if m.Peek {
		lo.G.Infof("[dry-run]: update org quota %s", quota.Name)
		return nil
	}
	lo.G.Infof("Updating org quota %s", quota.Name)
	update := &capi.OrganizationQuotaUpdateRequest{
		Name:     &quota.Name,
		Apps:     quota.Apps,
		Services: quota.Services,
		Routes:   quota.Routes,
		Domains:  quota.Domains,
	}
	_, err := m.OrgQuoteClient.Update(context.Background(), quotaGUID, update)
	return err
}

func (m *Manager) AssignQuotaToOrg(org *resource.Organization, quota *capi.OrganizationQuota) error {
	if m.Peek {
		lo.G.Infof("[dry-run]: assign quota %s to org %s", quota.Name, org.Name)
		return nil
	}
	lo.G.Infof("Assigning quota %s to org %s", quota.Name, org.Name)
	_, err := m.OrgQuoteClient.Apply(context.Background(), quota.GUID, []string{org.GUID})
	return err
}

func (m *Manager) GetSpaceQuota(guid string) (*capi.SpaceQuotaV3, error) {
	return m.SpaceQuoteClient.Get(context.Background(), guid)
}

func (m *Manager) GetOrgQuota(guid string) (*capi.OrganizationQuota, error) {
	return m.OrgQuoteClient.Get(context.Background(), guid)
}
