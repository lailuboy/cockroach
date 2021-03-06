// Copyright 2017 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License included
// in the file licenses/BSL.txt and at www.mariadb.com/bsl11.
//
// Change Date: 2022-10-01
//
// On the date above, in accordance with the Business Source License, use
// of this software will be governed by the Apache License, Version 2.0,
// included in the file licenses/APL.txt and at
// https://www.apache.org/licenses/LICENSE-2.0

package scrub

import "fmt"

const (
	// MissingIndexEntryError occurs when a primary k/v is missing a
	// corresponding secondary index k/v.
	MissingIndexEntryError = "missing_index_entry"
	// DanglingIndexReferenceError occurs when a secondary index k/v
	// points to a non-existing primary k/v.
	DanglingIndexReferenceError = "dangling_index_reference"
	// PhysicalError is a generic error when there is an error in the
	// SQL physical data.
	PhysicalError = "physical_error"
	// IndexKeyDecodingError occurs while decoding the an index key.
	IndexKeyDecodingError = "index_key_decoding_error"
	// IndexValueDecodingError occurs while decoding the an index value.
	IndexValueDecodingError = "index_value_decoding_error"
	// SecondaryIndexKeyExtraValueDecodingError occurs when the extra
	// columns stored in a key fail to decode.
	SecondaryIndexKeyExtraValueDecodingError = "secondary_index_key_extra_value_decoding_error"
	// UnexpectedNullValueError occurs when a null value is encountered where the
	// value is expected to be non-nullable.
	UnexpectedNullValueError = "null_value_error"
	// CheckConstraintViolation occurs when a row in a table is
	// violating a check constraint.
	CheckConstraintViolation = "check_constraint_violation"
	// ForeignKeyConstraintViolation occurs when a row in a
	// table is violating a foreign key constraint.
	ForeignKeyConstraintViolation = "foreign_key_violation"
)

// Error contains the details on the scrub error that was caught.
type Error struct {
	Code       string
	underlying error
}

func (s *Error) Error() string {
	return fmt.Sprintf("%s: %+v", s.Code, s.underlying)
}

// WrapError wraps an error with a Error.
func WrapError(code string, err error) *Error {
	return &Error{
		Code:       code,
		underlying: err,
	}
}

// IsScrubError checks if an error is a Error.
func IsScrubError(err error) bool {
	_, ok := err.(*Error)
	return ok
}

// UnwrapScrubError gets the underlying error if err is a scrub.Error.
// If err is not a scrub.Error nil is returned.
func UnwrapScrubError(err error) error {
	if IsScrubError(err) {
		return err.(*Error).underlying
	}
	return err
}
