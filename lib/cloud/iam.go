package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

func WithIamToken(req *http.Request) {
	if token, err := IamToken(req.Context()); err == nil {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

func IamToken(ctx context.Context) (string, error) {
	if token, err := getIamFromEnvironment(); err == nil {
		return token, nil
	}
	return getIamTokenFromVM(ctx, &http.Client{})
}

func getIamTokenFromVM(ctx context.Context, client *http.Client) (token string, err error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://169.254.169.254/computeMetadata/v1/instance/service-accounts/default/token", nil)
	req.Header.Set("Metadata-Flavor", "Google")
	if err != nil {
		return token, err
	}
	response, err := client.Do(req)
	if err != nil {
		return token, err
	}
	var body map[string]any
	defer response.Body.Close()
	if err = json.NewDecoder(response.Body).Decode(&body); err != nil {
		return token, err
	}
	if response.StatusCode != 200 {
		return token, fmt.Errorf("error getting iam token: %v", body)
	}
	return body["access_token"].(string), nil
}

func getIamFromEnvironment() (string, error) {
	if value, ok := os.LookupEnv("IAM_TOKEN"); ok {
		return value, nil
	}
	return "", fmt.Errorf("token is not in environment")
}
