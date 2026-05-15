// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package ntlmssp

import (
	"errors"
)

type varField struct {
	Len          uint16
	MaxLen       uint16
	BufferOffset uint32
}

func (f varField) ReadFrom(buffer []byte) ([]byte, error) {
	// f.Len is controlled by the sender, so we need to check that
	// it doesn't cause an overflow when added to f.BufferOffset.
	start := uint64(f.BufferOffset)
	end := start + uint64(f.Len)
	if end < start || end > uint64(len(buffer)) {
		return nil, errors.New("error reading data, varField extends beyond buffer")
	}
	return buffer[int(start):int(end)], nil
}

func (f varField) ReadStringFrom(buffer []byte, unicode bool) (string, error) {
	d, err := f.ReadFrom(buffer)
	if err != nil {
		return "", err
	}
	if unicode { // UTF-16LE encoding scheme
		return fromUnicode(d)
	}
	// OEM encoding, close enough to ASCII, since no code page is specified
	return string(d), err
}

func newVarField(ptr *int, fieldsize int) varField {
	f := varField{
		Len:          uint16(fieldsize),
		MaxLen:       uint16(fieldsize),
		BufferOffset: uint32(*ptr),
	}
	*ptr += fieldsize
	return f
}
