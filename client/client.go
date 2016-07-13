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

package client

import "github.com/m3db/m3db/interfaces/m3db"

type client struct {
	opts         m3db.ClientOptions
	newSessionFn newSessionFn
}

type newSessionFn func(opts m3db.ClientOptions) (clientSession, error)

// NewClient creates a new client
func NewClient(opts m3db.ClientOptions) (m3db.Client, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}
	return &client{opts: opts, newSessionFn: newSession}, nil
}

func (c *client) NewSession() (m3db.Session, error) {
	session, err := c.newSessionFn(c.opts)
	if err != nil {
		return nil, err
	}
	if err := session.Open(); err != nil {
		return nil, err
	}
	return session, nil
}
