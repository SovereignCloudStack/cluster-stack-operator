// Code generated by mockery v2.32.4. DO NOT EDIT.

package mocks

import (
	kube "github.com/SovereignCloudStack/cluster-stack-operator/pkg/kube"
	mock "github.com/stretchr/testify/mock"

	rest "k8s.io/client-go/rest"
)

// Factory is an autogenerated mock type for the Factory type
type Factory struct {
	mock.Mock
}

// NewClient provides a mock function with given fields: namespace, resCfg
func (_m *Factory) NewClient(namespace string, resCfg *rest.Config) kube.Client {
	ret := _m.Called(namespace, resCfg)

	var r0 kube.Client
	if rf, ok := ret.Get(0).(func(string, *rest.Config) kube.Client); ok {
		r0 = rf(namespace, resCfg)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(kube.Client)
		}
	}

	return r0
}

// NewFactory creates a new instance of Factory. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewFactory(t interface {
	mock.TestingT
	Cleanup(func())
}) *Factory {
	mock := &Factory{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
