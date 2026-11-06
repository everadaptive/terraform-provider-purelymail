package provider

import (
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func spec(rules ...sieveRuleSpec) []sieveRuleSpec { return rules }

func match(header, typ, value string) sieveMatchSpec {
	return sieveMatchSpec{
		Header: types.StringValue(header),
		Type:   types.StringValue(typ),
		Value:  types.StringValue(value),
	}
}

func action(typ string) sieveActionSpec {
	return sieveActionSpec{Type: types.StringValue(typ)}
}

func TestGenerateSieveBasic(t *testing.T) {
	out, err := generateSieve([]sieveRuleSpec{{
		Description: types.StringValue("file announce mail"),
		Matches:     []sieveMatchSpec{match("from", "is", "announce@example.com")},
		Actions: []sieveActionSpec{
			{Type: types.StringValue("fileinto"), Target: types.StringValue("INBOX.announce")},
			action("stop"),
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	want := "require [\"fileinto\"];\n\n" +
		"# file announce mail\n" +
		"if header :is \"from\" \"announce@example.com\" {\n" +
		"    fileinto \"INBOX.announce\";\n" +
		"    stop;\n" +
		"}\n"
	if out != want {
		t.Errorf("got:\n%s\nwant:\n%s", out, want)
	}
}

func TestGenerateSieveMultipleMatchesAreANDed(t *testing.T) {
	out, err := generateSieve([]sieveRuleSpec{{
		Matches: []sieveMatchSpec{
			match("from", "is", "a@b.com"),
			match("subject", "contains", "urgent"),
		},
		Actions: []sieveActionSpec{{Type: types.StringValue("discard")}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "allof (header :is \"from\" \"a@b.com\", header :contains \"subject\" \"urgent\")") {
		t.Errorf("missing allof:\n%s", out)
	}
	if strings.Contains(out, "require") {
		t.Errorf("discard needs no require:\n%s", out)
	}
}

func TestGenerateSieveVacation(t *testing.T) {
	out, err := generateSieve([]sieveRuleSpec{{
		Actions: []sieveActionSpec{{
			Type:    types.StringValue("vacation"),
			Subject: types.StringValue("Out of office"),
			Message: types.StringValue("I'm away."),
			Days:    types.Int64Value(7),
		}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	want := "require [\"vacation\"];\n\n" +
		"  vacation :days 7 :subject \"Out of office\" \"I'm away.\";\n"
	if out != want {
		t.Errorf("got:\n%s\nwant:\n%s", out, want)
	}
}

func TestGenerateSieveRequiresAreSortedAndDeduped(t *testing.T) {
	out, err := generateSieve([]sieveRuleSpec{
		{
			Matches: []sieveMatchSpec{match("subject", "contains", "x")},
			Actions: []sieveActionSpec{
				{Type: types.StringValue("addflag"), Flags: []types.String{types.StringValue("\\Seen")}},
			},
		},
		{
			Matches: []sieveMatchSpec{match("subject", "contains", "y")},
			Actions: []sieveActionSpec{
				{Type: types.StringValue("vacation"), Message: types.StringValue("gone")},
				{Type: types.StringValue("fileinto"), Target: types.StringValue("INBOX.z")},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `require ["fileinto", "imap4flags", "vacation"];`) {
		t.Errorf("require line wrong:\n%s", out)
	}
}

func TestGenerateSieveDeterministic(t *testing.T) {
	rules := []sieveRuleSpec{{
		Matches: []sieveMatchSpec{match("from", "is", "a@b.com")},
		Actions: []sieveActionSpec{
			{Type: types.StringValue("fileinto"), Target: types.StringValue("INBOX.x")},
			{Type: types.StringValue("stop")},
		},
	}}
	a, _ := generateSieve(rules)
	b, _ := generateSieve(rules)
	if a != b {
		t.Error("generation is not deterministic")
	}
}

func TestGenerateSieveQuotesValues(t *testing.T) {
	out, err := generateSieve([]sieveRuleSpec{{
		Matches: []sieveMatchSpec{match("subject", "contains", `say "hi"`)},
		Actions: []sieveActionSpec{{Type: types.StringValue("discard")}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `header :contains "subject" "say \"hi\""`) {
		t.Errorf("quoting wrong:\n%s", out)
	}
}

func TestGenerateSieveRejectsUnknownAction(t *testing.T) {
	_, err := generateSieve([]sieveRuleSpec{{
		Actions: []sieveActionSpec{{Type: types.StringValue("explode")}},
	}})
	if err == nil || !strings.Contains(err.Error(), "unsupported action type") {
		t.Fatalf("want unsupported action error, got %v", err)
	}
}
