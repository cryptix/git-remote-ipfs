package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cryptix/exp/git"
	"github.com/pkg/errors"
)

// "fetch $sha1 $ref" method 1 - unpacking loose objects
//   - look for it in ".git/objects/substr($sha1, 0, 2)/substr($sha, 2)"
//   - if found, download it and put it in place. (there may be a command for this)
//   - done \o/
func fetchObject(sha1 string) error {
	return recurseCommit(sha1)
}

func recurseCommit(sha1 string) error {
	obj, err := fetchAndWriteObj(sha1)
	if err != nil {
		return errors.Wrapf(err, "fetchAndWriteObj(%s) commit object failed", sha1)
	}
	commit, ok := obj.Commit()
	if !ok {
		return errors.Errorf("sha1<%s> is not a git commit object:%s ", sha1, obj)
	}
	if commit.Parent != "" {
		if err := recurseCommit(commit.Parent); err != nil {
			return errors.Wrapf(err, "recurseCommit(%s) commit Parent failed", commit.Parent)
		}
	}
	return fetchTree(commit.Tree)
}

func fetchTree(sha1 string) error {
	obj, err := fetchAndWriteObj(sha1)
	if err != nil {
		return errors.Wrapf(err, "fetchAndWriteObj(%s) commit tree failed", sha1)
	}
	entries, ok := obj.Tree()
	if !ok {
		return errors.Errorf("sha1<%s> is not a git tree object:%s ", sha1, obj)
	}
	for _, t := range entries {
		obj, err := fetchAndWriteObj(t.SHA1Sum.String())
		if err != nil {
			return errors.Wrapf(err, "fetchAndWriteObj(%s) commit tree failed", sha1)
		}
		if obj.Type != git.BlobT {
			return errors.Errorf("sha1<%s> is not a git tree object:%s ", t.SHA1Sum.String(), obj)
		}
	}
	return nil
}

// fetchAndWriteObj looks for the loose object under 'thisGitRepo' global git dir
// and usses an io.TeeReader to write it to the local repo
func fetchAndWriteObj(sha1 string) (*git.Object, error) {
	p := filepath.Join(ipfsRepoPath, "objects", sha1[:2], sha1[2:])
	ipfsCat, err := ipfsShell.Cat(p)
	if err != nil {
		return nil, errors.Wrapf(err, "shell.Cat() commit failed")
	}
	targetP := filepath.Join(thisGitRepo, "objects", sha1[:2], sha1[2:])
	if err := os.MkdirAll(filepath.Join(thisGitRepo, "objects", sha1[:2]), 0700); err != nil {
		return nil, errors.Wrapf(err, "mkDirAll() failed")
	}
	targetObj, err := os.Create(targetP)
	if err != nil {
		return nil, errors.Wrapf(err, "os.Create(%s) commit failed", targetP)
	}
	obj, err := git.DecodeObject(io.TeeReader(ipfsCat, targetObj))
	if err != nil {
		return nil, errors.Wrapf(err, "git.DecodeObject(commit) failed")
	}

	if err := ipfsCat.Close(); err != nil {
		err = errors.Wrap(err, "ipfs/cat Close failed")
		if errRm := os.Remove(targetObj.Name()); errRm != nil {
			err = errors.Wrapf(err, "failed removing targetObj: %s", errRm)
			return nil, err
		}
		return nil, errors.Wrapf(err, "closing ipfs cat failed")
	}

	if err := targetObj.Close(); err != nil {
		return nil, errors.Wrapf(err, "target file close() failed")
	}

	return obj, nil
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
	links, err := ipfsShell.List(packPath)
	if err != nil {
		return errors.Wrapf(err, "shell FileList(%q) failed", packPath)
	}
	var indexes []string
	for _, lnk := range links {
		if lnk.Type == 2 && strings.HasSuffix(lnk.Name, ".idx") {
			indexes = append(indexes, filepath.Join(packPath, lnk.Name))
		}
	}
	if len(indexes) == 0 {
		return errors.New("fetchPackedObject: no idx files found")
	}
	for _, idx := range indexes {
		idxF, err := ipfsShell.Cat(idx)
		if err != nil {
			return errors.Wrapf(err, "fetchPackedObject: idx<%s> cat(%s) failed", sha1, idx)
		}
		// using external git show-index < idxF for now
		// TODO: parse index file in go to make this portable
		var b bytes.Buffer
		showIdx := exec.Command("git", "show-index")
		showIdx.Stdin = idxF
		showIdx.Stdout = &b
		showIdx.Stderr = &b
		if err := showIdx.Run(); err != nil {
			return errors.Wrapf(err, "fetchPackedObject: idx<%s> show-index start failed", sha1)
		}
		cmdOut := b.String()
		if !strings.Contains(cmdOut, sha1) {
			log.Log("idx", filepath.Base(idx), "event", "debug", "msg", "git show-index: sha1 not in index, next idx file")
			continue
		}
		// we found an index with our hash inside
		pack := strings.Replace(idx, ".idx", ".pack", 1)
		packF, err := ipfsShell.Cat(pack)
		if err != nil {
			return errors.Wrapf(err, "fetchPackedObject: pack<%s> open() failed", sha1)
		}
		b.Reset()
		unpackIdx := exec.Command("git", "unpack-objects")
		unpackIdx.Dir = thisGitRepo // GIT_DIR
		unpackIdx.Stdin = packF
		unpackIdx.Stdout = &b
		unpackIdx.Stderr = &b
		if err := unpackIdx.Run(); err != nil {
			return errors.Wrapf(err, "fetchPackedObject: pack<%s> 'git unpack-objects' failed\nOutput: %s", sha1, b.String())
		}
		return nil
	}
	return errors.Errorf("did not find sha1<%s> in %d index files", sha1, len(indexes))
}
