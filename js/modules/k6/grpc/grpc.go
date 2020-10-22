/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2020 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package grpc

import (
	"google.golang.org/grpc/codes"

	"github.com/loadimpact/k6/js/internal/modules"
)

func init() {
	modules.Register("k6/protocols/grpc", New())
}

// GRPC represents the gRPC protocol module for k6
type GRPC struct {
	StatusOK                 codes.Code `js:"StatusOK"`
	StatusCanceled           codes.Code `js:"StatusCanceled"`
	StatusUnknown            codes.Code `js:"StatusUnknown"`
	StatusInvalidArgument    codes.Code `js:"StatusInvalidArgument"`
	StatusDeadlineExceeded   codes.Code `js:"StatusDeadlineExceeded"`
	StatusNotFound           codes.Code `js:"StatusNotFound"`
	StatusAlreadyExists      codes.Code `js:"StatusAlreadyExists"`
	StatusPermissionDenied   codes.Code `js:"StatusPermissionDenied"`
	StatusResourceExhausted  codes.Code `js:"StatusResourceExhausted"`
	StatusFailedPrecondition codes.Code `js:"StatusFailedPrecondition"`
	StatusAborted            codes.Code `js:"StatusAborted"`
	StatusOutOfRange         codes.Code `js:"StatusOutOfRange"`
	StatusUnimplemented      codes.Code `js:"StatusUnimplemented"`
	StatusInternal           codes.Code `js:"StatusInternal"`
	StatusUnavailable        codes.Code `js:"StatusUnavailable"`
	StatusDataLoss           codes.Code `js:"StatusDataLoss"`
	StatusUnauthenticated    codes.Code `js:"StatusUnauthenticated"`
}

// New creates a new gRPC module
func New() *GRPC {
	return &GRPC{
		StatusOK:                 codes.OK,
		StatusCanceled:           codes.Canceled,
		StatusUnknown:            codes.Unknown,
		StatusInvalidArgument:    codes.InvalidArgument,
		StatusDeadlineExceeded:   codes.DeadlineExceeded,
		StatusNotFound:           codes.NotFound,
		StatusAlreadyExists:      codes.AlreadyExists,
		StatusPermissionDenied:   codes.PermissionDenied,
		StatusResourceExhausted:  codes.ResourceExhausted,
		StatusFailedPrecondition: codes.FailedPrecondition,
		StatusAborted:            codes.Aborted,
		StatusOutOfRange:         codes.OutOfRange,
		StatusUnimplemented:      codes.Unimplemented,
		StatusInternal:           codes.Internal,
		StatusUnavailable:        codes.Unavailable,
		StatusDataLoss:           codes.DataLoss,
		StatusUnauthenticated:    codes.Unauthenticated,
	}
}
