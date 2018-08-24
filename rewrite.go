package rewrite

import (
	"fmt"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
)

const headerField = "X-Rewrite-Original-URI"

type Rule struct {
	Pattern  string
	To       string
	Redirect bool
	*regexp.Regexp
}

var regfmt = regexp.MustCompile(`:[^/#?()\.\\]+`)

func NewRule(pattern, to string, redirect bool) (*Rule, error) {
	pattern = regfmt.ReplaceAllStringFunc(pattern, func(m string) string {
		return fmt.Sprintf(`(?P<%s>[^/#?]+)`, m[1:])
	})

	reg, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	return &Rule{
		pattern,
		to,
		redirect,
		reg,
	}, nil
}

func (r *Rule) Rewrite(req *http.Request) bool {
	oriPath := req.URL.Path

	if !r.MatchString(oriPath) {
		return false
	}

	to := path.Clean(r.Replace(req.URL))

	u, e := url.Parse(to)
	if e != nil {
		return false
	}

	req.Header.Set(headerField, req.URL.RequestURI())

	req.URL.Path = u.Path
	req.URL.RawPath = u.RawPath
	if u.RawQuery != "" {
		req.URL.RawQuery = u.RawQuery
	}

	return true
}

func (r *Rule) Replace(u *url.URL) string {
	if !r.Hit("\\$|\\:", r.To) {
		return r.To
	}

	uri := u.RequestURI()

	regFrom := regexp.MustCompile(r.Pattern)
	match := regFrom.FindStringSubmatchIndex(uri)

	result := regFrom.ExpandString([]byte(""), r.To, uri, match)

	str := string(result[:])

	if r.Hit("\\:", str) {
		return r.replaceNamedParams(uri, str)
	}

	return str
}

var urlreg = regexp.MustCompile(`:[^/#?()\.\\]+|\(\?P<[a-zA-Z0-9]+>.*\)`)

func (r *Rule) replaceNamedParams(from, to string) string {
	fromMatches := r.FindStringSubmatch(from)

	if len(fromMatches) > 0 {
		for i, name := range r.SubexpNames() {
			if len(name) > 0 {
				to = strings.Replace(to, ":"+name, fromMatches[i], -1)
			}
		}
	}

	return to
}

func (r *Rule) Hit(pattern, str string) bool {
	ok, e := regexp.MatchString(pattern, str)
	if e != nil {
		return false
	}

	return ok
}
func HeaderRewrite(rules []*Rule, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, rw := range rules {
			//old := r.URL.String()
			if rw.Rewrite(r) {
				//fmt.Printf("rule:%s %+v %+v\n", rw.Pattern, old, r.URL.String())
				if rw.Redirect {
					http.Redirect(w, r, r.URL.Path, 302)
					return
				}
				break
			}
		}
		h.ServeHTTP(w, r)
	})
}
