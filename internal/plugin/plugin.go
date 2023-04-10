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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/argoproj/argo-rollouts/utils/plugin/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/argoproj/argo-rollouts/metricproviders/plugin"
	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	argoclientset "github.com/argoproj/argo-rollouts/pkg/client/clientset/versioned"
	metricutil "github.com/argoproj/argo-rollouts/utils/metric"
	timeutil "github.com/argoproj/argo-rollouts/utils/time"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	templateApi               = "/autopilot/api/v5/external/template"
	v5configIdLookupURLFormat = `/autopilot/api/v5/registerCanary`
	scoreUrlFormat            = `/autopilot/v5/canaries/`
	resumeAfter               = 3 * time.Second
	defaultTimeout            = 30
	defaultSecretName         = "opsmx-profile"
	cdIntegrationArgoRollouts = "argorollouts"
	cdIntegrationArgoCD       = "argocd"
	templateLog               = "LOG"
	templateMetric            = "METRIC"
	opsmxPlugin               = "argoproj-labs/rollouts-opsmx-metric-plugin"
)

// Here is a real implementation of MetricsPlugin
type RpcPlugin struct {
	LogCtx            log.Entry
	kubeclientset     kubernetes.Interface
	argoProjClientset argoclientset.Interface
	client            http.Client
}

type opsmxProfile struct {
	cdIntegration string
	opsmxIsdUrl   string
	sourceName    string
	user          string
	agentName     string
}

func (g *RpcPlugin) InitPlugin() types.RpcError {
	config, err := rest.InClusterConfig()
	if err != nil {
		return types.RpcError{ErrorString: err.Error()}
	}

	clientsetK8s, err := kubernetes.NewForConfig(config)
	if err != nil {
		return types.RpcError{ErrorString: err.Error()}
	}

	argoclient, err := argoclientset.NewForConfig(config)
	if err != nil {
		return types.RpcError{ErrorString: err.Error()}
	}

	httpclient := NewHttpClient()
	g.client = httpclient
	g.kubeclientset = clientsetK8s
	g.argoProjClientset = argoclient

	return types.RpcError{}
}

func (g *RpcPlugin) Run(analysisRun *v1alpha1.AnalysisRun, metric v1alpha1.Metric) v1alpha1.Measurement {
	startTime := timeutil.MetaNow()
	newMeasurement := v1alpha1.Measurement{
		StartedAt: &startTime,
	}
	OPSMXMetric := OPSMXMetric{}
	if err := json.Unmarshal(metric.Provider.Plugin[opsmxPlugin], &OPSMXMetric); err != nil {
		return metricutil.MarkMeasurementError(newMeasurement, err)
	}
	opsmxProfileData, err := getOpsmxProfile(g, OPSMXMetric, analysisRun.Namespace)
	if err != nil {
		return metricutil.MarkMeasurementError(newMeasurement, err)
	}
	if err := checkISDUrl(g, opsmxProfileData.opsmxIsdUrl); err != nil {
		return metricutil.MarkMeasurementError(newMeasurement, fmt.Errorf("error in processing url %s", opsmxProfileData.opsmxIsdUrl))
	}
	payload, err := OPSMXMetric.process(g, opsmxProfileData, analysisRun)
	if err != nil {
		return metricutil.MarkMeasurementError(newMeasurement, err)
	}
	canaryurl, err := url.JoinPath(opsmxProfileData.opsmxIsdUrl, v5configIdLookupURLFormat)
	if err != nil {
		return metricutil.MarkMeasurementError(newMeasurement, err)
	}

	log.Info(payload)
	log.Info("sending a POST request to registerCanary with the payload")
	data, urlToken, err := makeRequest(g.client, "POST", canaryurl, payload, opsmxProfileData.user)
	if err != nil {
		return metricutil.MarkMeasurementError(newMeasurement, err)
	}
	type canaryResponse struct {
		Error    string      `json:"error,omitempty"`
		Message  string      `json:"message,omitempty"`
		CanaryId json.Number `json:"canaryId,omitempty"`
	}
	var canary canaryResponse

	err = json.Unmarshal(data, &canary)
	if err != nil {
		return metricutil.MarkMeasurementError(newMeasurement, err)
	}
	log.Info("register canary response ", canary)
	if canary.Error != "" {
		errMessage := fmt.Sprintf("analysis Error: %s\nMessage: %s", canary.Error, canary.Message)
		return metricutil.MarkMeasurementError(newMeasurement, errors.New(errMessage))
	}

	mapMetadata := make(map[string]string)
	mapMetadata["canaryId"] = string(canary.CanaryId)
	mapMetadata["reportId"] = urlToken

	resumeTime := metav1.NewTime(timeutil.Now().Add(resumeAfter))
	newMeasurement.Metadata = mapMetadata
	newMeasurement.ResumeAt = &resumeTime
	newMeasurement.Phase = v1alpha1.AnalysisPhaseRunning
	return newMeasurement
}

