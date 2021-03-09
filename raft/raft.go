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
	"bytes"
	"errors"
	"math/rand"

	pb "github.com/pingcap-incubator/tinykv/proto/pkg/eraftpb"
)

// None is a placeholder node ID used when there is no leader.
const None uint64 = 0

// StateType represents the role of a node in a cluster.
type StateType uint64

// the raftNode's state
const (
	StateFollower = iota
	StateCandidate
	StateLeader
)

var stmap = [...]string{
	"StateFollower",
	"StateCandidate",
	"StateLeader",
}

func (st StateType) String() string {
	return stmap[uint64(st)]
}

// ErrProposalDropped is returned when the proposal is ignored by some cases,
// so that the proposer can be notified and fail fast.
var ErrProposalDropped = errors.New("raft proposal dropped")

// Config contains the parameters to start a raft.
type Config struct {
	// ID is the identity of the local raft. ID cannot be 0.
	ID uint64

	// peers contains the IDs of all nodes (including self) in the raft cluster. It
	// should only be set when starting a new raft cluster. Restarting raft from
	// previous configuration will panic if peers is set. peer is private and only
	// used for testing right now.
	peers []uint64

	// ElectionTick is the number of Node.Tick invocations that must pass between
	// elections. That is, if a follower does not receive any message from the
	// leader of current term before ElectionTick has elapsed, it will become
	// candidate and start an election. ElectionTick must be greater than
	// HeartbeatTick. We suggest ElectionTick = 10 * HeartbeatTick to avoid
	// unnecessary leader switching.
	ElectionTick int
	// HeartbeatTick is the number of Node.Tick invocations that must pass between
	// heartbeats. That is, a leader sends heartbeat messages to maintain its
	// leadership every HeartbeatTick ticks.
	HeartbeatTick int

	// Storage is the storage for raft. raft generates entries and states to be
	// stored in storage. raft reads the persisted entries and states out of
	// Storage when it needs. raft reads out the previous state and configuration
	// out of storage when restarting.
	Storage Storage
	// Applied is the last applied index. It should only be set when restarting
	// raft. raft will not return entries to the application smaller or equal to
	// Applied. If Applied is unset when restarting, raft might return previous
	// applied entries. This is a very application dependent configuration.
	Applied uint64
}

func (c *Config) validate() error {
	if c.ID == None {
		return errors.New("cannot use none as id")
	}

	if c.HeartbeatTick <= 0 {
		return errors.New("heartbeat tick must be greater than 0")
	}

	if c.ElectionTick <= c.HeartbeatTick {
		return errors.New("election tick must be greater than heartbeat tick")
	}

	if c.Storage == nil {
		return errors.New("storage cannot be nil")
	}

	return nil
}

// Progress represents a follower’s progress in the view of the leader. Leader maintains
// progresses of all followers, and sends entries to the follower based on its progress.
type Progress struct {
	Match, Next uint64
}

type Raft struct {
	id uint64

	Term uint64
	Vote uint64

	// the log
	RaftLog *RaftLog

	// log replication progress of each peers
	Prs map[uint64]*Progress

	// this peer's role
	State StateType

	// votes records
	votes map[uint64]bool

	// msgs need to send
	msgs []pb.Message

	// the leader id
	Lead uint64

	// heartbeat interval, should send
	heartbeatTimeout int
	// baseline of election interval
	electionTimeout int
	// number of ticks since it reached last heartbeatTimeout.
	// only leader keeps heartbeatElapsed.
	heartbeatElapsed int
	// Ticks since it reached last electionTimeout when it is leader or candidate.
	// Number of ticks since it reached last electionTimeout or received a
	// valid message from current leader when it is a follower.
	electionElapsed int

	// leadTransferee is id of the leader transfer target when its value is not zero.
	// Follow the procedure defined in section 3.10 of Raft phd thesis.
	// (https://web.stanford.edu/~ouster/cgi-bin/papers/OngaroPhD.pdf)
	// (Used in 3A leader transfer)
	leadTransferee uint64

	// Only one conf change may be pending (in the log, but not yet
	// applied) at a time. This is enforced via PendingConfIndex, which
	// is set to a value >= the log index of the latest pending
	// configuration change (if any). Config changes are only allowed to
	// be proposed if the leader's applied index is greater than this
	// value.
	// (Used in 3A conf change)
	PendingConfIndex uint64
}

