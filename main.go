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

func hello(ctx context.Context, w http.ResponseWriter, r *http.Request) (int, interface{}, error) {
	logger := xlog.FromContext(ctx)
	fmt.Fprintf(w, "api hello!")
	logger.Debugf("plain hello", xlog.F{"handler": "hello"})
	return http.StatusOK, "ok", nil
}

func staticHello(ctx context.Context, w http.ResponseWriter, r *http.Request) (int, interface{}, error) {
	logger := xlog.FromContext(ctx)
	fmt.Fprintf(w, "static hello!")
	logger.Debugf("static hello", xlog.F{"handler": "staticHello"})
	return http.StatusOK, "ok", nil
}

type myH struct {
	*app
	handler func(ctx context.Context, w http.ResponseWriter, r *http.Request) (int, interface{}, error)
}

func (h myH) ServeHTTPC(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	logger := xlog.FromContext(ctx)
	logger.Infof("%+v", h.app)
	status, res, err := h.handler(ctx, w, r)
	if err != nil {
		logger.Error(err.Error())
	}
	if status != http.StatusOK {
		logger.Debugf("%+v", res)
	}
	logger.Debugf("%+v", res)
	return
}

type app struct {
	Role string
}

func main() {
	host, _ := os.Hostname()
	conf := xlog.Config{
		Level: xlog.LevelDebug,
		Fields: xlog.F{
			"role": "my-service",
			"host": host,
		},
		Output: xlog.NewOutputChannel(xlog.MultiOutput{
			0: xlog.NewConsoleOutput(),
			1: xlog.NewJSONOutput(os.Stdout),
		}),
	}
	xlog.SetLogger(xlog.New(conf))

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

	a := &app{Role: "test-server"}
	mux := xmux.New()

	apiMux := mux.NewGroup("/v1")
	apiMux.GET("/hello", apiChain.HandlerC(myH{app: a, handler: hello}))

	staticMux := mux.NewGroup("/static")
	staticMux.GET("/hello", baseChain.HandlerC(myH{app: a, handler: staticHello}))

	xlog.Info("starting server")
	rootCtx := context.Background()
	if err := http.ListenAndServe(":8081", xhandler.New(rootCtx, mux)); err != nil {
		log.Fatal(err)
	}
}
