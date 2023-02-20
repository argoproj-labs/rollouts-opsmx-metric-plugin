package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMetrics(t *testing.T) {
	t.Run("basic flow with parameters defined globally - substitutions should happen for local groups", func(t *testing.T) {
		metricsData := `
        accountName: newacc
        metricWeight: 10
        nanStrategy: ReplaceWithZero
        criticality: HIGH
        metricType: ADVANCED
        metricTemplateSetup:
          percentDiffThreshold: hard
          isNormalize: false
          groups:
            - metrics:
                - riskDirection: Lower
                  name: >-
                    avg(rate(nginx_ingress_controller_ingress_upstream_latency_seconds{namespace="${namespace_key}",
                    service= "${service}",ingress = "${ingress}", quantile ="0.9"}[5m]))
                  watchlist: false
              group: Upstream Service Latency Per Ingress - 90th Percentile`

		metricTemplate, err := processYamlMetrics([]byte(metricsData), "templateMetrics", "${namespace_key},${service},${ingress}")
		assert.Nil(t, err)
		assert.Equal(t, "ADVANCED", metricTemplate.Data.Groups[0].Metrics[0].MetricType)
		assert.Equal(t, float64(10), *metricTemplate.Data.Groups[0].Metrics[0].MetricWeight)
		assert.Equal(t, "ReplaceWithZero", metricTemplate.Data.Groups[0].Metrics[0].NanStrategy)
		assert.Equal(t, "HIGH", metricTemplate.Data.Groups[0].Metrics[0].Criticality)
	})

	t.Run("metricWeight is not defined globally - local metricWeight should apply", func(t *testing.T) {
		metricsData := `
        accountName: newacc
        nanStrategy: ReplaceWithZero
        criticality: HIGH
        metricType: ADVANCED
        metricTemplateSetup:
          percentDiffThreshold: hard
          isNormalize: false
          groups:
            - metrics:
                - riskDirection: Lower
                  name: >-
                    avg(rate(nginx_ingress_controller_ingress_upstream_latency_seconds{namespace="${namespace_key}",
                    service= "${service}",ingress = "${ingress}", quantile ="0.9"}[5m]))
                  watchlist: false
                  metricWeight: 22
              group: Upstream Service Latency Per Ingress - 90th Percentile`

		metricTemplate, err := processYamlMetrics([]byte(metricsData), "templateMetrics", "${namespace_key},${service},${ingress}")
		assert.Nil(t, err)
		assert.Equal(t, float64(22), *metricTemplate.Data.Groups[0].Metrics[0].MetricWeight)

	})

	t.Run("nanStrategy not defined globally - local nanStrategy should apply", func(t *testing.T) {
		metricsData := `
        accountName: newacc
        metricWeight: 1
        criticality: LOW
        metricType: ADVANCED
        metricTemplateSetup:
          percentDiffThreshold: hard
          isNormalize: false
          groups:
            - metrics:
                - riskDirection: Lower
                  name: >-
                    avg(rate(nginx_ingress_controller_ingress_upstream_latency_seconds{namespace="${namespace_key}",
                    service= "${service}",ingress = "${ingress}", quantile ="0.9"}[5m]))
                  watchlist: false
                  nanStrategy: ReplaceWithZero
              group: Upstream Service Latency Per Ingress - 90th Percentile`

		metricTemplate, err := processYamlMetrics([]byte(metricsData), "templateMetrics", "${namespace_key},${service},${ingress}")
		assert.Nil(t, err)
		assert.Equal(t, "ReplaceWithZero", metricTemplate.Data.Groups[0].Metrics[0].NanStrategy)
	})

	t.Run("criticality not defined globally - local criticality should apply", func(t *testing.T) {
		metricsData := `
        accountName: newacc
        metricWeight: 1
        nanStrategy: ReplaceWithZero
        metricType: ADVANCED
        metricTemplateSetup:
          percentDiffThreshold: hard
          isNormalize: false
          groups:
            - metrics:
                - riskDirection: Lower
                  name: >-
                    avg(rate(nginx_ingress_controller_ingress_upstream_latency_seconds{namespace="${namespace_key}",
                    service= "${service}",ingress = "${ingress}", quantile ="0.9"}[5m]))
                  watchlist: false
                  criticality: HIGH
              group: Upstream Service Latency Per Ingress - 90th Percentile`

		metricTemplate, err := processYamlMetrics([]byte(metricsData), "templateMetrics", "${namespace_key},${service},${ingress}")
		assert.Nil(t, err)
		assert.Equal(t, "HIGH", metricTemplate.Data.Groups[0].Metrics[0].Criticality)
	})

	t.Run("metricType is not defined - local MetricType should apply", func(t *testing.T) {
		metricsData := `
        accountName: newacc
        metricWeight: 1
        nanStrategy: ReplaceWithZero
        criticality: LOW
        metricTemplateSetup:
          percentDiffThreshold: hard
          isNormalize: false
          groups:
            - metrics:
                - riskDirection: Lower
                  name: >-
                    avg(rate(nginx_ingress_controller_ingress_upstream_latency_seconds{namespace="${namespace_key}",
                    service= "${service}",ingress = "${ingress}", quantile ="0.9"}[5m]))
                  watchlist: false
                  metricType: ADVANCED
              group: Upstream Service Latency Per Ingress - 90th Percentile`

		metricTemplate, err := processYamlMetrics([]byte(metricsData), "templateMetric", "${namespace_key},${service},${ingress}")
		assert.Nil(t, err)
		assert.Equal(t, "ADVANCED", metricTemplate.Data.Groups[0].Metrics[0].MetricType)
	})

	t.Run("groups array is not defined - an error should be raised", func(t *testing.T) {
		metricsData := `
        accountName: newacc
        metricWeight: 1
        nanStrategy: ReplaceWithZero
        criticality: LOW
        metricType: ADVANCED
        metricTemplateSetup:
          percentDiffThreshold: hard
          isNormalize: false`

		_, err := processYamlMetrics([]byte(metricsData), "templateMetrics", "${namespace_key},${service},${ingress}")
		assert.NotEqual(t, nil, err)
	})

	t.Run("templateName differs from what the yaml has - templateName should be overriden", func(t *testing.T) {
		metricsData := `
        templateName: newTemplate
        accountName: newacc
        metricWeight: 1
        nanStrategy: ReplaceWithZero
        criticality: LOW
        metricTemplateSetup:
          percentDiffThreshold: hard
          isNormalize: false
          groups:
            - metrics:
                - riskDirection: Lower
                  name: >-
                    avg(rate(nginx_ingress_controller_ingress_upstream_latency_seconds{namespace="${namespace_key}",
                    service= "${service}",ingress = "${ingress}", quantile ="0.9"}[5m]))
                  watchlist: false
              group: Upstream Service Latency Per Ingress - 90th Percentile`

		metricTemplate, err := processYamlMetrics([]byte(metricsData), "templateMetrics", "${namespace_key},${service},${ingress}")
		assert.Nil(t, err)
		assert.Equal(t, "templateMetrics", metricTemplate.TemplateName)
	})

	t.Run("filterKey doesnt match the scopeVariables - filterKey should be overriden by scopeVariables", func(t *testing.T) {
		metricsData := `
        templateName: newTemplate
        accountName: newacc
        metricWeight: 1
        nanStrategy: ReplaceWithZero
        criticality: mustHave
        filterKey: ${namespace_key},${service}
        metricTemplateSetup:
          percentDiffThreshold: hard
          isNormalize: false
          groups:
            - metrics:
                - riskDirection: Lower
                  name: >-
                    avg(rate(nginx_ingress_controller_ingress_upstream_latency_seconds{namespace="${namespace_key}",
                    service= "${service}",ingress = "${ingress}", quantile ="0.9"}[5m]))
                  watchlist: false
              group: Upstream Service Latency Per Ingress - 90th Percentile`

		metricTemplate, err := processYamlMetrics([]byte(metricsData), "templateMetrics", "${namespace_key},${service},${ingress}")
		assert.Nil(t, err)
		assert.Equal(t, "${namespace_key},${service},${ingress}", metricTemplate.FilterKey)
	})

}
