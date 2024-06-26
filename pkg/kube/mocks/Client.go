// Code generated by mockery v2.32.4. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"

	v1alpha1 "github.com/SovereignCloudStack/cluster-stack-operator/api/v1alpha1"
)

// Client is an autogenerated mock type for the Client type
type Client struct {
	mock.Mock
}

// Apply provides a mock function with given fields: ctx, template, oldResources
func (_m *Client) Apply(ctx context.Context, template []byte, oldResources []*v1alpha1.Resource) ([]*v1alpha1.Resource, bool, error) {
	ret := _m.Called(ctx, template, oldResources)

	var r0 []*v1alpha1.Resource
	var r1 bool
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, []byte, []*v1alpha1.Resource) ([]*v1alpha1.Resource, bool, error)); ok {
		return rf(ctx, template, oldResources)
	}
	if rf, ok := ret.Get(0).(func(context.Context, []byte, []*v1alpha1.Resource) []*v1alpha1.Resource); ok {
		r0 = rf(ctx, template, oldResources)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*v1alpha1.Resource)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, []byte, []*v1alpha1.Resource) bool); ok {
		r1 = rf(ctx, template, oldResources)
	} else {
		r1 = ret.Get(1).(bool)
	}

	if rf, ok := ret.Get(2).(func(context.Context, []byte, []*v1alpha1.Resource) error); ok {
		r2 = rf(ctx, template, oldResources)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// ApplyNewClusterStack provides a mock function with given fields: ctx, oldTemplate, newTemplate
func (_m *Client) ApplyNewClusterStack(ctx context.Context, oldTemplate []byte, newTemplate []byte) ([]*v1alpha1.Resource, bool, error) {
	ret := _m.Called(ctx, oldTemplate, newTemplate)

	var r0 []*v1alpha1.Resource
	var r1 bool
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, []byte, []byte) ([]*v1alpha1.Resource, bool, error)); ok {
		return rf(ctx, oldTemplate, newTemplate)
	}
	if rf, ok := ret.Get(0).(func(context.Context, []byte, []byte) []*v1alpha1.Resource); ok {
		r0 = rf(ctx, oldTemplate, newTemplate)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*v1alpha1.Resource)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, []byte, []byte) bool); ok {
		r1 = rf(ctx, oldTemplate, newTemplate)
	} else {
		r1 = ret.Get(1).(bool)
	}

	if rf, ok := ret.Get(2).(func(context.Context, []byte, []byte) error); ok {
		r2 = rf(ctx, oldTemplate, newTemplate)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// Delete provides a mock function with given fields: ctx, template, oldResources
func (_m *Client) Delete(ctx context.Context, template []byte, oldResources []*v1alpha1.Resource) ([]*v1alpha1.Resource, bool, error) {
	ret := _m.Called(ctx, template, oldResources)

	var r0 []*v1alpha1.Resource
	var r1 bool
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, []byte, []*v1alpha1.Resource) ([]*v1alpha1.Resource, bool, error)); ok {
		return rf(ctx, template, oldResources)
	}
	if rf, ok := ret.Get(0).(func(context.Context, []byte, []*v1alpha1.Resource) []*v1alpha1.Resource); ok {
		r0 = rf(ctx, template, oldResources)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*v1alpha1.Resource)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, []byte, []*v1alpha1.Resource) bool); ok {
		r1 = rf(ctx, template, oldResources)
	} else {
		r1 = ret.Get(1).(bool)
	}

	if rf, ok := ret.Get(2).(func(context.Context, []byte, []*v1alpha1.Resource) error); ok {
		r2 = rf(ctx, template, oldResources)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// DeleteNewClusterStack provides a mock function with given fields: ctx, template
func (_m *Client) DeleteNewClusterStack(ctx context.Context, template []byte) ([]*v1alpha1.Resource, bool, error) {
	ret := _m.Called(ctx, template)

	var r0 []*v1alpha1.Resource
	var r1 bool
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, []byte) ([]*v1alpha1.Resource, bool, error)); ok {
		return rf(ctx, template)
	}
	if rf, ok := ret.Get(0).(func(context.Context, []byte) []*v1alpha1.Resource); ok {
		r0 = rf(ctx, template)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*v1alpha1.Resource)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, []byte) bool); ok {
		r1 = rf(ctx, template)
	} else {
		r1 = ret.Get(1).(bool)
	}

	if rf, ok := ret.Get(2).(func(context.Context, []byte) error); ok {
		r2 = rf(ctx, template)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// NewClient creates a new instance of Client. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewClient(t interface {
	mock.TestingT
	Cleanup(func())
}) *Client {
	mock := &Client{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
