// Copyright (c) 2012-2020 Ugorji Nwoke. All rights reserved.
// Use of this source code is governed by a MIT license found in the LICENSE file.

package codec

import "io"

// decReader abstracts the reading source, allowing implementations that can
// read from an io.Reader or directly off a byte slice with zero-copying.
type decReader interface {
	// readx will use the implementation scratch buffer if possible i.e. n < len(scratchbuf), OR
	// just return a view of the []byte being decoded from.
	// Ensure you call detachZeroCopyBytes later if this needs to be sent outside codec control.
	readx(n uint) []byte
	readb([]byte)

	readn1() byte
	readn2() [2]byte
	readn3() [3]byte
	readn4() [4]byte
	readn8() [8]byte
	// readn1eof() (v uint8, eof bool)

	// // read up to 8 bytes at a time
	// readn(num uint8) (v [8]byte)

	numread() uint // number of bytes read

	// readNumber(includeLastByteRead bool) []byte

	// skip any whitespace characters, and return the first non-matching byte
	skipWhitespace() (token byte)

	// jsonReadNum will include last read byte in first element of slice,
	// and continue numeric characters until it sees a non-numeric char
	// or EOF. If it sees a non-numeric character, it will unread that.
	jsonReadNum() []byte

	// jsonReadAsisChars will read json plain characters (anything but " or \)
	// and return a slice terminated by a non-json asis character.
	jsonReadAsisChars() []byte

	// skip will skip any byte that matches, and return the first non-matching byte
	// skip(accept *bitset256) (token byte)

	// readTo will read any byte that matches, stopping once no-longer matching.
	// readTo(accept *bitset256) (out []byte)

	// readUntil will read, only stopping once it matches the 'stop' byte (which it excludes).
	readUntil(stop byte) (out []byte)
}

// ------------------------------------------------

type unreadByteStatus uint8

// unreadByteStatus goes from
// undefined (when initialized) -- (read) --> canUnread -- (unread) --> canRead ...
const (
	unreadByteUndefined unreadByteStatus = iota
	unreadByteCanRead
	unreadByteCanUnread
)

// --------------------

type ioDecReaderCommon struct {
	r io.Reader // the reader passed in

	n uint // num read

	l  byte             // last byte
	ls unreadByteStatus // last byte status

	b [6]byte // tiny buffer for reading single bytes

	blist *bytesFreelist

	bufr []byte // buffer for readTo/readUntil
}

func (z *ioDecReaderCommon) reset(r io.Reader, blist *bytesFreelist) {
	z.blist = blist
	z.r = r
	z.ls = unreadByteUndefined
	z.l, z.n = 0, 0
}

func (z *ioDecReaderCommon) numread() uint {
	return z.n
}

// ------------------------------------------

// ioDecReader is a decReader that reads off an io.Reader.
//
// It also has a fallback implementation of ByteScanner if needed.
type ioDecReader struct {
	ioDecReaderCommon

	br io.ByteScanner

	x [64 + 48]byte // for: get struct field name, swallow valueTypeBytes, etc
}

func (z *ioDecReader) reset(r io.Reader, blist *bytesFreelist) {
	z.ioDecReaderCommon.reset(r, blist)

	z.br, _ = r.(io.ByteScanner)
}

func (z *ioDecReader) Read(p []byte) (n int, err error) {
	if len(p) == 0 {
		return
	}
	var firstByte bool
	if z.ls == unreadByteCanRead {
		z.ls = unreadByteCanUnread
		p[0] = z.l
		if len(p) == 1 {
			n = 1
			return
		}
		firstByte = true
		p = p[1:]
	}
	n, err = z.r.Read(p)
	if n > 0 {
		if err == io.EOF && n == len(p) {
			err = nil // read was successful, so postpone EOF (till next time)
		}
		z.l = p[n-1]
		z.ls = unreadByteCanUnread
	}
	if firstByte {
		n++
	}
	return
}

func (z *ioDecReader) ReadByte() (c byte, err error) {
	if z.br != nil {
		c, err = z.br.ReadByte()
		if err == nil {
			z.l = c
			z.ls = unreadByteCanUnread
		}
		return
	}

	n, err := z.Read(z.b[:1])
	if n == 1 {
		c = z.b[0]
		if err == io.EOF {
			err = nil // read was successful, so postpone EOF (till next time)
		}
	}
	return
}

