/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2017 Load Impact
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
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"hash"

	"golang.org/x/crypto/md4"
	"golang.org/x/crypto/ripemd160"

	"github.com/loadimpact/k6/js/common"
)

type Crypto struct{}

type Hasher struct {
	ctx context.Context

	hash hash.Hash
}

func New() *Crypto {
	return &Crypto{}
}

func (*Crypto) RandomBytes(ctx context.Context, size int) []byte {
	if size < 1 {
		common.Throw(common.GetRuntime(ctx), errors.New("Invalid size"))
	}
	bytes := make([]byte, size)
	_, err := rand.Read(bytes)
	if err != nil {
		common.Throw(common.GetRuntime(ctx), err)
	}
	return bytes
}

func (c *Crypto) Md4(ctx context.Context, input []byte, outputEncoding string) string {
	hasher := c.CreateHash(ctx, "md4")
	hasher.Update(input)
	return hasher.Digest(outputEncoding)
}

func (c *Crypto) Md5(ctx context.Context, input []byte, outputEncoding string) string {
	hasher := c.CreateHash(ctx, "md5")
	hasher.Update(input)
	return hasher.Digest(outputEncoding)
}

func (c *Crypto) Sha1(ctx context.Context, input []byte, outputEncoding string) string {
	hasher := c.CreateHash(ctx, "sha1")
	hasher.Update(input)
	return hasher.Digest(outputEncoding)
}

func (c *Crypto) Sha256(ctx context.Context, input []byte, outputEncoding string) string {
	hasher := c.CreateHash(ctx, "sha256")
	hasher.Update(input)
	return hasher.Digest(outputEncoding)
}

func (c *Crypto) Sha384(ctx context.Context, input []byte, outputEncoding string) string {
	hasher := c.CreateHash(ctx, "sha384")
	hasher.Update(input)
	return hasher.Digest(outputEncoding)
}

func (c *Crypto) Sha512(ctx context.Context, input []byte, outputEncoding string) string {
	hasher := c.CreateHash(ctx, "sha512")
	hasher.Update(input)
	return hasher.Digest(outputEncoding)
}

func (c *Crypto) Sha512_224(ctx context.Context, input []byte, outputEncoding string) string {
	hasher := c.CreateHash(ctx, "sha512_224")
	hasher.Update(input)
	return hasher.Digest(outputEncoding)
}

func (c *Crypto) Sha512_256(ctx context.Context, input []byte, outputEncoding string) string {
	hasher := c.CreateHash(ctx, "sha512_256")
	hasher.Update(input)
	return hasher.Digest(outputEncoding)
}

func (c *Crypto) Ripemd160(ctx context.Context, input []byte, outputEncoding string) string {
	hasher := c.CreateHash(ctx, "ripemd160")
	hasher.Update(input)
	return hasher.Digest(outputEncoding)
}

func (*Crypto) CreateHash(ctx context.Context, algorithm string) *Hasher {
	hasher := Hasher{}
	hasher.ctx = ctx

	switch algorithm {
	case "md4":
		hasher.hash = md4.New()
	case "md5":
		hasher.hash = md5.New()
	case "sha1":
		hasher.hash = sha1.New()
	case "sha256":
		hasher.hash = sha256.New()
	case "sha384":
		hasher.hash = sha512.New384()
	case "sha512_224":
		hasher.hash = sha512.New512_224()
	case "sha512_256":
		hasher.hash = sha512.New512_256()
	case "sha512":
		hasher.hash = sha512.New()
	case "ripemd160":
		hasher.hash = ripemd160.New()
	}

	return &hasher
}

func (hasher *Hasher) Update(input []byte) {
	_, err := hasher.hash.Write(input)
	if err != nil {
		common.Throw(common.GetRuntime(hasher.ctx), err)
	}
}

func (hasher *Hasher) Digest(outputEncoding string) string {
	sum := hasher.hash.Sum(nil)

	switch outputEncoding {
	case "base64":
		return base64.StdEncoding.EncodeToString(sum)

	case "base64url":
		return base64.URLEncoding.EncodeToString(sum)

	case "base64rawurl":
		return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(sum)

	case "hex":
		return hex.EncodeToString(sum)

	default:
		err := errors.New("Invalid output encoding: " + outputEncoding)
		common.Throw(common.GetRuntime(hasher.ctx), err)
	}

	return ""
}

func (c Crypto) CreateHMAC(ctx context.Context, algorithm string, key string) *Hasher {
	hasher := Hasher{}
	hasher.ctx = ctx
	keyBuffer := []byte(key)

	switch algorithm {
	case "md4":
		hasher.hash = hmac.New(md4.New, keyBuffer)
	case "md5":
		hasher.hash = hmac.New(md5.New, keyBuffer)
	case "sha1":
		hasher.hash = hmac.New(sha1.New, keyBuffer)
	case "sha256":
		hasher.hash = hmac.New(sha256.New, keyBuffer)
	case "sha384":
		hasher.hash = hmac.New(sha512.New384, keyBuffer)
	case "sha512_224":
		hasher.hash = hmac.New(sha512.New512_224, keyBuffer)
	case "sha512_256":
		hasher.hash = hmac.New(sha512.New512_256, keyBuffer)
	case "sha512":
		hasher.hash = hmac.New(sha512.New, keyBuffer)
	case "ripemd160":
		hasher.hash = hmac.New(ripemd160.New, keyBuffer)
	default:
		err := errors.New("Invalid algorithm: " + algorithm)
		common.Throw(common.GetRuntime(hasher.ctx), err)
	}

	return &hasher
}

func (c *Crypto) Hmac(ctx context.Context, algorithm string, key string, input []byte, outputEncoding string) string {
	hasher := c.CreateHMAC(ctx, algorithm, key)
	hasher.Update(input)
	return hasher.Digest(outputEncoding)
}
