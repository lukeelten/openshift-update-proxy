package utils

import (
	"crypto/sha256"
	"fmt"
	"net/url"
)

func hash(key string) string {
	sum := sha256.Sum256([]byte(key))

	return fmt.Sprintf("%x", sum)
}

func MakeKey(arch, channel, version string) string {
	return hash(MakeQueryString(arch, channel, version))
}

func MakeQueryString(arch, channel, version string) string {
	values := make(url.Values)
	values.Add(QUERY_PARAM_ARCH, arch)
	values.Add(QUERY_PARAM_CHANNEL, channel)
	values.Add(QUERY_PARAM_VERSION, version)
	return values.Encode()
}