func (z *ioDecReader) UnreadByte() (err error) {
	if z.br != nil {
		err = z.br.UnreadByte()
		if err == nil {
			z.ls = unreadByteCanRead
		}
		return
	}

	switch z.ls {
	case unreadByteCanUnread:
		z.ls = unreadByteCanRead
	case unreadByteCanRead:
		err = errDecUnreadByteLastByteNotRead
	case unreadByteUndefined:
		err = errDecUnreadByteNothingToRead
	default:
		err = errDecUnreadByteUnknown
	}
	return
}

func (z *ioDecReader) readn2() (bs [2]byte) {
	z.readb(bs[:])
	return
}

func (z *ioDecReader) readn3() (bs [3]byte) {
	z.readb(bs[:])
	return
}

func (z *ioDecReader) readn4() (bs [4]byte) {
	z.readb(bs[:])
	return
}

func (z *ioDecReader) readn8() (bs [8]byte) {
	z.readb(bs[:])
	return
}

func (z *ioDecReader) readx(n uint) (bs []byte) {
	if n == 0 {
		return
	}
	if n < uint(len(z.x)) {
		bs = z.x[:n]
	} else {
		bs = make([]byte, n)
	}
	_, err := readFull(z.r, bs)
	halt.onerror(err)
	z.n += uint(len(bs))
	return
}

func (z *ioDecReader) readb(bs []byte) {
	if len(bs) == 0 {
		return
	}
	_, err := readFull(z.r, bs)
	halt.onerror(err)
	z.n += uint(len(bs))
}

func (z *ioDecReader) readn1() (b uint8) {
	b, err := z.ReadByte()
	halt.onerror(err)
	z.n++
	return
}

func (z *ioDecReader) readn1eof() (b uint8, eof bool) {
	b, err := z.ReadByte()
	if err == nil {
		z.n++
	} else if err == io.EOF {
		eof = true
	} else {
		halt.onerror(err)
	}
	return
}

func (z *ioDecReader) jsonReadNum() (bs []byte) {
	z.unreadn1()
	z.bufr = z.blist.check(z.bufr, 256)
LOOP:
	i, eof := z.readn1eof()
	if eof {
		return z.bufr
	}
	if isNumberChar(i) {
		z.bufr = append(z.bufr, i)
		goto LOOP
	}
	z.unreadn1()
	return z.bufr
}

func (z *ioDecReader) jsonReadAsisChars() (bs []byte) {
	z.bufr = z.blist.check(z.bufr, 256)
LOOP:
	i := z.readn1()
	z.bufr = append(z.bufr, i)
	if i == '"' || i == '\\' {
		return z.bufr
	}
	goto LOOP
}

func (z *ioDecReader) skipWhitespace() (token byte) {
LOOP:
	token = z.readn1()
	if isWhitespaceChar(token) {
		goto LOOP
	}
	return
}

func (z *ioDecReader) readUntil(stop byte) []byte {
	z.bufr = z.blist.check(z.bufr, 256)
LOOP:
	token := z.readn1()
	z.bufr = append(z.bufr, token)
	if token == stop {
		return z.bufr[:len(z.bufr)-1]
	}
	goto LOOP
}

func (z *ioDecReader) unreadn1() {
	err := z.UnreadByte()
	halt.onerror(err)
	z.n--
}

// ------------------------------------

type bufioDecReader struct {
	ioDecReaderCommon

	c   uint // cursor
	buf []byte
}

func (z *bufioDecReader) reset(r io.Reader, bufsize int, blist *bytesFreelist) {
	z.ioDecReaderCommon.reset(r, blist)
	z.c = 0
	if cap(z.buf) < bufsize {
		z.buf = blist.get(bufsize)
	} else {
		z.buf = z.buf[:0]
	}
}

func (z *bufioDecReader) readb(p []byte) {
	var n = uint(copy(p, z.buf[z.c:]))
	z.n += n
	z.c += n
	if len(p) != int(n) {
		z.readbFill(p, n, true, false)
	}
}

func readbFillHandleErr(err error, must, eof bool) (isEOF bool) {
	if err == io.EOF {
		isEOF = true
	}
	if must && !(eof && isEOF) {
		halt.onerror(err)
	}
	return
}

