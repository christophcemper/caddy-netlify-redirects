package CaddyNetlifyRedirects

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/tj/go-redirects"
	"github.com/ucarion/urlpath"
)

func ParseUrlWithContext(urlStr string, ctx *MatchContext) (*url.URL, error) {
	original := urlStr

	if !strings.Contains(urlStr, "http://") && !strings.Contains(urlStr, "https://") {
		urlStr = fmt.Sprintf("%s%s", ctx.Scheme, urlStr)

		// use url parse to test if it has a Host, if it doesn't revert back to original
		testStr, err := url.Parse(urlStr)

		if err != nil {
			return nil, err
		}

		if testStr.Host == "" {
			urlStr = original
		}
	}

	return url.Parse(urlStr)
}

func MatchUrlToRule(rule redirects.Rule, reqUrl *url.URL, ctx *MatchContext) MatchResult {
	if reqUrl.Host == "" || reqUrl.Scheme == "" {
		return MatchResult{
			ResolvedTo:     nil,
			IsMatched:      false,
			IsHostRedirect: false,
			Error:          errors.New("request url must have both host and scheme"),
		}
	}

	/*
	 * Perform the match as soon as possible on the path itself, as we may need the resolved To
	 */

	from, errFrom := ParseUrlWithContext(rule.From, ctx)

	if errFrom != nil {
		return MatchResult{
			ResolvedTo:     nil,
			IsMatched:      false,
			IsHostRedirect: false,
			Error:          errFrom,
		}
	}

	// Check if our URL matches any path we have in the rules
	path := urlpath.New(strings.Trim(from.Path, "/"))
	matched, ok := path.Match(strings.Trim(reqUrl.Path, "/"))

	if !ok {
		// sugar.Errorw("not OK", "from.path", from.Path, "reqUrl.Path", reqUrl.Path)
		return MatchResult{
			ResolvedTo:     nil,
			IsMatched:      false,
			IsHostRedirect: false,
		}
	}

	toPath := rule.To
	toPath = replaceParams(toPath, matched)
	toPath = replaceSplat(toPath, matched)

	to, errTo := ParseUrlWithContext(toPath, ctx)

	if errTo != nil {
		return MatchResult{
			ResolvedTo:     nil,
			IsMatched:      false,
			IsHostRedirect: false,
			Error:          errTo,
		}
	}

	skipMatch := MatchResult{
		ResolvedTo:     to,
		Match:          &matched,
		IsMatched:      false,
		IsHostRedirect: false,
		IsNoRedirect:   true,
	}

	// if the to.path = reqURL.path we can skip the rest of the checks and NOT return a match for a redirect! otherwise redirect loop

	if to.Path == reqUrl.Path && (to.Host == reqUrl.Host || to.Host == "") {
		skipMatch.IsNoRedirect = true
		return skipMatch
	}

	/*
	 * If this rule has a query string element to it, we need to perform an identical match ONLY after the initial match of the path
	 */

	if from.RawQuery != "" && from.RawQuery == reqUrl.RawQuery {
		return MatchResult{
			ResolvedTo:     to,
			Match:          &matched,
			IsMatched:      true,
			IsHostRedirect: false,
			Source:         rule,
		}
	} else if from.RawQuery != "" {
		return skipMatch
	}

	hostToHost := from.Host != "" && to.Host != ""
	hostToRelative := from.Host != "" && to.Host == ""
	relativeToHost := from.Host == "" && to.Host != ""

	// dont need to redirect if on the same host, or no host on rule.To
	isHostRedirect := to.Host != "" && to.Host != reqUrl.Host

	if (hostToHost || hostToRelative) && from.Host != reqUrl.Host {
		return skipMatch
	}

	if relativeToHost && to.Host == reqUrl.Host {
		return skipMatch
	}

	specialToRules := strings.Split(rule.To, "|")

	for _, sItem := range specialToRules {
		if sItem == "$ENFORCE_TRAILING_SLASH" {
			// check to make sure this isn't a file request
			parts := strings.Split(ctx.OriginalUrl.Path, ".")
			if
			// make sure parts is greater than two, and then verify that the final element is one of these
			len(parts) >= 2 &&
				len(parts[len(parts)-1]) >= 2 &&
				len(parts[len(parts)-1]) <= 5 {
				return skipMatch
			}

			if !strings.HasSuffix(ctx.OriginalUrl.Path, "/") {
				// redirect
				prefixedTo := reqUrl
				prefixedTo.Path = fmt.Sprintf("%s/", prefixedTo.Path)

				return MatchResult{
					ResolvedTo:     prefixedTo,
					Match:          &matched,
					IsMatched:      true,
					IsHostRedirect: isHostRedirect,
				}
			}

			return skipMatch
		}
	}

	return MatchResult{
		ResolvedTo:     to,
		Match:          &matched,
		IsMatched:      true,
		IsHostRedirect: isHostRedirect,
		Source:         rule,
	}
}

func replaceParams(to string, matched urlpath.Match) string {
	if len(matched.Params) > 0 {
		for key, value := range matched.Params {
			to = strings.ReplaceAll(to, ":"+key, value)
		}
	}

	return to
}

func replaceSplat(to string, matched urlpath.Match) string {
	return strings.ReplaceAll(to, ":splat", matched.Trailing)
}
