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
	"crypto/sha1"
	"encoding/hex"
	"io"
	"math"
	"net/http"
	"strings"
)

func roundFloat(val float64, precision uint) float64 {
	ratio := math.Pow(10, float64(precision))
	return math.Round(val*ratio) / ratio
}

func makeRequest(client http.Client, requestType string, url string, body string, user string) ([]byte, string, error) {
	reqBody := strings.NewReader(body)
	req, _ := http.NewRequest(
		requestType,
		url,
		reqBody,
	)
	req.Header.Set("x-spinnaker-user", user)
	req.Header.Set("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return []byte{}, "", err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		return []byte{}, "", err
	}

	var urlToken string
	if strings.Contains(url, "registerCanary") {
		urlToken = res.Header.Get("x-opsmx-report-token")
	}
	return data, urlToken, err
}

func isExists(list []string, item string) bool {
	for _, v := range list {
		if item == v {
			return true
		}
	}
	return false
}

func serviceExists(list []service, serviceName string) bool {
	for _, v := range list {
		if v.serviceName == serviceName {
			return true
		}
	}
	return false
}

func generateSHA1(s string) string {
	h := sha1.New()
	h.Write([]byte(s))
	sha1_hash := hex.EncodeToString(h.Sum(nil))
	return sha1_hash
}