func (z *bufioDecReader) readbFill(p0 []byte, n uint, must, eof bool) (isEOF bool, err error) {
	// at this point, there's nothing in z.buf to read (z.buf is fully consumed)
	var p []byte
	if p0 != nil {
		p = p0[n:]
	}
	var n2 uint
	if len(p) > cap(z.buf) {
		n2, err = readFull(z.r, p)
		if err != nil {
			isEOF = readbFillHandleErr(err, must, eof)
			return
		}
		n += n2
		z.n += n2
		// always keep last byte in z.buf
		z.buf = z.buf[:1]
		z.buf[0] = p[len(p)-1]
		z.c = 1
		return
	}
	// z.c is now 0, and len(p) <= cap(z.buf)
	var n1 int
LOOP:
	// for len(p) > 0 && z.err == nil {
	z.buf = z.buf[0:cap(z.buf)]
	n1, err = z.r.Read(z.buf)
	n2 = uint(n1)
	if n2 == 0 && err != nil {
		isEOF = readbFillHandleErr(err, must, eof)
		return
	}
	err = nil
	z.buf = z.buf[:n2]
	z.c = 0
	if len(p) > 0 {
		n2 = uint(copy(p, z.buf))
		z.c = n2
		n += n2
		z.n += n2
		p = p[n2:]
		if len(p) > 0 {
			goto LOOP
		}
		if z.c == 0 {
			z.buf = z.buf[:1]
			z.buf[0] = p[len(p)-1]
			z.c = 1
		}
	}
	return
}

func (z *bufioDecReader) readn1() (b byte) {
	if z.c >= uint(len(z.buf)) {
		z.readbFill(nil, 0, true, false)
	}
	b = z.buf[z.c]
	z.c++
	z.n++
	return
}

func (z *bufioDecReader) readn1eof() (b byte, eof bool) {
	if z.c >= uint(len(z.buf)) {
		eof, _ = z.readbFill(nil, 0, true, true)
		if eof {
			return
		}
	}
	b = z.buf[z.c]
	z.c++
	z.n++
	return
}

func (z *bufioDecReader) unreadn1() {
	if z.c == 0 {
		halt.onerror(errDecUnreadByteNothingToRead)
	}
	z.c--
	z.n--
}

func (z *bufioDecReader) readn2() (bs [2]byte) {
	z.readb(bs[:])
	return
}

func (z *bufioDecReader) readn3() (bs [3]byte) {
	z.readb(bs[:])
	return
}

func (z *bufioDecReader) readn4() (bs [4]byte) {
	z.readb(bs[:])
	return
}

func (z *bufioDecReader) readn8() (bs [8]byte) {
	z.readb(bs[:])
	return
}

func (z *bufioDecReader) readx(n uint) (bs []byte) {
	if n == 0 {
		// return
	} else if z.c+n <= uint(len(z.buf)) {
		bs = z.buf[z.c : z.c+n]
		z.n += n
		z.c += n
	} else {
		bs = make([]byte, n)
		// n no longer used - can reuse
		n = uint(copy(bs, z.buf[z.c:]))
		z.n += n
		z.c += n
		z.readbFill(bs, n, true, false)
	}
	return
}

func (z *bufioDecReader) jsonReadNum() (bs []byte) {
	z.unreadn1()
	z.bufr = z.blist.check(z.bufr, 256)
LOOP:
	i, eof := z.readn1eof()
	if eof {
		return z.bufr
	}
	if isNumberChar(i) {
		z.bufr = append(z.bufr, i)
		goto LOOP
	}
	z.unreadn1()
	return z.bufr
}

func (z *bufioDecReader) jsonReadAsisChars() (bs []byte) {
	z.bufr = z.blist.check(z.bufr, 256)
LOOP:
	i := z.readn1()
	z.bufr = append(z.bufr, i)
	if i == '"' || i == '\\' {
		return z.bufr
	}
	goto LOOP
}

func (z *bufioDecReader) skipWhitespace() (token byte) {
	i := z.c
LOOP:
	if i < uint(len(z.buf)) {
		// inline z.skipLoopFn(i) and refactor, so cost is within inline budget
		token = z.buf[i]
		i++
		if isWhitespaceChar(token) {
			goto LOOP
		}
		z.n += i - 2 - z.c
		z.c = i
		return
	}
	return z.skipFillWhitespace()
}

