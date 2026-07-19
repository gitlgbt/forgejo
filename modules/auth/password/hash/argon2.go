// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package hash

import (
	"encoding/binary"
	"encoding/hex"
	"strings"
	"sync"

	"forgejo.org/modules/log"
	"forgejo.org/modules/util"
	lru "github.com/hashicorp/golang-lru/v2"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/blake2b"
)

func init() {
	MustRegister("argon2", NewArgon2Hasher)
}

// Argon2Hasher implements PasswordHasher
// and uses the Argon2 key derivation function, hybrant variant
type Argon2Hasher struct {
	time    uint32
	memory  uint32
	threads uint8
	keyLen  uint32
}

// codeberg specific
var (
	// Stores an hashed key of the inputs to the hashed value of
	// those inputs, which is expensive to compute.
	hashTokenCache *lru.TwoQueueCache[string, string]

	hashTokenCacheInit = sync.OnceFunc(func() {
		if true {
			var err error
			hashTokenCache, err = lru.New2Q[string, string](4096)
			if err != nil {
				panic(err)
			}
		}
	})
)

// HashWithSaltBytes a provided password and salt
func (hasher *Argon2Hasher) HashWithSaltBytes(password string, salt []byte) string {
	if hasher == nil {
		return ""
	}
	hashTokenCacheInit()
	if hashTokenCache == nil {
		tempHash := argon2.IDKey([]byte(password), salt, hasher.time, hasher.memory, hasher.threads, hasher.keyLen)
		return hex.EncodeToString(tempHash)
	}

	// codeberg specific
	// Use blake2b for a fast hash of the inputs:
	// H(len(token)||token||len(salt)||salt||time||memory||threads||keylen)
	// Where len(x) is the length of x in little endian, always 8 bytes.
	// Where || is the concatenation of two bitstrings.
	h, _ := blake2b.New256(nil)

	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], uint64(len(password)))
	h.Write(buf[:])
	h.Write([]byte(password))
	binary.LittleEndian.PutUint64(buf[:], uint64(len(salt)))
	h.Write(buf[:])
	h.Write([]byte(salt))
	binary.LittleEndian.PutUint64(buf[:], uint64(hasher.time))
	h.Write(buf[:])
	binary.LittleEndian.PutUint64(buf[:], uint64(hasher.memory))
	h.Write(buf[:])
	binary.LittleEndian.PutUint64(buf[:], uint64(hasher.threads))
	h.Write(buf[:])
	binary.LittleEndian.PutUint64(buf[:], uint64(hasher.keyLen))
	h.Write(buf[:])

	// Safe to convert to string, underlying []byte will not be changed.
	key := util.UnsafeBytesToString(h.Sum(nil))
	// Lookup the key in the cache.
	if hash, ok := hashTokenCache.Get(key); ok {
		return hash
	}

	// Otherwise compute it.
	tempHash := argon2.IDKey([]byte(password), salt, hasher.time, hasher.memory, hasher.threads, hasher.keyLen)
	encodedHash := hex.EncodeToString(tempHash)
	hashTokenCache.Add(key, encodedHash)
	return encodedHash
}

// NewArgon2Hasher is a factory method to create an Argon2Hasher
// The provided config should be either empty or of the form:
// "<time>$<memory>$<threads>$<keyLen>", where <x> is the string representation
// of an integer
func NewArgon2Hasher(config string) *Argon2Hasher {
	// This default configuration uses the following parameters:
	// time=2, memory=64*1024, threads=8, keyLen=50.
	// It will make two passes through the memory, using 64MiB in total.
	// This matches the original configuration for `argon2` prior to storing hash parameters
	// in the database.
	// THESE VALUES MUST NOT BE CHANGED OR BACKWARDS COMPATIBILITY WILL BREAK
	hasher := &Argon2Hasher{
		time:    2,
		memory:  1 << 16,
		threads: 8,
		keyLen:  50,
	}

	if config == "" {
		return hasher
	}

	vals := strings.SplitN(config, "$", 4)
	if len(vals) != 4 {
		log.Error("invalid argon2 hash spec %s", config)
		return nil
	}

	parsed, err := parseUIntParam(vals[0], "time", "argon2", config, nil)
	hasher.time = uint32(parsed)

	parsed, err = parseUIntParam(vals[1], "memory", "argon2", config, err)
	hasher.memory = uint32(parsed)

	parsed, err = parseUIntParam(vals[2], "threads", "argon2", config, err)
	hasher.threads = uint8(parsed)

	parsed, err = parseUIntParam(vals[3], "keyLen", "argon2", config, err)
	hasher.keyLen = uint32(parsed)
	if err != nil {
		return nil
	}

	return hasher
}
