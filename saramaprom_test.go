package saramaprom_test

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/rcrowley/go-metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wandera/saramaprom"
)

func TestMetricCreation(t *testing.T) {
	promRegistry := prometheus.NewRegistry()
	metricsRegistry := metrics.NewRegistry()

	err := metricsRegistry.Register("counter-for-broker-123", metrics.NewCounter())
	require.NoError(t, err)

	saramaprom.ExportMetrics(metricsRegistry, saramaprom.Options{
		Namespace:          "test",
		Subsystem:          "subsys",
		PrometheusRegistry: promRegistry,
	})
	require.NoError(t, err)

	gauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "test",
		Subsystem: "subsys",
		Name:      "counter",
		Help:      "counter",
	}, []string{"broker", "topic"})

	err = promRegistry.Register(gauge)
	require.Error(t, err, "Go-metrics registry didn't get registered to prometheus registry")
}

func TestLabels(t *testing.T) {
	promRegistry := prometheus.NewRegistry()
	metricsRegistry := metrics.NewRegistry()

	err := metricsRegistry.Register("counter1-for-broker-123", metrics.NewCounter())
	require.NoError(t, err)

	err = metricsRegistry.Register("counter2-for-topic-abc", metrics.NewCounter())
	require.NoError(t, err)

	err = metricsRegistry.Register("skip-counter", metrics.NewCounter())
	require.NoError(t, err)

	saramaprom.ExportMetrics(metricsRegistry, saramaprom.Options{
		Namespace:          "test",
		Subsystem:          "subsys",
		PrometheusRegistry: promRegistry,
	})

	t.Run("counter1-for-broker-123", func(t *testing.T) {
		want := []gaugeDetails{{
			name:        "test_subsys_counter1",
			labels:      map[string]string{"broker": "123", "topic": ""},
			gaugeValues: []float64{0},
		}}
		got := getMetricDetails(promRegistry, "test_subsys_counter1")
		assert.Equal(t, want, got)
	})
	t.Run("counter2-for-topic-abc", func(t *testing.T) {
		want := []gaugeDetails{{
			name:        "test_subsys_counter2",
			labels:      map[string]string{"broker": "", "topic": "abc"},
			gaugeValues: []float64{0},
		}}
		got := getMetricDetails(promRegistry, "test_subsys_counter2")
		assert.Equal(t, want, got)
	})
	t.Run("must skip metrics not related to brokers or topics", func(t *testing.T) {
		got := getMetricDetails(promRegistry, "test_subsys_skip_counter")
		assert.Nil(t, got)
	})
}

func TestMetricUpdate(t *testing.T) {
	promRegistry := prometheus.NewRegistry()
	metricsRegistry := metrics.NewRegistry()
	counter := metrics.NewCounter()

	err := metricsRegistry.Register("counter-for-broker-5", counter)
	require.NoError(t, err)

	saramaprom.ExportMetrics(metricsRegistry, saramaprom.Options{
		Namespace:          "test",
		Subsystem:          "subsys",
		PrometheusRegistry: promRegistry,
		RefreshInterval:    100 * time.Millisecond,
	})
	require.NoError(t, err)

	t.Run("by default metric is 0", func(t *testing.T) {
		want := []gaugeDetails{{
			name:        "test_subsys_counter",
			labels:      map[string]string{"broker": "5", "topic": ""},
			gaugeValues: []float64{0},
		}}
		got := getMetricDetails(promRegistry, "test_subsys_counter")
		assert.Equal(t, want, got)
	})

	counter.Inc(10)
	time.Sleep(200 * time.Millisecond)

	t.Run("after 1st increment", func(t *testing.T) {
		want := []gaugeDetails{{
			name:        "test_subsys_counter",
			labels:      map[string]string{"broker": "5", "topic": ""},
			gaugeValues: []float64{10},
		}}
		got := getMetricDetails(promRegistry, "test_subsys_counter")
		assert.Equal(t, want, got)
	})

	counter.Inc(10)
	time.Sleep(200 * time.Millisecond)

	t.Run("after 2nd increment", func(t *testing.T) {
		want := []gaugeDetails{{
			name:        "test_subsys_counter",
			labels:      map[string]string{"broker": "5", "topic": ""},
			gaugeValues: []float64{20},
		}}
		got := getMetricDetails(promRegistry, "test_subsys_counter")
		assert.Equal(t, want, got)
	})
}

