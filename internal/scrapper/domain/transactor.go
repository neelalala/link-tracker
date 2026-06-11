package domain

import (
	"context"
)

type Transactor interface {
	WithinTransaction(context.Context, func(context.Context) error) error
}
