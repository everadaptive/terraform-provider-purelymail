package purelymail

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newTestServer starts an httptest server that records the last request and
// replies with the given envelope JSON. It returns a client pointing at it.
func newTestServer(t *testing.T, status int, envelope string) (*Client, *record) {
	t.Helper()

	rec := &record{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.method = r.Method
		rec.path = r.URL.Path
		rec.token = r.Header.Get("Purelymail-Api-Token")
		b, _ := io.ReadAll(r.Body)
		rec.body = string(b)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, envelope)
	}))
	t.Cleanup(srv.Close)

	return New(srv.URL, "test-token"), rec
}

type record struct {
	method string
	path   string
	token  string
	body   string
}

func TestCheckAccountCredit(t *testing.T) {
	c, rec := newTestServer(t, http.StatusOK, `{"result":{"credit":"42.00"}}`)

	credit, err := c.CheckAccountCredit(context.Background())
	if err != nil {
		t.Fatalf("CheckAccountCredit: %v", err)
	}
	if credit != "42.00" {
		t.Errorf("credit = %q, want 42.00", credit)
	}
	if rec.token != "test-token" {
		t.Errorf("token header = %q, want test-token", rec.token)
	}
	if rec.path != "/api/v0/checkAccountCredit" {
		t.Errorf("path = %q", rec.path)
	}
	if rec.method != http.MethodPost {
		t.Errorf("method = %q, want POST", rec.method)
	}
}

func TestListUser(t *testing.T) {
	c, rec := newTestServer(t, http.StatusOK, `{"result":{"users":["a@x.com","b@x.com"]}}`)

	users, err := c.ListUser(context.Background())
	if err != nil {
		t.Fatalf("ListUser: %v", err)
	}
	if len(users) != 2 || users[0] != "a@x.com" || users[1] != "b@x.com" {
		t.Errorf("users = %v", users)
	}
	if rec.body != "{}" {
		t.Errorf("body = %q, want empty JSON object", rec.body)
	}
}

func TestCreateUserRequest(t *testing.T) {
	c, rec := newTestServer(t, http.StatusOK, `{"result":{}}`)
	yes := true

	err := c.CreateUser(context.Background(), CreateUserRequest{
		UserName:             "alice",
		DomainName:           "example.com",
		Password:             "hunter2",
		EnablePasswordReset:  &yes,
		EnableSearchIndexing: &yes,
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	var sent map[string]any
	if err := json.Unmarshal([]byte(rec.body), &sent); err != nil {
		t.Fatal(err)
	}
	if sent["userName"] != "alice" || sent["domainName"] != "example.com" || sent["password"] != "hunter2" {
		t.Errorf("body = %s", rec.body)
	}
	if _, ok := sent["recoveryEmail"]; ok {
		t.Errorf("empty omitempty field was sent: %s", rec.body)
	}
}

func TestAPIError(t *testing.T) {
	c, _ := newTestServer(t, http.StatusBadRequest, `{"code":"BadToken","message":"token invalid"}`)

	_, err := c.CheckAccountCredit(context.Background())
	if err == nil {
		t.Fatal("want error, got nil")
	}
	var apiErr APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("want APIError, got %T: %v", err, err)
	}
	if apiErr.Code != "BadToken" || apiErr.Message != "token invalid" {
		t.Errorf("apiErr = %+v", apiErr)
	}
}

func TestListDomains(t *testing.T) {
	envelope := `{"result":{"domains":[{"name":"x.com","isShared":false,"dnsSummary":{"passesMx":true,"passesSpf":false,"passesDkim":true,"passesDmarc":true}}]}}`
	c, rec := newTestServer(t, http.StatusOK, envelope)

	domains, err := c.ListDomains(context.Background(), ListDomainsRequest{})
	if err != nil {
		t.Fatalf("ListDomains: %v", err)
	}
	if len(domains) != 1 || domains[0].Name != "x.com" {
		t.Errorf("domains = %+v", domains)
	}
	if domains[0].DnsSummary == nil || domains[0].DnsSummary.PassesSpf == nil || *domains[0].DnsSummary.PassesSpf {
		t.Errorf("dnsSummary = %+v", domains[0].DnsSummary)
	}
	if rec.body != "{}" {
		t.Errorf("body = %q", rec.body)
	}
}

func TestLogicalErrorEnvelope(t *testing.T) {
	// The API returns logical errors as HTTP 200 with type "error".
	c, _ := newTestServer(t, http.StatusOK, `{"type":"error","code":"internalError","message":"DNS ownership checks did not pass"}`)

	err := c.CreateUser(context.Background(), CreateUserRequest{UserName: "x", DomainName: "d", Password: "p"})
	var apiErr APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("want APIError, got %T: %v", err, err)
	}
}

