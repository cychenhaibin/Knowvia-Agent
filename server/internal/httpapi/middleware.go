package httpapi

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"
)

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func newLoggingResponseWriter(w http.ResponseWriter) *loggingResponseWriter {
	return &loggingResponseWriter{
		ResponseWriter: w,
		status:         http.StatusOK,
	}
}

func (w *loggingResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *loggingResponseWriter) Write(data []byte) (int, error) {
	n, err := w.ResponseWriter.Write(data)
	w.bytes += n
	return n, err
}

func (w *loggingResponseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *loggingResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("http hijacker is not supported")
	}
	return hijacker.Hijack()
}

func (w *loggingResponseWriter) Push(target string, opts *http.PushOptions) error {
	pusher, ok := w.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, opts)
}

func (w *loggingResponseWriter) ReadFrom(r io.Reader) (int64, error) {
	if readerFrom, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		n, err := readerFrom.ReadFrom(r)
		w.bytes += int(n)
		return n, err
	}

	n, err := io.Copy(w.ResponseWriter, r)
	w.bytes += int(n)
	return n, err
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		queryKeys := make([]string, 0, len(r.URL.Query()))
		for key := range r.URL.Query() {
			queryKeys = append(queryKeys, key)
		}
		sort.Strings(queryKeys)

		log.Printf(
			"HTTP start method=%s path=%s host=%s query_keys=%s remote=%s ua=%q",
			r.Method,
			r.URL.Path,
			r.Host,
			formatQueryKeys(queryKeys),
			formatRemoteAddr(r.RemoteAddr),
			r.UserAgent(),
		)

		recorder := newLoggingResponseWriter(w)
		next.ServeHTTP(recorder, r)

		log.Printf(
			"HTTP done method=%s path=%s status=%d bytes=%d duration=%s remote=%s",
			r.Method,
			r.URL.Path,
			recorder.status,
			recorder.bytes,
			time.Since(startedAt).Round(time.Millisecond),
			formatRemoteAddr(r.RemoteAddr),
		)
	})
}

func formatQueryKeys(keys []string) string {
	if len(keys) == 0 {
		return "-"
	}
	return strings.Join(keys, ",")
}

func formatRemoteAddr(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}
