package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	pb "github.com/cpssd/rabble/services/proto"
	utils "github.com/cpssd/rabble/services/utils"
	"github.com/golang/protobuf/ptypes"
	tspb "github.com/golang/protobuf/ptypes/timestamp"
	wrappers "github.com/golang/protobuf/ptypes/wrappers"
	"github.com/mmcdole/gofeed"
	"google.golang.org/grpc"
)

const (
	findUserErrorFmt      = "ERROR: User id(%v) find failed. message: %v\n"
	findUserPostsErrorFmt = "ERROR: User id(%v) posts find failed. message: %v\n"
	rssTimeParseFormat    = "Mon, 02 Jan 2006 15:04:05 -0700"
	rssDeclare            = `<?xml version="1.0" encoding="UTF-8" ?><rss version="2.0"><channel>`
	rssDeclareEnd         = `</channel></rss>`
	letterBytes           = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

type Parser interface {
	ParseURL(string) (*gofeed.Feed, error)
}

type serverWrapper struct {
	dbConn     *grpc.ClientConn
	db         pb.DatabaseClient
	artConn    *grpc.ClientConn
	art        pb.ArticleClient
	feedParser Parser
	server     *grpc.Server
	hostname   string
}

// convertFeedItemDatetime converts gofeed.Item.Published type to protobuf timestamp
func (s *serverWrapper) convertFeedItemDatetime(gi *gofeed.Item) (*tspb.Timestamp, error) {
	parsedTimestamp := time.Now()
	if (gi.PublishedParsed != &time.Time{} && gi.PublishedParsed != nil) {
		// TODO (sailsick): Add log levels for golang
		//log.Printf("No timestamp for feed: %s\n", gi.Link)
		parsedTimestamp = *gi.PublishedParsed
	}

	return s.convertToProtoTimestamp(parsedTimestamp)
}

func (s *serverWrapper) convertToProtoTimestamp(timestamp time.Time) (*tspb.Timestamp, error) {
	protoTimestamp, protoTimeErr := ptypes.TimestampProto(timestamp)
	if protoTimeErr != nil {
		log.Printf("Error converting timestamp: %s\n", protoTimeErr)
		return nil, fmt.Errorf("Invalid timestamp")
	}
	return protoTimestamp, nil
}

func (s *serverWrapper) convertRssURLToHandle(url string) string {
	// Converts url in form: https://news.ycombinator.com/rss
	// to: news.ycombinator.com-rss
	if strings.HasPrefix(url, "http") {
		url = strings.Split(url, "//")[1]
	}
	return strings.Replace(url, "/", "-", -1)
}

func (s *serverWrapper) sendCreateArticle(ctx context.Context, authorID int64, title string, content string, cTime *tspb.Timestamp) {
	na := &pb.NewArticle{
		AuthorId:         authorID,
		Title:            title,
		Body:             content,
		CreationDatetime: cTime,
		Foreign:          false,
	}
	newArtResp, newArtErr := s.art.CreateNewArticle(ctx, na)
	if newArtErr != nil {
		log.Printf("ERROR: Could not create new article: %v", newArtErr)
	} else if newArtResp.ResultType != pb.ResultType_OK {
		log.Printf("ERROR: Could not create new article message: %v", newArtResp.Error)
	}
}

// createArticlesFromFeed converts gofeed.Feed types to article type.
func (s *serverWrapper) createArticlesFromFeed(ctx context.Context, gf *gofeed.Feed, authorID int64) {
	for _, r := range gf.Items {
		// convert time to creation_datetime
		creationTime, creationErr := s.convertFeedItemDatetime(r)
		if creationErr != nil {
			continue
		}
		content := r.Content
		if content == "" {
			content = r.Description
		}
		s.sendCreateArticle(ctx, authorID, r.Title, content, creationTime)
	}
}

func (s *serverWrapper) createRssHeader(ue *pb.UsersEntry) string {
	link := s.hostname + "/c2s/@" + ue.Handle
	datetime := time.Now().Format(rssTimeParseFormat)
	return "<title>Rabble blog for " + ue.Handle + "</title>\n" +
		"<description>" + ue.Bio + "</description>\n" +
		"<link>" + link + "</link>\n" +
		"<pubDate>" + datetime + "</pubDate>\n"
}

func (s *serverWrapper) createRssItem(ue *pb.UsersEntry, pe *pb.PostsEntry) string {
	link := s.hostname + "/c2s/@" + ue.Handle + "/" + strconv.FormatInt(pe.GlobalId, 10)
	timestamp, _ := ptypes.Timestamp(pe.CreationDatetime)
	datetime := timestamp.Format(rssTimeParseFormat)
	return "<item>\n" +
		"<title>" + pe.Title + "</title>\n" +
		"<link>" + link + "</link>\n" +
		"<description>" + pe.MdBody + "</description>\n" +
		"<pubDate>" + datetime + "</pubDate>\n" +
		"</item>\n"

}

func (s *serverWrapper) GetUser(ctx context.Context, globalID int64) (*pb.UsersEntry, error) {
	urFind := &pb.UsersRequest{
		RequestType: pb.RequestType_FIND,
		Match: &pb.UsersEntry{
			GlobalId: globalID,
		},
	}
	return utils.UserFindOne(ctx, urFind, s.db)
}

func (s *serverWrapper) GetUserPosts(ctx context.Context, authorID int64) ([]*pb.PostsEntry, error) {
	findReq := &pb.PostsRequest{
		RequestType: pb.RequestType_FIND,
		Match: &pb.PostsEntry{
			AuthorId: authorID,
		},
	}
	findResp, findErr := s.db.Posts(ctx, findReq)
	if findErr != nil {
		return nil, fmt.Errorf(findUserPostsErrorFmt, authorID, findErr)
	}
	if findResp.ResultType != pb.ResultType_OK {
		return nil, fmt.Errorf(findUserPostsErrorFmt, authorID, findResp.Error)
	}
	return findResp.Results, nil
}

func (s *serverWrapper) GetRssFeed(url string) (*gofeed.Feed, error) {
	feed, parseErr := s.feedParser.ParseURL(url)

	if parseErr != nil {
		return nil, fmt.Errorf("While getting rss feed `%s` got err: %v", url, parseErr)
	}

	return feed, nil
}

func (s *serverWrapper) PerUserRss(ctx context.Context, r *pb.UsersEntry) (*pb.RssResponse, error) {
	log.Printf("Got a per user request for user id: %v\n", r.GlobalId)
	rssr := &pb.RssResponse{}

	// Get user details
	ue, userErr := s.GetUser(ctx, r.GlobalId)
	if userErr != nil {
		log.Printf("PerUserRss user find got: %v\n", userErr.Error())
		rssr.ResultType = pb.ResultType_ERROR
		rssr.Message = userErr.Error()
		return rssr, nil
	}

	if ue.Private != nil && ue.Private.Value {
		log.Printf("id: %v is a private user.\n", r.GlobalId)
		rssr.ResultType = pb.ResultType_ERROR_401
		rssr.Message = "Can not create RSS feed for private user."
		return rssr, nil
	}

	// Get user posts
	posts, postFindErr := s.GetUserPosts(ctx, ue.GlobalId)
	if postFindErr != nil {
		log.Printf("PerUserRss posts find got: %v\n", postFindErr.Error())
		rssr.ResultType = pb.ResultType_ERROR
		rssr.Message = postFindErr.Error()
		return rssr, nil
	}

	// Construct rss header
	rssHeader := s.createRssHeader(ue)
	rssFeed := rssDeclare + rssHeader

	// Take most recent 10 posts
	sort.SliceStable(posts, func(i int, j int) bool {
		return posts[i].CreationDatetime.GetSeconds() < posts[j].CreationDatetime.GetSeconds()
	})
	n := 10
	if len(posts) < 10 {
		n = len(posts)
	}
	topTen := posts[:n]

	// Convert each post to rss entry
	for _, post := range topTen {
		// Add all rss entrys into body
		rssFeed += s.createRssItem(ue, post)
	}

	rssFeed += rssDeclareEnd
	rssr.ResultType = pb.ResultType_OK
	rssr.Feed = rssFeed

	return rssr, nil
}

func (s *serverWrapper) NewRssFollow(ctx context.Context, r *pb.NewRssFeed) (*pb.NewRssFeedResponse, error) {
	log.Printf("Got a new RSS follow for site: %s\n", r.RssUrl)
	rssr := &pb.NewRssFeedResponse{}

	feed, err := s.GetRssFeed(r.RssUrl)

	if err != nil {
		log.Println(err)
		rssr.ResultType = pb.ResultType_ERROR
		rssr.Message = err.Error()
		return rssr, nil
	}

	handle := s.convertRssURLToHandle(r.RssUrl)
	bio := "RSS/Atom feed from " + handle + " converted to a Rabble user"
	// add new user with feed details
	urInsert := &pb.UsersRequest{
		RequestType: pb.RequestType_INSERT,
		Entry: &pb.UsersEntry{
			Handle:     handle,
			Rss:        r.RssUrl,
			Bio:        bio,
			HostIsNull: true,
			Private:    &wrappers.BoolValue{Value: true},
		},
	}
	insertResp, insertErr := s.db.Users(ctx, urInsert)

	if insertErr != nil {
		log.Printf("Error on rss user insert: %v\n", insertErr)
		rssr.ResultType = pb.ResultType_ERROR
		rssr.Message = insertErr.Error()
		return rssr, nil
	}

	if insertResp.ResultType != pb.ResultType_OK {
		log.Printf("Rss user insert failed. message: %v\n", insertResp.Error)
		rssr.ResultType = pb.ResultType_ERROR
		rssr.Message = insertResp.Error
		return rssr, nil
	}

	// convert feed to post items and save
	s.createArticlesFromFeed(ctx, feed, insertResp.GlobalId)

	rssr.ResultType = pb.ResultType_OK
	rssr.GlobalId = insertResp.GlobalId

	return rssr, nil
}

func buildServerWrapper() *serverWrapper {
	hostname := os.Getenv("HOST_NAME")
	if hostname == "" {
		log.Fatal("HOST_NAME env var not set for rss service.")
	}

	dbConn := utils.GrpcConn("DB_SERVICE_HOST", "1798")
	dbClient := pb.NewDatabaseClient(dbConn)
	artConn := utils.GrpcConn("ARTICLE_SERVICE_HOST", "1601")
	artClient := pb.NewArticleClient(artConn)
	fp := gofeed.NewParser()
	grpcSrv := grpc.NewServer()

	return &serverWrapper{
		dbConn:     dbConn,
		db:         dbClient,
		artConn:    artConn,
		art:        artClient,
		feedParser: fp,
		server:     grpcSrv,
		hostname:   hostname,
	}
}

func main() {
	log.Print("Starting rss on port: 1973")
	lis, netErr := net.Listen("tcp", ":1973")
	if netErr != nil {
		log.Fatalf("failed to listen: %v", netErr)
	}

	serverWrapper := buildServerWrapper()
	pb.RegisterRSSServer(serverWrapper.server, serverWrapper)

	go func() {
		if serveErr := serverWrapper.server.Serve(lis); serveErr != nil {
			log.Fatalf("failed to serve: %v", serveErr)
		}
	}()

	rand.Seed(time.Now().UnixNano())
	scraperTicker := time.NewTicker(scraperInterval)

	for t := range scraperTicker.C {
		log.Print("Starting scraper, time: ", t.String())
		serverWrapper.runScraper()
	}

	// Accept graceful shutdowns when quit via SIGINT or SIGTERM. Other signals
	// (eg. SIGKILL, SIGQUIT) will not be caught.
	// Docker sends a SIGTERM on shutdown.
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	// Block until we receive signal.
	<-c
	scraperTicker.Stop()
	serverWrapper.server.Stop()
	serverWrapper.dbConn.Close()
	serverWrapper.artConn.Close()
	os.Exit(0)
}
