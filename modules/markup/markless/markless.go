// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markless

import (
	"bufio"
	"html/template"
	"io"
	"regexp"
	"shirakumo.org/markless"
	"shirakumo.org/markless/components"
	"shirakumo.org/markless/html"
	"shirakumo.org/markless/parser"
	"strings"
	
	"forgejo.org/modules/highlight"
	"forgejo.org/modules/log"
	"forgejo.org/modules/markup"
	"forgejo.org/modules/setting"
	
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

func init() {
	markup.RegisterRenderer(Renderer{})
}

// Renderer implements markup.Renderer for Markless files
type Renderer struct{}

var _ markup.PostProcessRenderer = (*Renderer)(nil)

// Name implements markup.Renderer
func (Renderer) Name() string {
	return "markless"
}

// NeedPostProcess implements markup.PostProcessRenderer
func (Renderer) NeedPostProcess() bool { return true }

// Extensions implements markup.Renderer
func (Renderer) Extensions() []string {
	return []string{".mess", ".markless"}
}

// SanitizerRules implements markup.Renderer
func (Renderer) SanitizerRules() []setting.MarkupSanitizerRule {
	return []setting.MarkupSanitizerRule{
		{Element: "div", AllowAttr: "style", Regexp: regexp.MustCompile(`^text-align:(left|right|center|justify);$`)},
		{Element: "span", AllowAttr: "class", Regexp: regexp.MustCompile(`^spoiler$`)},
		{Element: "blockquote", AllowAttr: "class", Regexp: regexp.MustCompile(`^inline$`)},
	}
}

func convertComponent(in components.Component) html.Node {
	switch c := in.(type) {
	case *components.CodeBlock:
		lang := c.Language
		lexer := lexers.Get(lang)
		if lexer == nil && c.Language == "" {
			lexer = lexers.Analyse(c.Content)
			if lexer == nil {
				lexer = lexers.Fallback
			}
		}
		if lexer != nil {
			lang = strings.ToLower(lexer.Config().Name)
			lexer = chroma.Coalesce(lexer)
			el := html.NewElement("code").AddClass("chroma").AddClass("language-"+lang)
			el.AppendChild(html.RawHTML(string(highlight.CodeFromLexer(lexer, c.Content))))
			return html.NewElement("pre").AppendChild(el)
		}
	}
	return html.DefaultConvert(in, convertComponent)
}

// Render implements markup.Renderer
func (Renderer) Render(ctx *markup.RenderContext, input io.Reader, output io.Writer) error {
	defer func() {
		if r := recover(); r != nil {
			log.Warn("Unable to render Markless due to panic - will return raw bytes")
			if log.IsDebug() {
				log.Debug("Panic in Markless: %v\n%s", r, log.Stack(2))
			}
			if _, r = io.Copy(output, input); r != nil {
				log.Error("io.Copy failed: %v", r)
			}
		}
	}()
	
	tmpBlock := bufio.NewWriter(output)
	maxSize := setting.UI.Markless.MaxFileSize
	safeMode := setting.UI.Markless.SafeMode

	if maxSize != 0 {
		input = io.LimitReader(input, maxSize+1)
	}

	var p *parser.Parser
	if safeMode {
		p = parser.NewSafe()
	} else {
		p = parser.NewDefault()
	}

	p, err := markless.Parse(input, p)
	if err != nil {
		return err
	}
	html.DefaultConvert(p.Root(), convertComponent).Write(tmpBlock)
	return tmpBlock.Flush()
}

// Render renders Markdown to HTML with all specific handling stuff.
func Render(ctx *markup.RenderContext, input io.Reader, output io.Writer) error {
	if ctx.Type == "" {
		ctx.Type = "markless"
	}
	return markup.Render(ctx, input, output)
}

// RenderString renders Markless string to HTML and return as a string
func RenderString(ctx *markup.RenderContext, content string) (template.HTML, error) {
	return RenderReader(ctx, strings.NewReader(content))
}

// RenderReader renders Markless io.Reader to HTML and return as a string
func RenderReader(ctx *markup.RenderContext, input io.Reader) (template.HTML, error) {
	var buf strings.Builder
	if err := Render(ctx, input, &buf); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil
}
