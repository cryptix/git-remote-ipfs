package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/errgo.v1"
)

func push(src, dst string) error {
	var force = strings.HasPrefix(src, "+")
	if force {
		src = src[1:]
	}
	var present []string
	for _, h := range ref2hash {
		present = append(present, h)
	}
	// also: track previously pushed branches in 2nd map and extend present with it
	need2push, err := gitListObjects(src, present)
	if err != nil {
		return errgo.Notef(err, "push: git list objects failed %q %v", src, present)
	}
	n := len(need2push)
	type pair struct {
		Sha1  string
		MHash string
		Err   error
	}
	added := make(chan pair)
	objHash2multi := make(map[string]string, n)
	for _, sha1 := range need2push {
		go func(sha1 string) {
			r, err := gitFlattenObject(sha1)
			if err != nil {
				added <- pair{Err: errgo.Notef(err, "gitFlattenObject failed")}
				return
			}
			mhash, err := ipfsShell.Add(r)
			if err != nil {
				added <- pair{Err: errgo.Notef(err, "shell.Add(%s) failed", sha1)}
				return
			}
			added <- pair{Sha1: sha1, MHash: mhash}
		}(sha1)
	}
	for n > 0 {
		select {
		// add timeout?
		case p := <-added:
			if p.Err != nil {
				return p.Err
			}
			log.WithField("pair", p).Debug("added")
			objHash2multi[p.Sha1] = p.MHash
			n--
		}
	}
	root, err := ipfsShell.ResolvePath(ipfsRepoPath)
	if err != nil {
		return errgo.Notef(err, "resolvePath(%s) failed", ipfsRepoPath)
	}
	for sha1, mhash := range objHash2multi {
		newRoot, err := ipfsShell.PatchLink(root, filepath.Join("objects", sha1[:2], sha1[2:]), mhash, true)
		if err != nil {
			return errgo.Notef(err, "patchLink failed")
		}
		root = newRoot
		log.WithField("newRoot", newRoot).WithField("sha1", sha1).Debug("updated object")
	}
	srcSha1, err := gitRefHash(src)
	if err != nil {
		return errgo.Notef(err, "gitRefHash(%s) failed", src)
	}
	h, ok := ref2hash[dst]
	if !ok {
		return errgo.Newf("writeRef: ref2hash entry missing: %s %+v", dst, ref2hash)
	}
	isFF := gitIsAncestor(h, srcSha1)
	if isFF != nil && !force {
		// TODO: print "non-fast-forward" to git
		return fmt.Errorf("non-fast-forward")
	}
	mhash, err := ipfsShell.Add(bytes.NewBufferString(fmt.Sprintf("%s\n", srcSha1)))
	if err != nil {
		return errgo.Notef(err, "shell.Add(%s) failed", srcSha1)
	}
	root, err = ipfsShell.PatchLink(root, dst, mhash, true)
	if err != nil {
		// TODO:print "fetch first" to git
		err = errgo.Notef(err, "patchLink(%s) failed", ipfsRepoPath)
		log.WithField("err", err).Error("shell.PatchLink failed")
		return fmt.Errorf("fetch first")
	}
	log.WithField("newRoot", root).WithField("dst", dst).WithField("hash", srcSha1).Debug("updated ref")
	// invalidate info/refs and HEAD(?)
	// TODO: unclean: need to put other revs, too make a soft git update-server-info maybe
	noInfoRefsHash, err := ipfsShell.Patch(root, "rm-link", "info/refs")
	if err == nil {
		log.WithField("newRoot", noInfoRefsHash).Debug("rm-link'ed info/refs")
		root = noInfoRefsHash
	} else {
		// todo shell.IsNotExists() ?
		log.WithField("err", err).Warning("shell.Patch rm-link info/refs failed - might be okay... TODO")
	}
	newRemoteURL := fmt.Sprintf("ipfs:///ipfs/%s", root)
	setUrlCmd := exec.Command("git", "remote", "set-url", thisGitRemote, newRemoteURL)
	out, err := setUrlCmd.CombinedOutput()
	if err != nil {
		return errgo.Notef(err, "updating remote url failed\nOut:%s", string(out))
	}
	log.Info("remote updated - new address:", newRemoteURL)
	return nil
}
