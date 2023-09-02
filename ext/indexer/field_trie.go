// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package indexer

import (
	"container/heap"
	"strings"
)

func newFieldTrie() *fieldTrie {
	return &fieldTrie{
		children: map[string]*fieldTrie{},
	}
}

type fieldTrie struct {
	parent       *fieldTrie
	fieldSegment string
	frequency    int
	children     map[string]*fieldTrie
	id           int64
}

func (t *fieldTrie) add(field string, id int64) {
	node := t
	fieldSegments := strings.Split(field, ".")
	for _, segment := range fieldSegments {
		child, found := node.children[segment]
		if !found {
			child = node.newChild(segment)
			node.children[segment] = child
		}
		child.frequency++
		node = child
	}
	node.id = id
}

func (t *fieldTrie) fieldName() string {
	if t.parent == nil {
		return t.fieldSegment
	}
	op := t.parent.fieldName()
	if len(op) == 0 {
		return t.fieldSegment
	}
	return op + "." + t.fieldSegment
}

func (t *fieldTrie) sortedPresenceFields() []*fieldFrequency {
	fq := make(fieldFrequencyQueue, 0, len(t.children))
	for _, child := range t.children {
		if child.id != 0 {
			fq = append(fq, &fieldFrequency{
				id:        child.id,
				parentID:  t.id,
				field:     child.fieldName(),
				frequency: child.frequency,
			})
		}
		childFrequencies := child.sortedPresenceFields()
		fq = append(fq, childFrequencies...)
	}
	heap.Init(&fq)
	sortedFrequencies := make([]*fieldFrequency, len(fq))
	i := 0
	for fq.Len() > 0 {
		sortedFrequencies[i] = heap.Pop(&fq).(*fieldFrequency)
		i++
	}
	return sortedFrequencies
}

func (t *fieldTrie) newChild(fieldSegment string) *fieldTrie {
	return &fieldTrie{
		parent:       t,
		fieldSegment: fieldSegment,
		frequency:    0,
		children:     map[string]*fieldTrie{},
	}
}

type fieldFrequency struct {
	id        int64
	parentID  int64
	field     string
	frequency int
}

func (ff *fieldFrequency) fieldName() string {
	return ff.field
}

func (ff *fieldFrequency) fieldPath() []string {
	return []string{ff.field}
}

type fieldFrequencyQueue []*fieldFrequency

func (fh fieldFrequencyQueue) Len() int { return len(fh) }

func (fh fieldFrequencyQueue) Less(i, j int) bool {
	// sort items in descendening order
	cmp := fh[i].frequency - fh[j].frequency
	if cmp > 0 {
		return true
	}
	if cmp < 0 {
		return false
	}
	// Ensure that ties are broken by id so that you get stable
	// frequency ordering.
	return fh[i].id < fh[j].id
}

func (fh fieldFrequencyQueue) Swap(i, j int) {
	fh[i], fh[j] = fh[j], fh[i]
}

func (fh *fieldFrequencyQueue) Push(item any) {
	*fh = append(*fh, item.(*fieldFrequency))
}

func (fh *fieldFrequencyQueue) Pop() any {
	old := *fh
	n := len(old)
	item := old[n-1]
	old[n-1] = nil // avoid memory leak
	*fh = old[0 : n-1]
	return item
}
