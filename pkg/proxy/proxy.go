package proxy

import (
	"context"
	"github.com/lukeelten/openshift-update-proxy/pkg/cache"
	"github.com/lukeelten/openshift-update-proxy/pkg/config"
	"github.com/lukeelten/openshift-update-proxy/pkg/controller"
	"github.com/lukeelten/openshift-update-proxy/pkg/metrics"
	"github.com/lukeelten/openshift-update-proxy/pkg/upstream"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type OpenShiftUpdateProxy struct {
	http.Handler

	Config *config.UpdateProxyConfig
	Logger *zap.SugaredLogger

	Server   http.Server
	OkdCache *cache.OpenShiftVersionCache
	OcpCache *cache.OpenShiftVersionCache

	Metrics *metrics.UpdateProxyMetrics

	OkdClient       *upstream.UpstreamClient
	OpenShiftClient *upstream.UpstreamClient
}

func NewOpenShiftUpdateProxy(cfg *config.UpdateProxyConfig, logger *zap.SugaredLogger) *OpenShiftUpdateProxy {
	m := metrics.NewUpdateProxyMetrics(cfg)
	proxy := OpenShiftUpdateProxy{
		Config:   cfg,
		Logger:   logger,
		OkdCache: cache.NewOpenShiftVersionCache(cfg, logger, m, "okd"),
		OcpCache: cache.NewOpenShiftVersionCache(cfg, logger, m, "ocp"),
		Server: http.Server{
			Addr: cfg.Listen,
		},
		Metrics:         m,
		OkdClient:       upstream.NewUpstreamClient(logger, m, "okd", cfg.OKD.Endpoint, cfg.OKD.Insecure, cfg.OKD.Timeout),
		OpenShiftClient: upstream.NewUpstreamClient(logger, m, "ocp", cfg.OCP.Endpoint, cfg.OCP.Insecure, cfg.OCP.Timeout),
	}

	proxy.Server.Handler = http.HandlerFunc(proxy.ServeHTTP)

	return &proxy
}

func (proxy *OpenShiftUpdateProxy) Run(globalContext context.Context) error {
	group, ctx := errgroup.WithContext(globalContext)

	proxy.Server.BaseContext = func(listener net.Listener) context.Context {
		return ctx
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

	// Run cache refresh collector
	group.Go(func() error {
		return controller.NewRefreshController(proxy.Config, proxy.Logger, proxy.Metrics, proxy.OkdCache, proxy.OkdClient).Run(globalContext)
	})

	group.Go(func() error {
		return controller.NewRefreshController(proxy.Config, proxy.Logger, proxy.Metrics, proxy.OcpCache, proxy.OkdClient).Run(globalContext)
	})

	// Run cache cleanup collector
	group.Go(func() error {
		return controller.NewCleanupController(proxy.Config, proxy.Logger, proxy.OkdCache).Run(globalContext)
	})

	group.Go(func() error {
		return controller.NewCleanupController(proxy.Config, proxy.Logger, proxy.OcpCache).Run(globalContext)
	})

	// Start server
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
	proxy.Logger.Debug("got healthcheck")
	proxy.Metrics.Healthcheck.Inc()

	response.WriteHeader(http.StatusOK)
}

func (proxy *OpenShiftUpdateProxy) ServeHTTP(response http.ResponseWriter, req *http.Request) {
	startTime := time.Now()
	if req.URL == nil {
		proxy.Logger.Error("Got invalid request")
		proxy.Metrics.ErrorResponses.WithLabelValues(strconv.Itoa(http.StatusInternalServerError)).Inc()
		response.WriteHeader(http.StatusInternalServerError)
		return
	}

	if req.Method != http.MethodGet && req.Method != http.MethodHead && req.Method != http.MethodOptions {
		proxy.Logger.Error("Got invalid request method")
		proxy.Logger.Debugw("request", "req", req)
		proxy.Metrics.ErrorResponses.WithLabelValues(strconv.Itoa(http.StatusBadRequest)).Inc()
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
	var versionCache *cache.OpenShiftVersionCache
	var client *upstream.UpstreamClient
	var product string

	okdPath := strings.TrimPrefix(proxy.Config.OKD.Path, "/")
	ocpPath := strings.TrimPrefix(proxy.Config.OCP.Path, "/")
	if strings.HasPrefix(path, okdPath) {
		versionCache = proxy.OkdCache
		client = proxy.OkdClient
		product = "okd"
	} else if strings.HasPrefix(path, ocpPath) {
		versionCache = proxy.OcpCache
		client = proxy.OpenShiftClient
		product = "ocp"
	} else {
		proxy.Logger.Errorw("found unknown path", "path", path)
		proxy.Metrics.ErrorResponses.WithLabelValues(strconv.Itoa(http.StatusNotFound)).Inc()
		response.WriteHeader(http.StatusNotFound)
		return
	}

	arch, channel, version := extractQueryParams(req)

	versionBody, err := versionCache.Get(arch, channel, version)
	if err != nil {
		proxy.Logger.Infow("loading info from upstream", "arch", arch, "channel", channel, "version", version)
		versionBody, err = client.LoadVersionInfo(arch, channel, version)
		if err != nil {
			proxy.Logger.Debugw("got error when loading upstream", "error", err, "arch", arch, "channel", channel, "version", version, "endpoint", client.Endpoint, "request", req)
			proxy.Logger.Errorw("error loading from upstream", "err", err)
			proxy.Metrics.ErrorResponses.WithLabelValues(strconv.Itoa(http.StatusInternalServerError)).Inc()
			response.WriteHeader(http.StatusInternalServerError)
			return
		}

		versionCache.Set(arch, channel, version, versionBody)
	}

	response.Header().Set("Content-Type", "application/vnd.redhat.cincinnati.graph+json; version=1.0")
	response.WriteHeader(http.StatusOK)
	_, err = response.Write(versionBody)

	elapsed := time.Until(startTime)
	proxy.Metrics.ResponseTime.WithLabelValues(product, arch, channel, version).Observe(float64(elapsed.Microseconds()))

	if err != nil {
		proxy.Logger.Errorw("error writing body", "err", err)
		proxy.Logger.Debugw("error", "err", err, "request", req, "endpoint", client.Endpoint, "body", versionBody)
		return
	}
}
