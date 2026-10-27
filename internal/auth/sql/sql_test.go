package sql

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/auth/jsonfile"
	"github.com/filegator/filegator/internal/config"
)

// fakeDriver is a database/sql driver that answers the handful of queries this
// provider issues, so the provider can be tested without a database server.
type fakeDriver struct {
	mu      sync.Mutex
	queries []string
	args    [][]driver.Value
	rows    map[string][]row // keyed by lower-case username
	addErr  error
}

type row struct {
	username, name, role, perms, homedir, password string
}

var fakeDrivers sync.Map

func (d *fakeDriver) Open(name string) (driver.Conn, error) { return &fakeConn{d: d}, nil }

type fakeConn struct{ d *fakeDriver }

func (c *fakeConn) Prepare(query string) (driver.Stmt, error) {
	c.d.mu.Lock()
	c.d.queries = append(c.d.queries, query)
	c.d.mu.Unlock()
	return &fakeStmt{d: c.d, query: query}, nil
}
func (c *fakeConn) Close() error              { return nil }
func (c *fakeConn) Begin() (driver.Tx, error) { return nil, errors.New("no transactions") }

type fakeStmt struct {
	d     *fakeDriver
	query string
}

func (s *fakeStmt) Close() error  { return nil }
func (s *fakeStmt) NumInput() int { return -1 }

func (s *fakeStmt) Exec(args []driver.Value) (driver.Result, error) {
	s.d.mu.Lock()
	defer s.d.mu.Unlock()
	s.d.args = append(s.d.args, args)

	q := strings.ToLower(s.query)
	switch {
	case strings.HasPrefix(q, "insert"):
		if len(args) >= 6 {
			uname := args[0].(string)
			if _, exists := s.d.rows[strings.ToLower(uname)]; exists {
				return nil, errors.New("duplicate key")
			}
			s.d.rows[strings.ToLower(uname)] = []row{{
				username: uname, name: args[1].(string), role: args[2].(string),
				perms: args[3].(string), homedir: args[4].(string), password: args[5].(string),
			}}
		}
		return fakeResult(1), nil
	case strings.HasPrefix(q, "update"):
		if len(args) == 0 {
			return fakeResult(0), nil
		}
		old := args[len(args)-1].(string)
		key := strings.ToLower(old)
		existing, ok := s.d.rows[key]
		if !ok {
			return fakeResult(0), nil
		}
		// Apply the fields present in the SET clause, in order.
		setPart := q[strings.Index(q, "set")+3:]
		cols := setColumns(setPart)
		cur := existing[0]
		ai := 0
		for _, col := range cols {
			if ai >= len(args)-1 {
				break
			}
			v, _ := args[ai].(string)
			switch col {
			case "username":
				cur.username = v
			case "name":
				cur.name = v
			case "role":
				cur.role = v
			case "permissions":
				cur.perms = v
			case "homedir":
				cur.homedir = v
			case "password":
				cur.password = v
			}
			ai++
		}
		delete(s.d.rows, key)
		s.d.rows[strings.ToLower(cur.username)] = []row{cur}
		return fakeResult(1), nil
	case strings.HasPrefix(q, "delete"):
		if len(args) > 0 {
			key := strings.ToLower(args[0].(string))
			if _, ok := s.d.rows[key]; !ok {
				return fakeResult(0), nil
			}
			delete(s.d.rows, key)
		}
		return fakeResult(1), nil
	}
	return fakeResult(0), nil
}

// setColumns extracts the column names from a SET clause.
func setColumns(setPart string) []string {
	var cols []string
	for _, part := range strings.Split(setPart, ",") {
		part = strings.TrimSpace(part)
		if i := strings.Index(part, "="); i > 0 {
			cols = append(cols, strings.TrimSpace(part[:i]))
		}
	}
	return cols
}

func (s *fakeStmt) Query(args []driver.Value) (driver.Rows, error) {
	s.d.mu.Lock()
	defer s.d.mu.Unlock()
	s.d.args = append(s.d.args, args)

	q := strings.ToLower(s.query)
	if strings.Contains(q, "where username") && len(args) > 0 {
		key := strings.ToLower(fmt.Sprint(args[0]))
		rows, ok := s.d.rows[key]
		if !ok {
			return &fakeRows{}, nil
		}
		return &fakeRows{rows: rows}, nil
	}
	// Full listing.
	var all []row
	for _, rs := range s.d.rows {
		all = append(all, rs...)
	}
	return &fakeRows{rows: all}, nil
}

type fakeResult int64

func (r fakeResult) LastInsertId() (int64, error) { return 0, nil }
func (r fakeResult) RowsAffected() (int64, error) { return int64(r), nil }

