package feedback

import "net/url"

// SourceURL is for source attribution, never a request address. Do not echo
// authentication, query tokens or fragments into application-authored details.
func SourceURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "Source URL unavailable"
	}
	u.User = nil
	u.RawQuery = ""
	u.ForceQuery = false
	u.Fragment = ""
	u.RawFragment = ""
	return u.String()
}
