package asynq

import (
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"
	"unicode/utf8"
)

func TestOptionParserQuotedDelimiters(t *testing.T) {
	for _, value := range []string{"a)b", "a(b)", "((nested))", "", `quote"slash\)`, "line\n)\t(\x00", "佇列(一)"} {
		for _, opt := range []Option{Queue(value), Header(value, value)} {
			got, err := parseOption(opt.String())
			if err != nil || got.Type() != opt.Type() || !reflect.DeepEqual(got.Value(), opt.Value()) {
				t.Errorf("option %q lost quoted argument: %v/%v", opt.String(), got, err)
			}
		}
	}
}

func TestOptionParserMalformedFrames(t *testing.T) {
	for _, value := range []string{"", "Queue", "MaxRetry", "(", ")", "Queue(", `Queue("x"`, `Queue("x")junk`, `Queue("x") `, `Queue("x"))`, `Header(["a","b"])junk`, `Header(["a","b"]))`, `MaxRetry(1)extra)`, `Queue(unquoted)`, `Header(["a",])`, `Unknown(x)`, `Unknown("x")`} {
		got, err := parseOption(value)
		if err == nil || got != nil {
			t.Errorf("malformed option %q accepted: %v", value, got)
		}
	}
}

func TestOptionParserExistingArgumentSemantics(t *testing.T) {
	// Header's existing JSON decoding behavior remains deliberately unchanged.
	for _, tc := range []struct {
		input string
		kind  OptionType
		value interface{}
	}{
		{`Queue("")`, QueueOpt, ""},
		{`Queue('x')`, QueueOpt, "x"},
		{"Queue(`raw(value)`)", QueueOpt, "raw(value)"},
		{`Header(null)`, HeaderOpt, [2]string{}},
		{`Header([])`, HeaderOpt, [2]string{}},
		{`Header(["a"])`, HeaderOpt, [2]string{"a", ""}},
		{`Header(["a","b","ignored"])`, HeaderOpt, [2]string{"a", "b"}},
		{`MaxRetry(-1)`, MaxRetryOpt, 0},
	} {
		got, err := parseOption(tc.input)
		if err != nil || got.Type() != tc.kind || !reflect.DeepEqual(got.Value(), tc.value) {
			t.Errorf("existing input %q changed: %v/%v", tc.input, got, err)
		}
	}
}

func optionQuotedRoundTrip(queue, key, value string) bool {
	opt := Queue(queue)
	got, err := parseOption(opt.String())
	if err != nil || got.Type() != opt.Type() || !reflect.DeepEqual(got.Value(), opt.Value()) {
		return false
	}
	// JSON strings represent Unicode text and replace invalid UTF-8. Queue
	// rendering uses Go quoting and can retain arbitrary string bytes.
	if !utf8.ValidString(key) || !utf8.ValidString(value) {
		return true
	}
	opt = Header(key, value)
	got, err = parseOption(opt.String())
	return err == nil && got.Type() == opt.Type() && reflect.DeepEqual(got.Value(), opt.Value())
}

func TestOptionParserRoundTripProperty(t *testing.T) {
	if err := quick.Check(optionQuotedRoundTrip, &quick.Config{MaxCount: 10000, Rand: rand.New(rand.NewSource(20261004))}); err != nil {
		t.Fatal(err)
	}
}

func FuzzOptionParser(f *testing.F) {
	f.Add("a)b", "((key))", "quote\"\\)")
	f.Add("", "", "")
	f.Add("佇列(一)", "key", "(value)")
	f.Fuzz(func(t *testing.T, queue, key, value string) {
		if !optionQuotedRoundTrip(queue, key, value) {
			t.Fatal("quoted option serialization lost values")
		}
		// Arbitrary malformed data must return an option or error, never panic.
		_, _ = parseOption(queue)
	})
}