type fakeRows struct {
	rows []row
	pos  int
}

func (r *fakeRows) Columns() []string {
	return []string{"username", "name", "role", "permissions", "homedir", "password"}
}
func (r *fakeRows) Close() error { return nil }
func (r *fakeRows) Next(dest []driver.Value) error {
	if r.pos >= len(r.rows) {
		return io.EOF
	}
	cur := r.rows[r.pos]
	r.pos++
	dest[0] = cur.username
	dest[1] = cur.name
	dest[2] = cur.role
	dest[3] = cur.perms
	dest[4] = cur.homedir
	dest[5] = cur.password
	return nil
}

// newFakeProvider builds a provider backed by the fake driver.
func newFakeProvider(t *testing.T, dialect Dialect, seed map[string]row) (*Provider, *fakeDriver) {
	t.Helper()

	rows := map[string][]row{}
	for k, v := range seed {
		rows[strings.ToLower(k)] = []row{v}
	}
	d := &fakeDriver{rows: rows}

	name := "fake-" + dialect.Name + "-" + t.Name()
	name = strings.ReplaceAll(name, "/", "_")
	sql.Register(name, d)

	db, err := sql.Open(name, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	p := &Provider{
		db:       db,
		dialect:  dialect,
		table:    "users",
		timeout:  time.Second,
		ttl:      DefaultTTL,
		describe: dialect.Name + ":users",
		cache:    make(map[string]cacheEntry),
	}
	return p, d
}

func hashFor(t *testing.T, password string) string {
	t.Helper()
	h, err := jsonfileHash(password)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// jsonfileHash hashes a password the way the provider does.
func jsonfileHash(password string) (string, error) {
	return jsonfile.HashPassword(password)
}

func TestDialectPlaceholders(t *testing.T) {
	if got := MySQL.Placeholder(1); got != "?" {
		t.Errorf("mysql placeholder = %q", got)
	}
	if got := Postgres.Placeholder(3); got != "$3" {
		t.Errorf("postgres placeholder = %q", got)
	}
	if got := MySQL.Quote("users"); got != "`users`" {
		t.Errorf("mysql quote = %q", got)
	}
	if got := Postgres.Quote("users"); got != `"users"` {
		t.Errorf("postgres quote = %q", got)
	}
}

func TestDialectDrivers(t *testing.T) {
	if MySQL.Driver != "mysql" {
		t.Errorf("mysql driver = %q", MySQL.Driver)
	}
	if Postgres.Driver != "pgx" {
		t.Errorf("postgres driver = %q", Postgres.Driver)
	}
}

func TestPostgresUpdateUsesNumberedPlaceholders(t *testing.T) {
	p, d := newFakeProvider(t, Postgres, map[string]row{
		"alice": {username: "alice", name: "Alice", role: "user", perms: "read", homedir: "/", password: "x"},
	})

	u := &auth.User{Username: "alice", Name: "Alice 2", Role: auth.RoleUser, HomeDir: "/a", Permissions: []auth.Permission{auth.PermRead}}
	if _, err := p.Update("alice", u, ""); err != nil {
		t.Fatalf("Update: %v", err)
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	found := false
	for _, q := range d.queries {
		if strings.HasPrefix(strings.ToLower(q), "update") {
			found = true
			if !strings.Contains(q, "$1") || !strings.Contains(q, "$5") {
				t.Errorf("postgres update is not numbered: %s", q)
			}
			if strings.Contains(q, "$0") {
				t.Errorf("placeholder numbering starts at 0: %s", q)
			}
		}
	}
	if !found {
		t.Fatal("no update query issued")
	}
}

func TestMySQLUpdateUsesQuestionMarks(t *testing.T) {
	p, d := newFakeProvider(t, MySQL, map[string]row{
		"alice": {username: "alice", name: "Alice", role: "user", perms: "read", homedir: "/", password: "x"},
	})
	u := &auth.User{Username: "alice", Name: "A", Role: auth.RoleUser, HomeDir: "/", Permissions: []auth.Permission{auth.PermRead}}
	if _, err := p.Update("alice", u, ""); err != nil {
		t.Fatal(err)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, q := range d.queries {
		if strings.HasPrefix(strings.ToLower(q), "update") && !strings.Contains(q, "?") {
			t.Errorf("mysql update lacks '?': %s", q)
		}
	}
}

func TestUpdateRenamesUser(t *testing.T) {
	p, d := newFakeProvider(t, Postgres, map[string]row{
		"alice": {username: "alice", name: "Alice", role: "user", perms: "read", homedir: "/", password: "x"},
	})

	u := &auth.User{Username: "alicia", Name: "Alice", Role: auth.RoleUser, HomeDir: "/", Permissions: []auth.Permission{auth.PermRead}}
	if _, err := p.Update("alice", u, ""); err != nil {
		t.Fatalf("Update: %v", err)
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.rows["alicia"]; !ok {
		t.Error("renamed user missing")
	}
	if _, ok := d.rows["alice"]; ok {
		t.Error("old username still present")
	}
}

func TestUpdateRenameCollision(t *testing.T) {
	p, _ := newFakeProvider(t, Postgres, map[string]row{
		"alice": {username: "alice", name: "A", role: "user", perms: "read", homedir: "/", password: "x"},
		"bob":   {username: "bob", name: "B", role: "user", perms: "read", homedir: "/", password: "x"},
	})
	u := &auth.User{Username: "bob", Name: "A", Role: auth.RoleUser, HomeDir: "/", Permissions: []auth.Permission{auth.PermRead}}
	if _, err := p.Update("alice", u, ""); !errors.Is(err, auth.ErrUsernameTaken) {
		t.Fatalf("got %v, want ErrUsernameTaken", err)
	}
}

func TestUpdateUnknownUser(t *testing.T) {
	p, _ := newFakeProvider(t, Postgres, nil)
	u := &auth.User{Username: "ghost", Name: "G", Role: auth.RoleUser, HomeDir: "/"}
	if _, err := p.Update("ghost", u, ""); !errors.Is(err, auth.ErrUserNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestDeleteUser(t *testing.T) {
	p, d := newFakeProvider(t, MySQL, map[string]row{
		"alice": {username: "alice", name: "A", role: "user", perms: "read", homedir: "/", password: "x"},
	})
	if err := p.Delete("alice"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	d.mu.Lock()
	remaining := len(d.rows)
	d.mu.Unlock()
	if remaining != 0 {
		t.Errorf("%d rows left", remaining)
	}
	if err := p.Delete("alice"); !errors.Is(err, auth.ErrUserNotFound) {
		t.Errorf("second delete: got %v", err)
	}
}

func TestValidateUser(t *testing.T) {
	cases := map[string]*auth.User{
		"empty username": {Username: "", Role: auth.RoleUser},
		"bad role":       {Username: "a", Role: "root"},
		"bad perm":       {Username: "a", Role: auth.RoleUser, Permissions: []auth.Permission{"teleport"}},
	}
	for name, u := range cases {
		if err := validateUser(u); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if err := validateUser(&auth.User{Username: "a", Role: auth.RoleUser}); err != nil {
		t.Errorf("valid user rejected: %v", err)
	}
}

func TestNormaliseHome(t *testing.T) {
	if got := normaliseHome(""); got != "/" {
		t.Errorf("got %q", got)
	}
	if got := normaliseHome("/x"); got != "/x" {
		t.Errorf("got %q", got)
	}
}

func TestSetColumnsParsing(t *testing.T) {
	cols := setColumns("username = $1, name = $2, role = $3")
	want := []string{"username", "name", "role"}
	if len(cols) != len(want) {
		t.Fatalf("got %v", cols)
	}
	for i := range want {
		if cols[i] != want[i] {
			t.Errorf("cols[%d] = %q", i, cols[i])
		}
	}
}

func TestCapabilities(t *testing.T) {
	p, _ := newFakeProvider(t, MySQL, nil)
	caps := p.Capabilities()
	if !caps.ManageUsers || !caps.ChangePassword {
		t.Errorf("caps = %+v", caps)
	}
	if !p.Writable() {
		t.Error("sql provider should be writable")
	}
}

func TestSessionFingerprintChangesWithRole(t *testing.T) {
	p, _ := newFakeProvider(t, MySQL, nil)
	a := &auth.User{Username: "x", Role: auth.RoleUser, HomeDir: "/"}
	b := &auth.User{Username: "x", Role: auth.RoleAdmin, HomeDir: "/"}
	if p.SessionFingerprint(a) == p.SessionFingerprint(b) {
		t.Error("fingerprint must change with the role")
	}
}

func TestNewRejectsEmptyDSN(t *testing.T) {
	if _, err := New(config.SQL{}, MySQL); err == nil {
		t.Fatal("empty DSN accepted")
	}
}

// Ensure the driver imports register the real drivers, so a typo in a driver
// name fails here rather than at runtime.
func TestRealDriversRegistered(t *testing.T) {
	for _, name := range []string{"mysql", "pgx"} {
		found := false
		for _, d := range sql.Drivers() {
			if d == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("driver %q is not registered (registered: %v)", name, sql.Drivers())
		}
	}
}
