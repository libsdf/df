package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/libsdf/df/conf"
	"github.com/libsdf/df/h2s"
	"github.com/libsdf/df/log"
	"github.com/libsdf/df/socks/backend"
	"github.com/libsdf/df/socks/socks5"
	_ "github.com/libsdf/df/transport/d1"
	_ "github.com/libsdf/df/transport/h1"
	_ "github.com/libsdf/df/transport/h1pool"
	"net/url"
	"os"
	"path/filepath"
)

//go:embed VERSION
var version string

type Conf struct {
	PortLocal int       `json:"port"`
	ServerUrl string    `json:"server_url"`
	Psk       string    `json:"psk"`
	Http      *h2s.Conf `json:"http"`
}

func isValidUrl(urlStr string) bool {
	_, err := url.Parse(urlStr)
	if err != nil {
		return false
	}
	return true
}

func main() {
	logger := log.GetLoggerDefault()
	for _, h := range logger.GetHandlers() {
		h.SetFormat("$time [$mod] <$filename:$lineno> $lev* $message")
	}

	displayVersion := false
	debug := false
	cfg := &Conf{}
	cfgPath := ""
	flag.IntVar(&cfg.PortLocal, "p", 0, "local port.")
	flag.StringVar(&cfg.ServerUrl, "s", "", "server url")
	flag.StringVar(&cfg.Psk, "k", "", "psk")
	flag.StringVar(&cfgPath, "c", "config.json", "path to config.json.")
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

	if len(cfgPath) > 0 {
		cfgBase := &Conf{}
		if dat, err := os.ReadFile(cfgPath); err != nil {
			log.Warnf("loading %s: %v", cfgPath, err)
		} else {
			if err := json.Unmarshal(dat, cfgBase); err != nil {
				log.Warnf("json.Unmarshal: %v", err)
			} else {
				log.Infof("%s loaded.", filepath.Base(cfgPath))
				if cfg.PortLocal <= 0 {
					cfg.PortLocal = cfgBase.PortLocal
				}
				if len(cfg.ServerUrl) <= 0 {
					cfg.ServerUrl = cfgBase.ServerUrl
				}
				if len(cfg.Psk) <= 0 {
					cfg.Psk = cfgBase.Psk
				}
				cfg.Http = cfgBase.Http
			}
		}
	}

	// if cfg.PortLocal <= 0 {
	// 	println("use -p to specify the local serving port.")
	// 	return
	// }

	// if !isValidUrl(cfg.ServerUrl) {
	// 	println("invalid server URL.")
	// 	return
	// }

	x, cancel := context.WithCancel(context.Background())
	defer cancel()

	go backend.CacheWorker(x)

	chFatal := make(chan int, 2)

	go func() {
		h2s.Server(x, cfg.Http)
		chFatal <- 1
	}()

	if cfg.PortLocal > 0 {
		go func() {
			params := make(conf.Values)
			params.Set("framer_psk", cfg.Psk)
			params.Set("server_url", cfg.ServerUrl)

			options := &socks5.Options{
				Port: cfg.PortLocal,
				BackendProvider: func() backend.Backend {
					return backend.GetBackend("h1pool", params)
				},
			}
			if err := socks5.Server(x, options); err != nil {
				log.Errorf("%v", err)
			}
			chFatal <- 1
		}()
	}

	select {
	case <-chFatal:
		return
	case <-x.Done():
	}

}
