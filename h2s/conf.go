package h2s

type Conf struct {
	Port      int    `json:"port"`
	ProxyAddr string `json:"proxy_addr"`
}
