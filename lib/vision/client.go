package vision

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
)

const serviceURL = "https://ocr.api.cloud.yandex.net"
const textRecognitionRecognize = "ocr/v1/recognizeText"

type Client struct {
	client http.Client
}

func NewClient() (*Client, error) {
	return &Client{
		client: http.Client{},
	}, nil
}

func (c *Client) DetectLicensePlates(ctx context.Context, mimeType string, content []byte, CredentialsProvider func(*http.Request)) ([]string, error) {
	request, err := textRecognitionRequest(mimeType, content)
	if err != nil {
		return nil, err
	}
	request = request.WithContext(ctx)
	CredentialsProvider(request)
	response, err := c.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var responseBody map[string]any
	err = json.NewDecoder(response.Body).Decode(&responseBody)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("could not detect license plates: %v", responseBody)
	}

	var output []string

	if result := responseBody["result"]; result != nil {
		if textAnnotation := result.(map[string]any)["textAnnotation"]; textAnnotation != nil {
			if blocks := textAnnotation.(map[string]any)["blocks"].([]any); blocks != nil {
				for _, block := range blocks {
					if lines := block.(map[string]any)["lines"].([]any); lines != nil {
						for _, line := range lines {
							output = append(output, line.(map[string]any)["text"].(string))
						}
					}
				}
			}
		}
	}
	return output, nil
}

func textRecognitionRequest(mimeType string, content []byte) (*http.Request, error) {
	var err error
	body := textRecognitionRecognizeRequestBody{
		MimeType:      mimeType,
		LanguageCodes: []string{"en", "ru"},
		Model:         "license-plates",
		Content:       base64.StdEncoding.EncodeToString(content),
	}
	var jsonBody = bytes.Buffer{}
	var jsonEncoder = json.NewEncoder(&jsonBody)
	err = jsonEncoder.Encode(body)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequest("POST", serviceURL+"/"+textRecognitionRecognize, &jsonBody)
	if err != nil {
		return nil, err
	}
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("x-data-logging-enabled", "true")
	request.Header.Add("x-folder-id", "b1gr2sfp90l7fhpvdi7c")

	return request, nil
}

type textRecognitionRecognizeRequestBody struct {
	MimeType      string   `json:"mimeType"`
	LanguageCodes []string `json:"languageCodes"`
	Model         string   `json:"model"`
	Content       string   `json:"content"`
}
