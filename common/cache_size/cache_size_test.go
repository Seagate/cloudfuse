/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2026 Seagate Technology LLC and/or its Affiliates

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

package cache_size

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Seagate/cloudfuse/common"
	"github.com/stretchr/testify/assert"
)

func TestAddClampsAtZero(t *testing.T) {
	tracker := NewTracker(time.Hour, t.TempDir())

	tracker.Add(100)
	tracker.Add(-40)
	assert.EqualValues(t, 60, tracker.Used())

	tracker.Add(-100)
	assert.EqualValues(t, 0, tracker.Used())
}

func TestRefreshSumsAllPaths(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "first")
	second := filepath.Join(root, "second")
	ignored := filepath.Join(root, "ignored")
	for _, dir := range []string{first, second, ignored} {
		assert.NoError(t, os.Mkdir(dir, 0755))
	}
	assert.NoError(t, os.WriteFile(filepath.Join(first, "a"), make([]byte, 3000), 0644))
	assert.NoError(t, os.WriteFile(filepath.Join(second, "b"), make([]byte, 5000), 0644))
	assert.NoError(t, os.WriteFile(filepath.Join(ignored, "c"), make([]byte, 7000), 0644))

	tracker := NewTracker(time.Hour, first, second)
	tracker.Add(1)
	assert.NoError(t, tracker.Refresh())

	firstUsage, err := common.GetUsage(first)
	assert.NoError(t, err)
	secondUsage, err := common.GetUsage(second)
	assert.NoError(t, err)
	assert.Equal(t, int64(firstUsage+secondUsage), tracker.Used())
}

func TestRefreshMissingPath(t *testing.T) {
	tracker := NewTracker(time.Hour, filepath.Join(t.TempDir(), "missing"))
	tracker.Add(10)

	assert.Error(t, tracker.Refresh())
	assert.EqualValues(t, 10, tracker.Used())
}

func TestReconcileRespectsInterval(t *testing.T) {
	root := t.TempDir()
	assert.NoError(t, os.WriteFile(filepath.Join(root, "a"), make([]byte, 4000), 0644))

	tracker := NewTracker(time.Hour, root)
	tracker.Reconcile()
	assert.EqualValues(t, 0, tracker.Used(), "reconciled before the interval elapsed")

	tracker = NewTracker(0, root)
	tracker.Reconcile()
	assert.GreaterOrEqual(t, tracker.Used(), int64(4000))
}
