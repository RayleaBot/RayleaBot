package douyin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

const (
	douyinNetworkCaptureQueueSize = 16
	douyinNetworkResponseMaxBytes = 256 << 10
)

var errDouyinQRCodePollBlocked = errors.New("douyin qrcode poll blocked by risk control")

type douyinBrowserPollResponse struct {
	State       string
	RedirectURL string
}

type douyinNetworkResponseKind uint8

const (
	douyinNetworkResponseUnknown douyinNetworkResponseKind = iota
	douyinNetworkResponseQRCode
	douyinNetworkResponsePoll
)

type douyinNetworkResponse struct {
	requestID network.RequestID
	kind      douyinNetworkResponseKind
}

type douyinNetworkCapture struct {
	ctx     context.Context
	queue   chan douyinNetworkResponse
	updated chan struct{}

	mu       sync.Mutex
	qrcode   []byte
	lastPoll []byte
	pollSeq  uint64
}

func newDouyinNetworkCapture(ctx context.Context) *douyinNetworkCapture {
	capture := &douyinNetworkCapture{
		ctx:     ctx,
		queue:   make(chan douyinNetworkResponse, douyinNetworkCaptureQueueSize),
		updated: make(chan struct{}, 1),
	}
	chromedp.ListenTarget(ctx, func(event any) {
		response, ok := event.(*network.EventResponseReceived)
		if !ok || response.Response == nil {
			return
		}
		kind := classifyDouyinNetworkResponse(response.Response.URL)
		if kind == douyinNetworkResponseUnknown {
			return
		}
		select {
		case capture.queue <- douyinNetworkResponse{requestID: response.RequestID, kind: kind}:
		default:
		}
	})
	go capture.run()
	return capture
}

func (c *douyinNetworkCapture) run() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case response := <-c.queue:
			body, err := c.responseBody(response.requestID)
			if err != nil || !c.storeResponse(response.kind, body) {
				continue
			}
			select {
			case c.updated <- struct{}{}:
			default:
			}
		}
	}
}

func (c *douyinNetworkCapture) responseBody(requestID network.RequestID) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		var body []byte
		err := chromedp.Run(c.ctx, chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			body, err = network.GetResponseBody(requestID).Do(ctx)
			return err
		}))
		if err == nil {
			return body, nil
		}
		lastErr = err
		select {
		case <-c.ctx.Done():
			return nil, c.ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	return nil, lastErr
}

func (c *douyinNetworkCapture) storeResponse(kind douyinNetworkResponseKind, body []byte) bool {
	if len(body) == 0 || len(body) > douyinNetworkResponseMaxBytes {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	switch kind {
	case douyinNetworkResponseQRCode:
		c.qrcode = append(c.qrcode[:0], body...)
	case douyinNetworkResponsePoll:
		c.lastPoll = append(c.lastPoll[:0], body...)
		c.pollSeq++
	default:
		return false
	}
	return true
}

func (c *douyinNetworkCapture) latestQRCode() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]byte(nil), c.qrcode...)
}

func (c *douyinNetworkCapture) latestPoll() ([]byte, uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]byte(nil), c.lastPoll...), c.pollSeq
}

func classifyDouyinNetworkResponse(rawURL string) douyinNetworkResponseKind {
	endpoint, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || !strings.EqualFold(endpoint.Scheme, "https") {
		return douyinNetworkResponseUnknown
	}
	host := strings.ToLower(strings.TrimSpace(endpoint.Hostname()))
	if host != "www.douyin.com" && host != douyinBrowserLoginHost && host != "sso.douyin.com" {
		return douyinNetworkResponseUnknown
	}
	switch endpoint.Path {
	case douyinBrowserQRCodePath:
		return douyinNetworkResponseQRCode
	case douyinBrowserQRConnectPath:
		return douyinNetworkResponsePoll
	default:
		return douyinNetworkResponseUnknown
	}
}

func callQRCodeAPI(requestCtx, tabCtx context.Context) (json.RawMessage, error) {
	js := fmt.Sprintf(`(function(){
var u = %q + '?aid=6383&service=' + encodeURIComponent('https://www.douyin.com/') + '&need_logo=true&t=' + Date.now();
return fetch(u, {credentials: 'include'}).then(function(r){ return r.text(); }).catch(function(e){ return '{"error":"'+String(e && e.message || e)+'"}'; });
})()`, "https://"+douyinBrowserLoginHost+douyinBrowserQRCodePath)

	actionCtx, cancel := douyinBrowserActionContext(tabCtx, requestCtx, 5*time.Second)
	defer cancel()
	var raw string
	if err := chromedp.Run(actionCtx, chromedp.Evaluate(js, &raw, func(params *runtime.EvaluateParams) *runtime.EvaluateParams {
		return params.WithAwaitPromise(true)
	})); err != nil {
		return nil, err
	}
	var check struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(raw), &check); err == nil && check.Error != "" {
		return nil, fmt.Errorf("douyin qrcode api call failed")
	}
	if len(raw) > douyinNetworkResponseMaxBytes {
		return nil, fmt.Errorf("douyin qrcode api response is too large")
	}
	return json.RawMessage(raw), nil
}

