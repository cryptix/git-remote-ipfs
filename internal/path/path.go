package path

import (
	"errors"
	"path"
	"strings"

	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
)

// ErrBadPath is returned when a given path is incorrectly formatted
var ErrBadPath = errors.New("invalid ipfs ref path")

// TODO: debate making this a private struct wrapped in a public interface
// would allow us to control creation, and cache segments.
type Path string

// FromString safely converts a string type to a Path type
func FromString(s string) Path {
	return Path(s)
}

func (p Path) Segments() []string {
	cleaned := path.Clean(string(p))
	segments := strings.Split(cleaned, "/")

	// Ignore leading slash
	if len(segments[0]) == 0 {
		segments = segments[1:]
	}

	return segments
}

func (p Path) String() string {
	return string(p)
}

// FromCid safely converts a cid.Cid type to a Path type
func FromCid(c *cid.Cid) Path {
	return Path("/ipfs/" + c.String())
}
func FromSegments(prefix string, seg ...string) (Path, error) {
	return ParsePath(prefix + strings.Join(seg, "/"))
}

func ParsePath(txt string) (Path, error) {
	parts := strings.Split(txt, "/")
	if len(parts) == 1 {
		kp, err := ParseCidToPath(txt)
		if err == nil {
			return kp, nil
		}
	}

	// if the path doesnt being with a '/'
	// we expect this to start with a hash, and be an 'ipfs' path
	if parts[0] != "" {
		if _, err := ParseCidToPath(parts[0]); err != nil {
			return "", ErrBadPath
		}
		// The case when the path starts with hash without a protocol prefix
		return Path("/ipfs/" + txt), nil
	}

	if len(parts) < 3 {
		return "", ErrBadPath
	}

	if parts[1] == "ipfs" {
		if _, err := ParseCidToPath(parts[2]); err != nil {
			return "", err
		}
	} else if parts[1] != "ipns" {
		return "", ErrBadPath
	}

	return Path(txt), nil
}

func ParseCidToPath(txt string) (Path, error) {
	if txt == "" {
		return "", ErrNoComponents
	}

	c, err := cid.Decode(txt)
	if err != nil {
		return "", err
	}

	return FromCid(&c), nil
}

func (p *Path) IsValid() error {
	_, err := ParsePath(p.String())
	return err
}

// Paths after a protocol must contain at least one component
var ErrNoComponents = errors.New(
	"path must contain at least one component")

// ErrNoLink is returned when a link is not found in a path
type ErrNoLink struct {
	Name string
	Node mh.Multihash
}
