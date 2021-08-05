// Copyright 2020-2021 InfluxData, Inc. All rights reserved.
// Use of this source code is governed by MIT
// license that can be found in the LICENSE file.

package api

import (
	"context"
	"fmt"
	"github.com/influxdata/influxdb-client-go/v2/domain"
)

// BucketsAPI provides methods for managing Buckets in a InfluxDB server.
type BucketsAPI interface {
	// GetBuckets returns all buckets.
	// GetBuckets supports PagingOptions: Offset, Limit, After. Empty pagingOptions means the default paging (first 20 results).
	GetBuckets(ctx context.Context, pagingOptions ...PagingOption) (*[]domain.Bucket, error)
	// FindBucketByName returns a bucket found using bucketName.
	FindBucketByName(ctx context.Context, bucketName string) (*domain.Bucket, error)
	// FindBucketByID returns a bucket found using bucketID.
	FindBucketByID(ctx context.Context, bucketID string) (*domain.Bucket, error)
	// FindBucketsByOrgID returns buckets belonging to the organization with ID orgID.
	// FindBucketsByOrgID supports PagingOptions: Offset, Limit, After. Empty pagingOptions means the default paging (first 20 results).
	FindBucketsByOrgID(ctx context.Context, orgID string, pagingOptions ...PagingOption) (*[]domain.Bucket, error)
	// FindBucketsByOrgName returns buckets belonging to the organization with name orgName, with the specified paging. Empty pagingOptions means the default paging (first 20 results).
	FindBucketsByOrgName(ctx context.Context, orgName string, pagingOptions ...PagingOption) (*[]domain.Bucket, error)
	// CreateBucket creates a new bucket.
	CreateBucket(ctx context.Context, bucket *domain.Bucket) (*domain.Bucket, error)
	// CreateBucketWithName creates a new bucket with bucketName in organization org, with retention specified in rules. Empty rules means infinite retention.
	CreateBucketWithName(ctx context.Context, org *domain.Organization, bucketName string, rules ...domain.RetentionRule) (*domain.Bucket, error)
	// CreateBucketWithNameWithID creates a new bucket with bucketName in organization with orgID, with retention specified in rules. Empty rules means infinite retention.
	CreateBucketWithNameWithID(ctx context.Context, orgID, bucketName string, rules ...domain.RetentionRule) (*domain.Bucket, error)
	// UpdateBucket updates a bucket.
	UpdateBucket(ctx context.Context, bucket *domain.Bucket) (*domain.Bucket, error)
	// DeleteBucket deletes a bucket.
	DeleteBucket(ctx context.Context, bucket *domain.Bucket) error
	// DeleteBucketWithID deletes a bucket with bucketID.
	DeleteBucketWithID(ctx context.Context, bucketID string) error
	// GetMembers returns members of a bucket.
	GetMembers(ctx context.Context, bucket *domain.Bucket) (*[]domain.ResourceMember, error)
	// GetMembersWithID returns members of a bucket with bucketID.
	GetMembersWithID(ctx context.Context, bucketID string) (*[]domain.ResourceMember, error)
	// AddMember adds a member to a bucket.
	AddMember(ctx context.Context, bucket *domain.Bucket, user *domain.User) (*domain.ResourceMember, error)
	// AddMemberWithID adds a member with id memberID to a bucket with bucketID.
	AddMemberWithID(ctx context.Context, bucketID, memberID string) (*domain.ResourceMember, error)
	// RemoveMember removes a member from a bucket.
	RemoveMember(ctx context.Context, bucket *domain.Bucket, user *domain.User) error
	// RemoveMemberWithID removes a member with id memberID from a bucket with bucketID.
	RemoveMemberWithID(ctx context.Context, bucketID, memberID string) error
	// GetOwners returns owners of a bucket.
	GetOwners(ctx context.Context, bucket *domain.Bucket) (*[]domain.ResourceOwner, error)
	// GetOwnersWithID returns owners of a bucket with bucketID.
	GetOwnersWithID(ctx context.Context, bucketID string) (*[]domain.ResourceOwner, error)
	// AddOwner adds an owner to a bucket.
	AddOwner(ctx context.Context, bucket *domain.Bucket, user *domain.User) (*domain.ResourceOwner, error)
	// AddOwnerWithID adds an owner with id memberID to a bucket with bucketID.
	AddOwnerWithID(ctx context.Context, bucketID, memberID string) (*domain.ResourceOwner, error)
	// RemoveOwner removes an owner from a bucket.
	RemoveOwner(ctx context.Context, bucket *domain.Bucket, user *domain.User) error
	// RemoveOwnerWithID removes a member with id memberID from a bucket with bucketID.
	RemoveOwnerWithID(ctx context.Context, bucketID, memberID string) error
}

