package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	wechatAccessTokenURL = "https://api.weixin.qq.com/sns/oauth2/access_token"
	wechatUserInfoURL    = "https://api.weixin.qq.com/sns/userinfo"
)

type VerifiedWeChatIdentity struct {
	OpenID      string
	UnionID     string
	Nickname    string
	AvatarURL   string
	Country     string
	Province    string
	City        string
	Privilege   []string
	AccessToken string
}

type WeChatCodeExchanger interface {
	ExchangeCode(ctx context.Context, code string) (VerifiedWeChatIdentity, error)
}

type wechatCodeExchanger struct {
	appID      string
	appSecret  string
	httpClient *http.Client
}

type wechatErrorPayload struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

type wechatTokenResponse struct {
	wechatErrorPayload
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	OpenID       string `json:"openid"`
	Scope        string `json:"scope"`
	UnionID      string `json:"unionid"`
}

type wechatUserInfoResponse struct {
	wechatErrorPayload
	OpenID     string   `json:"openid"`
	Nickname   string   `json:"nickname"`
	Sex        int      `json:"sex"`
	Province   string   `json:"province"`
	City       string   `json:"city"`
	Country    string   `json:"country"`
	HeadImgURL string   `json:"headimgurl"`
	Privilege  []string `json:"privilege"`
	UnionID    string   `json:"unionid"`
}

func NewWeChatCodeExchanger(appID, appSecret string) WeChatCodeExchanger {
	appID = strings.TrimSpace(appID)
	appSecret = strings.TrimSpace(appSecret)
	if appID == "" || appSecret == "" {
		return nil
	}
	return &wechatCodeExchanger{
		appID:     appID,
		appSecret: appSecret,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (e *wechatCodeExchanger) ExchangeCode(ctx context.Context, code string) (VerifiedWeChatIdentity, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return VerifiedWeChatIdentity{}, ErrInvalidWeChatCode
	}

	token, err := e.fetchAccessToken(ctx, code)
	if err != nil {
		return VerifiedWeChatIdentity{}, err
	}
	userInfo, err := e.fetchUserInfo(ctx, token.AccessToken, token.OpenID)
	if err != nil {
		return VerifiedWeChatIdentity{}, err
	}

	unionID := strings.TrimSpace(userInfo.UnionID)
	if unionID == "" {
		unionID = strings.TrimSpace(token.UnionID)
	}
	openID := strings.TrimSpace(userInfo.OpenID)
	if openID == "" {
		openID = strings.TrimSpace(token.OpenID)
	}
	if openID == "" || (unionID == "" && openID == "") {
		return VerifiedWeChatIdentity{}, ErrInvalidWeChatCode
	}

	return VerifiedWeChatIdentity{
		OpenID:      openID,
		UnionID:     unionID,
		Nickname:    strings.TrimSpace(userInfo.Nickname),
		AvatarURL:   strings.TrimSpace(userInfo.HeadImgURL),
		Country:     strings.TrimSpace(userInfo.Country),
		Province:    strings.TrimSpace(userInfo.Province),
		City:        strings.TrimSpace(userInfo.City),
		Privilege:   userInfo.Privilege,
		AccessToken: strings.TrimSpace(token.AccessToken),
	}, nil
}

func (e *wechatCodeExchanger) fetchAccessToken(ctx context.Context, code string) (wechatTokenResponse, error) {
	params := url.Values{}
	params.Set("appid", e.appID)
	params.Set("secret", e.appSecret)
	params.Set("code", code)
	params.Set("grant_type", "authorization_code")

	var payload wechatTokenResponse
	if err := e.getJSON(ctx, wechatAccessTokenURL, params, &payload); err != nil {
		return wechatTokenResponse{}, err
	}
	if payload.ErrCode != 0 {
		return wechatTokenResponse{}, ErrInvalidWeChatCode
	}
	if strings.TrimSpace(payload.AccessToken) == "" || strings.TrimSpace(payload.OpenID) == "" {
		return wechatTokenResponse{}, ErrInvalidWeChatCode
	}
	return payload, nil
}

func (e *wechatCodeExchanger) fetchUserInfo(ctx context.Context, accessToken, openID string) (wechatUserInfoResponse, error) {
	params := url.Values{}
	params.Set("access_token", accessToken)
	params.Set("openid", openID)

	var payload wechatUserInfoResponse
	if err := e.getJSON(ctx, wechatUserInfoURL, params, &payload); err != nil {
		return wechatUserInfoResponse{}, err
	}
	if payload.ErrCode != 0 {
		return wechatUserInfoResponse{}, ErrWeChatIdentityUnavailable
	}
	return payload, nil
}

func (e *wechatCodeExchanger) getJSON(ctx context.Context, endpoint string, params url.Values, target interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return ErrWeChatIdentityUnavailable
	}
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return ErrWeChatIdentityUnavailable
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return ErrWeChatIdentityUnavailable
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return ErrWeChatIdentityUnavailable
	}
	return nil
}

var _ WeChatCodeExchanger = (*wechatCodeExchanger)(nil)
