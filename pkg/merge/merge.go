// Package merge implements merging of channels into one
package merge

import (
	"reflect"
	"slices"

	"github.com/go-logr/logr"
)

// Bools merges multiple bool channels into a single channel
func Bools(log logr.Logger, ready ...<-chan bool) <-chan bool {
	log.V(2).Info("Merging channels")
	out := make(chan bool)
	go func() {
		cases := make([]reflect.SelectCase, len(ready))
		statuses := make([]bool, len(ready))
		for i, ch := range ready {
			cases[i] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)}
		}
		defer close(out)
		for {
			chosen, value, ok := reflect.Select(cases)
			if !ok {
				log.V(2).Info("channel is closed", "channel", chosen)
				cases[chosen].Chan = reflect.ValueOf(nil)
				continue
			}
			b := value.Bool()
			log.V(2).Info("channel changed state", "channel", chosen, "state", b)
			statuses[chosen] = b
			out <- !slices.Contains(statuses, false)
		}
	}()

	return out
}
