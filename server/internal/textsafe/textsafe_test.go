package textsafe

import "testing"

func TestSanitizeStringRemovesUnsafeControlsAndPreservesOrdinaryUnicode(t *testing.T) {
	t.Parallel()

	value := "群星怒\u2066，大明云玩家\u202e~喵\u2069\tok\n下一行\u0000\u2028尾巴\ufeff"
	got := SanitizeString(value)
	want := "群星怒，大明云玩家~喵\tok\n下一行\n尾巴"

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
			"nickname": "没错\u202e，是魔法！",
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

	if sender["nickname"] != "没错，是魔法！" {
		t.Fatalf("unexpected sanitized sender nickname: %#v", sender["nickname"])
	}
	if segmentData["text"] != "helloworld" {
		t.Fatalf("unexpected sanitized segment text: %#v", segmentData["text"])
	}
	if input["sender"].(map[string]any)["nickname"] != "没错\u202e，是魔法！" {
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
