//go:build !authtest

/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2025 Seagate Technology LLC and/or its Affiliates
   Copyright © 2020-2025 Microsoft Corporation. All rights reserved.

   Permission is hereby granted, free of charge, to any person obtaining a copy
   of this software and associated documentation files (the "Software"), to deal
   in the Software without restriction, including without limitation the rights
   to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
   copies of the Software, and to permit persons to whom the Software is
   furnished to do so, subject to the following conditions:

   The above copyright notice and this permission notice shall be included in all
   copies or substantial portions of the Software.

   THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
   IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
   FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
   AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
   LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
   OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
   SOFTWARE
*/

package block_cache

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type threadPoolTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *threadPoolTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	suite.assert.NoError(err)
}

func (suite *threadPoolTestSuite) cleanupTest() {
}

func (suite *threadPoolTestSuite) TestCreate() {
	suite.assert = assert.New(suite.T())

	tp := newThreadPool(0, nil, nil)
	suite.assert.Nil(tp)

	tp = newThreadPool(1, nil, nil)
	suite.assert.Nil(tp)

	tp = newThreadPool(1, func(*workItem) {}, nil)
	suite.assert.NotNil(tp)
	suite.assert.Equal(uint32(1), tp.worker)
}

func (suite *threadPoolTestSuite) TestStartStop() {
	suite.assert = assert.New(suite.T())

	r := func(i *workItem) {
		suite.assert.Equal(int32(1), i.failCnt)
	}

	tp := newThreadPool(2, r, nil)
	suite.assert.NotNil(tp)
	suite.assert.Equal(uint32(2), tp.worker)

	tp.Start()
	suite.assert.NotNil(tp.priorityCh)
	suite.assert.NotNil(tp.normalCh)

	tp.Stop()
}

func (suite *threadPoolTestSuite) TestSchedule() {
	suite.assert = assert.New(suite.T())

	r := func(i *workItem) {
		suite.assert.Equal(int32(1), i.failCnt)
	}

	tp := newThreadPool(2, r, nil)
	suite.assert.NotNil(tp)
	suite.assert.Equal(uint32(2), tp.worker)

	tp.Start()
	suite.assert.NotNil(tp.priorityCh)
	suite.assert.NotNil(tp.normalCh)

	tp.Schedule(false, &workItem{failCnt: 1})
	tp.Schedule(true, &workItem{failCnt: 1})

	tp.Stop()
}

func (suite *threadPoolTestSuite) TestPrioritySchedule() {
	suite.assert = assert.New(suite.T())

	callbackCnt := int32(0)
	r := func(i *workItem) {
		suite.assert.Equal(int32(5), i.failCnt)
		atomic.AddInt32(&callbackCnt, 1)
	}

	tp := newThreadPool(10, r, nil)
	suite.assert.NotNil(tp)
	suite.assert.Equal(uint32(10), tp.worker)

	tp.Start()
	suite.assert.NotNil(tp.priorityCh)
	suite.assert.NotNil(tp.normalCh)

	for i := 0; i < 100; i++ {
		tp.Schedule(i < 20, &workItem{failCnt: 5})
	}

	time.Sleep(100 * time.Millisecond)
	suite.assert.Equal(int32(100), callbackCnt)
	tp.Stop()
}

func (suite *threadPoolTestSuite) TestPriorityScheduleWithWriter() {
	suite.assert = assert.New(suite.T())

	callbackRCnt := int32(0)
	callbackWCnt := int32(0)
	r := func(i *workItem) {
		suite.assert.Equal(int32(5), i.failCnt)
		atomic.AddInt32(&callbackRCnt, 1)
	}

	w := func(i *workItem) {
		suite.assert.Equal(int32(5), i.failCnt)
		atomic.AddInt32(&callbackWCnt, 1)
	}

	tp := newThreadPool(10, r, w)
	suite.assert.NotNil(tp)
	suite.assert.Equal(uint32(10), tp.worker)

	tp.Start()
	suite.assert.NotNil(tp.priorityCh)
	suite.assert.NotNil(tp.normalCh)

	for i := 0; i < 100; i++ {
		tp.Schedule(i < 20, &workItem{failCnt: 5, upload: true, blockId: "test"})
	}

	time.Sleep(100 * time.Millisecond)
	suite.assert.Equal(int32(100), callbackWCnt)
	suite.assert.Equal(int32(0), callbackRCnt)
	tp.Stop()
}

func TestThreadPoolSuite(t *testing.T) {
	suite.Run(t, new(threadPoolTestSuite))
}
