// Copyright 2015 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package raft

import (
	"github.com/pingcap-incubator/tinykv/kv/raftstore/util"
	pb "github.com/pingcap-incubator/tinykv/proto/pkg/eraftpb"
)

// RaftLog manage the log entries, its struct look like:
//
//  snapshot/first.....applied....committed....stabled.....last
//  --------|------------------------------------------------|
//                            log entries
//
// for simplify the RaftLog implement should manage all log entries
// that not truncated
type RaftLog struct {
	// storage contains all stable entries since the last snapshot.
	storage Storage

	// committed is the highest log position that is known to be in
	// stable storage on a quorum of nodes.
	committed uint64

	// applied is the highest log position that the application has
	// been instructed to apply to its state machine.
	// Invariant: applied <= committed
	applied uint64

	// log entries with index <= stabled are persisted to storage.
	// It is used to record the logs that are not persisted by storage yet.
	// Everytime handling `Ready`, the unstabled logs will be included.
	stabled uint64

	// all entries that have not yet compact.
	entries []pb.Entry

	// the incoming unstable snapshot, if any.
	// (Used in 2C)
	pendingSnapshot *pb.Snapshot

	// Your Data Here (2A).
	rejectNum, acceptNum uint64
}

// newLog returns log using the given storage. It recovers the log
// to the state that it just commits and applies the latest snapshot.
func newLog(storage Storage) *RaftLog {
	// Your Code Here (2A).
	first, _ := storage.FirstIndex()
	last, _ := storage.LastIndex()
	ents, _ := storage.Entries(first, last+1)
	// fmt.Println(len(ents))
	return &RaftLog{
		storage:   storage,
		applied:   first - 1,
		committed: first - 1,
		stabled:   first + uint64(len(ents)) - 1,
		entries:   ents,
	}
}

// We need to compact the log entries in some point of time like
// storage compact stabled log entries prevent the log entries
// grow unlimitedly in memory
func (l *RaftLog) maybeCompact() {
	// Your Code Here (2C).
	offset := l.entries[0].Index

	i := l.pendingSnapshot.Metadata.Index - offset + 1

	if i == uint64(len(l.entries)) {
		l.entries = make([]pb.Entry, 0)
		return
	}

	ents := make([]pb.Entry, 1, uint64(len(l.entries))-i)
	ents[0].Index = l.entries[i].Index
	ents[0].Term = l.entries[i].Term
	ents = append(ents, l.entries[i+1:]...)
	l.entries = ents
}

// unstableEntries return all the unstable entries
func (l *RaftLog) unstableEntries() []pb.Entry {
	// Your Code Here (2A).
	offset, _ := l.storage.FirstIndex()
	// fmt.Println("len:", len(l.entries), "stabled:", l.stabled, "offset:", offset)
	return l.entries[int(l.stabled-offset+1):]
}

func (l *RaftLog) NextEnts() ([]pb.Entry, error) {
	if l.applied > l.committed {
		return nil, new(util.ErrStaleCommand)
	}
	return l.nextEnts(), nil
}

// nextEnts returns all the committed but not applied entries
func (l *RaftLog) nextEnts() (ents []pb.Entry) {
	// Your Code Here (2A).

	if len(l.entries) == 0 {
		return nil
	}
	offset, _ := l.storage.FirstIndex()
	// fmt.Println("len:", len(l.entries), "applied:", l.applied, "committed:", l.committed, "offset:", offset)
	if offset > l.applied+1 || offset > l.committed+1 {
		return nil
	}
	return l.entries[int(l.applied-offset+1):int(l.committed-offset+1)]
}

// LastIndex return the last index of the log entries
func (l *RaftLog) LastIndex() uint64 {
	// Your Code Here (2A).
	// l.maybeCompact()
	offset, _ := l.storage.FirstIndex()
	lastIndex := offset + uint64(len(l.entries)) - 1
	// lastIndex, _ := l.storage.LastIndex()
	if l.pendingSnapshot != nil && lastIndex < l.pendingSnapshot.Metadata.Index {
		return l.pendingSnapshot.Metadata.Index
	}
	return lastIndex
}

func (l *RaftLog) firstIndex() uint64 {
	offset, _ := l.storage.FirstIndex()
	if l.pendingSnapshot != nil && offset <= l.pendingSnapshot.Metadata.Index {
		return l.pendingSnapshot.Metadata.Index + 1
	}
	return offset
}

// Term return the term of the entry in the given index
func (l *RaftLog) Term(i uint64) (uint64, error) {
	// Your Code Here (2A).
	if l.pendingSnapshot != nil && i == l.pendingSnapshot.Metadata.Index {
		return l.pendingSnapshot.Metadata.Term, nil
	}
	if len(l.entries) == 0 {
		return 0, ErrUnavailable
	}
	offset, _ := l.storage.FirstIndex()
	// fmt.Println("Term's offset:", offset)
	if i < offset {
		return 0, ErrCompacted
	}
	if int(i-offset) >= len(l.entries) {
		return 0, ErrUnavailable
	}
	return l.entries[i-offset].Term, nil
}

// Entry return the entry in the given index
func (l *RaftLog) Entry(i uint64) (*pb.Entry, error) {
	offset, _ := l.storage.FirstIndex()
	if i < offset {
		return nil, ErrCompacted
	}
	if int(i-offset) >= len(l.entries) {
		return nil, ErrUnavailable
	}
	return &l.entries[i-offset], nil
}

func (l *RaftLog) SnapshotIndex() uint64 {
	return l.pendingSnapshot.Metadata.Index
}

func (l *RaftLog) SnapshotTerm() uint64 {
	return l.pendingSnapshot.Metadata.Term
}