// newRaft return a raft peer with the given config
func newRaft(c *Config) *Raft {
	if err := c.validate(); err != nil {
		panic(err.Error())
	}
	// Your Code Here (2A).
	hardState, confState, _ := c.Storage.InitialState()
	// fmt.Printf("confState: %+v\n", confState)
	votes := make(map[uint64]bool)
	prs := make(map[uint64]*Progress)

	var peers []uint64
	if len(c.peers) != 0 {
		peers = c.peers
	}
	if len(confState.Nodes) != 0 {
		peers = confState.Nodes
	}
	for _, peer := range peers {
		votes[peer] = false
		prs[peer] = &Progress{
			Match: 0,
			Next:  1,
		}
	}

	term := hardState.Term
	vote := hardState.Vote
	raftLog := newLog(c.Storage)
	if hardState.Commit != 0 {
		raftLog.committed = hardState.Commit
	}
	if c.Applied != 0 {
		raftLog.applied = c.Applied
	}

	return &Raft{
		id:               c.ID,
		Term:             term,
		State:            StateFollower,
		votes:            votes,
		Vote:             vote,
		electionTimeout:  c.ElectionTick,
		heartbeatTimeout: c.HeartbeatTick,
		RaftLog:          raftLog,
		Prs:              prs,
	}
}

// sendAppend sends an append RPC with new entries (if any) and the
// current commit index to the given peer. Returns true if a message was sent.
func (r *Raft) sendAppend(to uint64) bool {
	// Your Code Here (2A).
	offset, _ := r.RaftLog.FirstIndex()

	m := pb.Message{
		From:   r.id,
		To:     to,
		Term:   r.Term,
		Commit: r.RaftLog.committed,
	}

	if offset > r.Prs[to].Next {
		snapshot, err := r.RaftLog.storage.Snapshot()
		if err != nil {
			m.Snapshot = r.RaftLog.pendingSnapshot
		} else {
			m.Snapshot = &snapshot
		}
		m.Index, _ = r.RaftLog.SnapshotIndex()
		m.Term, _ = r.RaftLog.SnapshotTerm()
		m.MsgType = pb.MessageType_MsgSnapshot
	} else {
		m.Index = r.Prs[to].Next - 1
		m.LogTerm, _ = r.RaftLog.Term(m.Index)
		m.MsgType = pb.MessageType_MsgAppend
		m.Snapshot = nil
		lastIndex := r.RaftLog.LastIndex()
		for i := r.Prs[to].Next; i <= lastIndex; i++ {
			m.Entries = append(m.Entries, &r.RaftLog.entries[i-offset])
		}
	}

	r.msgs = append(r.msgs, m)
	return false
}

// sendHeartbeat sends a heartbeat RPC to the given peer.
func (r *Raft) sendHeartbeat(to uint64) {
	// Your Code Here (2A).

	r.msgs = append(r.msgs, pb.Message{
		From:    r.id,
		To:      to,
		Term:    r.Term,
		MsgType: pb.MessageType_MsgHeartbeat,
		Commit:  r.RaftLog.committed,
	})
}

// tick advances the internal logical clock by a single tick.
func (r *Raft) tick() {
	// Your Code Here (2A).
	// log.Info("tick")
	switch r.State {
	case StateFollower:
		r.electionElapsed++
		if r.electionElapsed == r.electionTimeout {
			r.electionElapsed = 0
			r.Step(pb.Message{From: r.id, To: r.id, MsgType: pb.MessageType_MsgHup})
		}
	case StateCandidate:
		// if r.electionElapsed == 0 {
		// 	// Send RequestVote RPCs to all other servers
		// 	r.Step(pb.Message{From: r.id, MsgType: pb.MessageType_MsgRequestVote})
		// }
		r.electionElapsed++
		if r.electionElapsed == r.electionTimeout {
			r.electionElapsed = 0
			r.Step(pb.Message{From: r.id, To: r.id, Term: r.Term, MsgType: pb.MessageType_MsgHup})
		}
	case StateLeader:
		r.heartbeatElapsed++
		if r.heartbeatElapsed == r.heartbeatTimeout {
			r.heartbeatElapsed = 0
			r.Step(pb.Message{MsgType: pb.MessageType_MsgBeat})
		}
	}
}

