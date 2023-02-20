package plugin

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	kubetesting "k8s.io/client-go/testing"
)

func TestOpsmxMetricValidations(t *testing.T) {
	logCtx := *log.WithFields(log.Fields{"plugin-test": "opsmx"})
	rpcPluginImp := &RpcPlugin{
		LogCtx:        logCtx,
		kubeclientset: k8sfake.NewSimpleClientset(),
		client:        NewHttpClient(),
	}
	opsmxProfileData := opsmxProfile{cdIntegration: "true",
		user:        "admin",
		sourceName:  "sourceName",
		opsmxIsdUrl: "https://opsmx.test.tst"}

	t.Run("pass score is less than marginal score - an error should be raised", func(t *testing.T) {
		opsmxMetric := OPSMXMetric{Application: "newapp",
			LifetimeMinutes: 3,
			Pass:            80,
			Marginal:        85,
			Services: []OPSMXService{{LogTemplateName: "logtemp",
				LogScopeVariables: "pod_name",
				CanaryLogScope:    "podHashCanary",
				BaselineLogScope:  "podHashBaseline",
			}},
		}

		_, err := opsmxMetric.process(rpcPluginImp, opsmxProfileData, "ns")
		assert.Contains(t, err.Error(), "pass score cannot be less than marginal score")
	})

	t.Run("neither lifetimeMinutes nor endTime is provided - an error should be raised", func(t *testing.T) {
		opsmxMetric := OPSMXMetric{Application: "newapp",
			Pass:     90,
			Marginal: 85,
			Services: []OPSMXService{{LogTemplateName: "logtemp",
				LogScopeVariables: "pod_name",
				CanaryLogScope:    "podHashCanary",
				BaselineLogScope:  "podHashBaseline",
			}},
		}

		_, err := opsmxMetric.process(rpcPluginImp, opsmxProfileData, "ns")
		assert.Contains(t, err.Error(), "provide either lifetimeMinutes or end time")
	})

	t.Run("canaryStartTime and baselineStartTime are different when using endTime - an error should be raised", func(t *testing.T) {
		opsmxMetric := OPSMXMetric{Application: "newapp",
			BaselineStartTime: "2022-08-02T13:15:00Z",
			CanaryStartTime:   "2022-08-02T13:25:00Z",
			EndTime:           "2022-08-02T13:45:10Z",
			Pass:              90,
			Marginal:          85,
			Services: []OPSMXService{{LogTemplateName: "logtemp",
				LogScopeVariables: "pod_name",
				CanaryLogScope:    "podHashCanary",
				BaselineLogScope:  "podHashBaseline",
			}},
		}

		_, err := opsmxMetric.process(rpcPluginImp, opsmxProfileData, "ns")
		assert.Contains(t, err.Error(), "both canaryStartTime and baselineStartTime should be kept same while using endTime argument for analysis")
	})

	t.Run("canaryStartTime is greater than endTime - an error should be raised", func(t *testing.T) {
		opsmxMetric := OPSMXMetric{Application: "newapp",
			CanaryStartTime:   "2022-08-02T13:25:00Z",
			BaselineStartTime: "2022-08-02T13:25:00Z",
			EndTime:           "2022-08-01T13:45:10Z",
			Pass:              90,
			Marginal:          85,
			Services: []OPSMXService{{LogTemplateName: "logtemp",
				LogScopeVariables: "pod_name",
				CanaryLogScope:    "podHashCanary",
				BaselineLogScope:  "podHashBaseline",
			}},
		}

		_, err := opsmxMetric.process(rpcPluginImp, opsmxProfileData, "ns")
		assert.Contains(t, err.Error(), "canaryStartTime cannot be greater than endTime")
	})

	t.Run("incorrect time format CanaryStartTime- an error should be raised", func(t *testing.T) {
		opsmxMetric := OPSMXMetric{Application: "newapp",
			BaselineStartTime: "2022-08-02T13:15:00Z",
			CanaryStartTime:   "2022-O8-02T13:15:00Z",
			LifetimeMinutes:   3,
			Pass:              90,
			Marginal:          85,
			Services: []OPSMXService{{LogTemplateName: "logtemp",
				LogScopeVariables: "pod_name",
				CanaryLogScope:    "podHashCanary",
				BaselineLogScope:  "podHashBaseline",
			}},
		}

		_, err := opsmxMetric.process(rpcPluginImp, opsmxProfileData, "ns")
		assert.Contains(t, err.Error(), "error in parsing canaryStartTime")
	})

	t.Run("incorrect time format BaselineStartTime- an error should be raised", func(t *testing.T) {
		opsmxMetric := OPSMXMetric{Application: "newapp",
			BaselineStartTime: "2022-O8-02T13:15:00Z",
			CanaryStartTime:   "2022-08-02T13:15:00Z",
			LifetimeMinutes:   3,
			Pass:              90,
			Marginal:          85,
			Services: []OPSMXService{{LogTemplateName: "logtemp",
				LogScopeVariables: "pod_name",
				CanaryLogScope:    "podHashCanary",
				BaselineLogScope:  "podHashBaseline",
			}},
		}

		_, err := opsmxMetric.process(rpcPluginImp, opsmxProfileData, "ns")
		assert.Contains(t, err.Error(), "error in parsing baselineStartTime")
	})

	t.Run("incorrect time format EndTime - an error should be raised", func(t *testing.T) {
		opsmxMetric := OPSMXMetric{Application: "newapp",
			BaselineStartTime: "2022-08-02T13:15:00Z",
			CanaryStartTime:   "2022-08-02T13:15:00Z",
			EndTime:           "2022-O8-02T13:45:10Z",
			Pass:              90,
			Marginal:          85,
			Services: []OPSMXService{{LogTemplateName: "logtemp",
				LogScopeVariables: "pod_name",
				CanaryLogScope:    "podHashCanary",
				BaselineLogScope:  "podHashBaseline",
			}},
		}

		_, err := opsmxMetric.process(rpcPluginImp, opsmxProfileData, "ns")
		assert.Contains(t, err.Error(), "error in parsing endTime")
	})

	t.Run("lifetimeMinutes cannot be less than 3 minutes - an error should be raised", func(t *testing.T) {
		opsmxMetric := OPSMXMetric{Application: "newapp",
			LifetimeMinutes: 2,
			Pass:            90,
			Marginal:        85,
			Services: []OPSMXService{{LogTemplateName: "logtemp",
				LogScopeVariables: "pod_name",
				CanaryLogScope:    "podHashCanary",
				BaselineLogScope:  "podHashBaseline",
			}},
		}

		_, err := opsmxMetric.process(rpcPluginImp, opsmxProfileData, "ns")
		assert.Contains(t, err.Error(), "lifetimeMinutes cannot be less than 3 minutes")
	})
	t.Run("intervalTime cannot be less than 3 minutes- an error should be raised", func(t *testing.T) {
		opsmxMetric := OPSMXMetric{Application: "newapp",
			LifetimeMinutes: 9,
			IntervalTime:    2,
			Pass:            90,
			Marginal:        85,
			Services: []OPSMXService{{LogTemplateName: "logtemp",
				LogScopeVariables: "pod_name",
				CanaryLogScope:    "podHashCanary",
				BaselineLogScope:  "podHashBaseline",
			}},
		}

		_, err := opsmxMetric.process(rpcPluginImp, opsmxProfileData, "ns")
		assert.Contains(t, err.Error(), "intervalTime cannot be less than 3 minutes")
	})
	t.Run("intervalTime should be given along with lookBackType to perform interval analysis - an error should be raised", func(t *testing.T) {
		opsmxMetric := OPSMXMetric{Application: "newapp",
			LifetimeMinutes: 9,
			LookBackType:    "growing",
			Pass:            90,
			Marginal:        85,
			Services: []OPSMXService{{LogTemplateName: "logtemp",
				LogScopeVariables: "pod_name",
				CanaryLogScope:    "podHashCanary",
				BaselineLogScope:  "podHashBaseline",
			}},
		}

		_, err := opsmxMetric.process(rpcPluginImp, opsmxProfileData, "ns")
		assert.Contains(t, err.Error(), "intervalTime should be given along with lookBackType to perform interval analysis")
	})

	t.Run("no Services are mentioned - an error should be raised", func(t *testing.T) {
		opsmxMetric := OPSMXMetric{Application: "newapp",
			LifetimeMinutes: 9,
			Pass:            90,
			Marginal:        85,
		}
		_, err := opsmxMetric.process(rpcPluginImp, opsmxProfileData, "ns")
		assert.Contains(t, err.Error(), "at least one of log or metric context must be provided")
	})

	t.Run("no log and metric details are mentioned - an error should be raised", func(t *testing.T) {
		opsmxMetric := OPSMXMetric{Application: "newapp",
			LifetimeMinutes: 9,
			Pass:            90,
			Marginal:        85,
			Services: []OPSMXService{
				{
					ServiceName: "service1",
				},
				{
					ServiceName: "service2",
				},
			}}
		_, err := opsmxMetric.process(rpcPluginImp, opsmxProfileData, "ns")
		assert.Contains(t, err.Error(), "at least one of log or metric context must be provided")
	})

	t.Run("mismatch in log scope variables and baseline/canary log scope - an error should be raised", func(t *testing.T) {
		opsmxMetric := OPSMXMetric{Application: "newapp",
			LifetimeMinutes: 9,
			Pass:            90,
			Marginal:        85,
			Services: []OPSMXService{
				{
					MetricScopeVariables: "pod_name",
					BaselineMetricScope:  "podHashBaseline",
					CanaryMetricScope:    "podHashCanary",
					MetricTemplateName:   "metrictemplate",
				},
				{
					MetricScopeVariables: "pod_name",
					BaselineMetricScope:  "podHashBaseline",
					CanaryMetricScope:    "podHashCanary",
					MetricTemplateName:   "metrictemplate",
					LogScopeVariables:    "job_name,pod_name",
					BaselineLogScope:     "podHashBaseline",
					CanaryLogScope:       "podHashCanary",
					LogTemplateName:      "logtemplate",
				},
			}}
		_, err := opsmxMetric.process(rpcPluginImp, opsmxProfileData, "ns")
		assert.Contains(t, err.Error(), "mismatch in number of log scope variables and baseline/canary log scope of service")
	})

	t.Run("missing canary/baseline for log analysis of service - an error should be raised", func(t *testing.T) {
		opsmxMetric := OPSMXMetric{Application: "newapp",
			LifetimeMinutes: 9,
			Pass:            90,
			Marginal:        85,
			Services: []OPSMXService{
				{
					MetricScopeVariables: "pod_name",
					BaselineMetricScope:  "podHashBaseline",
					CanaryMetricScope:    "podHashCanary",
					MetricTemplateName:   "metrictemplate",
					LogScopeVariables:    "pod_name",
					BaselineLogScope:     "podHashBaseline",
					LogTemplateName:      "logtemplate",
				},
			}}
		_, err := opsmxMetric.process(rpcPluginImp, opsmxProfileData, "ns")
		assert.Contains(t, err.Error(), "missing canary/baseline for log analysis of service")
	})

	t.Run("missing log template in Service - an error should be raised", func(t *testing.T) {
		opsmxMetric := OPSMXMetric{Application: "newapp",
			LifetimeMinutes: 9,
			Pass:            90,
			Marginal:        85,
			Services: []OPSMXService{
				{
					MetricScopeVariables: "pod_name",
					BaselineMetricScope:  "podHashBaseline",
					CanaryMetricScope:    "podHashCanary",
					MetricTemplateName:   "metrictemplate",
					LogScopeVariables:    "pod_name",
					CanaryLogScope:       "podHashCanary",
					BaselineLogScope:     "podHashBaseline",
				},
			}}
		_, err := opsmxMetric.process(rpcPluginImp, opsmxProfileData, "ns")
		assert.Contains(t, err.Error(), "provide either a service specific log template or global log template for service")
	})

	t.Run("missing log Scope placeholder - an error should be raised", func(t *testing.T) {
		opsmxMetric := OPSMXMetric{Application: "newapp",
			LifetimeMinutes: 9,
			Pass:            90,
			Marginal:        85,
			Services: []OPSMXService{
				{
					MetricScopeVariables: "pod_name",
					BaselineMetricScope:  "podHashBaseline",
					CanaryMetricScope:    "podHashCanary",
					MetricTemplateName:   "metrictemplate",
					CanaryLogScope:       "podHashCanary",
					BaselineLogScope:     "podHashBaseline",
					LogTemplateName:      "logtemplate",
				},
			}}
		_, err := opsmxMetric.process(rpcPluginImp, opsmxProfileData, "ns")
		assert.Contains(t, err.Error(), "missing log Scope placeholder")
	})

	t.Run("mismatch in metric scope variables and baseline/canary log scope - an error should be raised", func(t *testing.T) {
		opsmxMetric := OPSMXMetric{Application: "newapp",
			LifetimeMinutes: 9,
			Pass:            90,
			Marginal:        85,
			Services: []OPSMXService{
				{
					MetricScopeVariables: "job_name,pod_name",
					BaselineMetricScope:  "podHashBaseline",
					CanaryMetricScope:    "podHashCanary",
					MetricTemplateName:   "metrictemplate",
				},
				{
					MetricScopeVariables: "pod_name",
					BaselineMetricScope:  "podHashBaseline",
					CanaryMetricScope:    "podHashCanary",
					MetricTemplateName:   "metrictemplate",
					LogScopeVariables:    "pod_name",
					BaselineLogScope:     "podHashBaseline",
					CanaryLogScope:       "podHashCanary",
					LogTemplateName:      "logtemplate",
				},
			}}
		_, err := opsmxMetric.process(rpcPluginImp, opsmxProfileData, "ns")
		assert.Contains(t, err.Error(), "mismatch in number of metric scope variables and baseline/canary metric scope of service")
	})

	t.Run("missing canary for metric analysis of service - an error should be raised", func(t *testing.T) {
		opsmxMetric := OPSMXMetric{Application: "newapp",
			LifetimeMinutes: 9,
			Pass:            90,
			Marginal:        85,
			Services: []OPSMXService{
				{
					MetricScopeVariables: "pod_name",
					BaselineMetricScope:  "podHashBaseline",
					MetricTemplateName:   "metrictemplate",
					LogScopeVariables:    "pod_name",
					BaselineLogScope:     "podHashBaseline",
					CanaryLogScope:       "podHashCanary",
					LogTemplateName:      "logtemplate",
				},
			}}
		_, err := opsmxMetric.process(rpcPluginImp, opsmxProfileData, "ns")
		assert.Contains(t, err.Error(), "missing canary/baseline for metric analysis of service")
	})

	t.Run("missing baseline for metric analysis of service - an error should be raised", func(t *testing.T) {
		opsmxMetric := OPSMXMetric{Application: "newapp",
			LifetimeMinutes: 9,
			Pass:            90,
			Marginal:        85,
			Services: []OPSMXService{
				{
					MetricScopeVariables: "pod_name",
					CanaryMetricScope:    "podHashCanary",
					MetricTemplateName:   "metrictemplate",
					LogScopeVariables:    "pod_name",
					BaselineLogScope:     "podHashBaseline",
					CanaryLogScope:       "podHashCanary",
					LogTemplateName:      "logtemplate",
				},
			}}
		_, err := opsmxMetric.process(rpcPluginImp, opsmxProfileData, "ns")
		assert.Contains(t, err.Error(), "missing canary/baseline for metric analysis of service")
	})

	t.Run("missing metric template in Service - an error should be raised", func(t *testing.T) {
		opsmxMetric := OPSMXMetric{Application: "newapp",
			LifetimeMinutes: 9,
			Pass:            90,
			Marginal:        85,
			Services: []OPSMXService{
				{
					MetricScopeVariables: "pod_name",
					BaselineMetricScope:  "podHashBaseline",
					CanaryMetricScope:    "podHashCanary",
				},
			}}
		_, err := opsmxMetric.process(rpcPluginImp, opsmxProfileData, "ns")
		assert.Contains(t, err.Error(), "provide either a service specific metric template or global metric template for service")
	})

	t.Run("missing log Scope placeholder - an error should be raised", func(t *testing.T) {
		opsmxMetric := OPSMXMetric{Application: "newapp",
			LifetimeMinutes: 9,
			Pass:            90,
			Marginal:        85,
			Services: []OPSMXService{
				{
					BaselineMetricScope: "podHashBaseline",
					CanaryMetricScope:   "podHashCanary",
					MetricTemplateName:  "metrictemplate",
					LogScopeVariables:   "pod_name",
					CanaryLogScope:      "podHashCanary",
					BaselineLogScope:    "podHashBaseline",
					LogTemplateName:     "logtemplate",
				},
			}}
		_, err := opsmxMetric.process(rpcPluginImp, opsmxProfileData, "ns")
		assert.Contains(t, err.Error(), "missing metric Scope placeholder")
	})

	t.Run("serviceName exists more than once - an error should be raised", func(t *testing.T) {
		opsmxMetric := OPSMXMetric{Application: "newapp",
			LifetimeMinutes: 9,
			Pass:            90,
			Marginal:        85,
			Services: []OPSMXService{
				{
					LogScopeVariables: "pod_name",
					CanaryLogScope:    "podHashCanary",
					BaselineLogScope:  "podHashBaseline",
					LogTemplateName:   "logtemplate",
					ServiceName:       "serviceName",
				},
				{
					MetricScopeVariables: "pod_name",
					BaselineMetricScope:  "podHashBaseline",
					CanaryMetricScope:    "podHashCanary",
					MetricTemplateName:   "metrictemplate",
					ServiceName:          "serviceName",
				},
			}}
		_, err := opsmxMetric.process(rpcPluginImp, opsmxProfileData, "ns")
		assert.Contains(t, err.Error(), "serviceName 'serviceName' mentioned exists more than once")
	})

	t.Run("gitops flow for the template, configmap not present- configmap not found error", func(t *testing.T) {
		opsmxMetric := OPSMXMetric{
			Application:     "newapp",
			LifetimeMinutes: 9,
			Pass:            90,
			Marginal:        85,
			GitOPS:          true,
			Services: []OPSMXService{{
				LogTemplateName:   "logtemp",
				LogScopeVariables: "pod_name",
				CanaryLogScope:    "podHashCanary",
				BaselineLogScope:  "podHashBaseline",
			}},
		}
		_, err := opsmxMetric.process(rpcPluginImp, opsmxProfileData, "ns")
		assert.Contains(t, err.Error(), "template config map validation error")
	})

}

