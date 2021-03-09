

# 测试过程记录

## 1. 各project测试点

## 1.1 2a

1. 2aa

| 完成情况 | 函数名                                            | 测试内容                                                     | 结果                                                 |
| -------- | ------------------------------------------------- | ------------------------------------------------------------ | ---------------------------------------------------- |
| ✔        | TestFollowerUpdateTermFromMessage2AA              | 更新 Follower 的 term                                        |                                                      |
| ✔        | TestCandidateUpdateTermFromMessage2AA             | 更新 Candidate 的 term                                       |                                                      |
| ✔        | TestLeaderUpdateTermFromMessage2AA                | 更新 Leader 的 term                                          |                                                      |
| ✔        | TestStartAsFollower2AA                            | 每次创建节点都是从 Follower 开始                             |                                                      |
| ✔?       | TestLeaderBcastBeat2AA                            | 经过一个 heartbeatTimeout 后，leader 向 follower 发送心跳    | **心跳只发送空日志**                                 |
| ✔        | TestFollowerStartElection2AA                      | follower 经过 electionTimeout 后，开始选举                   | 1. term 自增; 2. state 变为 candidate; 3. 给自己投票 |
| ✔?       | TestCandidateStartNewElection2AA                  | candidate 经过 electionTimeout 后，开始选举                  | 同上                                                 |
| ✔        | TestLeaderElectionInOneRoundRPC2AA                | 测试不同的选举结果：1. 选举成功; 2. 选举失败; 3. 不知道结果  | 1. 成为 leader; 2/3. stay candidate                  |
| ✔?✔      | TestFollowerVote2AA                               | 测试 follower 会怎么投票：1. 没投过票; 2. 投的自己; 3. 投的别人 | Reject: 1. false; 2. false; 3. true                  |
| ✔        | TestCandidateFallback2AA                          | candidate 收到 Append RPC 的反应：1. m.Term < r.Term; 2. m.Term = r.Term | 都退回成 follower                                    |
| ✔        | TestFollowerElectionTimeoutRandomized2AA          | 测试 election timeout 是不是随机的                           |                                                      |
| ✔        | TestCandidateElectionTimeoutRandomized2AA         | 同上                                                         |                                                      |
| ✔        | TestFollowersElectionTimeoutNonconflict2AA        |                                                              |                                                      |
| ✔        | TestCandidatesElectionTimeoutNonconflict2AA       |                                                              |                                                      |
| ??✔      | TestVoter2AA                                      | 详情看代码                                                   | Reject: true                                         |
|          | --------------------------------------------      |                                                              |                                                      |
| ✔        | TestLeaderElection2AA                             | 测试选举过程：1. 选举成功; 2. 选举失败                       |                                                      |
| ✔        | TestLeaderCycle2AA                                | 测试三个 raft 连续竞选并当选leader                           |                                                      |
| ✔        | TestVoteFromAnyState2AA                           | 测试三种 State 收到 **term 更大, index 更大**的投票请求的反应 | 全转成 follower 并投票                               |
| ✔        | TestSingleNodeCandidate2AA                        | 只有单个节点的选举                                           | 选举直接成功                                         |
| ??✔      | TestRecvMessageType_MsgRequestVote2AA             | 收到 MsgRequestVote 的反应                                   |                                                      |
| ✔        | TestCandidateResetTermMessageType_MsgHeartbeat2AA | 测试 candidate 收到心跳的反应                                | 转换为 follower 并执行心跳反应                       |
| ✔        | TestCandidateResetTermMessageType_MsgAppend2AA    | 测试 candidate 收到 append RPC 的反应                        | 转换为 follower 并执行 append                        |
| ✔        | TestDisruptiveFollower2AA                         |                                                              |                                                      |
| ✔        | TestRecvMessageType_MsgBeat2AA                    | 测试各个State收到MessageType_MsgBeat的反应：                 |                                                      |
| ✔        | TestCampaignWhileLeader2AA                        |                                                              |                                                      |
| ✔        | TestSplitVote2AA                                  |                                                              |                                                      |



2. 2ab（日志复制）

- MsgPropose: Leader 向其他所有 peer 发送 Append，除了基础的 From, To, Term, MsgType 之外，
  - Index = r.Prs[peer].Next - 1（该 peer 的下一个需要添加的日志的前一个日志索引）
  - LogTerm = r.RaftLog.Term(index)（上面索引对应的 Term ）
  - Entries = r.RaftLog.Ents[r.Prs[peer].Next : ]（该 peer 所需要添加的所有日志）
  - Commit = r.RaftLog.Committed（ Leader 的 Committed 数）
