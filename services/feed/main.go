package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"google.golang.org/grpc"

	dbpb "github.com/cpssd/rabble/services/database/proto"
	pb "github.com/cpssd/rabble/services/feed/proto"
)

// convertDBToFeed converts PostsResponses to FeedResponses.
// Hopefully this will removed once we fix proto building.
func (s *server) convertDBToFeed(ctx context.Context, p *dbpb.PostsResponse) *pb.FeedResponse {
	fp := &pb.FeedResponse{}
	for _, r := range p.Results {
		// TODO(iandioch): Find a way to avoid or cache these requests.
		author, err := s.getAuthorFromDb(ctx, "", "", r.AuthorId)
		if err != nil {
			// Error has already been logged.
			continue
		}
		np := &pb.Post{
			GlobalId: r.GlobalId,
			// TODO(iandioch): Consider what happens for foreign users.
			Author:           author.Handle,
			Title:            r.Title,
			Body:             r.Body,
			CreationDatetime: r.CreationDatetime,
		}
		fp.Results = append(fp.Results, np)
	}
	return fp
}

func (s *server) getAuthorFromDb(ctx context.Context, handle string, host string, globalId int64) (*dbpb.UsersEntry, error) {
	r := &dbpb.UsersRequest{
		RequestType: dbpb.UsersRequest_FIND,
		Match: &dbpb.UsersEntry{
			Handle:   handle,
			Host:     host,
			GlobalId: globalId,
		},
	}

	resp, err := s.db.Users(ctx, r)
	if err != nil {
		return nil, fmt.Errorf("Could not find user %v@%v. error: %v",
			handle, host, err)
	}
	return resp.Results[0], nil
}

type server struct {
	db dbpb.DatabaseClient
}

func (s *server) Get(ctx context.Context, r *pb.FeedRequest) (*pb.FeedResponse, error) {
	pr := &dbpb.PostsRequest{
		RequestType: dbpb.PostsRequest_FIND,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	resp, err := s.db.Posts(ctx, pr)
	if err != nil {
		return nil, fmt.Errorf("feed.Get failed: db.Posts(%v) error: %v", *pr, err)
	}

	return s.convertDBToFeed(ctx, resp), nil
}

func (s *server) PerUser(ctx context.Context, r *pb.FeedRequest) (*pb.FeedResponse, error) {
	if r.Username == "" {
		return nil, fmt.Errorf("feed.PerUser failed: username field empty")
	}

	author, err := s.getAuthorFromDb(ctx, r.Username, "", 0)
	if err != nil {
		return nil, err
	}
	authorId := author.GlobalId

	pr := &dbpb.PostsRequest{
		RequestType: dbpb.PostsRequest_FIND,
		Match: &dbpb.PostsEntry{
			AuthorId: authorId,
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resp, err := s.db.Posts(ctx, pr)
	if err != nil {
		return nil, fmt.Errorf("feed.PerUser failed: db.Posts(%v) error: %v", *pr, err)
	}
	return s.convertDBToFeed(ctx, resp), nil
}

func newServer(c *grpc.ClientConn) *server {
	db := dbpb.NewDatabaseClient(c)
	return &server{db: db}
}

func main() {
	log.Print("Starting feed")
	lis, err := net.Listen("tcp", ":2012")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	host := os.Getenv("DB_SERVICE_HOST")
	if host == "" {
		log.Fatal("DB_SERVICE_HOST env var not set for skinny server.")
	}
	addr := host + ":1798"

	c, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Could not connect to database: %v", err)
	}
	defer c.Close()

	grpcSrv := grpc.NewServer()
	pb.RegisterFeedServer(grpcSrv, newServer(c))
	grpcSrv.Serve(lis)
}
