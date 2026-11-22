package depextify

import (
	"maps"
	"slices"
)

func set(keys ...string) map[string]bool {
	m := make(map[string]bool, len(keys))
	for _, k := range keys {
		m[k] = true
	}
	return m
}

// Ignore shell built-in commands
var builtins = set(
	"!", "(", ")", ".", ":", "[", "[[", "]]", "{", "}",
	"alias", "autoload", "bg", "bind", "bindkey", "break", "builtin", "bye",
	"case", "cd", "chdir", "command", "comparguments", "compcall", "compctl",
	"compdescribe", "compfiles", "compgroups", "compquote", "comptags",
	"comptry", "compvalues", "continue", "declare", "dirs", "disable",
	"disown", "do", "done", "echo", "echotc", "echoti", "elif", "else",
	"emulate", "enable", "esac", "eval", "exec", "exit", "export", "false",
	"fc", "fg", "fi", "for", "function", "functions", "getcap", "getln",
	"getopts", "hash", "help", "history", "if", "integer", "jobs", "kill",
	"let", "limit", "local", "log", "logout", "noglob", "popd", "print",
	"printf", "pushd", "pushln", "pwd", "read", "readonly", "rehash",
	"return", "sched", "select", "set", "setopt", "shift", "source", "stat",
	"suspend", "test", "then", "times", "trap", "true", "ttyctl", "type",
	"typeset", "ulimit", "umask", "unalias", "unfunction", "unhash",
	"unlimit", "unset", "unsetopt", "until", "vared", "wait", "whence",
	"where", "while", "which", "zcompile", "zformat", "zftp", "zle",
	"zmodload", "zparseopts", "zprof", "zpty", "zregexparse", "zstat", "ztcp",
	"zstyle", "add-zsh-hook", "compaudit", "compinit",
)

var coreutils = set(
	"arch", "b2sum", "base32", "base64", "basename",
	"basenc", "cat", "chcon", "chgrp", "chmod",
	"chown", "chroot", "cksum", "comm", "cp",
	"csplit", "cut", "date", "dd", "df",
	"dir", "dircolors", "dirname", "du", "expand",
	"expr", "factor", "fmt", "fold", "groups",
	"head", "hostid", "id", "install", "join",
	"link", "ln", "logname", "ls", "md5sum",
	"mkdir", "mkfifo", "mknod", "mktemp", "mv",
	"nice", "nl", "nohup", "nproc", "numfmt",
	"od", "paste", "pathchk", "pinky", "pr",
	"printenv", "ptx", "readlink", "realpath", "rm",
	"rmdir", "runcon", "seq", "sha1sum", "sha224sum",
	"sha256sum", "sha384sum", "sha512sum", "shred",
	"shuf", "sleep", "sort", "split", "stat",
	"stdbuf", "stty", "sum", "sync", "tac",
	"tail", "tee", "timeout", "touch", "tr",
	"truncate", "tsort", "tty", "uname", "unexpand",
	"uniq", "unlink", "uptime", "users", "vdir",
	"wc", "who", "whoami", "yes",
)

var common = set(
	"awk", "grep", "egrep", "fgrep", "sed",
	"find", "xargs", "diff", "patch", "tar",
	"gzip", "gunzip", "bzip2", "bunzip2", "xz",
	"unxz", "zip", "unzip", "ssh", "scp",
	"rsync", "curl", "wget", "git", "make",
	"sudo", "apt", "apt-get", "dpkg", "ps",
	"top", "htop", "killall", "mount", "umount",
	"df", "du", "free", "lscpu", "lsblk",
	"lsusb", "lspci", "ip", "ifconfig", "ping",
	"netstat", "ss", "traceroute", "dig", "host",
	"nslookup", "hostname", "man", "info", "less",
	"more", "nano", "vim", "vi", "emacs",
)

// hints describe differences between versions or variants of a command that affect portability.
var hints = func() map[string]string {
	h := map[string]string{
		"python":         "may run Python 2 or Python 3, or be missing, depending on the system; call python3 explicitly",
		"python2":        "Python 2 is end-of-life and missing from most current systems",
		"pip":            "may belong to Python 2 or Python 3; use python3 -m pip",
		"awk":            "may be gawk, mawk or BSD awk depending on the system; gawk extensions are not portable",
		"timeout":        "comes from GNU coreutils and is missing on macOS by default; Homebrew installs it as gtimeout",
		"nc":             "netcat variants (OpenBSD, GNU, Nmap ncat) accept different options",
		"docker-compose": "is the Compose V1 command, which is no longer maintained; Compose V2 runs as docker compose",
		"which":          "is not specified by POSIX and its output differs between systems; command -v is portable",
		"echo":           "handles -n, -e and backslashes differently between shells; printf is portable",
	}
	// GNU tools whose BSD (macOS) counterparts differ; Homebrew installs the GNU versions with a g prefix.
	gnu := map[string]string{
		"grep": "", "find": "", "xargs": "", "readlink": "", "tar": "",
		"sed":  ", for example in sed -i",
		"date": ", for example in parsing dates (-d and -j -f)",
		"stat": ", for example in format options (-c and -f)",
	}
	for cmd, example := range gnu {
		h[cmd] = "GNU and BSD (macOS) versions accept different options" + example + "; Homebrew installs the GNU version as g" + cmd
	}
	gnu["timeout"] = ""
	for cmd := range gnu {
		h["g"+cmd] = "is Homebrew's name for GNU " + cmd + "; GNU/Linux systems call it " + cmd
	}
	return h
}()

// GetHint returns a note on how versions or variants of cmd differ between systems, or "" when there is none.
func GetHint(cmd string) string {
	return hints[cmd]
}

// GetBuiltins returns a sorted list of shell built-in commands.
func GetBuiltins() []string {
	return slices.Sorted(maps.Keys(builtins))
}

// GetCoreutils returns a sorted list of GNU Coreutils commands.
func GetCoreutils() []string {
	return slices.Sorted(maps.Keys(coreutils))
}

// GetCommon returns a sorted list of common shell commands.
func GetCommon() []string {
	return slices.Sorted(maps.Keys(common))
}
