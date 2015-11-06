package main

import (
	"bufio"
	"bytes"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/cryptix/git-remote-ipfs/Godeps/_workspace/src/github.com/ipfs/go-ipfs-api"
	"github.com/cryptix/git-remote-ipfs/Godeps/_workspace/src/gopkg.in/errgo.v1"
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
	return headHash, headCat.Close()
}

func listIterateRefs(forPush bool) error {
	refsDir := filepath.Join(ipfsRepoPath, "refs")
	return Walk(refsDir, func(p string, info *shell.LsLink, err error) error {
		if err != nil {
			return errgo.Notef(err, "walk(%s) failed", p)
		}
		log.WithField("info", info).Debug("iterateRefs: walked to:", p)
		if info.Type == 2 {
			rc, err := ipfsShell.Cat(p)
			if err != nil {
				return errgo.Notef(err, "walk(%s) cat ref failed", p)
			}
			data, err := ioutil.ReadAll(rc)
			if err != nil {
				return errgo.Notef(err, "walk(%s) readAll failed", p)
			}
			if err := rc.Close(); err != nil {
				return errgo.Notef(err, "walk(%s) cat close failed", p)
			}
			sha1 := strings.TrimSpace(string(data))
			refName := strings.TrimPrefix(p, ipfsRepoPath+"/")
			ref2hash[refName] = sha1
			log.WithField("refMap", ref2hash).Debug("ref2hash map updated")
		}
		return nil
	})
}

// semi-todo make shell implement http.FileSystem
// then we can reuse filepath.Walk and make a lot of other stuff simpler
var SkipDir = errgo.Newf("walk: skipping")

type WalkFunc func(path string, info *shell.LsLink, err error) error

func walk(path string, info *shell.LsLink, walkFn WalkFunc) error {
	err := walkFn(path, info, nil)
	if err != nil {
		if info.Type == 1 && err == SkipDir {
			return nil
		}
		return err
	}
	if info.Type != 1 {
		return nil
	}
	list, err := ipfsShell.List(path)
	if err != nil {
		log.Error("walk list failed", err)
		return walkFn(path, info, err)
	}
	for _, lnk := range list {
		fname := filepath.Join(path, lnk.Name)
		err = walk(fname, lnk, walkFn)
		if err != nil {
			if lnk.Type != 1 || err != SkipDir {
				return err
			}
		}
	}
	return nil
}

func Walk(root string, walkFn WalkFunc) error {
	list, err := ipfsShell.List(root)
	if err != nil {
		log.Error("walk root failed", err)
		return walkFn(root, nil, err)
	}
	for _, l := range list {
		fname := filepath.Join(root, l.Name)
		if err := walk(fname, l, walkFn); err != nil {
			return err
		}
	}
	return nil
}
