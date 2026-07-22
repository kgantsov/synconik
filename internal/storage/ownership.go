package storage

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
)

// Ownership describes the uid/gid and permission bits to apply to files and the
// directories created for them by the restore/download path. A uid or gid of -1
// means "leave unchanged" (the semantics of os.Chown).
type Ownership struct {
	UID      int
	GID      int
	FileMode os.FileMode
	DirMode  os.FileMode
}

// ResolveOwnership builds an *Ownership from config strings. owner/group may be a
// name or a numeric id; empty leaves that id unchanged (-1). fileMode/dirMode are
// octal strings (e.g. "0644"); empty falls back to 0644/0755. It returns (nil, nil)
// when nothing is configured so callers can cheaply skip the fixup.
func ResolveOwnership(owner, group, fileMode, dirMode string) (*Ownership, error) {
	if owner == "" && group == "" && fileMode == "" && dirMode == "" {
		return nil, nil
	}

	uid, err := lookupID(owner, false)
	if err != nil {
		return nil, err
	}
	gid, err := lookupID(group, true)
	if err != nil {
		return nil, err
	}

	fMode, err := parseMode(fileMode, 0o644)
	if err != nil {
		return nil, fmt.Errorf("invalid restore file_mode %q: %w", fileMode, err)
	}
	dMode, err := parseMode(dirMode, 0o755)
	if err != nil {
		return nil, fmt.Errorf("invalid restore dir_mode %q: %w", dirMode, err)
	}

	return &Ownership{UID: uid, GID: gid, FileMode: fMode, DirMode: dMode}, nil
}

// Apply sets ownership and permissions on the file at path and on each parent
// directory between root (exclusive) and the file. root — the scan mount — is left
// untouched. It is a no-op on a nil receiver, so callers need not nil-check.
// Chown is skipped when both ids are -1.
func (o *Ownership) Apply(root, path string) error {
	if o == nil {
		return nil
	}

	if err := o.applyOne(path, o.FileMode); err != nil {
		return err
	}

	root = filepath.Clean(root)
	for dir := filepath.Dir(path); ; dir = filepath.Dir(dir) {
		cleaned := filepath.Clean(dir)
		if cleaned == root || cleaned == "." || cleaned == string(filepath.Separator) {
			break
		}
		if err := o.applyOne(dir, o.DirMode); err != nil {
			return err
		}
		// Guard against filepath.Dir fixpointing (e.g. "/") if root is not an
		// ancestor of path, so the loop always terminates.
		if filepath.Dir(dir) == dir {
			break
		}
	}
	return nil
}

func (o *Ownership) applyOne(path string, mode os.FileMode) error {
	if err := os.Chmod(path, mode); err != nil {
		return err
	}
	if o.UID != -1 || o.GID != -1 {
		if err := os.Chown(path, o.UID, o.GID); err != nil {
			return err
		}
	}
	return nil
}

// lookupID resolves a user or group name to its numeric id, accepting a numeric
// string directly. An empty value returns -1 ("leave unchanged").
func lookupID(name string, group bool) (int, error) {
	if name == "" {
		return -1, nil
	}

	if group {
		if g, err := user.LookupGroup(name); err == nil {
			return strconv.Atoi(g.Gid)
		}
	} else {
		if u, err := user.Lookup(name); err == nil {
			return strconv.Atoi(u.Uid)
		}
	}

	if id, err := strconv.Atoi(name); err == nil {
		return id, nil
	}

	kind := "user"
	if group {
		kind = "group"
	}
	return -1, fmt.Errorf("unknown %s %q", kind, name)
}

// parseMode parses an octal permission string like "0644"/"644"/"0o644", falling
// back to def when empty.
func parseMode(s string, def os.FileMode) (os.FileMode, error) {
	if s == "" {
		return def, nil
	}
	m, err := strconv.ParseUint(strings.TrimPrefix(s, "0o"), 8, 32)
	if err != nil {
		return 0, err
	}
	return os.FileMode(m), nil
}
