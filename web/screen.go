package web

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"strings"

	"github.com/a-h/templ"
)

// Screen renders a Stencil-backed screen custom element with serialized props.
// The helper keeps the return type as templ.Component so activities can return
// it directly without introducing a new result type.
func Screen(name string, props any) templ.Component {
	tag := strings.TrimSpace(name)
	if tag == "" {
		panic("screen name must not be empty")
	}
	if !strings.HasPrefix(tag, "screen-") {
		tag = "screen-" + tag
	}
	return Element(tag, props)
}

// Element renders a Stencil-backed custom element with optional serialized props.
func Element(name string, props any) templ.Component {
	tag := strings.TrimSpace(name)
	if tag == "" {
		panic("element name must not be empty")
	}
	return templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		if _, err := io.WriteString(w, "<"+tag); err != nil {
			return err
		}
		if props != nil {
			encoded, err := json.Marshal(props)
			if err != nil {
				return fmt.Errorf("marshal screen props: %w", err)
			}
			if _, err := io.WriteString(w, " data-screen-props=\""+html.EscapeString(string(encoded))+"\""); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(w, "></"+tag+"><script>(()=>{const el=document.currentScript?.previousElementSibling;if(!(el instanceof HTMLElement))return;const raw=el.getAttribute('data-screen-props')||'{}';let props={};try{props=JSON.parse(raw)}catch(_){return}Object.assign(el,props)})()</script>"); err != nil {
			return err
		}
		return nil
	})
}
