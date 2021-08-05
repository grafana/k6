// Copyright 2020-2021 InfluxData, Inc. All rights reserved.
// Use of this source code is governed by MIT
// license that can be found in the LICENSE file.

package api

import (
	"context"
	"fmt"

	"github.com/influxdata/influxdb-client-go/v2/domain"
)

// OrganizationsAPI provides methods for managing Organizations in a InfluxDB server.
type OrganizationsAPI interface {
	// GetOrganizations returns all organizations.
	// GetOrganizations supports PagingOptions: Offset, Limit, Descending
	GetOrganizations(ctx context.Context, pagingOptions ...PagingOption) (*[]domain.Organization, error)
	// FindOrganizationByName returns an organization found using orgName.
	FindOrganizationByName(ctx context.Context, orgName string) (*domain.Organization, error)
	// FindOrganizationByID returns an organization found using orgID.
	FindOrganizationByID(ctx context.Context, orgID string) (*domain.Organization, error)
	// FindOrganizationsByUserID returns organizations an user with userID belongs to.
	// FindOrganizationsByUserID supports PagingOptions: Offset, Limit, Descending
	FindOrganizationsByUserID(ctx context.Context, userID string, pagingOptions ...PagingOption) (*[]domain.Organization, error)
	// CreateOrganization creates new organization.
	CreateOrganization(ctx context.Context, org *domain.Organization) (*domain.Organization, error)
	// CreateOrganizationWithName creates new organization with orgName and with status active.
	CreateOrganizationWithName(ctx context.Context, orgName string) (*domain.Organization, error)
	// UpdateOrganization updates organization.
	UpdateOrganization(ctx context.Context, org *domain.Organization) (*domain.Organization, error)
	// DeleteOrganization deletes an organization.
	DeleteOrganization(ctx context.Context, org *domain.Organization) error
	// DeleteOrganizationWithID deletes an organization with orgID.
	DeleteOrganizationWithID(ctx context.Context, orgID string) error
	// GetMembers returns members of an organization.
	GetMembers(ctx context.Context, org *domain.Organization) (*[]domain.ResourceMember, error)
	// GetMembersWithID returns members of an organization with orgID.
	GetMembersWithID(ctx context.Context, orgID string) (*[]domain.ResourceMember, error)
	// AddMember adds a member to an organization.
	AddMember(ctx context.Context, org *domain.Organization, user *domain.User) (*domain.ResourceMember, error)
	// AddMemberWithID adds a member with id memberID to an organization with orgID.
	AddMemberWithID(ctx context.Context, orgID, memberID string) (*domain.ResourceMember, error)
	// RemoveMember removes a member from an organization.
	RemoveMember(ctx context.Context, org *domain.Organization, user *domain.User) error
	// RemoveMemberWithID removes a member with id memberID from an organization with orgID.
	RemoveMemberWithID(ctx context.Context, orgID, memberID string) error
	// GetOwners returns owners of an organization.
	GetOwners(ctx context.Context, org *domain.Organization) (*[]domain.ResourceOwner, error)
	// GetOwnersWithID returns owners of an organization with orgID.
	GetOwnersWithID(ctx context.Context, orgID string) (*[]domain.ResourceOwner, error)
	// AddOwner adds an owner to an organization.
	AddOwner(ctx context.Context, org *domain.Organization, user *domain.User) (*domain.ResourceOwner, error)
	// AddOwnerWithID adds an owner with id memberID to an organization with orgID.
	AddOwnerWithID(ctx context.Context, orgID, memberID string) (*domain.ResourceOwner, error)
	// RemoveOwner removes an owner from an organization.
	RemoveOwner(ctx context.Context, org *domain.Organization, user *domain.User) error
	// RemoveOwnerWithID removes an owner with id memberID from an organization with orgID.
	RemoveOwnerWithID(ctx context.Context, orgID, memberID string) error
}

// organizationsAPI implements OrganizationsAPI
type organizationsAPI struct {
	apiClient *domain.ClientWithResponses
}

// NewOrganizationsAPI creates new instance of OrganizationsAPI
func NewOrganizationsAPI(apiClient *domain.ClientWithResponses) OrganizationsAPI {
	return &organizationsAPI{
		apiClient: apiClient,
	}
}

