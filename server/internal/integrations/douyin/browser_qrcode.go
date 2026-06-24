package douyin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

var errDouyinQRCodePollBlocked = errors.New("douyin qrcode poll blocked by risk control")

// captureScript creates the state object AND intercepts fetch/XHR to
// passively capture QR code API responses. The wrappers are carefully
// crafted to match native function signatures so bdms.js cannot detect them:
//   - fetch.length === 1 (single `input` parameter)
//   - fetch.prototype is deleted (native fetch has no prototype)
//   - XMLHttpRequest.prototype === native _XHR.prototype
//   - toString() returns [native code] for both
//
// No browser prototypes (Canvas, WebGL, navigator, screen) are touched.
const captureScript = `(function(){
if (window.__rayleaDouyinLogin) return;

var state = {qrcode:null, lastPoll:null};
Object.defineProperty(window, '__rayleaDouyinLogin', {value:state, configurable:false, writable:false});

var _fetch = window.fetch;
var _XHR = window.XMLHttpRequest;

function match(url, path){ return typeof url==='string' && url.indexOf(path)!==-1; }
function capture(url, text){
if (!text) return;
try {
var p = JSON.parse(text);
if (match(url, '` + douyinBrowserQRCodePath + `')) state.qrcode = p;
if (match(url, '` + douyinBrowserQRConnectPath + `')) state.lastPoll = p;
} catch(e) {}
}

// Fetch wrapper — length=1, no prototype (matches native fetch).
window.fetch = function fetch(input){
var url = typeof input==='string' ? input : (input && input.url) || '';
return _fetch.call(this, input, arguments[1]).then(function(r){
if (match(url, '` + douyinBrowserQRCodePath + `') || match(url, '` + douyinBrowserQRConnectPath + `')){
try { r.clone().text().then(function(t){ capture(url, t); }).catch(function(){}); } catch(e) {}
}
return r;
});
};
try { delete window.fetch.prototype; } catch(e) {}
window.fetch.toString = function(){ return 'function fetch() { [native code] }'; };

// XMLHttpRequest wrapper — inherits native prototype.
var xhrWrapper = function XMLHttpRequest(){
var x = new _XHR();
var url = '';
var _open = x.open;
x.open = function(m, u){ url = String(u || ''); return _open.apply(x, arguments); };
x.addEventListener('load', function(){
if (match(url, '` + douyinBrowserQRCodePath + `') || match(url, '` + douyinBrowserQRConnectPath + `')){
capture(url, x.responseText || '');
}
});
return x;
};
xhrWrapper.prototype = _XHR.prototype;
xhrWrapper.toString = function(){ return 'function XMLHttpRequest() { [native code] }'; };
window.XMLHttpRequest = xhrWrapper;

// Critical: make Function.prototype.toString.call(fetch) also return [native code].
// bdms.js uses this to verify that fetch/XHR are the real native functions.
var _funcToString = Function.prototype.toString;
Function.prototype.toString = function toString(){
if (this === window.fetch || this === window.XMLHttpRequest) {
return this.toString();
}
return _funcToString.call(this);
};
Function.prototype.toString.toString = function(){ return 'function toString() { [native code] }'; };
})();`

func installCaptureScript(ctx context.Context) error {
	_, err := page.AddScriptToEvaluateOnNewDocument(captureScript).Do(ctx)
	return err
}

func callQRCodeAPI(ctx context.Context) (json.RawMessage, error) {
	js := `(function(){
var u = '/passport/web/get_qrcode/?aid=6383&service=' + encodeURIComponent('https://www.douyin.com/') + '&need_logo=true&t=' + Date.now();
return fetch(u, {credentials: 'include'}).then(function(r){ return r.text(); }).then(function(t){
var d = JSON.parse(t);
if (d && d.data) { window.__rayleaDouyinLogin.qrcode = d; }
return t;
}).catch(function(e){ return '{"error":"'+e.message+'"}'; });
})()`

	var raw string
	if err := chromedp.Evaluate(js, &raw, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
		return p.WithAwaitPromise(true)
	}).Do(ctx); err != nil {
		return nil, err
	}
	var check struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(raw), &check); err == nil && check.Error != "" {
		return nil, fmt.Errorf("douyin qrcode api call: %s", check.Error)
	}
	return json.RawMessage(raw), nil
}

func readQRCodeFromState(ctx context.Context) (string, error) {
	var raw string
	if err := chromedp.Evaluate(`(function(){var s=window.__rayleaDouyinLogin; return s && s.qrcode ? JSON.stringify(s.qrcode) : "";})()`, &raw).Do(ctx); err != nil {
		return "", err
	}
	return raw, nil
}

