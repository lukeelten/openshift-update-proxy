package cache

import (
	"github.com/lukeelten/openshift-update-proxy/pkg/utils"
)

type VersionEntry struct {
	Arch    string
	Channel string
	Version string

	Body []byte
}

func (entry VersionEntry) Key() string {
	return utils.MakeKey(entry.Arch, entry.Channel, entry.Version)
}
