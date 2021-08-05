// Copyright 2020-2021 InfluxData, Inc. All rights reserved.
// Use of this source code is governed by MIT
// license that can be found in the LICENSE file.

package api

import (
	"context"
	"fmt"

	"github.com/influxdata/influxdb-client-go/v2/domain"
)

// LabelsAPI provides methods for managing labels in a InfluxDB server.
type LabelsAPI interface {
	// GetLabels returns all labels.
	GetLabels(ctx context.Context) (*[]domain.Label, error)
	// FindLabelsByOrg returns labels belonging to organization org.
	FindLabelsByOrg(ctx context.Context, org *domain.Organization) (*[]domain.Label, error)
	// FindLabelsByOrgID returns labels belonging to organization with id orgID.
	FindLabelsByOrgID(ctx context.Context, orgID string) (*[]domain.Label, error)
	// FindLabelByID returns a label with labelID.
	FindLabelByID(ctx context.Context, labelID string) (*domain.Label, error)
	// FindLabelByName returns a label with name labelName under an organization orgID.
	FindLabelByName(ctx context.Context, orgID, labelName string) (*domain.Label, error)
	// CreateLabel creates a new label.
	CreateLabel(ctx context.Context, label *domain.LabelCreateRequest) (*domain.Label, error)
	// CreateLabelWithName creates a new label with label labelName and properties, under the organization org.
	// Properties example: {"color": "ffb3b3", "description": "this is a description"}.
	CreateLabelWithName(ctx context.Context, org *domain.Organization, labelName string, properties map[string]string) (*domain.Label, error)
	// CreateLabelWithName creates a new label with label labelName and properties, under the organization with id orgID.
	// Properties example: {"color": "ffb3b3", "description": "this is a description"}.
	CreateLabelWithNameWithID(ctx context.Context, orgID, labelName string, properties map[string]string) (*domain.Label, error)
	// UpdateLabel updates the label.
	// Properties can be removed by sending an update with an empty value.
	UpdateLabel(ctx context.Context, label *domain.Label) (*domain.Label, error)
	// DeleteLabelWithID deletes a label with labelID.
	DeleteLabelWithID(ctx context.Context, labelID string) error
	// DeleteLabel deletes a label.
	DeleteLabel(ctx context.Context, label *domain.Label) error
}

// labelsAPI implements LabelsAPI
type labelsAPI struct {
	apiClient *domain.ClientWithResponses
}

// NewLabelsAPI creates new instance of LabelsAPI
func NewLabelsAPI(apiClient *domain.ClientWithResponses) LabelsAPI {
	return &labelsAPI{
		apiClient: apiClient,
	}
}

func (u *labelsAPI) GetLabels(ctx context.Context) (*[]domain.Label, error) {
	params := &domain.GetLabelsParams{}
	return u.getLabels(ctx, params)
}

func (u *labelsAPI) getLabels(ctx context.Context, params *domain.GetLabelsParams) (*[]domain.Label, error) {
	response, err := u.apiClient.GetLabelsWithResponse(ctx, params)
	if err != nil {
		return nil, err
	}
	if response.JSONDefault != nil {
		return nil, domain.ErrorToHTTPError(response.JSONDefault, response.StatusCode())
	}
	return (*[]domain.Label)(response.JSON200.Labels), nil
}

func (u *labelsAPI) FindLabelsByOrg(ctx context.Context, org *domain.Organization) (*[]domain.Label, error) {
	return u.FindLabelsByOrgID(ctx, *org.Id)
}

func (u *labelsAPI) FindLabelsByOrgID(ctx context.Context, orgID string) (*[]domain.Label, error) {
	params := &domain.GetLabelsParams{OrgID: &orgID}
	return u.getLabels(ctx, params)
}

func (u *labelsAPI) FindLabelByID(ctx context.Context, labelID string) (*domain.Label, error) {
	params := &domain.GetLabelsIDParams{}
	response, err := u.apiClient.GetLabelsIDWithResponse(ctx, labelID, params)
	if err != nil {
		return nil, err
	}
	if response.JSONDefault != nil {
		return nil, domain.ErrorToHTTPError(response.JSONDefault, response.StatusCode())
	}
	return response.JSON200.Label, nil
}

func (u *labelsAPI) FindLabelByName(ctx context.Context, orgID, labelName string) (*domain.Label, error) {
	labels, err := u.FindLabelsByOrgID(ctx, orgID)
	if err != nil {
		return nil, err
	}
	var label *domain.Label
	for _, u := range *labels {
		if *u.Name == labelName {
			label = &u
			break
		}
	}
	if label == nil {
		return nil, fmt.Errorf("label '%s' not found", labelName)
	}
	return label, nil
}

func (u *labelsAPI) CreateLabelWithName(ctx context.Context, org *domain.Organization, labelName string, properties map[string]string) (*domain.Label, error) {
	return u.CreateLabelWithNameWithID(ctx, *org.Id, labelName, properties)
}

func (u *labelsAPI) CreateLabelWithNameWithID(ctx context.Context, orgID, labelName string, properties map[string]string) (*domain.Label, error) {
	props := &domain.LabelCreateRequest_Properties{AdditionalProperties: properties}
	label := &domain.LabelCreateRequest{Name: labelName, OrgID: orgID, Properties: props}
	return u.CreateLabel(ctx, label)
}

func (u *labelsAPI) CreateLabel(ctx context.Context, label *domain.LabelCreateRequest) (*domain.Label, error) {
	response, err := u.apiClient.PostLabelsWithResponse(ctx, domain.PostLabelsJSONRequestBody(*label))
	if err != nil {
		return nil, err
	}
	if response.JSONDefault != nil {
		return nil, domain.ErrorToHTTPError(response.JSONDefault, response.StatusCode())
	}
	return response.JSON201.Label, nil
}

func (u *labelsAPI) UpdateLabel(ctx context.Context, label *domain.Label) (*domain.Label, error) {
	var props *domain.LabelUpdate_Properties
	params := &domain.PatchLabelsIDParams{}
	if label.Properties != nil {
		props = &domain.LabelUpdate_Properties{AdditionalProperties: label.Properties.AdditionalProperties}
	}
	body := &domain.LabelUpdate{
		Name:       label.Name,
		Properties: props,
	}
	response, err := u.apiClient.PatchLabelsIDWithResponse(ctx, *label.Id, params, domain.PatchLabelsIDJSONRequestBody(*body))
	if err != nil {
		return nil, err
	}
	if response.JSON404 != nil {
		return nil, domain.ErrorToHTTPError(response.JSON404, response.StatusCode())
	}
	if response.JSONDefault != nil {
		return nil, domain.ErrorToHTTPError(response.JSONDefault, response.StatusCode())
	}
	return response.JSON200.Label, nil
}

func (u *labelsAPI) DeleteLabel(ctx context.Context, label *domain.Label) error {
	return u.DeleteLabelWithID(ctx, *label.Id)
}

func (u *labelsAPI) DeleteLabelWithID(ctx context.Context, labelID string) error {
	params := &domain.DeleteLabelsIDParams{}
	response, err := u.apiClient.DeleteLabelsIDWithResponse(ctx, labelID, params)
	if err != nil {
		return err
	}
	if response.JSON404 != nil {
		return domain.ErrorToHTTPError(response.JSON404, response.StatusCode())
	}
	if response.JSONDefault != nil {
		return domain.ErrorToHTTPError(response.JSONDefault, response.StatusCode())
	}
	return nil
}
