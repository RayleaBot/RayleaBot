package textsafe

import "testing"

func TestSanitizeStringRemovesUnsafeControlsAndPreservesOrdinaryUnicode(t *testing.T) {
	t.Parallel()

	value := "测试群名片\u2066，测试用户昵称\u202e~喵\u2069\tok\n下一行\u0000\u2028尾巴\ufeff"
	got := SanitizeString(value)
	want := "测试群名片，测试用户昵称~喵\tok\n下一行\n尾巴"

	if got != want {
		t.Fatalf("unexpected sanitized string: got %q want %q", got, want)
	}
}

func TestSanitizeStringDropsInvalidUTF8Bytes(t *testing.T) {
	t.Parallel()

	value := string([]byte{'A', 0xff, 'B'})
	if got := SanitizeString(value); got != "AB" {
		t.Fatalf("unexpected invalid utf8 sanitization: got %q want %q", got, "AB")
	}
}

func TestSanitizeAnyRecursivelyClonesAndSanitizesStrings(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"sender": map[string]any{
			"nickname": "测试用户\u202e昵称",
		},
		"segments": []any{
			map[string]any{
				"data": map[string]any{
					"text": "hello\u2066world",
				},
			},
		},
	}

	output := SanitizeAny(input).(map[string]any)
	sender := output["sender"].(map[string]any)
	segments := output["segments"].([]any)
	segmentData := segments[0].(map[string]any)["data"].(map[string]any)

	if sender["nickname"] != "测试用户昵称" {
		t.Fatalf("unexpected sanitized sender nickname: %#v", sender["nickname"])
	}
	if segmentData["text"] != "helloworld" {
		t.Fatalf("unexpected sanitized segment text: %#v", segmentData["text"])
	}
	if input["sender"].(map[string]any)["nickname"] != "测试用户\u202e昵称" {
		t.Fatalf("input should remain unchanged: %#v", input)
	}
}

func TestTruncateRunesKeepsWholeUnicodeCharacters(t *testing.T) {
	t.Parallel()

	value := "终末地🙂测试"
	if got := TruncateRunes(value, 4, "..."); got != "终末地🙂..." {
		t.Fatalf("unexpected truncated string: got %q want %q", got, "终末地🙂...")
	}
	if got := TruncateRunes(value, 99, "..."); got != value {
		t.Fatalf("unexpected untouched string: got %q want %q", got, value)
	}
}
