package feed

import (
	"context"
	"errors"
	"io"
	"net/http"
	"podmutt/internal/feedback"
	"podmutt/internal/store"
)

func (r *Refresher) fetch(ctx context.Context, source store.Feed) (Fetched, error) {
	return r.fetchWith(ctx, source, Parse)
}
func (r *Refresher) fetchWith(ctx context.Context, source store.Feed, decode func([]byte, string) (Document, error)) (result Fetched, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source.URL, nil)
	if err != nil {
		return result, &feedback.Fault{Code: feedback.InvalidURL, Safe: "Use an HTTP(S) feed or website URL without credentials.", Cause: err}
	}
	req.Header.Set("User-Agent", "podmutt/dev")
	if source.ETag != "" {
		req.Header.Set("If-None-Match", source.ETag)
	}
	if source.LastModified != "" {
		req.Header.Set("If-Modified-Since", source.LastModified)
	}
	response, err := r.client.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return result, context.Canceled
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return result, context.DeadlineExceeded
		}
		return result, &feedback.Fault{Code: feedback.Network, Safe: "Could not reach this feed. Check your connection or try another public feed.", Cause: err}
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case http.StatusNotModified:
		if source.ETag == "" && source.LastModified == "" {
			return result, feedback.New(feedback.InvalidResponse, "unexpected 304 without cached validators")
		}
		result.NotModified = true
		result.ETag = first(response.Header.Get("ETag"), source.ETag)
		result.LastModified = first(response.Header.Get("Last-Modified"), source.LastModified)
	case http.StatusOK:
		if response.ContentLength > MaxBytes {
			return result, feedback.New(feedback.InvalidResponse, "feed response too large")
		}
		data, err := io.ReadAll(io.LimitReader(response.Body, MaxBytes+1))
		if err != nil {
			if ctx.Err() != nil {
				return result, ctx.Err()
			}
			return result, &feedback.Fault{Code: feedback.InvalidResponse, Safe: "The feed response could not be read.", Cause: err}
		}
		finalURL := req.URL.String()
		if response.Request != nil && response.Request.URL != nil {
			finalURL = response.Request.URL.String()
		}
		result.Document, err = decode(data, finalURL)
		if err != nil {
			return result, &feedback.Fault{Code: feedback.InvalidResponse, Safe: "This URL did not return a readable feed or a website with usable RSS/Atom links.", Cause: err}
		}
		result.ETag, result.LastModified = response.Header.Get("ETag"), response.Header.Get("Last-Modified")
	default:
		return result, feedback.StatusError(response.StatusCode)
	}
	return result, nil
}