func (o *organizationsAPI) getOrganizations(ctx context.Context, params *domain.GetOrgsParams, pagingOptions ...PagingOption) (*[]domain.Organization, error) {
	options := defaultPaging()
	for _, opt := range pagingOptions {
		opt(options)
	}
	if options.limit > 0 {
		params.Limit = &options.limit
	}
	params.Offset = &options.offset
	params.Descending = &options.descending
	response, err := o.apiClient.GetOrgsWithResponse(ctx, params)
	if err != nil {
		return nil, err
	}
	if response.JSONDefault != nil {
		return nil, domain.ErrorToHTTPError(response.JSONDefault, response.StatusCode())
	}
	return response.JSON200.Orgs, nil
}
func (o *organizationsAPI) GetOrganizations(ctx context.Context, pagingOptions ...PagingOption) (*[]domain.Organization, error) {
	params := &domain.GetOrgsParams{}
	return o.getOrganizations(ctx, params, pagingOptions...)
}

func (o *organizationsAPI) FindOrganizationByName(ctx context.Context, orgName string) (*domain.Organization, error) {
	params := &domain.GetOrgsParams{Org: &orgName}
	organizations, err := o.getOrganizations(ctx, params)
	if err != nil {
		return nil, err
	}
	if organizations != nil && len(*organizations) > 0 {
		return &(*organizations)[0], nil
	}
	return nil, fmt.Errorf("organization '%s' not found", orgName)
}

func (o *organizationsAPI) FindOrganizationByID(ctx context.Context, orgID string) (*domain.Organization, error) {
	params := &domain.GetOrgsIDParams{}
	response, err := o.apiClient.GetOrgsIDWithResponse(ctx, orgID, params)
	if err != nil {
		return nil, err
	}
	if response.JSONDefault != nil {
		return nil, domain.ErrorToHTTPError(response.JSONDefault, response.StatusCode())
	}
	return response.JSON200, nil
}

func (o *organizationsAPI) FindOrganizationsByUserID(ctx context.Context, userID string, pagingOptions ...PagingOption) (*[]domain.Organization, error) {
	params := &domain.GetOrgsParams{UserID: &userID}
	return o.getOrganizations(ctx, params, pagingOptions...)
}

func (o *organizationsAPI) CreateOrganization(ctx context.Context, org *domain.Organization) (*domain.Organization, error) {
	params := &domain.PostOrgsParams{}
	req := domain.PostOrgsJSONRequestBody{
		Name:        org.Name,
		Description: org.Description,
	}
	response, err := o.apiClient.PostOrgsWithResponse(ctx, params, req)
	if err != nil {
		return nil, err
	}
	if response.JSONDefault != nil {
		return nil, domain.ErrorToHTTPError(response.JSONDefault, response.StatusCode())
	}
	return response.JSON201, nil
}

func (o *organizationsAPI) CreateOrganizationWithName(ctx context.Context, orgName string) (*domain.Organization, error) {
	status := domain.OrganizationStatusActive
	org := &domain.Organization{Name: orgName, Status: &status}
	return o.CreateOrganization(ctx, org)
}

func (o *organizationsAPI) DeleteOrganization(ctx context.Context, org *domain.Organization) error {
	return o.DeleteOrganizationWithID(ctx, *org.Id)
}

func (o *organizationsAPI) DeleteOrganizationWithID(ctx context.Context, orgID string) error {
	params := &domain.DeleteOrgsIDParams{}
	response, err := o.apiClient.DeleteOrgsIDWithResponse(ctx, orgID, params)
	if err != nil {
		return err
	}
	if response.JSONDefault != nil {
		return domain.ErrorToHTTPError(response.JSONDefault, response.StatusCode())
	}
	if response.JSON404 != nil {
		return domain.ErrorToHTTPError(response.JSON404, response.StatusCode())
	}
	return nil
}

func (o *organizationsAPI) UpdateOrganization(ctx context.Context, org *domain.Organization) (*domain.Organization, error) {
	params := &domain.PatchOrgsIDParams{}
	req := domain.PatchOrgsIDJSONRequestBody{
		Name:        &org.Name,
		Description: org.Description,
	}
	response, err := o.apiClient.PatchOrgsIDWithResponse(ctx, *org.Id, params, req)
	if err != nil {
		return nil, err
	}
	if response.JSONDefault != nil {
		return nil, domain.ErrorToHTTPError(response.JSONDefault, response.StatusCode())
	}
	return response.JSON200, nil
}

func (o *organizationsAPI) GetMembers(ctx context.Context, org *domain.Organization) (*[]domain.ResourceMember, error) {
	return o.GetMembersWithID(ctx, *org.Id)
}

