package binance

import (
	"crypto/hmac"
	"crypto/sha256"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// baseClient holds the shared HTTP + signing logic used by both spot and futures.
type baseClient struct {
	apiKey    string
	secretKey string
	baseURL   string
	http      *http.Client
}

func newBaseClient(apiKey, secretKey, baseURL string) baseClient {
	return baseClient{
		apiKey:    apiKey,
		secretKey: secretKey,
		baseURL:   baseURL,
		http:      &http.Client{Timeout: 10 * time.Second},
	}
}

func (b *baseClient) sign(payload string) string {
	mac := hmac.New(sha256.New, []byte(b.secretKey))
	mac.Write([]byte(payload))
	return fmt.Sprintf("%x", mac.Sum(nil))
}

func (b *baseClient) addSignature(params url.Values) {
	params.Set("timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))
	params.Set("signature", b.sign(params.Encode()))
}

func (b *baseClient) publicGET(ctx context.Context, endpoint string, params url.Values) ([]byte, error) {
	u := fmt.Sprintf("%s%s?%s", b.baseURL, endpoint, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	return b.do(req)
}

func (b *baseClient) signedGET(ctx context.Context, endpoint string, params url.Values) ([]byte, error) {
	b.addSignature(params)
	u := fmt.Sprintf("%s%s?%s", b.baseURL, endpoint, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-MBX-APIKEY", b.apiKey)
	return b.do(req)
}

func (b *baseClient) signedPOST(ctx context.Context, endpoint string, params url.Values) ([]byte, error) {
	b.addSignature(params)
	u := fmt.Sprintf("%s%s", b.baseURL, endpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-MBX-APIKEY", b.apiKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return b.do(req)
}

func (b *baseClient) do(req *http.Request) ([]byte, error) {
	resp, err := b.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}
