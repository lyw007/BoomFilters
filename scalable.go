/*
Original work Copyright (c) 2013 zhenjl
Modified work Copyright (c) 2015 Tyler Treat

Permission is hereby granted, free of charge, to any person obtaining a copy of
this software and associated documentation files (the "Software"), to deal in
the Software without restriction, including without limitation the rights to
use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies
of the Software, and to permit persons to whom the Software is furnished to do
so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.
*/

package boom

import "math"

// ScalableBloomFilter implements a Scalable Bloom Filter as described by
// Almeida, Baquero, Preguica, and Hutchison in Scalable Bloom Filters:
//
// http://gsd.di.uminho.pt/members/cbm/ps/dbloom.pdf
//
// A Scalable Bloom Filter dynamically adapts to the number of elements in the
// data set while enforcing a tight upper bound on the false-positive rate.
// This works by adding Bloom filters with geometrically decreasing
// false-positive rates as filters become full. The tightening ratio, r,
// controls the filter growth. The compounded probability over the whole series
// converges to a target value, even accounting for an infinite series.
//
// Scalable Bloom Filters are useful for cases where the size of the data set
// isn't known a priori and memory constraints aren't of particular concern.
// For situations where memory is bounded, consider using Inverse or Stable
// Bloom Filters.
type ScalableBloomFilter struct {
	filters []*PartitionedBloomFilter // filters with geometrically decreasing error rates
	r       float64                   // tightening ratio
	fp      float64                   // target false-positive rate
	p       float64                   // partition fill ratio
	hint    uint                      // filter size hint
}

// NewScalableBloomFilter creates a new Scalable Bloom Filter with the
// specified target false-positive rate and tightening ratio. Use
// NewDefaultScalableBloomFilter if you don't want to calculate these
// parameters.
func NewScalableBloomFilter(hint uint, fpRate, r float64) *ScalableBloomFilter {
	s := &ScalableBloomFilter{
		filters: make([]*PartitionedBloomFilter, 0, 1),
		r:       r,
		fp:      fpRate,
		p:       fillRatio,
		hint:    hint,
	}

	s.addFilter()
	return s
}

// NewDefaultScalableBloomFilter creates a new Scalable Bloom Filter with the
// specified target false-positive rate and an optimal tightening ratio.
func NewDefaultScalableBloomFilter(fpRate float64) *ScalableBloomFilter {
	return NewScalableBloomFilter(10000, fpRate, 0.8)
}

// Capacity returns the current Scalable Bloom Filter capacity, which is the
// sum of the capacities for the contained series of Bloom filters.
func (s *ScalableBloomFilter) Capacity() uint {
	capacity := uint(0)
	for _, bf := range s.filters {
		capacity += bf.Capacity()
	}
	return capacity
}

// K returns the number of hash functions used in each Bloom filter.
func (s *ScalableBloomFilter) K() uint {
	// K is the same across every filter.
	return s.filters[0].K()
}

// FillRatio returns the average ratio of set bits across every filter.
func (s *ScalableBloomFilter) FillRatio() float64 {
	sum := 0.0
	for _, filter := range s.filters {
		sum += filter.FillRatio()
	}
	return sum / float64(len(s.filters))
}

// Test will test for membership of the data and returns true if it is a
// member, false if not. This is a probabilistic test, meaning there is a
// non-zero probability of false positives but a zero probability of false
// negatives.
func (s *ScalableBloomFilter) Test(data []byte) bool {
	// Querying is made by testing for the presence in each filter.
	for _, bf := range s.filters {
		if bf.Test(data) {
			return true
		}
	}

	return false
}

// Add will add the data to the Bloom filter. It returns the filter to allow
// for chaining.
func (s *ScalableBloomFilter) Add(data []byte) *ScalableBloomFilter {
	idx := len(s.filters) - 1

	// If the last filter has reached its fill ratio, add a new one.
	if s.filters[idx].EstimatedFillRatio() >= s.p {
		s.addFilter()
		idx++
	}

	s.filters[idx].Add(data)
	return s
}

// TestAndAdd is equivalent to calling Test followed by Add. It returns true if
// the data is a member, false if not.
func (s *ScalableBloomFilter) TestAndAdd(data []byte) bool {
	member := s.Test(data)
	s.Add(data)
	return member
}

// Reset restores the Bloom filter to its original state. It returns the filter
// to allow for chaining.
func (s *ScalableBloomFilter) Reset() *ScalableBloomFilter {
	s.filters = make([]*PartitionedBloomFilter, 0, 1)
	s.addFilter()
	return s
}

// addFilter adds a new Bloom filter with a restricted false-positive rate to
// the Scalable Bloom Filter
func (s *ScalableBloomFilter) addFilter() {
	fpRate := s.fp * math.Pow(s.r, float64(len(s.filters)))
	s.filters = append(s.filters, NewPartitionedBloomFilter(s.hint, fpRate))
}
