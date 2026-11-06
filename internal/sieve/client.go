// Package sieve implements a thin client for the Manage Sieve protocol
// (RFC 5804), used to manage Sieve filtering scripts for PurelyMail users.
// PurelyMail exposes ManageSieve on imap.purelymail.com:4190 with STARTTLS
// and SASL PLAIN authentication.
package sieve

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// literalRe matches a trailing literal declaration like {5} or {5+}.
var literalRe = regexp.MustCompile(`\{(\d+)\+?\}$`)

// DefaultHost is PurelyMail's ManageSieve endpoint.
const DefaultHost = "imap.purelymail.com:4190"

// Capabilities of the server keyed by capability name.
type Capabilities map[string]string

// Script is one entry from LISTSCRIPTS.
type Script struct {
	Name   string
	Active bool
}

// ResponseError is a NO/BYE response carrying a parenthesized code, e.g.
// NO (ACTIVE) "...".
type ResponseError struct {
	Code string
	Line string
}

func (e *ResponseError) Error() string { return "sieve: " + e.Line }

// Client is a ManageSieve client for a single session.
type Client struct {
	conn net.Conn
	r    *bufio.Reader
}

// Dial connects, performs STARTTLS, and returns the unauthenticated client
// along with the server capabilities.
func Dial(ctx context.Context, addr string) (*Client, Capabilities, error) {
	if addr == "" {
		addr = DefaultHost
	}
	dialer := net.Dialer{Timeout: 10 * time.Second}
	raw, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, nil, fmt.Errorf("sieve: dial %s: %w", addr, err)
	}
	c := &Client{conn: raw, r: bufio.NewReader(raw)}

	cleanup := func(err error) (*Client, Capabilities, error) {
		_ = raw.Close()
		return nil, nil, err
	}

	caps, err := c.readCapabilities()
	if err != nil {
		return cleanup(err)
	}
	if err := c.writeLine("STARTTLS"); err != nil {
		return cleanup(err)
	}
	if _, err := c.readResponse(); err != nil {
		return cleanup(err)
	}
	// Some servers (Apache ManageSieve) re-send capabilities in plaintext
	// before the TLS handshake; drain them so the handshake sees a clean
	// stream. Stop on the trailing OK or on a read timeout (server sent
	// nothing more).
	_ = c.conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	for {
		if _, err := c.r.ReadByte(); err != nil {
			break // no pending data: timeout or EOF
		}
		_ = c.r.UnreadByte()
		line, err := c.r.ReadString('\n')
		if err != nil {
			break
		}
		if strings.HasPrefix(strings.TrimRight(line, "\r\n"), "OK") {
			break
		}
	}
	_ = c.conn.SetReadDeadline(time.Time{})

	host, _, splitErr := net.SplitHostPort(addr)
	if splitErr != nil {
		host = addr
	}
	_ = raw.SetDeadline(time.Now().Add(30 * time.Second))
	tlsConn := tls.Client(raw, &tls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
	})
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		return cleanup(fmt.Errorf("sieve: TLS handshake: %w", err))
	}
	_ = raw.SetDeadline(time.Time{})
	if cs := tlsConn.ConnectionState(); len(cs.PeerCertificates) == 0 {
		return cleanup(fmt.Errorf("sieve: no server certificate presented for %s", host))
	}
	c.conn = tlsConn
	c.r = bufio.NewReader(tlsConn)

	// PurelyMail (Apache ManageSieve) sends capabilities before STARTTLS
	// (both in plaintext after the STARTTLS OK) and does NOT re-send them
	// after the TLS handshake, so we return the initial capabilities.
	return c, caps, nil
}

// Authenticate performs SASL PLAIN authentication.
func (c *Client) Authenticate(user, password string) error {
	token := base64.StdEncoding.EncodeToString([]byte("\x00" + user + "\x00" + password))
	if err := c.writeLine(fmt.Sprintf(`AUTHENTICATE "PLAIN" "%s"`, token)); err != nil {
		return err
	}
	_, err := c.readResponse()
	return err
}

// PutScript uploads a script (creating or overwriting it) and optionally
// makes it the active one.
func (c *Client) PutScript(name, content string, activate bool) error {
	// LITERAL+ lets us send the content without waiting for a prompt.
	cmd := fmt.Sprintf("PUTSCRIPT %s {%d+}", quoted(name), len(content))
	if err := c.writeLine(cmd); err != nil {
		return err
	}
	if err := c.writeLine(content); err != nil {
		return err
	}
	if _, err := c.readResponse(); err != nil {
		return err
	}
	if activate {
		return c.SetActive(name)
	}
	return nil
}

// SetActive makes an existing script the active one.
func (c *Client) SetActive(name string) error {
	if err := c.writeLine(fmt.Sprintf("SETACTIVE %s", quoted(name))); err != nil {
		return err
	}
	_, err := c.readResponse()
	return err
}

// GetScript downloads a script by name.
func (c *Client) GetScript(name string) (string, error) {
	if err := c.writeLine(fmt.Sprintf("GETSCRIPT %s", quoted(name))); err != nil {
		return "", err
	}
	return c.readLiteralResponse()
}

