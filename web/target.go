package web

type TargetKind string

const (
	TargetApp  TargetKind = "app"
	TargetSite TargetKind = "site"
)

func (t TargetKind) String() string {
	if t == "" {
		return string(TargetApp)
	}
	return string(t)
}