func (o *organizationsAPI) GetMembersWithID(ctx context.Context, orgID string) (*[]domain.ResourceMember, error) {
	params := &domain.GetOrgsIDMembersParams{}
	response, err := o.apiClient.GetOrgsIDMembersWithResponse(ctx, orgID, params)
	if err != nil {
		return nil, err
	}
	if response.JSONDefault != nil {
		return nil, domain.ErrorToHTTPError(response.JSONDefault, response.StatusCode())
	}
	if response.JSON404 != nil {
		return nil, domain.ErrorToHTTPError(response.JSON404, response.StatusCode())
	}
	return response.JSON200.Users, nil
}

func (o *organizationsAPI) AddMember(ctx context.Context, org *domain.Organization, user *domain.User) (*domain.ResourceMember, error) {
	return o.AddMemberWithID(ctx, *org.Id, *user.Id)
}

func (o *organizationsAPI) AddMemberWithID(ctx context.Context, orgID, memberID string) (*domain.ResourceMember, error) {
	params := &domain.PostOrgsIDMembersParams{}
	body := &domain.PostOrgsIDMembersJSONRequestBody{Id: memberID}
	response, err := o.apiClient.PostOrgsIDMembersWithResponse(ctx, orgID, params, *body)
	if err != nil {
		return nil, err
	}
	if response.JSONDefault != nil {
		return nil, domain.ErrorToHTTPError(response.JSONDefault, response.StatusCode())
	}
	return response.JSON201, nil
}

func (o *organizationsAPI) RemoveMember(ctx context.Context, org *domain.Organization, user *domain.User) error {
	return o.RemoveMemberWithID(ctx, *org.Id, *user.Id)
}

func (o *organizationsAPI) RemoveMemberWithID(ctx context.Context, orgID, memberID string) error {
	params := &domain.DeleteOrgsIDMembersIDParams{}
	response, err := o.apiClient.DeleteOrgsIDMembersIDWithResponse(ctx, orgID, memberID, params)
	if err != nil {
		return err
	}
	if response.JSONDefault != nil {
		return domain.ErrorToHTTPError(response.JSONDefault, response.StatusCode())
	}
	return nil
}

func (o *organizationsAPI) GetOwners(ctx context.Context, org *domain.Organization) (*[]domain.ResourceOwner, error) {
	return o.GetOwnersWithID(ctx, *org.Id)
}

func (o *organizationsAPI) GetOwnersWithID(ctx context.Context, orgID string) (*[]domain.ResourceOwner, error) {
	params := &domain.GetOrgsIDOwnersParams{}
	response, err := o.apiClient.GetOrgsIDOwnersWithResponse(ctx, orgID, params)
	if err != nil {
		return nil, err
	}
	if response.JSONDefault != nil {
		return nil, domain.ErrorToHTTPError(response.JSONDefault, response.StatusCode())
	}
	if response.JSON404 != nil {
		return nil, domain.ErrorToHTTPError(response.JSON404, response.StatusCode())
	}
	return response.JSON200.Users, nil
}

func (o *organizationsAPI) AddOwner(ctx context.Context, org *domain.Organization, user *domain.User) (*domain.ResourceOwner, error) {
	return o.AddOwnerWithID(ctx, *org.Id, *user.Id)
}

func (o *organizationsAPI) AddOwnerWithID(ctx context.Context, orgID, memberID string) (*domain.ResourceOwner, error) {
	params := &domain.PostOrgsIDOwnersParams{}
	body := &domain.PostOrgsIDOwnersJSONRequestBody{Id: memberID}
	response, err := o.apiClient.PostOrgsIDOwnersWithResponse(ctx, orgID, params, *body)
	if err != nil {
		return nil, err
	}
	if response.JSONDefault != nil {
		return nil, domain.ErrorToHTTPError(response.JSONDefault, response.StatusCode())
	}
	return response.JSON201, nil
}

func (o *organizationsAPI) RemoveOwner(ctx context.Context, org *domain.Organization, user *domain.User) error {
	return o.RemoveOwnerWithID(ctx, *org.Id, *user.Id)
}

func (o *organizationsAPI) RemoveOwnerWithID(ctx context.Context, orgID, memberID string) error {
	params := &domain.DeleteOrgsIDOwnersIDParams{}
	response, err := o.apiClient.DeleteOrgsIDOwnersIDWithResponse(ctx, orgID, memberID, params)
	if err != nil {
		return err
	}
	if response.JSONDefault != nil {
		return domain.ErrorToHTTPError(response.JSONDefault, response.StatusCode())
	}
	return nil
}
