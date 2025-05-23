package main

import (
	"log/slog"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"github.com/imroc/req/v3"
	"go.nhat.io/cookiejar"
	"golang.org/x/net/publicsuffix"
)

func generateGreasedChromeVersion(chromeVersion int) string {
	greasedChars := []rune{' ', '(', ':', '-', '.', '/', ')', ';', '=', '?', '_'}
	greasedVersions := []int{8, 99, 24}

	v := strconv.Itoa(chromeVersion)

	grease1 := string(greasedChars[rand.IntN(len(greasedChars))])             //nolint:gosec
	grease2 := string(greasedChars[rand.IntN(len(greasedChars))])             //nolint:gosec
	grease3 := strconv.Itoa(greasedVersions[rand.IntN(len(greasedVersions))]) //nolint:gosec

	brands := []string{
		`"Chromium";v="` + v + `"`,
		`"Google Chrome";v="` + v + `"`,
		`"Not` + grease1 + "A" + grease2 + `Brand";v="` + grease3 + `"`,
	}

	rand.Shuffle(len(brands), func(i, j int) {
		brands[i], brands[j] = brands[j], brands[i]
	})

	return strings.Join(brands, ", ")
}

// createHTTPClient instantiates a req.Client that impersonates a generic, mainline Chrome browser.
func createHTTPClient(ctx *AppContext) *req.Client {
	client := req.C()
	if ctx.Cfg.HTTPDebug {
		client.DevMode()
	}

	if ctx.Cfg.CookieJarFileName != "" {
		slog.Debug("using persistent cookie jar", "filename", ctx.Cfg.CookieJarFileName)

		jar := cookiejar.NewPersistentJar(
			cookiejar.WithFilePath(ctx.Cfg.CookieJarFileName),
			cookiejar.WithAutoSync(true),
			cookiejar.WithPublicSuffixList(publicsuffix.List),
		)
		client.SetCookieJar(jar)
	}

	client.EnableAutoDecompress()
	client.SetMaxResponseHeaderBytes(2 * 1 << 20) // 2 MB
	client.SetTimeout(50 * time.Second)
	client.SetTLSHandshakeTimeout(25 * time.Second)
	client.SetIdleConnTimeout(120 * time.Second)

	userAgent := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/" + strconv.Itoa(ctx.Cfg.ChromeVersion) + ".0.0.0 Safari/537.36"

	client.ImpersonateChrome()
	client.SetCommonHeaders(map[string]string{
		"User-Agent":         userAgent,
		"Sec-Ch-Ua":          generateGreasedChromeVersion(ctx.Cfg.ChromeVersion),
		"Sec-Ch-Ua-Platform": "\"Windows\"",
		"Sec-Ch-Ua-Mobile":   "?0",

		"Sec-Fetch-Site": "same-origin",
		"Sec-Fetch-User": "?1",

		"Accept-Language": "en,ru;q=0.9",
		"Accept-Encoding": "gzip, deflate, br, zstd",
	})

	// These are set by ImpersonateChrome() and Chrome does not normally send them while interacting with HH,
	// so we have to delete them
	client.Headers.Del("Pragma")
	client.Headers.Del("Cache-Control")

	return client
}

// setGSSHeaders mirrors a bunch of cookie values to their corresponding HTTP headers,
// to maintain parity with the HH web frontend.
func setGSSHeaders(cl *req.Client, r *req.Request) {
	for _, cookie := range cl.Cookies {
		switch cookie.Name {
		case "gsscgib-w-hh":
			r.Headers.Set("x-gib-gsscgib-w-hh", cookie.Value)
		case "fgsscgib-w-hh":
			r.Headers.Set("x-gib-fgsscgib-w-hh", cookie.Value)
		}
	}
}
