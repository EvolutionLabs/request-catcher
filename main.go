package main

import (
	"crypto/subtle"
	"fmt"
	"io"
	"net/http"
	"net/http/pprof"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/EvolutionLabs/request-catcher/catcher"
)

func main() {
	if len(os.Args) < 2 {
		fatalf("Usage: request-catcher <config-filename>\n")
	}
	config, err := catcher.LoadConfiguration(os.Args[1])
	if err != nil {
		fatalf("error loading configuration file: %s\n", err)
	}

	newCatcher := catcher.NewCatcher(config)

	// Start the HTTP server.
	fullHost := config.Host + ":" + strconv.Itoa(config.HTTPPort)
	server := http.Server{
		Addr:         fullHost,
		Handler:      withPProfHandler(newCatcher),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	err = server.ListenAndServe()

	if err != nil {
		fatalf("error listening on %s: %s\n", fullHost, err)
	}
}

func withPProfHandler(next http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	pprofHandler := basicAuth(mux, os.Getenv("PPROFPW"), "admin")

	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.Host == "requestcatcher.com" && strings.HasPrefix(req.URL.Path, "/debug/pprof") {
			pprofHandler.ServeHTTP(rw, req)
			return
		}
		next.ServeHTTP(rw, req)
	})
}

func basicAuth(handler http.Handler, password, realm string) http.Handler {
	p := []byte(password)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, pass, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(pass), p) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="`+realm+`"`)
			w.WriteHeader(401)
			//noinspection GoUnhandledErrorResult
			io.WriteString(w, "Unauthorized\n")
			return
		}
		handler.ServeHTTP(w, r)
	})
}

func fatalf(format string, args ...interface{}) {
	//noinspection GoUnhandledErrorResult
	fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(1)
}