// bucketsAPI implements BucketsAPI
type bucketsAPI struct {
	apiClient *domain.ClientWithResponses
}

// NewBucketsAPI creates new instance of BucketsAPI
func NewBucketsAPI(apiClient *domain.ClientWithResponses) BucketsAPI {
	return &bucketsAPI{
		apiClient: apiClient,
	}
}

func (b *bucketsAPI) GetBuckets(ctx context.Context, pagingOptions ...PagingOption) (*[]domain.Bucket, error) {
	return b.getBuckets(ctx, nil, pagingOptions...)
}

func (b *bucketsAPI) getBuckets(ctx context.Context, params *domain.GetBucketsParams, pagingOptions ...PagingOption) (*[]domain.Bucket, error) {
	if params == nil {
		params = &domain.GetBucketsParams{}
	}
	options := defaultPaging()
	for _, opt := range pagingOptions {
		opt(options)
	}
	if options.limit > 0 {
		params.Limit = &options.limit
	}
	params.Offset = &options.offset

	response, err := b.apiClient.GetBucketsWithResponse(ctx, params)
	if err != nil {
		return nil, err
	}
	if response.JSONDefault != nil {
		return nil, domain.ErrorToHTTPError(response.JSONDefault, response.StatusCode())
	}
	return response.JSON200.Buckets, nil
}

func (b *bucketsAPI) FindBucketByName(ctx context.Context, bucketName string) (*domain.Bucket, error) {
	params := &domain.GetBucketsParams{Name: &bucketName}
	response, err := b.apiClient.GetBucketsWithResponse(ctx, params)
	if err != nil {
		return nil, err
	}
	if response.JSONDefault != nil {
		return nil, domain.ErrorToHTTPError(response.JSONDefault, response.StatusCode())
	}
	if response.JSON200.Buckets != nil && len(*response.JSON200.Buckets) > 0 {
		return &(*response.JSON200.Buckets)[0], nil
	}
	return nil, fmt.Errorf("bucket '%s' not found", bucketName)
}

func (b *bucketsAPI) FindBucketByID(ctx context.Context, bucketID string) (*domain.Bucket, error) {
	params := &domain.GetBucketsIDParams{}
	response, err := b.apiClient.GetBucketsIDWithResponse(ctx, bucketID, params)
	if err != nil {
		return nil, err
	}
	if response.JSONDefault != nil {
		return nil, domain.ErrorToHTTPError(response.JSONDefault, response.StatusCode())
	}
	return response.JSON200, nil
}

func (b *bucketsAPI) FindBucketsByOrgID(ctx context.Context, orgID string, pagingOptions ...PagingOption) (*[]domain.Bucket, error) {
	params := &domain.GetBucketsParams{OrgID: &orgID}
	return b.getBuckets(ctx, params, pagingOptions...)
}

func (b *bucketsAPI) FindBucketsByOrgName(ctx context.Context, orgName string, pagingOptions ...PagingOption) (*[]domain.Bucket, error) {
	params := &domain.GetBucketsParams{Org: &orgName}
	return b.getBuckets(ctx, params, pagingOptions...)
}