func waitDouyinBrowserQRCode(requestCtx, tabCtx context.Context, capture *douyinNetworkCapture, now time.Time) (BrowserCreateResult, error) {
	initialTimer := time.NewTimer(5 * time.Second)
	defer initialTimer.Stop()
	for {
		if raw := capture.latestQRCode(); len(raw) > 0 {
			result, err := parseDouyinBrowserQRCodeResponse(raw, now)
			result.pollMode = browserQRCodePollCaptured
			return result, err
		}
		select {
		case <-requestCtx.Done():
			return BrowserCreateResult{}, requestCtx.Err()
		case <-tabCtx.Done():
			return BrowserCreateResult{}, tabCtx.Err()
		case <-capture.updated:
			continue
		case <-initialTimer.C:
			goto fallback
		}
	}

fallback:
	for attempt := 0; attempt < 3; attempt++ {
		raw, err := callQRCodeAPI(requestCtx, tabCtx)
		if err == nil {
			result, parseErr := parseDouyinBrowserQRCodeResponse(raw, now)
			result.pollMode = browserQRCodePollActiveToken
			return result, parseErr
		}
		select {
		case <-requestCtx.Done():
			return BrowserCreateResult{}, requestCtx.Err()
		case <-tabCtx.Done():
			return BrowserCreateResult{}, tabCtx.Err()
		case <-time.After(time.Second):
		}
	}
	if raw := capture.latestQRCode(); len(raw) > 0 {
		result, err := parseDouyinBrowserQRCodeResponse(raw, now)
		result.pollMode = browserQRCodePollCaptured
		return result, err
	}
	return BrowserCreateResult{}, fmt.Errorf("douyin browser could not obtain a QR code")
}

func readDouyinBrowserPollState(capture *douyinNetworkCapture) (string, uint64, error) {
	raw, sequence := capture.latestPoll()
	if len(raw) == 0 {
		return "pending_scan", sequence, nil
	}
	state, err := parseDouyinBrowserPollState(raw)
	return state, sequence, err
}

func pollDouyinFallbackQRCode(requestCtx, tabCtx context.Context, token string) (douyinBrowserPollResponse, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return douyinBrowserPollResponse{}, fmt.Errorf("douyin browser qrcode poll token is missing")
	}
	checkURL := "https://" + douyinBrowserLoginHost + douyinBrowserQRConnectPath + "?" + url.Values{
		"aid":     {"6383"},
		"service": {douyinServiceURL},
		"token":   {token},
		"t":       {fmt.Sprintf("%d", time.Now().UnixMilli())},
	}.Encode()
	encodedURL, _ := json.Marshal(checkURL)
	script := fmt.Sprintf(`(function(){
return fetch(%s, {credentials: 'include'}).then(function(r){ return r.text(); }).catch(function(e){ return '{"error":"'+String(e && e.message || e)+'"}'; });
})()`, encodedURL)

	actionCtx, cancel := douyinBrowserActionContext(tabCtx, requestCtx, 5*time.Second)
	defer cancel()
	var raw string
	if err := chromedp.Run(actionCtx, chromedp.Evaluate(script, &raw, func(params *runtime.EvaluateParams) *runtime.EvaluateParams {
		return params.WithAwaitPromise(true)
	})); err != nil {
		return douyinBrowserPollResponse{}, err
	}
	var check struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(raw), &check); err == nil && check.Error != "" {
		return douyinBrowserPollResponse{}, fmt.Errorf("douyin browser qrcode poll request failed")
	}
	if len(raw) > douyinNetworkResponseMaxBytes {
		return douyinBrowserPollResponse{}, fmt.Errorf("douyin browser qrcode poll response is too large")
	}
	return parseDouyinBrowserPollResponse([]byte(raw))
}

func followDouyinBrowserRedirect(requestCtx, tabCtx context.Context, rawURL string) error {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil
	}
	endpoint, err := url.Parse(rawURL)
	if err != nil || endpoint.User != nil || !strings.EqualFold(endpoint.Scheme, "https") ||
		!thirdparty.HostMatches(endpoint.Hostname(), "douyin.com", "amemv.com", "bytedance.com") {
		return fmt.Errorf("douyin browser qrcode redirect is invalid")
	}
	actionCtx, cancel := douyinBrowserActionContext(tabCtx, requestCtx, 5*time.Second)
	defer cancel()
	if err := chromedp.Run(actionCtx, chromedp.Navigate(endpoint.String())); err != nil {
		return fmt.Errorf("douyin browser qrcode redirect failed")
	}
	return nil
}

