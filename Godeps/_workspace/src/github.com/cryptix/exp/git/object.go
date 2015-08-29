package git

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"gopkg.in/errgo.v1"
)

type Type int

const (
	_ Type = iota
	BlobT
	TreeT
	CommitT
)

type Object struct {
	Type   Type
	Size   int64
	tree   []Tree
	blob   *Blob
	commit *Commit
}

func (o Object) Tree() ([]Tree, bool) {
	if o.Type != TreeT || o.tree == nil {
		return nil, false
	}
	return o.tree, true
}

func (o Object) Commit() (*Commit, bool) {
	if o.Type != CommitT || o.commit == nil {
		return nil, false
	}
	return o.commit, true
}

func (o Object) Blob() ([]byte, bool) {
	if o.Type != CommitT || o.blob == nil {
		return nil, false
	}
	return *o.blob, true
}

func newBlob(content []byte) *Blob {
	b := Blob(content)
	return &b
}

type Hash [sha1.Size]byte

func (h Hash) String() string {
	return hex.EncodeToString(h[:])
}

type Blob []byte

type Tree struct {
	Mode, Name string
	SHA1Sum    Hash
}

type Stamp struct {
	Name  string
	Email string
	When  time.Time
}

func (s Stamp) String() string {
	return fmt.Sprintf("%s <%s> %s", s.Name, s.Email, s.When.Format(time.RFC3339))
	//return fmt.Sprintf("%q <%q> %d", s.Name, s.Email, s.When.Unix())
}

type Commit struct {
	Author, Committer *Stamp
	Tree, Parent      string
	Message           string
}

func (o Object) String() string {
	switch o.Type {
	case BlobT:
		if o.blob == nil {
			return "broken blob"
		}
		return fmt.Sprintf("blob<%d> %q", o.Size, string(*o.blob))
	case TreeT:
		if o.tree == nil {
			return "broken tree"
		}
		s := fmt.Sprintf("tree<%d>\n", o.Size)
		for _, t := range o.tree {
			s += fmt.Sprintf("%q\t%q\t%s\n", t.Mode, t.Name, t.SHA1Sum)
		}
		return s
	case CommitT:
		if o.commit == nil {
			return "broken commit"
		}
		s := fmt.Sprintf("commit<%d>\n", o.Size)
		s += fmt.Sprintln("Tree:", o.commit.Tree)
		s += fmt.Sprintln("Author:", o.commit.Author)
		s += fmt.Sprintln("Committer:", o.commit.Committer)
		s += o.commit.Message
		return s
	default:
		return "broken object"
	}
}

func DecodeObject(r io.Reader) (*Object, error) {
	zr, err := zlib.NewReader(r)
	if err != nil {
		return nil, errgo.Notef(err, "zlib newReader failed")
	}
	br := bufio.NewReader(zr)
	header, err := br.ReadBytes(0)
	if err != nil {
		return nil, errgo.Notef(err, "error finding header 0byte")
	}
	o := &Object{}
	var hdrLenStr string
	switch {
	case bytes.HasPrefix(header, []byte("blob ")):
		o.Type = BlobT
		hdrLenStr = string(header[5 : len(header)-1])
	case bytes.HasPrefix(header, []byte("tree ")):
		o.Type = TreeT
		hdrLenStr = string(header[5 : len(header)-1])
	case bytes.HasPrefix(header, []byte("commit ")):
		o.Type = CommitT
		hdrLenStr = string(header[7 : len(header)-1])
	default:
		return nil, errgo.Newf("illegal git object:%q", header)
	}
	hdrLen, err := strconv.ParseInt(hdrLenStr, 10, 64)
	if err != nil {
		return nil, errgo.Notef(err, "error parsing header length")
	}
	o.Size = hdrLen
	lr := io.LimitReader(br, hdrLen)
	switch o.Type {
	case BlobT:
		content, err := ioutil.ReadAll(lr)
		if err != nil {
			return nil, errgo.Notef(err, "error finding header 0byte")
		}
		o.blob = newBlob(content)
	case TreeT:
		o.tree, err = decodeTreeEntries(lr)
		if err != nil {
			if errgo.Cause(err) == io.EOF {
				return o, nil
			}
			return nil, errgo.Notef(err, "decodecodeTreeEntries failed")
		}
	case CommitT:
		o.commit, err = decodeCommit(lr)
		if err != nil {
			return nil, errgo.Notef(err, "decodeCommit() failed")
		}
	default:
		return nil, errgo.Newf("illegal object type:%T %v", o.Type, o.Type)
	}
	return o, nil
}

