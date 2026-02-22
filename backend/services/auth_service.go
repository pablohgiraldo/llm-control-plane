package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/upb/llm-control-plane/backend/config"
)

// TokenResponse represents the OAuth2 token endpoint response from Cognito
type TokenResponse struct {
	IDToken      string `json:"id_token"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// CognitoTokenExchanger exchanges authorization codes for tokens via Cognito
type CognitoTokenExchanger struct {
	cfg        config.CognitoConfig
	httpClient *http.Client
}

// NewCognitoTokenExchanger creates a new token exchanger
func NewCognitoTokenExchanger(cfg config.CognitoConfig) *CognitoTokenExchanger {
	return &CognitoTokenExchanger{
		cfg: cfg,
		httpClient: &http.Client{
			Transport: &http.Transport{},
		},
	}
}

// ExchangeCode exchanges an authorization code for ID and access tokens
func (e *CognitoTokenExchanger) ExchangeCode(ctx context.Context, code, redirectURI, state string) (idToken string, err error) {
	if e.cfg.Domain == "" || e.cfg.ClientID == "" {
		return "", fmt.Errorf("cognito not configured")
	}

	tokenURL := strings.TrimSuffix(e.cfg.Domain, "/") + "/oauth2/token"
	data := url.Values{
		"grant_type":   {"authorization_code"},
		"client_id":    {e.cfg.ClientID},
		"code":         {code},
		"redirect_uri": {redirectURI},
	}

	if e.cfg.ClientSecret != "" {
		data.Set("client_secret", e.cfg.ClientSecret)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token exchange failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("parse token response: %w", err)
	}

	if tokenResp.IDToken == "" {
		return "", fmt.Errorf("no id_token in response")
	}

	return tokenResp.IDToken, nil
}