func (g *RpcPlugin) Resume(analysisRun *v1alpha1.AnalysisRun, metric v1alpha1.Metric, measurement v1alpha1.Measurement) v1alpha1.Measurement {
	OPSMXMetric := OPSMXMetric{}
	if err := json.Unmarshal(metric.Provider.Plugin[opsmxPlugin], &OPSMXMetric); err != nil {
		return metricutil.MarkMeasurementError(measurement, err)
	}
	opsmxProfile, err := getOpsmxProfile(g, OPSMXMetric, analysisRun.Namespace)
	if err != nil {
		return metricutil.MarkMeasurementError(measurement, err)
	}
	scoreURL, err := url.JoinPath(opsmxProfile.opsmxIsdUrl, scoreUrlFormat, measurement.Metadata["canaryId"])
	if err != nil {
		return metricutil.MarkMeasurementError(measurement, err)
	}
	data, _, err := makeRequest(g.client, "GET", scoreURL, "", opsmxProfile.user)
	if err != nil {
		return metricutil.MarkMeasurementError(measurement, err)
	}
	scoreResponseMap, err := processScoreResponse(data)
	if err != nil {
		return metricutil.MarkMeasurementError(measurement, err)
	}

	measurement.Metadata["reportUrl"] = fmt.Sprintf("%s", scoreResponseMap["canaryReportURL"])

	if OPSMXMetric.LookBackType != "" {
		measurement.Metadata["Current intervalNo"] = fmt.Sprintf("%v", scoreResponseMap["intervalNo"])
	}
	//if the status is Running, resume analysis after delay
	if scoreResponseMap["status"] == "RUNNING" {
		resumeTime := metav1.NewTime(timeutil.Now().Add(resumeAfter))
		measurement.ResumeAt = &resumeTime
		measurement.Phase = v1alpha1.AnalysisPhaseRunning
		return measurement
	}
	//if run is cancelled mid-run
	if scoreResponseMap["status"] == "CANCELLED" {
		measurement.Phase = v1alpha1.AnalysisPhaseFailed
		measurement.Message = "Analysis Cancelled"
	} else {
		//POST-Run process
		measurement = processResume(data, OPSMXMetric, measurement)
	}
	finishTime := timeutil.MetaNow()
	measurement.FinishedAt = &finishTime
	return measurement
}

func (g *RpcPlugin) Terminate(analysisRun *v1alpha1.AnalysisRun, metric v1alpha1.Metric, measurement v1alpha1.Measurement) v1alpha1.Measurement {
	return measurement
}

func (g *RpcPlugin) GarbageCollect(*v1alpha1.AnalysisRun, v1alpha1.Metric, int) types.RpcError {
	return types.RpcError{}
}

func (g *RpcPlugin) Type() string {
	return plugin.ProviderType
}

func (g *RpcPlugin) GetMetadata(metric v1alpha1.Metric) map[string]string {
	metricsMetadata := make(map[string]string)
	return metricsMetadata
}

func NewHttpClient() http.Client {
	c := http.Client{
		Timeout: defaultTimeout * time.Second,
	}
	return c
}

// Evaluate canaryScore and accordingly set the AnalysisPhase
func evaluateResult(score int, pass int, marginal int) v1alpha1.AnalysisPhase {
	if score >= pass {
		return v1alpha1.AnalysisPhaseSuccessful
	}
	if score < pass && score >= marginal {
		return v1alpha1.AnalysisPhaseInconclusive
	}
	return v1alpha1.AnalysisPhaseFailed
}

func processScoreResponse(data []byte) (map[string]interface{}, error) {
	var response map[string]interface{}
	var reportUrlJson map[string]interface{}
	var status map[string]interface{}
	scoreResponseMap := make(map[string]interface{})

	err := json.Unmarshal(data, &response)
	if err != nil {
		return scoreResponseMap, fmt.Errorf("analysis Error: Error in post processing canary Response: %v", err)
	}
	canaryResultBytes, err := json.MarshalIndent(response["canaryResult"], "", "   ")
	if err != nil {
		return scoreResponseMap, err
	}
	err = json.Unmarshal(canaryResultBytes, &reportUrlJson)
	if err != nil {
		return scoreResponseMap, err
	}
	statusBytes, err := json.MarshalIndent(response["status"], "", "   ")
	if err != nil {
		return scoreResponseMap, err
	}
	err = json.Unmarshal(statusBytes, &status)
	if err != nil {
		return scoreResponseMap, err
	}

	scoreResponseMap["canaryReportURL"] = reportUrlJson["canaryReportURL"]
	scoreResponseMap["intervalNo"] = reportUrlJson["intervalNo"]
	scoreResponseMap["status"] = status["status"]

	return scoreResponseMap, nil
}

