package cache

import (
	"crypto/sha256"
	"errors"
	"fmt"
)

var (
	ERR_NOT_FOUND = errors.New("cannot find entry in response cache")
	ERR_EXPIRED   = errors.New("entry expired")
)

func hash(key string) string {
	sum := sha256.Sum256([]byte(key))

	return fmt.Sprintf("%x", sum)
}
