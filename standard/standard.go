/*
 * Copyright (c) 2013 Zhen, LLC. http://zhen.io. All rights reserved.
 * Use of this source code is governed by the MIT license.
 *
 */

package standard

import (
	"hash/fnv"
	"hash"
	"fmt"

	"github.com/willf/bitset"
	"github.com/zhenjl/bloom"
	"encoding/binary"
	"math"
)

// StandardBloom is the classic bloom filter implementation
type StandardBloom struct {
	// h is the hash function used to get the list of h1..hk values
	// By default we use hash/fnv.New64(). User can also set their own using SetHasher()
	h hash.Hash

	// m is the total number of bits for this bloom filter. m for the partitioned bloom filter
	// will be divided into k partitions, or slices. So each partition contains Math.ceil(m/k) bits.
	//
	// m =~ n / ((log(p)*log(1-p))/abs(log e))
	m uint

	// k is the number of hash values used to set and test bits. Each filter partition will be
	// set/tested using a single hash value. Note that the number of hash functions may not be the
	// same as hash values. For example, our implementation uses 32-bit hash values. So a single
	// Murmur3 128bit hash function can be used as 4 32-bit hash values. A single FNV 64bit hash function
	// can be used as 2 32-bit has values.
	//
	// k = log2(1/e)
	// Given that our e is defaulted to 0.001, therefore k ~= 10, which means we need 10 hash values
	k uint

	// s is the size of the partition, or slice.
	// s = m / k
	s uint

	// p is the fill ratio of the filter partitions. It's mainly used to calculate m at the start.
	// p is not checked when new items are added. So if the fill ratio goes above p, the likelihood
	// of false positives (error rate) will increase.
	//
	// By default we use the fill ratio of p = 0.5
	p float64

	// e is the desired error rate of the bloom filter. The lower the e, the higher the k.
	//
	// By default we use the error rate of e = 0.1% = 0.001. In some papers this is P (uppercase P)
	e float64

	// n is the number of elements the filter is predicted to hold while maintaining the error rate
	// or filter size (m). n is user supplied. But, in case you are interested, the formula is
	// n =~ m * ( (log(p) * log(1-p)) / abs(log e) )
	n uint

	// b is the set of bit array holding the bloom filters. There will be k b's.
	b *bitset.BitSet

	// c is the number of items we have added to the filter
	c uint
}

var _ bloom.Bloom = (*StandardBloom)(nil)

// New initializes a new partitioned bloom filter.
// n is the number of items this bloom filter predicted to hold.
func New(n uint) bloom.Bloom {
	var (
		p float64 = 0.5
		e float64 = 0.001
		k uint = bloom.K(e)
		m uint = bloom.M(n, p, e)
	)

	return &StandardBloom{
		h: fnv.New64(),
		n: n,
		p: p,
		e: e,
		k: k,
		m: m,
		b: bitset.New(m),
	}
}

func (this *StandardBloom) SetHasher(h hash.Hash) {
	this.h = h
}

func (this *StandardBloom) Reset() {
	this.k = bloom.K(this.e)
	this.m = bloom.M(this.n, this.p, this.e)
	this.b = bitset.New(this.m)

	if this.h == nil {
		this.h = fnv.New64()
	} else {
		this.h.Reset()
	}
}

func (this *StandardBloom) SetErrorProbability(e float64) {
	this.e = e
}

func (this *StandardBloom) EstimatedFillRatio() float64 {
	return 1-math.Exp(-float64(this.c)/float64(this.m))
}

func (this *StandardBloom) FillRatio() float64 {
	return float64(this.b.Count())/float64(this.m)
}

func (this *StandardBloom) Add(item []byte) bloom.Bloom {
	bs := this.bits(item)
	for i := uint(0); i < this.k; i++ {
		this.b.Set(bs[i])
	}
	this.c++
	return this
}

func (this *StandardBloom) Check(item []byte) bool {
	bs := this.bits(item)
	for i := uint(0); i < this.k; i++ {
		if !this.b.Test(bs[i]) {
			return false
		}
	}
	return true
}

func (this *StandardBloom) Count() uint {
	return this.c
}

func (this *StandardBloom) PrintStats() {
	fmt.Printf("m = %d, n = %d, k = %d, s = %d, p = %f, e = %f\n", this.m, this.n, this.k, this.s, this.p, this.e)
	fmt.Println("Total items:", this.c)
	c := this.b.Count()
	fmt.Printf("Total bits set: %d (%.1f%%)\n", c, float32(c)/float32(this.m)*100)
}


func (this *StandardBloom) bits(item []byte) []uint {
	this.h.Reset()
	this.h.Write(item)
	s := this.h.Sum(nil)
	a := binary.BigEndian.Uint32(s[4:8])
	b := binary.BigEndian.Uint32(s[0:4])
	bs := make([]uint, this.k)

	// Reference: Less Hashing, Same Performance: Building a Better Bloom Filter
	// URL: http://www.eecs.harvard.edu/~kirsch/pubs/bbbf/rsa.pdf
	for i := uint(0); i < this.k; i++ {
		bs[i] = (uint(a) + uint(b)*i) % this.m
	}

	return bs
}
