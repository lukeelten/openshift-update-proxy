package proxy

import (
	"context"
	"github.com/lukeelten/openshift-update-proxy/pkg/client"
	"github.com/lukeelten/openshift-update-proxy/pkg/config"
	"github.com/lukeelten/openshift-update-proxy/pkg/controller"
	"github.com/lukeelten/openshift-update-proxy/pkg/metrics"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"net"
	"net/http"
	"strconv"
	"time"
)

type OpenShiftUpdateProxy struct {
	http.Handler

	Config *config.UpdateProxyConfig
	Logger *zap.SugaredLogger

	Server http.Server

	Metrics *metrics.UpdateProxyMetrics

	OkdClient       *client.OpenShiftVersionClient
	OpenShiftClient *client.OpenShiftVersionClient
}

func NewOpenShiftUpdateProxy(cfg *config.UpdateProxyConfig, logger *zap.SugaredLogger) *OpenShiftUpdateProxy {
	m := metrics.NewUpdateProxyMetrics(cfg)
	mux := http.NewServeMux()

	proxy := OpenShiftUpdateProxy{
		Config: cfg,
		Logger: logger,
		Server: http.Server{
			Addr:    cfg.Listen,
			Handler: mux,
		},
		Metrics:         m,
		OkdClient:       client.NewUpstreamClient(logger, m, "okd", cfg.OKD.Endpoint, cfg.OKD.Insecure, cfg.OKD.Timeout),
		OpenShiftClient: client.NewUpstreamClient(logger, m, "ocp", cfg.OCP.Endpoint, cfg.OCP.Insecure, cfg.OCP.Timeout),
	}

	if proxy.Config.Health.Enabled {
		proxy.Logger.Infow("enabled health endpoint", "endpoint", proxy.Config.Health.Path)
		mux.HandleFunc(proxy.Config.Health.Path, proxy.healthCheck)
	}

	mux.HandleFunc(proxy.Config.OCP.Path, proxy.ocpHandler())
	mux.HandleFunc(proxy.Config.OKD.Path, proxy.okdHandler())

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

func (proxy *OpenShiftUpdateProxy) okdHandler() http.HandlerFunc {

	return func(writer http.ResponseWriter, request *http.Request) {

	}
}

func (proxy *OpenShiftUpdateProxy) ocpHandler() http.HandlerFunc {

	return func(writer http.ResponseWriter, request *http.Request) {

	}
}

func (proxy *OpenShiftUpdateProxy) productHandler(client *client.OpenShiftVersionClient) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		// @todo metrics
		body, err := client.Load(request)
		if err != nil {
			proxy.Logger.Debugw("got error while processing request", "err", err, "request", request)
			proxy.Logger.Errorw("error on request", "err", err)
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}

		writer.WriteHeader(http.StatusOK)
		_, err = writer.Write(body)
		if err != nil {
			proxy.Logger.Debugw("got error writing request", "err", err, "request", request)
			proxy.Logger.Errorw("error on request", "err", err)
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

func (proxy *OpenShiftUpdateProxy) ServeHTTP(response http.ResponseWriter, req *http.Request) {

	proxy.Logger.Infow("Handling request", "path", req.URL.Path, "params", req.URL.RawQuery)

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