func (b *bucketsAPI) createBucket(ctx context.Context, bucketReq *domain.PostBucketRequest) (*domain.Bucket, error) {
	params := &domain.PostBucketsParams{}
	response, err := b.apiClient.PostBucketsWithResponse(ctx, params, domain.PostBucketsJSONRequestBody(*bucketReq))
	if err != nil {
		return nil, err
	}
	if response.JSON422 != nil {
		return nil, domain.ErrorToHTTPError(response.JSON422, response.StatusCode())
	}
	if response.JSONDefault != nil {
		return nil, domain.ErrorToHTTPError(response.JSONDefault, response.StatusCode())
	}
	return response.JSON201, nil
}

func (b *bucketsAPI) CreateBucket(ctx context.Context, bucket *domain.Bucket) (*domain.Bucket, error) {
	bucketReq := &domain.PostBucketRequest{
		Description:    bucket.Description,
		Name:           bucket.Name,
		OrgID:          *bucket.OrgID,
		RetentionRules: bucket.RetentionRules,
		Rp:             bucket.Rp,
	}
	return b.createBucket(ctx, bucketReq)
}

func (b *bucketsAPI) CreateBucketWithNameWithID(ctx context.Context, orgID, bucketName string, rules ...domain.RetentionRule) (*domain.Bucket, error) {
	bucket := &domain.PostBucketRequest{Name: bucketName, OrgID: orgID, RetentionRules: rules}
	return b.createBucket(ctx, bucket)
}
func (b *bucketsAPI) CreateBucketWithName(ctx context.Context, org *domain.Organization, bucketName string, rules ...domain.RetentionRule) (*domain.Bucket, error) {
	return b.CreateBucketWithNameWithID(ctx, *org.Id, bucketName, rules...)
}

func (b *bucketsAPI) DeleteBucket(ctx context.Context, bucket *domain.Bucket) error {
	return b.DeleteBucketWithID(ctx, *bucket.Id)
}

