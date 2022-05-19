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
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/grafana/xk6-browser/k6"

	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/dop251/goja"
	"github.com/hashicorp/go-multierror"
	"github.com/sirupsen/logrus"
)

type objectOverflowError struct{}

// Error returns the description of the overflow error.
func (*objectOverflowError) Error() string {
	return "object is too large and will be parsed partially"
}

type objectPropertyParseError struct {
	error
	property string
}

// Error returns the reason of the failure, including the wrapper parsing error
// message.
func (pe *objectPropertyParseError) Error() string {
	return fmt.Sprintf("failed parsing object property %q: %s", pe.property, pe.error)
}

// Unwrap returns the wrapped parsing error.
func (pe *objectPropertyParseError) Unwrap() error {
	return pe.error
}

func parseRemoteObjectPreview(op *cdpruntime.ObjectPreview) (map[string]interface{}, error) {
	obj := make(map[string]interface{})
	var result error
	if op.Overflow {
		result = multierror.Append(result, &objectOverflowError{})
	}

	for _, p := range op.Properties {
		val, err := parseRemoteObjectValue(p.Type, p.Value, p.ValuePreview)
		if err != nil {
			result = multierror.Append(result, &objectPropertyParseError{err, p.Name})
			continue
		}
		obj[p.Name] = val
	}

	return obj, result
}

func parseRemoteObjectValue(t cdpruntime.Type, val string, op *cdpruntime.ObjectPreview) (interface{}, error) {
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
			return parseRemoteObjectPreview(op)
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

func parseExceptionDetails(exc *cdpruntime.ExceptionDetails) string {
	errMsg := fmt.Sprintf("%s", exc)
	if exc.Exception != nil {
		if o, _ := parseRemoteObject(exc.Exception); o != nil {
			errMsg += fmt.Sprintf("%s", o)
		}
	}
	return errMsg
}

func parseRemoteObject(obj *cdpruntime.RemoteObject) (interface{}, error) {
	if obj.UnserializableValue == "" {
		return parseRemoteObjectValue(obj.Type, string(obj.Value), obj.Preview)
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

func valueFromRemoteObject(ctx context.Context, robj *cdpruntime.RemoteObject) (goja.Value, error) {
	val, err := parseRemoteObject(robj)
	if val == "undefined" {
		return goja.Undefined(), err
	}
	return k6.Runtime(ctx).ToValue(val), err
}

func handleParseRemoteObjectErr(ctx context.Context, err error, logger *logrus.Entry) {
	var (
		ooe *objectOverflowError
		ope *objectPropertyParseError
	)
	merr, ok := err.(*multierror.Error)
	if !ok {
		// If this throws it's a bug :)
		k6Throw(ctx, "unable to parse remote object value: %w", err)
	}
	for _, e := range merr.Errors {
		switch {
		case errors.As(e, &ooe):
			logger.Warn(ooe)
		case errors.As(e, &ope):
			logger.WithError(ope).Error()
		default:
			// If this throws it's a bug :)
			k6Throw(ctx, "unable to parse remote object value: %w", e)
		}
	}
}
