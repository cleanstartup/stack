package activity

import "strings"

type URIRef struct {
	id string
}

func Ref(id string) URIRef {
	id = strings.TrimSpace(id)
	if id == "" {
		panic("ref id must not be empty")
	}
	return URIRef{id: id}
}

func RootRef() URIRef { return Ref("root") }

func (r URIRef) ID() string {
	return r.id
}
