package proxy

import (
	"context"
	"github.com/lukeelten/openshift-update-proxy/pkg/client"
	"github.com/lukeelten/openshift-update-proxy/pkg/config"
	"github.com/lukeelten/openshift-update-proxy/pkg/metrics"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"net"
	"net/http"
	"time"
)

type OpenShiftUpdateProxy struct {
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
		OkdClient:       client.NewOpenShiftVersionClient(cfg, m, logger, cfg.OKD.Endpoint, cfg.OKD.Insecure, cfg.OKD.Timeout),
		OpenShiftClient: client.NewOpenShiftVersionClient(cfg, m, logger, cfg.OCP.Endpoint, cfg.OCP.Insecure, cfg.OCP.Timeout),
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

	// OKD
	group.Go(func() error {
		for {
			select {
			case <-time.NewTimer(proxy.Config.Cache.ControllerCycle).C:
				proxy.OkdClient.RefreshEntries()
				proxy.OkdClient.CollectGarbage()
				continue

			case <-ctx.Done():
				return nil

			}
		}
	})

	// OCP
	group.Go(func() error {
		for {
			select {
			case <-time.NewTimer(proxy.Config.Cache.ControllerCycle).C:
				proxy.OpenShiftClient.RefreshEntries()
				proxy.OpenShiftClient.CollectGarbage()
				continue

			case <-ctx.Done():
				return nil

			}
		}
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
	response.Write([]byte("ok"))
}

func (proxy *OpenShiftUpdateProxy) handlerFunc(loadingFunc func(r *http.Request) ([]byte, error)) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		startTime := time.Now()
		defer func() {
			proxy.Metrics.ResponseTime.WithLabelValues(request.URL.Path).Observe(float64(time.Since(startTime).Microseconds()))
		}()

		body, err := loadingFunc(request)

		if err != nil {
			proxy.Metrics.ErrorResponses.WithLabelValues(request.URL.Path).Inc()
			proxy.Logger.Debugw("error when loading version info", "request", request, "err", err)
			proxy.Logger.Errorw("error when loading version info", "err", err)
			writer.WriteHeader(http.StatusInternalServerError)
			writer.Write([]byte(err.Error()))
			return
		}

		writer.WriteHeader(http.StatusOK)
		_, err = writer.Write(body)

		if err != nil {
			proxy.Logger.Debugw("got error when writing response", "request", request, "err", err, "body", body)
			proxy.Logger.Errorw("error writing response", "err", err)
			proxy.Metrics.ErrorResponses.WithLabelValues(request.URL.Path).Inc()
		}
	}
}

func (proxy *OpenShiftUpdateProxy) okdHandler() http.HandlerFunc {
	return proxy.handlerFunc(proxy.OkdClient.Load)
}

func (proxy *OpenShiftUpdateProxy) ocpHandler() http.HandlerFunc {
	return proxy.handlerFunc(proxy.OpenShiftClient.Load)
}
