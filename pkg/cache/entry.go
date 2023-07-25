package cache

import (
	"time"
)

const (
	QUERY_PARAM_ARCH    = "arch"
	QUERY_PARAM_CHANNEL = "channel"
	QUERY_PARAM_VERSION = "version"
)

type VersionEntry struct {
	Arch    string
	Channel string
	Version string

	Body []byte

	LastAccessed time.Time
	ValidUntil   time.Time
}

func (entry VersionEntry) Key() string {
	return makeKey(entry.Arch, entry.Channel, entry.Version)
}

func (entry VersionEntry) IsValid() bool {
	return time.Now().Before(entry.ValidUntil)
}
