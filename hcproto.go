package hcproto

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/html"
)

// Supported emotions list.
// Notice: keep this list sorted, binary search is performed on the list.
var supportedEmotions = []string{
	"atlassian",
	"bitbucket",
	"boom",
	"crucible",
	"fry",
	"ghost",
	"heart",
}

// HTTPGetter is a HTTP page downloader with HTTP GET method.
// http.Client satisfies this interface and can be used as
// a default implementation.
type HTTPGetter interface {
	Get(url string) (*http.Response, error)
}

// Link description.
type Link struct {
	// HTTP or HTTPS link URL itself.
	URL string `json:"url"`
	// Title of the HTML page pointed by the above link.
	Title string `json:"title"`
}

// MsgInfo contains information about elements of the message.
type MsgInfo struct {
	// Mentions is a list of mentioned usernames.
	Mentions []string `json:"mentions,omitempty"`
	// Emotions is a list of mentioned emotions.
	Emotions []string `json:"emotions,omitempty"`
	// Links is a list of mentioned links.
	Links []Link `json:"links,omitempty"`
}

// Parser is a HipChat messages parser implementation.
type Parser struct {
	hg HTTPGetter
}

// NewParser returns newly created Parser which uses given HTTPGetter
// for HTML pages quering during links parsing. It has no state so can
// be used in concurrent environment. http.Client can be used as HTTPGetter.
func NewParser(hg HTTPGetter) *Parser {
	return &Parser{hg: hg}
}

// Parse parses a message and returns information about elements
// (emotions, mentions, links) which it contains.
func (p *Parser) Parse(msg string) (*MsgInfo, error) {
	var mentions []string
	var emotions []string
	var urls []string
	var i int

	for i < len(msg) {
		s := msg[i:]
		r, size := utf8.DecodeRuneInString(s)
		if r == utf8.RuneError {
			return nil, fmt.Errorf("invalid rune at position %d", i)
		}

		switch r {
		case '(':
			e, n := emotion(s)
			if e != "" {
				size = n
				emotions = append(emotions, e)
			}
		case '@':
			m, n := mention(s)
			if m != "" {
				size = n
				mentions = append(mentions, m)
			}
		case 'H', 'h':
			// Check that link doesn't start at the middle
			// of the word.
			start := true
			if i > 0 {
				r, _ := utf8.DecodeLastRune([]byte(msg[:i]))
				start = !unicode.IsLetter(r) && !unicode.IsDigit(r)
			}
			if start {
				url, n := link(s)
				if url != "" {
					urls = append(urls, url)
					size = n
				}
			}
		}

		i += size
	}

	return &MsgInfo{
		Mentions: mentions,
		Emotions: emotions,
		Links:    p.urlsToLinks(urls),
	}, nil
}

func (p *Parser) ParseJSON(msg string) (string, error) {
	mi, err := p.Parse(msg)
	if err != nil {
		return "", err
	}
	s, err := json.Marshal(mi)
	if err != nil {
		return "", err
	}

	return string(s), nil
}

// TODO: urlToLinks returns Links in random order: page downloaded first
//       generates first Link. It is better to keep the original order.
func (p *Parser) urlsToLinks(urls []string) []Link {
	switch len(urls) {
	case 0:
		return nil
	case 1:
		// Ignore title errors. Log it in realworld app.
		t, _ := p.pagetTitle(urls[0])
		return []Link{{URL: urls[0], Title: t}}
	default:
		ch := make(chan Link, len(urls))
		for _, url := range urls {
			go func(u string) {
				// Ignore title errors. Log it in realworld app.
				t, _ := p.pagetTitle(u)
				ch <- Link{URL: u, Title: t}
			}(url)
		}

		links := make([]Link, 0, len(urls))
		for i := 0; i < len(urls); i++ {
			links = append(links, <-ch)
		}

		return links
	}
}

func (p *Parser) pagetTitle(url string) (string, error) {
	resp, err := p.hg.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	t := html.NewTokenizer(resp.Body)
	head := false
	for {
		tt := t.Next()

		switch tt {
		case html.ErrorToken:
			return "", errors.New("invalid html")
		case html.StartTagToken:
			name, _ := t.TagName()

			switch string(name) {
			case "head":
				head = true
			case "title":
				if true || head {
					if t.Next() == html.TextToken {
						return string(t.Text()), nil
					}
				}
			case "body":
				// Page has no TITLE tag.
				return "", nil
			}
		case html.EndTagToken:
			name, _ := t.TagName()

			if string(name) == "head" {
				return "", nil
			}
		}
	}

	return "", errors.New("invalid html")
}

func emotion(s string) (string, int) {
	i := strings.Index(s, ")")
	if i == -1 {
		return "", 0
	}
	e := s[1:i]

	i = sort.SearchStrings(supportedEmotions, e)
	if !(i < len(supportedEmotions) && supportedEmotions[i] == e) {
		return "", 0
	}

	return e, i + 1
}

func mention(s string) (string, int) {
	i := 1 // Skip '@'

	for i < len(s) {
		r, n := utf8.DecodeRuneInString(s[i:])
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			i += n
		} else {
			break
		}
	}
	if i == 1 {
		return "", 0
	}

	return s[1:i], i
}

// Correct URL parsing is to complex task so I've made this naive
// solution for now. Any number of chars starting with HTTP/HTTPS
// up to space character considered as URL.
func link(s string) (string, int) {
	// At least 'http://' must be present.
	if len(s) < 7 {
		return "", 0
	}

	proto := strings.ToLower(s[:4])
	if s[4] == 's' || s[4] == 'S' {
		proto += "s"
	}
	if proto != "http" && proto != "https" {
		return "", 0
	}
	pl := len(proto)
	if s[pl:pl+3] != "://" {
		return "", 0
	}

	end := spaceIndex(s)
	if end == -1 {
		end = len(s)
	}

	link := s[:end]
	// Validate link with standard parser.
	_, err := url.Parse(link)
	if err != nil {
		return "", 0
	}

	return link, len(link)
}

func spaceIndex(s string) int {
	i := 0
	for i < len(s) {
		r, n := utf8.DecodeRuneInString(s[i:])
		if unicode.IsSpace(r) {
			return i
		}
		i += n
	}

	return -1
}
