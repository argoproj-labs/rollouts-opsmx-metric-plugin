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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLogs(t *testing.T) {
	t.Run("basic flow - no error is raised", func(t *testing.T) {
		logsData := `
        monitoringProvider: ELASTICSEARCH
        accountName: ds-elastic
        scoringAlgorithm: Canary
        index: kubernetes*
        responseKeywords: log,message
        disableDefaultErrorTopics: false
        tags:
        - errorString: NonOutOfMemoryError
          tag: tag1
        errorTopics:
        - errorString: OnOutOfMemoryError
          topic: CRITICAL
          type: NotDefault`

		logTemplate, err := processYamlLogs([]byte(logsData), "templateLog", "${namespace_key}")
		assert.Equal(t, "templateLog", logTemplate.TemplateName)
		assert.Equal(t, "${namespace_key}", logTemplate.FilterKey)
		assert.Equal(t, true, logTemplate.TagEnabled)

		assert.Nil(t, err)
	})

	t.Run("basic flow with error topic present in json string - no error is raised", func(t *testing.T) {
		logsData := `
        monitoringProvider: ELASTICSEARCH
        accountName: ds-elastic
        scoringAlgorithm: Canary
        index: kubernetes*
        responseKeywords: log,message
        disableDefaultErrorTopics: false
        tags:
        - errorString: NonOutOfMemoryError
          tag: tag1
        errorTopics:
        - errorString: OnOutOfMemoryError
          topic: critical
          type: NotDefault`

		_, err := processYamlLogs([]byte(logsData), "templateLog", "${namespace_key}")
		assert.Nil(t, err)
	})

	t.Run("basic flow with disableDefaultErrorTopics set as true and no topics defined in the yaml- len of the errorTopics array should be 0", func(t *testing.T) {
		logsData := `
        monitoringProvider: ELASTICSEARCH
        accountName: ds-elastic
        scoringAlgorithm: Canary
        index: kubernetes*
        responseKeywords: log,message
        disableDefaultErrorTopics: true
        tags:
        - errorString: NonOutOfMemoryError
          tag: tag1`

		logTemplate, err := processYamlLogs([]byte(logsData), "templateLog", "${namespace_key}")
		assert.Equal(t, 0, len(logTemplate.ErrorTopics))
		assert.Nil(t, err)
	})
}
