package activity_test

import (
	"testing"

	"github.com/cleanstartup/stack/way2go/activity"
)

func TestPathFromIDRootMapsToSlash(t *testing.T) {
	got := activity.PathFromID("root")
	if got != "/" {
		t.Fatalf("expected / for root, got %q", got)
	}
}

func TestPathFromIDDotSeparatedToSlash(t *testing.T) {
	got := activity.PathFromID("backup.intro")
	if got != "/backup/intro" {
		t.Fatalf("expected /backup/intro, got %q", got)
	}
}
