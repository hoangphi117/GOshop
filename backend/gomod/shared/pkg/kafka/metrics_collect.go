package kafka

import (
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// collectVec flattens a Prometheus CounterVec into the map shape /healthz
// publishes. Each vector here has exactly one label, so its first label value
// is the map key.
func collectVec(vec *prometheus.CounterVec) map[string]int64 {
	out := make(map[string]int64)
	ch := make(chan prometheus.Metric, 64)
	go func() {
		vec.Collect(ch)
		close(ch)
	}()
	for m := range ch {
		var d dto.Metric
		if err := m.Write(&d); err != nil {
			continue
		}
		labels := d.GetLabel()
		if len(labels) == 0 {
			continue
		}
		out[labels[0].GetValue()] = int64(d.GetCounter().GetValue())
	}
	return out
}
