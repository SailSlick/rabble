package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	pb "github.com/cpssd/rabble/services/proto"
	util "github.com/cpssd/rabble/services/utils"
	webfinger "github.com/writeas/go-webfinger"
)

func getStrippedHost(hostname string) string {
	// Strip the port away from the hostname.
	split := strings.SplitN(hostname, ":", 2)
	// This shouldn't ever happen, but let's be sure.
	if split == nil || len(split) == 0 {
		log.Fatalf("Cannot split hostname %#v, expecting 'HOST:PORT'",
			hostname)
	}
	return split[0]
}

func (s *serverWrapper) newWebfingerHandler() http.HandlerFunc {
	wf := webfinger.Default(&wfResolver{
		users:     s.database,
		hostname:  getStrippedHost(s.hostname),
		debugHost: s.hostname,
	})
	wf.NoTLSHandler = nil
	return http.HandlerFunc(wf.Webfinger)
}

type wfResolver struct {
	users     util.UsersGetter
	hostname  string
	debugHost string
}

func (wf *wfResolver) subject(user *pb.UsersEntry) string {
	return fmt.Sprintf("acct:%s@%s", user.Handle, wf.hostname)
}

func (wf *wfResolver) genLinks(user *pb.UsersEntry) []webfinger.Link {
	// Add a special case for local debugging, where we won't have a "dot" in the
	// hostname.
	// This does assume that non debug rabble users are using HTTPS and a standard
	// port, but they're probably decent assumptions to make.
	host := wf.hostname
	protocol := "https"
	if !strings.Contains(wf.hostname, ".") {
		host = wf.debugHost
		protocol = "http"
	}

	//TODO(iandioch): Add magic-public-key webfinger links.
	html := webfinger.Link{
		HRef: fmt.Sprintf("%s://%s/#/@%s", protocol, host, user.Handle),
		Rel:  "http://webfinger.net/rel/profile-page",
		Type: "text/html",
	}
	ap := webfinger.Link{
		HRef: fmt.Sprintf("%s://%s/ap/@%s", protocol, host, user.Handle),
		Rel:  "self",
		Type: "application/activity+json",
	}
	// http://microformats.org/wiki/rel-feed
	rss := webfinger.Link{
		HRef: fmt.Sprintf("%s://%s/c2s/%s/rss", protocol, host, user.GlobalId),
		Rel:  "feed",
		Type: "application/rss+xml",
	}

	// http://microformats.org/wiki/rel-alternate
	altRss := webfinger.Link{
		HRef: fmt.Sprintf("%s://%s/c2s/%s/rss", protocol, host, user.GlobalId),
		Rel:  "alternative",
		Type: "application/rss+xml",
	}
	return []webfinger.Link{html, ap, rss, altRss}
}

// FindUser finds the user given the username and hostname
func (wf *wfResolver) FindUser(handle string, host, requestHost string, r []webfinger.Rel) (*webfinger.Resource, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutDuration)
	defer cancel()

	// We only support looking up hosts that exist on our server.
	if host != wf.hostname && host != wf.debugHost {
		fmt.Printf("webfinger: lookup on %s, expecting %s", host, wf.hostname)
		return nil, util.UserNotFoundErr
	}

	u, err := util.GetAuthorFromDb(ctx, handle, "", true, 0, wf.users)
	if err != nil {
		fmt.Printf("webfinger: user request error: %v", err)
		return nil, err
	}

	res := &webfinger.Resource{
		Subject: wf.subject(u),
		Links:   wf.genLinks(u),
	}
	return res, nil
}

// DummyUser allows us to fake a user, to prevent user enumeration.
//
// TODO(devoxel): create realistic fakes rather than 404s
// See https://github.com/sheenobu/go-webfinger/blob/master/resolver.go
func (wf *wfResolver) DummyUser(username string, hostname string, r []webfinger.Rel) (*webfinger.Resource, error) {
	return nil, util.UserNotFoundErr
}

// IsNotFoundError returns true if the given error is a not found error.
func (wf *wfResolver) IsNotFoundError(err error) bool {
	return err == util.UserNotFoundErr
}
