package proxy

import (
	"context"
	"errors"
	"github.com/acaloiaro/neoq"
	"github.com/acaloiaro/neoq/backends/memory"
	"github.com/acaloiaro/neoq/handler"
	"github.com/acaloiaro/neoq/jobs"
	"github.com/lukeelten/openshift-update-proxy/pkg/cache"
	"github.com/lukeelten/openshift-update-proxy/pkg/config"
	"github.com/lukeelten/openshift-update-proxy/pkg/utils"
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

	Scheduler neoq.Neoq

	Upstreams map[string]*UpstreamProxy

	Server http.Server

	Metrics *UpdateProxyMetrics
}

type UpstreamProxy struct {
	Path   string
	Client *UpstreamClient
	Cache  *cache.OpenShiftVersionCache
}

func NewOpenShiftUpdateProxy(cfg *config.UpdateProxyConfig, logger *zap.SugaredLogger) *OpenShiftUpdateProxy {
	m := NewUpdateProxyMetrics(cfg)

	proxy := OpenShiftUpdateProxy{
		Config: cfg,
		Logger: logger,
		Server: http.Server{
			Addr: cfg.Listen,
		},
		Upstreams: make(map[string]*UpstreamProxy),
		Metrics:   m,
	}

	proxy.Server.Handler = http.HandlerFunc(proxy.ServeHTTP)

	for _, upstreamCfg := range cfg.Upstreams {
		path := strings.TrimPrefix(upstreamCfg.Path, "/")
		upstream := UpstreamProxy{
			Path:   upstreamCfg.Path,
			Cache:  cache.NewOpenShiftVersionCache(),
			Client: NewUpstreamClient(logger, upstreamCfg),
		}

		proxy.Upstreams[path] = &upstream
	}

	return &proxy
}

func (proxy *OpenShiftUpdateProxy) Run(globalContext context.Context) error {
	group, ctx := errgroup.WithContext(globalContext)

	proxy.Server.BaseContext = func(listener net.Listener) context.Context {
		return ctx
	}

	nq, err := neoq.New(ctx, neoq.WithBackend(memory.Backend))
	if err != nil {
		proxy.Logger.Debugw("cannot initialize task broker")
		return err
	}

	nq.SetLogger(utils.WrapLogger(proxy.Logger))
	proxy.Scheduler = nq
	// Run cleanup every 30 minutes
	err = proxy.Scheduler.StartCron(ctx, "* * * * */30", proxy.cleanupHandler())
	if err != nil {
		return err
	}

	// Run refresh controller very 15 minutes
	err = proxy.Scheduler.StartCron(ctx, "* * * * */15", proxy.refreshHandler())
	if err != nil {
		return err
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

	// shutdown queue
	group.Go(func() error {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		proxy.Scheduler.Shutdown(shutdownCtx)
		return nil
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

	upstream, ok := proxy.Upstreams[path]
	if !ok {
		response.WriteHeader(http.StatusNotFound)
		return
	}

	arch, channel, version := utils.ExtractQueryParams(req)
	versionBody, err := upstream.Cache.Get(arch, channel, version)
	if err != nil {
		proxy.Logger.Infow("loading info from upstream", "arch", arch, "channel", channel, "version", version)
		versionBody, err = upstream.Client.LoadVersionInfo(arch, channel, version)
		if err != nil {
			proxy.Logger.Debugw("got error when loading upstream", "error", err, "arch", arch, "channel", channel, "version", version, "endpoint", upstream.Client.Endpoint, "request", req)
			proxy.Logger.Errorw("error loading from upstream", "err", err)
			proxy.Metrics.ErrorResponses.WithLabelValues(strconv.Itoa(http.StatusInternalServerError)).Inc()
			response.WriteHeader(http.StatusInternalServerError)
			return
		}

		upstream.Cache.Set(arch, channel, version, versionBody)
	}

	response.Header().Set("Content-Type", "application/vnd.redhat.cincinnati.graph+json; version=1.0")
	response.WriteHeader(http.StatusOK)
	_, err = response.Write(versionBody)
	defer func() {
		elapsed := time.Since(startTime)
		proxy.Metrics.ResponseTime.WithLabelValues(upstream.Client.Endpoint, arch, channel, version).Observe(float64(elapsed))
	}()

	if err != nil {
		proxy.Logger.Errorw("error writing body", "err", err, "path", upstream.Path)
		proxy.Logger.Debugw("error", "err", err, "request", req, "endpoint", upstream.Client.Endpoint, "body", versionBody)
		return
	}
}

func (proxy *OpenShiftUpdateProxy) cleanupHandler() handler.Handler {
	h := handler.NewPeriodic(func(ctx context.Context) error {
		now := time.Now()
		for _, upstream := range proxy.Upstreams {
			num := upstream.Cache.DeleteAll(func(entry *cache.VersionEntry) bool {
				return entry.LastAccessed.Add(proxy.Config.Cache.EvictAfter).Before(now)
			})

			proxy.Logger.Debugw("Deleted entries", "path", upstream.Path, "num", num)
			if num > 0 {
				proxy.Logger.Infow("Deleted entries from cache", "path", upstream.Path, "entries", num)
			}
		}

		return nil
	})

	h.WithOptions(handler.JobTimeout(utils.DEFAULT_JOB_TIMEOUT), handler.Concurrency(1))

	return h
}

func (proxy *OpenShiftUpdateProxy) updateEntryHandler() handler.Handler {
	h := handler.New(utils.QUEUE_REFRESH_ENTRY, func(ctx context.Context) error {
		job, err := jobs.FromContext(ctx)
		if err != nil {
			return err
		}

		upstream := job.Payload["upstream"].(string)
		arch := job.Payload["arch"].(string)
		channel := job.Payload["channel"].(string)
		version := job.Payload["version"].(string)

		if upstreamProxy, ok := proxy.Upstreams[upstream]; ok {
			proxy.Logger.Debugw("start refresh entry", "path", upstreamProxy.Path, "arch", arch, "channel", channel, "version", version)

			body, err := upstreamProxy.Client.LoadVersionInfo(arch, channel, version)
			if err != nil {
				proxy.Logger.Errorw("got error refreshing entry", "err", err, "path", upstreamProxy.Path, "arch", arch, "channel", channel, "version", version)
				return err
			}

			upstreamProxy.Cache.Set(arch, channel, version, body)
			proxy.Logger.Infow("successfully refreshed entry", "path", upstreamProxy.Path, "arch", arch, "channel", channel, "version", version)

			proxy.Scheduler.Enqueue(context.Background(), &jobs.Job{
				Queue: utils.QUEUE_REFRESH_ENTRY,
				Payload: map[string]any{
					"upstream": upstream,
					"arch":     arch,
					"channel":  channel,
					"version":  version,
				},
				RunAfter: time.Now().Add(proxy.Config.DefaultLifetime),
				MaxRetries:
			})
		} else {
			return errors.New("invalid upstream")
		}

		return nil
	})

	h.WithOptions(handler.Concurrency(4), handler.JobTimeout(utils.DEFAULT_JOB_TIMEOUT))

	return h
}