func TestOpsmxMetricVariousFlows(t *testing.T) {
	logCtx := *log.WithFields(log.Fields{"plugin-test": "opsmx"})
	rpcPluginImp := &RpcPlugin{
		LogCtx:        logCtx,
		kubeclientset: k8sfake.NewSimpleClientset(),
		client:        NewHttpClient(),
	}
	opsmxProfileData := opsmxProfile{cdIntegration: "argocd",
		user:        "admin",
		sourceName:  "sourceName",
		opsmxIsdUrl: "https://opsmx.test.tst"}

	t.Run("basic no gitops single service - no error should be raised", func(t *testing.T) {
		opsmxMetric := OPSMXMetric{Application: "newapp",
			BaselineStartTime: "2022-08-10T13:15:00Z",
			CanaryStartTime:   "2022-08-10T13:15:00Z",
			EndTime:           "2022-08-10T13:45:00Z",
			IntervalTime:      3,
			Delay:             1,
			Pass:              90,
			Marginal:          85,
			LookBackType:      "growing",
			Services: []OPSMXService{{
				MetricScopeVariables:  "pod_name",
				BaselineMetricScope:   "podHashBaseline",
				CanaryMetricScope:     "podHashCanary",
				MetricTemplateName:    "metrictemplate",
				MetricTemplateVersion: "v1.0",
			}},
		}
		expectedPayload := `{"application":"newapp","sourceName":"sourceName","sourceType":"argocd","canaryConfig":{"lifetimeMinutes":"30","lookBackType":"growing","interval":"3","delay":"1","canaryHealthCheckHandler":{"minimumCanaryResultScore":"85"},"canarySuccessCriteria":{"canaryResultScore":"90"}},"canaryDeployments":[{"canaryStartTimeMs":"1660137300000","baselineStartTimeMs":"1660137300000","canary":{"metric":{"service1":{"serviceGate":"gate1","pod_name":"podHashCanary","template":"metrictemplate","templateVersion":"v1.0"}}},"baseline":{"metric":{"service1":{"serviceGate":"gate1","pod_name":"podHashBaseline","template":"metrictemplate","templateVersion":"v1.0"}}}}]}`
		payload, err := opsmxMetric.process(rpcPluginImp, opsmxProfileData, "ns")
		expectedBodyI, bodyI := getPayload(expectedPayload, payload)
		assert.Equal(t, expectedBodyI, bodyI)
		assert.Nil(t, err)
	})

	t.Run("basic no gitops multiservice - no error should be raised", func(t *testing.T) {
		opsmxMetric := OPSMXMetric{Application: "newapp",
			BaselineStartTime: "2022-08-10T13:15:00Z",
			CanaryStartTime:   "2022-08-10T13:15:00Z",
			EndTime:           "2022-08-10T13:45:00Z",
			Pass:              90,
			Marginal:          85,
			Services: []OPSMXService{
				{
					ServiceName:          "service1",
					MetricScopeVariables: "job_name",
					BaselineMetricScope:  "oes-platform-br",
					CanaryMetricScope:    "oes-platform-cr",
					MetricTemplateName:   "metricTemplate",
				},
				{
					MetricScopeVariables: "job_name",
					BaselineMetricScope:  "oes-sapor-br",
					CanaryMetricScope:    "oes-sapor-cr",
					MetricTemplateName:   "metricTemplate",
					LogScopeVariables:    "kubernetes.container_name",
					BaselineLogScope:     "oes-datascience-br",
					CanaryLogScope:       "oes-datascience-cr",
					LogTemplateName:      "logTemplate",
				},
			},
		}
		expectedPayload := `{"application":"newapp","sourceName":"sourceName","sourceType":"argocd","canaryConfig":{"lifetimeMinutes":"30","canaryHealthCheckHandler":{"minimumCanaryResultScore":"85"},"canarySuccessCriteria":{"canaryResultScore":"90"}},"canaryDeployments":[{"canaryStartTimeMs":"1660137300000","baselineStartTimeMs":"1660137300000","canary":{"log":{"service2":{"serviceGate":"gate2","kubernetes.container_name":"oes-datascience-cr","template":"logTemplate"}},"metric":{"service1":{"serviceGate":"gate1","job_name":"oes-platform-cr","template":"metricTemplate"},"service2":{"serviceGate":"gate2","job_name":"oes-sapor-cr","template":"metricTemplate"}}},"baseline":{"log":{"service2":{"serviceGate":"gate2","kubernetes.container_name":"oes-datascience-br","template":"logTemplate"}},"metric":{"service1":{"serviceGate":"gate1","job_name":"oes-platform-br","template":"metricTemplate"},"service2":{"serviceGate":"gate2","job_name":"oes-sapor-br","template":"metricTemplate"}}}}]}`
		payload, err := opsmxMetric.process(rpcPluginImp, opsmxProfileData, "ns")
		expectedBodyI, bodyI := getPayload(expectedPayload, payload)
		assert.Equal(t, expectedBodyI, bodyI)
		assert.Nil(t, err)
	})

	t.Run("basic gitops flow for the yaml metric template - no error should be raised", func(t *testing.T) {
		opsmxMetric := OPSMXMetric{
			Application:     "newapp",
			LifetimeMinutes: 9,
			Pass:            90,
			Marginal:        85,
			GitOPS:          true,
			Services: []OPSMXService{{
				MetricScopeVariables: "pod_name",
				BaselineMetricScope:  "podHashBaseline",
				CanaryMetricScope:    "podHashCanary",
				MetricTemplateName:   "metrictemplate",
			}},
		}
		c := NewTestClient(func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, "https://opsmx.test.tst/autopilot/api/v5/external/template?sha1=a5b311c084cebce5b2e40b388e2e11c6e397c970&templateName=metrictemplate&templateType=METRIC", req.URL.String())
			return &http.Response{
				StatusCode: 200,
				Body: io.NopCloser(bytes.NewBufferString(`
				true
				`)),
				Header: make(http.Header),
			}, nil
		})

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

		cmMetric := map[string]string{"metrictemplate": metricsData}
		rpcPluginImp.kubeclientset = getFakeClientForCM(cmMetric)
		rpcPluginImp.client = c
		_, err := opsmxMetric.process(rpcPluginImp, opsmxProfileData, "ns")
		assert.Nil(t, err)
	})

	t.Run("basic gitops flow for the yaml metric template when the template gets created- no error should be raised", func(t *testing.T) {
		opsmxMetric := OPSMXMetric{
			Application:     "newapp",
			LifetimeMinutes: 9,
			Pass:            90,
			Marginal:        85,
			GitOPS:          true,
			Services: []OPSMXService{{
				MetricScopeVariables: "pod_name",
				BaselineMetricScope:  "podHashBaseline",
				CanaryMetricScope:    "podHashCanary",
				MetricTemplateName:   "metrictemplate",
			}},
		}
		c := NewTestClient(func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, "https://opsmx.test.tst/autopilot/api/v5/external/template?sha1=a5b311c084cebce5b2e40b388e2e11c6e397c970&templateName=metrictemplate&templateType=METRIC", req.URL.String())
			if req.Method == "GET" {
				return &http.Response{
					StatusCode: 200,
					Body: io.NopCloser(bytes.NewBufferString(`
				false
				`)),
					Header: make(http.Header),
				}, nil
			}
			return &http.Response{
				StatusCode: 200,
				Body: io.NopCloser(bytes.NewBufferString(`
			{
				"status" :"CREATED"
			}
			`)),
				Header: make(http.Header),
			}, nil

		})

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

		cmMetric := map[string]string{"metrictemplate": metricsData}

		rpcPluginImp.kubeclientset = getFakeClientForCM(cmMetric)
		rpcPluginImp.client = c
		_, err := opsmxMetric.process(rpcPluginImp, opsmxProfileData, "ns")
		assert.Nil(t, err)
	})

	t.Run("basic gitops flow for the yaml metric template when the template creation is unsucessful- an error should be raised", func(t *testing.T) {
		opsmxMetric := OPSMXMetric{
			Application:     "newapp",
			LifetimeMinutes: 9,
			Pass:            90,
			Marginal:        85,
			GitOPS:          true,
			Services: []OPSMXService{{
				MetricScopeVariables: "pod_name",
				BaselineMetricScope:  "podHashBaseline",
				CanaryMetricScope:    "podHashCanary",
				MetricTemplateName:   "metrictemplate",
			}},
		}
		c := NewTestClient(func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, "https://opsmx.test.tst/autopilot/api/v5/external/template?sha1=a5b311c084cebce5b2e40b388e2e11c6e397c970&templateName=metrictemplate&templateType=METRIC", req.URL.String())
			if req.Method == "GET" {
				return &http.Response{
					StatusCode: 200,
					Body: io.NopCloser(bytes.NewBufferString(`
				false
				`)),
					Header: make(http.Header),
				}, nil
			}
			return &http.Response{
				StatusCode: 200,
				Body: io.NopCloser(bytes.NewBufferString(`
			{
				"errorMessage" :"something went wrong"
			}
			`)),
				Header: make(http.Header),
			}, nil

		})
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

		cmMetric := map[string]string{"metrictemplate": metricsData}

		rpcPluginImp.kubeclientset = getFakeClientForCM(cmMetric)
		rpcPluginImp.client = c
		_, err := opsmxMetric.process(rpcPluginImp, opsmxProfileData, "ns")
		assert.Contains(t, err.Error(), "something went wrong")
	})

	t.Run("gitops flow for the json metric template when the there is a mismatch in the templateName - an error should be raised", func(t *testing.T) {
		opsmxMetric := OPSMXMetric{
			Application:     "newapp",
			LifetimeMinutes: 9,
			Pass:            90,
			Marginal:        85,
			GitOPS:          true,
			Services: []OPSMXService{{
				MetricScopeVariables: "pod_name",
				BaselineMetricScope:  "podHashBaseline",
				CanaryMetricScope:    "podHashCanary",
				MetricTemplateName:   "metrictemplate",
			}},
		}
		metricsData := `
		{"filterKey":"pod_name","accountName":"newacc","data":{"isNormalize":false,"groups":[{"metrics":[{"metricType":"ADVANCED","metricWeight":1,"nanStrategy":"ReplaceWithZero","riskDirection":"Lower","name":"avg(rate(nginx_ingress_controller_ingress_upstream_latency_seconds{namespace=\"${namespace_key}\", service= \"${service}\",ingress = \"${ingress}\", quantile =\"0.9\"}[5m]))","criticality":"LOW","watchlist":false}],"group":"Upstream Service Latency Per Ingress - 90th Percentile"}]},"templateName":"template"}`
		cmMetric := map[string]string{"metrictemplate": metricsData}
		rpcPluginImp.kubeclientset = getFakeClientForCM(cmMetric)
		_, err := opsmxMetric.process(rpcPluginImp, opsmxProfileData, "ns")
		assert.Contains(t, err.Error(), "Mismatch between templateName and data.metrictemplate key")
	})

	t.Run("gitops flow for the json log template when the data element is missing- an error should be raised", func(t *testing.T) {
		opsmxMetric := OPSMXMetric{
			Application:     "newapp",
			LifetimeMinutes: 9,
			Pass:            90,
			Marginal:        85,
			GitOPS:          true,
			Services: []OPSMXService{{
				LogScopeVariables: "pod_name",
				BaselineLogScope:  "podHashBaseline",
				CanaryLogScope:    "podHashCanary",
				LogTemplateName:   "logtemplate",
			}},
		}

		logData := `
		{"filterKey":"${namespace_key}","tagEnabled":true,"monitoringProvider":"ELASTICSEARCH","accountName":"ds-elastic","scoringAlgorithm":"Canary","index":"kubernetes*","responseKeywords":"log,message","tags":[{"string":"NonOutOfMemoryError","tag":"tag1"}],"errorTopics":[]}`
		cmMetric := map[string]string{"template": logData}
		rpcPluginImp.kubeclientset = getFakeClientForCM(cmMetric)
		_, err := opsmxMetric.process(rpcPluginImp, opsmxProfileData, "ns")
		assert.Contains(t, err.Error(), "missing data element logtemplate")
	})

	t.Run("gitops flow template name not provided inside json- an error should be raised", func(t *testing.T) {
		opsmxMetric := OPSMXMetric{
			Application:     "newapp",
			LifetimeMinutes: 9,
			Pass:            90,
			Marginal:        85,
			GitOPS:          true,
			Services: []OPSMXService{{
				LogScopeVariables: "pod_name",
				BaselineLogScope:  "podHashBaseline",
				CanaryLogScope:    "podHashCanary",
				LogTemplateName:   "logtemplate",
			}},
		}

		logData := `
		{"filterKey":"${namespace_key}","tagEnabled":true,"monitoringProvider":"ELASTICSEARCH","accountName":"ds-elastic","scoringAlgorithm":"Canary","index":"kubernetes*","responseKeywords":"log,message","tags":[{"string":"NonOutOfMemoryError","tag":"tag1"}],"errorTopics":[]}`
		cmMetric := map[string]string{"logtemplate": logData}
		rpcPluginImp.kubeclientset = getFakeClientForCM(cmMetric)
		_, err := opsmxMetric.process(rpcPluginImp, opsmxProfileData, "ns")
		assert.Contains(t, err.Error(), "template name not provided inside json")
	})

	t.Run("gitops flow with log yaml - no error is raised", func(t *testing.T) {
		opsmxMetric := OPSMXMetric{
			Application:     "newapp",
			LifetimeMinutes: 9,
			Pass:            90,
			Marginal:        85,
			GitOPS:          true,
			Services: []OPSMXService{{
				LogScopeVariables: "pod_name",
				BaselineLogScope:  "podHashBaseline",
				CanaryLogScope:    "podHashCanary",
				LogTemplateName:   "logtemplate",
			}},
		}
		c := NewTestClient(func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, "https://opsmx.test.tst/autopilot/api/v5/external/template?sha1=fb14b5dbf9c619c54ad001fcc757e6f2aae19503&templateName=logtemplate&templateType=LOG", req.URL.String())
			if req.Method == "GET" {
				return &http.Response{
					StatusCode: 200,
					Body: io.NopCloser(bytes.NewBufferString(`
				false
				`)),
					Header: make(http.Header),
				}, nil
			}
			return &http.Response{
				StatusCode: 200,
				Body: io.NopCloser(bytes.NewBufferString(`
			{
				"status" :"CREATED"
			}
			`)),
				Header: make(http.Header),
			}, nil

		})
		logData := `
        monitoringProvider: ELASTICSEARCH
        accountName: ds-elastic
        scoringAlgorithm: Canary
        index: kubernetes*
        responseKeywords: log,message
        disableDefaultErrorTopics: true
        tags:
        - errorString: NonOutOfMemoryError
          tag: tag1`

		cmMetric := map[string]string{"logtemplate": logData}
		rpcPluginImp.client = c
		rpcPluginImp.kubeclientset = getFakeClientForCM(cmMetric)
		_, err := opsmxMetric.process(rpcPluginImp, opsmxProfileData, "ns")
		assert.Nil(t, err)
	})

}

func getFakeClientForCM(data map[string]string) *k8sfake.Clientset {
	opsmxSecret := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: defaultSecretName,
		},
		Data: data,
	}
	fakeClient := k8sfake.NewSimpleClientset()
	fakeClient.PrependReactor("get", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, opsmxSecret, nil
	})
	return fakeClient
}

func getPayload(expectedPayload string, payload string) (map[string]interface{}, map[string]interface{}) {
	bodyI := map[string]interface{}{}
	err := json.Unmarshal([]byte(payload), &bodyI)
	if err != nil {
		panic(err)
	}
	expectedBodyI := map[string]interface{}{}
	err = json.Unmarshal([]byte(expectedPayload), &expectedBodyI)
	if err != nil {
		panic(err)
	}
	return expectedBodyI, bodyI
}

// RoundTripFunc .
type RoundTripFunc func(req *http.Request) (*http.Response, error)

// RoundTrip .
func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// NewTestClient returns *http.Client with Transport replaced to avoid making real calls
func NewTestClient(fn RoundTripFunc) http.Client {
	return http.Client{
		Transport: fn,
	}
}
