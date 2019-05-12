/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
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

package crypto

import (
	"encoding/base64"
	"encoding/hex"
	"unicode/utf8"

	"github.com/pkg/errors"
)

func decodeBinary(encoded interface{}, format string) ([]byte, error) {
	if format != "" {
		return decodeBinaryKnown(encoded, format)
	}
	return decodeBinaryDetect(encoded)
}

func decodeBinaryKnown(encoded interface{}, format string) ([]byte, error) {
	switch format {
	case "binary":
		return decodeBytes(encoded)
	case "hex":
		return decodeHex(encoded)
	case "base64":
		return decodeBase64(encoded)
	default:
		err := errors.New("unsupported binary encoding: " + format)
		return nil, err
	}
}

func decodeBinaryDetect(encoded interface{}) ([]byte, error) {
	decoded, err := decodeBytes(encoded)
	if err == nil {
		return decoded, nil
	}
	decoded, err = decodeHex(encoded)
	if err == nil {
		return decoded, nil
	}
	decoded, err = decodeBase64(encoded)
	if err == nil {
		return decoded, nil
	}
	err = errors.New("unrecognized binary encoding")
	return nil, err
}

func decodeBytes(abstracted interface{}) ([]byte, error) {
	switch encoded := abstracted.(type) {
	case []uint8:
		return decodeInternalBytes(encoded), nil
	case []interface{}:
		return decodeExternalBytes(encoded)
	default:
		err := errors.New("not a byte array")
		return nil, err
	}
}

// Bytes originating in Go unmarshaled as slice of uint8
func decodeInternalBytes(encoded []uint8) []byte {
	return []byte(encoded)
}

// Bytes originating in JavaScript unmarshaled as slice of abstracted int64
func decodeExternalBytes(encoded []interface{}) ([]byte, error) {
	decoded := make([]byte, len(encoded))
	for i, itemAbstracted := range encoded {
		itemDecoded, err := decodeExternalByte(itemAbstracted)
		if err != nil {
			return nil, err
		}
		decoded[i] = itemDecoded
	}
	return decoded, nil
}

func decodeExternalByte(abstracted interface{}) (byte, error) {
	encoded, ok := abstracted.(int64)
	if !ok {
		err := errors.New("not a byte array")
		return 0, err
	}
	return byte(encoded), nil
}

func decodeHex(abstracted interface{}) ([]byte, error) {
	encoded, ok := abstracted.(string)
	if !ok {
		err := errors.New("not a hex string")
		return nil, err
	}
	return hex.DecodeString(encoded)
}

func decodeBase64(abstracted interface{}) ([]byte, error) {
	encoded, ok := abstracted.(string)
	if !ok {
		err := errors.New("not a base64 string")
		return nil, err
	}
	return base64.StdEncoding.DecodeString(encoded)
}

func decodeString(encoded []byte) (string, error) {
	if !utf8.Valid(encoded) {
		err := errors.New("not a UTF-8 string")
		return "", err
	}
	return string(encoded), nil
}

func encodeBinary(value []byte, format string) (interface{}, error) {
	switch format {
	case "":
		fallthrough
	case "binary":
		return value, nil
	case "hex":
		encoded := hex.EncodeToString(value)
		return encoded, nil
	case "base64":
		encoded := base64.StdEncoding.EncodeToString(value)
		return encoded, nil
	default:
		err := errors.New("unsupported binary encoding: " + format)
		return "", err
	}
}