// ListScripts returns all scripts and their active status.
func (c *Client) ListScripts() ([]Script, error) {
	if err := c.writeLine("LISTSCRIPTS"); err != nil {
		return nil, err
	}
	var scripts []Script
	for {
		line, err := c.r.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("sieve: listscripts: %w", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if strings.HasPrefix(line, "OK") {
			return scripts, nil
		}
		if strings.HasPrefix(line, "NO") || strings.HasPrefix(line, "BYE") {
			return nil, fmt.Errorf("sieve: listscripts: %s", line)
		}
		fields := splitQuoted(line)
		if len(fields) == 0 {
			continue
		}
		// Apache ManageSieve (PurelyMail) emits a constant malformed first
		// line `"\\"` (a bare quote glyph) before the real entries; skip
		// anything that does not parse as a usable name.
		if fields[0] == `"` || fields[0] == `\` || fields[0] == "" {
			continue
		}
		scripts = append(scripts, Script{
			Name:   fields[0],
			Active: len(fields) >= 2 && strings.EqualFold(fields[1], "ACTIVE"),
		})
	}
}

// DeleteScript removes a script by name. If the script is active, it is
// deactivated first (this server rejects deleting the active script).
func (c *Client) DeleteScript(name string) error {
	if err := c.writeLine(fmt.Sprintf("DELETESCRIPT %s", quoted(name))); err != nil {
		return err
	}
	if _, err := c.readResponse(); err != nil {
		// This server rejects deleting the active script; deactivate and
		// retry once.
		var rerr *ResponseError
		if errors.As(err, &rerr) && rerr.Code == "ACTIVE" {
			if err := c.SetActive(""); err != nil {
				return err
			}
			if err := c.writeLine(fmt.Sprintf("DELETESCRIPT %s", quoted(name))); err != nil {
				return err
			}
			_, err = c.readResponse()
			return err
		}
		return err
	}
	return nil
}

// Close sends LOGOUT and closes the connection.
func (c *Client) Close() error {
	_ = c.writeLine("LOGOUT")
	_, _ = c.readResponse()
	return c.conn.Close()
}

// opDeadline bounds each protocol round trip so a hung server cannot block
// the caller indefinitely.
const opDeadline = 30 * time.Second

func (c *Client) setOpDeadline() {
	_ = c.conn.SetDeadline(time.Now().Add(opDeadline))
}

func (c *Client) writeLine(line string) error {
	c.setOpDeadline()
	_, err := io.WriteString(c.conn, line+"\r\n")
	return err
}

// readCapabilities reads the greeting/capability block terminated by OK.
func (c *Client) readCapabilities() (Capabilities, error) {
	caps := Capabilities{}
	for {
		line, err := c.r.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("sieve: greeting: %w", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "OK") || strings.HasPrefix(line, "NO") {
			if strings.HasPrefix(line, "NO") || strings.HasPrefix(line, "BYE") {
				return nil, fmt.Errorf("sieve: greeting: %s", line)
			}
			return caps, nil
		}
		fields := splitQuoted(line)
		switch len(fields) {
		case 1:
			caps[fields[0]] = ""
		case 2:
			caps[fields[0]] = fields[1]
		}
	}
}

// responseCode extracts the parenthesized response code from a NO line,
// e.g. NO (ACTIVE) "..." yields "ACTIVE".
func responseCode(line string) string {
	start := strings.Index(line, "(")
	end := strings.Index(line, ")")
	if start >= 0 && end > start {
		return line[start+1 : end]
	}
	return ""
}

// readResponse reads one command result line, erroring on NO/BYE.
func (c *Client) readResponse() (string, error) {
	for {
		line, err := c.r.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("sieve: read response: %w", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "NO") || strings.HasPrefix(line, "BYE") {
			if code := responseCode(line); code != "" {
				return line, &ResponseError{Code: code, Line: line}
			}
			return line, fmt.Errorf("sieve: %s", line)
		}
		return line, nil
	}
}

// readLiteralResponse reads a "{len}" literal payload followed by an OK line.
func (c *Client) readLiteralResponse() (string, error) {
	line, err := c.r.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("sieve: read: %w", err)
	}
	line = strings.TrimRight(line, "\r\n")
	if strings.HasPrefix(line, "NO") || strings.HasPrefix(line, "BYE") {
		return "", fmt.Errorf("sieve: %s", line)
	}
	m := literalRe.FindStringSubmatch(line)
	if m == nil {
		return "", fmt.Errorf("sieve: expected literal, got %q", line)
	}
	size, err := strconv.Atoi(m[1])
	if err != nil {
		return "", fmt.Errorf("sieve: bad literal size in %q", line)
	}
	buf := make([]byte, size)
	if _, err := io.ReadFull(c.r, buf); err != nil {
		return "", fmt.Errorf("sieve: read literal: %w", err)
	}
	if _, err := c.readResponse(); err != nil {
		return "", err
	}
	return string(buf), nil
}

// quoted returns a double-quoted string, escaping backslashes and quotes.
func quoted(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

// splitQuoted splits a line into quoted strings or bare tokens, handling
// escaped characters inside quotes.
func splitQuoted(line string) []string {
	var out []string
	var cur strings.Builder
	inQuote := false
	hasToken := false
	for i := 0; i < len(line); i++ {
		ch := line[i]
		switch {
		case ch == '\\' && inQuote && i+1 < len(line):
			cur.WriteByte(line[i+1])
			i++
			hasToken = true
		case ch == '"':
			inQuote = !inQuote
			hasToken = true
		case (ch == ' ' || ch == '\t') && !inQuote:
			if hasToken {
				out = append(out, cur.String())
				cur.Reset()
				hasToken = false
			}
		default:
			cur.WriteByte(ch)
			hasToken = true
		}
	}
	if hasToken {
		out = append(out, cur.String())
	}
	return out
}