// becomeFollower transform this peer's state to Follower
func (r *Raft) becomeFollower(term uint64, lead uint64) {
	// Your Code Here (2A).
	r.Term = term
	r.Lead = lead
	r.Vote = None
	r.State = StateFollower
	r.electionTimeout = rand.Intn(10) + 10

	// r.RaftLog.entries = append(r.RaftLog.entries, pb.Entry{
	// 	Term:  term,
	// 	Index: r.RaftLog.LastIndex() + 1,
	// })
	// fmt.Println("becomeFollower later, Logterm:", term, "LogIndex:", r.RaftLog.LastIndex())
}

// becomeCandidate transform this peer's state to candidate
func (r *Raft) becomeCandidate() {
	// Your Code Here (2A).

	for id := range r.votes {
		r.votes[id] = false
	}
	r.State = StateCandidate
	// testNonleaderElectionTimeoutRandomized
	r.electionTimeout = rand.Intn(10) + 10
	// Increment currentTerm
	r.Term++
	// Vote for self
	r.Vote = r.id
	r.votes[r.id] = true
	// Reset election timer
	r.electionElapsed = 0

	r.RaftLog.rejectNum = 0
	r.RaftLog.acceptNum = 1
}

// becomeLeader transform this peer's state to leader
func (r *Raft) becomeLeader() {
	// Your Code Here (2A).
	// NOTE: Leader should propose a noop entry on its term
	r.State = StateLeader
	r.Vote = None
	r.Lead = r.id

	lastIndex := r.RaftLog.LastIndex()
	for peer := range r.Prs {
		if peer == r.id {
			r.Prs[peer].Match = lastIndex + 1
			r.Prs[peer].Next = lastIndex + 2
		} else {
			r.Prs[peer].Match, _ = r.RaftLog.storage.FirstIndex()
			r.Prs[peer].Match--
			r.Prs[peer].Next = lastIndex + 1
		}
	}

	r.RaftLog.entries = append(r.RaftLog.entries, pb.Entry{
		// EntryType: pb.EntryType_EntryNormal,
		Index: lastIndex + 1,
		Term:  r.Term,
	})
	// r.Step(pb.Message{From: r.id, To: r.id, Term: r.Term, MsgType: pb.MessageType_MsgBeat})
	// log.Infof("peer: %d successed become leader", r.id)
}

