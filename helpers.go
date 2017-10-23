package main

import (
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/pkg/errors"
)

func fetchFullBareRepo(root string) (string, error) {
	// TODO: get host from envvar
	tmpPath := filepath.Join("/", os.TempDir(), root)
	_, err := os.Stat(tmpPath)
	switch {
	case os.IsNotExist(err) || err == nil:
		if err := ipfsShell.Get(root, tmpPath); err != nil {
			return "", errors.Wrapf(err, "shell.Get(%s, %s) failed: %s", root, tmpPath, err)
		}
		return tmpPath, nil
	default:
		return "", errors.Wrap(err, "os.Stat(): unhandled error")
	}
}

func interrupt() error {
	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	return errors.Errorf("%s", <-c)
}
