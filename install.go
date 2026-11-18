package main

// #include <unistd.h>
import "C"

import (
	"bufio"
	"errors"
	"fmt"
	"html"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	label    = "local.mic-priority"
	sysBin   = "/usr/local/bin/mic-priority"
	sysPlist = "/Library/LaunchAgents/" + label + ".plist"
	logHint  = "~/Library/Logs/mic-priority.log"
)

var (
	home, _   = os.UserHomeDir()
	userBin   = filepath.Join(home, ".local", "bin", "mic-priority")
	userPlist = filepath.Join(home, "Library", "LaunchAgents", label+".plist")
	confPath  = filepath.Join(home, ".config", "mic-priority.conf")
	logPath   = filepath.Join(home, "Library", "Logs", "mic-priority.log")
)

func service(uid int) string { return fmt.Sprintf("gui/%d/%s", uid, label) }

// plist has no log paths: a plist in /Library/LaunchAgents can't name a
// per-user file, so the agent opens ~/Library/Logs itself (see logToFile).
func plist(bin string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>` + label + `</string>
	<key>ProgramArguments</key>
	<array>
		<string>` + html.EscapeString(bin) + `</string>
		<string>run</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
</dict>
</plist>
`
}

func launchctl(args ...string) (string, error) {
	out, err := exec.Command("launchctl", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("launchctl %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// agentPID returns whether the agent is loaded for uid and its pid (0 if not running).
func agentPID(uid int) (loaded bool, pid int) {
	out, err := launchctl("print", service(uid))
	if err != nil {
		return false, 0
	}
	for _, line := range strings.Split(out, "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "pid = "); ok {
			pid, _ = strconv.Atoi(v)
			break
		}
	}
	return true, pid
}

// start (re)loads the agent from plist into uid's login session and waits until it runs.
func start(uid int, plist string) error {
	launchctl("bootout", service(uid)) // restart if already loaded; "not loaded" is fine
	// bootstrap can fail briefly while the old instance is still shutting down.
	for i := 0; ; i++ {
		_, err := launchctl("bootstrap", fmt.Sprintf("gui/%d", uid), plist)
		if err == nil {
			break
		}
		if i == 10 {
			return err
		}
		time.Sleep(300 * time.Millisecond)
	}
	for range 10 {
		if _, pid := agentPID(uid); pid > 0 {
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("agent is loaded but not running; see %s", logHint)
}

// writeFileAtomic writes via a temp file + rename. For binaries this matters:
// overwriting a signed executable in place gets it killed on the next launch.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	return writeAtomic(path, perm, func(w io.Writer) error {
		_, err := w.Write(data)
		return err
	})
}

func writeAtomic(path string, perm os.FileMode, fill func(io.Writer) error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".mic-priority-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename
	if err := fill(tmp); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func sameFile(a, b string) bool {
	sa, err1 := os.Stat(a)
	sb, err2 := os.Stat(b)
	return err1 == nil && err2 == nil && os.SameFile(sa, sb)
}

// copySelf installs this executable at dst unless it already is dst.
func copySelf(dst string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if sameFile(exe, dst) {
		return nil
	}
	src, err := os.Open(exe)
	if err != nil {
		return err
	}
	defer src.Close()
	return writeAtomic(dst, 0o755, func(w io.Writer) error {
		_, err := io.Copy(w, src)
		return err
	})
}

func removeAll(paths ...string) error {
	for _, p := range paths {
		if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return nil
}

// install installs for the current user only.
func install() error {
	if exists(sysPlist) {
		return errors.New("already installed for all users; to switch, run: mic-priority uninstall --all-users")
	}
	if err := copySelf(userBin); err != nil {
		return err
	}
	if err := ensureConfig(confPath); err != nil {
		return err
	}
	if err := writeFileAtomic(userPlist, []byte(plist(userBin)), 0o644); err != nil {
		return err
	}
	return start(os.Getuid(), userPlist)
}

func uninstall() error {
	if exists(sysPlist) && !exists(userPlist) {
		return errors.New("installed for all users; run: mic-priority uninstall --all-users")
	}
	launchctl("bootout", service(os.Getuid())) // "not loaded" is fine
	if err := removeAll(userPlist, userBin); err != nil {
		return err
	}
	fmt.Println("Uninstalled. Config kept:", tilde(confPath))
	return nil
}

// invokingUID is the user who asked for root: set by sudo, or by elevate().
func invokingUID() int {
	uid, _ := strconv.Atoi(os.Getenv("SUDO_UID"))
	return uid
}

// installAllUsers runs as root. Each user's agent creates its own config on first run.
func installAllUsers() error {
	// A strict caller umask (e.g. 077) would make a newly created /usr/local/bin
	// unreadable to the users the agent runs as.
	syscall.Umask(0o022)
	if err := copySelf(sysBin); err != nil {
		return err
	}
	if err := writeFileAtomic(sysPlist, []byte(plist(sysBin)), 0o644); err != nil {
		return err
	}
	uid := invokingUID()
	// The same label in ~/Library/LaunchAgents would clash, so drop the
	// invoking user's per-user install; start() replaces its running instance.
	if u, err := user.LookupId(strconv.Itoa(uid)); uid != 0 && err == nil {
		err := removeAll(filepath.Join(u.HomeDir, "Library", "LaunchAgents", label+".plist"),
			filepath.Join(u.HomeDir, ".local", "bin", "mic-priority"))
		if err != nil {
			return err
		}
	}
	// No GUI session to start in (plain root, or sudo over ssh): launchd starts it at login.
	if _, err := launchctl("print", fmt.Sprintf("gui/%d", uid)); uid == 0 || err != nil {
		fmt.Println("Installed for all users; it starts at each user's next login.")
		return nil
	}
	if err := start(uid, sysPlist); err != nil {
		return err
	}
	fmt.Println("Installed for all users. Other logged-in users get it at their next login.")
	return nil
}

// uninstallAllUsers runs as root.
func uninstallAllUsers() error {
	if uid := invokingUID(); uid != 0 {
		launchctl("bootout", service(uid)) // "not loaded" is fine
	}
	if err := removeAll(sysPlist, sysBin); err != nil {
		return err
	}
	fmt.Println("Uninstalled for all users; configs in ~/.config are kept. Other logged-in users' agents stop at their logout.")
	return nil
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

func appleQuote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

// elevate re-runs this binary with args as root via the standard macOS admin password dialog.
func elevate(prompt string, args ...string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := "SUDO_UID=" + strconv.Itoa(os.Getuid())
	for _, a := range append([]string{exe}, args...) {
		cmd += " " + shellQuote(a)
	}
	script := "do shell script " + appleQuote(cmd+" 2>&1") + " with prompt " + appleQuote(prompt) +
		" with administrator privileges without altering line endings"
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "(-128)") {
			return errors.New("cancelled")
		}
		return fmt.Errorf("%s (or run with sudo)", strings.TrimSpace(string(out)))
	}
	fmt.Print(string(out))
	return nil
}

func tilde(p string) string {
	if rest, ok := strings.CutPrefix(p, home+"/"); ok {
		return "~/" + rest
	}
	return p
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// configLines returns the non-comment lines of the config file.
func configLines(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return "(" + err.Error() + ")\n"
	}
	var out strings.Builder
	for _, line := range strings.Split(string(b), "\n") {
		if line = strings.TrimSpace(line); line != "" && line[0] != '#' {
			out.WriteString(line + "\n")
		}
	}
	return out.String()
}

func indent(s, prefix string) string {
	var b strings.Builder
	for _, line := range strings.SplitAfter(s, "\n") {
		if line != "" {
			b.WriteString(prefix + line)
		}
	}
	return b.String()
}

// installed reports the active install: all-users takes precedence, as at login.
func installed() (ok, allUsers bool, bin string) {
	if exists(sysPlist) {
		return true, true, sysBin
	}
	return exists(userPlist) && exists(userBin), false, userBin
}

func status() {
	_, allUsers, bin := installed()
	scope, reinstall := "for this user", "mic-priority install"
	if allUsers {
		scope, reinstall = "for all users", "mic-priority install --all-users"
	}
	loaded, pid := agentPID(os.Getuid())
	switch {
	case pid > 0:
		fmt.Printf("mic-priority is installed %s and running (pid %d).\n", scope, pid)
	case loaded:
		fmt.Printf("mic-priority is installed %s but not running; see %s\n", scope, logHint)
	default:
		fmt.Printf("mic-priority is installed %s but not running in this session; run: %s\n", scope, reinstall)
	}
	fmt.Printf("  binary: %s\n  config: %s (edits apply within ~2 s)\n  log:    %s\n  input:  %s\n\n",
		tilde(bin), tilde(confPath), tilde(logPath), currentInput(inputs()))
	fmt.Print("Priority:\n" + indent(configLines(confPath), "  "))
	if !inPath(filepath.Dir(bin)) {
		fmt.Printf("\nNote: %s is not in PATH; call the binary by its full path.\n", tilde(filepath.Dir(bin)))
	}
}

func inPath(dir string) bool {
	for _, p := range filepath.SplitList(os.Getenv("PATH")) {
		if filepath.Clean(p) == dir {
			return true
		}
	}
	return false
}

func interactive() {
	exe, err := os.Executable()
	check(err)
	if ok, allUsers, bin := installed(); ok {
		status()
		if !sameFile(exe, bin) {
			cmd := "install"
			if allUsers {
				cmd = "install --all-users"
			}
			fmt.Printf("\nThis binary (%s) is not the installed copy; run it with %q to update.\n", tilde(exe), cmd)
		}
		return
	}

	fmt.Print(usage)
	// isatty, not ModeCharDevice: /dev/null is a char device too.
	if C.isatty(0) == 0 || C.isatty(1) == 0 {
		fmt.Println("\nNot installed. Run: mic-priority install")
		return
	}
	fmt.Printf("\nNot installed. Install for this user will:\n  - copy this binary to %s\n", tilde(userBin))
	if exists(confPath) {
		fmt.Printf("  - keep the existing config %s:\n%s", tilde(confPath), indent(configLines(confPath), "      "))
	} else {
		fmt.Printf("  - create %s (edit it any time):\n%s", tilde(confPath), indent(deviceLines(inputs()), "      "))
	}
	fmt.Printf("  - start it now and at every login (LaunchAgent %s)\n", label)
	fmt.Print("For every user on this Mac instead, run: mic-priority install --all-users\n\nInstall for this user? [Y/n] ")

	answer, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if a := strings.ToLower(strings.TrimSpace(answer)); err != nil || a != "" && a != "y" && a != "yes" {
		fmt.Println("Not installed.")
		return
	}
	check(install())
	fmt.Println()
	status()
}