func processResume(data []byte, metric OPSMXMetric, measurement v1alpha1.Measurement) v1alpha1.Measurement {
	var (
		canaryScore string
		result      map[string]interface{}
		finalScore  map[string]interface{}
	)

	err := json.Unmarshal(data, &result)
	if err != nil {
		err := fmt.Errorf("analysis Error: Error in post processing canary Response. Error: %v", err)
		return metricutil.MarkMeasurementError(measurement, err)
	}
	jsonBytes, _ := json.MarshalIndent(result["canaryResult"], "", "   ")
	err = json.Unmarshal(jsonBytes, &finalScore)
	if err != nil {
		return metricutil.MarkMeasurementError(measurement, err)
	}
	if finalScore["overallScore"] == nil {
		canaryScore = "0"
	} else {
		canaryScore = fmt.Sprintf("%v", finalScore["overallScore"])
	}

	var score int
	if strings.Contains(canaryScore, ".") {
		floatScore, err := strconv.ParseFloat(canaryScore, 64)
		if err != nil {
			return metricutil.MarkMeasurementError(measurement, err)
		}
		score = int(roundFloat(floatScore, 0))
	} else {
		score, err = strconv.Atoi(canaryScore)
		if err != nil {
			return metricutil.MarkMeasurementError(measurement, err)
		}
	}
	measurement.Value = canaryScore
	measurement.Phase = evaluateResult(score, metric.Pass, metric.Marginal)
	if measurement.Phase == "Failed" && metric.LookBackType != "" {
		measurement.Metadata["interval analysis message"] = fmt.Sprintf("Interval Analysis Failed at intervalNo. %s", measurement.Metadata["Current intervalNo"])
	}
	return measurement
}

func getSecretData(g *RpcPlugin, metric OPSMXMetric, namespace string) (opsmxProfile, error) {
	secretName := defaultSecretName
	if metric.Profile != "" {
		secretName = metric.Profile
	}
	secret := opsmxProfile{}

	v1Secret, err := g.kubeclientset.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return opsmxProfile{}, err
	}

	secretUser, ok := v1Secret.Data["user"]
	if !ok {
		err = errors.New("opsmx profile secret validation error: `user` key not present in the secret file\n Action Required: secret file must carry data element 'user'")
		return opsmxProfile{}, err
	}
	secret.user = string(secretUser)

	secretOpsmxIsdUrl, ok := v1Secret.Data["opsmxIsdUrl"]
	if !ok {
		err = errors.New("opsmx profile secret validation error: `opsmxIsdUrl` key not present in the secret file\n Action Required: secret file must carry data element 'opsmxIsdUrl'")
		return opsmxProfile{}, err
	}
	secret.opsmxIsdUrl = string(secretOpsmxIsdUrl)

	secretSourceName, ok := v1Secret.Data["sourceName"]
	if !ok {
		err = errors.New("opsmx profile secret validation error: `sourceName` key not present in the secret file\n Action Required: secret file must carry data element 'sourceName'")
		return opsmxProfile{}, err
	}
	secret.sourceName = string(secretSourceName)

	secretcdintegration, ok := v1Secret.Data["cdIntegration"]
	if !ok {
		err = errors.New("opsmx profile secret validation error: `cdIntegration` key not present in the secret file\n Action Required: secret file must carry data element 'cdIntegration'")
		return opsmxProfile{}, err
	}
	secret.cdIntegration = string(secretcdintegration)

	secretAgentName, ok := v1Secret.Data["agentName"]
	if !ok && string(secretcdintegration) == "true" {
		err = errors.New("opsmx profile secret validation error: `agentName` key not present in the secret file\n Action Required: secret file must carry data element 'agentName' for 'cdIntegration' as 'true'")
		return opsmxProfile{}, err
	}

	secret.agentName = string(secretAgentName)

	if secret.cdIntegration != "true" && secret.cdIntegration != "false" {
		err := errors.New("opsmx profile secret validation error: `cdIntegration` should be either true or false")
		return opsmxProfile{}, err
	}

	return secret, nil
}

func getOpsmxProfile(g *RpcPlugin, metric OPSMXMetric, namespace string) (opsmxProfile, error) {
	s, err := getSecretData(g, metric, namespace)
	if err != nil {
		return opsmxProfile{}, err
	}
	if metric.OpsmxIsdUrl != "" {
		s.opsmxIsdUrl = metric.OpsmxIsdUrl
	}
	if metric.User != "" {
		s.user = metric.User
	}
	cdIntegration := cdIntegrationArgoRollouts
	if s.cdIntegration == "true" {
		cdIntegration = cdIntegrationArgoCD
	}
	s.cdIntegration = cdIntegration
	return s, nil
}

func checkISDUrl(c *RpcPlugin, opsmxIsdUrl string) error {
	resp, err := c.client.Get(opsmxIsdUrl)
	if err != nil && opsmxIsdUrl != "" && !strings.Contains(err.Error(), "timeout") {
		errorMsg := fmt.Sprintf("analysisTemplate/secret validation error: incorrect opsmxIsdUrl: %v", opsmxIsdUrl)
		return errors.New(errorMsg)
	} else if err != nil {
		return errors.New(err.Error())
	} else if resp.StatusCode != 200 {
		return errors.New(resp.Status)
	}
	return nil
}
