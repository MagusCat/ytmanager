package device

import (
	"bufio"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// /proc/mounts escapes spaces, tabs and backslashes in paths.
var unescape = strings.NewReplacer(`\040`, " ", `\011`, "\t", `\134`, `\`)

// drives lists what udisks or the user mounted under /run/media, /media and /mnt.
func drives() []string {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		p := unescape.Replace(fields[1])
		for _, pre := range []string{"/run/media/", "/media/", "/mnt/"} {
			if strings.HasPrefix(p, pre) {
				out = append(out, p)
			}
		}
	}
	return out
}

func gvfsRoot() string { return fmt.Sprintf("/run/user/%d/gvfs", os.Getuid()) }

// phoneStorages lists MTP storages mounted by gvfs: <gvfs>/mtp:host=<phone>/<storage>.
func phoneStorages() []string {
	out, _ := filepath.Glob(filepath.Join(gvfsRoot(), "mtp:host=*", "*"))
	return out
}

// MountPhones mounts connected MTP phones that gvfs has not mounted yet. If the phone shows an
// "allow access" prompt, gio waits for it; the timeout keeps detection from hanging.
func MountPhones() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "gio", "mount", "-li").Output()
	if err != nil {
		return
	}
	for _, l := range strings.Split(string(out), "\n") {
		uri, ok := strings.CutPrefix(strings.TrimSpace(l), "activation_root=")
		if !ok || !strings.HasPrefix(uri, "mtp://") {
			continue
		}
		host := strings.Trim(strings.TrimPrefix(uri, "mtp://"), "/")
		if _, err := os.Stat(filepath.Join(gvfsRoot(), "mtp:host="+host)); err != nil {
			_ = exec.CommandContext(ctx, "gio", "mount", uri).Run()
		}
	}
}

// resolveMTP maps mtp://HOST/path to the gvfs FUSE mount; without this the URI would be
// created as a local ./mtp:/ folder.
func resolveMTP(u *url.URL) (string, error) {
	p := filepath.Join(gvfsRoot(), "mtp:host="+u.Host, u.Path)
	if _, err := os.Stat(filepath.Dir(p)); err != nil {
		return "", fmt.Errorf("phone not mounted: open it in your file manager or run gio mount mtp://%s/", u.Host)
	}
	return p, nil
}

var phoneDir = regexp.MustCompile(`^/run/user/\d+/gvfs/mtp:host=([^/]+)/?`)

func phoneLabel(p string) (name, rest string, ok bool) {
	m := phoneDir.FindStringSubmatch(p)
	if m == nil {
		return "", "", false
	}
	return strings.ReplaceAll(m[1], "_", " "), p[len(m[0]):], true
}
