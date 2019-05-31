package main

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"os"
	"strconv"
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
		Handler:      basicMiddleware(newCatcher, config),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	err = server.ListenAndServe()

	if err != nil {
		fatalf("error listening on %s: %s\n", fullHost, err)
	}
}

func basicMiddleware(next http.Handler, config *catcher.Configuration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if config.PostOnly == true && r.Method != "POST" {
			basicAuthHandler := basicAuth(next, []byte(config.User), []byte(config.Password), "logged-in")
			basicAuthHandler.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func basicAuth(handler http.Handler, username []byte, password []byte, realm string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(user), username) != 1 || subtle.ConstantTimeCompare([]byte(pass), password) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="`+realm+`"`)
			w.WriteHeader(401)
			//noinspection GoUnhandledErrorResult
			w.Write([]byte("Unauthorized.\n"))
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
