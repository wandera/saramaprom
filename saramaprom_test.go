package saramaprom_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/iimos/saramaprom"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rcrowley/go-metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		got := fmt.Sprint(metricFamilies[0])
		want := `name:"test_subsys_metric" help:"metric" type:SUMMARY metric:<label:<name:"broker" value:"" > label:<name:"topic" value:"x" > summary:<sample_count:100 sample_sum:129 quantile:<quantile:0.05 value:1 > quantile:<quantile:0.1 value:1 > quantile:<quantile:0.25 value:1 > quantile:<quantile:0.5 value:1 > quantile:<quantile:0.75 value:1 > quantile:<quantile:0.9 value:1 > quantile:<quantile:0.95 value:5 > quantile:<quantile:0.99 value:9.949999999999974 > > > `
		assert.Equal(t, want, got)
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
