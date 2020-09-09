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
	"context"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
	"gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/testutils/httpmultibin"
	"github.com/loadimpact/k6/stats"
)

func TestClient(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)
	defer tb.Cleanup()
	sr := tb.Replacer.Replace

	root, err := lib.NewGroup("", nil)
	assert.NoError(t, err)

	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})
	samples := make(chan stats.SampleContainer, 1000)
	state := &lib.State{
		Group:     root,
		Dialer:    tb.Dialer,
		TLSConfig: tb.TLSClientConfig,
		Samples:   samples,
		Options: lib.Options{
			SystemTags: stats.NewSystemTagSet(
				stats.TagName,
			),
			UserAgent: null.StringFrom("k6-test"),
		},
	}

	ctx := common.WithRuntime(context.Background(), rt)

	rt.Set("grpc", common.Bind(rt, New(), &ctx))

	t.Run("New", func(t *testing.T) {
		_, err := common.RunString(rt, `
			var client = grpc.newClient();
			if (!client) throw new Error("no client created")
		`)
		assert.NoError(t, err)
	})

	t.Run("LoadNotFound", func(t *testing.T) {
		_, err := common.RunString(rt, `
			client.load([], "./does_not_exist.proto");	
		`)
		if !assert.Error(t, err) {
			return
		}
		assert.Contains(t, err.Error(), "no such file or directory")
	})

	t.Run("Load", func(t *testing.T) {
		respV, err := common.RunString(rt, `
			client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");	
		`)
		if !assert.NoError(t, err) {
			return
		}
		resp := respV.Export()
		assert.IsType(t, []MethodDesc{}, resp)
		assert.Len(t, resp, 6)
	})

	t.Run("ConnectInit", func(t *testing.T) {
		_, err := common.RunString(rt, `
			var err = client.connect();
			throw new Error(err)
		`)
		if !assert.Error(t, err) {
			return
		}
		assert.Contains(t, err.Error(), "connecting to a gRPC server in the init context is not supported")
	})

	ctx = lib.WithState(ctx, state)

	t.Run("NoConnect", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
			client.invokeRPC("grpc.testing.TestService/EmptyCall", {})
		`))
		if !assert.Error(t, err) {
			return
		}
		assert.Contains(t, err.Error(), "no gRPC connection, you must call connect first")
	})

	t.Run("Connect", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
			var err = client.connect("GRPCBIN_ADDR"); 
			if (err) throw new Error("connection failed with error: " + err)
		`))
		assert.NoError(t, err)
	})

	t.Run("InvokeRPC", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
			var resp = client.invokeRPC("grpc.testing.TestService/EmptyCall", {})
			if (resp.status !== grpc.StatusUnimplemented) {
				throw new Error("unexpected error status: " + resp.status)
			}
		`))
		assert.NoError(t, err)
	})

	t.Run("LoadNotInit", func(t *testing.T) {
		_, err := common.RunString(rt, "client.load()")
		assert.Contains(t, err.Error(), "load must be called in the init context")
	})
}
