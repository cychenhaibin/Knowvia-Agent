package feishu

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

func (c *Client) tenantAccessToken(ctx context.Context, appID, appSecret string) (string, error) {
	url := c.baseURL + "/auth/v3/tenant_access_token/internal"
	reqBody := map[string]string{
		"app_id":     appID,
		"app_secret": appSecret,
	}
	var payload struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
	}
	if err := c.doJSON(ctx, http.MethodPost, url, "", nil, reqBody, &payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.TenantAccessToken) == "" {
		return "", fmt.Errorf("feishu tenant_access_token missing in response")
	}
	return strings.TrimSpace(payload.TenantAccessToken), nil
}
