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

package twirp

import "github.com/twitchtv/twirp"

// Twirp represents the Twirp protocol module for k6
type Twirp struct {
	NoError            twirp.ErrorCode `js:"NoError"`
	Canceled           twirp.ErrorCode `js:"Canceled"`
	Unknown            twirp.ErrorCode `js:"Unknown"`
	InvalidArgument    twirp.ErrorCode `js:"InvalidArgument"`
	Malformed          twirp.ErrorCode `js:"Malformed"`
	DeadlineExceeded   twirp.ErrorCode `js:"DeadlineExceeded"`
	NotFound           twirp.ErrorCode `js:"NotFound"`
	BadRoute           twirp.ErrorCode `js:"BadRoute"`
	AlreadyExists      twirp.ErrorCode `js:"AlreadyExists"`
	PermissionDenied   twirp.ErrorCode `js:"PermissionDenied"`
	Unauthenticated    twirp.ErrorCode `js:"Unauthenticated"`
	ResourceExhausted  twirp.ErrorCode `js:"ResourceExhausted"`
	FailedPrecondition twirp.ErrorCode `js:"FailedPrecondition"`
	Aborted            twirp.ErrorCode `js:"Aborted"`
	OutOfRange         twirp.ErrorCode `js:"OutOfRange"`
	Unimplemented      twirp.ErrorCode `js:"Unimplemented"`
	Internal           twirp.ErrorCode `js:"Internal"`
	Unavailable        twirp.ErrorCode `js:"Unavailable"`
	DataLoss           twirp.ErrorCode `js:"DataLoss"`
}

// New creates a new gRPC module
func New() *Twirp {
	return &Twirp{
		NoError:            twirp.NoError,
		Canceled:           twirp.Canceled,
		Unknown:            twirp.Unknown,
		InvalidArgument:    twirp.InvalidArgument,
		Malformed:          twirp.Malformed,
		DeadlineExceeded:   twirp.DeadlineExceeded,
		NotFound:           twirp.NotFound,
		BadRoute:           twirp.BadRoute,
		AlreadyExists:      twirp.AlreadyExists,
		PermissionDenied:   twirp.PermissionDenied,
		Unauthenticated:    twirp.Unauthenticated,
		ResourceExhausted:  twirp.ResourceExhausted,
		FailedPrecondition: twirp.FailedPrecondition,
		Aborted:            twirp.Aborted,
		OutOfRange:         twirp.OutOfRange,
		Unimplemented:      twirp.Unimplemented,
		Internal:           twirp.Internal,
		Unavailable:        twirp.Unavailable,
		DataLoss:           twirp.DataLoss,
	}
}
