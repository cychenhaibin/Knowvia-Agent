package knowledge

import (
	"fmt"
	"strings"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/textutil"
)

func BuildChunkContent(title, repo string, updatedAt time.Time, content string) string {
	meta := make([]string, 0, 3)
	if title = strings.TrimSpace(title); title != "" {
		meta = append(meta, title)
	}
	if repo = strings.TrimSpace(repo); repo != "" {
		meta = append(meta, repo)
	}
	if !updatedAt.IsZero() {
		meta = append(meta, updatedAt.Format(time.DateOnly))
	}
	if len(meta) == 0 {
		return strings.TrimSpace(content)
	}
	return fmt.Sprintf("[%s]\n%s", strings.Join(meta, " | "), textutil.NormalizeLines(strings.TrimSpace(content)))
}
