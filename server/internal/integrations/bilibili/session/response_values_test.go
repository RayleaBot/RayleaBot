package session

import "testing"

func TestExtractVVoucherFromBody(t *testing.T) {
	t.Parallel()
	body := []byte(`{"code":-352,"message":"风控校验失败","data":{"v_voucher":"test_voucher_abc123","url":""}}`)
	got := ExtractVVoucher(body)
	if got != "test_voucher_abc123" {
		t.Fatalf("ExtractVVoucher = %q, want test_voucher_abc123", got)
	}
}

func TestExtractVVoucherEmptyBody(t *testing.T) {
	t.Parallel()
	if got := ExtractVVoucher([]byte{}); got != "" {
		t.Fatalf("ExtractVVoucher(empty) = %q, want \"\"", got)
	}
	if got := ExtractVVoucher([]byte(`{}`)); got != "" {
		t.Fatalf("ExtractVVoucher(no data) = %q, want \"\"", got)
	}
	if got := ExtractVVoucher([]byte(`{"data":{}}`)); got != "" {
		t.Fatalf("ExtractVVoucher(no v_voucher) = %q, want \"\"", got)
	}
}
