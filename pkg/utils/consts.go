package utils

import (
	"errors"
	"time"
)

var (
	ERR_NOT_FOUND = errors.New("cannot find entry in cache")
)

const (
	QUERY_PARAM_ARCH    = "arch"
	QUERY_PARAM_CHANNEL = "channel"
	QUERY_PARAM_VERSION = "version"

	QUEUE_CLEANUP       = "cleanup"
	QUEUE_REFRESH       = "refresh"
	QUEUE_REFRESH_ENTRY = "refresh-entry"

	DEFAULT_JOB_TIMEOUT = 10 + time.Second
)
