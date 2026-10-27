// Package logger writes an append-only event log of security-relevant
// activity: sign-ins, rejected sign-ins, blocked addresses, refused CSRF
// tokens, permission denials and changes to files, users and settings.
//
// v7 used Monolog and logged three things (login success, login failure,
// lockout) plus CSRF and IP-list rejections, to private/logs/app.log. This
// package keeps the same destinations and levels but adds an audit trail for
// changes, because "who deleted this file" is the question an administrator
// actually asks.
//
// Only the standard library is used: log/slog provides the levels, the JSON
// output and the file rotation hook.
package logger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// Event names. They are stable identifiers so the log can be grepped and
// aggregated; the human-readable message is separate.
const (
	EventLogin               = "auth.login"
	EventLoginFailed         = "auth.login_failed"
	EventLoginLocked         = "auth.login_locked"
	EventLogout              = "auth.logout"
	EventPasswordChanged     = "auth.password_changed"
	EventSSOLogin            = "auth.sso_login"
	EventSSOFailed           = "auth.sso_failed"
	EventCSRFFailed          = "security.csrf_failed"
	EventIPBlocked           = "security.ip_blocked"
	EventPermissionDenied    = "security.permission_denied"
	EventFileUpload          = "file.upload"
	EventFileCreated         = "file.create"
	EventFileSaved           = "file.save"
	EventFileRenamed         = "file.rename"
	EventFileCopied          = "file.copy"
	EventFileDeleted         = "file.delete"
	EventFileChmod           = "file.chmod"
	EventZip                 = "file.zip"
	EventUnzip               = "file.unzip"
	EventBatchDownload       = "file.batch_download"
	EventAdminUserCreated    = "admin.user_created"
	EventAdminUserUpdated    = "admin.user_updated"
	EventAdminUserDeleted    = "admin.user_deleted"
	EventAdminSettings       = "admin.settings_saved"
	EventAdminStorageChanged = "admin.storage_changed"
	EventAdminAuthChanged    = "admin.auth_changed"
	EventServerError         = "server.error"
)

// Fields carries structured detail for one event. A nil map is valid, so a
// caller with nothing to add can pass nil.
type Fields map[string]any

// Logger records events. It is safe for concurrent use.
type Logger struct {
	handler slog.Handler
	level   *slog.LevelVar
	// sink is closed on Close; nil when logging to stderr only.
	sink io.Closer
}

// Config describes where events go.
type Config struct {
	// File is the log destination. A "-" writes to standard output, which is
	// what a container wants: the supervisor collects it and handles rotation.
	// Any other value is a file path. Empty means standard error, matching how
	// a server without a writable volume still reports what it is doing.
	File string
	// Level is one of "debug", "info", "warn", "error".
	Level string
	// MaxSize is the size in bytes at which the current file is rotated to
	// "<file>.1". Zero disables rotation. Exactly one previous file is kept,
	// so the disk use is bounded at roughly twice MaxSize.
	MaxSize int64
}

// ParseLevel converts a config level name into a slog level. An unknown name
// falls back to info rather than failing: a typo in a log level must not stop
// the server.
func ParseLevel(name string) slog.Level {
	switch name {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// New builds a logger. With an empty Config.File it writes to stderr; when a
// file cannot be opened the error is returned so startup can warn and carry on
// with stderr.
func New(cfg Config) (*Logger, error) {
	level := &slog.LevelVar{}
	level.Set(ParseLevel(cfg.Level))

	// Standard streams need no folder, no file mode and no rotation: the
	// process supervisor owns the file and rotates it.
	switch cfg.File {
	case "":
		return newStreamLogger(os.Stderr, level), nil
	case Stdout:
		return newStreamLogger(os.Stdout, level), nil
	}

	// A bare name like "app.log" has no folder part, and MkdirAll("") fails,
	// so only real folders are created.
	if dir := filepath.Dir(cfg.File); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("logger: create log folder: %w", err)
		}
	}
	// Owner-only: the log names users, addresses and paths.
	f, err := os.OpenFile(cfg.File, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("logger: open log file: %w", err)
	}

	w := io.Writer(f)
	if cfg.MaxSize > 0 {
		w = newRotateWriter(f, cfg.File, cfg.MaxSize)
	}
	return &Logger{
		handler: slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}),
		level:   level,
		// Only a real file needs closing. noteSink keeps that decision in one
		// place: assigning the io.Writer here rather than f would also work,
		// but then a rotated writer could never be flushed on shutdown.
		sink: f,
	}, nil
}

// Stdout is the File value that writes records to standard output.
const Stdout = "-"

// newStreamLogger builds a logger over a standard stream.
func newStreamLogger(w io.Writer, level *slog.LevelVar) *Logger {
	return &Logger{
		handler: slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}),
		level:   level,
	}
}

// SetLevel changes the threshold at runtime, so an administrator can switch to
// debug without a restart.
func (l *Logger) SetLevel(name string) {
	if l == nil {
		return
	}
	l.level.Set(ParseLevel(name))
}

// Close releases the log file.
func (l *Logger) Close() error {
	if l == nil || l.sink == nil {
		return nil
	}
	return l.sink.Close()
}

// Info records a routine, successful action.
func (l *Logger) Info(event, message string, fields Fields) {
	l.log(slog.LevelInfo, event, message, fields)
}

