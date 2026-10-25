package device

import (
	"os"
	"testing"
)

func TestLabel(t *testing.T) {
	home, _ := os.UserHomeDir()
	cases := map[string]string{
		"/run/user/1000/gvfs/mtp:host=SAMSUNG_Android_X/Internal storage/Music": "Phone SAMSUNG Android X › Internal storage/Music",
		home + "/Music": "~/Music",
		"/mnt/code":     "/mnt/code",
	}
	for in, want := range cases {
		if got := Label(in); got != want {
			t.Errorf("Label(%q) = %q, want %q", in, got, want)
		}
	}
}
