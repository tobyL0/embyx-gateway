package main

import (
    "context"
    "flag"
    "log"
    "net"
    "net/http"
    "net/http/httputil"
    "net/url"
    "os"
    "os/signal"
    "strings"
    "syscall"
    "time"
)

func getenv(key, fallback string) string {
    value := strings.TrimSpace(os.Getenv(key))
    if value == "" {
        return fallback
    }
    return value
}

func main() {
    defaultListen := getenv("LISTEN_ADDR", ":8096")
    defaultUpstream := getenv("UPSTREAM_URL", "http://127.0.0.1:18096")

    listenAddr := flag.String("listen", defaultListen, "gateway listen address, e.g. :8096")
    upstreamRaw := flag.String("upstream", defaultUpstream, "Emby upstream URL, e.g. http://127.0.0.1:18096")
    flag.Parse()

    upstreamURL, err := url.Parse(*upstreamRaw)
    if err != nil {
        log.Fatalf("invalid upstream URL %q: %v", *upstreamRaw, err)
    }

    proxy := httputil.NewSingleHostReverseProxy(upstreamURL)
    originalDirector := proxy.Director

    proxy.Director = func(req *http.Request) {
        originalDirector(req)

        if host, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
            appendForwardedHeader(req, "X-Forwarded-For", host)
        }

        if req.Header.Get("X-Forwarded-Proto") == "" {
            if req.TLS != nil {
                req.Header.Set("X-Forwarded-Proto", "https")
            } else {
                req.Header.Set("X-Forwarded-Proto", "http")
            }
        }

        if req.Header.Get("X-Forwarded-Host") == "" {
            req.Header.Set("X-Forwarded-Host", req.Host)
        }
    }

    proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, proxyErr error) {
        log.Printf("proxy error: method=%s path=%s err=%v", req.Method, req.URL.Path, proxyErr)
        rw.WriteHeader(http.StatusBadGateway)
        _, _ = rw.Write([]byte("bad gateway\n"))
    }

    mux := http.NewServeMux()
    mux.Handle("/", loggingMiddleware(proxy))
    mux.HandleFunc("/healthz", func(rw http.ResponseWriter, _ *http.Request) {
        rw.WriteHeader(http.StatusOK)
        _, _ = rw.Write([]byte("ok\n"))
    })

    server := &http.Server{
        Addr:              *listenAddr,
        Handler:           mux,
        ReadHeaderTimeout: 10 * time.Second,
        IdleTimeout:       120 * time.Second,
    }

    stopCh := make(chan os.Signal, 1)
    signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)

    go func() {
        log.Printf("emby gateway listening on %s, upstream=%s", *listenAddr, upstreamURL.String())
        if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("server failed: %v", err)
        }
    }()

    <-stopCh
    log.Println("shutdown signal received, stopping gateway...")

    shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    if err := server.Shutdown(shutdownCtx); err != nil {
        log.Printf("graceful shutdown failed: %v", err)
    }

    log.Println("gateway stopped")
}

func appendForwardedHeader(req *http.Request, key, value string) {
    current := req.Header.Get(key)
    if current == "" {
        req.Header.Set(key, value)
        return
    }
    req.Header.Set(key, current+", "+value)
}

func loggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
        start := time.Now()
        next.ServeHTTP(rw, req)
        log.Printf("request: method=%s path=%s remote=%s duration=%s", req.Method, req.URL.RequestURI(), req.RemoteAddr, time.Since(start))
    })
}
