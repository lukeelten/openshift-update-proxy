package proxy

import (
	"context"
	"github.com/lukeelten/openshift-update-proxy/pkg/cache"
	"github.com/lukeelten/openshift-update-proxy/pkg/config"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"net"
	"net/http"
	"strings"
	"time"
)

type OpenShiftUpdateProxy struct {
	http.Handler

	Config *config.UpdateProxyConfig
	Logger *zap.SugaredLogger

	Client http.Client
	Server http.Server
	Cache  *cache.ResponseCache
}

func NewOpenShiftUpdateProxy(cfg *config.UpdateProxyConfig, logger *zap.SugaredLogger) *OpenShiftUpdateProxy {
	proxy := OpenShiftUpdateProxy{
		Config: cfg,
		Logger: logger,
		Cache:  cache.NewResponseCache(cfg.DefaultTTL),
		Client: http.Client{},
		Server: http.Server{
			Addr: cfg.Listen,
		},
	}

	proxy.Server.Handler = http.HandlerFunc(proxy.ServeHTTP)

	return &proxy
}

func (proxy *OpenShiftUpdateProxy) Run(globalContext context.Context) error {
	group, ctx := errgroup.WithContext(globalContext)

	proxy.Server.BaseContext = func(listener net.Listener) context.Context {
		return ctx
	}

	group.Go(func() error {
		proxy.Logger.Infow("Start listening", "address", proxy.Config.Listen)
		return proxy.Server.ListenAndServe()
	})

	// shutdown server
	group.Go(func() error {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		return proxy.Server.Shutdown(shutdownCtx)
	})

	return group.Wait()
}

func (proxy *OpenShiftUpdateProxy) healthCheck(response http.ResponseWriter, req *http.Request) {

}

func (proxy *OpenShiftUpdateProxy) ServeHTTP(response http.ResponseWriter, req *http.Request) {
	if req.URL == nil {
		proxy.Logger.Error("Got invalid request")
		response.WriteHeader(http.StatusInternalServerError)
		return
	}

	if req.Method != http.MethodGet && req.Method != http.MethodHead && req.Method != http.MethodOptions {
		proxy.Logger.Error("Got invalid request method")
		proxy.Logger.Debugw("request", "req", req)
		response.WriteHeader(http.StatusBadRequest)
		return
	}

	if strings.Contains(req.URL.Path, "health") {
		proxy.healthCheck(response, req)
		return
	}

	proxy.Logger.Infow("Handling request", "path", req.URL.Path, "params", req.URL.RawQuery)

	upstreamUrl, err := proxy.buildUpstreamURL(req.URL)
	if err != nil {
		proxy.Logger.Error("error building upstream url")
		proxy.Logger.Debugw("upstream url", "err", err, "url", upstreamUrl, "config", proxy.Config)
		response.WriteHeader(http.StatusInternalServerError)
		return
	}

	body, err := proxy.Cache.GetOrCompute(upstreamUrl, func() ([]byte, error) {
		return proxy.loadUpstream(upstreamUrl)
	})
	if err != nil {
		proxy.Logger.Error("error requesting upstream body")
		proxy.Logger.Debugw("upstream url", "err", err, "url", upstreamUrl, "config", proxy.Config)
		response.WriteHeader(http.StatusInternalServerError)
		return
	}

	response.Header().Set("Content-Type", "application/vnd.redhat.cincinnati.graph+json; version=1.0")
	response.WriteHeader(http.StatusOK)
	_, err = response.Write(body)

	if err != nil {
		proxy.Logger.Errorw("error writing body", "err", err)
		proxy.Logger.Debugw("error", "err", err, "request", req, "upstreamUrl", upstreamUrl, "body", body)
		return
	}
}
