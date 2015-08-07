package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/whyrusleeping/ipfs-shell"
)

// fetfetchFullBareRepo 'ipfs get's a root hash and returns its location in an os temp directory
// TODO(cryptix): clean up
func fetchFullBareRepo(root string) string {
	// TODO: document host format
	shell := shell.NewShell("localhost:5001")
	tmpPath := filepath.Join("/", os.TempDir(), root)
	s, err := os.Stat(tmpPath)
	if err != nil {
		if os.IsNotExist(err) {
			if err := shell.Get(root, tmpPath); err != nil {
				log.Fatalf("shell.Get(%s, %s) failed: %s", root, tmpPath, err)
			}
			log.Println("DEBUG: shell got:", root)
			return tmpPath
		}
		log.Fatalf("stat err: %s", err)
	}
	if !s.IsDir() {
		log.Fatalf("please delete %s")
	}
	return tmpPath
}
