package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

type client struct {
	// user/password for basic authentication
	user, password string

	// the http client
	c *http.Client

	// user-agent to send to remote server
	userAgent string

	// if not empty, append data to the specified file name
	// if empty, write data to stdout
	outputFile string

	// offset at which to start producing the remote resource
	byteOffset int64

	// if false 'byteOffset' refers to the offset in the remote content from its end
	// if true 'byteOffset' refers to the offset in the remote content from its beginning
	byteOffsetFromStart bool

	// duration to sleep between rescanning for new content or
	// <= 0 to _not_ repeatedly scan
	tick int

	dumpHeaders bool // true to dump request/response headers to stderr
}

func (c *client) isFollowMode() bool {
	return c.tick > 0
}

type fetchState struct {
	resource     string    // the remote file, e.g. "https://had-url.dev:8888/mylogs/app.log"
	offset       int64     // the current offset in the remote file
	lastModified time.Time // the lastModified timestamp of remote file
	expires      time.Time // the timestamp when the remote file's caching expires
}

func (c *client) tail(resource string) error {

	var state fetchState
	if err := c.initFetch(&state, resource); err != nil {
		return err
	}

	var out io.Writer
	if c.outputFile == "" {
		out = os.Stdout
	} else {
		f, err := os.Create(c.outputFile)
		if err != nil {
			return err
		}
		defer f.Close()
		out = f
	}

	// ~ if we are not in follow mode we just get the first package of
	// data, dump it, and we're done.
	if !c.isFollowMode() {
		return c.fetch(&state, out)
	}

	// ~ here we try to "follow" the remote file by repeatedly making
	// requests against it.
	tick := time.NewTicker(time.Duration(c.tick) * time.Second)
	for {
		if err := c.fetch(&state, out); err != nil {
			tick.Stop()
			return err
		}
		<-tick.C
	}
	return nil
}

// Tries to determine the offset at which to begin fetching.  This
// method assumes to be invoked as the first step in tailing the
// resource in question.
func (c *client) initFetch(s *fetchState, resource string) error {
	s.resource = resource
	s.offset = 0
	s.lastModified = time.Time{}
	s.expires = time.Time{}

	// ~ no need to make the additional head request if the user
	// gave us the exactly the offset as of which we are supposed
	// to tail :)
	if c.byteOffsetFromStart {
		s.offset = c.byteOffset
		return nil
	}

	resp, err := c.doRequest("HEAD", s)
	if err != nil {
		return err
	}
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		size, err := strconv.ParseInt(cl, 10, 64)
		if err == nil {
			s.offset = max(size-c.byteOffset, 0)
		}
	}
	return nil
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// Writes the remote resources as of the given offset to out. Returns
// number of bytes written or error.
func (c *client) fetch(s *fetchState, out io.Writer) error {
	// If the resource expires only in future, we're done for now
	if !s.expires.IsZero() && time.Now().Before(s.expires) {
		return nil
	}

	// Make the request
	resp, err := c.doRequest("GET", s)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Success
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		// ~ remember the last-modified for the next request we'll be doing
		if lm := resp.Header.Get("Last-Modified"); lm != "" {
			if t, err := parseHttpTime(lm); err == nil {
				s.lastModified = t
			}
		}
		// ~ remember the "expires" timestamp such that we can defer
		// making the next request
		if et := resp.Header.Get("Expires"); et != "" {
			if t, err := parseHttpTime(et); err == nil {
				s.expires = t
			}
		}
		// ~ process the received content
		n, err := io.Copy(out, resp.Body)
		s.offset += n // remember how much we copied to out
		return err
	}

	// Not Modified
	if resp.StatusCode == 304 {
		return nil
	}

	// Error
	return errors.New(resp.Status)
}

func (c *client) doRequest(method string, s *fetchState) (*http.Response, error) {
	req, err := http.NewRequest(method, s.resource, nil)
	if err != nil {
		return nil, err
	}
	if c.user != "" {
		req.SetBasicAuth(c.user, c.password)
	}
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	if !s.lastModified.IsZero() {
		req.Header.Set("If-Modified-Since", formatHttpTime(s.lastModified))
	}
	if s.offset > 0 {
		req.Header.Set("Range", "bytes="+strconv.FormatInt(s.offset, 10)+"-")
	}
	resp, err := c.c.Do(req)
	if c.dumpHeaders {
		dumpHeaders(req, resp)
	}
	return resp, err
}

func dumpHeaders(req *http.Request, resp *http.Response) {
	fmt.Fprintf(os.Stderr, "\n-- REQUEST: %s %s\n", req.Method, req.URL)
	fmt.Fprintf(os.Stderr, "-- REQUEST HEADERS BEGIN --\n")
	req.Header.Write(os.Stderr)
	fmt.Fprintf(os.Stderr, "-- REQUEST HEADERS END --\n\n")
	fmt.Fprintf(os.Stderr, "-- RESPONSE: %s\n", resp.Status)
	fmt.Fprintf(os.Stderr, "-- RESPONSE HEADERS BEGIN --\n")
	resp.Header.Write(os.Stderr)
	fmt.Fprintf(os.Stderr, "-- RESPONSE HEADERS END --\n\n")
}

func formatHttpTime(t time.Time) string {
	return t.Format(time.RFC1123)
}

func parseHttpTime(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC1123, s)
	if err == nil {
		return t, nil
	}
	t, err = time.Parse(time.RFC850, s)
	if err == nil {
		return t, nil
	}
	return time.Parse(time.ANSIC, s)
}