// Step the entrance of handle message, see `MessageType`
// on `eraftpb.proto` for what msgs should be handled
func (r *Raft) Step(m pb.Message) error {
	// Your Code Here (2A).
	// log.Infof("message: \n%+v\n", m)
	// if m.Term > r.Term {
	// 	r.becomeFollower(m.Term, m.From)
	// }
	switch r.State {
	case StateFollower:
		switch m.MsgType {
		case pb.MessageType_MsgHup:
			// r.readMessages()
			// Send RequestVote RPCs to all other servers
			r.State = StateCandidate
			r.Step(pb.Message{From: r.id, To: r.id, Term: r.Term + 1, MsgType: pb.MessageType_MsgRequestVote})
			r.becomeCandidate()
			// fmt.Println(len(r.votes))
			if len(r.votes) == 1 {
				r.becomeLeader()
				r.RaftLog.committed = r.RaftLog.LastIndex()
				r.Step(pb.Message{From: r.id, To: r.id, Term: r.Term, MsgType: pb.MessageType_MsgBeat})
			}
		case pb.MessageType_MsgAppend:
			r.handleAppendEntries(m)
		case pb.MessageType_MsgRequestVote:
			// follower 收到投票请求
			r.handleVote(m)
		case pb.MessageType_MsgHeartbeat:
			r.handleHeartbeat(m)
		case pb.MessageType_MsgSnapshot:
			r.handleSnapshot(m)
		}
	case StateCandidate:
		switch m.MsgType {
		case pb.MessageType_MsgHup:
			// r.readMessages()
			// Send RequestVote RPCs to all other servers
			r.becomeFollower(r.Term, None)
			r.Step(pb.Message{From: r.id, To: r.id, Term: r.Term + 1, MsgType: pb.MessageType_MsgHup})
			// r.becomeCandidate()
		case pb.MessageType_MsgAppend:
			r.becomeFollower(m.Term, m.From)
			r.handleAppendEntries(m)
		case pb.MessageType_MsgRequestVote:
			// candidate 发送投票请求
			if m.From != r.id {
				r.handleVote(m)
				break
			}
			for peer := range r.votes {
				if peer == r.id {
					continue
				}
				index := r.RaftLog.LastIndex()
				logTerm, _ := r.RaftLog.Term(index)
				r.msgs = append(r.msgs, pb.Message{
					From:    r.id,
					To:      peer,
					Term:    r.Term + 1,
					Index:   index,
					LogTerm: logTerm,
					Commit:  r.RaftLog.committed,
					MsgType: pb.MessageType_MsgRequestVote,
				})
			}
		case pb.MessageType_MsgRequestVoteResponse:
			if m.To != r.id {
				break
			}
			if m.Term == r.Term && !m.Reject {
				r.votes[m.From] = true
				r.RaftLog.acceptNum++
			} else {
				r.RaftLog.rejectNum++
			}
			// fmt.Println(" len: ", len(r.votes), " allVote: ", allVote)
			if r.RaftLog.acceptNum > (uint64)(len(r.votes)/2) {
				r.becomeLeader()
				r.Step(pb.Message{From: r.id, To: r.id, Term: r.Term, MsgType: pb.MessageType_MsgBeat})
			} else if r.RaftLog.rejectNum > (uint64)(len(r.votes)/2) {
				r.becomeFollower(m.Term, 0)
				// TODO: 把退化成 Follower 放到 Tick() 中，也就是判断 electionEclapsed
			}
		case pb.MessageType_MsgHeartbeat:
			r.handleHeartbeat(m)
		}
	case StateLeader:
		switch m.MsgType {
		case pb.MessageType_MsgBeat:
			// snapshot, _ := r.RaftLog.storage.Snapshot()
			// if len(r.RaftLog.entries) > 0 && snapshot.Metadata != nil && snapshot.Metadata.Index > r.RaftLog.entries[0].Index {
			// 	r.RaftLog.pendingSnapshot = &snapshot
			// 	r.RaftLog.maybeCompact()
			// }
			for peer := range r.votes {
				if peer == r.id {
					// r.Prs[r.id].Match = r.RaftLog.LastIndex()
					// r.Prs[r.id].Next = r.RaftLog.LastIndex() + 1
					continue
				}
				r.sendHeartbeat(peer)
			}
			// if len(r.Prs) == 1 {
			// 	r.RaftLog.committed = r.RaftLog.LastIndex()
			// }
		case pb.MessageType_MsgPropose:
			// appendEntry
			for _, ent := range m.Entries {
				ent.Index = r.RaftLog.LastIndex()
				ent.Index++
				ent.Term = r.Term
				ent.EntryType = pb.EntryType_EntryNormal
				r.RaftLog.entries = append(r.RaftLog.entries, *ent)
			}
			// bcastAppend
			lastIndex := r.RaftLog.LastIndex()
			for peer := range r.Prs {
				if peer == r.id {
					r.Prs[r.id].Match = lastIndex
					r.Prs[r.id].Next = lastIndex + 1
					continue
				}
				r.sendAppend(peer)
			}
			if len(r.Prs) == 1 {
				r.RaftLog.committed = lastIndex
			}
		case pb.MessageType_MsgAppend:
			if m.Term > r.Term {
				r.becomeFollower(m.Term, m.From)
				r.handleAppendEntries(m)
			} else if m.Term == r.Term {
				r.handleAppendEntries(m)
			} else {
				// TODO: 拒绝
			}

		case pb.MessageType_MsgAppendResponse:
			if m.Term > r.Term {
				r.becomeFollower(m.Term, m.From)
				break
			} else if m.Term < r.Term {
				break
			}
			// if m.Index != r.Prs[m.From].Next-1 {
			// 	// r.Prs[m.From].Next 被修改过，m.Index 过期
			// 	break
			// }
			if m.Reject {
				// 如果拒绝
				r.Prs[m.From].Next--
				r.sendAppend(m.From)
			} else {
				// 如果成功
				r.Prs[m.From].Next = m.Index + 1
				r.Prs[m.From].Match = m.Index

				// 判断接受的 append 的 term 是不是当前 term
				logTerm, _ := r.RaftLog.Term(m.Index)
				if logTerm < r.Term {
					break
				}

				// 已经committed
				if m.Index <= r.RaftLog.committed {
					break
				}

				// 判断是不是大多数都复制成功了，可以commit
				sucNum := 0
				for _, pr := range r.Prs {
					// fmt.Printf("#%d.match: %d, m.Index: %d\n", i, pr.Match, m.Index)
					if pr.Match >= m.Index {
						sucNum++
					}
				}
				// fmt.Println("responseId:", m.From, "MatchIndex:", m.Index, "sucNum:", sucNum)
				if sucNum > len(r.Prs)/2 {
					r.RaftLog.committed = m.Index
					// 给其他peer commit

					for peer := range r.votes {
						if peer == r.id {
							continue
						}

						r.sendAppend(peer)
					}
				}
				// fmt.Println("committed =", r.RaftLog.committed)
			}
		case pb.MessageType_MsgRequestVote:
			// if r.Term < m.Term {
			// 	r.becomeFollower(m.Term, m.From)
			// }
			r.handleVote(m)
		// case pb.MessageType_MsgHeartbeat:
		// 	r.handleHeartbeat(m)
		case pb.MessageType_MsgHeartbeatResponse:
			offset, err := r.RaftLog.FirstIndex()
			lastIndex := r.RaftLog.LastIndex()
			if err != nil {
				break
			}
			if r.Prs[m.From].Match < lastIndex && offset <= r.Prs[m.From].Next {
				r.sendAppend(m.From)
			}
		}
	}
	return nil
}

