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
	cfg := &Conf{}
	flag.IntVar(&cfg.port, "p", 8080, "serving port.")
	flag.StringVar(&cfg.psk, "k", "", "psk")
	flag.BoolVar(&displayVersion, "V", false, "display version info.")
	flag.Parse()

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

	if err := transport.GetSuit("h1").Server(params); err != nil {
		log.Errorf("%v", err)
		return
	}

}