// Warn records something suspicious that was handled safely, such as a refused
// login or a blocked address.
func (l *Logger) Warn(event, message string, fields Fields) {
	l.log(slog.LevelWarn, event, message, fields)
}

// Error records a failure the administrator probably needs to look at.
func (l *Logger) Error(event, message string, fields Fields) {
	l.log(slog.LevelError, event, message, fields)
}

// Debug records detail only useful while diagnosing a problem.
func (l *Logger) Debug(event, message string, fields Fields) {
	l.log(slog.LevelDebug, event, message, fields)
}

// now is the clock used for record timestamps.
var now = time.Now

// log writes one record. A nil logger is a no-op so callers never need to
// guard, and tests can leave logging out entirely.
func (l *Logger) log(level slog.Level, event, message string, fields Fields) {
	if l == nil {
		return
	}
	if !l.handler.Enabled(context.Background(), level) {
		return
	}

	// Values are rendered before the handler is called. slog serialises the
	// record on its own goroutine, so an Any value would be read later — by
	// then the caller's map may have been reused and the fields would be
	// missing. Converting to a string here makes each record self-contained.
	attrs := make([]slog.Attr, 0, len(fields)+1)
	attrs = append(attrs, slog.String("event", event))
	for key, value := range fields {
		// A nil value would print as <nil>; skipping keeps records tidy.
		if value == nil {
			continue
		}
		if text, ok := value.(string); ok && text == "" {
			continue
		}
		attrs = append(attrs, slog.String(key, Render(value)))
	}
	record := slog.NewRecord(now(), level, message, 0)
	record.AddAttrs(attrs...)
	_ = l.handler.Handle(context.Background(), record)
}

// RequestFields collects the request context every event should carry: the
// client address, the method and the path. Paths can contain file names, which
// is intended — an audit entry without the target is useless.
func RequestFields(r *http.Request) Fields {
	return Fields{
		"ip":     remoteIP(r),
		"method": r.Method,
		"path":   r.URL.Path,
	}
}

// UserFields adds the acting user to a Fields map, creating it when nil.
func UserFields(fields Fields, username string) Fields {
	if fields == nil {
		fields = Fields{}
	}
	fields["user"] = username
	return fields
}

// Render turns a field value into the string written to the log.
//
// Slices and maps are rendered as JSON, so "paths": ["/a", "/b"] stays one
// readable value instead of Go's [a b] notation.
func Render(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case bool:
		if v {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case []string, []any, map[string]string, map[string]any:
		if data, err := json.Marshal(v); err == nil {
			return string(data)
		}
		return fmt.Sprint(v)
	default:
		return fmt.Sprint(value)
	}
}

// Copy returns a shallow copy of fields. Handlers build the map for one record
// and pass it on, but the map is mutated afterwards (a later event adds a
// different user), so the logger must own its copy to avoid two records racing
// on the same map.
func Copy(fields Fields) Fields {
	if fields == nil {
		return nil
	}
	out := make(Fields, len(fields))
	for k, v := range fields {
		out[k] = v
	}
	return out
}

// remoteIP returns the address as it appears in RemoteAddr, without the port.
// The trusted-header resolution lives in the security filter; the log records
// what the connection reported so an entry cannot be spoofed by a header.
func remoteIP(r *http.Request) string {
	host := r.RemoteAddr
	for i := len(host) - 1; i >= 0; i-- {
		if host[i] == ':' {
			return host[:i]
		}
	}
	return host
}

// rotateWriter rotates the log file once it passes max bytes.
//
// Copy-then-truncate rather than rename: an open file handle keeps writing to
// the same inode after a rename, which would leave the new file empty. Only
// one previous file is kept, matching v7's "small and bounded" behaviour.
type rotateWriter struct {
	mu   sync.Mutex
	file *os.File
	path string
	max  int64
	size int64
}

// newRotateWriter starts a writer whose size accounting continues from the
// existing file, which matters when the server restarts on a log that has
// already grown past the limit.
func newRotateWriter(f *os.File, path string, max int64) *rotateWriter {
	w := &rotateWriter{file: f, path: path, max: max}
	if info, err := f.Stat(); err == nil {
		w.size = info.Size()
	}
	return w
}

func (w *rotateWriter) Write(p []byte) (int, error) {

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.size+int64(len(p)) > w.max && w.size > 0 {
		if err := w.rotate(); err != nil {
			// Rotation failing must not stop logging: keep appending.
			return w.file.Write(p)
		}
	}
	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}

// rotate moves the current contents to "<path>.1" and empties the live file.
//
// The data is read back through a fresh read-only handle rather than the live
// write-only one: io.Copy on the live descriptor lets the kernel use
// copy_file_range, which needs a readable source and fails with EBADF on a
// file opened O_WRONLY.
func (w *rotateWriter) rotate() error {
	if err := w.file.Sync(); err != nil {
		return err
	}
	source, err := os.Open(w.path)
	if err != nil {
		return err
	}
	defer source.Close()

	previous, err := os.OpenFile(w.path+".1", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return errors.New("logger: cannot open previous log file: " + err.Error())
	}
	_, copyErr := io.Copy(previous, source)
	closeErr := previous.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if err := w.file.Truncate(0); err != nil {
		return err
	}
	if _, err := w.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	w.size = 0
	return nil
}
