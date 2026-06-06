package web

import (
	"net/http"
)

func RegisterAllDefault(modules ...LegacyModule) {
	for _, module := range modules {
		if module == nil {
			continue
		}
		module.register(defaultRegistry)
	}
}

func LegacyRun(addr string, modules ...LegacyModule) error {
	Reset()
	RegisterAllDefault(modules...)
	return http.ListenAndServe(addr, DefaultHandler())
}