func readPollFromState(ctx context.Context) (string, error) {
	runCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var raw string
	if err := chromedp.Run(runCtx, chromedp.Evaluate(`(function(){var s=window.__rayleaDouyinLogin; return s && s.lastPoll ? JSON.stringify(s.lastPoll) : "";})()`, &raw)); err != nil {
		return "", err
	}
	return raw, nil
}

func waitDouyinBrowserQRCode(ctx context.Context, now time.Time) (BrowserCreateResult, error) {
	initialWait := 3 * time.Second
	select {
	case <-ctx.Done():
		return BrowserCreateResult{}, ctx.Err()
	case <-time.After(initialWait):
	}

	if raw, err := readQRCodeFromState(ctx); err == nil && strings.TrimSpace(raw) != "" {
		return parseDouyinBrowserQRCodeResponse([]byte(raw), now)
	}

	for attempt := 0; attempt < 5; attempt++ {
		raw, err := callQRCodeAPI(ctx)
		if err == nil {
			return parseDouyinBrowserQRCodeResponse(raw, now)
		}
		select {
		case <-ctx.Done():
			return BrowserCreateResult{}, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}

	if raw, err := readQRCodeFromState(ctx); err == nil && strings.TrimSpace(raw) != "" {
		return parseDouyinBrowserQRCodeResponse([]byte(raw), now)
	}

	return BrowserCreateResult{}, fmt.Errorf("douyin browser: unable to obtain QR code after %v", time.Since(now))
}

func readDouyinBrowserPollState(ctx context.Context) (string, error) {
	raw, err := readPollFromState(ctx)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(raw) == "" {
		return "pending_scan", nil
	}
	return parseDouyinBrowserPollState([]byte(raw))
}

func parseDouyinBrowserQRCodeResponse(body []byte, now time.Time) (BrowserCreateResult, error) {
	var response struct {
		Message string `json:"message"`
		Data    struct {
			ErrorCode      int    `json:"error_code"`
			Description    string `json:"description"`
			QRCodeIndexURL string `json:"qrcode_index_url"`
			Token          string `json:"token"`
			ExpireTime     int64  `json:"expire_time"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return BrowserCreateResult{}, fmt.Errorf("douyin browser qrcode response: %w", err)
	}
	if response.Data.ErrorCode != 0 {
		return BrowserCreateResult{}, fmt.Errorf("douyin browser qrcode create failed: %s", firstNonEmpty(response.Data.Description, response.Message, "invalid response"))
	}
	token := strings.TrimSpace(response.Data.Token)
	qrcodeURL := strings.TrimSpace(response.Data.QRCodeIndexURL)
	if token == "" || qrcodeURL == "" {
		return BrowserCreateResult{}, fmt.Errorf("douyin browser qrcode create missing token or qrcode url")
	}
	expiresAt := now.Add(3 * time.Minute)
	if response.Data.ExpireTime > 0 {
		remoteExpiresAt := time.Unix(response.Data.ExpireTime, 0).UTC()
		if remoteExpiresAt.After(now) {
			expiresAt = remoteExpiresAt
		}
	}
	return BrowserCreateResult{
		Token:     token,
		QRCodeURL: qrcodeURL,
		ExpiresAt: expiresAt,
	}, nil
}

func parseDouyinBrowserPollState(body []byte) (string, error) {
	var response struct {
		Message string `json:"message"`
		Data    struct {
			ErrorCode   int             `json:"error_code"`
			Description string          `json:"description"`
			Status      json.RawMessage `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("douyin browser qrcode poll response: %w", err)
	}
	if response.Data.ErrorCode != 0 {
		message := firstNonEmpty(response.Data.Description, response.Message, "invalid response")
		if isDouyinQRCodePollBlocked(message) {
			return "", fmt.Errorf("%w: %s", errDouyinQRCodePollBlocked, message)
		}
		return "", fmt.Errorf("douyin browser qrcode poll failed: %s", message)
	}
	switch douyinStatus(response.Data.Status) {
	case "", "1", "new":
		return "pending_scan", nil
	case "2", "scan", "scanned":
		return "pending_confirm", nil
	case "3", "confirm", "confirmed", "success", "succeeded":
		return "succeeded", nil
	case "4", "5", "expire", "expired", "cancel", "canceled", "cancelled":
		return "expired", nil
	default:
		return "", fmt.Errorf("douyin browser qrcode poll status %s", string(response.Data.Status))
	}
}

func isDouyinQRCodePollBlocked(message string) bool {
	message = strings.TrimSpace(message)
	return strings.Contains(message, "安全风险") || strings.Contains(message, "已阻止此次访问")
}
