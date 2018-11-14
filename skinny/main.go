package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"

	articlepb "github.com/cpssd/rabble/services/article/proto"
	dbpb "github.com/cpssd/rabble/services/database/proto"
	feedpb "github.com/cpssd/rabble/services/feed/proto"
	followspb "github.com/cpssd/rabble/services/follows/proto"
	createpb "github.com/cpssd/rabble/services/activities/create/proto"

	"github.com/golang/protobuf/ptypes"
	tspb "github.com/golang/protobuf/ptypes/timestamp"
	"github.com/gorilla/mux"
	//"github.com/piprate/json-gold/ld"
	"google.golang.org/grpc"
)

const (
	staticAssets    = "/repo/build_out/chump_dist"
	timeParseFormat = "2006-01-02T15:04:05.000Z"
	timeoutDuration = time.Minute * 5
)

type createActivityObjectStruct struct {
	Content string `json:"content"`
	Name string `json:"name"`
	Published string `json:"published"`
	AttributedTo string `json:"attributedTo"`
	Recipient []string `json:"to"`
	Type string `json:"type"`
}

type createActivityStruct struct {
	Actor string `json:"actor"`
	Context string `json:"@context"`
	Object createActivityObjectStruct `json:"object"`
	Recipient []string `json:"to"`
	Type string `json:"type"`
}

type createArticleStruct struct {
	Author           string `json:"author"`
	Body             string `json:"body"`
	Title            string `json:"title"`
	CreationDatetime string `json:"creation_datetime"`
}

// serverWrapper encapsulates the dependencies and config values of the server
// into one struct. Server endpoint handlers hang off of this struct and can
// access their dependencies through it. See
// https://medium.com/statuscode/how-i-write-go-http-services-after-seven-years-37c208122831
// for rationale and further explanation.
type serverWrapper struct {
	router *mux.Router
	server *http.Server
	// shutdownWait specifies how long the server should wait when shutting
	// down for existing connections to finish before forcing a shutdown.
	shutdownWait time.Duration

	// databaseConn is the underlying connection to the Database
	// service. This reference must be retained so it can by closed later.
	databaseConn *grpc.ClientConn
	// database is the RPC client for talking to the database service.
	database dbpb.DatabaseClient

	followsConn *grpc.ClientConn
	follows     followspb.FollowsClient
	articleConn *grpc.ClientConn
	article     articlepb.ArticleClient
	feedConn    *grpc.ClientConn
	feed        feedpb.FeedClient
	createConn  *grpc.ClientConn
	create      createpb.CreateClient
}

func (s *serverWrapper) handleFeed() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		v := mux.Vars(r)

		fr := &feedpb.FeedRequest{Username: v["username"]}
		resp, err := s.feed.Get(ctx, fr)
		if err != nil {
			log.Printf("Error in feed.Get(%v): %v\n", *fr, err)
		}

		w.Header().Set("Content-Type", "application/json")
		// TODO(devoxel): Remove SetEscapeHTML and properly handle that client side
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		err = enc.Encode(resp.Results)
		if err != nil {
			log.Printf("could not marshal blogs: %v", err)
			w.WriteHeader(500)
			return
		}
	}
}

func (s *serverWrapper) handleFeedPerUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		v := mux.Vars(r)
		if username, ok := v["username"]; !ok || username == "" {
			w.WriteHeader(400)  // Bad Request
			return
		}
		fr := &feedpb.FeedRequest{Username: v["username"]}
		resp, err := s.feed.PerUser(ctx, fr)
		if err != nil {
			log.Print("Error in feed.PerUser(%v): %v", *fr, err)
		}

		w.Header().Set("Content-Type", "application/json")
		// TODO(devoxel): Remove SetEscapeHTML and properly handle that client side
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		err = enc.Encode(resp.Results)
		if err != nil {
			log.Printf("could not marshal blogs: %v", err)
			w.WriteHeader(500)
			return
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
			http.StatusNotImplemented)
	}
}

