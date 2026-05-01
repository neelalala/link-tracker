package validation

import (
	"errors"
	"fmt"
	"strings"
)

type Validator interface {
	Validate() Problems
}

type Problems map[string]string

func (p Problems) Add(field, msg string) {
	p[field] = msg
}

func (p Problems) String() string {
	if len(p) == 0 {
		return ""
	}
	msgs := make([]string, 0, len(p))
	for field, msg := range p {
		msgs = append(msgs, fmt.Sprintf("%s: %s", field, msg))
	}
	return strings.Join(msgs, "; ")
}

func Check(v Validator) error {
	if v == nil {
		return errors.New("value is nil, cannot validate")
	}
	problems := v.Validate()
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("validation failed: %s",
		problems.String(),
	)
}
