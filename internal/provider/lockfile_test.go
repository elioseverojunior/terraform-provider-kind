// SPDX-FileCopyrightText: 2026 Elio Severo Junior <elioseverojunior@gmail.com>
//
// SPDX-License-Identifier: MIT OR Apache-2.0

package provider

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeLockFile creates ~/.kube/config.lock with the given age.
func writeLockFile(t *testing.T, home string, age time.Duration) string {
	t.Helper()

	dir := filepath.Join(home, ".kube")
	require.NoError(t, os.MkdirAll(dir, 0o755))

	lock := filepath.Join(dir, "config.lock")
	require.NoError(t, os.WriteFile(lock, []byte("lock"), 0o600))

	modTime := time.Now().Add(-age)
	require.NoError(t, os.Chtimes(lock, modTime, modTime))

	return lock
}

// A lock left behind by an interrupted run blocks every later kubeconfig write,
// so a clearly stale one is removed.
func TestCleanupStaleLockFile_RemovesStaleLock(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	lock := writeLockFile(t, home, 5*time.Minute)

	cleanupStaleLockFile()

	assert.NoFileExists(t, lock, "a five-minute-old lock must be removed")
}

// A fresh lock may belong to a concurrent, healthy operation and must survive.
func TestCleanupStaleLockFile_KeepsFreshLock(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	lock := writeLockFile(t, home, time.Second)

	cleanupStaleLockFile()

	assert.FileExists(t, lock, "a one-second-old lock must be left alone")
}

func TestCleanupStaleLockFile_NoLockFileIsNoop(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	assert.NotPanics(t, cleanupStaleLockFile)
}

func TestCleanupStaleLockFile_UnresolvableHomeIsNoop(t *testing.T) {
	t.Setenv("HOME", "")

	assert.NotPanics(t, cleanupStaleLockFile)
}
