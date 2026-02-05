package depextify

import (
	"maps"
	"slices"
)

// Ignore shell built-in commands
var builtins = func() map[string]bool {
	keys := []string{
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
	}

	b := make(map[string]bool, len(keys))

	for _, k := range keys {
		b[k] = true
	}

	return b
}()

var coreutils = map[string]bool{
	"arch": true, "b2sum": true, "base32": true, "base64": true, "basename": true,
	"basenc": true, "cat": true, "chcon": true, "chgrp": true, "chmod": true,
	"chown": true, "chroot": true, "cksum": true, "comm": true, "cp": true,
	"csplit": true, "cut": true, "date": true, "dd": true, "df": true,
	"dir": true, "dircolors": true, "dirname": true, "du": true, "expand": true,
	"expr": true, "factor": true, "fmt": true, "fold": true, "groups": true,
	"head": true, "hostid": true, "id": true, "install": true, "join": true,
	"link": true, "ln": true, "logname": true, "ls": true, "md5sum": true,
	"mkdir": true, "mkfifo": true, "mknod": true, "mktemp": true, "mv": true,
	"nice": true, "nl": true, "nohup": true, "nproc": true, "numfmt": true,
	"od": true, "paste": true, "pathchk": true, "pinky": true, "pr": true,
	"printenv": true, "ptx": true, "readlink": true, "realpath": true, "rm": true,
	"rmdir": true, "runcon": true, "seq": true, "sha1sum": true, "sha224sum": true,
	"sha256sum": true, "sha384sum": true, "sha512sum": true, "shred": true,
	"shuf": true, "sleep": true, "sort": true, "split": true, "stat": true,
	"stdbuf": true, "stty": true, "sum": true, "sync": true, "tac": true,
	"tail": true, "tee": true, "timeout": true, "touch": true, "tr": true,
	"truncate": true, "tsort": true, "tty": true, "uname": true, "unexpand": true,
	"uniq": true, "unlink": true, "uptime": true, "users": true, "vdir": true,
	"wc": true, "who": true, "whoami": true, "yes": true,
}

var common = map[string]bool{
	"awk": true, "grep": true, "egrep": true, "fgrep": true, "sed": true,
	"find": true, "xargs": true, "diff": true, "patch": true, "tar": true,
	"gzip": true, "gunzip": true, "bzip2": true, "bunzip2": true, "xz": true,
	"unxz": true, "zip": true, "unzip": true, "ssh": true, "scp": true,
	"rsync": true, "curl": true, "wget": true, "git": true, "make": true,
	"sudo": true, "apt": true, "apt-get": true, "dpkg": true, "ps": true,
	"top": true, "htop": true, "killall": true, "mount": true, "umount": true,
	"df": true, "du": true, "free": true, "lscpu": true, "lsblk": true,
	"lsusb": true, "lspci": true, "ip": true, "ifconfig": true, "ping": true,
	"netstat": true, "ss": true, "traceroute": true, "dig": true, "host": true,
	"nslookup": true, "hostname": true, "man": true, "info": true, "less": true,
	"more": true, "nano": true, "vim": true, "vi": true, "emacs": true,
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
