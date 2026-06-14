package session

import "time"

const (
	cookieInfoURL           = "https://passport.bilibili.com/x/passport-login/web/cookie/info"
	cookieRefreshURL        = "https://passport.bilibili.com/x/passport-login/web/cookie/refresh"
	cookieRefreshConfirmURL = "https://passport.bilibili.com/x/passport-login/web/confirm/refresh"
	correspondBaseURL       = "https://www.bilibili.com/correspond/1/"
	biliTicketURL           = "https://api.bilibili.com/bapis/bilibili.api.ticket.v1.Ticket/GenWebTicket"
	buvidSPIURL             = "https://api.bilibili.com/x/frontend/finger/spi"

	biliTicketKeyID   = "ec02"
	biliTicketHMACKey = "XgwSnGZ1p"

	refreshCheckInterval = 6 * time.Hour
	wbiKeyTTL            = 12 * time.Hour
	deviceCookieTTL      = 24 * time.Hour
)

const correspondPublicKeyPEM = `-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDLgd2OAkcGVtoE3ThUREbio0Eg
Uc/prcajMKXvkCKFCWhJYJcLkcM2DKKcSeFpD/j6Boy538YXnR6VhcuUJOhH2x71
nzPjfdTcqMz7djHum0qSZA0AyCBDABUqCrfNgCiJ00Ra7GmRj+YCK1NJEuewlb40
JNrRuoEUXpabUzGB8QIDAQAB
-----END PUBLIC KEY-----`

var wbiMixinKeyEncTab = []int{
	46, 47, 18, 2, 53, 8, 23, 32, 15, 50, 10, 31, 58, 3, 45, 35,
	27, 43, 5, 49, 33, 9, 42, 19, 29, 28, 14, 39, 12, 38, 41, 13,
	37, 48, 7, 16, 24, 55, 40, 61, 26, 17, 0, 1, 60, 51, 30, 4,
	22, 25, 54, 21, 56, 59, 6, 63, 57, 62, 11, 36, 20, 34, 44, 52,
}
