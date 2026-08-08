package httpserver

import (
	"context"
	"net/http"

	"github.com/mohan9182/financer/api"
)

// --- user settings ---

type reqProfile struct {
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarUrl string `json:"avatarUrl"`
}

type reqPassword struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

type wireProfile struct {
	UserId    string `json:"userId"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarUrl string `json:"avatarUrl"`
}

func profileWire(p *api.ProfileResponse) wireProfile {
	return wireProfile{UserId: p.UserId, Name: p.Name, Email: p.Email, AvatarUrl: p.AvatarUrl}
}

func (a *API) getProfile(ctx context.Context, uid string, _ *http.Request) (any, error) {
	out, err := a.userSvc.GetProfile(ctx, uid)
	if err != nil {
		return nil, err
	}
	return profileWire(out), nil
}

func (a *API) updateProfile(ctx context.Context, uid string, r *http.Request) (any, error) {
	var in reqProfile
	if err := decodeBody(r.Body, &in); err != nil {
		return nil, err
	}
	out, err := a.userSvc.UpdateProfile(ctx, uid, &api.UpdateProfileRequest{
		Name: in.Name, Email: in.Email, AvatarUrl: in.AvatarUrl,
	})
	if err != nil {
		return nil, err
	}
	return profileWire(out), nil
}

func (a *API) changePassword(ctx context.Context, uid string, r *http.Request) (any, error) {
	var in reqPassword
	if err := decodeBody(r.Body, &in); err != nil {
		return nil, err
	}
	out, err := a.userSvc.ChangePassword(ctx, uid, &api.ChangePasswordRequest{
		CurrentPassword: in.CurrentPassword, NewPassword: in.NewPassword,
	})
	if err != nil {
		return nil, err
	}
	return authWire(out), nil
}

func (a *API) logoutAll(ctx context.Context, uid string, _ *http.Request) (any, error) {
	out, err := a.userSvc.LogoutAll(ctx, uid)
	if err != nil {
		return nil, err
	}
	return bulkWire(&api.BulkOperationResponse{Success: out.Success, Message: out.Message}), nil
}

func (a *API) uploadAvatar(ctx context.Context, uid string, r *http.Request) (any, error) {
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		return nil, bad("avatar must be a multipart upload")
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		return nil, bad("missing 'file' field")
	}
	defer file.Close()
	out, err := a.userSvc.SaveAvatar(ctx, uid, file)
	if err != nil {
		return nil, err
	}
	return created(profileWire(out)), nil
}
