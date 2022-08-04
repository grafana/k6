package lib

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/lib/types"
)

// Separator for group IDs.
const GroupSeparator = "::"

// Error emitted if you attempt to instantiate a Group or Check that contains the separator.
var ErrNameContainsGroupSeparator = errors.New("group and check names may not contain '::'")

// StageFields defines the fields used for a Stage; this is a dumb hack to make the JSON code
// cleaner. pls fix.
type StageFields struct {
	// Duration of the stage.
	Duration types.NullDuration `json:"duration"`

	// If Valid, the VU count will be linearly interpolated towards this value.
	Target null.Int `json:"target"`
}

// A Stage defines a step in a test's timeline.
type Stage StageFields

// For some reason, implementing UnmarshalText makes encoding/json treat the type as a string.
func (s *Stage) UnmarshalJSON(b []byte) error {
	var fields StageFields
	if err := json.Unmarshal(b, &fields); err != nil {
		return err
	}
	*s = Stage(fields)
	return nil
}

func (s Stage) MarshalJSON() ([]byte, error) {
	return json.Marshal(StageFields(s))
}

func (s *Stage) UnmarshalText(b []byte) error {
	var stage Stage
	parts := strings.SplitN(string(b), ":", 2)
	if len(parts) > 0 && parts[0] != "" {
		d, err := time.ParseDuration(parts[0])
		if err != nil {
			return err
		}
		stage.Duration = types.NullDurationFrom(d)
	}
	if len(parts) > 1 && parts[1] != "" {
		t, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return err
		}
		stage.Target = null.IntFrom(t)
	}
	*s = stage
	return nil
}

// A Group is an organisational block, that samples and checks may be tagged with.
//
// For more information, refer to the js/modules/k6.K6.Group() function.
type Group struct {
	// Arbitrary name of the group.
	Name string `json:"name"`

	// A group may belong to another group, which may belong to another group, etc. The Path
	// describes the hierarchy leading down to this group, with the segments delimited by '::'.
	// As an example: a group "Inner" inside a group named "Outer" would have a path of
	// "::Outer::Inner". The empty first item is the root group, which is always named "".
	Parent *Group `json:"-"`
	Path   string `json:"path"`

	// A group's ID is a hash of the Path. It is deterministic between different k6
	// instances of the same version, but should be treated as opaque - the hash function
	// or length may change.
	ID string `json:"id"`

	// Groups and checks that are children of this group.
	Groups        map[string]*Group `json:"groups"`
	OrderedGroups []*Group          `json:"-"`

	Checks        map[string]*Check `json:"checks"`
	OrderedChecks []*Check          `json:"-"`

	groupMutex sync.Mutex
	checkMutex sync.Mutex
}

// Creates a new group with the given name and parent group.
//
// The root group must be created with the name "" and parent set to nil; this is the only case
// where a nil parent or empty name is allowed.
func NewGroup(name string, parent *Group) (*Group, error) {
	if strings.Contains(name, GroupSeparator) {
		return nil, ErrNameContainsGroupSeparator
	}

	path := name
	if parent != nil {
		path = parent.Path + GroupSeparator + path
	}

	hash := md5.Sum([]byte(path))
	id := hex.EncodeToString(hash[:])

	return &Group{
		ID:     id,
		Path:   path,
		Name:   name,
		Parent: parent,
		Groups: make(map[string]*Group),
		Checks: make(map[string]*Check),
	}, nil
}

// Group creates a child group belonging to this group.
// This is safe to call from multiple goroutines simultaneously.
func (g *Group) Group(name string) (*Group, error) {
	g.groupMutex.Lock()
	defer g.groupMutex.Unlock()
	group, ok := g.Groups[name]
	if !ok {
		group, err := NewGroup(name, g)
		if err != nil {
			return nil, err
		}
		g.Groups[name] = group
		g.OrderedGroups = append(g.OrderedGroups, group)
		return group, nil
	}
	return group, nil
}

// Check creates a child check belonging to this group.
// This is safe to call from multiple goroutines simultaneously.
func (g *Group) Check(name string) (*Check, error) {
	g.checkMutex.Lock()
	defer g.checkMutex.Unlock()
	check, ok := g.Checks[name]
	if !ok {
		check, err := NewCheck(name, g)
		if err != nil {
			return nil, err
		}
		g.Checks[name] = check
		g.OrderedChecks = append(g.OrderedChecks, check)
		return check, nil
	}
	return check, nil
}

// A Check stores a series of successful or failing tests against a value.
//
// For more information, refer to the js/modules/k6.K6.Check() function.
type Check struct {
	// Arbitrary name of the check.
	Name string `json:"name"`

	// A Check belongs to a Group, which may belong to other groups. The Path describes
	// the hierarchy of these groups, with the segments delimited by '::'.
	// As an example: a check "My Check" within a group "Inner" within a group "Outer"
	// would have a Path of "::Outer::Inner::My Check". The empty first item is the root group,
	// which is always named "".
	Group *Group `json:"-"`
	Path  string `json:"path"`

	// A check's ID is a hash of the Path. It is deterministic between different k6
	// instances of the same version, but should be treated as opaque - the hash function
	// or length may change.
	ID string `json:"id"`

	// Counters for how many times this check has passed and failed respectively.
	Passes int64 `json:"passes"`
	Fails  int64 `json:"fails"`
}

// Creates a new check with the given name and parent group. The group may not be nil.
func NewCheck(name string, group *Group) (*Check, error) {
	if strings.Contains(name, GroupSeparator) {
		return nil, ErrNameContainsGroupSeparator
	}

	path := group.Path + GroupSeparator + name
	hash := md5.Sum([]byte(path))
	id := hex.EncodeToString(hash[:])

	return &Check{
		ID:    id,
		Path:  path,
		Group: group,
		Name:  name,
	}, nil
}
