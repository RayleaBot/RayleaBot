package redact

import "testing"

func TestSensitiveTextPreservesCredentialBoundariesAndHeaderPrecedence(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, input, want string }{
		{"folded inner rune", "toKen=fixture", "toKen=[REDACTED]"},
		{"folded first rune at start", "ſecret=fixture", "ſecret=fixture"},
		{"folded first rune after word", "xſecret=fixture", "xſecret=[REDACTED]"},
		{"ASCII word prefix", "mytoken=fixture", "mytoken=fixture"},
		{"Unicode prefix", "测试token=fixture", "测试token=[REDACTED]"},
		{"form feed", "password\f:\tfixture", "password\f:\t[REDACTED]"},
		{"vertical tab", "password\v=fixture", "password\v=fixture"},
		{"Unicode space", "password\u00a0=fixture", "password\u00a0=fixture"},
		{"quoted value", "token='fixture'", "token='fixture'"},
		{"assignment punctuation", "token=first; password=second&keep=third", "token=[REDACTED]; password=[REDACTED]&keep=third"},
		{"header precedence", "Cookie: token=first; password=second\npassword=third", "Cookie: [REDACTED]\npassword=[REDACTED]"},
		{"header multiline whitespace", "Authorization:\r\n \tBearer fixture\r\nnext=value", "Authorization:\r\n \t[REDACTED]\r\nnext=value"},
		{"header whitespace value", "Cookie:  ", "Cookie: [REDACTED]"},
		{"invalid UTF8 boundary", "\xfftoken=fixture\xff&keep=\xff", "\xfftoken=[REDACTED]&keep=\xff"},
		{"already masked", "Cookie: [REDACTED]\ntoken=[REDACTED]", "Cookie: [REDACTED]\ntoken=[REDACTED]"},
		{"masked between credentials", "token=first&password=[REDACTED]; api_key=second", "token=[REDACTED]&password=[REDACTED]; api_key=[REDACTED]"},
		{"masked before credential", "token=[REDACTED]&password=second", "token=[REDACTED]&password=[REDACTED]"},
		{"placeholder prefix remains sensitive", "token=[REDACTED]suffix&Cookie: [REDACTED] suffix", "token=[REDACTED]&Cookie: [REDACTED]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := SensitiveText(tc.input); got != tc.want {
				t.Fatalf("SensitiveText(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