func TestSummary(t *testing.T) {
	promRegistry := prometheus.NewRegistry()
	metricsRegistry := metrics.NewRegistry()

	gm := metrics.NewHistogram(metrics.NewUniformSample(1028))
	err := metricsRegistry.Register("metric-for-topic-x", gm)
	require.NoError(t, err)

	for ii := 0; ii < 94; ii++ {
		gm.Update(1)
	}
	for ii := 0; ii < 5; ii++ {
		gm.Update(5)
	}
	gm.Update(10)

	saramaprom.ExportMetrics(metricsRegistry, saramaprom.Options{
		Namespace:          "test",
		Subsystem:          "subsys",
		PrometheusRegistry: promRegistry,
		RefreshInterval:    100 * time.Millisecond,
	})
	require.NoError(t, err)

	time.Sleep(time.Second)
	metricFamilies, err := promRegistry.Gather()
	require.NoError(t, err)

	t.Run("check summary", func(t *testing.T) {
		t.Log(metricFamilies[0].GetMetric()[0].GetSummary())

		assert.Equal(t, "test_subsys_metric", metricFamilies[0].GetName())
		assert.Equal(t, "metric", metricFamilies[0].GetHelp())
		assert.Equal(t, io_prometheus_client.MetricType_SUMMARY, metricFamilies[0].GetType())

		assert.Equal(t, []*io_prometheus_client.LabelPair{
			ptr(io_prometheus_client.LabelPair{
				Name:  ptr("broker"),
				Value: ptr(""),
			}), ptr(io_prometheus_client.LabelPair{
				Name:  ptr("topic"),
				Value: ptr("x"),
			}),
		}, metricFamilies[0].GetMetric()[0].GetLabel())

		assert.Equal(t, uint64(100), metricFamilies[0].GetMetric()[0].GetSummary().GetSampleCount())
		assert.Equal(t, float64(129), metricFamilies[0].GetMetric()[0].GetSummary().GetSampleSum())

		assert.Equal(t, 0.05, metricFamilies[0].GetMetric()[0].GetSummary().GetQuantile()[0].GetQuantile())
		assert.Equal(t, 1.0, metricFamilies[0].GetMetric()[0].GetSummary().GetQuantile()[0].GetValue())
		assert.Equal(t, 0.1, metricFamilies[0].GetMetric()[0].GetSummary().GetQuantile()[1].GetQuantile())
		assert.Equal(t, 1.0, metricFamilies[0].GetMetric()[0].GetSummary().GetQuantile()[1].GetValue())
		assert.Equal(t, 0.25, metricFamilies[0].GetMetric()[0].GetSummary().GetQuantile()[2].GetQuantile())
		assert.Equal(t, 1.0, metricFamilies[0].GetMetric()[0].GetSummary().GetQuantile()[2].GetValue())
		assert.Equal(t, 0.5, metricFamilies[0].GetMetric()[0].GetSummary().GetQuantile()[3].GetQuantile())
		assert.Equal(t, 1.0, metricFamilies[0].GetMetric()[0].GetSummary().GetQuantile()[3].GetValue())
		assert.Equal(t, 0.75, metricFamilies[0].GetMetric()[0].GetSummary().GetQuantile()[4].GetQuantile())
		assert.Equal(t, 1.0, metricFamilies[0].GetMetric()[0].GetSummary().GetQuantile()[4].GetValue())
		assert.Equal(t, 0.9, metricFamilies[0].GetMetric()[0].GetSummary().GetQuantile()[5].GetQuantile())
		assert.Equal(t, 1.0, metricFamilies[0].GetMetric()[0].GetSummary().GetQuantile()[5].GetValue())
		assert.Equal(t, 0.95, metricFamilies[0].GetMetric()[0].GetSummary().GetQuantile()[6].GetQuantile())
		assert.Equal(t, 5.0, metricFamilies[0].GetMetric()[0].GetSummary().GetQuantile()[6].GetValue())
		assert.Equal(t, 0.99, metricFamilies[0].GetMetric()[0].GetSummary().GetQuantile()[7].GetQuantile())
		assert.Equal(t, 9.949999999999974, metricFamilies[0].GetMetric()[0].GetSummary().GetQuantile()[7].GetValue())
	})
}

type gaugeDetails struct {
	name        string
	labels      map[string]string
	gaugeValues []float64
}

func getMetricDetails(pr *prometheus.Registry, fullName string) []gaugeDetails {
	metricFamilies, err := pr.Gather()
	if err != nil {
		panic(err)
	}

	for _, mf := range metricFamilies {
		if mf.GetName() == fullName {
			ret := make([]gaugeDetails, 0)
			for _, m := range mf.Metric {
				gd := gaugeDetails{
					name:        mf.GetName(),
					labels:      make(map[string]string),
					gaugeValues: make([]float64, 0),
				}
				for _, l := range m.GetLabel() {
					gd.labels[l.GetName()] = l.GetValue()
				}

				switch mf.GetType().String() {
				case "GAUGE":
					gd.gaugeValues = append(gd.gaugeValues, m.GetGauge().GetValue())
				case "HISTOGRAM":
					// TODO
					// buckets := make(map[float64]uint64)
					// m.GetHistogram().GetSampleSum()
					// m.GetHistogram().GetSampleCount()
					// for _, b := range m.GetHistogram().GetBucket() {
					//	buckets[b.GetUpperBound()] = b.GetCumulativeCount()
					// }
				}
				ret = append(ret, gd)
			}
			return ret
		}
	}
	return nil
}

func ptr[T any](t T) *T {
	return &t
}
