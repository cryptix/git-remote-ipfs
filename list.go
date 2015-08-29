package main

import (
	"bufio"
	"bytes"
	"io/ioutil"
	"path/filepath"
	"strings"

	"gopkg.in/errgo.v1"
)

func listInfoRefs(forPush bool) error {
	refsCat, err := ipfsShell.Cat(filepath.Join(ipfsRepoPath, "info", "refs"))
	if err != nil {
		return errgo.Notef(err, "failed to cat info/refs from %s", ipfsRepoPath)
	}
	s := bufio.NewScanner(refsCat)
	for s.Scan() {
		hashRef := strings.Split(s.Text(), "\t")
		if len(hashRef) != 2 {
			return errgo.Newf("processing info/refs: what is this: %v", hashRef)
		}
		ref2hash[hashRef[1]] = hashRef[0]
		log.WithField("ref", hashRef[1]).WithField("sha1", hashRef[0]).Debug("got ref")
	}
	if err := s.Err(); err != nil {
		return errgo.Notef(err, "ipfs.Cat(info/refs) scanner error")
	}
	return nil
}

func listHeadRef() (string, error) {
	headCat, err := ipfsShell.Cat(filepath.Join(ipfsRepoPath, "HEAD"))
	if err != nil {
		return "", errgo.Notef(err, "failed to cat HEAD from %s", ipfsRepoPath)
	}
	head, err := ioutil.ReadAll(headCat)
	if err != nil {
		return "", errgo.Notef(err, "failed to readAll HEAD from %s", ipfsRepoPath)
	}
	if !bytes.HasPrefix(head, []byte("ref: ")) {
		return "", errgo.Newf("illegal HEAD file from %s: %q", ipfsRepoPath, head)
	}
	headRef := string(bytes.TrimSpace(head[5:]))
	headHash, ok := ref2hash[headRef]
	if !ok {
		// use first hash in map?..
		return "", errgo.Newf("unknown HEAD reference %q", headRef)
	}
	log.WithField("ref", headRef).WithField("sha1", headHash).Debug("got HEAD ref")
	return headHash, nil
}
