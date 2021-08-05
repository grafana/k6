// Copyright 2020-2021 InfluxData, Inc. All rights reserved.
// Use of this source code is governed by MIT
// license that can be found in the LICENSE file.

package api

import (
	"context"
	"encoding/base64"
	"fmt"
	nethttp "net/http"
	"net/http/cookiejar"
	"sync"

	"github.com/influxdata/influxdb-client-go/v2/api/http"
	"github.com/influxdata/influxdb-client-go/v2/domain"
	"golang.org/x/net/publicsuffix"
)

// UsersAPI provides methods for managing users in a InfluxDB server
type UsersAPI interface {
	// GetUsers returns all users
	GetUsers(ctx context.Context) (*[]domain.User, error)
	// FindUserByID returns user with userID
	FindUserByID(ctx context.Context, userID string) (*domain.User, error)
	// FindUserByName returns user with name userName
	FindUserByName(ctx context.Context, userName string) (*domain.User, error)
	// CreateUser creates new user
	CreateUser(ctx context.Context, user *domain.User) (*domain.User, error)
	// CreateUserWithName creates new user with userName
	CreateUserWithName(ctx context.Context, userName string) (*domain.User, error)
	// UpdateUser updates user
	UpdateUser(ctx context.Context, user *domain.User) (*domain.User, error)
	// UpdateUserPassword sets password for an user
	UpdateUserPassword(ctx context.Context, user *domain.User, password string) error
	// UpdateUserPasswordWithID sets password for an user with userID
	UpdateUserPasswordWithID(ctx context.Context, userID string, password string) error
	// DeleteUserWithID deletes an user with userID
	DeleteUserWithID(ctx context.Context, userID string) error
	// DeleteUser deletes an user
	DeleteUser(ctx context.Context, user *domain.User) error
	// Me returns actual user
	Me(ctx context.Context) (*domain.User, error)
	// MeUpdatePassword set password of actual user
	MeUpdatePassword(ctx context.Context, oldPassword, newPassword string) error
	// SignIn exchanges username and password credentials to establish an authenticated session with the InfluxDB server. The Client's authentication token is then ignored, it can be empty.
	SignIn(ctx context.Context, username, password string) error
	// SignOut signs out previously signed in user
	SignOut(ctx context.Context) error
}

// usersAPI implements UsersAPI
type usersAPI struct {
	apiClient       *domain.ClientWithResponses
	httpService     http.Service
	httpClient      *nethttp.Client
	deleteCookieJar bool
	lock            sync.Mutex
}

// NewUsersAPI creates new instance of UsersAPI
func NewUsersAPI(apiClient *domain.ClientWithResponses, httpService http.Service, httpClient *nethttp.Client) UsersAPI {
	return &usersAPI{
		apiClient:   apiClient,
		httpService: httpService,
		httpClient:  httpClient,
	}
}

func (u *usersAPI) GetUsers(ctx context.Context) (*[]domain.User, error) {
	params := &domain.GetUsersParams{}
	response, err := u.apiClient.GetUsersWithResponse(ctx, params)
	if err != nil {
		return nil, err
	}
	if response.JSONDefault != nil {
		return nil, domain.ErrorToHTTPError(response.JSONDefault, response.StatusCode())
	}
	return userResponsesToUsers(response.JSON200.Users), nil
}

func (u *usersAPI) FindUserByID(ctx context.Context, userID string) (*domain.User, error) {
	params := &domain.GetUsersIDParams{}
	response, err := u.apiClient.GetUsersIDWithResponse(ctx, userID, params)
	if err != nil {
		return nil, err
	}
	if response.JSONDefault != nil {
		return nil, domain.ErrorToHTTPError(response.JSONDefault, response.StatusCode())
	}
	return userResponseToUser(response.JSON200), nil
}

func (u *usersAPI) FindUserByName(ctx context.Context, userName string) (*domain.User, error) {
	users, err := u.GetUsers(ctx)
	if err != nil {
		return nil, err
	}
	var user *domain.User
	for _, u := range *users {
		if u.Name == userName {
			user = &u
			break
		}
	}
	if user == nil {
		return nil, fmt.Errorf("user '%s' not found", userName)
	}
	return user, nil
}

func (u *usersAPI) CreateUserWithName(ctx context.Context, userName string) (*domain.User, error) {
	user := &domain.User{Name: userName}
	return u.CreateUser(ctx, user)
}

func (u *usersAPI) CreateUser(ctx context.Context, user *domain.User) (*domain.User, error) {
	params := &domain.PostUsersParams{}
	response, err := u.apiClient.PostUsersWithResponse(ctx, params, domain.PostUsersJSONRequestBody(*user))
	if err != nil {
		return nil, err
	}
	if response.JSONDefault != nil {
		return nil, domain.ErrorToHTTPError(response.JSONDefault, response.StatusCode())
	}
	return userResponseToUser(response.JSON201), nil
}

func (u *usersAPI) UpdateUser(ctx context.Context, user *domain.User) (*domain.User, error) {
	params := &domain.PatchUsersIDParams{}
	response, err := u.apiClient.PatchUsersIDWithResponse(ctx, *user.Id, params, domain.PatchUsersIDJSONRequestBody(*user))
	if err != nil {
		return nil, err
	}
	if response.JSONDefault != nil {
		return nil, domain.ErrorToHTTPError(response.JSONDefault, response.StatusCode())
	}
	return userResponseToUser(response.JSON200), nil
}

