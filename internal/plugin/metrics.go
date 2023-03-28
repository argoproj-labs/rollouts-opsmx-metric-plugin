package plugin

import (
	"errors"
	"fmt"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type Metrics struct {
	MetricWeight          *float64 `yaml:"metricWeight" json:"metricWeight,omitempty"`
	NanStrategy           string   `yaml:"nanStrategy" json:"nanStrategy,omitempty"`
	AccountName           string   `yaml:"accountName" json:"accountName,omitempty"`
	RiskDirection         string   `yaml:"riskDirection" json:"riskDirection,omitempty"`
	CustomThresholdHigher int      `yaml:"customThresholdHigherPercentage" json:"customThresholdHigher,omitempty"`
	Name                  string   `yaml:"name" json:"name,omitempty"`
	Criticality           string   `yaml:"criticality" json:"criticality,omitempty"`
	CustomThresholdLower  int      `yaml:"customThresholdLowerPercentage" json:"customThresholdLower,omitempty"`
	Watchlist             bool     `yaml:"watchlist" json:"watchlist"`
}

type Groups struct {
	Metrics []Metrics `yaml:"metrics" json:"metrics"`
	Group   string    `yaml:"group" json:"group,omitempty"`
}

type Data struct {
	//PercentDiffThreshold string   `yaml:"percentDiffThreshold" json:"percentDiffThreshold,omitempty"`
	IsNormalize bool     `yaml:"-" json:"isNormalize"`
	Groups      []Groups `yaml:"groups" json:"groups"`
}
type MetricISDTemplate struct {
	FilterKey          string   `yaml:"filterKey" json:"filterKey,omitempty"`
	AccountName        string   `yaml:"accountName" json:"accountName,omitempty"`
	Data               Data     `yaml:"metricTemplateSetup" json:"data"`
	TemplateName       string   `yaml:"templateName" json:"templateName,omitempty"`
	MonitoringProvider string   `yaml:"monitoringProvider" json:"monitoringProvider,omitempty"`
	MetricWeight       *float64 `yaml:"metricWeight" json:"metricWeight,omitempty"`
	NanStrategy        string   `yaml:"nanStrategy" json:"nanStrategy,omitempty"`
	Criticality        string   `yaml:"criticality" json:"criticality,omitempty"`
}

func (m *MetricISDTemplate) setMetricWeight(templateName string) {
	//metricWeight
	if m.MetricWeight == nil {
		log.Infof("the metricWeight field is not defined at the global level for metric template %s, values at the metric level will be used", templateName)
		return
	}
	for _, metric := range m.Data.Groups {
		for i := range metric.Metrics {
			if metric.Metrics[i].MetricWeight == nil {
				metric.Metrics[i].MetricWeight = m.MetricWeight
			}
		}
	}
	m.MetricWeight = nil
}

func (m *MetricISDTemplate) setNanStrategy(templateName string) {
	//nanStrategy
	if m.NanStrategy == "" {
		log.Infof("the nanStrategy field is not defined at the global level for metric template %s, values at the metric level will be used", templateName)
		return
	}
	for _, metric := range m.Data.Groups {
		for i := range metric.Metrics {
			if metric.Metrics[i].NanStrategy == "" {
				metric.Metrics[i].NanStrategy = m.NanStrategy
			}
		}
	}
	m.NanStrategy = ""
}

func (m *MetricISDTemplate) setTemplateName(templateName string) {
	if m.TemplateName != "" && m.TemplateName != templateName {
		log.Warnf("the templateName field has been defined in the metric template %s, it will be overriden", templateName)
	}
	m.TemplateName = templateName
}

func (m *MetricISDTemplate) setFilterKey(templateName string, metricScopeVariables string) {
	if m.FilterKey != "" && m.FilterKey != metricScopeVariables {
		log.Warnf("the filterKey field has been defined in the metric template %s, it will be overriden by %s", templateName, metricScopeVariables)
	}
	m.FilterKey = metricScopeVariables
}

func (m *MetricISDTemplate) setCriticality(templateName string) {
	//criticality
	if m.Criticality == "" {
		log.Infof("the criticality field is not defined at the global level for metric template %s, values at the metric level will be used", templateName)
		return
	}
	for _, metric := range m.Data.Groups {
		for i := range metric.Metrics {
			if metric.Metrics[i].Criticality == "" {
				metric.Metrics[i].Criticality = m.Criticality
			}
		}
	}
	m.Criticality = ""
}

func (m *MetricISDTemplate) checkMetricTemplateErrors(templateName string) error {
	//check for groups array
	if len(m.Data.Groups) == 0 {
		errMsg := fmt.Sprintf("gitops '%s' template config map validation error: metric template %s does not have any members defined for the groups field", templateName, templateName)
		return errors.New(errMsg)
	}
	return nil
}

func processYamlMetrics(templateData []byte, templateName string, scopeVariables string) (MetricISDTemplate, error) {
	metric := MetricISDTemplate{}
	err := yaml.Unmarshal(templateData, &metric)
	if err != nil {
		errorMsg := fmt.Sprintf("gitops '%s' template config map validation error: %v", templateName, err)
		return MetricISDTemplate{}, errors.New(errorMsg)
	}

	metric.setFilterKey(templateName, scopeVariables)
	metric.setTemplateName(templateName)
	metric.setMetricWeight(templateName)
	metric.setNanStrategy(templateName)
	metric.setCriticality(templateName)

	if err = metric.checkMetricTemplateErrors(templateName); err != nil {
		return MetricISDTemplate{}, err
	}
	log.Info("processed template and converting to json", metric)
	return metric, nil
}