type douyinPageSignals struct {
	URL                  string `json:"url"`
	HasUserLogin         bool   `json:"has_user_login"`
	VerificationRequired bool   `json:"verification_required"`
}

func readDouyinPageSignals(requestCtx, tabCtx context.Context) (douyinPageSignals, error) {
	const script = `(function(){
function visible(el){
  if (!el) return false;
  var style = window.getComputedStyle(el);
  if (!style || style.display === 'none' || style.visibility === 'hidden' || Number(style.opacity) === 0) return false;
  var rect = el.getBoundingClientRect();
  return rect.width > 0 && rect.height > 0;
}
var selectors = [
  'input[autocomplete="one-time-code"]',
  'input[placeholder*="验证码"]',
  'input[placeholder*="短信"]',
  'iframe[src*="captcha"]',
  '[class*="captcha"]',
  '[id*="captcha"]',
  '[class*="slider"]'
];
var challenge = selectors.some(function(selector){
  return Array.prototype.some.call(document.querySelectorAll(selector), visible);
});
if (!challenge) {
  var text = document.body ? document.body.innerText || '' : '';
  challenge = ['短信验证','安全验证','拖动滑块','输入验证码','验证身份'].some(function(term){ return text.indexOf(term) !== -1; });
}
var login = false;
try {
  var marker = localStorage.getItem('HasUserLogin');
  login = marker === '1' || marker === 'true';
} catch (e) {}
return {url: String(location.href || ''), has_user_login: login, verification_required: challenge};
})()`
	actionCtx, cancel := douyinBrowserActionContext(tabCtx, requestCtx, 3*time.Second)
	defer cancel()
	var signals douyinPageSignals
	if err := chromedp.Run(actionCtx, chromedp.Evaluate(script, &signals)); err != nil {
		return douyinPageSignals{}, err
	}
	return signals, nil
}

func douyinBrowserActionContext(tabCtx, requestCtx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	actionCtx, cancel := context.WithTimeout(tabCtx, timeout)
	stop := context.AfterFunc(requestCtx, cancel)
	return actionCtx, func() {
		stop()
		cancel()
	}
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
	return BrowserCreateResult{Token: token, QRCodeURL: qrcodeURL, ExpiresAt: expiresAt}, nil
}

func parseDouyinBrowserPollState(body []byte) (string, error) {
	response, err := parseDouyinBrowserPollResponse(body)
	return response.State, err
}

func parseDouyinBrowserPollResponse(body []byte) (douyinBrowserPollResponse, error) {
	var response struct {
		ErrorCode   int    `json:"error_code"`
		Description string `json:"description"`
		Message     string `json:"message"`
		Data        struct {
			ErrorCode   int             `json:"error_code"`
			Description string          `json:"description"`
			Status      json.RawMessage `json:"status"`
			RedirectURL string          `json:"redirect_url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return douyinBrowserPollResponse{}, fmt.Errorf("douyin browser qrcode poll response: %w", err)
	}
	if response.ErrorCode != 0 {
		message := firstNonEmpty(response.Description, response.Message, "invalid response")
		if isDouyinQRCodePollBlocked(message) {
			return douyinBrowserPollResponse{}, fmt.Errorf("%w: risk control", errDouyinQRCodePollBlocked)
		}
		return douyinBrowserPollResponse{}, fmt.Errorf("douyin browser qrcode poll failed")
	}
	if response.Data.ErrorCode != 0 {
		message := firstNonEmpty(response.Data.Description, response.Message, "invalid response")
		if isDouyinQRCodePollBlocked(message) {
			return douyinBrowserPollResponse{}, fmt.Errorf("%w: risk control", errDouyinQRCodePollBlocked)
		}
		return douyinBrowserPollResponse{}, fmt.Errorf("douyin browser qrcode poll failed")
	}
	result := douyinBrowserPollResponse{RedirectURL: strings.TrimSpace(response.Data.RedirectURL)}
	switch douyinStatus(response.Data.Status) {
	case "", "1", "new":
		result.State = thirdparty.QRLoginStatePendingScan
	case "2", "scan", "scanned":
		result.State = thirdparty.QRLoginStatePendingConfirm
	case "3", "confirm", "confirmed", "success", "succeeded":
		result.State = thirdparty.QRLoginStateSucceeded
	case "4", "5", "expire", "expired", "cancel", "canceled", "cancelled":
		result.State = thirdparty.QRLoginStateExpired
	default:
		return douyinBrowserPollResponse{}, fmt.Errorf("douyin browser qrcode poll returned an unknown state")
	}
	return result, nil
}

func isDouyinQRCodePollBlocked(message string) bool {
	message = strings.TrimSpace(message)
	return strings.Contains(message, "安全风险") || strings.Contains(message, "已阻止此次访问")
}
