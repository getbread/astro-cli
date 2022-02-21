// Code generated by mockery v2.9.4. DO NOT EDIT.

package mocks

import mock "github.com/stretchr/testify/mock"

// ContainerHandler is an autogenerated mock type for the ContainerHandler type
type ContainerHandler struct {
	mock.Mock
}

// ExecCommand provides a mock function with given fields: containerID, command
func (_m *ContainerHandler) ExecCommand(containerID string, command string) (string, error) {
	ret := _m.Called(containerID, command)

	var r0 string
	if rf, ok := ret.Get(0).(func(string, string) string); ok {
		r0 = rf(containerID, command)
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string) error); ok {
		r1 = rf(containerID, command)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetContainerID provides a mock function with given fields: containerName
func (_m *ContainerHandler) GetContainerID(containerName string) (string, error) {
	ret := _m.Called(containerName)

	var r0 string
	if rf, ok := ret.Get(0).(func(string) string); ok {
		r0 = rf(containerName)
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(containerName)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Kill provides a mock function with given fields:
func (_m *ContainerHandler) Kill() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Logs provides a mock function with given fields: follow, containerNames
func (_m *ContainerHandler) Logs(follow bool, containerNames ...string) error {
	_va := make([]interface{}, len(containerNames))
	for _i := range containerNames {
		_va[_i] = containerNames[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, follow)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 error
	if rf, ok := ret.Get(0).(func(bool, ...string) error); ok {
		r0 = rf(follow, containerNames...)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// PS provides a mock function with given fields:
func (_m *ContainerHandler) PS() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Run provides a mock function with given fields: args, user
func (_m *ContainerHandler) Run(args []string, user string) error {
	ret := _m.Called(args, user)

	var r0 error
	if rf, ok := ret.Get(0).(func([]string, string) error); ok {
		r0 = rf(args, user)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Start provides a mock function with given fields: dockerfile
func (_m *ContainerHandler) Start(dockerfile string) error {
	ret := _m.Called(dockerfile)

	var r0 error
	if rf, ok := ret.Get(0).(func(string) error); ok {
		r0 = rf(dockerfile)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Stop provides a mock function with given fields:
func (_m *ContainerHandler) Stop() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