- 心跳：
  - 心跳是不需要给 Leader 增加空日志的

| 完成情况 | 测试名                                     | 测试内容                                                     | 测试结果                                                     |
| -------- | ------------------------------------------ | ------------------------------------------------------------ | ------------------------------------------------------------ |
| ✔        | TestLeaderStartReplication2AB              | Leader 开始发送 MsgPropose ，leader 的操作                   | Leader 向除了自己以外的其他 peer 发送 MsgAppend，包括：Index, LogTerm, Entries, Commit |
| ✔        | TestLeaderCommitEntry2AB                   | Leader 收到了各个 peer 的肯定的回复，判断 Leader 有没有修改它的 Committed | 修改 Committed 到最新                                        |
| ✔        | TestLeaderAcknowledgeCommit2AB             | 测试各种不同的 follower 回复，leader 的 committed 情况       | 如果大多数都 append 成功，那么修改 committed，否则 committed 不变 |
| ✔        | TestLeaderCommitPrecedingEntries2AB        | 测试已有之前日志的 Leader 的 MsgPropose 发送，被发送的 Follower 都没有被 Append过 | Entries 会加上之前的日志和 Leader 选举成功时添加的空日志，最后加上 propose 的日志一起发送 |
| ✔        | TestFollowerCommitEntry2AB                 | Follower 收到 append 请求时，对 commit 做出的修改            | 如果 follower 的日志已经更新到 m.commit 之后了，则更新自己的 committed |
| ✔        | TestFollowerCheckMessageType_MsgAppend2AB  | 测试 m.Index 和 m.LogTerm 与 Follower 的匹配情况             | 1. 匹配到 committed 或者 uncommitted 日志，则成功 Append; 2. 对于不匹配的日志或者不存在的日志，则拒绝 Append |
| ✔        | TestFollowerAppendEntries2AB               |                                                              | Append 成功的日志成功复制到 Follower 的 RaftLog 中           |
| ✔        | TestLeaderSyncFollowerLog2AB               | 模拟新 Leader 成功选举，然后进行 MsgPropose 的过程           | Leader 和 Follower 的日志是否同步                            |
| ✔        | TestVoteRequest2AB                         |                                                              |                                                              |
| ✔        | TestLeaderOnlyCommitsLogFromCurrentTerm2AB | 对于之前 term 的日志不进行 commit                            |                                                              |
| ✔        | TestProgressLeader2AB                      |                                                              |                                                              |
| ?✔       | TestLeaderElectionOverwriteNewerLogs2AB    | 模拟 5 个节点：1. 节点 1 因为票数不够选举失败; 2. 节点 1 选举成功并发送心跳 |                                                              |
| ✔？      | TestLogReplication2AB                      |                                                              |                                                              |
| ✔        | TestSingleNodeCommit2AB                    |                                                              |                                                              |
| ✔        | TestCommitWithoutNewTermEntry2AB           |                                                              |                                                              |
| ?✔       | TestDuelingCandidates2AB                   | 测试高 Term 但是低 LogTerm 的 candidate 进行选举的情况       | 由于 Term 大，所以 Leader 和 Follower 的都转为 Follower ，且 Term 也转为 m.Term; 由于 LogTerm 小，所以也会拒绝 candidate 的投票请求 |
| ✔        | TestCandidateConcede2AB                    |                                                              |                                                              |
| ✔        | TestOldMessages2AB                         |                                                              |                                                              |
| ✔        | TestProposal2AB                            |                                                              |                                                              |
| ?✔       | TestHandleMessageType_MsgAppend2AB         | 测试收到 Append 的回复：LastIndex, committed, reject         |                                                              |
| ?✔       | TestAllServerStepdown2AB                   | 各种 State 经过 Vote 和 Append 的状态: term, lastindex, len(entries), r.lead |                                                              |
| ✔        | TestHeartbeatUpdateCommit2AB               |                                                              |                                                              |
| ✔        | TestLeaderIncreaseNext2AB                  |                                                              |                                                              |

3. 2ac

| 完成情况 | 测试名                | 测试内容                                                 | 测试结果 |
| -------- | --------------------- | -------------------------------------------------------- | -------- |
|          | TestRawNodeStart2AC   | test node 可以正常启动，可以 accept and commit proposals |          |
|          | TestRawNodeRestart2AC |                                                          |          |



## 2. 测试过程

### part 2a