func (b *bucketsAPI) DeleteBucketWithID(ctx context.Context, bucketID string) error {
	params := &domain.DeleteBucketsIDParams{}
	response, err := b.apiClient.DeleteBucketsIDWithResponse(ctx, bucketID, params)
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

func (b *bucketsAPI) UpdateBucket(ctx context.Context, bucket *domain.Bucket) (*domain.Bucket, error) {
	params := &domain.PatchBucketsIDParams{}
	req := domain.PatchBucketsIDJSONRequestBody{
		Description:    bucket.Description,
		Name:           &bucket.Name,
		RetentionRules: retentionRulesToPatchRetentionRules(&bucket.RetentionRules),
	}
	response, err := b.apiClient.PatchBucketsIDWithResponse(ctx, *bucket.Id, params, req)
	if err != nil {
		return nil, err
	}
	if response.JSONDefault != nil {
		return nil, domain.ErrorToHTTPError(response.JSONDefault, response.StatusCode())
	}
	return response.JSON200, nil
}

func (b *bucketsAPI) GetMembers(ctx context.Context, bucket *domain.Bucket) (*[]domain.ResourceMember, error) {
	return b.GetMembersWithID(ctx, *bucket.Id)
}

func (b *bucketsAPI) GetMembersWithID(ctx context.Context, bucketID string) (*[]domain.ResourceMember, error) {
	params := &domain.GetBucketsIDMembersParams{}
	response, err := b.apiClient.GetBucketsIDMembersWithResponse(ctx, bucketID, params)
	if err != nil {
		return nil, err
	}
	if response.JSONDefault != nil {
		return nil, domain.ErrorToHTTPError(response.JSONDefault, response.StatusCode())
	}
	return response.JSON200.Users, nil
}

func (b *bucketsAPI) AddMember(ctx context.Context, bucket *domain.Bucket, user *domain.User) (*domain.ResourceMember, error) {
	return b.AddMemberWithID(ctx, *bucket.Id, *user.Id)
}

func (b *bucketsAPI) AddMemberWithID(ctx context.Context, bucketID, memberID string) (*domain.ResourceMember, error) {
	params := &domain.PostBucketsIDMembersParams{}
	body := &domain.PostBucketsIDMembersJSONRequestBody{Id: memberID}
	response, err := b.apiClient.PostBucketsIDMembersWithResponse(ctx, bucketID, params, *body)
	if err != nil {
		return nil, err
	}
	if response.JSONDefault != nil {
		return nil, domain.ErrorToHTTPError(response.JSONDefault, response.StatusCode())
	}
	return response.JSON201, nil
}

func (b *bucketsAPI) RemoveMember(ctx context.Context, bucket *domain.Bucket, user *domain.User) error {
	return b.RemoveMemberWithID(ctx, *bucket.Id, *user.Id)
}

func (b *bucketsAPI) RemoveMemberWithID(ctx context.Context, bucketID, memberID string) error {
	params := &domain.DeleteBucketsIDMembersIDParams{}
	response, err := b.apiClient.DeleteBucketsIDMembersIDWithResponse(ctx, bucketID, memberID, params)
	if err != nil {
		return err
	}
	if response.JSONDefault != nil {
		return domain.ErrorToHTTPError(response.JSONDefault, response.StatusCode())
	}
	return nil
}

func (b *bucketsAPI) GetOwners(ctx context.Context, bucket *domain.Bucket) (*[]domain.ResourceOwner, error) {
	return b.GetOwnersWithID(ctx, *bucket.Id)
}

func (b *bucketsAPI) GetOwnersWithID(ctx context.Context, bucketID string) (*[]domain.ResourceOwner, error) {
	params := &domain.GetBucketsIDOwnersParams{}
	response, err := b.apiClient.GetBucketsIDOwnersWithResponse(ctx, bucketID, params)
	if err != nil {
		return nil, err
	}
	if response.JSONDefault != nil {
		return nil, domain.ErrorToHTTPError(response.JSONDefault, response.StatusCode())
	}
	return response.JSON200.Users, nil
}

func (b *bucketsAPI) AddOwner(ctx context.Context, bucket *domain.Bucket, user *domain.User) (*domain.ResourceOwner, error) {
	return b.AddOwnerWithID(ctx, *bucket.Id, *user.Id)
}

func (b *bucketsAPI) AddOwnerWithID(ctx context.Context, bucketID, memberID string) (*domain.ResourceOwner, error) {
	params := &domain.PostBucketsIDOwnersParams{}
	body := &domain.PostBucketsIDOwnersJSONRequestBody{Id: memberID}
	response, err := b.apiClient.PostBucketsIDOwnersWithResponse(ctx, bucketID, params, *body)
	if err != nil {
		return nil, err
	}
	if response.JSONDefault != nil {
		return nil, domain.ErrorToHTTPError(response.JSONDefault, response.StatusCode())
	}
	return response.JSON201, nil
}

func (b *bucketsAPI) RemoveOwner(ctx context.Context, bucket *domain.Bucket, user *domain.User) error {
	return b.RemoveOwnerWithID(ctx, *bucket.Id, *user.Id)
}

func (b *bucketsAPI) RemoveOwnerWithID(ctx context.Context, bucketID, memberID string) error {
	params := &domain.DeleteBucketsIDOwnersIDParams{}
	response, err := b.apiClient.DeleteBucketsIDOwnersIDWithResponse(ctx, bucketID, memberID, params)
	if err != nil {
		return err
	}
	if response.JSONDefault != nil {
		return domain.ErrorToHTTPError(response.JSONDefault, response.StatusCode())
	}
	return nil
}

func retentionRulesToPatchRetentionRules(rrs *domain.RetentionRules) *domain.PatchRetentionRules {
	if rrs == nil {
		return nil
	}
	prrs := make([]domain.PatchRetentionRule, len(*rrs))
	for i, rr := range *rrs {
		prrs[i] = domain.PatchRetentionRule{
			EverySeconds:              &rr.EverySeconds,
			ShardGroupDurationSeconds: rr.ShardGroupDurationSeconds,
			Type:                      domain.PatchRetentionRuleType(rr.Type),
		}
	}
	dprrs := domain.PatchRetentionRules(prrs)
	return &dprrs
}
