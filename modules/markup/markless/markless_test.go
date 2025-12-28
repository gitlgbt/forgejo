// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markless_test

import (
	"context"
	"os"
	"testing"

	"forgejo.org/models/unittest"
	"forgejo.org/modules/git"
	"forgejo.org/modules/log"
	"forgejo.org/modules/markup"
	"forgejo.org/modules/markup/markless"

	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	unittest.InitSettings()
	if err := git.InitSimple(context.Background()); err != nil {
		log.Fatal("git init failed, err: %v", err)
	}
	markup.Init(&markup.ProcessorHelper{
		IsUsernameMentionable: func(ctx context.Context, username string) bool {
			return username == "r-lyeh"
		},
	})
	os.Exit(m.Run())
}

func TestRender(t *testing.T) {
	test := func(input, expected string) {
		result, err := markless.RenderString(&markup.RenderContext{
			Ctx: git.DefaultContext,
		}, input)
		require.NoError(t, err)
		if expected != string(result) {
			t.Errorf("\nParsed:\n%v\nExpected: %v\nGot:      %v",
				input, expected, result)
		}
	}

	// alignment
	test("|<a", "<article><div style=\"text-align:left;\"><p>a</p></div></article>")
	test("|>a", "<article><div style=\"text-align:right;\"><p>a</p></div></article>")
	test("><a", "<article><div style=\"text-align:center;\"><p>a</p></div></article>")
	test("||a", "<article><div style=\"text-align:justify;\"><p>a</p></div></article>")
	// blockquote
	test("| a", "<article><blockquote><p>a</p></blockquote></article>")
	test("| a\n~ b", "<article><blockquote><p>a</p><cite>b</cite></blockquote></article>")
	test("~ a | b", "<article><blockquote class=\"inline\"><p>b</p><cite>a </cite></blockquote></article>")
	// bold
	test("**a**", "<article><p><strong>a</strong></p></article>")
	// code-block
	test("::\na\n::", "<article><pre class=\"code-block\"><code>a</code></pre></article>")
	test(":: a\na\n::", "<article><pre class=\"code-block\"><code class=\"language-a\">a</code></pre></article>")
	// code
	test("``a``", "<article><p><code>a</code></p></article>")
	// compound
	test("''a''(https://example.com)", "<article><p><a href=\"https://example.com\" rel=\"nofollow\">a</a></p></article>")
	test("''a''(spoiler)", "<article><p><span class=\"spoiler\" style=\"color: black\">a</span></p></article>")
	// embed
	test("[ image a ]", "<article><figure><img alt=\"a\" src=\"/a\"/></figure></article>")
	test("[ audio a ]", "<article><figure><p>HTML5 audio is not supported. The audio source is here:<a href=\"a\" target=\"_blank\" rel=\"nofollow noopener\">a</a></p></figure></article>")
	test("[ video a ]", "<article><figure><video controls=\"\" src=\"a\"><p>HTML5 video is not supported. The video source is here:<a href=\"a\" target=\"_blank\" rel=\"nofollow noopener\">a</a></p></video></figure></article>")
	// entities
	test("--", "<article><p>–</p></article>")
	test("---", "<article><p>—</p></article>")
	test("-/-", "<article><p><br/></p></article>")
	// footnote-reference
	test("[1]", "<article><p><sup><a href=\"#user-content-1\" rel=\"nofollow\">[1]</a></sup></p></article>")
	// footnote
	test("[1] a", "<article><ol><li id=\"user-content-1\" value=\"1\">a</li></ol></article>")
	// header
	test("# Hello", "<article><h1 id=\"user-content-hello\">Hello</h1></article>")
	// horizontal-rule
	test("==", "<article><hr/></article>")
	// italic
	test("//a//", "<article><p><em>a</em></p></article>")
	// ordered-list
	test("1.a", "<article><ol><li value=\"1\"><p>a</p></li></ol></article>")
	// paragraph
	test("a\nb", "<article><p>a<br/>b</p></article>")
	// strikethrough
	test("<-a->", "<article><p><s>a</s></p></article>")
	// subtext
	test("v(a)", "<article><p><sub>a</sub></p></article>")
	// supertext
	test("^(a)", "<article><p><sup>a</sup></p></article>")
	// underline
	test("__a__", "<article><p><u>a</u></p></article>")
	// unordered-list
	test("- a", "<article><ul><li><p>a</p></li></ul></article>")
	test("- a\n  b", "<article><ul><li><p>a<br/>b</p></li></ul></article>")
	// url
	test("https://example.com", "<article><p><a href=\"https://example.com\" rel=\"nofollow\">https://example.com</a></p></article>")
}
