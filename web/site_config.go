package web

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type SiteOptions struct {
	Title        string
	BaseURL      string
	DisableKinds []string
	Params       map[string]string
	MarkupUnsafe *bool
}

type siteOptionsPart struct {
	opts SiteOptions
}

func SiteConfig(opts SiteOptions) Part {
	return siteOptionsPart{opts: opts}
}

func (p siteOptionsPart) Apply(app *WebApp) {
	if app == nil {
		return
	}
	app.registerSiteConfig(p.opts)
}

func (a *WebApp) registerSiteConfig(opts SiteOptions) {
	if a == nil {
		return
	}
	if a.siteConfig == nil {
		a.siteConfig = &SiteOptions{}
	}
	if strings.TrimSpace(opts.Title) != "" {
		a.siteConfig.Title = strings.TrimSpace(opts.Title)
	}
	if strings.TrimSpace(opts.BaseURL) != "" {
		a.siteConfig.BaseURL = strings.TrimSpace(opts.BaseURL)
	}
	if len(opts.DisableKinds) > 0 {
		a.siteConfig.DisableKinds = mergeStrings(a.siteConfig.DisableKinds, opts.DisableKinds)
	}
	if len(opts.Params) > 0 {
		if a.siteConfig.Params == nil {
			a.siteConfig.Params = map[string]string{}
		}
		for key, value := range opts.Params {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			a.siteConfig.Params[key] = value
		}
	}
	if opts.MarkupUnsafe != nil {
		if a.siteConfig.MarkupUnsafe == nil {
			a.siteConfig.MarkupUnsafe = new(bool)
		}
		*a.siteConfig.MarkupUnsafe = *opts.MarkupUnsafe
	}
}

func (a *WebApp) SiteConfig() *SiteOptions {
	if a == nil || a.siteConfig == nil {
		return nil
	}
	out := *a.siteConfig
	if len(out.DisableKinds) > 0 {
		out.DisableKinds = append([]string{}, out.DisableKinds...)
	}
	if len(out.Params) > 0 {
		out.Params = make(map[string]string, len(out.Params))
		for key, value := range a.siteConfig.Params {
			out.Params[key] = value
		}
	}
	if a.siteConfig.MarkupUnsafe != nil {
		val := *a.siteConfig.MarkupUnsafe
		out.MarkupUnsafe = &val
	}
	return &out
}

func siteConfigToml(opts SiteOptions) (string, error) {
	var b strings.Builder
	if strings.TrimSpace(opts.Title) != "" {
		fmt.Fprintf(&b, "title = %s\n", tomlString(opts.Title))
	}
	if strings.TrimSpace(opts.BaseURL) != "" {
		fmt.Fprintf(&b, "baseURL = %s\n", tomlString(opts.BaseURL))
	}
	if len(opts.DisableKinds) > 0 {
		b.WriteString("disableKinds = [")
		for i, kind := range mergeStrings(nil, opts.DisableKinds) {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(tomlString(kind))
		}
		b.WriteString("]\n")
	}
	if len(opts.Params) > 0 {
		keys := make([]string, 0, len(opts.Params))
		for key := range opts.Params {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("[params]\n")
		for _, key := range keys {
			value := strings.TrimSpace(opts.Params[key])
			fmt.Fprintf(&b, "%s = %s\n", key, tomlString(value))
		}
	}
	if opts.MarkupUnsafe != nil {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("[markup.goldmark.renderer]\n")
		if *opts.MarkupUnsafe {
			b.WriteString("unsafe = true\n")
		} else {
			b.WriteString("unsafe = false\n")
		}
	}
	return b.String(), nil
}

func mergeStrings(dst []string, src []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(dst)+len(src))
	for _, value := range dst {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	for _, value := range src {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func tomlString(value string) string {
	return strconv.Quote(value)
}
