package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"time"

	pb "github.com/cpssd/rabble/services/proto"
	"github.com/golang/protobuf/ptypes"
	tspb "github.com/golang/protobuf/ptypes/timestamp"
	wrapperpb "github.com/golang/protobuf/ptypes/wrappers"
	"github.com/gorilla/mux"
)

const (
	staticAssets              = "/repo/build_out/chump_dist"
	defaultTimeoutDuration    = time.Second * 5
	invalidJSONError          = "Invalid JSON"
	invalidJSONErrorWithPrint = "Invalid JSON, error: %v\n"
	loginRequired             = "Login Required"
)

type clientResp struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	ID      string `json:"id"`
}

func parseTimestamp(w http.ResponseWriter, published string, resp *clientResp) (*tspb.Timestamp, error) {
	invalidCreationTimeMessage := "Invalid creation time\n"

	// We accept standard AP dates in a variety of resolutions.
	formats := []string{
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05.000000000Z",
	}

	var (
		protoTimestamp *tspb.Timestamp
		success        bool
	)

	for _, format := range formats {
		parsedCreationDatetime, err := time.Parse(format, published)
		if err != nil {
			continue
		}

		protoTimestamp, err = ptypes.TimestampProto(parsedCreationDatetime)
		if err != nil {
			continue
		}

		success = true
		break
	}

	if !success {
		w.WriteHeader(http.StatusBadRequest)
		resp.Error = invalidCreationTimeMessage
		return nil, fmt.Errorf(invalidCreationTimeMessage)
	}

	return protoTimestamp, nil
}

// SearchResult holds the posts and users from a search
type SearchResult struct {
	Posts []*pb.Post `json:"posts"`
	Users []*pb.User `json:"users"`
}

