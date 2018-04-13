// Code generated by mockery v1.0.0
package vault

import jsonrpc "github.com/ybbus/jsonrpc"
import mock "github.com/stretchr/testify/mock"

// MockRPCClient is an autogenerated mock type for the RPCClient type
type MockRPCClient struct {
	mock.Mock
}

// Call provides a mock function with given fields: method, params
func (_m *MockRPCClient) Call(method string, params ...interface{}) (*jsonrpc.RPCResponse, error) {
	var _ca []interface{}
	_ca = append(_ca, method)
	_ca = append(_ca, params...)
	ret := _m.Called(_ca...)

	var r0 *jsonrpc.RPCResponse
	if rf, ok := ret.Get(0).(func(string, ...interface{}) *jsonrpc.RPCResponse); ok {
		r0 = rf(method, params...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*jsonrpc.RPCResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, ...interface{}) error); ok {
		r1 = rf(method, params...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
