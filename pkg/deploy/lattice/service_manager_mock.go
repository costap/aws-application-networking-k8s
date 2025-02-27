// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/aws/aws-application-networking-k8s/pkg/deploy/lattice (interfaces: ServiceManager)

// Package lattice is a generated GoMock package.
package lattice

import (
	context "context"
	reflect "reflect"

	lattice "github.com/aws/aws-application-networking-k8s/pkg/model/lattice"
	gomock "github.com/golang/mock/gomock"
)

// MockServiceManager is a mock of ServiceManager interface.
type MockServiceManager struct {
	ctrl     *gomock.Controller
	recorder *MockServiceManagerMockRecorder
}

// MockServiceManagerMockRecorder is the mock recorder for MockServiceManager.
type MockServiceManagerMockRecorder struct {
	mock *MockServiceManager
}

// NewMockServiceManager creates a new mock instance.
func NewMockServiceManager(ctrl *gomock.Controller) *MockServiceManager {
	mock := &MockServiceManager{ctrl: ctrl}
	mock.recorder = &MockServiceManagerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockServiceManager) EXPECT() *MockServiceManagerMockRecorder {
	return m.recorder
}

// Create mocks base method.
func (m *MockServiceManager) Create(arg0 context.Context, arg1 *lattice.Service) (lattice.ServiceStatus, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Create", arg0, arg1)
	ret0, _ := ret[0].(lattice.ServiceStatus)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Create indicates an expected call of Create.
func (mr *MockServiceManagerMockRecorder) Create(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Create", reflect.TypeOf((*MockServiceManager)(nil).Create), arg0, arg1)
}

// Delete mocks base method.
func (m *MockServiceManager) Delete(arg0 context.Context, arg1 *lattice.Service) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Delete", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Delete indicates an expected call of Delete.
func (mr *MockServiceManagerMockRecorder) Delete(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockServiceManager)(nil).Delete), arg0, arg1)
}
