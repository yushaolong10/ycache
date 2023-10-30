package prometheus

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

const (
	httpApi    = "http_api"
	levelCache = "level_cache"
)

var (
	namespace      = "ycache"
	subsystem      = "demo"
	defaultBuckets = []float64{100, 200, 500, 800, 1000, 3000, 5000, 10000, 20000, 30000, 50000, 80000, 100000, 200000, 300000, 500000, 1000000}
	histogramVec   = make(map[string]*prometheus.HistogramVec)
	counterVec     = make(map[string]*prometheus.CounterVec)
)

func NewHttpHandler() http.Handler {
	return promhttp.Handler()
}

func UpdateHttp(path string, status int, timeUsed int64) {
	if vec, ok := histogramVec[httpApi]; ok {
		vec.WithLabelValues(path, fmt.Sprintf("%d", status)).Observe(float64(timeUsed))
	}
}

func UpdateLevelCache(name, instance, index string, value int64) {
	if vec, ok := counterVec[levelCache]; ok {
		vec.WithLabelValues(name, instance, index).Add(float64(value))
	}
}

func registerHistogram(name string, metrics []string, buckets []float64) error {
	if _, ok := histogramVec[name]; !ok {
		if buckets == nil {
			buckets = defaultBuckets
		}
		histogramVec[name] = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      name,
				Buckets:   buckets,
			},
			metrics,
		)
		return prometheus.Register(histogramVec[name])
	}
	return fmt.Errorf("prometheus histogram name(%s) repeats", name)
}

func registerCounter(name string, metrics []string) error {
	if _, ok := counterVec[name]; !ok {
		counterVec[name] = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      name,
			},
			metrics,
		)
		return prometheus.Register(counterVec[name])
	}
	return fmt.Errorf("prometheus counter name(%s) repeats", name)
}

func init() {
	_ = registerHistogram(httpApi, []string{"path", "status"}, nil)
	_ = registerCounter(levelCache, []string{"name", "instance", "index"})
}
