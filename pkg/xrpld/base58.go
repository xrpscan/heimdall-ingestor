package xrpld

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
)

// XRPL Base58Check encoding.
//
// The XRP Ledger uses a Base58Check encoding identical to Bitcoin's algorithm but with a different
// alphabet: "rpshnaf39wBUDNEGHJKLM4PQRST7VWXYZ2bcdeCg65jkm8oFqi1tuvAxyz". The alphabet excludes
// visually ambiguous characters (0, O, I, l) to prevent transcription errors.
//
// The encoding format is: [type-prefix (1 byte)] [payload] [checksum (4 bytes)].
// The checksum is the first 4 bytes of SHA256(SHA256(type-prefix + payload)).
//
// Different type prefixes produce different leading characters in the encoded output:
//
//	0x00 → "r" (account address, 20-byte payload)
//	0x1C → "n" (node/validation public key, 33-byte payload)
//	0x21 → "s" (seed value, 16-byte payload)
//	0x23 → "a" (account public key, 33-byte payload)
//
// See: https://xrpl.org/docs/references/protocol/data-types/base58-encodings

const xrplBase58Alphabet = "rpshnaf39wBUDNEGHJKLM4PQRST7VWXYZ2bcdeCg65jkm8oFqi1tuvAxyz"

// nodePublicKeyPrefix is the type byte prepended to 33-byte node/validation public keys before
// Base58Check encoding. It causes the encoded output to start with "n".
const nodePublicKeyPrefix = 0x1C

// EncodeNodePublicKey converts a hex-encoded node/validation public key (e.g. "ED45E80A04D79CB9...")
// to its XRPL Base58Check representation (e.g. "nHU2k8Po4...").
//
// The input is the raw 33-byte public key in hexadecimal. An optional "0x" prefix is stripped.
// The output follows the Base58Check format: [0x1C prefix] [33-byte key] [4-byte checksum],
// base58-encoded with the XRPL alphabet.
//
// See: https://xrpl.org/docs/references/protocol/data-types/base58-encodings
func EncodeNodePublicKey(hexKey string) (string, error) {
	hexKey = strings.TrimPrefix(hexKey, "0x")

	raw, err := hex.DecodeString(hexKey)
	if err != nil {
		return "", fmt.Errorf("failed to decode the hex key to binary: %w", err)
	}

	if len(raw) != 33 {
		return "", fmt.Errorf("malformed key, expected binary length 33, got %d", len(raw))
	}

	// Prepend the 0x1C type prefix that identifies this as a node/validation public key.
	payload := append([]byte{nodePublicKeyPrefix}, raw...)

	// Checksum: first 4 bytes of the double-SHA256 hash of the prefixed payload.
	h1 := sha256.Sum256(payload)
	h2 := sha256.Sum256(h1[:])
	checksum := h2[:4]

	final := append(payload, checksum...)
	return base58Encode(final), nil
}

// base58Encode encodes the input bytes using the XRPL base58 alphabet.
func base58Encode(input []byte) string {
	// Leading zero bytes map to the alphabet's first character ('r').
	leadingZeros := 0
	for _, b := range input {
		if b != 0 {
			break
		}
		leadingZeros++
	}

	n := new(big.Int).SetBytes(input)
	base := big.NewInt(58)
	mod := new(big.Int)
	var result []byte

	for n.Sign() > 0 {
		n.DivMod(n, base, mod)
		result = append(result, xrplBase58Alphabet[mod.Int64()])
	}

	for i := 0; i < leadingZeros; i++ {
		result = append(result, xrplBase58Alphabet[0])
	}

	// DivMod produces digits in reverse order.
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return string(result)
}
