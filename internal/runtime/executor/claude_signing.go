package executor

import (
	"encoding/binary"
	"fmt"
	"math/bits"
	"regexp"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const claudeCCHSeed uint64 = 0x6E52736AC806831E

const (
	xxHashPrime1 uint64 = 11400714785074694791
	xxHashPrime2 uint64 = 14029467366897019727
	xxHashPrime3 uint64 = 1609587929392839161
	xxHashPrime4 uint64 = 9650029242287828579
	xxHashPrime5 uint64 = 2870177450012600261
)

var claudeBillingHeaderCCHPattern = regexp.MustCompile(`\bcch=([0-9a-f]{5});`)

func signAnthropicMessagesBody(body []byte) []byte {
	billingHeader := gjson.GetBytes(body, "system.0.text").String()
	if !strings.HasPrefix(billingHeader, "x-anthropic-billing-header:") {
		return body
	}
	if !claudeBillingHeaderCCHPattern.MatchString(billingHeader) {
		return body
	}

	unsignedBillingHeader := claudeBillingHeaderCCHPattern.ReplaceAllString(billingHeader, "cch=00000;")
	unsignedBody, err := sjson.SetBytes(body, "system.0.text", unsignedBillingHeader)
	if err != nil {
		return body
	}

	cch := fmt.Sprintf("%05x", xxh64Checksum(unsignedBody, claudeCCHSeed)&0xFFFFF)
	signedBillingHeader := claudeBillingHeaderCCHPattern.ReplaceAllString(unsignedBillingHeader, "cch="+cch+";")
	signedBody, err := sjson.SetBytes(unsignedBody, "system.0.text", signedBillingHeader)
	if err != nil {
		return unsignedBody
	}
	return signedBody
}

func resolveClaudeKeyConfig(cfg *config.Config, auth *cliproxyauth.Auth) *config.ClaudeKey {
	if cfg == nil || auth == nil {
		return nil
	}

	apiKey, baseURL := claudeCreds(auth)
	if apiKey == "" {
		return nil
	}

	for i := range cfg.ClaudeKey {
		entry := &cfg.ClaudeKey[i]
		cfgKey := strings.TrimSpace(entry.APIKey)
		cfgBase := strings.TrimSpace(entry.BaseURL)
		if !strings.EqualFold(cfgKey, apiKey) {
			continue
		}
		if baseURL != "" && cfgBase != "" && !strings.EqualFold(cfgBase, baseURL) {
			continue
		}
		return entry
	}

	return nil
}

func experimentalCCHSigningEnabled(cfg *config.Config, auth *cliproxyauth.Auth) bool {
	entry := resolveClaudeKeyConfig(cfg, auth)
	return entry != nil && entry.ExperimentalCCHSigning
}

func xxh64Checksum(in []byte, seed uint64) uint64 {
	var h uint64
	p := in

	if len(p) >= 32 {
		v1 := seed + xxHashPrime1 + xxHashPrime2
		v2 := seed + xxHashPrime2
		v3 := seed
		v4 := seed - xxHashPrime1

		for len(p) >= 32 {
			v1 = xxh64Round(v1, binary.LittleEndian.Uint64(p[0:8]))
			v2 = xxh64Round(v2, binary.LittleEndian.Uint64(p[8:16]))
			v3 = xxh64Round(v3, binary.LittleEndian.Uint64(p[16:24]))
			v4 = xxh64Round(v4, binary.LittleEndian.Uint64(p[24:32]))
			p = p[32:]
		}

		h = bits.RotateLeft64(v1, 1) +
			bits.RotateLeft64(v2, 7) +
			bits.RotateLeft64(v3, 12) +
			bits.RotateLeft64(v4, 18)

		h = xxh64MergeRound(h, v1)
		h = xxh64MergeRound(h, v2)
		h = xxh64MergeRound(h, v3)
		h = xxh64MergeRound(h, v4)
	} else {
		h = seed + xxHashPrime5
	}

	h += uint64(len(in))

	for len(p) >= 8 {
		k1 := xxh64Round(0, binary.LittleEndian.Uint64(p[:8]))
		h ^= k1
		h = bits.RotateLeft64(h, 27)*xxHashPrime1 + xxHashPrime4
		p = p[8:]
	}

	if len(p) >= 4 {
		h ^= uint64(binary.LittleEndian.Uint32(p[:4])) * xxHashPrime1
		h = bits.RotateLeft64(h, 23)*xxHashPrime2 + xxHashPrime3
		p = p[4:]
	}

	for _, b := range p {
		h ^= uint64(b) * xxHashPrime5
		h = bits.RotateLeft64(h, 11) * xxHashPrime1
	}

	h ^= h >> 33
	h *= xxHashPrime2
	h ^= h >> 29
	h *= xxHashPrime3
	h ^= h >> 32
	return h
}

func xxh64Round(acc, input uint64) uint64 {
	acc += input * xxHashPrime2
	acc = bits.RotateLeft64(acc, 31)
	acc *= xxHashPrime1
	return acc
}

func xxh64MergeRound(acc, val uint64) uint64 {
	acc ^= xxh64Round(0, val)
	acc = acc*xxHashPrime1 + xxHashPrime4
	return acc
}
