/*
Olivier Wulveryck - author of Gorchestrator
Copyright (C) 2015 Olivier Wulveryck

This file is part of the Gorchestrator project and
is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this prograv.Digraph.  If not, see <http://www.gnu.org/licenses/>.
*/
package orchestrator

import (
	"encoding/json"
	"github.com/owulveryck/gorchestrator/structure"
	//"log"
	"regexp"
	"time"
)

type Information struct {
	Sequence int
	Matrix   structure.Matrix
}

// Graph is the input of the orchestrator
type Graph struct {
	Name    string           `json:"name,omitempty"`
	State   int              `json:"state"`
	Digraph structure.Matrix `json:"digraph"`
	Nodes   []Node           `json:"nodes"`
	Timeout <-chan time.Time `json:"-"`
	ID      string           `json:"id,omitempty"`
}

func (g *Graph) UnmarshalJSON(b []byte) (err error) {
	type graph struct {
		Name    string           `json:"name,omitempty"`
		State   int              `json:"state"`
		Digraph []int64          `json:"digraph"`
		Nodes   []Node           `json:"nodes"`
		Timeout <-chan time.Time `json:"-"`
	}
	s := graph{}
	if err = json.Unmarshal(b, &s); err == nil {
		g.Name = s.Name
		g.State = s.State
		g.Digraph = s.Digraph
		g.Nodes = s.Nodes
	} else {
		return err
	}
	return nil
}

func (g *Graph) MarshalJSON() ([]byte, error) {
	type graph struct {
		Name    string           `json:"name,omitempty"`
		State   int              `json:"state"`
		Digraph []int64          `json:"digraph"`
		Nodes   []Node           `json:"nodes"`
		Timeout <-chan time.Time `json:"-"`
	}
	s := graph{}
	s.Name = g.Name
	s.State = g.State
	s.Digraph = g.Digraph
	s.Nodes = g.Nodes
	return json.Marshal(s)
}

func (n *Graph) GetState() int {
	var state int
	state = n.State
	return state
}

func (n *Graph) SetState(s int) {
	n.State = s
}

func (v *Graph) getNodesFromRegexp(n string) ([]*Node, error) {
	re := regexp.MustCompile(n)
	var nn []*Node
	for i, _ := range v.Nodes {
		if re.MatchString(v.Nodes[i].Name) {
			nn = append(nn, &v.Nodes[i])
		}
	}
	return nn, nil
}

func (v *Graph) getNodeFromName(n string) (*Node, error) {
	var nn *Node
	return nn, nil
}

// Run executes the Graph structure
func (v *Graph) Run(exe []ExecutorBackend) {
	//log.Println("V's address:", &v)
	v.SetState(Running)

	n := v.Digraph.Dim()
	cs := make([]<-chan Message, n)
	co := make(chan Information)
	defer close(co)
	cos := broadcast(co, n, n)
	// Set up a done channel that's shared by the whole pipeline,
	// and close that channel when this pipeline exits, as a signal
	// for all the goroutines we started to exit.
	done := make(chan struct{})
	defer close(done)

	for i := 0; i < n; i++ {
		cs[i] = v.Nodes[i].Run(exe, done, cos[i])
	}
	c := merge(done, cs...)
	//c := fanIn(cs...)
	sequence := 0
	co <- Information{
		Matrix:   v.Digraph,
		Sequence: sequence,
	}
	for node := range c {
		//log.Printf("[SEQ %v] RANGE, got %v information", sequence, node)
		if node.State >= Running {
			for c := 0; c < n; c++ {
				val := v.Digraph.At(node.ID, c)
				if val != 0 {
					//log.Printf("[SEQ %v] Setting %v:%v = %v (current is %v)", sequence, node.ID, c, int64(node.State), v.Digraph.At(node.ID, c))
					v.Digraph.Set(node.ID, c, int64(node.State))
					//log.Printf("[SEQ %v] Checking %v:%v = %v", sequence, node.ID, c, v.Digraph.At(node.ID, c))
				}
			}
		}
		state := Success
		for r := 0; r < n; r++ {
			for c := 0; c < n; c++ {
				switch {
				case v.Digraph.At(r, c) == ToRun:
					state = Running
				case v.Digraph.At(r, c) == Running:
					state = Running
				case v.Digraph.At(r, c) > Success:
					state = Failure
				}
			}
		}
		v.SetState(state)
		if v.GetState() >= Success {
			return
		}
		//co <- v
		select {
		//case node := <-c:
		case co <- Information{
			Matrix:   v.Digraph,
			Sequence: sequence,
		}:
			//log.Printf("[SEQ %v] Message sent (node%v should be change to state %v): ", sequence, node.ID, node.State, v.Digraph)
		case <-v.Timeout:
			close(done)
			//co <- v
			v.SetState(Timeout)
			return
		}
		sequence += 1
	}
}

// Check is the structure is coherent, (a squared matrix with as many nodes as needed)
func (i *Graph) Check() Error {
	if len(i.Nodes)*len(i.Nodes) != len(i.Digraph) {
		return Error{1, "Structure is not coherent"}
	}
	return Error{0, ""}
}
