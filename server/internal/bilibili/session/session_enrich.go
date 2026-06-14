package session

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bilibili/fingerprint"
)

func (c *SessionClient) enrichCookie(ctx context.Context, cookie string) (string, bool, error) {
	values := cookieValues(cookie)
	updates := map[string]string{}
	if strings.TrimSpace(values["buvid3"]) == "" || strings.TrimSpace(values["buvid4"]) == "" {
		device, err := c.ensureDeviceCookies(ctx, cookie)
		if err != nil {
			return cookie, false, err
		}
		if values["buvid3"] == "" && device.Buvid3 != "" {
			updates["buvid3"] = device.Buvid3
		}
		if values["buvid4"] == "" && device.Buvid4 != "" {
			updates["buvid4"] = device.Buvid4
		}
		if values["b_nut"] == "" {
			updates["b_nut"] = strconv.FormatInt(c.now().Unix(), 10)
		}
	}
	ticketExpires := int64Value(values["bili_ticket_expires"])
	if strings.TrimSpace(values["bili_ticket"]) == "" || ticketExpires <= c.now().Add(30*time.Minute).Unix() {
		ticket, err := c.ensureBiliTicket(ctx, cookie)
		if err != nil {
			return cookie, false, err
		}
		if ticket.Ticket != "" {
			updates["bili_ticket"] = ticket.Ticket
			updates["bili_ticket_expires"] = strconv.FormatInt(ticket.ExpiresAt.Unix(), 10)
		}
	}
	if strings.TrimSpace(values["buvid_fp"]) == "" {
		updates["buvid_fp"] = fingerprint.GenBuvidFP(c.identity.UserAgent())
	}
	if strings.TrimSpace(values["_uuid"]) == "" {
		updates["_uuid"] = fingerprint.GenUUID()
	}
	if len(updates) == 0 {
		return cookie, false, nil
	}
	return mergeCookieValues(cookie, updates), true, nil
}

func (c *SessionClient) ensureDeviceCookies(ctx context.Context, cookie string) (deviceCookieCache, error) {
	now := c.now()
	c.mu.Lock()
	if c.device.Buvid3 != "" && c.device.Buvid4 != "" && now.Before(c.device.ExpiresAt) {
		device := c.device
		c.mu.Unlock()
		return device, nil
	}
	c.mu.Unlock()

	body, _, status, err := c.send(ctx, http.MethodGet, buvidSPIURL, cookie, nil)
	if err != nil {
		return deviceCookieCache{}, err
	}
	var doc struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Buvid3 string `json:"b_3"`
			Buvid4 string `json:"b_4"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return deviceCookieCache{}, &Error{Kind: ErrorInvalidResponse, HTTPStatus: status, Message: responseExcerpt(body), Err: err}
	}
	if doc.Code != 0 {
		return deviceCookieCache{}, apiError(status, doc.Code, doc.Message, body)
	}
	device := deviceCookieCache{
		Buvid3:    strings.TrimSpace(doc.Data.Buvid3),
		Buvid4:    strings.TrimSpace(doc.Data.Buvid4),
		ExpiresAt: now.Add(deviceCookieTTL),
	}
	if device.Buvid3 == "" {
		device.Buvid3 = fingerprint.GenBuvid("XX")
	}
	if device.Buvid4 == "" {
		device.Buvid4 = fingerprint.GenBuvid("XY")
	}
	c.mu.Lock()
	c.device = device
	c.mu.Unlock()
	return device, nil
}

func (c *SessionClient) ensureBiliTicket(ctx context.Context, cookie string) (ticketCache, error) {
	now := c.now()
	c.mu.Lock()
	if c.ticket.Ticket != "" && now.Before(c.ticket.ExpiresAt.Add(-30*time.Minute)) {
		ticket := c.ticket
		c.mu.Unlock()
		return ticket, nil
	}
	c.mu.Unlock()

	ts := strconv.FormatInt(now.Unix(), 10)
	values := url.Values{
		"key_id":      {biliTicketKeyID},
		"hexsign":     {biliTicketHexSign(ts)},
		"context[ts]": {ts},
		"csrf":        {cookieValues(cookie)["bili_jct"]},
	}
	body, _, status, err := c.send(ctx, http.MethodPost, biliTicketURL+"?"+values.Encode(), cookie, nil)
	if err != nil {
		return ticketCache{}, err
	}
	var doc struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Ticket    string `json:"ticket"`
			CreatedAt int64  `json:"created_at"`
			TTL       int64  `json:"ttl"`
			Nav       struct {
				Img string `json:"img"`
				Sub string `json:"sub"`
			} `json:"nav"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return ticketCache{}, &Error{Kind: ErrorInvalidResponse, HTTPStatus: status, Message: responseExcerpt(body), Err: err}
	}
	if doc.Code != 0 {
		return ticketCache{}, apiError(status, doc.Code, doc.Message, body)
	}
	expiresAt := now.Add(48 * time.Hour)
	if doc.Data.CreatedAt > 0 && doc.Data.TTL > 0 {
		expiresAt = time.Unix(doc.Data.CreatedAt+doc.Data.TTL, 0).UTC()
	}
	ticket := ticketCache{
		Ticket:    strings.TrimSpace(doc.Data.Ticket),
		ExpiresAt: expiresAt,
		WBI: wbiKeyCache{
			ImgKey:    extractWBIKey(doc.Data.Nav.Img),
			SubKey:    extractWBIKey(doc.Data.Nav.Sub),
			ExpiresAt: now.Add(wbiKeyTTL),
		},
	}
	c.mu.Lock()
	c.ticket = ticket
	if ticket.WBI.ImgKey != "" && ticket.WBI.SubKey != "" {
		c.wbi = ticket.WBI
	}
	c.mu.Unlock()
	return ticket, nil
}

func biliTicketHexSign(timestamp string) string {
	mac := hmac.New(sha256.New, []byte(biliTicketHMACKey))
	mac.Write([]byte("ts" + timestamp))
	return hex.EncodeToString(mac.Sum(nil))
}
