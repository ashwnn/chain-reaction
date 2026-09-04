package benchmark

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

const seedDomain = "chain-reaction/benchmark.v2/"

// CommitSeed returns a public, domain-separated commitment to a private seed.
func CommitSeed(seed []byte, scope string) (string, error) {
	if len(seed) < 16 {
		return "", fmt.Errorf("seed must contain at least 16 bytes")
	}
	if err := validateIdentifier("seed scope", scope); err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, []byte(seedDomain+"commitment/"+scope))
	_, _ = mac.Write(seed)
	return hex.EncodeToString(mac.Sum(nil)), nil
}

// DeriveDNSName returns a neutral DNS-safe name that never embeds seed or scope
// text. Scope provides domain separation only.
func DeriveDNSName(seed []byte, scope string, length int) (string, error) {
	if length < 8 || length > 63 {
		return "", fmt.Errorf("DNS name length must be between 8 and 63")
	}
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	name := make([]byte, length)
	name[0], name[1], name[2] = 'c', 'r', '-'
	index := 3
	for counter := uint64(0); index < length; counter++ {
		digest, err := derive(seed, "dns-name/"+scope, counter)
		if err != nil {
			return "", err
		}
		for _, value := range digest {
			// 252 is the largest multiple of the alphabet length below 256.
			// Rejection sampling avoids modulo bias in the generated name.
			if value >= 252 {
				continue
			}
			name[index] = alphabet[int(value)%len(alphabet)]
			index++
			if index == length {
				break
			}
		}
	}
	return string(name), nil
}

// DerivePort uniformly derives a port in the inclusive range [minimum, maximum].
func DerivePort(seed []byte, scope string, minimum, maximum uint16) (uint16, error) {
	if minimum == 0 || minimum > maximum {
		return 0, fmt.Errorf("invalid port range")
	}
	span := uint64(maximum) - uint64(minimum) + 1
	cutoff := -span % span
	for counter := uint64(0); ; counter++ {
		digest, err := derive(seed, "port/"+scope, counter)
		if err != nil {
			return 0, err
		}
		value := binary.BigEndian.Uint64(digest[:8])
		if value < cutoff {
			continue
		}
		return uint16(uint64(minimum) + ((value - cutoff) % span)), nil
	}
}

func derive(seed []byte, scope string, counter uint64) ([]byte, error) {
	if len(seed) < 16 {
		return nil, fmt.Errorf("seed must contain at least 16 bytes")
	}
	if err := validateIdentifier("derivation scope", scope); err != nil {
		return nil, err
	}
	message := make([]byte, len(seedDomain)+len(scope)+8)
	copy(message, seedDomain+scope)
	binary.BigEndian.PutUint64(message[len(message)-8:], counter)
	mac := hmac.New(sha256.New, seed)
	_, _ = mac.Write(message)
	return mac.Sum(nil), nil
}
