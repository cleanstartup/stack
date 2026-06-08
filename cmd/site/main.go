package main

import "github.com/cleanstartup/stack/web"

func main() {
	unsafe := true
	web.SiteAt("cmd/site",
		web.Styles("cmd/site"),
		web.Components("cmd/site"),
		web.SiteConfig(web.SiteOptions{
			Title:        "stack site demo",
			BaseURL:      "http://127.0.0.1:8080/",
			DisableKinds: []string{"taxonomy", "term"},
			MarkupUnsafe: &unsafe,
		}),
	)
}
