// Copyright 2019 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package coldata

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// nulls3 is a nulls vector with every third value set to null.
var nulls3 Nulls

// nulls5 is a nulls vector with every fifth value set to null.
var nulls5 Nulls

// nulls10 is a double-length nulls vector with every tenth value set to null.
var nulls10 Nulls

// pos is a collection of interesting boundary indices to use in tests.
var pos = []uint64{0, 1, 63, 64, 65, BatchSize - 1, BatchSize}

func init() {
	nulls3 = NewNulls(BatchSize)
	nulls5 = NewNulls(BatchSize)
	nulls10 = NewNulls(BatchSize * 2)
	for i := uint16(0); i < BatchSize; i++ {
		if i%3 == 0 {
			nulls3.SetNull(i)
		}
		if i%5 == 0 {
			nulls5.SetNull(i)
		}
	}
	for i := uint16(0); i < BatchSize*2; i++ {
		if i%10 == 0 {
			nulls10.SetNull(i)
		}
	}
}

func TestNullAt(t *testing.T) {
	for i := uint16(0); i < BatchSize; i++ {
		if i%3 == 0 {
			require.True(t, nulls3.NullAt(i))
		} else {
			require.False(t, nulls3.NullAt(i))
		}
	}
}

func TestSetNullRange(t *testing.T) {
	for _, start := range pos {
		for _, end := range pos {
			n := NewNulls(BatchSize)
			n.SetNullRange(start, end)
			for i := uint64(0); i < BatchSize; i++ {
				expected := i >= start && i < end
				require.Equal(t, expected, n.NullAt64(i),
					"NullAt(%d) should be %t after SetNullRange(%d, %d)", i, expected, start, end)
			}
		}
	}
}

func TestNullsTruncate(t *testing.T) {
	for _, size := range pos {
		n := NewNulls(BatchSize)
		n.Truncate(uint16(size))
		for i := uint16(0); i < BatchSize; i++ {
			expected := uint64(i) >= size
			require.Equal(t, expected, n.NullAt(i),
				"NullAt(%d) should be %t after Truncate(%d)", i, expected, size)
		}
	}
}

func TestSetAndUnsetNulls(t *testing.T) {
	n := NewNulls(BatchSize)
	for i := uint16(0); i < BatchSize; i++ {
		require.False(t, n.NullAt(i))
	}
	n.SetNulls()
	for i := uint16(0); i < BatchSize; i++ {
		require.True(t, n.NullAt(i))
	}
	n.UnsetNulls()
	for i := uint16(0); i < BatchSize; i++ {
		require.False(t, n.NullAt(i))
	}
}

func TestExtend(t *testing.T) {
	for _, destStartIdx := range pos {
		for _, srcStartIdx := range pos {
			for _, srcEndIdx := range pos {
				if destStartIdx <= srcStartIdx && srcStartIdx <= srcEndIdx {
					toAppend := srcEndIdx - srcStartIdx
					name := fmt.Sprintf("destStartIdx=%d,srcStartIdx=%d,toAppend=%d", destStartIdx,
						srcStartIdx, toAppend)
					t.Run(name, func(t *testing.T) {
						n := nulls3.Copy()
						n.Extend(&nulls5, destStartIdx, uint16(srcStartIdx), uint16(toAppend))
						for i := uint64(0); i < destStartIdx; i++ {
							require.Equal(t, nulls3.NullAt64(i), n.NullAt64(i))
						}
						for i := uint64(0); i < toAppend; i++ {
							// TODO(solon): Arguably the null value should also be false if the source
							// value was false, but that's not how the current implementation works.
							// Fix this when we replace it with a faster bitwise implementation.
							if nulls5.NullAt64(srcStartIdx + i) {
								destIdx := destStartIdx + i
								require.True(t, n.NullAt64(destIdx),
									"n.NullAt64(%d) should be true", destIdx)
							}
						}
					})
				}
			}
		}
	}
}

func TestExtendSel(t *testing.T) {
	// Make a selection vector with every even index. (This turns nulls10 into
	// nulls5.)
	sel := make([]uint16, BatchSize)
	for i := range sel {
		sel[i] = uint16(i) * 2
	}

	for _, destStartIdx := range pos {
		for _, srcStartIdx := range pos {
			for _, srcEndIdx := range pos {
				if destStartIdx <= srcStartIdx && srcStartIdx <= srcEndIdx {
					toAppend := srcEndIdx - srcStartIdx
					name := fmt.Sprintf("destStartIdx=%d,srcStartIdx=%d,toAppend=%d", destStartIdx,
						srcStartIdx, toAppend)
					t.Run(name, func(t *testing.T) {
						n := nulls3.Copy()
						n.ExtendWithSel(&nulls10, destStartIdx, uint16(srcStartIdx), uint16(toAppend), sel)
						for i := uint64(0); i < destStartIdx; i++ {
							require.Equal(t, nulls3.NullAt64(i), n.NullAt64(i))
						}
						for i := uint64(0); i < toAppend; i++ {
							// TODO(solon): Arguably the null value should also be false if the source
							// value was false, but that's not how the current implementation works.
							// Fix this when we replace it with a faster bitwise implementation.
							if nulls5.NullAt64(srcStartIdx + i) {
								destIdx := destStartIdx + i
								require.True(t, n.NullAt64(destIdx),
									"n.NullAt64(%d) should be true", destIdx)
							}
						}
					})
				}
			}
		}
	}
}

func TestSlice(t *testing.T) {
	for _, start := range pos {
		for _, end := range pos {
			n := nulls3.Slice(start, end)
			for i := uint64(0); i < uint64(8*len(n.nulls)); i++ {
				expected := start+i < end && nulls3.NullAt64(start+i)
				require.Equal(t, expected, n.NullAt64(i),
					"expected nulls3.Slice(%d, %d).NullAt(%d) to be %b", start, end, i, expected)
			}
		}
	}
	// Ensure we haven't modified the receiver.
	for i := uint16(0); i < BatchSize; i++ {
		expected := i%3 == 0
		require.Equal(t, expected, nulls3.NullAt(i))
	}
}

func TestNullsOr(t *testing.T) {
	length1, length2 := uint64(300), uint64(400)
	n1 := nulls3.Slice(0, length1)
	n2 := nulls5.Slice(0, length2)
	or := n1.Or(&n2)
	require.True(t, or.hasNulls)
	for i := uint64(0); i < length2; i++ {
		if i < length1 && n1.NullAt64(i) || i < length2 && n2.NullAt64(i) {
			require.True(t, or.NullAt64(i), "or.NullAt64(%d) should be true", i)
		} else {
			require.False(t, or.NullAt64(i), "or.NullAt64(%d) should be false", i)
		}
	}
}
