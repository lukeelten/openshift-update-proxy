package utils

import "github.com/prometheus/client_golang/prometheus"

func Counter(subsystem, name string) prometheus.CounterOpts {
	return prometheus.CounterOpts{
		Namespace: METRIC_NAMESPACE,
		Subsystem: subsystem,
		Name:      name,
	}
}

func Gauge(subsystem, name string) prometheus.GaugeOpts {
	return prometheus.GaugeOpts{
		Namespace: METRIC_NAMESPACE,
		Subsystem: subsystem,
		Name:      name,
	}
}

func Histogram(subsystem, name string) prometheus.HistogramOpts {
	return prometheus.HistogramOpts{
		Namespace: METRIC_NAMESPACE,
		Subsystem: subsystem,
		Name:      name,
	}
}
