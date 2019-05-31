package catcher

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/op/go-logging"
)

type Catcher struct {
	config   *Configuration
	router   *mux.Router
	upgrader websocket.Upgrader
	logger   *logging.Logger

	hostsMu sync.Mutex
	hosts   map[string]*Host
}

//noinspection GoUnusedExportedFunction
func NewCatcher(config *Configuration) *Catcher {
	c := &Catcher{
		config: config,
		router: mux.NewRouter(),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		logger: logging.MustGetLogger("request-catcher"),

		hosts: make(map[string]*Host),
	}
	c.router.HandleFunc("/", c.rootHandler).Host(c.config.RootHost)
	c.router.HandleFunc("/", c.indexHandler)
	c.router.HandleFunc("/init-client", c.initClient)
	c.router.PathPrefix("/assets").Handler(http.StripPrefix("/assets",
		withCacheHeaders(http.FileServer(http.Dir("frontend/dist")))))
	c.router.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "frontend/favicon.ico")
	})
	c.router.NotFoundHandler = http.HandlerFunc(c.catchRequests)
	return c
}

func withCacheHeaders(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		oneYear := time.Now().Add(time.Hour * 24 * 365).Format(time.RFC3339)
		w.Header().Set("Cache-Control", "public, max-age=31536000")
		w.Header().Set("Expires", oneYear)
		h.ServeHTTP(w, r)
	})
}

func (c *Catcher) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if strings.HasPrefix(req.Host, "www.") {
		rw.Header().Set("Connection", "close")
		url := "http://" + strings.TrimPrefix(req.Host, "www.") + req.URL.String()
		http.Redirect(rw, req, url, http.StatusMovedPermanently)
		return
	}

	c.router.ServeHTTP(rw, req)
}

func (c *Catcher) host(hostString string) *Host {
	hostString = hostWithoutPort(hostString)

	c.hostsMu.Lock()
	defer c.hostsMu.Unlock()
	if host, ok := c.hosts[hostString]; ok {
		return host
	}
	host := newHost(hostString)
	c.hosts[hostString] = host
	return host
}

func (c *Catcher) rootHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "frontend/dist/root.html")
}

func (c *Catcher) indexHandler(w http.ResponseWriter, r *http.Request) {
	// Some people mistakenly expect requests to the index of the subdomain
	// to be caught. For now, just catch those as well. Later I should move
	// the index to be hosted at requestcatcher.com.

	http.ServeFile(w, r, "frontend/dist/index.html")
}

func (c *Catcher) catchRequests(w http.ResponseWriter, r *http.Request) {
	c.Catch(r)

	// Respond to the request
	fmt.Fprintf(w, "ok")
}

func (c *Catcher) initClient(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	hostString := hostWithoutPort(r.Host)
	_, a := c.hosts[hostString]

	clientHost := c.host(r.Host)

	if c.config.AllowMultiple == false && a == true {
		http.Error(w, "Method not allowed", 405)
		return
	}

	ws, err := c.upgrader.Upgrade(w, r, nil)
	if err != nil {
		c.logger.Error(err.Error())
		return
	}

	c.logger.Infof("Initializing a new client on host %v", clientHost.Host)
	clientHost.clients.Store(c, newClient(c, clientHost, ws))
}

func (c *Catcher) Catch(r *http.Request) {
	hostString := hostWithoutPort(r.Host)
	c.hostsMu.Lock()
	host, ok := c.hosts[hostString]
	c.hostsMu.Unlock()

	if !ok {
		// No one is listening, so no reason to catch it.
		return
	}

	// Broadcast it to everyone listening for requests on this host
	caughtRequest := convertRequest(r)
	host.broadcast <- caughtRequest
}