func (s *serverWrapper) getIndexFile() []byte {
	// This flag is used in "go test", so we can use that to check if we're
	// in a test.
	if flag.Lookup("test.v") != nil {
		return []byte("testing html")
	}

	indexPath := path.Join(staticAssets, "index.html")
	b, err := ioutil.ReadFile(indexPath)
	if err != nil {
		log.Fatalf("could not find index.html: %v", err)
	}
	return b
}

func (s *serverWrapper) handleIndex() http.HandlerFunc {
	b := s.getIndexFile()
	return func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write(b)
		if err != nil {
			log.Printf("handleIndex failed to write: %v\n", err)
		}
	}
}

func (s *serverWrapper) handleFollow() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		var j followspb.LocalToAnyFollow
		err := decoder.Decode(&j)

		enc := json.NewEncoder(w)
		if err != nil {
			log.Printf("Invalid JSON. Err = %#v", err)
			w.WriteHeader(http.StatusBadRequest)
			e := &followspb.FollowResponse{
				ResultType: followspb.FollowResponse_ERROR,
				Error:      "Invalid JSON",
			}
			enc.Encode(e)
			return
		}

		ts := ptypes.TimestampNow()
		j.Datetime = ts

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		resp, err := s.follows.SendFollowRequest(ctx, &j)
		if err != nil {
			log.Fatalf("Could not send follow request: %#v", err)
			w.WriteHeader(http.StatusInternalServerError)
			e := &followspb.FollowResponse{
				ResultType: followspb.FollowResponse_ERROR,
				Error:      "Invalid JSON",
			}
			enc.Encode(e)
			return
		}

		err = enc.Encode(resp)
		if err != nil {
			log.Printf("Could not marshal follow result: %#v", err)
			w.WriteHeader(http.StatusInternalServerError)
			e := &followspb.FollowResponse{
				ResultType: followspb.FollowResponse_ERROR,
				Error:      "Invalid JSON",
			}
			enc.Encode(e)
		}
	}
}

func (s *serverWrapper) handleCreateArticle() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		var t createArticleStruct
		jsonErr := decoder.Decode(&t)
		if jsonErr != nil {
			log.Printf("Invalid JSON\n")
			log.Printf("Error: %s\n", jsonErr)
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Invalid JSON\n")
			return
		}

		protoTimestamp, parseErr := parseTimestamp(w, t.CreationDatetime)
		if parseErr != nil {
			log.Println(parseErr)
			return
		}

		na := &articlepb.NewArticle{
			Author:           t.Author,
			Body:             t.Body,
			Title:            t.Title,
			CreationDatetime: protoTimestamp,
			Foreign:          false,
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		resp, err := s.article.CreateNewArticle(ctx, na)
		if err != nil {
			log.Printf("Could not create new article: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Issue with creating article\n")
			return
		}

		log.Printf("User %#v attempted to create a post with title: %v\n", t.Author, t.Title)
		fmt.Fprintf(w, "Created blog with title: %v and result type: %d\n", t.Title, resp.ResultType)
		// TODO(sailslick) send the response
	}
}

func parseTimestamp(w http.ResponseWriter, published string) (*tspb.Timestamp, error) {
	parsedCreationDatetime, timeErr := time.Parse(timeParseFormat, published)
	protoTimestamp, protoTimeErr := ptypes.TimestampProto(parsedCreationDatetime)
	if timeErr != nil || protoTimeErr != nil {
		log.Printf("Error: %s\n", timeErr)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Invalid creation time\n")
		return nil, fmt.Errorf("Invalid creation time")
	}

	timeSinceRequest := time.Since(parsedCreationDatetime)
	if timeSinceRequest >= timeoutDuration || timeSinceRequest < 0 {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Old creation time\n")
		return nil, fmt.Errorf("Old creation time")
	}
	return protoTimestamp, nil
}