func TestListRoutingRules(t *testing.T) {
	envelope := `{"result":{"rules":[{"id":7,"domainName":"x.com","prefix":false,"matchUser":"all","targetAddresses":["a@b.com"],"catchall":true}]}}`
	c, rec := newTestServer(t, http.StatusOK, envelope)

	rules, err := c.ListRoutingRules(context.Background())
	if err != nil {
		t.Fatalf("ListRoutingRules: %v", err)
	}
	if len(rules) != 1 || rules[0].ID != 7 || !rules[0].Catchall || rules[0].TargetAddresses[0] != "a@b.com" {
		t.Errorf("rules = %+v", rules)
	}
	if rec.body != "{}" {
		t.Errorf("body = %q", rec.body)
	}
}

func TestCreateAndDeleteRoutingRule(t *testing.T) {
	c, rec := newTestServer(t, http.StatusOK, `{"result":{}}`)

	err := c.CreateRoutingRule(context.Background(), CreateRoutingRuleRequest{
		DomainName:      "x.com",
		Prefix:          false,
		MatchUser:       "all",
		TargetAddresses: []string{"a@b.com"},
		Catchall:        true,
	})
	if err != nil {
		t.Fatalf("CreateRoutingRule: %v", err)
	}
	var sent struct {
		DomainName      string   `json:"domainName"`
		TargetAddresses []string `json:"targetAddresses"`
		Catchall        bool     `json:"catchall"`
	}
	if err := json.Unmarshal([]byte(rec.body), &sent); err != nil {
		t.Fatal(err)
	}
	if sent.DomainName != "x.com" || !sent.Catchall || len(sent.TargetAddresses) != 1 {
		t.Errorf("sent = %+v", sent)
	}

	rec = &record{}
	// re-primed recorder via new server call
	c2, rec2 := newTestServer(t, http.StatusOK, `{"result":{}}`)
	if err := c2.DeleteRoutingRule(context.Background(), DeleteRoutingRuleRequest{RoutingRuleID: 7}); err != nil {
		t.Fatalf("DeleteRoutingRule: %v", err)
	}
	if rec2.body != `{"routingRuleId":7}` {
		t.Errorf("delete body = %q", rec2.body)
	}
}

func TestCreateAppPassword(t *testing.T) {
	c, rec := newTestServer(t, http.StatusOK, `{"result":{"appPassword":"passw0rd-full-value"}}`)

	out, err := c.CreateAppPassword(context.Background(), CreateAppPasswordRequest{UserHandle: "u@d.com", Name: "laptop"})
	if err != nil {
		t.Fatalf("CreateAppPassword: %v", err)
	}
	if out.AppPassword != "passw0rd-full-value" {
		t.Errorf("password = %q", out.AppPassword)
	}
	if rec.body != `{"userHandle":"u@d.com","name":"laptop"}` {
		t.Errorf("body = %q", rec.body)
	}
}

func TestDeleteAppPassword(t *testing.T) {
	c, rec := newTestServer(t, http.StatusOK, `{"result":{}}`)

	err := c.DeleteAppPassword(context.Background(), DeleteAppPasswordRequest{UserName: "u@d.com", AppPassword: "pw"})
	if err != nil {
		t.Fatalf("DeleteAppPassword: %v", err)
	}
	if rec.body != `{"userName":"u@d.com","appPassword":"pw"}` {
		t.Errorf("body = %q", rec.body)
	}
}

func TestPasswordResetRoundTrip(t *testing.T) {
	c, rec := newTestServer(t, http.StatusOK, `{"result":{"users":[{"type":"email","target":"r@x.com","description":"backup","allowMfaReset":true}]}}`)

	methods, err := c.ListPasswordReset(context.Background(), GetUserRequest{UserName: "u@d.com"})
	if err != nil {
		t.Fatalf("ListPasswordReset: %v", err)
	}
	if len(methods) != 1 || methods[0].Target != "r@x.com" || !methods[0].AllowMfaReset {
		t.Errorf("methods = %+v", methods)
	}
	if rec.body != `{"userName":"u@d.com"}` {
		t.Errorf("body = %q", rec.body)
	}

	c2, rec2 := newTestServer(t, http.StatusOK, `{"result":{}}`)
	yes := true
	err = c2.UpsertPasswordReset(context.Background(), UpsertPasswordResetRequest{UserName: "u@d.com", Type: "email", Target: "r@x.com", AllowMfaReset: &yes})
	if err != nil {
		t.Fatalf("UpsertPasswordReset: %v", err)
	}
	if rec2.body != `{"userName":"u@d.com","type":"email","target":"r@x.com","allowMfaReset":true}` {
		t.Errorf("upsert body = %q", rec2.body)
	}
}
