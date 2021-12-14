package krest

import "context"

// Mock mocks the krest.Provider interface with a configurable structure
type Mock struct {
	GetFn    func(ctx context.Context, url string, data RequestData) (resp Response, err error)
	PostFn   func(ctx context.Context, url string, data RequestData) (resp Response, err error)
	PutFn    func(ctx context.Context, url string, data RequestData) (resp Response, err error)
	PatchFn  func(ctx context.Context, url string, data RequestData) (resp Response, err error)
	DeleteFn func(ctx context.Context, url string, data RequestData) (resp Response, err error)
}

// Get mocks the krest.Provider.Get method
func (m Mock) Get(ctx context.Context, url string, data RequestData) (resp Response, err error) {
	if m.GetFn != nil {
		return m.GetFn(ctx, url, data)
	}
	return Response{}, nil
}

// Post mocks the krest.Provider.Post method
func (m Mock) Post(ctx context.Context, url string, data RequestData) (resp Response, err error) {
	if m.PostFn != nil {
		return m.PostFn(ctx, url, data)
	}
	return Response{}, nil
}

// Put mocks the krest.Provider.Put method
func (m Mock) Put(ctx context.Context, url string, data RequestData) (resp Response, err error) {
	if m.PutFn != nil {
		return m.PutFn(ctx, url, data)
	}
	return Response{}, nil
}

// Patch mocks the krest.Provider.Patch method
func (m Mock) Patch(ctx context.Context, url string, data RequestData) (resp Response, err error) {
	if m.PatchFn != nil {
		return m.PatchFn(ctx, url, data)
	}
	return Response{}, nil
}

// Delete mocks the krest.Provider.Delete method
func (m Mock) Delete(ctx context.Context, url string, data RequestData) (resp Response, err error) {
	if m.DeleteFn != nil {
		return m.DeleteFn(ctx, url, data)
	}
	return Response{}, nil
}
