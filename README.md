# saramaprom
is a library for exporting [sarama](https://github.com/Shopify/sarama) metrics (provided through [go-metrics](https://github.com/rcrowley/go-metrics)) to Prometheus. It is
a fork of [saramaprom](https://github.com/iimos/saramaprom/tree/ab69b9d3b9e65611e5377c2fd40882124e491f50) with few fixes
and tweaks:
* go-metrics histograms are registered as Prometheus summaries (to better present client side quantiles)
* removed histogram and timer words from metric names
* removed configuration of optional labels from saramaprom (we never configure it and it was creating additional unnecessary dimension to metrics due to bad implementation)

## Usage
*saramaprom* periodically refreshes exposed Prometheus metrics based on provided `metrics.Registry`. The registry of metrics must be shared
between sarama consumers/producers and *saramaprom* library, so typical usage looks like this:
```
// create shared metric registry
metricRegistry := metrics.NewRegistry()

// create sarama configuration which uses the shared registry
config := sarama.NewConfig()
config.MetricRegistry = metricRegistry

// export metrics from shared metric registry using saramaprom
saramaprom.ExportMetrics(metricRegistry, saramaprom.Options{})
```

`saramaprom.Options` enables you to for example configure how often the metrics are refreshed, namespace of the metrics, etc.

Multiple metric registries can be defined to not mix values of several Kafka cluster.
But in that case the prometheus metrics have to be distinguished by labels. Use `ExtraLabels` option for that:
```
// create new registry and export metrics for cluster Foo 
configFoo := sarama.NewConfig()
configFoo.MetricRegistry = metrics.NewRegistry()
saramaprom.ExportMetrics(configFoo.MetricRegistry, saramaprom.Options{ExtraLabels: map[string]string{"cluster": "foo"}})

// create new registry and export metrics for cluster Bar 
configBar := sarama.NewConfig()
configBar.MetricRegistry = metrics.NewRegistry()
saramaprom.ExportMetrics(configBar.MetricRegistry, saramaprom.Options{ExtraLabels: map[string]string{"cluster": "bar"}})
```
