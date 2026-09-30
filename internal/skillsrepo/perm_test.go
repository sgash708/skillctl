package skillsrepo

import (
	"os"
	"runtime"
)

// cannotTestPermissions reports whether chmod-based permission errors cannot be
// provoked here: Windows ignores POSIX permission bits, and root bypasses them.
func cannotTestPermissions() bool {
	return runtime.GOOS == "windows" || os.Geteuid() == 0
}
