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