func (z *bufioDecReader) skipFillWhitespace() (token byte) {
	z.n += uint(len(z.buf)) - z.c
	var i, n2 int
	var err error
	for {
		z.c = 0
		z.buf = z.buf[0:cap(z.buf)]
		n2, err = z.r.Read(z.buf)
		if n2 == 0 {
			halt.onerror(err)
		}
		z.buf = z.buf[:n2]
		for i, token = range z.buf {
			if !isWhitespaceChar(token) {
				z.n += (uint(i) - z.c) - 1
				z.loopFn(uint(i + 1))
				return
			}
		}
		z.n += uint(n2)
	}
}

func (z *bufioDecReader) loopFn(i uint) {
	z.c = i
}

func (z *bufioDecReader) readUntil(stop byte) (out []byte) {
	i := z.c
LOOP:
	if i < uint(len(z.buf)) {
		if z.buf[i] == stop {
			z.n += (i - z.c) - 1
			i++
			out = z.buf[z.c:i]
			z.c = i
			goto FINISH
		}
		i++
		goto LOOP
	}
	out = z.readUntilFill(stop)
FINISH:
	return out[:len(out)-1]
}

func (z *bufioDecReader) readUntilFill(stop byte) []byte {
	z.bufr = z.blist.check(z.bufr, 256)
	z.n += uint(len(z.buf)) - z.c
	z.bufr = append(z.bufr, z.buf[z.c:]...)
	for {
		z.c = 0
		z.buf = z.buf[0:cap(z.buf)]
		n1, err := z.r.Read(z.buf)
		if n1 == 0 {
			halt.onerror(err)
		}
		n2 := uint(n1)
		z.buf = z.buf[:n2]
		for i, token := range z.buf {
			if token == stop {
				z.n += (uint(i) - z.c) - 1
				z.bufr = append(z.bufr, z.buf[z.c:i+1]...)
				z.loopFn(uint(i + 1))
				return z.bufr
			}
		}
		z.bufr = append(z.bufr, z.buf...)
		z.n += n2
	}
}

// ------------------------------------

// bytesDecReader is a decReader that reads off a byte slice with zero copying
//
// Note: we do not try to convert index'ing out of bounds to an io.EOF.
// instead, we let it bubble up to the exported Encode/Decode method
// and recover it as an io.EOF.
//
// see panicValToErr(...) function in helper.go.
type bytesDecReader struct {
	b []byte // data
	c uint   // cursor
}

func (z *bytesDecReader) reset(in []byte) {
	z.b = in[:len(in):len(in)] // reslicing must not go past capacity
	z.c = 0
}

func (z *bytesDecReader) numread() uint {
	return z.c
}

// Note: slicing from a non-constant start position is more expensive,
// as more computation is required to decipher the pointer start position.
// However, we do it only once, and it's better than reslicing both z.b and return value.

func (z *bytesDecReader) readx(n uint) (bs []byte) {
	x := z.c + n
	bs = z.b[z.c:x]
	z.c = x
	return
}

func (z *bytesDecReader) readb(bs []byte) {
	copy(bs, z.readx(uint(len(bs))))
}

func (z *bytesDecReader) readn1() (v uint8) {
	v = z.b[z.c]
	z.c++
	return
}

// func (z *bytesDecReader) readn(num uint8) (bs [8]byte) {
// 	x := z.c + uint(num)
// 	copy(bs[:], z.b[z.c:x]) // slice z.b completely, so we get bounds error if past
// 	z.c = x
// 	return
// }

func (z *bytesDecReader) readn2() (bs [2]byte) {
	x := z.c + 2
	copy(bs[:], z.b[z.c:x]) // slice z.b completely, so we get bounds error if past
	z.c = x
	return
}

func (z *bytesDecReader) readn3() (bs [3]byte) {
	x := z.c + 3
	copy(bs[:], z.b[z.c:x]) // slice z.b completely, so we get bounds error if past
	z.c = x
	return
}

func (z *bytesDecReader) readn4() (bs [4]byte) {
	x := z.c + 4
	copy(bs[:], z.b[z.c:x]) // slice z.b completely, so we get bounds error if past
	z.c = x
	return
}

func (z *bytesDecReader) readn8() (bs [8]byte) {
	x := z.c + 8
	copy(bs[:], z.b[z.c:x]) // slice z.b completely, so we get bounds error if past
	z.c = x
	return
}

