// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"sync"
	"strings"

	"forgejo.org/modules/base"
	"forgejo.org/modules/cache"
	"forgejo.org/modules/hcaptcha"
	"forgejo.org/modules/log"
	"forgejo.org/modules/mcaptcha"
	"forgejo.org/modules/recaptcha"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/turnstile"

	mc "code.forgejo.org/go-chi/cache"
	"code.forgejo.org/go-chi/captcha"
)

var (
	imageCaptchaOnce sync.Once
	imageCachePrefix = "captcha:"
)

type imageCaptchaStore struct {
	c mc.Cache
}

func (c *imageCaptchaStore) Set(id string, digits []byte) {
	if err := c.c.Put(imageCachePrefix+id, string(digits), int64(captcha.Expiration.Seconds())); err != nil {
		log.Error("Couldn't store captcha cache for %q: %v", id, err)
	}
}

func (c *imageCaptchaStore) Get(id string, clear bool) (digits []byte) {
	val, ok := c.c.Get(imageCachePrefix + id).(string)
	if !ok {
		return digits
	}

	if clear {
		if err := c.c.Delete(imageCachePrefix + id); err != nil {
			log.Error("Couldn't delete captcha cache for %q: %v", id, err)
		}
	}

	return []byte(val)
}

// GetCaptchaService gets captcha service by service ID.
func GetCaptchaService(ctx *Context) string {
	switch setting.Service.CaptchaType {
	case setting.ImageCaptcha:
		return "image"
	case setting.ReCaptcha:
		return "recaptcha"
	case setting.HCaptcha:
		return "hcaptcha"
	case setting.MCaptcha:
		return "mcaptcha"
	case setting.CfTurnstile:
		return "cfturnstile"
	default:
		return "service"
	}
}

// GetImageCaptcha returns image captcha ID.
func GetImageCaptcha() string {
	imageCaptchaOnce.Do(func() {
		captcha.SetCustomStore(&imageCaptchaStore{c: cache.GetCache()})
	})
	return captcha.New()
}

// SetCaptchaData sets common captcha data
func SetCaptchaData(ctx *Context) {
	if !setting.Service.EnableCaptcha {
		return
	}
	ctx.Data["CaptchaServiceProvider"] = GetCaptchaService(ctx)
	ctx.Data["EnableCaptcha"] = setting.Service.EnableCaptcha
	ctx.Data["RecaptchaURL"] = setting.Service.RecaptchaURL
	ctx.Data["Captcha"] = GetImageCaptcha()
	ctx.Data["CaptchaType"] = setting.Service.CaptchaType
	ctx.Data["RecaptchaSitekey"] = setting.Service.RecaptchaSitekey
	ctx.Data["HcaptchaSitekey"] = setting.Service.HcaptchaSitekey
	ctx.Data["McaptchaSitekey"] = setting.Service.McaptchaSitekey
	ctx.Data["McaptchaURL"] = setting.Service.McaptchaURL
	ctx.Data["CfTurnstileSitekey"] = setting.Service.CfTurnstileSitekey
}

func VerifyDummy(ctx *Context, response string, captchaID string) bool {
	// accept dummy input in dev mode
	if !setting.IsProd || strings.Contains(strings.ToLower(strings.TrimSpace(captchaID)), ctx.Data["CaptchaServiceProvider"].(string))  {
		return true
	}
	return captcha.VerifyString(response, captchaID)
}

const (
	imgCaptchaIDField        = "img-captcha-id"
	imgCaptchaResponseField  = "img-captcha-response"
	gRecaptchaResponseField  = "g-recaptcha-response"
	hCaptchaResponseField    = "h-captcha-response"
	mCaptchaResponseField    = "m-captcha-response"
	cfTurnstileResponseField = "cf-turnstile-response"
)

// VerifyCaptcha verifies Captcha data
// No-op if captchas are not enabled
func VerifyCaptcha(ctx *Context, tpl base.TplName, form any) {
	if !setting.Service.EnableCaptcha {
		return
	}

	var valid bool
	var err error
	switch ctx.Data["CaptchaServiceProvider"] {
	case "image":
		valid = captcha.VerifyString(ctx.Req.Form.Get(imgCaptchaIDField), ctx.Req.Form.Get(imgCaptchaResponseField))
	case "recaptcha":
		valid, err = recaptcha.Verify(ctx, ctx.Req.Form.Get(gRecaptchaResponseField))
	case "hcaptcha":
		valid, err = hcaptcha.Verify(ctx, ctx.Req.Form.Get(hCaptchaResponseField))
	case "mcaptcha":
		valid, err = mcaptcha.Verify(ctx, ctx.Req.Form.Get(mCaptchaResponseField))
	case "cfturnstile":
		valid, err = turnstile.Verify(ctx, ctx.Req.Form.Get(cfTurnstileResponseField))
	default:
		valid = VerifyDummy(ctx, ctx.Req.Form.Get(imgCaptchaIDField), ctx.Req.Form.Get(imgCaptchaResponseField))
	}
	if err != nil {
		log.Debug("Captcha Verify failed: %v", err)
	}

	if !valid {
		ctx.Data["Err_Captcha"] = true
		ctx.RenderWithErr(ctx.Tr("form.captcha_incorrect"), tpl, form)
	}
}
