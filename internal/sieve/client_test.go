package sieve

import (
	"bufio"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeServer speaks enough ManageSieve for a scripted session.
type fakeServer struct {
	mu sync.Mutex
	// responses per command, consumed in order; "" means generic OK.
	lines []string
	got   []string // commands received
}

func (fs *fakeServer) commands() []string {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return append([]string(nil), fs.got...)
}

// waitCommands polls until the fake has recorded n commands.
func waitCommands(t *testing.T, fs *fakeServer, n int) []string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cmds := fs.commands(); len(cmds) >= n {
			return cmds
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d commands; got %q", n, fs.commands())
	return nil
}

func startFake(t *testing.T, fs *fakeServer) *Client {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		r := bufio.NewReader(conn)
		// greeting with capabilities + OK
		_, _ = conn.Write([]byte("\"SASL\" \"PLAIN\"\r\n\"STARTTLS\"\r\nOK\r\n"))
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimRight(line, "\r\n")
			fs.mu.Lock()
			fs.got = append(fs.got, line)
			fs.mu.Unlock()
			resp := ""
			if len(fs.lines) > 0 {
				resp = fs.lines[0]
				fs.lines = fs.lines[1:]
			}
			if resp == "" {
				resp = "OK"
			}
			_, _ = conn.Write([]byte(resp + "\r\n"))
		}
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	// Bypass STARTTLS for tests: build the client directly.
	c := &Client{conn: conn, r: bufio.NewReader(conn)}
	if _, err := c.readCapabilities(); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestListScriptsSkipsMalformedQuirkLine(t *testing.T) {
	// The server emits a constant malformed `"\\"` line before the entries.
	c := startFake(t, &fakeServer{
		lines: []string{"\"\\\x22\r\n\"main\" ACTIVE\r\nOK"},
	})

	scripts, err := c.ListScripts()
	if err != nil {
		t.Fatalf("ListScripts: %v", err)
	}
	if len(scripts) != 1 || scripts[0].Name != "main" || !scripts[0].Active {
		t.Fatalf("scripts = %+v", scripts)
	}
}

func TestPutScriptSendsLiteralPlus(t *testing.T) {
	fs := &fakeServer{lines: []string{"OK", "OK"}} // PUTSCRIPT + SETACTIVE
	c := startFake(t, fs)

	if err := c.PutScript("main", "keep;", true); err != nil {
		t.Fatalf("PutScript: %v", err)
	}

	cmds := waitCommands(t, fs, 3)
	if cmds[0] != `PUTSCRIPT "main" {5+}` {
		t.Errorf("put command = %q", cmds[0])
	}
	if cmds[1] != "keep;" {
		t.Errorf("literal = %q", cmds[1])
	}
	if cmds[2] != `SETACTIVE "main"` {
		t.Errorf("setactive = %q", cmds[2])
	}
}

func TestDeleteScriptRetriesAfterDeactivate(t *testing.T) {
	fs := &fakeServer{lines: []string{
		`NO (ACTIVE) "You may not delete an active script"`,
		"OK", // SETACTIVE ""
		"OK", // DELETESCRIPT retry
	}}
	c := startFake(t, fs)

	if err := c.DeleteScript("main"); err != nil {
		t.Fatalf("DeleteScript: %v", err)
	}
	cmds := waitCommands(t, fs, 3)
	if cmds[1] != `SETACTIVE ""` {
		t.Errorf("deactivate = %q", cmds[1])
	}
	if cmds[2] != `DELETESCRIPT "main"` {
		t.Errorf("retry delete = %q", cmds[2])
	}
}

func TestSplitQuoted(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{`"a b" ACTIVE`, []string{"a b", "ACTIVE"}},
		{`"say \"hi\"" ""`, []string{`say "hi"`, ""}},
		{`OK`, []string{"OK"}},
		{`"\"`, []string{`"`}}, // the server quirk line
	}
	for _, tc := range cases {
		got := splitQuoted(tc.in)
		if len(got) != len(tc.want) {
			t.Errorf("splitQuoted(%q) = %#v, want %#v", tc.in, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("splitQuoted(%q)[%d] = %q, want %q", tc.in, i, got[i], tc.want[i])
			}
		}
	}
}

func TestQuoted(t *testing.T) {
	if got := quoted(`a"b\c`); got != `"a\"b\\c"` {
		t.Errorf("quoted = %q", got)
	}
}
