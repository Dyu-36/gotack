package recall

import "errors"

var ErrSchemaMismatch = errors.New("recall: crush.db schema does not match the recall contract")

var ErrSourceMissing = errors.New("recall: crush.db not found in data directory")

var ErrInvalidQuery = errors.New("recall: search query contains no searchable words")

var ErrUnknownSession = errors.New("recall: session not found in the recall index")

var ErrUnknownMessage = errors.New("recall: message anchor not found")
