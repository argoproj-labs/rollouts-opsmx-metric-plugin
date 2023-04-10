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
	"encoding/json"
	"errors"
	"fmt"

	"gopkg.in/yaml.v2"
)

const DefaultsErrorTopicsJson = `{
	"errorTopics": [
	  {
		"string": "OnOutOfMemoryError",
		"topic": "critical",
		"type": "default"
	  },
	  {
		"string": "StackOverflowError",
		"topic": "critical",
		"type": "default"
	  },
	  {
		"string": "ClassNotFoundException",
		"topic": "critical",
		"type": "default"
	  },
	  {
		"string": "FileNotFoundException",
		"topic": "critical",
		"type": "default"
	  },
	  {
		"string": "ArrayIndexOutOfBounds",
		"topic": "critical",
		"type": "default"
	  },
	  {
		"string": "NullPointerException",
		"topic": "critical",
		"type": "default"
	  },
	  {
		"string": "StringIndexOutOfBoundsException",
		"topic": "critical",
		"type": "default"
	  },
	  {
		"string": "FATAL",
		"topic": "critical",
		"type": "default"
	  },
	  {
		"string": "SEVERE",
		"topic": "critical",
		"type": "default"
	  },
	  {
		"string": "NoClassDefFoundError",
		"topic": "error",
		"type": "default"
	  },
	  {
		"string": "NoSuchMethodFoundError",
		"topic": "error",
		"type": "default"
	  },
	  {
		"string": "NumberFormatException",
		"topic": "error",
		"type": "default"
	  },
	  {
		"string": "IllegalArgumentException",
		"topic": "error",
		"type": "default"
	  },
	  {
		"string": "ParseException",
		"topic": "error",
		"type": "default"
	  },
	  {
		"string": "SQLException",
		"topic": "error",
		"type": "default"
	  },
	  {
		"string": "ArithmeticException",
		"topic": "error",
		"type": "default"
	  },
	  {
		"string": "status=404",
		"topic": "error",
		"type": "default"
	  },
	  {
		"string": "status=500",
		"topic": "error",
		"type": "default"
	  },
	  {
		"string": "EXCEPTION",
		"topic": "error",
		"type": "default"
	  },
	  {
		"string": "ERROR",
		"topic": "error",
		"type": "default"
	  },
	  {
		"string": "WARN",
		"topic": "warn",
		"type": "default"
	  }
	]
  }`

type LogTemplateYaml struct {
	DisableDefaultsErrorTopics bool          `yaml:"disableDefaultErrorTopics" json:"-"`
	TemplateName               string        `yaml:"templateName" json:"templateName"`
	FilterKey                  string        `yaml:"filterKey" json:"filterKey"`
	TagEnabled                 bool          `yaml:"-" json:"tagEnabled"`
	MonitoringProvider         string        `yaml:"monitoringProvider" json:"monitoringProvider"`
	AccountName                string        `yaml:"accountName" json:"accountName"`
	ScoringAlgorithm           string        `yaml:"scoringAlgorithm" json:"scoringAlgorithm"`
	Index                      string        `yaml:"index,omitempty" json:"index,omitempty"`
	ResponseKeywords           string        `yaml:"responseKeywords" json:"responseKeywords"`
	ContextualCluster          bool          `yaml:"contextualCluster,omitempty" json:"contextualCluster,omitempty"`
	ContextualWindowSize       int           `yaml:"contextualWindowSize,omitempty" json:"contextualWindowSize,omitempty"`
	InfoScoring                bool          `yaml:"infoScoring,omitempty" json:"infoScoring,omitempty"`
	RegExFilter                bool          `yaml:"regExFilter,omitempty" json:"regExFilter,omitempty"`
	RegExResponseKey           string        `yaml:"regExResponseKey,omitempty" json:"regExResponseKey,omitempty"`
	RegularExpression          string        `yaml:"regularExpression,omitempty" json:"regularExpression,omitempty"`
	AutoBaseline               bool          `yaml:"autoBaseline,omitempty" json:"autoBaseline,omitempty"`
	Sensitivity                string        `yaml:"sensitivity,omitempty" json:"sensitivity,omitempty"`
	StreamID                   string        `yaml:"streamId,omitempty" json:"streamId,omitempty"`
	Tags                       []customTags  `yaml:"tags" json:"tags,omitempty"`
	ErrorTopics                []errorTopics `yaml:"errorTopics" json:"errorTopics"`
}

type customTags struct {
	ErrorStrings string `yaml:"errorString" json:"string"`
	Tag          string `yaml:"tag" json:"tag"`
}

type errorTopics struct {
	ErrorStrings string `yaml:"errorString" json:"string"`
	Topic        string `yaml:"topic" json:"topic"`
	Type         string `yaml:"-" json:"type"`
}

func processYamlLogs(templateFileData []byte, template string, ScopeVariables string) (LogTemplateYaml, error) {
	var logdata LogTemplateYaml
	if err := yaml.Unmarshal([]byte(templateFileData), &logdata); err != nil {
		errorMessage := fmt.Sprintf("gitops '%s' template config map validation error: %v", template, err)
		return LogTemplateYaml{}, errors.New(errorMessage)
	}
	logdata.TemplateName = template
	logdata.FilterKey = ScopeVariables
	if len(logdata.Tags) >= 1 {
		logdata.TagEnabled = true
	}

	var defaults LogTemplateYaml
	err := json.Unmarshal([]byte(DefaultsErrorTopicsJson), &defaults)
	if err != nil {
		return LogTemplateYaml{}, err
	}

	var defaultErrorString []string
	defaultErrorStringMapType := make(map[string]string)
	for _, items := range defaults.ErrorTopics {
		defaultErrorStringMapType[items.ErrorStrings] = items.Topic
		defaultErrorString = append(defaultErrorString, items.ErrorStrings)
	}

	var errorStringsAvailable []string

	for i, items := range logdata.ErrorTopics {
		errorStringsAvailable = append(errorStringsAvailable, items.ErrorStrings)

		if isExists(defaultErrorString, items.ErrorStrings) {
			if items.Topic == defaultErrorStringMapType[items.ErrorStrings] {
				logdata.ErrorTopics[i].Type = "default"
			} else {
				logdata.ErrorTopics[i].Type = "custom"
			}
		}
	}

	if !logdata.DisableDefaultsErrorTopics {
		for _, items := range defaults.ErrorTopics {
			if !isExists(errorStringsAvailable, items.ErrorStrings) {
				logdata.ErrorTopics = append(logdata.ErrorTopics, items)
			}
		}
	}
	if logdata.ErrorTopics == nil {
		logdata.ErrorTopics = make([]errorTopics, 0)
	}
	return logdata, nil
}
