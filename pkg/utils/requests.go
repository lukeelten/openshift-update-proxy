package utils

import "net/http"

func ExtractQueryParams(req *http.Request) (string, string, string) {
	query := req.URL.Query()
	return query.Get(QUERY_PARAM_ARCH), query.Get(QUERY_PARAM_CHANNEL), query.Get(QUERY_PARAM_VERSION)
}
