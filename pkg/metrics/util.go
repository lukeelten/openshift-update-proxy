package metrics

import "github.com/prometheus/client_golang/prometheus"

func counter(subsystem, name string) prometheus.CounterOpts {
	return prometheus.CounterOpts{
		Namespace: METRIC_NAMESPACE,
		Subsystem: subsystem,
		Name:      name,
	}
}

func gauge(subsystem, name string) prometheus.GaugeOpts {
	return prometheus.GaugeOpts{
		Namespace: METRIC_NAMESPACE,
		Subsystem: subsystem,
		Name:      name,
	}
}

func histogram(subsystem, name string) prometheus.HistogramOpts {
	return prometheus.HistogramOpts{
		Namespace: METRIC_NAMESPACE,
		Subsystem: subsystem,
		Name:      name,
	}
}
