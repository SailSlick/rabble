package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"path"
	"strconv"

	pb "github.com/cpssd/rabble/services/proto"
	util "github.com/cpssd/rabble/services/utils"
	wrapperpb "github.com/golang/protobuf/ptypes/wrappers"
	"github.com/gorilla/mux"
)

const (
	couldNotLoadProfilePic = "Could not load profile pic from request"
)

type loginStruct struct {
	Handle   string `json:"handle"`
	Password string `json:"password"`
}

type registerRequest struct {
	Handle      string `json:"handle"`
	Password    string `json:"password"`
	DisplayName string `json:"displayName"`
	Bio         string `json:"bio"`
}

type userResponse struct {
	Error   string `json:"error"`
	Success bool   `json:"success"`
	UserID  int64  `json:"user_id"`
}

func (s *serverWrapper) getSessionHandle(r *http.Request) (string, error) {
	session, err := s.store.Get(r, "rabble-session")
	if err != nil {
		log.Printf("Error getting session: %v", err)
		return "", err
	}
	if _, ok := session.Values["handle"]; !ok {
		return "", fmt.Errorf("Handle doesn't exist, user not logged in")
	}
	return session.Values["handle"].(string), nil
}

func (s *serverWrapper) getSessionGlobalID(r *http.Request) (int64, error) {
	session, err := s.store.Get(r, "rabble-session")
	if err != nil {
		log.Printf("Error getting session: %v", err)
		return 0, err
	}
	if _, ok := session.Values["global_id"]; !ok {
		return 0, fmt.Errorf("Global ID doesn't exist, user not logged in")
	}
	return session.Values["global_id"].(int64), nil
}

// handleLogin sends an RPC to the users service to check if a login
// is correct.
func (s *serverWrapper) handleLogin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		var t loginStruct
		var jsonResp userResponse
		jsonResp.Success = false
		enc := json.NewEncoder(w)

		err := decoder.Decode(&t)
		if err != nil {
			log.Println(invalidJSONError)
			log.Printf("Error: %s\n", err)
			jsonResp.Error = invalidJSONError
			w.WriteHeader(http.StatusBadRequest)
			enc.Encode(jsonResp)
			return
		}
		lr := &pb.LoginRequest{
			Handle:   t.Handle,
			Password: t.Password,
		}
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutDuration)
		defer cancel()

		resp, err := s.users.Login(ctx, lr)
		if err != nil {
			log.Println(err)
			jsonResp.Error = "Issue with handling login request"
			w.WriteHeader(http.StatusInternalServerError)
			enc.Encode(jsonResp)
			return
		}
		if resp.Result == pb.ResultType_OK {
			session, err := s.store.Get(r, "rabble-session")
			if err != nil {
				log.Println(err)
				jsonResp.Error = "Issue with handling login request"
				w.WriteHeader(http.StatusInternalServerError)
				enc.Encode(jsonResp)
				return
			}
			log.Printf("User %d login success: %t", resp.GlobalId, jsonResp.Success)
			jsonResp.Success = true
			jsonResp.UserID = resp.GlobalId

			session.Values["handle"] = t.Handle
			session.Values["global_id"] = resp.GlobalId
			session.Values["display_name"] = resp.DisplayName
			session.Save(r, w)
		} else if resp.Result == pb.ResultType_ERROR_401 {
			w.WriteHeader(http.StatusUnauthorized)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}

		w.Header().Set("Content-Type", "application/json")
		// Intentionally not revealing to the user if an error occurred.
		enc.Encode(jsonResp)
	}
}

// Clears the user's session when called.
func (s *serverWrapper) handleLogout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, err := s.store.Get(r, "rabble-session")
		var jsonResp userResponse
		jsonResp.Success = false
		enc := json.NewEncoder(w)
		if err != nil {
			fmt.Println(err)
			jsonResp.Error = "Issue with handling logout request"
			w.WriteHeader(http.StatusInternalServerError)
			enc.Encode(jsonResp)
			return
		}
		session.Options.MaxAge = -1 // Marks the session for deletion.
		err = session.Save(r, w)
		if err != nil {
			fmt.Println(err)
			jsonResp.Error = "Issue with handling logout request"
			w.WriteHeader(http.StatusInternalServerError)
			enc.Encode(jsonResp)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		jsonResp.Success = true
		enc.Encode(jsonResp)
	}
}

