package CaddyNetlifyRedirects

import (
	"net/url"

	"github.com/tj/go-redirects"
	"github.com/ucarion/urlpath"
	"go.uber.org/zap"
)

type Middleware struct {
	Logger    *zap.Logger
	Redirects []redirects.Rule
}

type MatchContext struct {
	Scheme      string
	OriginalUrl *url.URL
}

type MatchResult struct {
	Match      *urlpath.Match
	ResolvedTo *url.URL
	Source     redirects.Rule

	IsNoRedirect   bool
	IsMatched      bool
	IsHostRedirect bool

	Error error
}
