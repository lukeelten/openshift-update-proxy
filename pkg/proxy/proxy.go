package proxy

import (
	"context"
	"crypto/tls"
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

	Healthchecks *cache.HealthCheckCache
	Metrics      *UpdateProxyMetrics
}

func NewOpenShiftUpdateProxy(cfg *config.UpdateProxyConfig, logger *zap.SugaredLogger) *OpenShiftUpdateProxy {
	proxy := OpenShiftUpdateProxy{
		Config:       cfg,
		Logger:       logger,
		Cache:        cache.NewResponseCache(cfg.DefaultTTL),
		Healthchecks: cache.NewHealthCheckCache(cfg, logger),
		Client:       http.Client{},
		Server: http.Server{
			Addr: cfg.Listen,
		},
		Metrics: NewUpdateProxyMetrics(cfg),
	}

	if cfg.Insecure {
		tlsConfig := tls.Config{
			InsecureSkipVerify: true,
		}
		transport := http.Transport{
			TLSClientConfig: &tlsConfig,
		}
		proxy.Client.Transport = &transport
	}

	proxy.Server.Handler = http.HandlerFunc(proxy.ServeHTTP)

	return &proxy
}

func (proxy *OpenShiftUpdateProxy) Run(globalContext context.Context) error {
	group, ctx := errgroup.WithContext(globalContext)

	proxy.Server.BaseContext = func(listener net.Listener) context.Context {
		return ctx
	}

	if proxy.Config.Health.Enabled {
		// Start healthcheck controller
		group.Go(func() error {
			return proxy.Healthchecks.Run(ctx)
		})
	}

	if proxy.Config.Metrics.Enabled {
		// Start metrics server
		group.Go(func() error {
			proxy.Logger.Infow("starting metrics server", "address", proxy.Config.Metrics.Listen)
			return proxy.Metrics.Run()
		})

		// Stop metrics server
		group.Go(func() error {
			<-ctx.Done()
			return proxy.Metrics.Shutdown()
		})
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
	status := proxy.Healthchecks.Alive()
	proxy.Logger.Debugw("got healthcheck", "status", status)
	proxy.Metrics.Healthcheck(status)

	if status {
		response.WriteHeader(http.StatusOK)
		return
	}

	response.WriteHeader(http.StatusInternalServerError)
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

	path := strings.TrimPrefix(req.URL.Path, "/")
	healthPath := strings.TrimPrefix(proxy.Config.Health.Path, "/")
	if strings.HasPrefix(path, healthPath) {
		proxy.healthCheck(response, req)
		return
	}

	proxy.Logger.Infow("Handling request", "path", req.URL.Path, "params", req.URL.RawQuery)

	query := req.URL.Query()
	proxy.Metrics.UpdateInfo(query.Get(QUERY_PARAM_ARCH), query.Get(QUERY_PARAM_CHANNEL), query.Get(QUERY_PARAM_VERSION))

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
