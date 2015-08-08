package main

import (
	"bytes"
	"log"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cryptix/exp/git"

	"gopkg.in/errgo.v1"
)

// "fetch $sha1 $ref" method 1
//   - look for it in ".git/objects/substr($sha1, 0, 2)/substr($sha, 2)"
//   - if found, download it and put it in place. (there may be a command for this)
//   - done \o/
func fetchObject(sha1 string) error {
	p := filepath.Join(ipfsRepoPath, "objects", sha1[:2], sha1[2:])
	objF, err := ipfsShell.Cat(p)
	if err != nil {
		return errgo.Notef(err, "shell.Cat(%q) failed", p)
	}
	obj, err := git.DecodeObject(objF)
	if err != nil {
		return errgo.Notef(err, "git.DecodeObject() failed")
	}
	log.Println(obj)
	// assert(typ=commit)
	// >recurese parent?
	// >recurse tree & store blobs

	return errgo.Newf("TODO: unsupported - please see issue #1")
}

// "fetch $sha1 $ref" method 2
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
			// sha1 not in index, next idx file
			continue
		}
		//log.Println("git show-index:", cmdOut)

		// we found an index with our hash inside
		pack := strings.Replace(idx, ".idx", ".pack", 1)
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
		log.Println("git index-pack ...:", b.String())
		// found and unpacked - done
		// TODO(cryptix): somehow git doesnt checkout now..?
		//b.Reset()
		//symRef := exec.Command("git", "symbolic-ref", "HEAD", "ref/heads/master")
		//symRef.Dir = thisGitRepo // GIT_DIR
		//symRef.Stdout = &b
		//symRef.Stderr = &b
		//if err := symRef.Run(); err != nil {
		//	return errgo.Notef(err, "fetchPackedObject: 'git symbolic-ref HEAD ref/heads/master' failed\nOutput: %s", b.String())
		//}
		return nil
	}
	return errgo.Newf("did not find sha1<%s> in %d index files", sha1, len(indexes))
}
