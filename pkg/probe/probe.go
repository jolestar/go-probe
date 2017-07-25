package probe

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
)

var single = newProbe()

func newProbe() *Probe {
	return &Probe{probeFuncs: map[string]ProbeFunc{}, lock: sync.RWMutex{}}
}

type Result struct {
	Name string            `json:"name"`
	Summary string 	`json:"summary"`
	Data map[string]string `json:"data"`
}

func NewResult(name string) *Result {
	return &Result{Name: name, Data: map[string]string{}}
}

type ProbeFunc func(ctx context.Context) (*Result, error)

type Probe struct {
	probeFuncs map[string]ProbeFunc
	lock       sync.RWMutex
}

func (p *Probe) DoProbe(ctx context.Context, name string) (interface{}, error) {
	p.lock.RLock()
	defer p.lock.RUnlock()

	if name != "" {
		probeFunc, ok := p.probeFuncs[name]
		if !ok {
			return nil, fmt.Errorf("No such probe [%s]", name)
		}
		return probeFunc(ctx)
	} else {
		var results []*Result
		for k, probeFunc := range p.probeFuncs {
			result, err := probeFunc(ctx)
			if err != nil {
				log.Fatalf("Probe %s error: %s \n", k, err.Error())
			} else {
				results = append(results, result)
			}
		}
		sort.Slice(results, func(i, j int) bool {
			return results[i].Name < results[j].Name
		})
		return results, nil
	}
}

func (p *Probe) Register(name string, probeFunc ProbeFunc) {
	p.lock.Lock()
	p.probeFuncs[name] = probeFunc
	p.lock.Unlock()
}

func DoProbe(ctx context.Context, name string) (interface{}, error) {
	return single.DoProbe(ctx, name)
}
