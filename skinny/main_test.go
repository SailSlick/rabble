package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"google.golang.org/grpc"

	dbpb "proto/database"
)

const (
	fakeTitle = "fake"
)

type DatabaseFake struct {
	dbpb.DatabaseClient

	// The most recent postRequest
	rq *dbpb.PostsRequest
}

func (d *DatabaseFake) Posts(_ context.Context, r *dbpb.PostsRequest, _ ...grpc.CallOption) (*dbpb.PostsResponse, error) {
	d.rq = r
	return &dbpb.PostsResponse{
		Results: []*dbpb.PostsEntry{{
			Title: fakeTitle,
		}},
	}, nil
}

func newTestServerWrapper() *serverWrapper {
	// TODO(iandioch): Fake/mock instead of using real dependencies
	r := mux.NewRouter()
	srv := &http.Server{}
	s := &serverWrapper{
		router:            r,
		server:            srv,
		shutdownWait:      20 * time.Second,
		greetingCardsConn: nil,
		greetingCards:     nil,
		database:          &DatabaseFake{},
	}
	s.setupRoutes()
	return s
}

func TestHandleNotImplemented(t *testing.T) {
	req, _ := http.NewRequest("GET", "/test", nil)
	res := httptest.NewRecorder()
	newTestServerWrapper().handleNotImplemented()(res, req)

	if res.Code != http.StatusNotImplemented {
		t.Errorf("Expected 501 Not Implemented, got %#v", res.Code)
	}
	if res.Body.String() != "Not Implemented\n" {
		t.Errorf("Expected 'Not Implemented' body, got %#v", res.Body.String())
	}
}

func TestHandleFollow(t *testing.T) {
	req, _ := http.NewRequest("GET", "/test", nil)
	res := httptest.NewRecorder()
	vars := map[string]string{
		"username": "testuser",
	}
	req = mux.SetURLVars(req, vars)
	newTestServerWrapper().handleFollow()(res, req)
	if res.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %#v", res.Code)
	}
	if res.Body.String() != "Followed testuser\n" {
		t.Errorf("Expected 'Followed testuser' body, got %#v", res.Body.String())
	}
}

func TestHandleCreateArticleSuccess(t *testing.T) {
	timeParseFormat := "2006-01-02T15:04:05.000Z"
	currentTimeString := time.Now().Format(timeParseFormat)
	jsonString := `{ "author": "testuser", "body": "test post", "title": "test title", "creation_datetime": "` + currentTimeString + `" }`
	jsonBuffer := bytes.NewBuffer([]byte(jsonString))
	req, _ := http.NewRequest("POST", "/test?username=testuser", jsonBuffer)
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	newTestServerWrapper().handleCreateArticle()(res, req)
	if res.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %#v", res.Code)
	}
	if res.Body.String() != "Created blog with title: test title\n" {
		t.Errorf("Expected 'Created blog with title: test title' body, got %#v", res.Body.String())
	}
}

func TestHandleCreateArticleBadJSON(t *testing.T) {
	jsonString := `{ author: "testuser", "body": "test post", "title": "test title" }`
	jsonBuffer := bytes.NewBuffer([]byte(jsonString))
	req, _ := http.NewRequest("POST", "/test?username=testuser", jsonBuffer)
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	vars := map[string]string{
		"username": "testuser",
	}
	req = mux.SetURLVars(req, vars)
	newTestServerWrapper().handleCreateArticle()(res, req)
	if res.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %#v", res.Code)
	}
	if res.Body.String() != "Invalid JSON\n" {
		t.Errorf("Expected 'Invalid JSON' body, got %#v", res.Body.String())
	}
}

func TestHandleCreateArticleBadCreationDatetime(t *testing.T) {
	jsonString := `{ "author": "testuser", "body": "test post", "title": "test title", "creation_datetime": "never" }`
	jsonBuffer := bytes.NewBuffer([]byte(jsonString))
	req, _ := http.NewRequest("POST", "/test?username=testuser", jsonBuffer)
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	newTestServerWrapper().handleCreateArticle()(res, req)
	if res.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %#v", res.Code)
	}
	if res.Body.String() != "Invalid creation time\n" {
		t.Errorf("Expected 'Invalid creation time' body, got %#v", res.Body.String())
	}
}

func TestHandleCreateArticleOldCreationDatetime(t *testing.T) {
	jsonString := `{ "author": "testuser", "body": "test post", "title": "test title", "creation_datetime": "2006-01-02T15:04:05.000Z" }`
	jsonBuffer := bytes.NewBuffer([]byte(jsonString))
	req, _ := http.NewRequest("POST", "/test?username=testuser", jsonBuffer)
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	newTestServerWrapper().handleCreateArticle()(res, req)
	if res.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %#v", res.Code)
	}
	if res.Body.String() != "Old creation time\n" {
		t.Errorf("Expected 'Old creation time' body, got %#v", res.Body.String())
	}
}

func TestHandleCreateArticleUnauthorizedUser(t *testing.T) {
	timeParseFormat := "2006-01-02T15:04:05.000Z"
	currentTimeString := time.Now().Format(timeParseFormat)
	jsonString := `{ "author": "badguy", "body": "test post", "title": "test title", "creation_datetime": "` + currentTimeString + `" }`
	jsonBuffer := bytes.NewBuffer([]byte(jsonString))
	req, _ := http.NewRequest("POST", "/test?username=testuser", jsonBuffer)
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	newTestServerWrapper().handleCreateArticle()(res, req)
	if res.Code != http.StatusForbidden {
		t.Errorf("Expected 403, got %#v", res.Code)
	}
	if res.Body.String() != "Post under incorrect username\n" {
		t.Errorf("Expected 'Post under incorrect username' body, got %#v", res.Body.String())
	}
}

func TestFeed(t *testing.T) {
	req, _ := http.NewRequest("GET", "/c2s/feed", nil)
	res := httptest.NewRecorder()
	srv := newTestServerWrapper()

	srv.handleFeed()(res, req)
	if res.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %#v", res.Code)
	}

	var r []dbpb.PostsEntry
	if err := json.Unmarshal(res.Body.Bytes(), &r); err != nil {
		t.Fatalf("json.Unmarshal(%#v) unexpected error: %v", res.Body.String(), err)
	}

	if len(r) != 1 {
		t.Fatalf("Expected one result, got %v", len(r))
	}

	if r[0].Title != fakeTitle {
		t.Fatalf("Expected faked response, got %v", r[0].Title)
	}
}