func (s *serverWrapper) handleCreateActivity() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		v := mux.Vars(r)
		recipient := v["username"]

		log.Printf("User %v received a create activity\n", recipient)

		decoder := json.NewDecoder(r.Body)
		var t createActivityStruct
		// map[string]interface{}
		jsonErr := decoder.Decode(&t)
		if jsonErr != nil {
			log.Printf("Invalid JSON\n")
			log.Printf("Error: %s\n", jsonErr)
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Invalid JSON\n")
			return
		}

		log.Printf("structure: %s\n", t)
		// https://www.w3.org/TR/json-ld-api/
		// proc := ld.NewJsonLdProcessor()
		// options := ld.NewJsonLdOptions("")
		flattenedDoc := t
		// flattenedDoc, err := proc.Flatten(t, nil, options)
		// https://stackoverflow.com/questions/23125144/unable-to-load-a-json-ld-remote-context
		// log.Printf("flattened structure: %s\n", flattenedDoc)

		protoTimestamp, parseErr := parseTimestamp(w, flattenedDoc.Object.Published)
		if parseErr != nil {
			log.Println(parseErr)
			return
		}

		nfa := &createpb.NewForeignArticle{
			AttributedTo: flattenedDoc.Object.AttributedTo,
			Content:      flattenedDoc.Object.Content,
			Published:		protoTimestamp,
			Recipient:		flattenedDoc.Recipient[0],
			Title:        flattenedDoc.Object.Name,
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		resp, err := s.create.ReceiveCreate(ctx, nfa)
		if err != nil || resp.ResultType == createpb.CreateResponse_ERROR {
			log.Printf("Could not receive create activity. Error: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Issue with receiving create activity\n")
			return
		}

		log.Printf("Activity was alright :+1:Received: %v\n", resp.Error)
		fmt.Fprintf(w, "Created blog with title\n")
	}
}

// handleNewUser sends an RPC to the database service to create a user with the
// given info.
func (s *serverWrapper) handleNewUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := r.URL.Query()
		// TODO(iandioch): Return error if parameters are missing.
		displayName := vars["display_name"][0]
		handle := vars["handle"][0]
		password := vars["password"][0]
		log.Printf("Trying to add new user %#v (%#v).\n", handle, displayName)
		u := &dbpb.UsersEntry{
			DisplayName: displayName,
			Handle:      handle,
			Password: password,
			Bio: "nothing",
		}
		ur := &dbpb.UsersRequest{
			Entry:       u,
			RequestType: dbpb.UsersRequest_INSERT,
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		resp, err := s.database.Users(ctx, ur)
		if err != nil {
			log.Fatalf("could not add new user: %v", err)
		}
		fmt.Fprintf(w, "Received: %#v\n", resp.Error)
		// TODO(iandioch): Return JSON with response status or error.
	}
}

// setupRoutes specifies the routing of all endpoints on the server.
// Centralised routing config allows easier debugging of a specific endpoint,
// as the code handling it can be looked up here.
// The server uses mux for routing. See instructions and examples for mux at
// https://www.gorillatoolkit.org/pkg/mux .
// TODO(iandioch): Move setupRoutes() to its own file if/when it gets too big.
func (s *serverWrapper) setupRoutes() {
	const (
		assetPath = "/assets/"
	)
	log.Printf("Setting up routes on skinny server.\n")

	r := s.router
	fs := http.StripPrefix(assetPath, http.FileServer(http.Dir(staticAssets)))

	r.PathPrefix(assetPath).Handler(fs)

	// User-facing routes
	r.HandleFunc("/", s.handleIndex())

	// c2s routes
	r.HandleFunc("/c2s/create_article", s.handleCreateArticle())
	r.HandleFunc("/c2s/feed", s.handleFeed())
	r.HandleFunc("/c2s/feed/{username}", s.handleFeed())
	r.HandleFunc("/c2s/@{username}", s.handleFeedPerUser())
	r.HandleFunc("/c2s/follow", s.handleFollow())
	r.HandleFunc("/c2s/new_user", s.handleNewUser())

	// ActivityPub routes
	r.HandleFunc("/ap/", s.handleNotImplemented())
	r.HandleFunc("/ap/@{username}/inbox", s.handleCreateActivity())
}

func (s *serverWrapper) shutdown() {
	log.Printf("Stopping skinny server.\n")
	ctx, cancel := context.WithTimeout(context.Background(), s.shutdownWait)
	defer cancel()
	// Waits for active connections to terminate, or until it hits the timeout.
	s.server.Shutdown(ctx)

	s.databaseConn.Close()
	s.articleConn.Close()
	s.followsConn.Close()
	s.createConn.Close()
	s.feedConn.Close()
}