1. make project2aa: 

   | 测试名              | 测试错误                | 错误原因                                                     | 修改                   |
   | ------------------- | ----------------------- | ------------------------------------------------------------ | ---------------------- |
   | TestFollowerVote2AA | #4#5 的 reject 返回错误 | 当投票和请求投票的 term 相同的时候，没有对投票者的已投票内容进行判断 | 把 5 的情况并入 7-9 中 |

2. 更改了心跳过程，在传输心跳中添加了 Index 和 LogTerm，也把心跳直接写进了 becomeLeader 里面

   | 测试名                             | 测试错误                            | 错误原因                                       | 修改                                                         |
   | ---------------------------------- | ----------------------------------- | ---------------------------------------------- | ------------------------------------------------------------ |
   | TestLeaderBcastBeat2AA             | msg 中 Index 和 LogTerm 应该为 0    | 因为心跳传输中添加了 Index 和 LogTerm          | 看了[etcd](https://bbs.huaweicloud.com/blogs/110887)之后发现，是在 Leader 接受 HeartBeatRsp 的时候，再对 Follower 进行 Append的 |
   | TestLeaderElectionInOneRoundRPC2AA | #8结果为 candidate，而不是 Follower | 在接受 VoteResp 的时候主动把 State = Candidate | 把处理 VoteResp 过程中，退化成 Follower 的代码删掉           |
   | TestSplitVote2AA                   |                                     |                                                | 同上                                                         |

3. 进行了 2 的修改之后，make project2aa PASS，make project2ab 有以下错误

   | 测试名                                  | 测试错误                                                     | 错误原因                                                     | 修改                                                         |
   | --------------------------------------- | ------------------------------------------------------------ | ------------------------------------------------------------ | ------------------------------------------------------------ |
   | TestLeaderElectionOverwriteNewerLogs2AB | node 1 选举失败后，state = StateCandidate, want StateFollower | 与上面的 TestLeaderElectionIn OneRoundRPC2AA 冲突            | 写回选举失败后直接变成 Follower 的代码，把判断条件 >= 改成 >。上面的test中的#8，只有两个拒绝，并没有真的有 3 个拒绝，它还是有可能选举成功的，所以 stay candidate |
   | TestDuelingCandidates2AB                | Term 没有转成大的 m.Term; State 没有转成 Follower            | 由于 Term 大，所以 Leader 和 Follower 的都转为 Follower ，且 Term 也转为 m.Term; 由于 LogTerm 小，所以也会拒绝 candidate 的投票请求 | 对于 m.Term 大的情况，把节点转为 Follower, 并继续后面的投票。之前 Leader 转 Follower 之后直接结束了 |

4. 进行了 3 的修改之后，make project2aa 报错

   | 测试名                           | 测试错误  | 错误原因                                                     | 修改                                            |
   | -------------------------------- | --------- | ------------------------------------------------------------ | ----------------------------------------------- |
   | TestCandidateStartNewElection2AA | term 错误 | 在 candidate 处理 MsgHup 的时候，进行了两次 becomeCandidate() ，所以 Term 自增了两次 | 删除 candidate 处理 MsgHup 时的 becomeCandidate |

5. 进行 4 的修改之后， make project2aa PASS ，继续调试 2ab

   | 测试名                             | 测试错误              | 错误原因                                                     | 修改                                                      |
   | ---------------------------------- | --------------------- | ------------------------------------------------------------ | --------------------------------------------------------- |
   | TestHandleMessageType_MsgAppend2AB | #5-10: committed 错误 | 直接让 ommitted = m.commit                                   | 改成 min(m.Commit, m.Index+len(m.Entries))                |
   | TestAllServerStepdown2AB           | Lead 错误             | 投票的时候直接让 r.Lead = m.From, 这个时候还不知道 Leader 是谁; append 的时候没有指定 Lead | 投票的时候， r.Lead = None; append 的时候 r.lead = m.From |

6. 2a 通过之后，做 2b 的过程中，涉及到初始 Index 的问题，即初始化时对 lastIndex, applyIndex, committedIndex, firstIndex 的取值

   修改 1：把之前的 `offset = entries[0].Index` 改成 `offset, _ = storage.FirstIndex()` 。

   修改 2：之前对 raftLog 结构体的理解有误，进行了修改

   | 成员              | 备注（修改后）                                 | 备注（修改前）                                    |
   | ----------------- | ---------------------------------------------- | ------------------------------------------------- |
   | storage           |                                                | 包含快照以来的所有持久化数据                      |
   | committed         | [first:committed] ，表示 commit 的最后一个下标 | [first:committed)，表示个数，初始化为**0**        |
   | applied           | [first:applied] ，表示 apply 的最后一个下标    | [first:applied)，表示个数，初始化为**0**          |
   | stabled           | [first:stable] ，表示 stable 的最后一个下标    | [first:stable), 表示个数，存storage里的持久化数据 |
   | entries           | [first:last]                                   | [first:last]                                      |
   | pendingSnapshot   |                                                |                                                   |
   | **函数**          |                                                | **备注**                                          |
   | Term(i *uint64*)  |                                                | 返回下标为 i 的Term                               |
   | LastIndex()       |                                                | 返回 entries 的最后一个下标                       |
   | nextEnts()        | 返回 (applied, committed] 的日志               | 返回 [applied, committed) 的日志                  |
   | unstableEntries() | 返回 (stable, last] 的日志                     | 返回 [stabled, last] 的日志                       |

   make project2a 报错

   | 测试名                   | 测试错误                                | 错误原因                                                     | 修改                                                      |
   | ------------------------ | --------------------------------------- | ------------------------------------------------------------ | --------------------------------------------------------- |
   | TestProgressLeader2AB    | Prs 与正确输出不符                      | becomeLeader 中，Prs 初始化出现了问题                        | 在 becomeLeader 中，对自己的 match, next 初始化时，多加 1 |
   | TestDuelingCandidates2AB | a, b 的 committed 错误，要求 1 ，实际 0 |                                                              | 同上                                                      |
   | TestRawNodeStart2AC      | committedEntries 错误，要求 1，实际 2   | becomeLeader 的时候，如果只有自己一个节点，要直接把 committed 改成 r.LastInde() |                                                           |

### part 2b

1. 各个节点之间不能通信，刚开始创建节点的时候没有把其他 peer 的信息放到 cfg 中

   解决：发现 cfg.storage.ConfState 中存放着节点信息，可以用来初始化 raft

2. scan resp 出错

   ![image-20210130020245270](\第一次调试.jpg)

   <img src="\第一次调试2.jpg" alt="image-20210130020316276" style="zoom:50%;" />

   3. 解决：按 md 中的要求，给 snap.resp 添加相应的参数。调试过程中发现，在 append 之后运行 nextEnts 会出现 range[-5:] 的情况。检查后发现，在 handleAppend 中对 stabled 的赋值出现了错误， `r.RaftLog.stabled = index - offset` 改成 ``r.RaftLog.stabled = index - 1` 。
   
   4. make project2b 报错
   
      | 测试名                           | 测试错误                                                     | 错误原因                                                     | 修改                                        |
      | -------------------------------- | ------------------------------------------------------------ | ------------------------------------------------------------ | ------------------------------------------- |
      | TestManyPartitionsManyClients2B  | 1. get wrong value, client 2;                                | 之前都没出错，突然 scan 出错，而且从 46 变成了 25 ，应该是被截断了 |                                             |
      |                                  | 2. 0 missing element x 0 35 y in Append result;              | 可能是 put 最后一个 kv ，没有存储成功。下面有一种情况，是上层的 leader 没有转变过来 | 在接收到command 的时候判断自身是否是 leader |
      | 频繁更换 leader ，               | 3. runtime error: slice bounds out of range [261:259]。applied > committed, 261 > 259 | 查看 applied 和 committed 的赋值发现，applied 完全是跟着 committed 走的，不会出错。所以只有可能是 committed 自己变小了 |                                             |
      |                                  | 3                                                            | 是这样的，有一个旧的 leader ，它的 committed 比新的 leader 的 committed 要大，然后经过一次 partition 之后，他们两被分到了一起，新 leader 给旧 leader 发心跳，更新了旧 leader 的 committed ，触发了 Ready ，然后调用了 nextEntries 发生了错误 |                                             |
      |                                  |                                                              |                                                              |                                             |
      | TestManyPartitionsOneClient2B    | get wrong value                                              |                                                              |                                             |
      | TestPersistConcurrent2B          | 3 missing element x 3 249 y in Append result                 |                                                              |                                             |
      | TestPersistPartition2B           | get wrong value, client 3                                    |                                                              |                                             |
      | TestPersistPartitionUnreliable2B | 1. request timeout; 2. runtime error: slice bounds out of range [275:273] |                                                              |                                             |
   
   - TestManyPartitionsManyClients2B
   
     ![image-20210201215858334](\记录1.png)
   
     上图为什么 peer 2 需要重新竞选一次 leader ？
   
     ![image-20210201214713906](\记录2.png)
   
     现象：在 peer 1 成为 Leader 后，peer 2 对 put 4 47 进行了第二次的 propose
   
     原因：
   
     - 第一次 propose ：在 partition 之后进行的，这个时候， leader 2 只能 propose 给 follower 4 ，所以 peer 1/3/5 都没有收到这段时间 propose 的日志；（疑问， leader 只收到少数个 heartbeatresp 的时候是不是应该退出 leader）
   - 在 peer 1/3/5 竞选出 leader 1 后，又进行了一次 partition ，这时， 
     
   - 在多次测试中， `TestManyPartitionsOneClient2B` 也出现了 `wrong value` 的错误。所以基本可以确定这个错误是存在代码中的，只是 `manyClient` 的 request 次数成倍增加，所以增大了错误出现的概率。而且每次报错的时候都是在更换 Leader 之后，经过之前对日志的观察，初步判断出错是因为**返回 callback 的时机不对**，一个 callback 应该代表着大多数 peers 都 applied 之后，才成功。
   
   - > [TiKV 源码解析系列文章（十八）Raft Propose 的 Commit 和 Apply 情景分析](https://pingcap.com/blog-cn/tikv-source-code-reading-18/)
   
   ![runtime error](\记录3.png)
   
   ![get wrong value](\记录4.png)

![image-20210305151758769](\记录5.png)

![image-20210305190540045](\记录6.png)

总结一下，总算是解决了。

- `get wrong value` 这个问题是因为 raft 在经过 partition 之后，客户端还是会请求旧 leader （即少数那部分的 leader ）。而我一开始写的只读操作是在接受到 request 之后直接回复的，毕竟 scan 操作只需要返回一个 txn 和 region 就可以了，图省事结果卡了这么久！！！！！

- `committed < applied` 是因为 partition 之后，新 leader 和旧 leader 恰好分到了一起，而且旧 leader 的 committed 比新 leader 大，但是新 leader 的 term 比旧 leader 大，所以旧 leader 的 committed 就被替换成了新 leader 的 committed 了。而旧 leader 之前是 committed  = applied 的，这个时候旧 leader 的 committed 就比 applied 小了，从而触发了 Ready，出现了错误。

- 还有一个问题：因为 snap 需要返回一个 txn ，但是 `cb.Txn = d.ctx.engine.Kv.NewTransaction(false)` 这句放到 `proposeRaftCommand` 中就可以运行，放到 `HandleRaftReady` 中就会报错，暂时不知道是因为什么。

  ![image-20210306175517733](\记录7.png)

- TODO: 现在添加日志时寻找 `nextIndex` 是一个一个向前遍历的，论文中有更好的方法。

### part 2c

1. | 完成情况 | 测试名                      | 测试内容                                                     | 测试结果 |
   | -------- | --------------------------- | ------------------------------------------------------------ | -------- |
   | √        | TestRestoreSnapshot2C       | handleSnapshot 之后。lastIndex = Snapshot.Index; Term[lastIndex] = Snapshot.Term; ConfState.Nodex = Snapshot.Nodes |          |
   | √        | TestRestoreIgnoreSnapshot2C | if (Snapshot.commit < r.committed)  ignore                   |          |
   | √        | TestProvideSnap2C           | handleSnapshot 之后。执行 propose ，send 的 msg 是 MsgSnapshot |          |
   | √        | TestRestoreFromSnapMsg2C    | 添加 Step 中对 Snapshot 的处理，if m.Term > r.Term { becomeFollower } |          |
   | √        | TestSlowNodeRestore2C       |                                                              |          |

2. make project2c 基本能通过测试，但是偶尔会出现下面问题，TestManyPartitionsManyClients2B 和 TestSnapshotUnreliableRecoverConcurrentPartition2C 都出现过。出现频率不大。

   还有个问题：

   - peer_storage.go 和 peer.go 中注明需要在 2c 完成的代码，**我还没写，但是测试代码基本就跑过了**。。。。

3. 一些错误：

   - TestManyPartitionsManyClients2B 再次出现 missing element 的错误，猜测是遗留问题。

     ![image-20210308133432668](\记录8.png)

   - 在 TestManyPartitionsManyClients2B，TestPersistPartition2B ，TestPersistPartitionUnreliable2B   中 get wrong value 也出现了。

     ![image-20210308133824194](\记录9.png)

     ![image-20210308134618644](\记录10.png)

     ![image-20210308135053188](\记录11.png)

     