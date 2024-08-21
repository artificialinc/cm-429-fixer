// Package merge_test implements tests for merge.go
package merge_test

import (
	"sync"
	"testing"

	"github.com/artificialinc/cm-429-fixer/pkg/merge"
	"github.com/go-logr/logr"
)

func TestBools(t *testing.T) {
	type test struct {
		name     string
		expected func(*testing.T, <-chan bool, *sync.WaitGroup)
		inputs   func(*testing.T) ([]<-chan bool, func())
	}

	tests := []test{
		{
			name: "return false if any channel is still false",
			expected: func(_ *testing.T, out <-chan bool, wg *sync.WaitGroup) {
				expects := []bool{false, false, true}
				for _, expect := range expects {
					if <-out != expect {
						t.Errorf("expected %v, got %v", expect, !expect)
					}
				}
				wg.Done()
			},
			inputs: func(*testing.T) ([]<-chan bool, func()) {
				ch1 := make(chan bool)
				ch2 := make(chan bool)
				ch3 := make(chan bool)

				return []<-chan bool{ch1, ch2, ch3}, func() {
					ch1 <- true
					ch2 <- true
					ch3 <- true

					close(ch1)
					close(ch2)
					close(ch3)
				}
			},
		},
		{
			name: "return false if any channel goes false",
			expected: func(t *testing.T, out <-chan bool, wg *sync.WaitGroup) {
				expects := []bool{false, false, true, false}
				for _, expect := range expects {
					if <-out != expect {
						t.Errorf("expected %v, got %v", expect, !expect)
					}
				}
				wg.Done()
			},
			inputs: func(*testing.T) ([]<-chan bool, func()) {
				ch1 := make(chan bool)
				ch2 := make(chan bool)
				ch3 := make(chan bool)

				return []<-chan bool{ch1, ch2, ch3}, func() {
					ch1 <- true
					ch2 <- true
					ch3 <- true
					ch1 <- false

					close(ch1)
					close(ch2)
					close(ch3)
				}
			},
		},
		{
			name: "keep working if a channel closes",
			expected: func(_ *testing.T, out <-chan bool, wg *sync.WaitGroup) {
				expects := []bool{false, false, true, false}
				for _, expect := range expects {
					if <-out != expect {
						t.Errorf("expected %v, got %v", expect, !expect)
					}
				}
				wg.Done()
			},
			inputs: func(*testing.T) ([]<-chan bool, func()) {
				ch1 := make(chan bool)
				ch2 := make(chan bool)
				ch3 := make(chan bool)

				return []<-chan bool{ch1, ch2, ch3}, func() {
					ch1 <- true
					ch2 <- true
					close(ch2)
					ch3 <- true
					ch1 <- false

					close(ch1)
					close(ch3)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bools, run := test.inputs(t)

			out := merge.Bools(logr.Discard(), bools...)

			wg := &sync.WaitGroup{}
			wg.Add(1)
			go test.expected(t, out, wg)
			run()
		})
	}

}
