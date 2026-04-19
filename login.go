package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/imroc/req/v3"
)

// hhAuthenticate authenticates against the HH backend.
func hhAuthenticate(ctx *AppContext, cl *req.Client, xsrf string) error {
	slog.Debug("authenticating in HH")

	r := cl.R()
	r.SetHeaders(map[string]string{
		"Sec-Fetch-Dest":   "empty",
		"Sec-Fetch-Mode":   "cors",
		"Sec-Fetch-Site":   "same-origin",
		"X-Requested-With": "XMLHTTPRequest",
		"X-Xsrftoken":      xsrf,
		"X-Hhtmsource":     "account_login",
		"X-Hhtmfrom":       "main",
		"Accept":           "application/json",
		"Referer":          buildHHURL(ctx, "/applicant/resumes?role=applicant"),
	})

	r.EnableForceMultipart()

	r.SetFormData(map[string]string{
		"accountType": "APPLICANT",
		"remember":    "true",
		"username":    ctx.Cfg.Login,
		"password":    ctx.Cfg.Password,
		"failUrl":     "/account/login?backurl=%2Fapplicant%2Fresumes&role=applicant",
		"captchaText": "",
	})

	setGSSHeaders(cl, r)

	resp, err := r.Post(buildHHURL(ctx, "/account/login?backurl=%2Fapplicant%2Fresumes&role=applicant"))
	if err != nil {
		return fmt.Errorf("sending HTTP request: %w", err)
	}

	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.Error("failed to close response body", "error", closeErr)
		}
	}()

	if !resp.IsSuccessState() {
		return fmt.Errorf("received an HTTP error: status code %v", resp.StatusCode)
	}

	var hhResponse struct {
		Recaptcha struct {
			IsBot bool `json:"isBot"`
		} `json:"recaptcha"`

		HHCaptcha struct {
			IsBot        bool   `json:"isBot"`
			CaptchaState string `json:"captchaState"`
		} `json:"hhcaptcha"`

		RedirectURL string `json:"redirectUrl"`
		LoginError  struct {
			Code        string `json:"code"`
			Translation string `json:"trl"`
		} `json:"loginError"`
	}

	err = json.NewDecoder(resp.Body).Decode(&hhResponse)
	if err != nil {
		return err
	}

	if hhResponse.Recaptcha.IsBot {
		return errors.New("triggered ReCaptcha bot protection")
	}

	if hhResponse.HHCaptcha.IsBot {
		return fmt.Errorf("triggered HHCaptcha bot protection: state = %v", hhResponse.HHCaptcha.CaptchaState)
	}

	if hhResponse.LoginError.Code != "" {
		return fmt.Errorf("authentication failure: %q (%q)", hhResponse.LoginError.Code, hhResponse.LoginError.Translation)
	}

	slog.Debug("authenticated successfully")
	return nil
}
