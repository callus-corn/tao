package http

import (
	"log/slog"
	"net/http"
	"os"
	"strings"
)

type HTTPConfig struct {
	IsEnable bool   `json:"IsEnable"`
	Address  string `json:"Address"`
	SrvDir   string `json:"SrvDir"`
}

var logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
var addr string
var srvDir = "./"

func Listen(c HTTPConfig) error {
	addr = c.Address
	srvDir = c.SrvDir
	go listen()
	return nil
}

func listen() {
	http.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Info("HTTP connection start from "+r.RemoteAddr, "module", "HTTP")
		upath := r.URL.Path
		if !strings.HasPrefix(upath, "/") {
			upath = "/" + upath
			r.URL.Path = upath
		}
		http.ServeFile(w, r, srvDir+upath)
	}))
	logger.Error(http.ListenAndServe(addr, nil).Error(), "module", "HTTP")
}
