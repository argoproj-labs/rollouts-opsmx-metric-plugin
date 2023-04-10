/*
 * Copyright 2023 OpsMx, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License")
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package plugin

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	kubetesting "k8s.io/client-go/testing"
)

func TestRun(t *testing.T) {
	t.Run("basic flow for Run - no error is raised", func(t *testing.T) {
		logCtx := *log.WithFields(log.Fields{"plugin-test": "opsmx"})
		rpcPluginImp := &RpcPlugin{
			LogCtx: logCtx,
		}

		secretData := map[string][]byte{
			"cdIntegration": []byte("true"),
			"opsmxIsdUrl":   []byte("https://opsmx.secret.tst"),
			"user":          []byte("admin"),
			"agentName":     []byte("agent123"),
			"sourceName":    []byte("sourcename"),
		}
		rpcPluginImp.kubeclientset = getFakeClient(secretData)

		c := NewTestClient(func(req *http.Request) (*http.Response, error) {
			if req.Method == "GET" {
				return &http.Response{
					StatusCode: 200,
					Header:     make(http.Header),
				}, nil
			}
			return &http.Response{
				StatusCode: 200,
				Body: io.NopCloser(bytes.NewBufferString(`
				{
					"canaryId": 53
				}
			`)),
				Header: make(http.Header),
			}, nil

		})
		rpcPluginImp.client = c

		metric := v1alpha1.Metric{
			Name: "testapp",
			Provider: v1alpha1.MetricProvider{
				Plugin: map[string]json.RawMessage{opsmxPlugin: json.RawMessage([]byte(`{"application":"newapp","lifetimeMinutes":9,"passScore":90,"marginalScore":85,"serviceList":[{"logTemplateName":"logtemp","logScopeVariables":"pod_name","baselineLogScope":"podHashBaseline","canaryLogScope":"podHashCanary"}]}`))}},
		}

		measurement := rpcPluginImp.Run(newAnalysisRun(), metric)
		assert.NotNil(t, measurement.StartedAt)
		assert.Nil(t, measurement.FinishedAt)
		assert.Equal(t, "53", measurement.Metadata["canaryId"])
		assert.Equal(t, v1alpha1.AnalysisPhaseRunning, measurement.Phase)
	})
}

func TestResume(t *testing.T) {

	logCtx := *log.WithFields(log.Fields{"plugin-test": "opsmx"})
	rpcPluginImp := &RpcPlugin{
		LogCtx: logCtx,
	}
	secretData := map[string][]byte{
		"cdIntegration": []byte("true"),
		"opsmxIsdUrl":   []byte("https://opsmx.secret.tst"),
		"user":          []byte("admin"),
		"agentName":     []byte("agent123"),
		"sourceName":    []byte("sourcename"),
	}
	rpcPluginImp.kubeclientset = getFakeClient(secretData)

	metric := v1alpha1.Metric{
		Name: "testapp",
		Provider: v1alpha1.MetricProvider{
			Plugin: map[string]json.RawMessage{opsmxPlugin: json.RawMessage([]byte(`{"application":"newapp","lifetimeMinutes":9,"passScore":90,"marginalScore":85,"serviceList":[{"logTemplateName":"logtemp","logScopeVariables":"pod_name","baselineLogScope":"podHashBaseline","canaryLogScope":"podHashCanary"}]}`))}},
	}

	mapMetadata := make(map[string]string)
	mapMetadata["canaryId"] = "53"
	measurement := v1alpha1.Measurement{
		Metadata: mapMetadata,
		Phase:    v1alpha1.AnalysisPhaseRunning,
	}

	t.Run("basic flow for Resume with the status as running - no error is raised", func(t *testing.T) {
		c := NewTestClient(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body: io.NopCloser(bytes.NewBufferString(`
				{
					"owner": "admin",
					"application": "test-plugin",
					"canaryResult": {
						"duration": "9 seconds",
						"lastUpdated": "2023-02-16 08:53:33.182",
						"canaryReportURL": "https://opsmx.secret.tst/ui/application/deploymentverification/newapp/53"
					},
					"launchedDate": "2023-02-16 08:53:23.439",
					"canaryConfig": {
						"combinedCanaryResultStrategy": "LOWEST",
						"minimumCanaryResultScore": 0.0,
						"name": "admin",
						"lifetimeMinutes": 3,
						"canaryAnalysisIntervalMins": 3,
						"maximumCanaryResultScore": 90.0
					},
					"id": "53",
					"status": {
						"complete": false,
						"status": "RUNNING"
					}
				}
				`)),
				Header: make(http.Header),
			}, nil
		})
		rpcPluginImp.client = c

		measurement = rpcPluginImp.Resume(newAnalysisRun(), metric, measurement)
		assert.Nil(t, measurement.FinishedAt)
		assert.Equal(t, "53", measurement.Metadata["canaryId"])
		assert.Equal(t, "https://opsmx.secret.tst/ui/application/deploymentverification/newapp/53", measurement.Metadata["reportUrl"])
		assert.Equal(t, v1alpha1.AnalysisPhaseRunning, measurement.Phase)
	})
	t.Run("basic flow for Resume with the status as inconclusive - no error is raised", func(t *testing.T) {
		c := NewTestClient(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body: io.NopCloser(bytes.NewBufferString(`
				{
					"owner": "admin",
					"application": "newapp",
					"canaryResult": {
						"canaryReportURL": "https://opsmx.secret.tst/ui/application/deploymentverification/newapp/53",
						"overallScore": 85,
						"overallResult": "HEALTHY",
						"message": "Canary Is HEALTHY",
						"errors": []
					},
					"id": "53",
					"status": {
						"complete": true,
						"status": "COMPLETED"
					}}
				`)),
				Header: make(http.Header),
			}, nil
		})
		rpcPluginImp.client = c

		measurement = rpcPluginImp.Resume(newAnalysisRun(), metric, measurement)
		assert.NotNil(t, measurement.FinishedAt)
		assert.Equal(t, "53", measurement.Metadata["canaryId"])
		assert.Equal(t, "https://opsmx.secret.tst/ui/application/deploymentverification/newapp/53", measurement.Metadata["reportUrl"])
		assert.Equal(t, v1alpha1.AnalysisPhaseInconclusive, measurement.Phase)
	})

	t.Run("basic flow for Resume with the status as cancelled - no error is raised", func(t *testing.T) {
		c := NewTestClient(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body: io.NopCloser(bytes.NewBufferString(`
				{
					"owner": "admin",
					"application": "newapp",
					"canaryResult": {
						"canaryReportURL": "https://opsmx.secret.tst/ui/application/deploymentverification/newapp/53",
						"overallScore": 85,
						"overallResult": "HEALTHY",
						"message": "Canary Is HEALTHY",
						"errors": []
					},
					"id": "53",
					"status": {
						"complete": false,
						"status": "CANCELLED"
					}}
				`)),
				Header: make(http.Header),
			}, nil
		})
		rpcPluginImp.client = c

		measurement = rpcPluginImp.Resume(newAnalysisRun(), metric, measurement)
		assert.NotNil(t, measurement.FinishedAt)
		assert.Equal(t, "53", measurement.Metadata["canaryId"])
		assert.Equal(t, "https://opsmx.secret.tst/ui/application/deploymentverification/newapp/53", measurement.Metadata["reportUrl"])
		assert.Equal(t, v1alpha1.AnalysisPhaseFailed, measurement.Phase)
	})

	t.Run("basic flow for Resume with the status as failed - no error is raised", func(t *testing.T) {
		c := NewTestClient(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body: io.NopCloser(bytes.NewBufferString(`
				{
					"owner": "admin",
					"application": "newapp",
					"canaryResult": {
						"canaryReportURL": "https://opsmx.secret.tst/ui/application/deploymentverification/newapp/53",
						"overallScore": 0,
						"overallResult": "HEALTHY",
						"message": "Canary Is HEALTHY",
						"errors": []
					},
					"id": "53",
					"status": {
						"complete": true,
						"status": "COMPLETED"
					}}
				`)),
				Header: make(http.Header),
			}, nil
		})
		rpcPluginImp.client = c

		measurement = rpcPluginImp.Resume(newAnalysisRun(), metric, measurement)
		assert.NotNil(t, measurement.FinishedAt)
		assert.Equal(t, "53", measurement.Metadata["canaryId"])
		assert.Equal(t, "https://opsmx.secret.tst/ui/application/deploymentverification/newapp/53", measurement.Metadata["reportUrl"])
		assert.Equal(t, v1alpha1.AnalysisPhaseFailed, measurement.Phase)
	})
}

func TestOpsmxProfile(t *testing.T) {

	logCtx := *log.WithFields(log.Fields{"plugin-test": "opsmx"})

	rpcPluginImp := &RpcPlugin{
		LogCtx: logCtx,
		client: NewHttpClient(),
	}

	opsmxMetric := OPSMXMetric{Application: "newapp",
		Profile:         "opsmx-profile-test",
		LifetimeMinutes: 9,
		Pass:            90,
		Marginal:        85,
		Services: []OPSMXService{{LogTemplateName: "logtemp",
			LogScopeVariables: "pod_name",
			CanaryLogScope:    "podHashCanary",
			BaselineLogScope:  "podHashBaseline",
		}},
	}
	t.Run("cdIntegration is missing in the secret - an error should be raised", func(t *testing.T) {
		secretData := map[string][]byte{
			"opsmxIsdUrl": []byte("https://opsmx.secret.tst"),
			"sourceName":  []byte("sourcename"),
			"user":        []byte("admin"),
			"agentName":   []byte("agent123"),
		}
		rpcPluginImp.kubeclientset = getFakeClient(secretData)
		_, err := getOpsmxProfile(rpcPluginImp, opsmxMetric, "ns")
		assert.Contains(t, err.Error(), "`cdIntegration` key not present in the secret file")
	})

	t.Run("sourceName is missing in the secret - an error should be raised", func(t *testing.T) {
		secretData := map[string][]byte{
			"cdIntegration": []byte("true"),
			"opsmxIsdUrl":   []byte("https://opsmx.secret.tst"),
			"user":          []byte("admin"),
			"agentName":     []byte("agent123"),
		}
		rpcPluginImp.kubeclientset = getFakeClient(secretData)
		_, err := getOpsmxProfile(rpcPluginImp, opsmxMetric, "ns")
		assert.Contains(t, err.Error(), "`sourceName` key not present in the secret file")
	})

	t.Run("opsmxIsdUrl is missing in the secret - an error should be raised", func(t *testing.T) {
		secretData := map[string][]byte{
			"cdIntegration": []byte("true"),
			"sourceName":    []byte("sourcename"),
			"user":          []byte("admin"),
			"agentName":     []byte("agent123"),
		}
		rpcPluginImp.kubeclientset = getFakeClient(secretData)
		_, err := getOpsmxProfile(rpcPluginImp, opsmxMetric, "ns")
		assert.Contains(t, err.Error(), "`opsmxIsdUrl` key not present in the secret file")
	})

	t.Run("opsmxIsdUrl is missing in the secret - an error should be raised", func(t *testing.T) {
		secretData := map[string][]byte{
			"cdIntegration": []byte("true"),
			"sourceName":    []byte("sourcename"),
			"user":          []byte("admin"),
			"agentName":     []byte("agent123"),
		}
		rpcPluginImp.kubeclientset = getFakeClient(secretData)
		_, err := getOpsmxProfile(rpcPluginImp, opsmxMetric, "ns")
		assert.Contains(t, err.Error(), "`opsmxIsdUrl` key not present in the secret file")
	})

	t.Run("user is missing in the secret - an error should be raised", func(t *testing.T) {
		secretData := map[string][]byte{
			"cdIntegration": []byte("true"),
			"opsmxIsdUrl":   []byte("https://opsmx.secret.tst"),
		}
		rpcPluginImp.kubeclientset = getFakeClient(secretData)
		_, err := getOpsmxProfile(rpcPluginImp, opsmxMetric, "ns")
		assert.Contains(t, err.Error(), "`user` key not present in the secret file")
	})
	t.Run("cdIntegration is neither true nor false in the secret - an error should be raised", func(t *testing.T) {
		secretData := map[string][]byte{
			"cdIntegration": []byte("test"),
			"opsmxIsdUrl":   []byte("https://opsmx.secret.tst"),
			"user":          []byte("admin"),
			"sourceName":    []byte("sourcename"),
		}
		rpcPluginImp.kubeclientset = getFakeClient(secretData)
		_, err := getOpsmxProfile(rpcPluginImp, opsmxMetric, "ns")
		assert.Contains(t, err.Error(), "`cdIntegration` should be either true or false")
	})

	t.Run("agentName is not present in the secret - an error should be raised", func(t *testing.T) {
		secretData := map[string][]byte{
			"cdIntegration": []byte("true"),
			"opsmxIsdUrl":   []byte("https://opsmx.secret.tst"),
			"user":          []byte("admin"),
			"sourceName":    []byte("sourcename"),
		}
		rpcPluginImp.kubeclientset = getFakeClient(secretData)
		_, err := getOpsmxProfile(rpcPluginImp, opsmxMetric, "ns")
		assert.Contains(t, err.Error(), "`agentName` key not present in the secret file")
	})

	t.Run("cdIntegration is false in the secret and agentName is not present- no error is raised", func(t *testing.T) {
		secretData := map[string][]byte{
			"cdIntegration": []byte("false"),
			"opsmxIsdUrl":   []byte("https://opsmx.secret.tst"),
			"user":          []byte("admin"),
			"sourceName":    []byte("sourcename"),
		}
		rpcPluginImp.kubeclientset = getFakeClient(secretData)
		_, err := getOpsmxProfile(rpcPluginImp, opsmxMetric, "ns")
		assert.Nil(t, err)
	})

	t.Run("basic flow - no error is raised", func(t *testing.T) {
		secretData := map[string][]byte{
			"cdIntegration": []byte("true"),
			"opsmxIsdUrl":   []byte("https://opsmx.secret.tst"),
			"user":          []byte("admin"),
			"sourceName":    []byte("sourcename"),
			"agentName":     []byte("agent123"),
		}
		rpcPluginImp.kubeclientset = getFakeClient(secretData)
		_, err := getOpsmxProfile(rpcPluginImp, opsmxMetric, "ns")
		assert.Nil(t, err)
	})

	t.Run("when user and url are also defined in the metric - values from the metric get picked up", func(t *testing.T) {
		secretData := map[string][]byte{
			"cdIntegration": []byte("true"),
			"opsmxIsdUrl":   []byte("https://opsmx.secret.tst"),
			"user":          []byte("admin"),
			"sourceName":    []byte("sourcename"),
			"agentName":     []byte("agent123"),
		}
		rpcPluginImp.kubeclientset = getFakeClient(secretData)

		opsmxMetric := OPSMXMetric{Application: "newapp",
			OpsmxIsdUrl:     "https://url.from.metric",
			User:            "userFromMetric",
			Profile:         "opsmx-profile-test",
			LifetimeMinutes: 9,
			Pass:            90,
			Marginal:        85,
			Services: []OPSMXService{{LogTemplateName: "logtemp",
				LogScopeVariables: "pod_name",
				CanaryLogScope:    "podHashCanary",
				BaselineLogScope:  "podHashBaseline",
			}},
		}
		opsmxProfileData, err := getOpsmxProfile(rpcPluginImp, opsmxMetric, "ns")
		assert.Nil(t, err)
		assert.Equal(t, "https://url.from.metric", opsmxProfileData.opsmxIsdUrl)
		assert.Equal(t, "userFromMetric", opsmxProfileData.user)
	})

}

func getFakeClient(data map[string][]byte) *k8sfake.Clientset {
	opsmxSecret := &corev1.Secret{
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

func newAnalysisRun() *v1alpha1.AnalysisRun {
	return &v1alpha1.AnalysisRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "analysisRunTest",
			Namespace:       "default",
			OwnerReferences: []metav1.OwnerReference{{Name: "rolloutTest", Kind: "Rollout"}},
		},
		Spec: v1alpha1.AnalysisRunSpec{
			Metrics: []v1alpha1.Metric{{
				Name: "metricTest",
			}},
		},
		Status: v1alpha1.AnalysisRunStatus{},
	}
}
