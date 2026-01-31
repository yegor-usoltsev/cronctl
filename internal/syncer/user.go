package syncer

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

var errUserNotFound = errors.New("user not found")

func resolveJobUser(username string) (uid int, gid int, _ error) {
	u, err := lookupPasswd(username)
	if err != nil {
		return 0, 0, err
	}
	return u.uid, u.gid, nil
}

type passwdEntry struct {
	uid int
	gid int
}

func lookupPasswd(username string) (passwdEntry, error) {
	f, err := os.Open("/etc/passwd")
	if err != nil {
		return passwdEntry{}, fmt.Errorf("open /etc/passwd: %w", err)
	}
	defer func() { _ = f.Close() }()

	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) < 4 {
			continue
		}
		if parts[0] != username {
			continue
		}
		uid, err := strconv.Atoi(parts[2])
		if err != nil {
			return passwdEntry{}, fmt.Errorf("parse uid for %s: %w", username, err)
		}
		gid, err := strconv.Atoi(parts[3])
		if err != nil {
			return passwdEntry{}, fmt.Errorf("parse gid for %s: %w", username, err)
		}
		return passwdEntry{uid: uid, gid: gid}, nil
	}
	if err := s.Err(); err != nil {
		return passwdEntry{}, fmt.Errorf("failed to scan /etc/passwd: %w", err)
	}
	return passwdEntry{}, fmt.Errorf("%w: %s", errUserNotFound, username)
}