func (s *serverWrapper) handleSearch() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutDuration)
		defer cancel()

		query := r.URL.Query().Get("query")
		if query == "" {
			w.WriteHeader(http.StatusBadRequest) // Bad Request
			return
		}
		query, qUErr := url.QueryUnescape(query)
		if qUErr != nil {
			log.Printf("Error in Search(%v) query unescape: %v", query, qUErr)
			w.WriteHeader(http.StatusBadRequest) // Bad Request
			return
		}

		sq := &pb.SearchQuery{QueryText: query}
		req := &pb.SearchRequest{Query: sq}
		if globalID, gIErr := s.getSessionGlobalID(r); gIErr == nil {
			// If the user is logged in then propagate their global ID.
			req.UserGlobalId = &wrapperpb.Int64Value{Value: globalID}
		}
		resp, err := s.search.Search(ctx, req)
		if err != nil {
			log.Printf("Error in Search(%v): %v", *req, err)
		}

		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		body := &SearchResult{Users: resp.UResults, Posts: resp.Results}
		err = enc.Encode(body)
		if err != nil {
			log.Printf("Could not marshal recommended follows: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

func (s *serverWrapper) handleTrackView() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		var v pb.View
		err := decoder.Decode(&v)
		if err != nil {
			log.Printf(invalidJSONErrorWithPrint, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// uid = 0 if no user is logged in.
		uid, _ := s.getSessionGlobalID(r)

		v.User = uid

		ts := ptypes.TimestampNow()
		v.Datetime = ts

		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutDuration)
		defer cancel()
		_, err = s.database.AddView(ctx, &v)
		if err != nil {
			log.Printf("Could not send view: %#v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
}

func (s *serverWrapper) handleAddLog() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		var v pb.ClientLog
		err := decoder.Decode(&v)
		if err != nil {
			log.Printf(invalidJSONErrorWithPrint, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// uid = 0 if no user is logged in.
		uid, _ := s.getSessionGlobalID(r)

		v.User = uid

		ts := ptypes.TimestampNow()
		v.Datetime = ts

		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutDuration)
		defer cancel()
		_, err = s.database.AddLog(ctx, &v)
		if err != nil {
			log.Printf("Could not add log: %#v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
}

// handleNotImplemented returns a http.HandlerFunc with a 501 Not Implemented
// error.
func (s *serverWrapper) handleNotImplemented() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Error(
			w,
			http.StatusText(http.StatusNotImplemented),
			http.StatusNotImplemented,
		)
	}
}

func (s *serverWrapper) handleIndex() http.HandlerFunc {
	indexPath := path.Join(staticAssets, "index.html")
	b, err := ioutil.ReadFile(indexPath)
	if err != nil {
		log.Fatalf("could not find index.html: %v", err)
	}
	return func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write(b)
		if err != nil {
			log.Printf("handleIndex failed to write: %v\n", err)
		}
	}
}

func (s *serverWrapper) handleFeed() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutDuration)
		defer cancel()

		v := mux.Vars(r)
		strUserID := v["userId"]
		userID := int64(0)
		if strUserID != "" {
			var err error
			userID, err = strconv.ParseInt(strUserID, 10, 64)
			if err != nil {
				log.Printf("Could not convert userId to int64: id(%v)\n", strUserID)
				w.WriteHeader(http.StatusBadRequest) // Bad Request.
				return
			}
		}

		fr := &pb.FeedRequest{UserId: userID}
		if globalID, err := s.getSessionGlobalID(r); err == nil {
			// If the user is logged in then propagate their global ID.
			fr.UserGlobalId = &wrapperpb.Int64Value{Value: globalID}
		}
		resp, err := s.feed.Get(ctx, fr)
		if err != nil {
			log.Printf("Error in feed.Get(%v): %v\n", *fr, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		err = enc.Encode(resp)
		if err != nil {
			log.Printf("could not marshal blogs: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

func (s *serverWrapper) handleFeedPerUser() http.HandlerFunc {
	errorMap := map[pb.FeedResponse_FeedError]int{
		pb.FeedResponse_USER_NOT_FOUND: 404,
		pb.FeedResponse_UNAUTHORIZED:   401,
	}
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutDuration)
		defer cancel()

		v := mux.Vars(r)

		username, ok := v["username"]
		if !ok || username == "" {
			w.WriteHeader(http.StatusBadRequest) // Bad Request.
			return
		}

		fr := &pb.FeedRequest{Username: username}
		if globalID, err := s.getSessionGlobalID(r); err == nil {
			// If the user is logged in then propagate their global ID.
			fr.UserGlobalId = &wrapperpb.Int64Value{Value: globalID}
		}
		resp, err := s.feed.PerUser(ctx, fr)
		if err != nil {
			log.Printf("Error in feed.PerUser(%v): %v", *fr, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if resp.Error != pb.FeedResponse_NO_ERROR {
			w.WriteHeader(errorMap[resp.Error])
			return
		}

		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		err = enc.Encode(resp)
		if err != nil {
			log.Printf("could not marshal blogs: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

func (s *serverWrapper) handleRssPerUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutDuration)
		defer cancel()

		v := mux.Vars(r)
		strUserID, ok := v["userId"]
		if !ok || strUserID == "" {
			log.Printf("Could not parse userId from url in RssPerUserl\n")
			w.WriteHeader(http.StatusBadRequest) // Bad Request.
			return
		}
		userID, err := strconv.ParseInt(strUserID, 10, 64)
		if err != nil {
			log.Printf("Could not convert userId to int64: id(%v)\n", strUserID)
			w.WriteHeader(http.StatusBadRequest) // Bad Request.
			return
		}
		ue := &pb.UsersEntry{GlobalId: userID}
		resp, err := s.rss.PerUserRss(ctx, ue)
		if err != nil {
			log.Printf("Error in rss.PerUserRss(%v): %v", *ue, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if resp.ResultType == pb.RssResponse_ERROR {
			log.Printf("Error in rss.PerUserRss(%v): %v", *ue, resp.Message)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if resp.ResultType == pb.RssResponse_DENIED {
			log.Printf("Access denied in rss.PerUserRss(%v): %v", *ue, resp.Message)
			w.WriteHeader(http.StatusForbidden)
			return
		}

		w.Header().Set("Content-Type", "application/rss+xml")
		fmt.Fprintf(w, resp.Feed)
	}
}

func (s *serverWrapper) handlePerArticlePage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutDuration)
		defer cancel()
		log.Println("Per Article page called")

		v := mux.Vars(r)
		strArticleID, aOk := v["article_id"]
		if !aOk || strArticleID == "" {
			log.Println("Per Article page passed bad articleId value")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		articleID, string2IntErr := strconv.ParseInt(strArticleID, 10, 64)
		if string2IntErr != nil {
			log.Println("Article ID could not be converted to int")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		fr := &pb.ArticleRequest{ArticleId: articleID}
		if globalID, err := s.getSessionGlobalID(r); err == nil {
			// If the user is logged in then propagate their global ID.
			fr.UserGlobalId = &wrapperpb.Int64Value{Value: globalID}
		}
		resp, err := s.feed.PerArticle(ctx, fr)
		if err != nil {
			log.Printf("Error in getting per Article page: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		err = enc.Encode(resp)
		if err != nil {
			log.Printf("Could not marshal article: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

type createArticleStruct struct {
	Author           string   `json:"author"`
	Body             string   `json:"body"`
	Title            string   `json:"title"`
	CreationDatetime string   `json:"creation_datetime"`
	Tags             []string `json:"tags"`
	Summary          string   `json:"summary"`
}

func (s *serverWrapper) handleCreateArticle() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		var t createArticleStruct
		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		var cResp clientResp

		jsonErr := decoder.Decode(&t)
		if jsonErr != nil {
			log.Printf(invalidJSONErrorWithPrint, jsonErr)
			w.WriteHeader(http.StatusBadRequest)
			cResp.Error = invalidJSONError
			enc.Encode(cResp)
			return
		}

		protoTimestamp, parseErr := parseTimestamp(w, t.CreationDatetime, &cResp)
		if parseErr != nil {
			log.Println(parseErr)
			w.WriteHeader(http.StatusBadRequest)
			enc.Encode(cResp)
			return
		}

		globalID, gIErr := s.getSessionGlobalID(r)
		if gIErr != nil {
			log.Printf("Create Article call from user not logged in")
			w.WriteHeader(http.StatusForbidden)
			cResp.Error = loginRequired
			enc.Encode(cResp)
			return
		}

		na := &pb.NewArticle{
			AuthorId:         globalID,
			Body:             t.Body,
			Title:            t.Title,
			CreationDatetime: protoTimestamp,
			Foreign:          false,
			Tags:             t.Tags,
			Summary:          t.Summary,
		}
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutDuration)
		defer cancel()

		resp, err := s.article.CreateNewArticle(ctx, na)
		if err != nil {
			log.Printf("Could not create new article: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			cResp.Error = "Issue with creating article"
			enc.Encode(cResp)
			return
		}
		if resp.ResultType == pb.ResultType_ERROR {
			log.Printf("Could not create new article: %v", resp.Error)
			w.WriteHeader(http.StatusInternalServerError)
			cResp.Error = "Issue with creating article"
			enc.Encode(cResp)
			return
		}

		log.Printf("User Id: %#v attempted to create a post with title: %v and Id: %v\n", globalID, t.Title, resp.GlobalId)
		cResp.Message = "Article created"
		cResp.ID = resp.GlobalId

		enc.Encode(cResp)
	}
}

type editArticleRequest struct {
	ArticleID int64    `json:"article_id"`
	Body      string   `json:"body"`
	Title     string   `json:"title"`
	Tags      []string `json:"tags"`
	Summary   string   `json:"summary"`
}

func (s *serverWrapper) handleEditArticle() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		var t editArticleRequest
		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		var cResp clientResp

		jsonErr := decoder.Decode(&t)
		if jsonErr != nil {
			log.Printf(invalidJSONErrorWithPrint, jsonErr)
			w.WriteHeader(http.StatusBadRequest)
			cResp.Error = invalidJSONError
			enc.Encode(cResp)
			return
		}

		globalID, gIErr := s.getSessionGlobalID(r)
		if gIErr != nil {
			log.Printf("Edit article call from user not logged in")
			w.WriteHeader(http.StatusForbidden)
			cResp.Error = loginRequired
			enc.Encode(cResp)
			return
		}

		ud := &pb.UpdateDetails{
			UserId:    globalID,
			ArticleId: t.ArticleID,
			Body:      t.Body,
			Tags:      t.Tags,
			Title:     t.Title,
			Summary:   t.Summary,
		}
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutDuration)
		defer cancel()

		resp, err := s.s2sUpdate.SendUpdateActivity(ctx, ud)
		if err != nil {
			log.Printf("Could not edit article: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			cResp.Error = "Issue with editing article"
			enc.Encode(cResp)
			return
		} else if resp.ResultType == pb.ResultType_ERROR {
			log.Printf("Could not edit article: %v", resp.Error)
			w.WriteHeader(http.StatusInternalServerError)
			cResp.Error = "Issue with editing article"
			enc.Encode(cResp)
			return
		} else if resp.ResultType == pb.ResultType_DENIED {
			log.Printf("Editing of article denied")
			w.WriteHeader(http.StatusForbidden)
			cResp.Error = "Editing of article is denied"
			enc.Encode(cResp)
			return
		}

		log.Printf("User Id: %#v attempted to edit an article with title: %v\n",
			globalID, t.Title)
		cResp.Message = "Article edited"
		enc.Encode(cResp)
	}
}

type deleteArticleRequest struct {
	ArticleID int64 `json:"article_id"`
}

func (s *serverWrapper) handleDeleteArticle() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		var t deleteArticleRequest
		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		var cResp clientResp

		jsonErr := decoder.Decode(&t)
		if jsonErr != nil {
			log.Printf(invalidJSONErrorWithPrint, jsonErr)
			w.WriteHeader(http.StatusBadRequest)
			cResp.Error = invalidJSONError
			enc.Encode(cResp)
			return
		}

		globalID, gIErr := s.getSessionGlobalID(r)
		if gIErr != nil {
			log.Printf("Delete article call from user not logged in")
			w.WriteHeader(http.StatusForbidden)
			cResp.Error = loginRequired
			enc.Encode(cResp)
			return
		}

		delpb := &pb.DeleteDetails{
			UserId:    globalID,
			ArticleId: t.ArticleID,
		}
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutDuration)
		defer cancel()

		resp, err := s.s2sDelete.SendDeleteActivity(ctx, delpb)
		if err != nil {
			log.Printf("Could not delete article: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			cResp.Error = "Issue with deleting article"
			enc.Encode(cResp)
			return
		} else if resp.ResultType == pb.ResultType_ERROR {
			log.Printf("Could not delete article: %v", resp.Error)
			w.WriteHeader(http.StatusInternalServerError)
			cResp.Error = "Issue with deleting article"
			enc.Encode(cResp)
			return
		} else if resp.ResultType == pb.ResultType_ERROR_400 {
			log.Printf("Could not delete article: %v", resp.Error)
			w.WriteHeader(http.StatusBadRequest)
			cResp.Error = "Issue with deleting article"
			enc.Encode(cResp)
			return
		} else if resp.ResultType == pb.ResultType_DENIED {
			log.Printf("Deletion of article denied")
			w.WriteHeader(http.StatusForbidden)
			cResp.Error = "Deletion of article is denied"
			enc.Encode(cResp)
			return
		}

		log.Printf("User Id: %#v attempted to delete an article id: %v\n",
			globalID, t.ArticleID)
		cResp.Message = "Article deleted"
		enc.Encode(cResp)
	}
}

func (s *serverWrapper) handlePreviewArticle() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		var t createArticleStruct
		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		var cResp clientResp

		jsonErr := decoder.Decode(&t)
		if jsonErr != nil {
			log.Printf(invalidJSONErrorWithPrint, jsonErr)
			w.WriteHeader(http.StatusBadRequest)
			cResp.Error = invalidJSONError
			enc.Encode(cResp)
			return
		}

		protoTimestamp, parseErr := parseTimestamp(w, t.CreationDatetime, &cResp)
		if parseErr != nil {
			log.Println(parseErr)
			w.WriteHeader(http.StatusBadRequest)
			enc.Encode(cResp)
			return
		}

		globalID, gIErr := s.getSessionGlobalID(r)
		if gIErr != nil {
			log.Printf("Preview Article call from user not logged in")
			w.WriteHeader(http.StatusForbidden)
			cResp.Error = loginRequired
			enc.Encode(cResp)
			return
		}

		na := &pb.NewArticle{
			AuthorId:         globalID,
			Body:             t.Body,
			Title:            t.Title,
			CreationDatetime: protoTimestamp,
		}
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutDuration)
		defer cancel()

		resp, err := s.article.PreviewArticle(ctx, na)
		if err != nil {
			log.Printf("Could not create preview. Err: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			cResp.Error = "Issue with creating preview\n"
			enc.Encode(cResp)
			return
		}

		log.Printf("User Id: %#v attempted to create preview with title: %v\n", globalID, t.Title)
		// TODO(devoxel): Remove SetEscapeHTML and properly handle that client side
		enc.SetEscapeHTML(false)
		err = enc.Encode(resp.Preview)
		if err != nil {
			log.Printf("could not marshal post: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

type likeStruct struct {
	ArticleID int64 `json:"article_id"`
	IsLiked   bool  `json:"is_liked"`
}

func (s *serverWrapper) handleLike() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		decoder := json.NewDecoder(req.Body)
		var t likeStruct
		var cResp clientResp
		enc := json.NewEncoder(w)
		w.Header().Set("Content-Type", "application/json")

		jsonErr := decoder.Decode(&t)
		if jsonErr != nil {
			log.Printf(invalidJSONErrorWithPrint, jsonErr)
			w.WriteHeader(http.StatusBadRequest)
			cResp.Error = jsonErr.Error()
			enc.Encode(cResp)
			return
		}

		handle, err := s.getSessionHandle(req)
		if err != nil {
			log.Printf("Like call from user not logged in")
			w.WriteHeader(http.StatusForbidden)
			cResp.Error = loginRequired
			enc.Encode(cResp)
			return
		}

		if t.IsLiked {
			// Send a like
			like := &pb.LikeDetails{
				ArticleId:   t.ArticleID,
				LikerHandle: handle,
			}
			ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutDuration)
			defer cancel()
			resp, err := s.s2sLike.SendLikeActivity(ctx, like)
			if err != nil {
				log.Printf("Could not send like: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				cResp.Error = "Issue with sending like"
				enc.Encode(cResp)
				return
			} else if resp.ResultType != pb.ResultType_OK {
				log.Printf("Could not send like: %v", resp.Error)
				w.WriteHeader(http.StatusInternalServerError)
				cResp.Error = "Issue with sending like"
				enc.Encode(cResp)
				return
			}
		} else {
			// Send an unlike (undo)
			del := &pb.LikeUndoDetails{
				ArticleId:   t.ArticleID,
				LikerHandle: handle,
			}
			ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutDuration)
			defer cancel()
			resp, err := s.s2sUndo.SendLikeUndoActivity(ctx, del)
			if err != nil {
				log.Printf("Could not send undo: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				cResp.Error = "Issue with unliking"
				enc.Encode(cResp)
				return
			} else if resp.ResultType != pb.ResultType_OK {
				log.Printf("Could not send undo: %v", resp.Error)
				w.WriteHeader(http.StatusInternalServerError)
				cResp.Error = "Issue with unliking"
				enc.Encode(cResp)
				return
			}
		}
		cResp.Message = "Success"
		enc.Encode(cResp)
	}
}

// handleAnnounce handles the parsing and sending an Announces.
//
// Note that only the Article ID is neccesary to send, both the
// Announcer ID and the timestamp get generated by this handler.
func (s *serverWrapper) handleAnnounce() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		var v pb.AnnounceDetails
		err := decoder.Decode(&v)
		if err != nil {
			log.Printf(invalidJSONErrorWithPrint, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// We use their logged in GlobalID, since the client shouldn't
		// need to care about that detail.
		uid, err := s.getSessionGlobalID(r)
		if err != nil {
			log.Printf("Access denied in handleAnnounce: %v", err)
			w.WriteHeader(http.StatusForbidden)
			return
		}

		v.AnnouncerId = uid
		ts := ptypes.TimestampNow()
		v.AnnounceTime = ts

		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutDuration)
		defer cancel()
		resp, err := s.announce.SendAnnounceActivity(ctx, &v)
		if resp.ResultType == pb.ResultType_ERROR {
			log.Printf("Could not send announce: %#v", resp.Error)
			w.WriteHeader(http.StatusInternalServerError)
			return
		} else if resp.ResultType == pb.ResultType_ERROR_400 {
			log.Printf("Could not send announce: %#v", resp.Error)
			w.WriteHeader(http.StatusBadRequest)
			return
		} else if err != nil {
			log.Printf("Could not send announce: %#v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

/*
This section is for recommendations and their replacement no-op functions
*/

func (s *serverWrapper) handleRecommendFollows() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutDuration)
		defer cancel()

		v := mux.Vars(r)
		strUserID, ok := v["userId"]
		if !ok || strUserID == "" {
			log.Printf("Could not parse userId from url in handleRecommendFollows\n")
			w.WriteHeader(http.StatusBadRequest) // Bad Request.
			return
		}
		userID, err := strconv.ParseInt(strUserID, 10, 64)
		if err != nil {
			log.Printf("Could not convert userId to int64: id(%v)\n", strUserID)
			w.WriteHeader(http.StatusBadRequest) // Bad Request.
			return
		}

		req := &pb.FollowRecommendationRequest{UserId: userID}
		resp, err := s.followRecommendations.GetFollowRecommendations(ctx, req)
		if err != nil {
			log.Printf("Error in handleRecommendFollows(%v): %v", *req, err)
		}

		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		err = enc.Encode(resp.Results)
		if err != nil {
			log.Printf("Could not marshal recommended follows: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

func (s *serverWrapper) handlePostRecommendations() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutDuration)
		defer cancel()

		uid, err := s.getSessionGlobalID(r)
		if err != nil {
			log.Printf("Access denied in handlePostRecommendations: %v", err)
			w.WriteHeader(http.StatusForbidden)
			return
		}
		prr := &pb.PostRecommendationsRequest{UserId: uid}
		resp, err := s.postRecommendations.Get(ctx, prr)
		if err != nil {
			log.Printf("Error in postRecommendations.Get: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if resp.ResultType != pb.ResultType_OK {
			log.Printf("Error in postRecommendations.Get: %v", resp.Message)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		err = enc.Encode(resp)
		if err != nil {
			log.Printf("Could not marshal post recommendations: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

// handleNoOp is the handler for any service that is not running on this
// instance. The services that are configurable provide their docker routes as
// env vars to the skinny server. If those routes are equal to the no-op
// container skinny will route all calls to those services to this handler.
func (s *serverWrapper) handleNoOp() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, err := s.getSessionGlobalID(r)
		if err != nil {
			log.Printf("Access denied in handleNoOp: %v", err)
			w.WriteHeader(http.StatusForbidden)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)

		reply := clientResp{
			Message: "This option has been turned off on this rabble instance",
		}
		w.WriteHeader(http.StatusNotImplemented)
		enc.SetEscapeHTML(false)
		err = enc.Encode(reply)
		if err != nil {
			log.Printf("Could not marshal no op reply: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}
