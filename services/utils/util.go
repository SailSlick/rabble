package util

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	pb "github.com/cpssd/rabble/services/proto"
	"github.com/golang/protobuf/ptypes"
	// gpb "google.golang.org/protobuf/proto"
	tspb "github.com/golang/protobuf/ptypes/timestamp"
	"google.golang.org/grpc"
)

const (
	defaultImage     = "https://upload.wikimedia.org/wikipedia/commons/8/89/Portrait_Placeholder.png"
	MaxItemsReturned = 50
	timeParseFormat  = "2006-01-02T15:04:05.000Z"
)

// Notable error types
var (
	UserNotFoundErr = errors.New("GetAuthorFromDb: user not found")
)

// ParseUsername takes a fully qualified username and returns the username and
// host seperatly.
//
// Invalid urls return a non-nil error.
//
// For example:
//   "admin@http://r.dev" returns ("admin", "r.dev", nil)
//   "admin@rabble.dev" returns ("admin", "rabble.dev", nil)
//   "@admin" returns ("admin", "", nil)
//   "admin@foo@bar" returns ("", "", error)
func ParseUsername(fqu string) (string, string, error) {
	handleError := func() error {
		e := fmt.Sprintf("Couldn't parse username %s", fqu)
		log.Println(e)
		return errors.New(e)
	}

	fqu = strings.TrimLeft(fqu, "@")
	split := strings.Split(fqu, "@")

	if len(split) == 1 {
		if split[0] == "" {
			return "", "", handleError()
		}

		// Local user
		return split[0], "", nil
	}

	if len(split) != 2 {
		return "", "", handleError()
	}

	host := split[1]
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimPrefix(host, "https://")

	if split[0] == "" || host == "" {
		return "", "", handleError()
	}

	return split[0], host, nil
}
//
// func GeneralExternalErrorHandler(msg gpb.Message, err error, resp gpb.Message, w http.ResponseWriter) (gpb.Message) {
// 	if err != nil {
// 		log.Printf("Got error: %v", err)
// 		resp.Error = "Error communicating with rss service"
// 		w.WriteHeader(http.StatusInternalServerError)
// 		return resp
// 	} else if resp.ResultType == pb.ResultType_ERROR {
// 		log.Printf("Error updating user feed: %s", resp.Error)
// 		w.WriteHeader(http.StatusInternalServerError)
// 		return resp
// 	} else if resp.ResultType == pb.ResultType_ERROR_400 {
// 		log.Printf("Error updating user feed: %s", resp.Error)
// 		w.WriteHeader(http.StatusBadRequest)
// 		resp.Error = invalidRequestError
// 		return resp
// 	}
// 	return nil
// }

// NormaliseHost takes a hostname like "jimjim.dev" and it returns
// https://jimjim.dev.
//
// If the host is a development host, like skinny_1:1916, it returns http://skinny_1:1916
func NormaliseHost(host string) string {
	if host == "" {
		return ""
	}
	if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		return host
	}
	if !strings.Contains(host, ".") {
		return "http://" + host
	}
	return "https://" + host
}

// ConvertPbTimestamp converts a timestamp into a format readable by the frontend
func ConvertPbTimestamp(t *tspb.Timestamp) string {
	goTime, err := ptypes.Timestamp(t)
	if err != nil {
		log.Print(err)
		return time.Now().Format(timeParseFormat)
	}
	return goTime.Format(timeParseFormat)
}

// SplitTags converts a string of tags separated by | into a string array
func SplitTags(tags string) []string {
	var cleanTags []string
	splitTags := strings.Split(tags, "|")
	for _, tag := range splitTags {
		// -1 stand for replace all strings.
		if tag != "" {
			cleanTags = append(cleanTags, strings.Replace(tag, "%7C", "|", -1))
		}
	}
	return cleanTags
}

type UsersGetter interface {
	Users(ctx context.Context, in *pb.UsersRequest, opts ...grpc.CallOption) (*pb.UsersResponse, error)
}

func GetAuthorFromDb(ctx context.Context, handle string, host string, hostIsNull bool, globalId int64, db UsersGetter) (*pb.UsersEntry, error) {
	r := &pb.UsersRequest{
		RequestType: pb.RequestType_FIND,
		Match: &pb.UsersEntry{
			Handle:     handle,
			Host:       host,
			HostIsNull: hostIsNull,
			GlobalId:   globalId,
		},
	}
	return UserFindOne(ctx, r, db)
}

func UserFindOne(ctx context.Context, ufr *pb.UsersRequest, db UsersGetter) (*pb.UsersEntry, error) {
	results, err := UserFind(ctx, ufr, db)
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, UserNotFoundErr
	} else if len(results) > 1 {
		return nil, fmt.Errorf("Expected 1 user for GetAuthorFromDb request, got %d",
			len(results))
	}
	return results[0], nil
}

