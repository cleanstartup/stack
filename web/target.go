package web

type TargetKind string

const (
	TargetApp  TargetKind = "app"
	TargetHugo TargetKind = "hugo"
)

func (t TargetKind) String() string {
	if t == "" {
		return string(TargetApp)
	}
	return string(t)
}