func (r *Raft) checkSnap() bool {
	firstIndex, _ := r.RaftLog.FirstIndex()
	for peer := range r.Prs {
		if firstIndex > r.Prs[peer].Next {
			return true
		}
	}
	return false
}

// sendAppendResponse
func (r *Raft) sendAppendResponse(m pb.Message) {
	// fmt.Printf("entries: %+v\n", m.Entries)
	if !m.Reject {
		r.Lead = m.From
	}
	r.msgs = append(r.msgs, pb.Message{
		To:      m.From,
		From:    m.To,
		Term:    r.Term,
		MsgType: pb.MessageType_MsgAppendResponse,
		Index:   m.Index + uint64(len(m.Entries)),
		Reject:  m.Reject,
	})
}

// equal
func equal(e1, e2 *pb.Entry) bool {
	if e1.GetEntryType() != e2.GetEntryType() {
		return false
	} else if e1.GetIndex() != e2.GetIndex() {
		return false
	} else if e1.GetTerm() != e2.GetTerm() {
		return false
	} else if !bytes.Equal(e1.GetData(), e2.GetData()) {
		return false
	}
	return true
}

// handleAppendEntries handle AppendEntries RPC request
func (r *Raft) handleAppendEntries(m pb.Message) {
	// Your Code Here (2A).
	m.Reject = false

	if r.Term < m.Term {
		r.Term = m.Term
	} else if r.Term > m.Term {
		m.Reject = true
		// 接受到任期较小的leader的append RPC
		r.sendAppendResponse(m)
		return
	}

	index := r.RaftLog.LastIndex()
	if index < m.Index {
		// fmt.Println("cnt < m.Index")
		m.Reject = true
		r.sendAppendResponse(m)
		return
	}
	// 找到一样的index
	for m.Index != index {
		index--
	}

	if index != 0 {
		logTerm, _ := r.RaftLog.Term(index)
		if logTerm != m.LogTerm {
			// fmt.Println("not find logterm")
			m.Reject = true
			r.sendAppendResponse(m)
			return
		}
	}

	// 如果成功
	// fmt.Println("successed find")
	succeed := false

	for _, ent := range m.Entries {

		index++
		lastIndex := r.RaftLog.LastIndex()
		if index > lastIndex {
			succeed = true
		}

		// fmt.Println("index =", index, "len:", len(r.RaftLog.entries))

		if succeed {
			// fmt.Println("++++++++")
			r.RaftLog.entries = append(r.RaftLog.entries, *ent)
		} else {
			entry, _ := r.RaftLog.Entry(index)
			// fmt.Printf("entries[%d]: %+v\n", index, entry)
			if !equal(ent, entry) {
				// 舍弃后面的日志
				// fmt.Println("=========")
				offset := r.RaftLog.entries[0].Index
				r.RaftLog.entries = append(r.RaftLog.entries[:index-offset], *ent)
				r.RaftLog.stabled = index - 1
				succeed = true
			}
		}
	}
	r.RaftLog.committed = min(m.Commit, m.Index+(uint64)(len(m.Entries)))
	if r.RaftLog.committed < r.RaftLog.applied {
		r.RaftLog.applied = r.RaftLog.committed
	}
	// r.RaftLog.committed = min(m.Commit, r.RaftLog.LastIndex())

	// fmt.Println("committed:", m.Commit)
	r.sendAppendResponse(m)
}

