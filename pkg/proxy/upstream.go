package proxy

import (
	"errors"
	"io"
	"net/http"
	"net/url"
)

const (
	QUERY_PARAM_ARCH       = "arch"
	QUERY_PARAM_CHANNEL    = "channel"
	QUERY_PARAM_CLUSTER_ID = "id"
	QUERY_PARAM_VERSION    = "version"
)

func (proxy *OpenShiftUpdateProxy) loadUpstream(finalUrl string) ([]byte, error) {
	proxy.Logger.Debugw("Create request", "url", finalUrl)

	req, err := http.NewRequest(http.MethodGet, finalUrl, nil)
	if err != nil {
		proxy.Logger.Debugw("got error when creating request", "err", err, "url", finalUrl)
		return []byte{}, err
	}

	res, err := proxy.Client.Do(req)
	if err != nil {
		proxy.Logger.Debugw("got error on request", "err", err, "request", req)
		return []byte{}, err
	}

	if res.StatusCode >= 400 {
		proxy.Logger.Debugw("got error response", "response", res, "request", req)
		return []byte{}, errors.New("got error response")
	}

	body, err := io.ReadAll(res.Body)
	defer res.Body.Close()

	if err != nil {
		proxy.Logger.Debugw("got error on reading response", "err", err, "request", req, "response", res)
		return []byte{}, err
	}

	return body, nil
}

func (proxy *OpenShiftUpdateProxy) buildUpstreamURL(requestUrl *url.URL) (string, error) {
	finalUrl, err := url.Parse(proxy.Config.UpstreamUrl)
	if err != nil {
		proxy.Logger.Debugw("cannot parse upstream url", "err", err, "url", proxy.Config.UpstreamUrl)
		return "", err
	}

	finalUrl.Path = requestUrl.Path
	newQuery := url.Values{}
	requestUrl.Query()
	query := requestUrl.Query()

	newQuery.Set(QUERY_PARAM_ARCH, query.Get(QUERY_PARAM_ARCH))
	newQuery.Set(QUERY_PARAM_CHANNEL, query.Get(QUERY_PARAM_CHANNEL))
	newQuery.Set(QUERY_PARAM_VERSION, query.Get(QUERY_PARAM_VERSION))

	if !proxy.Config.HideClusterId {
		newQuery.Set(QUERY_PARAM_CLUSTER_ID, query.Get(QUERY_PARAM_CLUSTER_ID))
	}

	finalUrl.RawQuery = newQuery.Encode()
	return finalUrl.String(), nil
}
