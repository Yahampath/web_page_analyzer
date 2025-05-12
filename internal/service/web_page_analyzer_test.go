// internal/service/analyzer_functions_test.go
package service

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"golang.org/x/net/html"
	"web_page_analyzer/internal/pkg/errors"
)

// mockWebClient implements adaptors.WebClient.
type mockWebClient struct {
	doFunc func(ctx context.Context, url, method string) ([]byte, int, error)
}

func (m *mockWebClient) Do(ctx context.Context, url, method string) ([]byte, int, error) {
	return m.doFunc(ctx, url, method)
}

func TestGetWebPage(t *testing.T) {
	logger := logrus.New()
	an := NewAnalyzer(logger, &mockWebClient{})

	ctx := context.Background()
	const testURL = "http://example.com"

	cases := []struct {
		name      string
		doFunc    func(context.Context, string, string) ([]byte, int, error)
		wantCode  int
		wantBody  []byte
		shouldErr bool
	}{
		{
			name: "success",
			doFunc: func(_ context.Context, url, method string) ([]byte, int, error) {
				if url != testURL || method != http.MethodGet {
					t.Fatalf("unexpected call: %s %s", method, url)
				}
				return []byte("hello"), http.StatusOK, nil
			},
			wantCode:  http.StatusOK,
			wantBody:  []byte("hello"),
			shouldErr: false,
		},
		{
			name: "network error",
			doFunc: func(_ context.Context, _, _ string) ([]byte, int, error) {
				return nil, 0, errors.New("fail")
			},
			wantCode:  400,
			shouldErr: true,
		},
		{
			name: "non-200 status",
			doFunc: func(_ context.Context, _, _ string) ([]byte, int, error) {
				return nil, http.StatusNotFound, nil
			},
			wantCode:  http.StatusNotFound,
			shouldErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			an.webClient = &mockWebClient{doFunc: tc.doFunc}
			body, code, err := an.getWebPage(ctx, testURL)

			if tc.shouldErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
			if code != tc.wantCode {
				t.Errorf("code = %d; want %d", code, tc.wantCode)
			}
			if !bytes.Equal(body, tc.wantBody) {
				t.Errorf("body = %q; want %q", body, tc.wantBody)
			}
		})
	}
}

func TestGetHTMLVersion(t *testing.T) {
	a := &Analyzer{}
	ctx := context.Background()
	cases := []struct {
		name string
		body []byte
		want string
	}{
		{"HTML5 simple", []byte("<!DOCTYPE html><html></html>"), "HTML5"},
		{"HTML4 strict", []byte("<!DOCTYPE HTML 4.01 STRICT><html></html>"), "HTML 4.01 Strict"},
		{"XHTML transitional", []byte("<!DOCTYPE XHTML 1.0 Transitional><html></html>"), "XHTML 1.0 Transitional"},
		{"unknown", []byte("<!DOCTYPE FOO BAR><html></html>"), "<!DOCTYPE FOO BAR>"},
		{"none", []byte("<html></html>"), ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := a.getHTMLVersion(ctx, tc.body)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q; want %q", got, tc.want)
			}
		})
	}
}

