package debugapi

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
)

type Service interface {
	http.Handler
	MustRegisterMetrics(cs ...prometheus.Collector)
}

type server struct {
	Options
	http.Handler

	metricsRegistry *prometheus.Registry
}

type Options struct{}

func New(o Options) Service {
	s := &server{
		Options:         o,
		metricsRegistry: newMetricsRegistry(),
	}

	s.setupRouting()

	return s
}
