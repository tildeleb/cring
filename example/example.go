// Copyright © 2016,2017, 2019 Lawrence E. Bakst. All rights reserved.

// Send ints from a producer to a consumer and verify monotonicity.
// Uses ring buffer or channel.

package main

import (
	"flag"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"leb.io/hrff"
	"leb.io/ring"
	_ "leb.io/stats"
)

var n = flag.Int("n", 10*1000*1000, "n")
var s = flag.Int("s", 10, "ring size power of 2 exponent")
var iles = flag.Int("iles", 10, "how many buckets for histogram")
var cf = flag.Bool("cf", false, "chans not concurrent ring")
var lf = flag.Bool("lf", false, "benchmark loads per second")
var dc = flag.Bool("dc", false, "do not check numbers")
var alf = flag.Bool("alf", false, "benchmark atomic loads per second")
var pstats []float64
var cstats []float64

var wg sync.WaitGroup

func tdiff(begin, end time.Time) time.Duration {
	d := end.Sub(begin)
	return d
}

func producer(r *ring.Ring, n int) {
	runtime.LockOSThread()
	for i := 0; i < n; i++ {
	retry:
		if r.Put(i) {
			//fmt.Printf("pR ")
			goto retry
		}
		//pstats[i] = float64(r.Pcnt)
		//fmt.Printf("p=%d\n", i)
	}
	wg.Done()
}

func consumer(r *ring.Ring, n int) {
	var cnt int
	runtime.LockOSThread()
	for i := 0; i < n; i++ {
	retry:
		v, b := r.Get()
		if b {
			//fmt.Printf("cR=%v\n", cr)
			goto retry
		}
		if !*dc && v != cnt {
			fmt.Printf("v=%d, cnt=%d\n", v, cnt)
			panic("consumer")
		}
		//fmt.Printf("c=%d\n", v)
		cnt++
		//cstats[i] = float64(r.Gcnt)
	}
	wg.Done()
}

func chanProducer(c chan int, n int) {
	for i := 0; i < n; i++ {
		c <- i
		//fmt.Printf("p=%d\n", i)
	}
	wg.Done()
}

func chanConsumer(c chan int, n int) {
	var cnt int
	for i := 0; i < n; i++ {
		v, ok := <-c
		if !ok {
			//fmt.Printf("cR=%v\n", cr)
			return
		}
		if !*dc && v != cnt {
			fmt.Printf("v=%d, cnt=%d\n", v, cnt)
			panic("chanConsumer")
		}
		//fmt.Printf("c=%d\n", v)
		cnt++
	}
	wg.Done()
}

var ui uint64 = 3

// 64 bit loads per second
func lps(n int) uint64 {
	var totui uint64

	p := &ui
	for i := 0; i < n/10; i++ {
		totui += *p
		totui += *p
		totui += *p
		totui += *p
		totui += *p
		totui += *p
		totui += *p
		totui += *p
		totui += *p
		totui += *p
	}
	return totui
}

// atomic 64 bit loads per second
func alps(n int) uint64 {
	var totui uint64

	p := &ui
	for i := 0; i < n/10; i++ {
		totui += atomic.LoadUint64(p)
		totui += atomic.LoadUint64(p)
		totui += atomic.LoadUint64(p)
		totui += atomic.LoadUint64(p)
		totui += atomic.LoadUint64(p)
		totui += atomic.LoadUint64(p)
		totui += atomic.LoadUint64(p)
		totui += atomic.LoadUint64(p)
		totui += atomic.LoadUint64(p)
		totui += atomic.LoadUint64(p)
	}
	return totui
}

func basiTest() {
	r := ring.New(*s)
	fmt.Printf("size=%d\n", unsafe.Sizeof(r))
	r.Put(1)
	r.Put(2)
	r.Put(3)
	r.Put(4)
	for i := 0; i <= 4; i++ {
		v, b := r.Get()
		fmt.Printf("b=%v, v=%v\n", b, v)
	}
}

func main() {
	flag.Parse()
	if *lf {
		t := lps(*n)
		fmt.Printf("%d\n", t)
		return
	}
	if *alf {
		t := alps(*n)
		fmt.Printf("%d\n", t)
		return
	}
	//pstats = make([]float64, *n)
	//cstats = make([]float64, *n)
	wg.Add(2)
	r := ring.New(*s)
	start := time.Now()
	if *cf {
		c := make(chan int, 1<<uint(*s))
		go chanProducer(c, *n)
		go chanConsumer(c, *n)
	} else {
		//fmt.Printf("size=%d\n", uint16(1<<r.size))
		go producer(r, *n)
		go consumer(r, *n)
	}
	wg.Wait()
	stop := time.Now()
	dur := tdiff(start, stop)
	opsSec := float64(*n) / dur.Seconds() * 1000
	fmt.Printf("%0.2h\n", hrff.Float64{opsSec, "ops/sec"})
	if !*cf {
		fmt.Printf("Ops=%.1h, Put Spins=%.1h, Get Spins=%.1h, Puts Full=%h, Gets Empty=%h\n",
			hrff.Float64{V: float64(*n), U: "ops"},
			hrff.Float64{V: float64(r.PutSpins), U: "spins"}, hrff.Float64{V: float64(r.GetSpins), U: "spins"},
			hrff.Int64{V: r.PutsFull}, hrff.Int64{V: r.GetsEmpty})
		fmt.Printf("SwapsSucceeded=%.1h, SwapsFailed=%.1h\n",
			hrff.Float64{V: float64(r.SwapsSucceeded), U: "stores"}, hrff.Float64{V: float64(r.SwapsFailed), U: "stores"})
	}
	return

}
