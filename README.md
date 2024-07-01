# Dualface Project

[![Go Reference](https://pkg.go.dev/badge/github.com/libsdf/df.svg)](https://pkg.go.dev/github.com/libsdf/df)

## SOCKS5 Server

Setting up a socks5 server at the local side. eg. your laptop at home.

```
import (
    "context"
    "github.com/libsdf/df/conf"
    "github.com/libsdf/df/h2s"
    "github.com/libsdf/df/socks/socks5"
)

func main() {
    // Launch a http proxy server along side the socks5 server.
    go h2s.Server(context.Background(), &h2s.Conf{Port:3127, ProxyAddr:"localhost:1080"})
   
    params := make(conf.Values)
    params.Set(conf.FRAMER_PSK, "******") // encryption AES key
    params.Set(conf.SERVER_URL, "http://10.13.5.200:1081") // a transport server runs at the far side.

    options := &socks5.Options{
        Port: 1080,
        BackendProvider: func() backend.Backend {
            return backend.GetBackend("h1", params)
        },
    }
    socks5.Server(context.Background(), options)
}
```

You can implement your own [backend].Backend.


## Transport Server

The transport server is required if the SOCKS5 server uses a builtin tunnel backend. Transport server should be setup at the far side. eg. your workspace.

```
import (
    "github.com/libsdf/df/conf"
    "github.com/libsdf/df/transport"
    _ "github.com/libsdf/df/transport/h1"
)

func main() {
    params := make(conf.Values)
    params.Set(conf.FRAMER_PSK, "******") // encryption AES key
    params.Set(conf.PORT, "1081")
    transport.GetSuit("h1").Server(params)
}
```
 
Once the server and the transport server are both run well, you can do this then:

```
HTTP_PROXY=http://localhost:3127 HTTPS_PROXY=http://localhost:3127 curl -v https://www.google.com/
```