//
func (r *Raft) sendVoteResponse(m pb.Message) {
	if m.Reject == false {
		r.Vote = m.From
		r.Term = m.Term
	}

	r.msgs = append(r.msgs, pb.Message{
		From:    m.To,
		To:      m.From,
		Term:    r.Term, // 不记得在哪用了
		MsgType: pb.MessageType_MsgRequestVoteResponse,
		Reject:  m.Reject,
	})
}

// handleVote handle RequestVote RPC request
func (r *Raft) handleVote(m pb.Message) {
	// fmt.Println("m.Term:", m.Term, "m.logTerm:", m.LogTerm, "r.Term:", r.Term)
	// fmt.Println("Handle Vote begin, id:", r.id)
	if r.State == StateCandidate || r.State == StateLeader {
		if m.Term <= r.Term {
			// fmt.Println(0)
			m.Reject = true
			r.sendVoteResponse(m)
			return
		}
	}
	if m.Term > r.Term {
		r.becomeFollower(m.Term, None)
	}
	if m.LogTerm <= m.Term {
		lastIndex := r.RaftLog.LastIndex()
		if m.Index < lastIndex {
			term, err := r.RaftLog.Term(m.Index)
			if err == ErrCompacted {
				// fmt.Println(2)
				lastTerm, _ := r.RaftLog.Term(lastIndex)
				if lastTerm >= m.LogTerm {
					m.Reject = true
				} else {
					m.Reject = false
				}
			} else if term+1 <= m.LogTerm {
				// fmt.Println(3)
				m.Reject = false
			} else {
				// fmt.Println(4)
				m.Reject = true
			}
		} else {
			lastTerm, _ := r.RaftLog.Term(lastIndex)
			if lastTerm > m.LogTerm {
				// fmt.Println(7)
				m.Reject = true
			} else {
				if r.RaftLog.committed <= m.Index {
					// fmt.Println(8)
					if r.Vote == None || r.Vote == m.From {
						m.Reject = false
					} else if r.Vote != m.From && r.Term == m.Term {
						m.Reject = true
					}
				} else {
					// fmt.Println(9)
					m.Reject = true
				}
			}
		}
	} else {
		// fmt.Println(10)
		m.Reject = true
	}
	r.sendVoteResponse(m)
}

// handleHeartbeat handle Heartbeat RPC request
func (r *Raft) handleHeartbeat(m pb.Message) {
	// Your Code Here (2A).
	if r.Term > m.Term || r.id != m.To {
		return
	}

	r.Lead = m.From
	r.electionElapsed = 0
	lastIndex := r.RaftLog.LastIndex()
	r.RaftLog.committed = min(m.Commit, lastIndex)
	if r.RaftLog.committed < r.RaftLog.applied {
		r.RaftLog.applied = r.RaftLog.committed
	}

	r.msgs = append(r.msgs, pb.Message{
		From:    r.id,
		To:      m.From,
		MsgType: pb.MessageType_MsgHeartbeatResponse,
	})
}

// handleSnapshot handle Snapshot RPC request
func (r *Raft) handleSnapshot(m pb.Message) {
	// Your Code Here (2C).
	if m.Term > r.Term {
		r.becomeFollower(m.Term, m.From)
	}

	if m.Snapshot.Metadata.Index < r.RaftLog.committed {
		return
	}
	r.RaftLog.committed = max(r.RaftLog.committed, m.Commit)
	r.RaftLog.pendingSnapshot = m.Snapshot
	r.Prs = make(map[uint64]*Progress)
	lastIndex := r.RaftLog.LastIndex()
	for _, peer := range m.Snapshot.Metadata.ConfState.Nodes {
		r.Prs[peer] = &Progress{
			Match: 0,
			Next:  lastIndex + 1,
		}
		if peer == r.id {
			r.Prs[peer].Match = r.Prs[peer].Next - 1
		}
	}

	r.sendAppendResponse(m)
}

// addNode add a new node to raft group
func (r *Raft) addNode(id uint64) {
	// Your Code Here (3A).
}

// removeNode remove a node from raft group
func (r *Raft) removeNode(id uint64) {
	// Your Code Here (3A).
}
