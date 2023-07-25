package proxy

import (
	"github.com/lukeelten/openshift-update-proxy/pkg/cache"
	"net/http"
)

func extractQueryParams(req *http.Request) (string, string, string) {
	query := req.URL.Query()
	return query.Get(cache.QUERY_PARAM_ARCH), query.Get(cache.QUERY_PARAM_CHANNEL), query.Get(cache.QUERY_PARAM_VERSION)
}
