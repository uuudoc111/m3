// Automatically generated by MockGen. DO NOT EDIT!
// Source: series.go

// Copyright (c) 2016 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package mocks

import (
	time "time"

	m3db "github.com/m3db/m3db/interfaces/m3db"
	time0 "github.com/m3db/m3db/x/time"

	gomock "github.com/golang/mock/gomock"
)

// Mock of databaseSeries interface
type MockdatabaseSeries struct {
	ctrl     *gomock.Controller
	recorder *_MockdatabaseSeriesRecorder
}

// Recorder for MockdatabaseSeries (not exported)
type _MockdatabaseSeriesRecorder struct {
	mock *MockdatabaseSeries
}

func NewMockdatabaseSeries(ctrl *gomock.Controller) *MockdatabaseSeries {
	mock := &MockdatabaseSeries{ctrl: ctrl}
	mock.recorder = &_MockdatabaseSeriesRecorder{mock}
	return mock
}

func (_m *MockdatabaseSeries) EXPECT() *_MockdatabaseSeriesRecorder {
	return _m.recorder
}

func (_m *MockdatabaseSeries) ID() string {
	ret := _m.ctrl.Call(_m, "ID")
	ret0, _ := ret[0].(string)
	return ret0
}

func (_mr *_MockdatabaseSeriesRecorder) ID() *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "ID")
}

func (_m *MockdatabaseSeries) Tick() error {
	ret := _m.ctrl.Call(_m, "Tick")
	ret0, _ := ret[0].(error)
	return ret0
}

func (_mr *_MockdatabaseSeriesRecorder) Tick() *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "Tick")
}

func (_m *MockdatabaseSeries) Write(ctx m3db.Context, timestamp time.Time, value float64, unit time0.Unit, annotation []byte) error {
	ret := _m.ctrl.Call(_m, "Write", ctx, timestamp, value, unit, annotation)
	ret0, _ := ret[0].(error)
	return ret0
}

func (_mr *_MockdatabaseSeriesRecorder) Write(arg0, arg1, arg2, arg3, arg4 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "Write", arg0, arg1, arg2, arg3, arg4)
}

func (_m *MockdatabaseSeries) ReadEncoded(ctx m3db.Context, start time.Time, end time.Time) ([][]m3db.SegmentReader, error) {
	ret := _m.ctrl.Call(_m, "ReadEncoded", ctx, start, end)
	ret0, _ := ret[0].([][]m3db.SegmentReader)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (_mr *_MockdatabaseSeriesRecorder) ReadEncoded(arg0, arg1, arg2 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "ReadEncoded", arg0, arg1, arg2)
}

func (_m *MockdatabaseSeries) Empty() bool {
	ret := _m.ctrl.Call(_m, "Empty")
	ret0, _ := ret[0].(bool)
	return ret0
}

func (_mr *_MockdatabaseSeriesRecorder) Empty() *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "Empty")
}

func (_m *MockdatabaseSeries) Bootstrap(rs m3db.DatabaseSeriesBlocks, cutover time.Time) error {
	ret := _m.ctrl.Call(_m, "Bootstrap", rs, cutover)
	ret0, _ := ret[0].(error)
	return ret0
}

func (_mr *_MockdatabaseSeriesRecorder) Bootstrap(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "Bootstrap", arg0, arg1)
}

func (_m *MockdatabaseSeries) FlushToDisk(ctx m3db.Context, writer m3db.FileSetWriter, blockStart time.Time, segmentHolder [][]byte) error {
	ret := _m.ctrl.Call(_m, "FlushToDisk", ctx, writer, blockStart, segmentHolder)
	ret0, _ := ret[0].(error)
	return ret0
}

func (_mr *_MockdatabaseSeriesRecorder) FlushToDisk(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "FlushToDisk", arg0, arg1, arg2, arg3)
}
