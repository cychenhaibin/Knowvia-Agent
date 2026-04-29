package pythonproxy

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/config"
)

type Client struct {
	baseURL     string
	staticToken string
	username    string
	password    string
	deviceInfo  string
	httpClient  *http.Client
	syncClient  *http.Client

	mu          sync.Mutex
	cachedToken string
}

func New(cfg config.Config) *Client {
	if strings.TrimSpace(cfg.PythonProxyBaseURL) == "" {
		return nil
	}
	return &Client{
		baseURL:     strings.TrimRight(cfg.PythonProxyBaseURL, "/"),
		staticToken: strings.TrimSpace(cfg.PythonProxyToken),
		username:    strings.TrimSpace(cfg.PythonProxyUsername),
		password:    cfg.PythonProxyPassword,
		deviceInfo:  strings.TrimSpace(cfg.PythonProxyDeviceInfo),
		httpClient: &http.Client{
			Timeout: 90 * time.Second,
		},
		syncClient: &http.Client{
			Timeout: 10 * time.Minute,
		},
	}
}
