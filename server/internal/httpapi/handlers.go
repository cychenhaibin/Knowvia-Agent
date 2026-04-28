package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/auth"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/chat"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/knowledge"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/mirror"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/run"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillimport"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/store"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/taskqueue"
)

type Handler struct {
	authService      *auth.Service
	store            store.Store
	runService       *run.Service
	knowledgeService *knowledge.Service
	mirrorService    *mirror.Service
	chatService      *chat.Service
	skillImporter    *skillimport.Service
	dispatcher       taskqueue.Dispatcher
	broker           *run.EventBroker
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	switch status {
	case http.StatusInternalServerError:
		writeInternalError(w, message)
		return
	case http.StatusServiceUnavailable:
		writeServiceUnavailable(w, message)
		return
	}
	writeJSON(w, status, map[string]string{"error": message})
}

func writeMirrorError(w http.ResponseWriter, message string, err error) {
	details := errorDetailsString("cause", err.Error())
	if details == nil {
		details = map[string]any{"upstream": "python_mirror"}
	} else {
		details["upstream"] = "python_mirror"
	}
	writeBadGateway(w, message, details)
}

func streamJSON(w http.ResponseWriter, flusher http.Flusher, payload any) {
	data, _ := json.Marshal(payload)
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}

func splitCommaQuery(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	parts := strings.Split(raw, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			items = append(items, part)
		}
	}
	return items
}
