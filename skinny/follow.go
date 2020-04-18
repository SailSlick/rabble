package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	pb "github.com/cpssd/rabble/services/proto"
	"github.com/golang/protobuf/ptypes"
	wrapperpb "github.com/golang/protobuf/ptypes/wrappers"
	"github.com/gorilla/mux"
	"google.golang.org/grpc"
)

const (
	pendingFollowsNotFound = "Issue with finding pending follows.\n"
	modifyFollowFailed     = "Could not modify follow"
)

func (s *serverWrapper) handleFollow() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		var j pb.LocalToAnyFollow
		err := decoder.Decode(&j)
		enc := json.NewEncoder(w)

		errResp := &pb.GeneralResponse{
			ResultType: pb.ResultType_ERROR,
		}
		if err != nil {
			log.Printf(invalidJSONErrorWithPrint, err)
			w.WriteHeader(http.StatusBadRequest)
			errResp.Error = invalidJSONError
			enc.Encode(errResp)
			return
		}

		handle, err := s.getSessionHandle(r)
		if err != nil {
			log.Printf("Call to follow by not logged in user")
			w.WriteHeader(http.StatusForbidden)
			errResp.Error = loginRequired
			enc.Encode(errResp)
			return
		}
		// Even if the request was sent with a different follower, use the
		// handle of the logged in user.
		j.Follower = handle

		ts := ptypes.TimestampNow()
		j.Datetime = ts

		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutDuration)
		defer cancel()
		resp, err := s.follows.SendFollowRequest(ctx, &j)
		if err != nil {
			log.Printf("Could not send follow request: %#v", err)
			w.WriteHeader(http.StatusInternalServerError)
			errResp.Error = invalidJSONError
			enc.Encode(errResp)
			return
		}

		err = enc.Encode(resp)
		if err != nil {
			log.Printf("Could not marshal follow result: %#v", err)
			w.WriteHeader(http.StatusInternalServerError)
			enc.Encode(errResp)
		}
	}
}

func (s *serverWrapper) handleUnfollow() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		var j pb.LocalToAnyFollow
		err := decoder.Decode(&j)
		enc := json.NewEncoder(w)

		errResp := &pb.GeneralResponse{
			ResultType: pb.ResultType_ERROR,
		}
		if err != nil {
			log.Printf(invalidJSONErrorWithPrint, err)
			w.WriteHeader(http.StatusBadRequest)
			errResp.Error = invalidJSONError
			enc.Encode(errResp)
			return
		}

		handle, err := s.getSessionHandle(r)
		if err != nil {
			log.Printf("Call to unfollow by not logged in user")
			w.WriteHeader(http.StatusForbidden)
			errResp.Error = loginRequired
			enc.Encode(errResp)
			return
		}
		// Even if the request was sent with a different follower, use the
		// handle of the logged in user.
		j.Follower = handle

		ts := ptypes.TimestampNow()
		j.Datetime = ts

		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutDuration)
		defer cancel()
		resp, err := s.follows.SendUnfollow(ctx, &j)
		if err != nil {
			log.Printf("Could not send unfollow: %#v", err)
			w.WriteHeader(http.StatusInternalServerError)
			errResp.Error = invalidJSONError
			enc.Encode(errResp)
			return
		}

		err = enc.Encode(resp)
		if err != nil {
			log.Printf("Could not marshal unfollow result: %#v", err)
			w.WriteHeader(http.StatusInternalServerError)
			enc.Encode(errResp)
		}
	}
}

