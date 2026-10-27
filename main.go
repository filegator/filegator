package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/filegator/filegator/internal/app"
	"github.com/filegator/filegator/internal/auth/jsonfile"
	"github.com/filegator/filegator/internal/config"
	"github.com/filegator/filegator/internal/router"
	"github.com/filegator/filegator/internal/version"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// canAnswer reports whether the first-start question can be asked on in.
//
// It replaces a terminal ioctl with the standard library, which works on every
// platform the project builds for. A pipe counts, so `printf 'y\n' | filegator`
// works in a script; a regular file does not, because nothing there can answer.
// A character device is the terminal, except stdin on /dev/null, which is what
// a service unit and a container hand over: asking there would block forever.
func canAnswer(in *os.File) bool {
	info, err := in.Stat()
	if err != nil {
		return false
	}
	if info.Mode()&os.ModeNamedPipe != 0 {
		return true
	}
	if info.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	target, err := os.Readlink("/proc/self/fd/0")
	return err != nil || target != os.DevNull
}

// askYesNo reads one answer. Anything but y/yes — including pressing Enter —
// means no, so the server never creates files by accident.
func askYesNo(_ string) (bool, error) {
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return false, err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}

// run is main without the exit, so a test can drive the flags. It returns the
// process exit code: 0 after -h, -export-frontend or -reset-admin-password,
// 2 for a bad command line, 1 for a failure while starting.
func run(args []string, stdout, stderr io.Writer) int {
	fs, opts := newFlagSet(stderr)
	// -h is checked before Parse because flag.Parse prints the screen itself
	// when it meets one: handling ErrHelp afterwards printed it twice.
	for _, a := range args {
		if a == "--" {
			break
		}
		if a == "-h" || a == "-help" || a == "--help" {
			usage(stdout, fs)
			return 0
		}
	}
	if err := fs.Parse(args); err != nil {
		// A bad invocation explains itself on stderr: flag writes the reason and
		// calls the Usage function, which writes to the set's own output.
		return 2
	}
	if fs.NArg() > 0 {
		// Running with no flags starts the server, so a stray word is almost
		// always a forgotten flag name: -addr written as a bare argument would
		// otherwise be ignored and the server would listen somewhere else.
		fmt.Fprintf(stderr, "unexpected argument %q; flags come before it, e.g. -addr 127.0.0.1:8080\n\n", fs.Arg(0))
		usage(stderr, fs)
		return 2
	}

	// Listing the administrators answers "the reset says no such account, so
	// which ones are there?" — the same shell access, and no password changed.
	if opts.listAdmins {
		if err := app.ListAdmins(opts.configPath, stdout); err != nil {
			fmt.Fprintln(stderr, "list admins:", err)
			return 1
		}
		return 0
	}

	// Resetting a password needs the config (to find the users file) but not a
	// socket or storage, so it comes before either is opened.
	if opts.resetUser != "" {
		if err := app.ResetAdminPassword(opts.configPath, opts.resetUser, os.Stdin, stdout); err != nil {
			fmt.Fprintln(stderr, "reset password:", err)
			return 1
		}
		return 0
	}

	// Exporting needs no configuration, no storage and no listening socket, so
	// it happens before any of that is set up.
	if opts.export != "" {
		written, err := app.ExportFrontend(opts.export)
		if err != nil {
			log.Fatalf("%v", err)
		}
		log.Printf("wrote %d files to %s; start with -frontend-dir %s to use them", written, opts.export, opts.export)
		return 0
	}

	cfg, err := config.Load(opts.configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	// The prefix is a property of the invocation, not of the configuration:
	// one instance's mount point says nothing about another's, and it is the
	// proxy or container that decides where the app lives. It is therefore
	// never written to config.json.
	if err := router.ValidateBasePath(opts.basePath); err != nil {
		log.Fatalf("base path: %v", err)
	}
	if normalized := router.NormalizeBasePath(opts.basePath); normalized != "" {
		log.Printf("base path %q", normalized)
	}

	// Like -base-path, this applies to one run and is not written back: which
	// copy of the interface to serve is a deployment choice.
	if opts.frontendDir != "" {
		cfg = cfg.WithFrontendDir(opts.frontendDir)
	}

	// Creating private/, config.json, users.json and the storage root is a
	// decision, not a side effect: a binary started in the wrong folder used to
	// produce a configured instance nobody asked for. Ask, or take the flag.
	req := app.InitRequest{
		ConfigPath:  cfg.Path(),
		UsersFile:   cfg.Get().Auth.UsersFile,
		StorageRoot: app.LocalRoot(cfg.Get().Storage),
	}
	// The question is only asked when someone can answer it: a terminal, or a
	// pipe a script writes "y" into. stdin on /dev/null (systemd, a container)
	// will never produce an answer, so asking there would block the start.
	//
	// Refusing ends the run: a server that wrote nothing would not be the one
	// the operator asked for, and answering "n" is a decision about the folder,
	// not a request for a different kind of server.
	create, reason := app.ResolveInit(opts.initMode(), req, stdout, canAnswer(os.Stdin), askYesNo)
	if !create {
		fmt.Fprintf(stderr, "cannot start: %s\n", reason)
		return 1
	}

	a, err := app.New(cfg, opts.basePath)
	if err != nil {
		if errors.Is(err, jsonfile.ErrReadOnly) {
			log.Fatalf("cannot start: %v\n"+
				"FileGator needs to write the users file so password and permission changes are saved.\n"+
				"Fix the permissions or mount the volume read-write.", err)
		}
		log.Fatalf("init app: %v", err)
	}

	defer a.Close()

	if err := a.ListenAndServe(opts.addr); err != nil {
		log.Fatalf("server: %v", err)
	}
	return 0
}

// newFlagSet builds the command line. It is a function rather than inline in
// run so a test can parse it without starting a server or touching a config
// file.
//
// out receives the flag list: os.Stderr for a bad invocation (a help screen on
// stdout would be piped into the next command), os.Stdout for -h.
func newFlagSet(out io.Writer) (fs *flag.FlagSet, opts *options) {
	fs = flag.NewFlagSet("filegator", flag.ContinueOnError)
	fs.SetOutput(out)
	fs.Usage = func() { usage(out, fs) }

	opts = &options{}
	fs.StringVar(&opts.addr, "addr", "127.0.0.1:8080", "HTTP listen address")
	fs.StringVar(&opts.configPath, "config", "private/config.json", "path to configuration file")
	fs.StringVar(&opts.basePath, "base-path", "", "URL prefix the app is served under, e.g. /browser (default: the site root)")
	fs.StringVar(&opts.frontendDir, "frontend-dir", "", "serve the interface from this folder instead of the built-in copy")
	fs.StringVar(&opts.export, "export-frontend", "", "write the built-in interface to this folder and exit, for editing with -frontend-dir")
	fs.StringVar(&opts.resetUser, "reset-admin-password", "", "set this account's password and exit (needs shell access; the server must be restarted after)")
	fs.BoolVar(&opts.listAdmins, "list-admins", false, "print the administrator accounts and exit, for when the username is what was forgotten")
	fs.BoolVar(&opts.init, "init", false, "create the folders and files a first start needs, without asking (for scripts, containers and service units)")
	return fs, opts
}

// initMode maps the flag onto the decision the server takes.
func (o *options) initMode() app.InitMode {
	if o.init {
		return app.InitYes
	}
	return app.InitAsk
}

// options holds the parsed command line.
type options struct {
	addr        string
	configPath  string
	basePath    string
	frontendDir string
	export      string
	resetUser   string
	listAdmins  bool
	init        bool
}

// usage prints what the binary is for before listing the flags. Without the
// introduction, -h was a wall of lowercase flag names with no statement that
// running with no arguments starts the server, which is the thing a first-time
// reader is trying to find out.
func usage(w io.Writer, fs *flag.FlagSet) {
	// The version is the first thing someone asks when reporting a problem or
	// checking whether an upgrade took, so it belongs on the help screen rather
	// than only in the admin area and version.json.
	fmt.Fprintf(w, "FileGator %s - a web file manager.\n\n", version.Version)
	fmt.Fprintf(w, "Usage:\n")
	fmt.Fprintf(w, "  filegator [flags]                     serve the web interface (the default)\n")
	fmt.Fprintf(w, "  filegator -export-frontend <dir>      write the built-in interface out and exit\n")
	fmt.Fprintf(w, "  filegator -reset-admin-password <u>   set a password and exit\n")
	fmt.Fprintf(w, "  filegator -list-admins                print the administrator accounts and exit\n\n")
	fmt.Fprintf(w, "The first start in an empty folder asks before it creates private/, repository/,\n")
	fmt.Fprintf(w, "config.json and users.json; answering no stops it. Use -init to create them without\n")
	fmt.Fprintf(w, "asking, for containers and service units.\n\n")
	fmt.Fprintf(w, "Flags:\n")
	// PrintDefaults writes to the set's own output, not to w, so the flag list
	// has to be aimed at w before it is printed or it lands in the other
	// stream: -h sent the prose to stdout and every flag to stderr.
	fs.SetOutput(w)
	fs.PrintDefaults()
	// The set is owned by the caller and keeps being parsed after a help
	// screen, so its output is restored rather than left pointed at w.
	fs.SetOutput(os.Stderr)
	fmt.Fprintf(w, "\nExamples:\n")
	fmt.Fprintf(w, "  filegator\n")
	fmt.Fprintf(w, "  filegator -addr 0.0.0.0:8080 -config /etc/filegator/config.json\n")
	fmt.Fprintf(w, "  filegator -base-path /files\n")
	fmt.Fprintf(w, "  filegator -export-frontend ./ui && filegator -frontend-dir ./ui\n")
	fmt.Fprintf(w, "  filegator -list-admins\n")
	fmt.Fprintf(w, "  filegator -init -addr 0.0.0.0:8080      first start in a container, no terminal to ask on\n")
	fmt.Fprintf(w, "  filegator -reset-admin-password alice\n")
}
