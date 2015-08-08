package main

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cryptix/exp/git"

	"gopkg.in/errgo.v1"
)

// "fetch $sha1 $ref" method 1 - unpacking loose objects
//   - look for it in ".git/objects/substr($sha1, 0, 2)/substr($sha, 2)"
//   - if found, download it and put it in place. (there may be a command for this)
//   - done \o/
func fetchObject(sha1 string) error {
	p := filepath.Join(ipfsRepoPath, "objects", sha1[:2], sha1[2:])
	objF, err := ipfsShell.Cat(p)
	if err != nil {
		return errgo.Notef(err, "shell.Cat(%q) commit failed", p)
	}
	obj, err := git.DecodeObject(objF)
	if err != nil {
		return errgo.Notef(err, "git.DecodeObject(commit) failed")
	}
	commit, ok := obj.Commit()
	if !ok {
		return errgo.Newf("sha1 is not a git commit object")
	}
	// >recurese parent?

	// >recurse tree & store blobs
	p = filepath.Join(ipfsRepoPath, "objects", commit.Tree[:2], commit.Tree[2:])
	objF, err = ipfsShell.Cat(p)
	if err != nil {
		return errgo.Notef(err, "shell.Cat(%q) tree failed", p)
	}
	obj, err = git.DecodeObject(objF)
	if err != nil {
		return errgo.Notef(err, "git.DecodeObject(tree) failed")
	}

	tree, ok := obj.Tree()
	if !ok {
		return errgo.Newf("sha1 is not a git tree object")
	}

	log.Warning("Trees!")
	log.Warning(tree)
	return errgo.Newf("TODO: unsupported - please see issue #1")
}

// "fetch $sha1 $ref" method 2 - unpacking packed objects
//   - look for it in packfiles by fetching ".git/objects/pack/*.idx"
//     and looking at each idx with cat <idx> | git show-index  (alternatively can learn to read the format in go)
//   - if found in an <idx>, download the relevant .pack file,
//     and feed it into `git index-pack --stdin --fix-thin` which will put it into place.
//   - done \o/
func fetchPackedObject(sha1 string) error {
	// search for all index files
	packPath := filepath.Join(ipfsRepoPath, "objects", "pack")
	lsobj, err := ipfsShell.FileList(packPath)
	if err != nil {
		return errgo.Notef(err, "shell FileList(%q) failed", packPath)
	}
	var indexes []string
	for _, lnk := range lsobj.Links {
		if lnk.Type == "File" && strings.HasSuffix(lnk.Name, ".idx") {
			indexes = append(indexes, filepath.Join(packPath, lnk.Name))
		}
	}
	if len(indexes) == 0 {
		return errgo.New("fetchPackedObject: no idx files found")
	}
	for _, idx := range indexes {
		idxF, err := ipfsShell.Cat(idx)
		if err != nil {
			return errgo.Notef(err, "fetchPackedObject: idx<%s> cat(%s) failed", sha1, idx)
		}

		// using external git show-index < idxF for now
		// TODO: parse index file in go to make this portable
		var b bytes.Buffer
		showIdx := exec.Command("git", "show-index")
		showIdx.Stdin = idxF
		showIdx.Stdout = &b
		showIdx.Stderr = &b
		if err := showIdx.Run(); err != nil {
			return errgo.Notef(err, "fetchPackedObject: idx<%s> show-index start failed", sha1)
		}
		cmdOut := b.String()
		if !strings.Contains(cmdOut, sha1) {
			log.WithField("idx", filepath.Base(idx)).Debug("git show-index: sha1 not in index, next idx file")
			continue
		}
		//log.Debug("git show-index:", cmdOut)

		// we found an index with our hash inside
		pack := strings.Replace(idx, ".idx", ".pack", 1)
		log.Debug("unpacking:", pack)
		packF, err := ipfsShell.Cat(pack)
		if err != nil {
			return errgo.Notef(err, "fetchPackedObject: pack<%s> open() failed", sha1)
		}
		b.Reset()
		unpackIdx := exec.Command("git", "unpack-objects")
		unpackIdx.Dir = thisGitRepo // GIT_DIR
		unpackIdx.Stdin = packF
		unpackIdx.Stdout = &b
		unpackIdx.Stderr = &b
		if err := unpackIdx.Run(); err != nil {
			return errgo.Notef(err, "fetchPackedObject: pack<%s> 'git unpack-objects' failed\nOutput: %s", sha1, b.String())
		}
		log.Debug("git unpack-objects ...:", b.String())
		return nil
	}
	return errgo.Newf("did not find sha1<%s> in %d index files", sha1, len(indexes))
}
