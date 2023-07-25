package upstream

import (
	"crypto/tls"
	"errors"
	"github.com/lukeelten/openshift-update-proxy/pkg/cache"
	"go.uber.org/zap"
	"io"
	"net/http"
	"net/url"
	"time"
)

type UpstreamClient struct {
	Logger *zap.SugaredLogger

	Client http.Client

	Endpoint string
}

func NewUpstreamClient(logger *zap.SugaredLogger, endpoint string, insecure bool, timeout time.Duration) *UpstreamClient {
	client := http.Client{
		Timeout: timeout,
	}
	if insecure {
		tlsConfig := tls.Config{
			InsecureSkipVerify: true,
		}
		transport := http.Transport{
			TLSClientConfig: &tlsConfig,
		}
		client.Transport = &transport
	}

	return &UpstreamClient{
		Logger:   logger,
		Client:   client,
		Endpoint: endpoint,
	}
}

func (client *UpstreamClient) LoadVersionInfo(arch, channel, version string) ([]byte, error) {
	finalUrl, err := client.buildURL(arch, channel, version)
	if err != nil {
		client.Logger.Debugw("cannot build upstream url", "endpoint", client.Endpoint, "error", err)
		return []byte{}, err
	}

	client.Logger.Debugw("Create request", "url", finalUrl)

	req, err := http.NewRequest(http.MethodGet, finalUrl, nil)
	if err != nil {
		client.Logger.Debugw("got error when creating request", "err", err, "url", finalUrl)
		return []byte{}, err
	}

	res, err := client.Client.Do(req)
	if err != nil {
		client.Logger.Debugw("got error on request", "err", err, "request", req)
		return []byte{}, err
	}

	if res.StatusCode >= 400 {
		client.Logger.Debugw("got error response", "response", res, "request", req)
		return []byte{}, errors.New("got error response")
	}

	body, err := io.ReadAll(res.Body)
	defer res.Body.Close()

	if err != nil {
		client.Logger.Debugw("got error on reading response", "err", err, "request", req, "response", res)
		return []byte{}, err
	}

	return body, nil
}

func (client *UpstreamClient) buildURL(arch, channel, version string) (string, error) {
	finalUrl, err := url.Parse(client.Endpoint)
	if err != nil {
		client.Logger.Debugw("cannot parse upstream url", "err", err, "url", client.Endpoint)
		return "", err
	}

	newQuery := url.Values{}

	newQuery.Set(cache.QUERY_PARAM_ARCH, arch)
	newQuery.Set(cache.QUERY_PARAM_CHANNEL, channel)
	newQuery.Set(cache.QUERY_PARAM_VERSION, version)

	finalUrl.RawQuery = newQuery.Encode()
	return finalUrl.String(), nil
}
