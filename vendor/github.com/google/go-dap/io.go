// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// This file contains utilities for DAP Base protocol I/O.
// For additional information, see "Base protocol" section in
// https://microsoft.github.io/debug-adapter-protocol/overview.

package dap

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// BaseProtocolError represents base protocol error, which occurs when the raw
// message does not conform to the header+content format of the base protocol.
type BaseProtocolError struct {
	Err string
}

func (bpe *BaseProtocolError) Error() string { return bpe.Err }

var (
	// ErrHeaderDelimiterNotCrLfCrLf is returned when only partial header
	// delimiter \r\n\r\n is encountered.
	ErrHeaderDelimiterNotCrLfCrLf = &BaseProtocolError{fmt.Sprintf("header delimiter is not %q", crLfcrLf)}

	// ErrHeaderNotContentLength is returned when the parsed header is
	// not of valid Content-Length format.
	ErrHeaderNotContentLength = &BaseProtocolError{fmt.Sprintf("header format is not %q", contentLengthHeaderRegex)}

	// ErrHeaderContentTooLong is returned when the content length specified in
	// the header is above contentMaxLength.
	ErrHeaderContentTooLong = &BaseProtocolError{fmt.Sprintf("content length over %v bytes", contentMaxLength)}
)

const (
	crLfcrLf               = "\r\n\r\n"
	contentLengthHeaderFmt = "Content-Length: %d\r\n\r\n"
	contentMaxLength       = 4 * 1024 * 1024
)

var (
	contentLengthHeaderRegex = regexp.MustCompile("^Content-Length: ([0-9]+)$")
)

// WriteBaseMessage formats content with Content-Length header and delimiters
// as per the base protocol and writes the resulting message to w.
func WriteBaseMessage(w io.Writer, content []byte) error {
	header := fmt.Sprintf(contentLengthHeaderFmt, len(content))
	if _, err := w.Write([]byte(header)); err != nil {
		return err
	}
	_, err := w.Write(content)
	return err
}

// ReadBaseMessage reads one message from r consisting of a Content-Length
// header and a content part. It parses the header to determine the size of
// the content part and extracts and returns the actual content of the message.
// Returns nil bytes on error, which can be one of the standard IO errors or
// a BaseProtocolError defined in this package.
func ReadBaseMessage(r *bufio.Reader) ([]byte, error) {
	contentLength, err := readContentLengthHeader(r)
	if err != nil {
		return nil, err
	}
	if contentLength > contentMaxLength {
		return nil, ErrHeaderContentTooLong
	}
	content := make([]byte, contentLength)
	if _, err = io.ReadFull(r, content); err != nil {
		return nil, err
	}
	return content, nil
}

// readContentLengthHeader looks for the only header field that is supported
// and required:
// 		Content-Length: [0-9]+\r\n\r\n
// Extracts and returns the content length.
func readContentLengthHeader(r *bufio.Reader) (contentLength int64, err error) {
	// Look for <some header>\r\n\r\n
	headerWithCr, err := r.ReadString('\r')
	if err != nil {
		return 0, err
	}
	nextThree := make([]byte, 3)
	if _, err = io.ReadFull(r, nextThree); err != nil {
		return 0, err
	}
	if string(nextThree) != "\n\r\n" {
		return 0, ErrHeaderDelimiterNotCrLfCrLf
	}

	// If header is in the right format, get the length
	header := strings.TrimSuffix(headerWithCr, "\r")
	headerAndLength := contentLengthHeaderRegex.FindStringSubmatch(header)
	if len(headerAndLength) < 2 {
		return 0, ErrHeaderNotContentLength
	}
	return strconv.ParseInt(headerAndLength[1], 10, 64)
}

// WriteProtocolMessage encodes message and writes it to w.
func WriteProtocolMessage(w io.Writer, message Message) error {
	b, err := json.Marshal(message)
	if err != nil {
		return err
	}
	return WriteBaseMessage(w, b)
}

// ReadProtocolMessage reads a message from r, decodes and returns it.
func ReadProtocolMessage(r *bufio.Reader) (Message, error) {
	content, err := ReadBaseMessage(r)
	if err != nil {
		return nil, err
	}
	return DecodeProtocolMessage(content)
}
