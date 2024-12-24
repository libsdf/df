package main

import (
	_ "embed"
	"flag"
	"fmt"
	"github.com/libsdf/df/conf"
	"github.com/libsdf/df/log"
	"github.com/libsdf/df/transport"
	_ "github.com/libsdf/df/transport/d1"
	_ "github.com/libsdf/df/transport/h1"
	_ "github.com/libsdf/df/transport/h1pool"
)

//go:embed VERSION
var version string

type Conf struct {
	port int
	psk  string
}

func main() {
	logger := log.GetLoggerDefault()
	for _, h := range logger.GetHandlers() {
		h.SetFormat("$time [$mod] <$filename:$lineno> $lev* $message")
	}

	displayVersion := false
	debug := false
	useTLS := true
	cfg := &Conf{}
	flag.IntVar(&cfg.port, "p", 8080, "serving port.")
	flag.StringVar(&cfg.psk, "k", "", "psk")
	flag.BoolVar(&useTLS, "s", true, "serving in TLS mode.")
	flag.BoolVar(&debug, "d", false, "logging in debug level.")
	flag.BoolVar(&displayVersion, "V", false, "display version info.")
	flag.Parse()

	if debug {
		log.SetLevel(log.DEBUG)
	} else {
		log.SetLevel(log.INFO)
	}

	if displayVersion {
		println(fmt.Sprintf("client v%s", version))
		return
	}

	if cfg.port <= 0 {
		println("use -p to specify the serving port.")
		return
	}

	params := make(conf.Values)
	params.Set("port", fmt.Sprintf("%d", cfg.port))
	params.Set("framer_psk", cfg.psk)
	if useTLS {
		params.Set("tls", "yes")
	} else {
		params.Set("tls", "")
	}

	if err := transport.GetSuit("h1pool").Server(params); err != nil {
		log.Errorf("%v", err)
		return
	}

}