func UserFind(ctx context.Context, ufr *pb.UsersRequest, db UsersGetter) ([]*pb.UsersEntry, error) {
	const errFmt = "Could not find user %v@%v. error: %v"

	resp, err := db.Users(ctx, ufr)
	if err != nil {
		return nil, fmt.Errorf(errFmt, ufr.Match.Handle, ufr.Match.Host, err)
	}

	if resp.ResultType != pb.ResultType_OK {
		return nil, fmt.Errorf(errFmt, ufr.Match.Handle, ufr.Match.Host, resp.Error)
	}
	return resp.Results, nil
}

// convertDBToFeed converts PostsResponses to PostsEntry[]
// Hopefully this will removed once we fix proto building.
func ConvertDBToFeed(ctx context.Context, p *pb.PostsResponse, db UsersGetter) []*pb.Post {
	pe := []*pb.Post{}
	for i, r := range p.Results {
		if i >= MaxItemsReturned {
			// Have hit limit for number of items returned for this request.
			break
		}

		// TODO(iandioch): Find a way to avoid or cache these requests.
		author, err := GetAuthorFromDb(ctx, "", "", false, r.AuthorId, db)
		if err != nil {
			log.Println(err)
			continue
		}
		tags := SplitTags(r.Tags)
		np := &pb.Post{
			GlobalId:      r.GlobalId,
			Author:        author.Handle,
			AuthorDisplay: author.DisplayName,
			AuthorHost:    author.Host,
			AuthorId:      r.AuthorId,
			Title:         r.Title,
			Bio:           author.Bio,
			Body:          r.Body,
			Image:         defaultImage,
			LikesCount:    r.LikesCount,
			MdBody:        r.MdBody,
			IsLiked:       r.IsLiked,
			Published:     ConvertPbTimestamp(r.CreationDatetime),
			IsFollowed:    r.IsFollowed,
			IsShared:      r.IsShared,
			SharesCount:   r.SharesCount,
			Tags:          tags,
			Summary:       r.Summary,
		}
		pe = append(pe, np)
	}
	return pe
}

// ConvertShareToFeed converts SharesResponses to feed.proto Share
func ConvertShareToFeed(ctx context.Context, p *pb.SharesResponse, db UsersGetter) []*pb.Share {
	pe := []*pb.Share{}
	for i, r := range p.Results {
		if i >= MaxItemsReturned {
			// Have hit limit for number of items returned for this request.
			break
		}

		// TODO(iandioch): Find a way to avoid or cache these requests.
		author, err := GetAuthorFromDb(ctx, "", "", false, r.AuthorId, db)
		if err != nil {
			log.Println(err)
			continue
		}
		// TODO(iandioch): Find a way to avoid or cache these requests.
		sharer, err := GetAuthorFromDb(ctx, "", "", false, r.SharerId, db)
		if err != nil {
			log.Println(err)
			continue
		}
		tags := SplitTags(r.Tags)
		np := &pb.Share{
			GlobalId:      r.GlobalId,
			Author:        author.Handle,
			AuthorDisplay: author.DisplayName,
			AuthorHost:    author.Host,
			Title:         r.Title,
			Bio:           author.Bio,
			Body:          r.Body,
			Image:         defaultImage,
			LikesCount:    r.LikesCount,
			IsLiked:       r.IsLiked,
			Published:     ConvertPbTimestamp(r.CreationDatetime),
			IsFollowed:    r.IsFollowed,
			IsShared:      r.IsShared,
			SharerBio:     sharer.Bio,
			Sharer:        sharer.Handle,
			SharerHost:    sharer.Host,
			ShareDatetime: ConvertPbTimestamp(r.AnnounceDatetime),
			AuthorId:      author.GlobalId,
			SharesCount:   r.SharesCount,
			Tags:          tags,
			Summary:       r.Summary,
		}
		pe = append(pe, np)
	}
	return pe
}

func StripUser(p *pb.UsersEntry) *pb.User {
	return &pb.User{
		Handle:      p.Handle,
		Host:        p.Host,
		GlobalId:    p.GlobalId,
		Bio:         p.Bio,
		IsFollowed:  p.IsFollowed,
		DisplayName: p.DisplayName,
		Private:     p.Private,
		CustomCss:   p.CustomCss,
	}
}

// ConvertDBToUsers converts database.UserResponses to search.Users[]
func ConvertDBToUsers(ctx context.Context, p *pb.UsersResponse, db UsersGetter) []*pb.User {
	ue := []*pb.User{}
	for i, r := range p.Results {

		if i >= MaxItemsReturned {
			// Have hit limit for number of items returned for this request.
			break
		}
		ue = append(ue, StripUser(r))
	}
	return ue
}

// Creates a connction to GRPC server
func GrpcConn(env string, port string) *grpc.ClientConn {
	host := os.Getenv(env)
	if host == "" {
		log.Fatalf("%s env var not set.", env)
	}
	addr := host + ":" + port
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Server could not connect to %s: %v", addr, err)
	}
	return conn
}
