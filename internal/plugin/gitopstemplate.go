package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/tidwall/gjson"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getTemplateUrl(opsmxUrl string, sha1Code string, templateType string, templateName string) (string, error) {
	_url, err := url.JoinPath(opsmxUrl, templateApi)
	if err != nil {
		return "", err
	}

	urlParse, err := url.Parse(_url)
	if err != nil {
		return "", err
	}
	values := urlParse.Query()
	values.Add("sha1", sha1Code)
	values.Add("templateType", templateType)
	values.Add("templateName", templateName)
	urlParse.RawQuery = values.Encode()
	return urlParse.String(), nil
}

func generateTemplate(c *RpcPlugin, opsmxProfileData opsmxProfile, templateName string, templateType string, templateFileData string) (string, error) {
	sha1Code := generateSHA1(templateFileData)
	templateUrl, err := getTemplateUrl(opsmxProfileData.opsmxIsdUrl, sha1Code, templateType, templateName)
	if err != nil {
		return "", err
	}

	data, _, err := makeRequest(c.client, "GET", templateUrl, "", opsmxProfileData.user)
	if err != nil {
		return "", err
	}
	var templateVerification bool
	err = json.Unmarshal(data, &templateVerification)
	if err != nil {
		err := fmt.Errorf("analysis Error: Expected bool response from gitops verifyTemplate response  Error: %v. Action: Check endpoint given in secret/analysisTemplate", err)
		return "", err
	}

	var templateCheckSave map[string]interface{}
	if !templateVerification {
		data, _, err = makeRequest(c.client, "POST", templateUrl, templateFileData, opsmxProfileData.user)
		if err != nil {
			return "", err
		}
		err = json.Unmarshal(data, &templateCheckSave)
		if err != nil {
			return "", err
		}

		errMsg := fmt.Sprintf("%v", templateCheckSave["error"])
		if templateCheckSave["errorMessage"] != nil && templateCheckSave["errorMessage"] != "" {
			errMsg = fmt.Sprintf("%v", templateCheckSave["errorMessage"])
		}

		if templateCheckSave["status"] != "CREATED" {
			err = fmt.Errorf("gitops '%s' template config map validation error: %s", templateName, errMsg)
			return "", err
		}
	}
	return sha1Code, nil
}

func getTemplateDataYaml(templateFileData []byte, template string, templateType string, ScopeVariables string) ([]byte, error) {
	if templateType == templateLog {
		logData, err := processYamlLogs(templateFileData, template, ScopeVariables)
		if err != nil {
			return nil, err
		}
		return json.Marshal(logData)
	}
	metricStruct, err := processYamlMetrics(templateFileData, template, ScopeVariables)
	if err != nil {
		return nil, err
	}
	return json.Marshal(metricStruct)
}

func processGitopsTemplates(c *RpcPlugin, opsmxProfileData opsmxProfile, templateName string, templateType string, scopeVars string, nameSpace string) (string, error) {
	v1ConfigMap, err := c.kubeclientset.CoreV1().ConfigMaps(nameSpace).Get(context.TODO(), templateName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("gitops '%s' template config map validation error: %v", templateName, err)
	}
	templateDataCM, ok := v1ConfigMap.Data[templateName]
	if !ok {
		return "", fmt.Errorf("gitops '%s' template config map validation error: missing data element %s", templateName, templateName)
	}
	templateFileData := []byte(templateDataCM)
	if !json.Valid(templateFileData) {
		templateFileData, err = getTemplateDataYaml(templateFileData, templateName, templateType, scopeVars)
		if err != nil {
			return "", err
		}
	} else {
		checktemplateName := gjson.Get(string(templateFileData), "templateName")
		if checktemplateName.String() == "" {
			err := fmt.Errorf("gitops '%s' template config map validation error: template name not provided inside json", templateName)
			return "", err
		}
		if templateName != checktemplateName.String() {
			err := fmt.Errorf("gitops '%s' template config map validation error: Mismatch between templateName and data.%s key", templateName, templateName)
			return "", err
		}
	}
	sha1code, err := generateTemplate(c, opsmxProfileData, templateName, templateType, string(templateFileData))
	if err != nil {
		return "", err
	}
	return sha1code, nil
}