func (z *bytesDecReader) jsonReadNum() (out []byte) {
	z.c--
	i := z.c
LOOP:
	if i < uint(len(z.b)) && isNumberChar(z.b[i]) {
		i++
		goto LOOP
	}
	out = z.b[z.c:i]
	z.c = i
	return
}

func (z *bytesDecReader) jsonReadAsisChars() (out []byte) {
	i := z.c
LOOP:
	token := z.b[i]
	i++
	if token == '"' || token == '\\' {
		out = z.b[z.c:i]
		z.c = i
		return // z.b[c:i]
	}
	goto LOOP
}

func (z *bytesDecReader) skipWhitespace() (token byte) {
	i := z.c
LOOP:
	if isWhitespaceChar(z.b[i]) {
		i++
		goto LOOP
	}
	z.c = i + 1
	return z.b[i]
}

// func (z *bytesDecReader) skipWhitespace() (token byte) {
// LOOP:
// 	token = z.b[z.c]
// 	z.c++
// 	if isWhitespaceChar(token) {
// 		goto LOOP
// 	}
// 	return
// }

func (z *bytesDecReader) readUntil(stop byte) (out []byte) {
	i := z.c
LOOP:
	if z.b[i] == stop {
		i++
		out = z.b[z.c : i-1]
		z.c = i
		return
	}
	i++
	goto LOOP
}

// --------------

type decRd struct {
	mtr bool // is maptype a known type?
	str bool // is slicetype a known type?

	be   bool // is binary encoding
	js   bool // is json handle
	jsms bool // is json handle, and MapKeyAsString
	cbor bool // is cbor handle

	bytes bool // is bytes reader
	bufio bool // is this a bufioDecReader?

	rb bytesDecReader
	ri *ioDecReader
	bi *bufioDecReader

	decReader
}

// From out benchmarking, we see the following in terms of performance:
//
// - interface calls
// - branch that can inline what it calls
//
// the if/else-if/else block is expensive to inline.
// Each node of this construct costs a lot and dominates the budget.
// Best to only do an if fast-path else block (so fast-path is inlined).
// This is irrespective of inlineExtraCallCost set in $GOROOT/src/cmd/compile/internal/gc/inl.go
//
// In decRd methods below, we delegate all IO functions into their own methods.
// This allows for the inlining of the common path when z.bytes=true.
// Go 1.12+ supports inlining methods with up to 1 inlined function (or 2 if no other constructs).
//
// However, up through Go 1.13, decRd's readXXX, skip and unreadXXX methods are not inlined.
// Consequently, there is no benefit to do the xxxIO methods for decRd at this time.
// Instead, we have a if/else-if/else block so that IO calls do not have to jump through
// a second unnecessary function call.
//
// If golang inlining gets better and bytesDecReader methods can be inlined,
// then we can revert to using these 2 functions so the bytesDecReader
// methods are inlined and the IO paths call out to a function.
//
// decRd is designed to embed a decReader, and then re-implement some of the decReader
// methods using a conditional branch. We only override the ones that have a bytes version
// that is small enough to be inlined. We use ./run.sh -z to check.
// Right now, only numread and readn1 can be inlined.

func (z *decRd) numread() uint {
	if z.bytes {
		return z.rb.numread()
	} else if z.bufio {
		return z.bi.numread()
	} else {
		return z.ri.numread()
	}
}

func (z *decRd) readn1() (v uint8) {
	if z.bytes {
		// MARKER: manually inline, else this function is not inlined.
		// Keep in sync with bytesDecReader.readn1
		// return z.rb.readn1()
		v = z.rb.b[z.rb.c]
		z.rb.c++
	} else {
		v = z.readn1IO()
	}
	return
}
func (z *decRd) readn1IO() uint8 {
	if z.bufio {
		return z.bi.readn1()
	}
	return z.ri.readn1()
}

func readFull(r io.Reader, bs []byte) (n uint, err error) {
	var nn int
	for n < uint(len(bs)) && err == nil {
		nn, err = r.Read(bs[n:])
		if nn > 0 {
			if err == io.EOF {
				// leave EOF for next time
				err = nil
			}
			n += uint(nn)
		}
	}
	// do not do this below - it serves no purpose
	// if n != len(bs) && err == io.EOF { err = io.ErrUnexpectedEOF }
	return
}

var _ decReader = (*decRd)(nil)
