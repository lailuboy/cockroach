// Copyright 2018 The Cockroach Authors.
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

package engine

import "unsafe"

func nonZeroingMakeByteSlice(len int) []byte {
	ptr := mallocgc(uintptr(len), nil, false)
	return (*[maxArrayLen]byte)(ptr)[:len:len]
}

// Replacement for C.GoBytes which does not zero initialize the returned slice
// before overwriting it.
//
// TODO(peter): Remove when go1.11 is released which has a similar change to
// C.GoBytes.
func gobytes(ptr unsafe.Pointer, len int) []byte {
	if len == 0 {
		return make([]byte, 0)
	}
	x := nonZeroingMakeByteSlice(len)
	src := (*[maxArrayLen]byte)(ptr)[:len:len]
	copy(x, src)
	return x
}