// handleRegister sends an RPC to the users service to create a user with the
// given info.
func (s *serverWrapper) handleRegister() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		var req registerRequest
		var jsonResp userResponse
		jsonResp.Success = false
		err := decoder.Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		if err != nil {
			log.Printf(invalidJSONErrorWithPrint, err)
			w.WriteHeader(http.StatusBadRequest)
			jsonResp.Error = invalidJSONError
			enc.Encode(jsonResp)
			return
		}
		log.Printf("Trying to add new user %#v.\n", req.Handle)
		u := &pb.CreateUserRequest{
			DisplayName: req.DisplayName,
			Handle:      req.Handle,
			Password:    req.Password,
			Bio:         req.Bio,
		}
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutDuration)
		defer cancel()

		resp, err := s.users.Create(ctx, u)
		if err != nil {
			log.Printf("could not add new user: %v", err)
			jsonResp.Error = "Error communicating with create user service"
			w.WriteHeader(http.StatusInternalServerError)
		} else if resp.ResultType != pb.ResultType_OK {
			log.Printf("Error creating user: %s", resp.Error)
			jsonResp.Error = resp.Error
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			session, err := s.store.Get(r, "rabble-session")
			if err != nil {
				log.Printf("Error getting session store after create: %s", err)
				jsonResp.Error = "Issue with login after create\n"
				w.WriteHeader(http.StatusInternalServerError)
			} else {
				jsonResp.UserID = resp.GlobalId
				jsonResp.Success = true
				session.Values["handle"] = req.Handle
				session.Values["global_id"] = resp.GlobalId
				session.Values["display_name"] = req.DisplayName
				session.Save(r, w)
			}
		}
		enc.Encode(jsonResp)
	}
}

// handleUserUpdate sends an RPC to the users service to update a user with the
// given info.
func (s *serverWrapper) handleUserUpdate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		w.Header().Set("Content-Type", "application/json")

		var (
			req  pb.UpdateUserRequest
			resp userResponse
		)
		resp.Success = false

		enc := json.NewEncoder(w)

		handle, err := s.getSessionHandle(r)
		if err != nil {
			log.Printf("Call to update user by not logged in user")
			w.WriteHeader(http.StatusForbidden)
			resp.Error = invalidJSONError
			enc.Encode(resp)
			return
		}

		err = decoder.Decode(&req)
		if err != nil {
			log.Printf(invalidJSONErrorWithPrint, err)
			w.WriteHeader(http.StatusBadRequest)
			resp.Error = invalidJSONError
			enc.Encode(resp)
			return
		}

		// This makes the handle optional to send, since it's already
		// provided by the session handler.
		req.Handle = handle

		log.Printf("Trying to update user %#v.\n", req.Handle)
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutDuration)
		defer cancel()

		updateResp, err := s.users.Update(ctx, &req)

		if err != nil {
			log.Printf("Could not update user: %v", err)
			resp.Error = "Error communicating with user update service"
			w.WriteHeader(http.StatusInternalServerError)
		} else if updateResp.Result != pb.ResultType_OK {
			// Unlike in user response, we will be clear that they
			// provided an incorrect password.
			log.Printf("Error updating user: %s", resp.Error)
			resp.Error = updateResp.Error
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			log.Print("Update session display_name if it changed")
			resp.Success = true
		}

		enc.Encode(resp)
	}
}

// handleUserUpdate sends an RPC to the users service to update a user with the
// given info.
func (s *serverWrapper) handleFeedUserVerification() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		w.Header().Set("Content-Type", "application/json")

		var (
			req  pb.UpdateUserRequest
			resp userResponse
		)
		resp.Success = false

		enc := json.NewEncoder(w)

		handle, err := s.getSessionHandle(r)
		if err != nil {
			log.Printf("Call to update user by not logged in user")
			w.WriteHeader(http.StatusForbidden)
			resp.Error = invalidJSONError
			enc.Encode(resp)
			return
		}

		err = decoder.Decode(&req)
		if err != nil {
			log.Printf(invalidJSONErrorWithPrint, err)
			w.WriteHeader(http.StatusBadRequest)
			resp.Error = invalidJSONError
			enc.Encode(resp)
			return
		}

		// This makes the handle optional to send, since it's already
		// provided by the session handler.
		req.Handle = handle

		log.Printf("Trying to update user %#v.\n", req.Handle)
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutDuration)
		defer cancel()

		updateResp, err := s.users.Update(ctx, &req)

		if err != nil {
			log.Printf("Could not update user: %v", err)
			resp.Error = "Error communicating with user update service"
			w.WriteHeader(http.StatusInternalServerError)
		} else if updateResp.Result != pb.ResultType_OK {
			// Unlike in user response, we will be clear that they
			// provided an incorrect password.
			log.Printf("Error updating user: %s", resp.Error)
			resp.Error = updateResp.Error
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			log.Print("Update session display_name if it changed")
			resp.Success = true
		}

		enc.Encode(resp)
	}
}

