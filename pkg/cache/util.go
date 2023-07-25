package cache

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"net/url"
)

var (
	ERR_NOT_FOUND = errors.New("cannot find entry in response cache")
	ERR_EXPIRED   = errors.New("entry expired")
)

func hash(key string) string {
	sum := sha256.Sum256([]byte(key))

	return fmt.Sprintf("%x", sum)
}

func makeKey(arch, channel, version string) string {
	values := make(url.Values)
	values.Add(QUERY_PARAM_ARCH, arch)
	values.Add(QUERY_PARAM_CHANNEL, channel)
	values.Add(QUERY_PARAM_VERSION, version)
	return hash(values.Encode())
}
