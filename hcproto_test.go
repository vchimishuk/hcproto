package hcproto

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

const golangOrgPage = `
<html>
    <head>
        <title>The Go Programming Language</title>
    </head>
    <body>
        <b>Some page content...</b>
    </body>
</html>
`

const atlassianComPage = `
<html>
    <head>
        <script src="..."></script>
        <title>Atlassian</title>
    </head>
    <body>
        <b>Some page content...</b>
    </body>
</html>
`

type fakeHTTPGetter map[string]string

func (g fakeHTTPGetter) Get(url string) (*http.Response, error) {
	body, ok := g[url]
	if !ok {
		return nil, errors.New("page not found")
	}

	return &http.Response{
		Status:     "200 OK",
		StatusCode: 200,
		Body:       ioutil.NopCloser(strings.NewReader(body)),
	}, nil
}

func TestParse(t *testing.T) {
	testCases := []struct {
		Msg     string
		MsgInfo *MsgInfo
	}{
		// 0
		{"", &MsgInfo{}},
		// 1
		{"Hello world!", &MsgInfo{}},
		// 2
		{"@username, hello", &MsgInfo{
			Mentions: []string{"username"}}},
		// 3
		{"@one,@two,some text,@three@four", &MsgInfo{
			Mentions: []string{"one", "two", "three", "four"}}},
		// 4
		{"Hi, (atlassian)!", &MsgInfo{
			Emotions: []string{"atlassian"}}},
		// 5
		{"(atlassian)(non-emotion),(crucible)(bitbucket)", &MsgInfo{
			Emotions: []string{
				"atlassian",
				"crucible",
				"bitbucket",
			}}},
		// 6
		{"http://golang.org", &MsgInfo{Links: []Link{{
			URL:   "http://golang.org",
			Title: "The Go Programming Language",
		}}}},
		// 7
		{"Golang homepage:http://golang.org", &MsgInfo{Links: []Link{{
			URL:   "http://golang.org",
			Title: "The Go Programming Language",
		}}}},
		// 8
		{"Golang homepage: http://golang.org\nAtlassian: https://atlassian.com",
			&MsgInfo{Links: []Link{
				{URL: "http://golang.org",
					Title: "The Go Programming Language"},
				{URL: "https://atlassian.com",
					Title: "Atlassian"},
			}}},
		// 9
		{"http://golang.org?foo=bar&baz=true#anchor foo", &MsgInfo{
			Links: []Link{{
				URL:   "http://golang.org?foo=bar&baz=true#anchor",
				Title: "The Go Programming Language",
			}}}},
		// 10
		{"Hi, @atlassian(atlassian)! Here is a link:http://golang.org (fry)",
			&MsgInfo{
				Mentions: []string{"atlassian"},
				Emotions: []string{"atlassian", "fry"},
				Links: []Link{{
					URL:   "http://golang.org",
					Title: "The Go Programming Language",
				}}}},
	}

	fhg := fakeHTTPGetter(map[string]string{
		"http://golang.org":                         golangOrgPage,
		"http://golang.org?foo=bar&baz=true#anchor": golangOrgPage,
		"https://atlassian.com":                     atlassianComPage,
	})
	parser := NewParser(fhg)

	for i, tc := range testCases {
		mi, err := parser.Parse(tc.Msg)
		if err != nil {
			t.Fatalf("Testcase %d parsing failed: %s", i, err)
		}
		miJson, err := parser.ParseJSON(tc.Msg)
		if err != nil {
			t.Fatalf("Testcase %d json parsing failed: %s", i, err)
		}

		var miJsonOrig MsgInfo
		err = json.Unmarshal([]byte(miJson), &miJsonOrig)
		if err != nil {
			t.Fatalf("Testcase %d JSON unmarshaling failed: %s", err)
		}

		if !msgInfoEqual(tc.MsgInfo, mi) {
			t.Fatalf("Testcase %d failed: expected %#v found %#v",
				i, tc.MsgInfo, mi)
		}
		if !msgInfoEqual(tc.MsgInfo, &miJsonOrig) {
			t.Fatalf("Testcase %d failed: expected %#v found %#v",
				i, tc.MsgInfo, miJsonOrig)
		}
	}
}

func msgInfoEqual(a, b *MsgInfo) bool {
	return reflect.DeepEqual(a.Mentions, b.Mentions) &&
		reflect.DeepEqual(a.Emotions, b.Emotions) &&
		linksEqual(a.Links, b.Links)
}

func linksEqual(a, b []Link) bool {
	if len(a) != len(b) {
		return false
	}
	for _, aa := range a {
		found := false
		for _, bb := range b {
			if aa == bb {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}