func TestGetTitle(t *testing.T) {
	a := &Analyzer{}
	doc, err := html.Parse(strings.NewReader(`<html><head><title>MyPage</title></head><body></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	got, err := a.getTitle(context.Background(), doc)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got != "MyPage" {
		t.Errorf("got %q; want MyPage", got)
	}
}

func TestCountHeadings(t *testing.T) {
	a := &Analyzer{}
	htmlStr := `<html><body>
	<h1>One</h1><h2>Two</h2><h2>Another</h2><h3>Three</h3>
	</body></html>`
	doc, _ := html.Parse(strings.NewReader(htmlStr))
	got, err := a.countHeadings(context.Background(), doc)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	want := map[string]int{"h1": 1, "h2": 2, "h3": 1, "h4": 0, "h5": 0, "h6": 0}
	for tag, w := range want {
		if got[tag] != w {
			t.Errorf("%s = %d; want %d", tag, got[tag], w)
		}
	}
}

func TestGetHref(t *testing.T) {
	a := &Analyzer{}
	node := &html.Node{
		Type: html.ElementNode,
		Data: "a",
		Attr: []html.Attribute{{Key: "href", Val: "/path"}},
	}
	if got := a.getHref(context.Background(), node); got != "/path" {
		t.Errorf("got %q; want /path", got)
	}
	// no href
	node.Attr = nil
	if got := a.getHref(context.Background(), node); got != "" {
		t.Errorf("got %q; want empty", got)
	}
}

func TestGetCanonicalHost(t *testing.T) {
	a := &Analyzer{}
	cases := []struct {
		raw  string
		want string
	}{
		{"http://example.com", "example.com"},
		{"http://example.com:80", "example.com"},
		{"https://foo:443", "foo"},
		{"http://example.com:8080", "example.com:8080"},
	}
	for _, tc := range cases {
		u, _ := url.Parse(tc.raw)
		if got := a.getCanonicalHost(context.Background(), u); got != tc.want {
			t.Errorf("%s -> %q; want %q", tc.raw, got, tc.want)
		}
	}
}

func TestCollectAndCountLinks(t *testing.T) {
	a := &Analyzer{}
	htmlStr := `<html><body>
	<a href="/intra"></a>
	<a href="http://outside"></a>
	</body></html>`
	doc, _ := html.Parse(strings.NewReader(htmlStr))
	base, _ := url.Parse("http://example.com")

	links, err := a.collectLinks(context.Background(), doc, base)
	if err != nil {
		t.Fatalf("collectLinks error: %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("len = %d; want 2", len(links))
	}
	if links[0].url != "http://example.com/intra" || !links[0].isInternal {
		t.Errorf("first link = %+v; want internal", links[0])
	}
	if links[1].url != "http://outside" || links[1].isInternal {
		t.Errorf("second link = %+v; want external", links[1])
	}

	// countLinks
	inside, outside, err := a.countLinks(context.Background(), doc, base)
	if err != nil {
		t.Fatalf("countLinks error: %v", err)
	}
	if inside != 1 || outside != 1 {
		t.Errorf("countLinks = (%d,%d); want (1,1)", inside, outside)
	}
}

func TestCheckLinksAccessibility(t *testing.T) {
	logger := logrus.New()
	// HEAD to good returns 200, to bad returns 500
	mock := &mockWebClient{
		doFunc: func(_ context.Context, urlStr, method string) ([]byte, int, error) {
			if method != http.MethodHead {
				t.Fatalf("expected HEAD; got %s", method)
			}
			if strings.Contains(urlStr, "good") {
				return nil, 200, nil
			}
			return nil, 500, nil
		},
	}
	a := NewAnalyzer(logger, mock)

	links := []linkInfo{
		{url: "http://good", isInternal: true},
		{url: "http://bad", isInternal: false},
	}
	got, err := a.checkLinksAccessibility(context.Background(), links)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got != 1 {
		t.Errorf("inaccessible = %d; want 1", got)
	}
}

func TestHasLoginFormAndPasswordField(t *testing.T) {
	a := &Analyzer{}

	// with password
	doc1, _ := html.Parse(strings.NewReader(`
		<html><body>
		  <form><input type="password"/></form>
		</body></html>`))
	has, err := a.hasLoginForm(context.Background(), doc1)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !has {
		t.Error("expected hasLoginForm=true")
	}

	// without password
	doc2, _ := html.Parse(strings.NewReader(`
		<html><body>
		  <form><input type="text"/></form>
		</body></html>`))
	has, err = a.hasLoginForm(context.Background(), doc2)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if has {
		t.Error("expected hasLoginForm=false")
	}
}