func createArticleClient() (*grpc.ClientConn, articlepb.ArticleClient) {
	host := os.Getenv("ARTICLE_SERVICE_HOST")
	if host == "" {
		log.Fatal("ARTICLE_SERVICE_HOST env var not set for skinny server.")
	}
	addr := host + ":1601"

	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Skinny server did not connect to Article: %v", err)
	}
	return conn, articlepb.NewArticleClient(conn)
}

func createCreateClient() (*grpc.ClientConn, createpb.CreateClient) {
	host := os.Getenv("CREATE_SERVICE_HOST")
	if host == "" {
		log.Fatal("CREATE_SERVICE_HOST env var not set for skinny server.")
	}
	addr := host + ":1922"

	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Skinny server did not connect to Create: %v", err)
	}
	return conn, createpb.NewCreateClient(conn)
}

func createDatabaseClient() (*grpc.ClientConn, dbpb.DatabaseClient) {
	host := os.Getenv("DB_SERVICE_HOST")
	if host == "" {
		log.Fatal("DB_SERVICE_HOST env var not set for skinny server.")
	}
	addr := host + ":1798"

	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Skinny server did not connect: %v", err)
	}
	client := dbpb.NewDatabaseClient(conn)
	return conn, client
}

func createFollowsClient() (*grpc.ClientConn, followspb.FollowsClient) {
	host := os.Getenv("FOLLOWS_SERVICE_HOST")
	if host == "" {
		log.Fatal("FOLLOWS_SERVICE_HOST env var not set for skinny server.")
	}
	addr := host + ":1641"

	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Skinny server did not connect: %v", err)
	}
	client := followspb.NewFollowsClient(conn)
	return conn, client
}

func createFeedClient() (*grpc.ClientConn, feedpb.FeedClient) {
	const env = "FEED_SERVICE_HOST"
	host := os.Getenv(env)
	if host == "" {
		log.Fatalf("%s env var not set for skinny server", env)
	}
	addr := host + ":2012"

	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Skinny server could not connect to %s: %v", addr, err)
	}
	client := feedpb.NewFeedClient(conn)
	return conn, client
}

// buildServerWrapper sets up all necessary individual parts of the server
// wrapper, and returns one that is ready to run.
func buildServerWrapper() *serverWrapper {
	r := mux.NewRouter()
	env := "SKINNY_SERVER_PORT"
	port := os.Getenv(env)
	if port == "" {
		log.Fatalf("%s env var not set for skinny server", env)
	}
	addr := "0.0.0.0:" + port
	srv := &http.Server{
		Addr: addr,
		// Important to specify timeouts in order to prevent Slowloris attacks.
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      r,
	}
	databaseConn, databaseClient := createDatabaseClient()
	followsConn, followsClient := createFollowsClient()
	articleConn, articleClient := createArticleClient()
	feedConn, feedClient := createFeedClient()
	createConn, createClient := createCreateClient()
	s := &serverWrapper{
		router:       r,
		server:       srv,
		shutdownWait: 20 * time.Second,
		databaseConn: databaseConn,
		database:     databaseClient,
		articleConn:  articleConn,
		article:      articleClient,
		followsConn:  followsConn,
		follows:      followsClient,
		feedConn:     feedConn,
		feed:         feedClient,
		createConn:   createConn,
		create:       createClient,
	}
	s.setupRoutes()
	return s
}

func main() {
	log.Printf("Starting skinny server.\n")
	s := buildServerWrapper()

	// The following code is partially taken from this link:
	// https://github.com/gorilla/mux#graceful-shutdown
	// It lays out a way for a server to gracefully shutdown when a SIGINT
	// (Ctrl+C) or SIGTERM (kill) is received.
	// See also: https://gobyexample.com/signals

	go func() {
		if err := s.server.ListenAndServe(); err != nil {
			log.Println(err)
		}
	}()

	// Accept graceful shutdowns when quit via SIGINT or SIGTERM. Other signals
	// (eg. SIGKILL, SIGQUIT) will not be caught.
	// Docker sends a SIGTERM on shutdown.
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	// Block until we receive signal.
	<-c
	s.shutdown()
	os.Exit(0)
}
