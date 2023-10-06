package utils

import (
	"errors"
)

var (
	ERR_NOT_FOUND = errors.New("cannot find entry in cache")
)

const (
	QUERY_PARAM_ARCH    = "arch"
	QUERY_PARAM_CHANNEL = "channel"
	QUERY_PARAM_VERSION = "version"
	METRIC_NAMESPACE    = "openshift_update_proxy"
)
