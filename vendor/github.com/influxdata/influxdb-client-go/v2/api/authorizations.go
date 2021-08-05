// Copyright 2020-2021 InfluxData, Inc. All rights reserved.
// Use of this source code is governed by MIT
// license that can be found in the LICENSE file.

package api

import (
	"context"

	"github.com/influxdata/influxdb-client-go/v2/domain"
)

// AuthorizationsAPI provides methods for organizing Authorization in a InfluxDB server
type AuthorizationsAPI interface {
	// GetAuthorizations returns all authorizations
	GetAuthorizations(ctx context.Context) (*[]domain.Authorization, error)
	// FindAuthorizationsByUserName returns all authorizations for given userName
	FindAuthorizationsByUserName(ctx context.Context, userName string) (*[]domain.Authorization, error)
	// FindAuthorizationsByUserID returns all authorizations for given userID
	FindAuthorizationsByUserID(ctx context.Context, userID string) (*[]domain.Authorization, error)
	// FindAuthorizationsByOrgName returns all authorizations for given organization name
	FindAuthorizationsByOrgName(ctx context.Context, orgName string) (*[]domain.Authorization, error)
	// FindAuthorizationsByOrgID returns all authorizations for given organization id
	FindAuthorizationsByOrgID(ctx context.Context, orgID string) (*[]domain.Authorization, error)
	// CreateAuthorization creates new authorization
	CreateAuthorization(ctx context.Context, authorization *domain.Authorization) (*domain.Authorization, error)
	// CreateAuthorizationWithOrgID creates new authorization with given permissions scoped to given orgID
	CreateAuthorizationWithOrgID(ctx context.Context, orgID string, permissions []domain.Permission) (*domain.Authorization, error)
	// UpdateAuthorizationStatus updates status of authorization
	UpdateAuthorizationStatus(ctx context.Context, authorization *domain.Authorization, status domain.AuthorizationUpdateRequestStatus) (*domain.Authorization, error)
	// UpdateAuthorizationStatusWithID updates status of authorization with authID
	UpdateAuthorizationStatusWithID(ctx context.Context, authID string, status domain.AuthorizationUpdateRequestStatus) (*domain.Authorization, error)
	// DeleteAuthorization deletes authorization
	DeleteAuthorization(ctx context.Context, authorization *domain.Authorization) error
	// DeleteAuthorization deletes authorization with authID
	DeleteAuthorizationWithID(ctx context.Context, authID string) error
}

// authorizationsAPI implements AuthorizationsAPI
type authorizationsAPI struct {
	apiClient *domain.ClientWithResponses
}

// NewAuthorizationsAPI creates new instance of AuthorizationsAPI
func NewAuthorizationsAPI(apiClient *domain.ClientWithResponses) AuthorizationsAPI {
	return &authorizationsAPI{
		apiClient: apiClient,
	}
}

func (a *authorizationsAPI) GetAuthorizations(ctx context.Context) (*[]domain.Authorization, error) {
	authQuery := &domain.GetAuthorizationsParams{}
	return a.listAuthorizations(ctx, authQuery)
}

func (a *authorizationsAPI) FindAuthorizationsByUserName(ctx context.Context, userName string) (*[]domain.Authorization, error) {
	authQuery := &domain.GetAuthorizationsParams{User: &userName}
	return a.listAuthorizations(ctx, authQuery)
}

func (a *authorizationsAPI) FindAuthorizationsByUserID(ctx context.Context, userID string) (*[]domain.Authorization, error) {
	authQuery := &domain.GetAuthorizationsParams{UserID: &userID}
	return a.listAuthorizations(ctx, authQuery)
}

func (a *authorizationsAPI) FindAuthorizationsByOrgName(ctx context.Context, orgName string) (*[]domain.Authorization, error) {
	authQuery := &domain.GetAuthorizationsParams{Org: &orgName}
	return a.listAuthorizations(ctx, authQuery)
}

func (a *authorizationsAPI) FindAuthorizationsByOrgID(ctx context.Context, orgID string) (*[]domain.Authorization, error) {
	authQuery := &domain.GetAuthorizationsParams{OrgID: &orgID}
	return a.listAuthorizations(ctx, authQuery)
}

func (a *authorizationsAPI) listAuthorizations(ctx context.Context, query *domain.GetAuthorizationsParams) (*[]domain.Authorization, error) {
	response, err := a.apiClient.GetAuthorizationsWithResponse(ctx, query)
	if err != nil {
		return nil, err
	}
	if response.JSONDefault != nil {
		return nil, domain.ErrorToHTTPError(response.JSONDefault, response.StatusCode())
	}
	return response.JSON200.Authorizations, nil
}

func (a *authorizationsAPI) CreateAuthorization(ctx context.Context, authorization *domain.Authorization) (*domain.Authorization, error) {
	params := &domain.PostAuthorizationsParams{}
	req := domain.PostAuthorizationsJSONRequestBody{
		AuthorizationUpdateRequest: authorization.AuthorizationUpdateRequest,
		OrgID:                      authorization.OrgID,
		Permissions:                authorization.Permissions,
		UserID:                     authorization.UserID,
	}
	response, err := a.apiClient.PostAuthorizationsWithResponse(ctx, params, req)
	if err != nil {
		return nil, err
	}
	if response.JSONDefault != nil {
		return nil, domain.ErrorToHTTPError(response.JSONDefault, response.StatusCode())
	}
	if response.JSON400 != nil {
		return nil, domain.ErrorToHTTPError(response.JSON400, response.StatusCode())
	}
	return response.JSON201, nil
}

func (a *authorizationsAPI) CreateAuthorizationWithOrgID(ctx context.Context, orgID string, permissions []domain.Permission) (*domain.Authorization, error) {
	status := domain.AuthorizationUpdateRequestStatusActive
	auth := &domain.Authorization{
		AuthorizationUpdateRequest: domain.AuthorizationUpdateRequest{Status: &status},
		OrgID:                      &orgID,
		Permissions:                &permissions,
	}
	return a.CreateAuthorization(ctx, auth)
}

func (a *authorizationsAPI) UpdateAuthorizationStatusWithID(ctx context.Context, authID string, status domain.AuthorizationUpdateRequestStatus) (*domain.Authorization, error) {
	params := &domain.PatchAuthorizationsIDParams{}
	body := &domain.PatchAuthorizationsIDJSONRequestBody{Status: &status}
	response, err := a.apiClient.PatchAuthorizationsIDWithResponse(ctx, authID, params, *body)
	if err != nil {
		return nil, err
	}
	if response.JSONDefault != nil {
		return nil, domain.ErrorToHTTPError(response.JSONDefault, response.StatusCode())
	}
	return response.JSON200, nil
}

func (a *authorizationsAPI) UpdateAuthorizationStatus(ctx context.Context, authorization *domain.Authorization, status domain.AuthorizationUpdateRequestStatus) (*domain.Authorization, error) {
	return a.UpdateAuthorizationStatusWithID(ctx, *authorization.Id, status)
}

func (a *authorizationsAPI) DeleteAuthorization(ctx context.Context, authorization *domain.Authorization) error {
	return a.DeleteAuthorizationWithID(ctx, *authorization.Id)
}

func (a *authorizationsAPI) DeleteAuthorizationWithID(ctx context.Context, authID string) error {
	params := &domain.DeleteAuthorizationsIDParams{}
	response, err := a.apiClient.DeleteAuthorizationsIDWithResponse(ctx, authID, params)
	if err != nil {
		return err
	}
	if response.JSONDefault != nil {
		return domain.ErrorToHTTPError(response.JSONDefault, response.StatusCode())
	}
	return nil
}
