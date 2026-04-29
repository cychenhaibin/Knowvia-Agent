package knowledge

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/textutil"
)

func ContentHash(content string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(content)))
	return hex.EncodeToString(sum[:])
}

func NormalizeBody(body string) string {
	return textutil.NormalizeBody(body)
}

func SplitIntoChunks(content string, chunkSize int) []string {
	if chunkSize <= 0 {
		chunkSize = 460
	}
	runes := []rune(strings.TrimSpace(content))
	if len(runes) == 0 {
		return nil
	}

	overlap := 100
	if overlap >= chunkSize {
		overlap = chunkSize / 4
	}
	if overlap < 0 {
		overlap = 0
	}

	chunks := []string{}
	for start := 0; start < len(runes); {
		end := start + chunkSize
		if end >= len(runes) {
			chunk := strings.TrimSpace(string(runes[start:]))
			if chunk != "" {
				chunks = append(chunks, chunk)
			}
			break
		}

		split := findChunkBoundary(runes, start, end)
		chunk := strings.TrimSpace(string(runes[start:split]))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}

		nextStart := split - overlap
		if nextStart <= start {
			nextStart = split
		}
		start = nextStart
	}
	return chunks
}

func findChunkBoundary(runes []rune, start, end int) int {
	minSplit := start + (end-start)/2
	for idx := end; idx > minSplit; idx-- {
		if isChunkSeparator(runes[idx-1]) {
			return idx
		}
	}
	return end
}

func isChunkSeparator(r rune) bool {
	switch r {
	case '\n', '。', '！', '？', '，', '、', '.', '!', '?', ',', ';', '；', ':', '：', ' ':
		return true
	default:
		return false
	}
}
