package config

import (
	"encoding/base64"
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

type Config struct {
	ServerAddr            string
	StoreBackend          string
	JWTSecret             string
	AccessTTL             time.Duration
	RefreshTTL            time.Duration
	QueueMode             string
	RedisAddr             string
	QueueName             string
	QueueWorkers          int
	QueueMaxAttempts      int
	QueuePollTimeout      time.Duration
	QueueRetryBaseDelay   time.Duration
	QueueRetryMaxDelay    time.Duration
	QueueMetricsLogPeriod time.Duration
	QueueShutdownTimeout  time.Duration
	QueueDedupTTL         time.Duration
	PostgresDSN           string
	GoogleWebClientID     string
	MicrosoftClientID     string
	MicrosoftTenantID     string
	FluxAPaidOrigin       string
	FluxAFreeOrigin       string
	FluxACredentialsKey   []byte
	OpenAIBaseURL         string
	OpenAIAPIKey          string
	OpenAIChatModel       string
	OpenAIEmbedModel      string
	RerankModel           string
	PythonProxyBaseURL    string
	PythonProxyToken      string
	PythonProxyUsername   string
	PythonProxyPassword   string
	PythonProxyDeviceInfo string
	DevUsers              []DevUser
	KnowledgeChunkSize    int
}

type DevUser struct {
	Username    string
	Password    string
	DisplayName string
}

func Load() Config {
	postgresDSN := getenv("QQA_POSTGRES_DSN", "")
	storeBackend := strings.ToLower(strings.TrimSpace(os.Getenv("QQA_STORE_BACKEND")))
	if storeBackend == "" {
		if postgresDSN != "" {
			storeBackend = "postgres"
		} else {
			storeBackend = "memory"
		}
	}

	fluxACredentialsKey, _ := DecodeFluxACredentialsKey(os.Getenv("QQA_FLUXA_CREDENTIALS_KEY"))

	return Config{
		ServerAddr:            getenv("QQA_SERVER_ADDR", "0.0.0.0:8088"),
		StoreBackend:          storeBackend,
		JWTSecret:             getenv("QQA_JWT_SECRET", "quickque-agent-dev-secret"),
		AccessTTL:             time.Duration(getenvInt("QQA_ACCESS_TTL_MINUTES", 30)) * time.Minute,
		RefreshTTL:            time.Duration(getenvInt("QQA_REFRESH_TTL_HOURS", 24*7)) * time.Hour,
		QueueMode:             getenv("QQA_QUEUE_MODE", "inline"),
		RedisAddr:             getenv("QQA_REDIS_ADDR", "127.0.0.1:6380"),
		QueueName:             getenv("QQA_QUEUE_NAME", "knowvia:tasks"),
		QueueWorkers:          getenvInt("QQA_QUEUE_WORKERS", 4),
		QueueMaxAttempts:      getenvInt("QQA_QUEUE_MAX_ATTEMPTS", 5),
		QueuePollTimeout:      time.Duration(getenvInt("QQA_QUEUE_POLL_TIMEOUT_SECONDS", 5)) * time.Second,
		QueueRetryBaseDelay:   time.Duration(getenvInt("QQA_QUEUE_RETRY_BASE_SECONDS", 2)) * time.Second,
		QueueRetryMaxDelay:    time.Duration(getenvInt("QQA_QUEUE_RETRY_MAX_SECONDS", 120)) * time.Second,
		QueueMetricsLogPeriod: time.Duration(getenvInt("QQA_QUEUE_METRICS_LOG_SECONDS", 30)) * time.Second,
		QueueShutdownTimeout:  time.Duration(getenvInt("QQA_QUEUE_SHUTDOWN_TIMEOUT_SECONDS", 15)) * time.Second,
		QueueDedupTTL:         time.Duration(getenvInt("QQA_QUEUE_DEDUP_TTL_SECONDS", 21600)) * time.Second,
		PostgresDSN:           postgresDSN,
		GoogleWebClientID:     getenv("QQA_GOOGLE_WEB_CLIENT_ID", ""),
		MicrosoftClientID:     getenv("QQA_MICROSOFT_CLIENT_ID", ""),
		MicrosoftTenantID:     getenv("QQA_MICROSOFT_TENANT_ID", "consumers"),
		FluxAPaidOrigin:       NormalizeFluxAOrigin(getenv("QQA_FLUXA_PAID_ORIGIN", "https://fluxa.camila.qzz.io")),
		FluxAFreeOrigin:       NormalizeFluxAOrigin(getenv("QQA_FLUXA_FREE_ORIGIN", "https://free.camila.qzz.io")),
		FluxACredentialsKey:   fluxACredentialsKey,
		OpenAIBaseURL:         getenv("QQA_OPENAI_BASE_URL", "http://127.0.0.1:11434/v1"),
		OpenAIAPIKey:          getenv("QQA_OPENAI_API_KEY", "ollama"),
		OpenAIChatModel:       getenv("QQA_OPENAI_CHAT_MODEL", domain.DefaultKnowledgeChatModelName),
		OpenAIEmbedModel:      getenv("QQA_OPENAI_EMBEDDING_MODEL", "Qwen/Qwen3-Embedding-4B"),
		RerankModel:           getenv("QQA_RERANK_MODEL", "Qwen/Qwen3-Reranker-4B"),
		PythonProxyBaseURL:    strings.TrimRight(getenv("QQA_PYTHON_PROXY_BASE_URL", ""), "/"),
		PythonProxyToken:      getenv("QQA_PYTHON_PROXY_TOKEN", "quickque-python-internal-dev-token"),
		PythonProxyUsername:   getenv("QQA_PYTHON_PROXY_USERNAME", ""),
		PythonProxyPassword:   os.Getenv("QQA_PYTHON_PROXY_PASSWORD"),
		PythonProxyDeviceInfo: getenv("QQA_PYTHON_PROXY_DEVICE_INFO", "quickque-agent-go-proxy"),
		DevUsers:              parseDevUsers(getenv("QQA_DEV_USERS", "admin:admin123")),
		KnowledgeChunkSize:    getenvInt("QQA_KNOWLEDGE_CHUNK_SIZE", 900),
	}
}

func DecodeFluxACredentialsKey(raw string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || len(key) != 32 {
		return nil, errors.New("FluxA credentials key must be a base64-encoded 32-byte key")
	}
	return key, nil
}

// NormalizeFluxAOrigin accepts only an HTTPS origin. Invalid values return an
// empty string so downstream authentication fails closed without making an
// outbound request to an attacker-controlled URL.
func NormalizeFluxAOrigin(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	origin, err := url.Parse(value)
	if err != nil || origin.Scheme != "https" || origin.Host == "" || origin.User != nil || origin.RawQuery != "" || origin.ForceQuery || origin.Fragment != "" || strings.Contains(value, "#") || origin.Opaque != "" {
		return ""
	}
	if origin.Path != "" && origin.Path != "/" {
		return ""
	}
	return "https://" + origin.Host
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return parsed
}

func parseDevUsers(raw string) []DevUser {
	users := make([]DevUser, 0)
	for _, pair := range strings.Split(raw, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, ":", 3)
		if len(parts) < 2 {
			continue
		}
		user := DevUser{
			Username: strings.TrimSpace(parts[0]),
			Password: strings.TrimSpace(parts[1]),
		}
		if len(parts) == 3 {
			user.DisplayName = strings.TrimSpace(parts[2])
		}
		if user.Username == "" || user.Password == "" {
			continue
		}
		if user.DisplayName == "" {
			user.DisplayName = user.Username
		}
		users = append(users, user)
	}
	return users
}
