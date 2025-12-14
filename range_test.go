package sudp

import (
	"reflect"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRangenotIn(t *testing.T) {
	assert := assert.New(t)

	assert.False(rng[int]{69, 420}.in(0))
	assert.False(rng[int]{69, 420}.in(5))
	assert.False(rng[int]{69, 420}.in(68))

	assert.True(rng[int]{69, 420}.in(69))
	assert.True(rng[int]{69, 420}.in(70))
	assert.True(rng[int]{69, 420}.in(128))
	assert.True(rng[int]{69, 420}.in(419))
	assert.True(rng[int]{69, 420}.in(420))

	assert.False(rng[int]{69, 420}.in(421))
	assert.False(rng[int]{69, 420}.in(999))
}

func TestRangesTryAppend(t *testing.T) {
	t.Run("Is notIn ranges", func(t *testing.T) {
		assert := assert.New(t)
		ranges := []rng[int]{{0, 3}, {5, 5}, {7, 8}, {12, 14}}

		_, notIn := rangesTryAppend(slices.Clone(ranges), -1)
		assert.True(notIn)

		_, notIn = rangesTryAppend(slices.Clone(ranges), 0)
		assert.False(notIn)
		_, notIn = rangesTryAppend(slices.Clone(ranges), 1)
		assert.False(notIn)
		_, notIn = rangesTryAppend(slices.Clone(ranges), 2)
		assert.False(notIn)
		_, notIn = rangesTryAppend(slices.Clone(ranges), 3)
		assert.False(notIn)

		_, notIn = rangesTryAppend(slices.Clone(ranges), 4)
		assert.True(notIn)

		_, notIn = rangesTryAppend(slices.Clone(ranges), 5)
		assert.False(notIn)

		_, notIn = rangesTryAppend(slices.Clone(ranges), 6)
		assert.True(notIn)

		_, notIn = rangesTryAppend(slices.Clone(ranges), 7)
		assert.False(notIn)
		_, notIn = rangesTryAppend(slices.Clone(ranges), 8)
		assert.False(notIn)

		_, notIn = rangesTryAppend(slices.Clone(ranges), 9)
		assert.True(notIn)
		_, notIn = rangesTryAppend(slices.Clone(ranges), 10)
		assert.True(notIn)
		_, notIn = rangesTryAppend(slices.Clone(ranges), 11)
		assert.True(notIn)

		_, notIn = rangesTryAppend(slices.Clone(ranges), 12)
		assert.False(notIn)
		_, notIn = rangesTryAppend(slices.Clone(ranges), 13)
		assert.False(notIn)
		_, notIn = rangesTryAppend(slices.Clone(ranges), 14)
		assert.False(notIn)

		_, notIn = rangesTryAppend(slices.Clone(ranges), 15)
		assert.True(notIn)
	})

	t.Run("Appending", func(t *testing.T) {
		t.Run("Return same slice if value already in", func(t *testing.T) {
			ranges := []rng[int]{{0, 3}, {5, 5}}
			res, _ := rangesTryAppend(ranges, 5)
			assert.Equal(t, ranges, res)
			assert.Equal(t, reflect.ValueOf(ranges).Pointer(), reflect.ValueOf(res).Pointer())
		})

		t.Run("Empty slice", func(t *testing.T) {
			res, _ := rangesTryAppend(nil, 100)
			assert.Equal(t, []rng[int]{{100, 100}}, res)

			res, _ = rangesTryAppend([]rng[int]{}, 500)
			assert.Equal(t, []rng[int]{{500, 500}}, res)
		})

		t.Run("Single element slice", func(t *testing.T) {
			res, _ := rangesTryAppend([]rng[int]{{69, 420}}, 3)
			assert.Equal(t, []rng[int]{{3, 3}, {69, 420}}, res)

			res, _ = rangesTryAppend([]rng[int]{{69, 420}}, 500)
			assert.Equal(t, []rng[int]{{69, 420}, {500, 500}}, res)
		})

		t.Run("At the beginning of slice before first", func(t *testing.T) {
			res, _ := rangesTryAppend([]rng[int]{{3, 7}, {9, 10}}, 1)
			assert.Equal(t, []rng[int]{{1, 1}, {3, 7}, {9, 10}}, res)

			res, _ = rangesTryAppend([]rng[int]{{3, 7}, {9, 10}, {69, 420}}, 1)
			assert.Equal(t, []rng[int]{{1, 1}, {3, 7}, {9, 10}, {69, 420}}, res)
		})

		t.Run("At the beginning of slice in first front", func(t *testing.T) {
			res, _ := rangesTryAppend([]rng[int]{{3, 7}, {9, 10}}, 2)
			assert.Equal(t, []rng[int]{{2, 7}, {9, 10}}, res)

			res, _ = rangesTryAppend([]rng[int]{{3, 7}, {9, 10}, {69, 420}}, 2)
			assert.Equal(t, []rng[int]{{2, 7}, {9, 10}, {69, 420}}, res)
		})

		t.Run("At the beginning of slice in first back", func(t *testing.T) {
			res, _ := rangesTryAppend([]rng[int]{{3, 7}, {11, 14}}, 8)
			assert.Equal(t, []rng[int]{{3, 8}, {11, 14}}, res)

			res, _ = rangesTryAppend([]rng[int]{{0, 1}, {3, 7}, {11, 14}, {69, 420}}, 8)
			assert.Equal(t, []rng[int]{{0, 1}, {3, 8}, {11, 14}, {69, 420}}, res)
		})

		t.Run("In the middle of slice without concatenation", func(t *testing.T) {
			res, _ := rangesTryAppend([]rng[int]{{3, 7}, {11, 14}}, 9)
			assert.Equal(t, []rng[int]{{3, 7}, {9, 9}, {11, 14}}, res)

			res, _ = rangesTryAppend([]rng[int]{{0, 1}, {3, 7}, {11, 14}, {69, 420}}, 9)
			assert.Equal(t, []rng[int]{{0, 1}, {3, 7}, {9, 9}, {11, 14}, {69, 420}}, res)
		})

		t.Run("In the middle of slice with concatenation", func(t *testing.T) {
			res, _ := rangesTryAppend([]rng[int]{{3, 7}, {9, 10}}, 8)
			assert.Equal(t, []rng[int]{{3, 10}}, res)

			res, _ = rangesTryAppend([]rng[int]{{0, 1}, {3, 7}, {9, 10}, {69, 420}}, 8)
			assert.Equal(t, []rng[int]{{0, 1}, {3, 10}, {69, 420}}, res)
		})

		t.Run("At the end of slice in last back", func(t *testing.T) {
			res, _ := rangesTryAppend([]rng[int]{{3, 7}, {9, 10}}, 11)
			assert.Equal(t, []rng[int]{{3, 7}, {9, 11}}, res)

			res, _ = rangesTryAppend([]rng[int]{{0, 1}, {3, 7}, {9, 10}}, 11)
			assert.Equal(t, []rng[int]{{0, 1}, {3, 7}, {9, 11}}, res)
		})

		t.Run("At the end of slice after last", func(t *testing.T) {
			res, _ := rangesTryAppend([]rng[int]{{3, 7}, {9, 10}}, 12)
			assert.Equal(t, []rng[int]{{3, 7}, {9, 10}, {12, 12}}, res)

			res, _ = rangesTryAppend([]rng[int]{{0, 1}, {3, 7}, {9, 10}}, 12)
			assert.Equal(t, []rng[int]{{0, 1}, {3, 7}, {9, 10}, {12, 12}}, res)
		})
	})
}
