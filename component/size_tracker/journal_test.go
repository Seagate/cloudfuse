/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2025 Seagate Technology LLC and/or its Affiliates
*/

package size_tracker

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Seagate/cloudfuse/common"
	"github.com/stretchr/testify/require"
)

// Helpers
func withTempWorkDir(t *testing.T) (string, func()) {
	t.Helper()
	dir := t.TempDir()
	// Point DefaultWorkDir to temp dir
	old := common.DefaultWorkDir
	common.DefaultWorkDir = dir
	cleanup := func() {
		common.DefaultWorkDir = old
	}
	return dir, cleanup
}

func writeLegacyJournal(t *testing.T, dir, name string, val uint64) {
	t.Helper()
	p := filepath.Join(dir, name)
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	require.NoError(t, err)
	defer f.Close()
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], val)
	_, err = f.Write(b[:])
	require.NoError(t, err)
}

func writeINIJournal(t *testing.T, dir, name string, lines []string) {
	t.Helper()
	p := filepath.Join(dir, name)
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	require.NoError(t, err)
	defer f.Close()
	w := bufio.NewWriter(f)
	for _, ln := range lines {
		_, err := w.WriteString(ln + "\n")
		require.NoError(t, err)
	}
	require.NoError(t, w.Flush())
}

func readFileString(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	b, err := os.ReadFile(p)
	require.NoError(t, err)
	return string(b)
}

// Tests

func TestJournal_Legacy8ByteInitialization(t *testing.T) {
	dir, restore := withTempWorkDir(t)
	defer restore()

	const jname = "journal_legacy.dat"
	// seed legacy 8-byte file
	writeLegacyJournal(t, dir, jname, 1234)

	ms, err := CreateSizeJournal(jname)
	require.NoError(t, err)
	// Expect size from legacy file and default epoch=1
	require.Equal(t, uint64(1234), ms.GetSize())
	require.Equal(t, uint64(1), ms.epoch.Load())

	// Apply a small delta and sync to force INI conversion
	ms.Add(6)
	require.NoError(t, ms.sync())

	content := readFileString(t, dir, jname)
	require.Contains(t, content, "version=1")
	require.Contains(t, content, "epoch=1")
	require.Contains(t, content, "size_bytes=1240")
	require.Contains(t, content, "updated_unix_ms=")
}

func TestJournal_INIDefaultEpochTo1(t *testing.T) {
	dir, restore := withTempWorkDir(t)
	defer restore()
	const jname = "journal_ini_default_epoch.dat"

	// Write INI without epoch
	writeINIJournal(t, dir, jname, []string{
		"version=1",
		"size_bytes=42",
		"updated_unix_ms=0",
	})

	ms, err := CreateSizeJournal(jname)
	require.NoError(t, err)
	require.Equal(t, uint64(42), ms.GetSize())
	require.Equal(t, uint64(1), ms.epoch.Load())

	// Ensure rewrite uses epoch=1
	require.NoError(t, ms.sync())
	content := readFileString(t, dir, jname)
	require.Contains(t, content, "epoch=1")
}

func TestJournal_EpochBumpDiscardsDelta(t *testing.T) {
	dir, restore := withTempWorkDir(t)
	defer restore()
	const jname = "journal_epoch_bump.dat"

	// Start with epoch=1 size=100
	writeINIJournal(t, dir, jname, []string{
		"version=1",
		"epoch=1",
		"size_bytes=100",
		"updated_unix_ms=0",
	})

	ms, err := CreateSizeJournal(jname)
	require.NoError(t, err)
	require.Equal(t, uint64(100), ms.GetSize())
	require.Equal(t, uint64(1), ms.epoch.Load())

	// Accumulate a delta locally but don't sync yet
	ms.Add(50)

	// External auditor bumps epoch to 2 and sets size to 999
	writeINIJournal(t, dir, jname, []string{
		"version=1",
		"epoch=2",
		"size_bytes=999",
		"updated_unix_ms=1",
	})

	// Now sync; should discard pending +50 and adopt epoch=2
	require.NoError(t, ms.sync())
	require.Equal(t, uint64(2), ms.epoch.Load())
	require.Equal(t, uint64(999), ms.GetSize())
	require.Equal(t, int64(0), ms.pendingDelta.Load())

	// File should reflect epoch=2 and size 999 (or timestamp changed).
	content := readFileString(t, dir, jname)
	require.Contains(t, content, "epoch=2")
	require.True(
		t,
		strings.Contains(content, "size_bytes=999"),
		fmt.Sprintf("content: %s", content),
	)
}

