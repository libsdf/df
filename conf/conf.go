package conf

import (
	"strconv"
)

type Values map[string]string

func (v Values) Clone() Values {
	d := make(map[string]string, len(v))
	for name, val := range v {
		d[name] = val
	}
	return d
}

func (v Values) Set(name, value string) {
	v[name] = value
}

func (v Values) Get(name string) string {
	if val, found := v[name]; found {
		return val
	}
	return ""
}

func (v Values) GetInt(name string) int {
	if val, found := v[name]; found {
		if iv, err := strconv.Atoi(val); err == nil {
			return iv
		}
	}
	return 0
}

func (v Values) GetInt64(name string) int64 {
	if val, found := v[name]; found {
		if iv, err := strconv.ParseInt(val, 10, 64); err == nil {
			return iv
		}
	}
	return 0
}