func (s *serverWrapper) handleRssFollow() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		var j pb.LocalToRss
		err := decoder.Decode(&j)
		enc := json.NewEncoder(w)

		errResp := &pb.GeneralResponse{
			ResultType: pb.ResultType_ERROR,
		}
		if err != nil {
			log.Printf(invalidJSONErrorWithPrint, err)
			w.WriteHeader(http.StatusBadRequest)
			errResp.Error = invalidJSONError
			enc.Encode(errResp)
			return
		}

		handle, err := s.getSessionHandle(r)
		if err != nil {
			log.Printf("Call to follow rss by not logged in user")
			w.WriteHeader(http.StatusForbidden)
			errResp.Error = loginRequired
			enc.Encode(errResp)
			return
		}

		// Even if the request was sent with a different follower user the
		// handle of the logged in user.
		j.Follower = handle

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		resp, err := s.follows.RssFollowRequest(ctx, &j)
		if err != nil {
			log.Printf("Could not send rss follow request: %#v", err)
			w.WriteHeader(http.StatusInternalServerError)
			errResp.Error = invalidJSONError
			enc.Encode(errResp)
			return
		}

		err = enc.Encode(resp)
		if err != nil {
			log.Printf("Could not marshal rss follow result: %#v", err)
			w.WriteHeader(http.StatusInternalServerError)
			enc.Encode(errResp)
			return
		}
	}
}

// FollowGetter is a wrapper around get follows so the handler can
// get following or followers without duplication of code
type FollowGetter func(context.Context, *pb.GetFollowsRequest,
	...grpc.CallOption) (*pb.GetFollowsResponse, error)

func (s *serverWrapper) handleGetFollows(f FollowGetter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		v := mux.Vars(r)
		username, ok := v["username"]
		if !ok || username == "" {
			w.WriteHeader(http.StatusBadRequest) // Bad Request.
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutDuration)
		defer cancel()
		fq := &pb.GetFollowsRequest{
			Username: username,
		}

		if uid, err := s.getSessionGlobalID(r); err == nil {
			fq.UserGlobalId = &wrapperpb.Int64Value{Value: uid}
		}

		followers, err := f(ctx, fq)
		if err != nil {
			log.Printf("Error in handleGetFollowers(): could not get followers: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		err = enc.Encode(followers)
		if err != nil {
			log.Printf("could not marshal followers: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

func (s *serverWrapper) handleGetFollowers() http.HandlerFunc {
	return s.handleGetFollows(s.follows.GetFollowers)
}

func (s *serverWrapper) handleGetFollowing() http.HandlerFunc {
	return s.handleGetFollows(s.follows.GetFollowing)
}

func (s *serverWrapper) handlePendingFollows() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		var cResp clientResp

		handle, err := s.getSessionHandle(r)
		if err != nil {
			log.Printf("Call to follow by not logged in user")
			w.WriteHeader(http.StatusForbidden)
			return
		}

		pr := &pb.PendingFollowRequest{
			Handle: handle,
		}
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutDuration)
		defer cancel()

		resp, err := s.database.PendingFollows(ctx, pr)
		if err != nil {
			log.Printf("Could not get pending follows. Err: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			cResp.Error = pendingFollowsNotFound
			enc.Encode(cResp)
			return
		}

		if resp.ResultType != pb.ResultType_OK {
			log.Printf("Could not get pending follows. Err: %v", resp.Error)
			w.WriteHeader(http.StatusInternalServerError)
			cResp.Error = pendingFollowsNotFound
			enc.Encode(cResp)
			return
		}

		log.Print(resp)
		err = enc.Encode(resp)
		if err != nil {
			log.Printf("could not marshal pending response: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

func (s *serverWrapper) handleAcceptFollow() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, err := s.getSessionGlobalID(r)
		if err != nil {
			log.Printf("Call to follow by not logged in user")
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		var cResp clientResp

		decoder := json.NewDecoder(r.Body)
		var af pb.AcceptFollowRequest
		err = decoder.Decode(&af)
		if err != nil {
			log.Printf(invalidJSONErrorWithPrint, err)
			w.WriteHeader(http.StatusBadRequest)
			cResp.Error = invalidJSONError
			enc.Encode(cResp)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutDuration)
		defer cancel()
		resp, err := s.follows.AcceptFollow(ctx, &af)
		if err != nil {
			log.Printf(modifyFollowFailed+": %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			cResp.Error = modifyFollowFailed
			enc.Encode(cResp)
			return
		}

		if resp.ResultType != pb.ResultType_OK {
			log.Printf(modifyFollowFailed+": %v", resp.Error)
			w.WriteHeader(http.StatusInternalServerError)
			cResp.Error = modifyFollowFailed
			enc.Encode(cResp)
			return
		}

		w.Write([]byte("OK"))
	}
}
