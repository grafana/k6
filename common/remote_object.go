/*
 *
 * xk6-browser - a browser automation extension for k6
 * Copyright (C) 2021 Load Impact
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

package common

import (
	"context"
	"encoding/json"
	"math"
	"strconv"
	"strings"

	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/dop251/goja"
	"github.com/sirupsen/logrus"
	k6common "go.k6.io/k6/js/common"
)

func parseRemoteObjectPreview(op *cdpruntime.ObjectPreview, logger *logrus.Entry) (map[string]interface{}, error) {
	obj := make(map[string]interface{})
	if op.Overflow {
		logger.Warn("object will be parsed partially")
	}

	for _, p := range op.Properties {
		val, err := parseRemoteObjectValue(p.Type, p.Value, p.ValuePreview, logger)
		if err != nil {
			logger.WithError(err).Errorf("failed parsing object property %q", p.Name)
			continue
		}
		obj[p.Name] = val
	}

	return obj, nil
}

func parseRemoteObjectValue(
	t cdpruntime.Type, val string, op *cdpruntime.ObjectPreview, logger *logrus.Entry,
) (interface{}, error) {
	switch t {
	case cdpruntime.TypeAccessor:
		return "accessor", nil
	case cdpruntime.TypeBigint:
		n, err := strconv.ParseInt(strings.Replace(val, "n", "", -1), 10, 64)
		if err != nil {
			return nil, BigIntParseError{err}
		}
		return n, nil
	case cdpruntime.TypeFunction:
		return "function()", nil
	case cdpruntime.TypeString:
		if !strings.HasPrefix(val, `"`) {
			return val, nil
		}
	case cdpruntime.TypeSymbol:
		return val, nil
	case cdpruntime.TypeObject:
		if op != nil {
			return parseRemoteObjectPreview(op, logger)
		}
		if val == "Object" {
			return val, nil
		}
	case cdpruntime.TypeUndefined:
		return "undefined", nil
	}

	var v interface{}
	if err := json.Unmarshal([]byte(val), &v); err != nil {
		return nil, err
	}

	return v, nil
}

func parseRemoteObject(obj *cdpruntime.RemoteObject, logger *logrus.Entry) (interface{}, error) {
	if obj.UnserializableValue == "" {
		return parseRemoteObjectValue(obj.Type, string(obj.Value), obj.Preview, logger)
	}

	switch obj.UnserializableValue.String() {
	case "-0": // To handle +0 divided by negative number
		return math.Float64frombits(0 | (1 << 63)), nil
	case "NaN":
		return math.NaN(), nil
	case "Infinity":
		return math.Inf(0), nil
	case "-Infinity":
		return math.Inf(-1), nil
	}

	return nil, UnserializableValueError{obj.UnserializableValue}
}

func valueFromRemoteObject(
	ctx context.Context, remoteObject *cdpruntime.RemoteObject, logger *logrus.Entry,
) (goja.Value, error) {
	val, err := parseRemoteObject(remoteObject, logger)
	if val == "undefined" {
		return goja.Undefined(), err
	}
	return k6common.GetRuntime(ctx).ToValue(val), err
}
