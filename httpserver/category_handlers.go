package httpserver

import (
	"context"
	"net/http"

	"github.com/mohan9182/financer/api"
)

// --- categories ---

type reqCategory struct {
	Id   string   `json:"id"`
	Name string   `json:"name"`
	Ids  []string `json:"ids"`
	Bulk []struct {
		Id   string `json:"id"`
		Name string `json:"name"`
	} `json:"categories"`
}

func (a *API) listCategories(ctx context.Context, _ string, r *http.Request) (any, error) {
	q := r.URL.Query()
	out, err := a.category.ListCategories(ctx, &api.ListCategoriesRequest{PageSize: queryInt(q, "pageSize"), PageToken: q.Get("pageToken")})
	if err != nil {
		return nil, err
	}
	items := make([]wireCategory, 0, len(out.Categories))
	for _, c := range out.Categories {
		items = append(items, categoryWire(c))
	}
	return struct {
		Categories    []wireCategory `json:"categories"`
		NextPageToken string         `json:"nextPageToken"`
	}{Categories: items, NextPageToken: out.NextPageToken}, nil
}

func (a *API) createCategories(ctx context.Context, _ string, r *http.Request) (any, error) {
	var in reqCategory
	if err := decodeBody(r.Body, &in); err != nil {
		return nil, err
	}
	var cats []*api.CreateCategoryRequest
	for _, c := range in.Bulk {
		cats = append(cats, &api.CreateCategoryRequest{Name: c.Name})
	}
	out, err := a.category.CreateCategories(ctx, &api.CreateCategoriesRequest{Categories: cats})
	if err != nil {
		return nil, err
	}
	return bulkWire(out), nil
}

func (a *API) updateCategories(ctx context.Context, _ string, r *http.Request) (any, error) {
	var in reqCategory
	if err := decodeBody(r.Body, &in); err != nil {
		return nil, err
	}
	var cats []*api.UpdateCategoryRequest
	for _, c := range in.Bulk {
		cats = append(cats, &api.UpdateCategoryRequest{Id: c.Id, Name: c.Name})
	}
	out, err := a.category.UpdateCategories(ctx, &api.UpdateCategoriesRequest{Categories: cats})
	if err != nil {
		return nil, err
	}
	return bulkWire(out), nil
}

func (a *API) deleteCategories(ctx context.Context, _ string, r *http.Request) (any, error) {
	var in reqCategory
	if err := decodeBody(r.Body, &in); err != nil {
		return nil, err
	}
	out, err := a.category.DeleteCategories(ctx, &api.DeleteCategoriesRequest{Ids: in.Ids})
	if err != nil {
		return nil, err
	}
	return bulkWire(out), nil
}
