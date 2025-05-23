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

package exectime

import (
	"math"
	"time"
)

type RunningStatistics struct {
	N    uint64
	oldM time.Duration
	newM time.Duration
	oldS time.Duration
	newS time.Duration
}

func NewRunningStatistics() *RunningStatistics {
	return &RunningStatistics{
		N: 0,
	}
}

func (rs *RunningStatistics) Push(dur time.Duration) {
	rs.N++
	if rs.N == 1 {
		rs.oldM = dur
		rs.newM = dur
		rs.oldS = 0
		return
	}

	rs.newM = rs.oldM + ((dur - rs.oldM) / time.Duration(rs.N))
	rs.newS = rs.oldS + (dur-rs.oldM)*(dur-rs.newM)

	rs.oldM = rs.newM
	rs.oldS = rs.newS
}

func (rs *RunningStatistics) Mean() time.Duration {
	return rs.newM
}

func (rs *RunningStatistics) Variance() time.Duration {
	if rs.N > 1 {
		return rs.newS / time.Duration(rs.N-1)
	}
	return time.Duration(0)
}

func (rs *RunningStatistics) StandardDeviation() time.Duration {
	dev := math.Sqrt(float64(rs.Variance()))
	return time.Duration(dev)
}
