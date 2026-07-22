package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveOwnershipUnset(t *testing.T) {
	// Nothing configured -> no fixup.
	own, err := ResolveOwnership("", "", "", "")
	require.NoError(t, err)
	assert.Nil(t, own)
}

func TestResolveOwnershipNumericAndModes(t *testing.T) {
	own, err := ResolveOwnership("501", "20", "0640", "0750")
	require.NoError(t, err)
	require.NotNil(t, own)
	assert.Equal(t, 501, own.UID)
	assert.Equal(t, 20, own.GID)
	assert.Equal(t, os.FileMode(0o640), own.FileMode)
	assert.Equal(t, os.FileMode(0o750), own.DirMode)
}

func TestResolveOwnershipModeDefaults(t *testing.T) {
	// Only a mode set -> ids left unchanged (-1), the other mode defaulted.
	own, err := ResolveOwnership("", "", "0600", "")
	require.NoError(t, err)
	require.NotNil(t, own)
	assert.Equal(t, -1, own.UID)
	assert.Equal(t, -1, own.GID)
	assert.Equal(t, os.FileMode(0o600), own.FileMode)
	assert.Equal(t, os.FileMode(0o755), own.DirMode)
}

func TestResolveOwnershipInvalid(t *testing.T) {
	_, err := ResolveOwnership("", "", "not-octal", "")
	assert.Error(t, err)

	_, err = ResolveOwnership("definitely-no-such-user-xyz", "", "", "")
	assert.Error(t, err)
}

func TestApplyChmodsFileAndParents(t *testing.T) {
	root := t.TempDir()
	// Give root a distinct mode so we can assert Apply leaves the scan mount alone.
	require.NoError(t, os.Chmod(root, 0o700))
	nested := filepath.Join(root, "2026", "2026-06-05")
	require.NoError(t, os.MkdirAll(nested, 0o700))
	file := filepath.Join(nested, "photo.dng")
	require.NoError(t, os.WriteFile(file, []byte("bytes"), 0o600))

	// Chown to the current uid/gid (a no-op the test process is always allowed to
	// perform) so the chmod path is exercised without needing root.
	own := &Ownership{UID: os.Getuid(), GID: os.Getgid(), FileMode: 0o644, DirMode: 0o755}
	require.NoError(t, own.Apply(root, file))

	fi, err := os.Stat(file)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), fi.Mode().Perm())

	for _, dir := range []string{filepath.Join(root, "2026"), nested} {
		di, err := os.Stat(dir)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o755), di.Mode().Perm(), dir)
	}

	// root itself (the scan mount) must be left untouched.
	ri, err := os.Stat(root)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), ri.Mode().Perm(), "root should not be modified")
}

func TestApplyNilReceiver(t *testing.T) {
	var own *Ownership
	assert.NoError(t, own.Apply("/tmp", "/tmp/x"))
}
