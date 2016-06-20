package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"golang.org/x/net/context"

	"github.com/rs/xhandler"
	"github.com/rs/xlog"
	"github.com/rs/xmux"
)

func loggingMiddleware(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		t1 := time.Now()
		next.ServeHTTP(w, r)
		t2 := time.Now()
		log.Printf("[%s] %q %v\n", r.Method, r.URL.String(), t2.Sub(t1))
	}
	return http.HandlerFunc(fn)
}

func recoverMiddleware(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic: %+v", err)
				http.Error(w, http.StatusText(500), 500)
			}
		}()
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}

func authMiddleware(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		log.Println("auth middle start")
		next.ServeHTTP(w, r)
		log.Println("auth middle end")
	}
	return http.HandlerFunc(fn)
}

func hello(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	l := xlog.FromContext(ctx)
	fmt.Fprintf(w, "api hello!")
	l.Debugf("debug")
	return
}

func staticHello(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "static hello!")
	return
}

func main() {
	host, _ := os.Hostname()
	conf := xlog.Config{
		// Log info level and higher
		Level: xlog.LevelInfo,
		// Set some global env fields
		Fields: xlog.F{
			"role": "my-service",
			"host": host,
		},
		// Output everything on console
		Output: xlog.NewOutputChannel(xlog.NewConsoleOutput()),
	}

	// Optionally plug the xlog handler's input to Go's default logger
	log.SetFlags(0)
	xlogger := xlog.New(conf)
	log.SetOutput(xlogger)

	baseChain := xhandler.Chain{}
	baseChain.Add(
		recoverMiddleware,
		loggingMiddleware,
		xlog.NewHandler(conf),
		xlog.MethodHandler("method"),
		xlog.URLHandler("url"),
		xlog.RemoteAddrHandler("ip"),
		xlog.UserAgentHandler("user_agent"),
		xlog.RefererHandler("referer"),
		xlog.RequestIDHandler("req_id", "Request-Id"),
	)
	apiChain := baseChain.With(
		authMiddleware,
	)

	mux := xmux.New()

	apiMux := mux.NewGroup("/v1")
	apiMux.GET("/hello", apiChain.HandlerC(xhandler.HandlerFuncC(hello)))

	staticMux := mux.NewGroup("/static")
	staticMux.GET("/hello", baseChain.HandlerC(xhandler.HandlerFuncC(staticHello)))

	rootCtx := context.Background()
	if err := http.ListenAndServe(":8081", xhandler.New(rootCtx, mux)); err != nil {
		log.Fatal(err)
	}
}
