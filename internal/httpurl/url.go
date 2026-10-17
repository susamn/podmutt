// Package httpurl validates public-feed URLs without disclosing their contents
// in errors. Local HTTP addresses remain allowed for self-hosted feeds/tests.
package httpurl

import (
	"fmt"
	"net/url"
)

func Parse(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil {
		return nil, fmt.Errorf("expected HTTP(S) URL without credentials")
	}
	return u, nil
}

func Resolve(base *url.URL, reference string) (string, error) {
	if reference == "" {
		return "", fmt.Errorf("empty URL reference")
	}
	ref, err := url.Parse(reference)
	if err != nil {
		return "", fmt.Errorf("invalid URL reference")
	}
	u := base.ResolveReference(ref)
	if _, err := Parse(u.String()); err != nil {
		return "", err
	}
	return u.String(), nil
}
