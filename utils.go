package main

import (
	"fmt"
	"net/url"

	"github.com/imroc/req/v3"
)

func buildHHURL(ctx *AppContext, pathWithQuery string) string {
	u1, err := url.Parse(ctx.Cfg.Endpoint)
	if err != nil {
		// This should not happen:
		// we don't cache the parsed endpoint URL but we still validate it anyway
		panic(fmt.Errorf("parsing HH endpoint URL: %w", err))
	}

	u2, err := url.Parse(pathWithQuery)
	if err != nil {
		// This should not happen too: all paths are hardcoded
		panic(fmt.Errorf("parsing path/query: %w", err))
	}

	u1.Path = u2.Path
	u1.RawQuery = u2.RawQuery
	return u1.String()
}

func getXSRFToken(resp *req.Response) string {
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "_xsrf" {
			return cookie.Value
		}
	}

	return ""
}