func (u *usersAPI) UpdateUserPassword(ctx context.Context, user *domain.User, password string) error {
	return u.UpdateUserPasswordWithID(ctx, *user.Id, password)
}

func (u *usersAPI) UpdateUserPasswordWithID(ctx context.Context, userID string, password string) error {
	params := &domain.PostUsersIDPasswordParams{}
	body := &domain.PasswordResetBody{Password: password}
	response, err := u.apiClient.PostUsersIDPasswordWithResponse(ctx, userID, params, domain.PostUsersIDPasswordJSONRequestBody(*body))
	if err != nil {
		return err
	}
	if response.JSONDefault != nil {
		return domain.ErrorToHTTPError(response.JSONDefault, response.StatusCode())
	}
	return nil
}

func (u *usersAPI) DeleteUser(ctx context.Context, user *domain.User) error {
	return u.DeleteUserWithID(ctx, *user.Id)
}

func (u *usersAPI) DeleteUserWithID(ctx context.Context, userID string) error {
	params := &domain.DeleteUsersIDParams{}
	response, err := u.apiClient.DeleteUsersIDWithResponse(ctx, userID, params)
	if err != nil {
		return err
	}
	if response.JSONDefault != nil {
		return domain.ErrorToHTTPError(response.JSONDefault, response.StatusCode())
	}
	return nil
}

func (u *usersAPI) Me(ctx context.Context) (*domain.User, error) {
	params := &domain.GetMeParams{}
	response, err := u.apiClient.GetMeWithResponse(ctx, params)
	if err != nil {
		return nil, err
	}
	if response.JSONDefault != nil {
		return nil, domain.ErrorToHTTPError(response.JSONDefault, response.StatusCode())
	}
	return userResponseToUser(response.JSON200), nil
}

func (u *usersAPI) MeUpdatePassword(ctx context.Context, oldPassword, newPassword string) error {
	u.lock.Lock()
	defer u.lock.Unlock()
	me, err := u.Me(ctx)
	if err != nil {
		return err
	}
	creds := base64.StdEncoding.EncodeToString([]byte(me.Name + ":" + oldPassword))
	auth := u.httpService.Authorization()
	defer u.httpService.SetAuthorization(auth)
	u.httpService.SetAuthorization("Basic " + creds)
	params := &domain.PutMePasswordParams{}
	body := &domain.PasswordResetBody{Password: newPassword}
	response, err := u.apiClient.PutMePasswordWithResponse(ctx, params, domain.PutMePasswordJSONRequestBody(*body))
	if err != nil {
		return err
	}
	if response.JSONDefault != nil {
		return domain.ErrorToHTTPError(response.JSONDefault, response.StatusCode())
	}
	return nil
}

func (u *usersAPI) SignIn(ctx context.Context, username, password string) error {
	u.lock.Lock()
	defer u.lock.Unlock()
	if u.httpClient.Jar == nil {
		jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
		if err != nil {
			return err
		}
		u.httpClient.Jar = jar
		u.deleteCookieJar = true
	}
	creds := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	u.httpService.SetAuthorization("Basic " + creds)
	defer u.httpService.SetAuthorization("")
	resp, err := u.apiClient.PostSigninWithResponse(ctx, &domain.PostSigninParams{})
	if err != nil {
		return err
	}
	if resp.JSONDefault != nil {
		return domain.ErrorToHTTPError(resp.JSONDefault, resp.StatusCode())
	}
	if resp.JSON401 != nil {
		return domain.ErrorToHTTPError(resp.JSON401, resp.StatusCode())
	}
	if resp.JSON403 != nil {
		return domain.ErrorToHTTPError(resp.JSON403, resp.StatusCode())
	}
	return nil
}

func (u *usersAPI) SignOut(ctx context.Context) error {
	u.lock.Lock()
	defer u.lock.Unlock()
	resp, err := u.apiClient.PostSignoutWithResponse(ctx, &domain.PostSignoutParams{})
	if err != nil {
		return err
	}
	if resp.JSONDefault != nil {
		return domain.ErrorToHTTPError(resp.JSONDefault, resp.StatusCode())
	}
	if resp.JSON401 != nil {
		return domain.ErrorToHTTPError(resp.JSON401, resp.StatusCode())
	}
	if u.deleteCookieJar {
		u.httpClient.Jar = nil
	}
	return nil
}

func userResponseToUser(ur *domain.UserResponse) *domain.User {
	if ur == nil {
		return nil
	}
	user := &domain.User{
		Id:      ur.Id,
		Name:    ur.Name,
		OauthID: ur.OauthID,
		Status:  userResponseStatusToUserStatus(ur.Status),
	}
	return user
}

func userResponseStatusToUserStatus(urs *domain.UserResponseStatus) *domain.UserStatus {
	if urs == nil {
		return nil
	}
	us := domain.UserStatus(*urs)
	return &us
}

func userResponsesToUsers(urs *[]domain.UserResponse) *[]domain.User {
	if urs == nil {
		return nil
	}
	us := make([]domain.User, len(*urs))
	for i, ur := range *urs {
		us[i] = *userResponseToUser(&ur)
	}
	return &us
}
