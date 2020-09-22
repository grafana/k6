package dns

import (
	"strings"
)

// Compressor encodes domain names.
type Compressor interface {
	Length(...string) (int, error)
	Pack([]byte, string) ([]byte, error)
}

// Decompressor decodes domain names.
type Decompressor interface {
	Unpack([]byte) (string, []byte, error)
}

type compressor struct {
	tbl    map[string]int
	offset int
}

func (c compressor) Length(names ...string) (int, error) {
	var visited map[string]struct{}
	if c.tbl != nil {
		visited = make(map[string]struct{})
	}

	var n int
	for _, name := range names {
		nn, err := c.length(name, visited)
		if err != nil {
			return 0, err
		}
		n += nn
	}
	return n, nil
}

func (c compressor) length(name string, visited map[string]struct{}) (int, error) {
	if name == "." || name == "" {
		return 1, nil
	}
	if !strings.HasSuffix(name, ".") {
		return 0, errInvalidFQDN
	}

	if c.tbl != nil {
		if _, ok := c.tbl[name]; ok {
			return 2, nil
		}
		if _, ok := visited[name]; ok {
			return 2, nil
		}

		visited[name] = struct{}{}
	}

	pvt := strings.IndexByte(name, '.')
	n, err := c.length(name[pvt+1:], visited)
	if err != nil {
		return 0, err
	}
	return pvt + 1 + n, nil
}

func (c compressor) Pack(b []byte, fqdn string) ([]byte, error) {
	if fqdn == "." || fqdn == "" {
		return append(b, 0x00), nil
	}

	if c.tbl != nil {
		if idx, ok := c.tbl[fqdn]; ok {
			ptr, err := pointerTo(idx)
			if err != nil {
				return nil, err
			}

			return append(b, ptr...), nil
		}
	}

	pvt := strings.IndexByte(fqdn, '.')
	switch {
	case pvt == -1:
		return nil, errInvalidFQDN
	case pvt == 0:
		return nil, errZeroSegLen
	case pvt > 63:
		return nil, errSegTooLong
	}

	if c.tbl != nil {
		idx := len(b) - c.offset
		if int(uint16(idx)) != idx {
			return nil, errInvalidPtr
		}
		c.tbl[fqdn] = idx
	}

	b = append(b, byte(pvt))
	b = append(b, fqdn[:pvt]...)

	return c.Pack(b, fqdn[pvt+1:])
}

type decompressor []byte

func (d decompressor) Unpack(b []byte) (string, []byte, error) {
	name, b, err := d.unpack(make([]byte, 0, 32), b, nil)
	if err != nil {
		return "", nil, err
	}
	return string(name), b, nil
}

func (d decompressor) unpack(name, b []byte, visited []int) ([]byte, []byte, error) {
	lenb := len(b)
	if lenb == 0 {
		return nil, nil, errBaseLen
	}
	if b[0] == 0x00 {
		if len(name) == 0 {
			return append(name, '.'), b[1:], nil
		}
		return name, b[1:], nil
	}
	if lenb < 2 {
		return nil, nil, errBaseLen
	}

	if isPointer(b[0]) {
		if d == nil {
			return nil, nil, errBaseLen
		}

		ptr := nbo.Uint16(b[:2])
		name, err := d.deref(name, ptr, visited)
		if err != nil {
			return nil, nil, err
		}

		return name, b[2:], nil
	}

	lenl, b := int(b[0]), b[1:]

	if len(b) < lenl {
		return nil, nil, errCalcLen
	}

	name = append(name, b[:lenl]...)
	name = append(name, '.')

	return d.unpack(name, b[lenl:], visited)
}

func (d decompressor) deref(name []byte, ptr uint16, visited []int) ([]byte, error) {
	idx := int(ptr & 0x3FFF)
	if len(d) < idx {
		return nil, errInvalidPtr
	}

	if isPointer(d[idx]) {
		return nil, errInvalidPtr
	}

	for _, v := range visited {
		if idx == v {
			return nil, errPtrCycle
		}
	}

	name, _, err := d.unpack(name, d[idx:], append(visited, idx))
	return name, err
}

func isPointer(b byte) bool { return b&0xC0 > 0 }

func pointerTo(idx int) ([]byte, error) {
	ptr := uint16(idx)
	if int(ptr) != idx {
		return nil, errInvalidPtr
	}
	ptr |= 0xC000

	buf := [2]byte{}
	nbo.PutUint16(buf[:], ptr)
	return buf[:], nil
}
