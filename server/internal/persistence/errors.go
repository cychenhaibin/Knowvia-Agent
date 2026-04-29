package persistence

import "errors"

var ErrNotFound = errors.New("persistence: not found")
var ErrConflict = errors.New("persistence: conflict")
