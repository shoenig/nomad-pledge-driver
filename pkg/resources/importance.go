package resources

import (
	"fmt"
	"strings"
)

// Importance maps easy to use labels to Linux kernel nice-ness values.
//
// highest -> -20
// high    -> -10
// normal  -> -5
// low     ->  10
// lowest  ->  19
type Importance struct {
	Label string
	Nice  int
}

func (s *Importance) String() string {
	return fmt.Sprintf("(%s %d)", s.Label, s.Nice)
}

func ParseImportance(s string) (*Importance, error) {
	label := strings.ToLower(s)
	nice := 0
	switch label {
	case "highest":
		nice = -20
	case "high":
		nice = -10
	case "", "normal":
		nice = -5
	case "low":
		nice = 10
	case "lowest":
		nice = 19
	default:
		return nil, fmt.Errorf("importance of %q not recognized", label)
	}
	return &Importance{
		Label: label,
		Nice:  nice,
	}, nil
}