func (s *serverWrapper) getProfilePicPath(userID int64) string {
	filename := fmt.Sprintf("user_%d", userID)
	filepath := path.Join(staticAssets, filename)
	return filepath
}

func (s *serverWrapper) handleUserUpdateProfilePic() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var resp userResponse
		resp.Success = false
		enc := json.NewEncoder(w)
		w.Header().Set("Content-Type", "application/json")

		userID, err := s.getSessionGlobalID(r)
		if err != nil {
			log.Printf("Call to update user by not logged in user")
			w.WriteHeader(http.StatusForbidden)
			resp.Error = invalidJSONError
			enc.Encode(resp)
			return
		}

		image, _, err := r.FormFile("profile_pic")
		defer image.Close()
		if err != nil {
			log.Printf(couldNotLoadProfilePic+": %v", err)
			w.WriteHeader(http.StatusBadRequest)
			resp.Error = couldNotLoadProfilePic
			enc.Encode(resp)
			return
		}
		buf := bytes.NewBuffer(nil)
		if _, err := io.Copy(buf, image); err != nil {
			log.Printf("Error copying image to buffer: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			resp.Error = couldNotLoadProfilePic
			enc.Encode(resp)
			return
		}
		allowedTypes := []string{
			"image/gif",
			"image/jpeg",
			"image/png",
			"image/webp",
		}
		detectedType := http.DetectContentType(buf.Bytes())
		found := false
		for _, t := range allowedTypes {
			if detectedType == t {
				found = true
				break
			}
		}
		if !found {
			log.Printf("Type dissallowed: %v", detectedType)
			w.WriteHeader(http.StatusBadRequest)
			resp.Error = "Upload is not of an allowed type"
			enc.Encode(resp)
			return
		}
		filepath := s.getProfilePicPath(userID)
		log.Printf("Writing image to %s", filepath)
		if err := ioutil.WriteFile(filepath, buf.Bytes(), 0644); err != nil {
			log.Printf("Error writing file to %s: %v", filepath, err)
			w.WriteHeader(http.StatusBadRequest)
			resp.Error = "Error writing image to disk"
			enc.Encode(resp)
			return
		}
		resp.Success = true
		enc.Encode(resp)
	}
}

func (s *serverWrapper) handleUserCSS() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		v := mux.Vars(r)
		strUserID, ok := v["userId"]
		if !ok || strUserID == "" {
			log.Printf("Could not parse userId from url in UserCss\n")
			w.WriteHeader(http.StatusBadRequest) // Bad Request.
			return
		}
		userID, err := strconv.ParseInt(strUserID, 10, 64)
		if err != nil {
			log.Printf("Could not convert userId to int64: id(%v)\n", strUserID)
			w.WriteHeader(http.StatusBadRequest) // Bad Request.
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutDuration)
		defer cancel()
		resp, err := s.users.GetCss(ctx, &pb.GetCssRequest{
			UserId: userID,
		})
		if err != nil {
			log.Printf("Error in users.GetCss: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		} else if resp.Result != pb.ResultType_OK {
			log.Printf("Error getting css: %s", resp.Error)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/css")
		fmt.Fprintf(w, resp.Css)
		return
	}
}

func (s *serverWrapper) handleUserDetails() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		v := mux.Vars(r)
		username, ok := v["username"]
		if !ok || username == "" {
			w.WriteHeader(http.StatusBadRequest) // Bad Request.
			return
		}

		handle, host, err := util.ParseUsername(username)
		if err != nil {
			log.Printf("got bad username: %s", username)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		ur := &pb.UsersRequest{
			RequestType: pb.UsersRequest_FIND,
			Match: &pb.UsersEntry{
				Handle:     handle,
				Host:       util.NormaliseHost(host),
				HostIsNull: host == "",
			},
		}

		// uid = 0 if no user is logged in.
		if uid, _ := s.getSessionGlobalID(r); uid != 0 {
			ur.UserGlobalId = &wrapperpb.Int64Value{Value: uid}
		}

		log.Printf("Sending request for user: @%s@%s", ur.Match.Handle, ur.Match.Host)

		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutDuration)
		defer cancel()
		resp, err := s.database.Users(ctx, ur)
		if err != nil {
			log.Printf("could not get user, error: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		} else if len(resp.Results) != 1 {
			log.Printf("could not get user, got %v results", len(resp.Results))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		err = enc.Encode(util.StripUser(resp.Results[0]))
		if err != nil {
			log.Printf("could not marshal blogs: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}