func decodeTreeEntries(r io.Reader) ([]Tree, error) {
	isEOF := errgo.Is(io.EOF)
	var entries []Tree
	br := bufio.NewReader(r)
	for {
		var t Tree
		hdr, err := br.ReadSlice(0)
		if err != nil {
			return entries, errgo.NoteMask(err, "error finding modeName 0byte", isEOF)
		}
		modeName := bytes.Split(hdr[:len(hdr)-1], []byte(" "))
		if len(modeName) != 2 {
			return entries, errgo.Newf("illegal modeName block: %v", modeName)
		}
		t.Mode = string(modeName[0])
		t.Name = string(modeName[1])
		var hash [sha1.Size]byte
		n, err := br.Read(hash[:])
		if err != nil {
			return entries, errgo.NoteMask(err, "br.Read() hash failed", isEOF)
		}
		if n != 20 {
			return entries, errgo.Newf("br.Read() fell short: %d", n)
		}
		t.SHA1Sum = hash
		entries = append(entries, t)
	}
}

func decodeCommit(r io.Reader) (*Commit, error) {
	var (
		s     = bufio.NewScanner(r)
		c     Commit
		isMsg bool
		err   error
	)
	for s.Scan() {
		line := s.Text()
		switch {
		case strings.HasPrefix(line, "tree "):
			c.Tree = line[5:]
		case strings.HasPrefix(line, "parent "):
			c.Parent = line[7:]
		case strings.HasPrefix(line, "author "):
			c.Author, err = decodeStamp(line[7:])
			if err != nil {
				return nil, errgo.Notef(err, "decodeStamp(%q) failed", line[7:])
			}
		case strings.HasPrefix(line, "committer "):
			c.Committer, err = decodeStamp(line[10:])
			if err != nil {
				return nil, errgo.Notef(err, "decodeStamp(%q) failed", line[10:])
			}
		case line == "":
			isMsg = true
		case isMsg:
			c.Message += line
		default:
			return nil, errgo.Newf("unhandled commit line: %q", line)
		}
	}
	if err := s.Err(); err != nil {
		return nil, errgo.Notef(err, "scanner error")
	}
	return &c, nil
}

func decodeStamp(s string) (*Stamp, error) {
	var stamp Stamp
	mailIdxLeft := strings.Index(s, "<")
	if mailIdxLeft == -1 {
		return nil, errgo.New("stamp: no '<' in stamp")
	}
	mailIdxRight := strings.Index(s, ">")
	if mailIdxRight == -1 {
		return nil, errgo.New("stamp: no '>' in stamp")
	}
	if mailIdxLeft > mailIdxRight {
		return nil, errgo.New("stamp: '>' left of '<'")
	}
	if mailIdxLeft == 0 {
		stamp.Name = "empty name"
	} else {
		stamp.Name = s[:mailIdxLeft-1]
	}
	stamp.Email = s[mailIdxLeft+1 : mailIdxRight]
	if len(s)-6-(mailIdxRight+2) < 0 {
		return nil, errgo.Newf("stamp: illegal timestamp: %q", s)
	}
	epoc, err := strconv.ParseInt(s[mailIdxRight+2:len(s)-6], 10, 64)
	if err != nil {
		return nil, errgo.Notef(err, "parseInt() failed")
	}
	when := time.Unix(epoc, 0)
	loc, err := time.Parse("-0700", s[len(s)-5:])
	if err != nil {
		return nil, errgo.Notef(err, "timezone decode failed")
	}
	stamp.When = when.In(loc.Location())
	return &stamp, nil
}