func TestJournal_ApplyDeltaAndWriteINI(t *testing.T) {
	dir, restore := withTempWorkDir(t)
	defer restore()
	const jname = "journal_apply_delta.dat"

	writeINIJournal(t, dir, jname, []string{
		"version=1",
		"epoch=1",
		"size_bytes=10",
		"updated_unix_ms=0",
	})

	ms, err := CreateSizeJournal(jname)
	require.NoError(t, err)
	require.Equal(t, uint64(10), ms.GetSize())
	require.Equal(t, uint64(1), ms.epoch.Load())

	ms.Add(5)
	require.NoError(t, ms.sync())
	require.Equal(t, uint64(15), ms.GetSize())
	require.Equal(t, int64(0), ms.pendingDelta.Load())

	content := readFileString(t, dir, jname)
	require.Contains(t, content, "epoch=1")
	require.Contains(t, content, "size_bytes=15")
}

func TestJournal_HigherLocalEpochOverwritesFile(t *testing.T) {
	dir, restore := withTempWorkDir(t)
	defer restore()
	const jname = "journal_higher_epoch.dat"

	// Start with epoch=1 size=100 in file
	writeINIJournal(t, dir, jname, []string{
		"version=1",
		"epoch=1",
		"size_bytes=100",
		"updated_unix_ms=0",
	})

	ms, err := CreateSizeJournal(jname)
	require.NoError(t, err)
	require.Equal(t, uint64(100), ms.GetSize())
	require.Equal(t, uint64(1), ms.epoch.Load())

	// Bump local epoch to 3
	ms.epoch.Store(3)

	// Accumulate a delta locally
	ms.Add(50)
	require.Equal(t, int64(50), ms.pendingDelta.Load())

	// Sync should apply the delta and write epoch=3 to file
	require.NoError(t, ms.sync())
	require.Equal(t, uint64(3), ms.epoch.Load())
	require.Equal(t, uint64(150), ms.GetSize())
	require.Equal(t, int64(0), ms.pendingDelta.Load())

	// File should now reflect epoch=3 and size 150
	content := readFileString(t, dir, jname)
	require.Contains(t, content, "epoch=3")
	require.True(
		t,
		strings.Contains(content, "size_bytes=150"),
		fmt.Sprintf("expected size_bytes=150 in content: %s", content),
	)

	// To further verify: write a file with lower epoch
	// When sync() reads this file, it will see epoch=2 < myEpoch=3
	// Since myEpoch >= fileEpoch, we keep our local size (150) and apply the delta
	writeINIJournal(t, dir, jname, []string{
		"version=1",
		"epoch=2",
		"size_bytes=999",
		"updated_unix_ms=1",
	})

	// Add another delta and sync
	ms.Add(25)
	require.NoError(t, ms.sync())

	// Local epoch=3 should remain, and we use our local size as base
	require.Equal(t, uint64(3), ms.epoch.Load())
	// Since we preserve our size (150) and apply delta=25, we get 175
	require.Equal(t, uint64(175), ms.GetSize())
	require.Equal(t, int64(0), ms.pendingDelta.Load())

	// File should show epoch=3 and size=175 (overwriting the epoch=2 file)
	content = readFileString(t, dir, jname)
	require.Contains(t, content, "epoch=3")
	require.True(
		t,
		strings.Contains(content, "size_bytes=175"),
		fmt.Sprintf("expected size_bytes=175 in content: %s", content),
	)
}
