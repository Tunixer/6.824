Lab 2 of 6.824—Implementation of a Toy Raft
---
本次作业2017年8月初，大作业的主要目标是对MIT 6.824所提供的框架代码加以完善，提供的框架并没有过多的细节，提供框架的主要目的是方便对程序进行测试。
实现的理论依据是Ongaro所表的关于Raft的论文。

主要流程是将整个Raft协议分成三部分：Leader Election,Log Replication和State Persistence，进而对三个功能模块分别实现。

Raft理论和实现的难点
---
### 0.1 基本框架的实现
![Figure 1](https://github.com/Tunixer/6.824/raw/master/Lecture/Figure/figure1.png "Figure 1")

我实现这一Lab的出发点是基于这张图片,它描述了Raft的状态变化通过进一步实现图片中的细节可以非常容易地实现Leader Election的状态

这一状态变换的具体实现是`raft.go`的`Loop()`:
```go
func (rf *Raft) Loop(){
	TimeOutCon := 0
	for {
		TimeOutCon = ElectTimeOut()
		switch rf.GetStt(){
		case "follower":
			select{
			case <- rf.heartbeatCh:
			case <- time.After(time.Duration(TimeOutCon) * time.Millisecond):
				if rf.GetStt() != "leader"{
					rf.mu.Lock()
					rf.State = "candidate"
					rf.currentTerm += 1
					rf.mu.Unlock()
				}
			}
		case "candidate":
			rf.CandidateRt()//candidate routine
		case "leader":
			rf.LeaderRt()
		}
	}
}
```

### 1.1 Leader Election
Leader Election个人认为实现的难点主要有两点：
1.获得多数票时如何通知`Loop()`跳转状态
2.当自己落后于对方RPC的Term时，如何退出当前工作并退回Follower状态

为了使得当server落后于对方的Term时立刻返回Follower状态，我在这里实现了一个函数`IsOutOfTime(thisTerm int)`，这个函数的要出现在两个位置，一个是处理RPC请求的函数，即`AppendEntries`和`RequestVote`两个函数，每次执行这两个函数时都要先执行`IsOutOfTime(thisTerm int)`，另一个位置则是请求即`AppendEntries`和`RequestVote`的函数
比如在`AppendEntries`：
```go
func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	oot1, oot2 := rf.IsOutOfTime(args.Term)
	....
	return ....
}
```
我们首先要执行的是`IsOutOfTime()`，通过其给出的两个返回值判断出自己和对方的Term的大小比较结果，后面的所有执行过程都需要依据这个函数的返回值来决定。
此外，在执行完`SendAppendEntries`之类的请求之后，也会得到对方的Term，这时也要进行处理。
比如在`sendRequestVote`:
```go
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {

	ok := rf.peers[server].Call("Raft.RequestVote", *args, reply)
	if ok {
	......
		}
		rf.IsOutOfTime(reply.Term)
	}
	return ok
}
```
每次接受完请求，都需要调用`IsOutOfTime()`来进行处理
为了让`IsOutOfTime()`通知`Loop()`自己的Term过期了，我们在这各个状态之间建立管道来交互信息
```go
func (rf *Raft) IsOutOfTime(thisTerm int) (bool,bool){
	currTerm := rf.GetCurrTerm()
	if currTerm < thisTerm {
		go func(){
			if rf.GetCurrTerm() < thisTerm {
				rf.OutOfTimeCh <- thisTerm//把过期信息传入管道
			}
		}()
		......
}

func (rf *Raft) LeaderRt(){
	......
	select{
	case <- rf.OutOfTimeCh://收到管道传来的信息
	case <- time.After((time.Duration(TimeOutCon))/8 * time.Millisecond):
	}
}
```
通过这样的同步过程，可以保证Leader Election的正确

### 1.2 Log Replication
在`Raft`协议里，最重要的就是保持状态机的一致性，而最主要的途径就是保证每个Server都能够正确地`Replicate`Leader的`log`。在协议中，保持这一一致性的具体实现是通过维护一个指向`log`的指针`CommitIndex`，在`CommitIndex`之前的`log`的内容是无法被改变的，也是可以安全的应用到状态机上去的，从而保持了各个服务器之间状态机的一致性。
一个`Log Entry`只有在Leader所发送的`AppendEntriesRPC`被过半数的服务器所接受才能够被称之为`Commited`的`Log Entry`，只有`Commited`的`Log Entry`才能够被应用到状态机。

服务器的接收Client的请求和自己发送`AppendEntriesRPC`应该保持各自独立，在我自己的实现里是这样做的:
```go
func (rf *Raft) Start(command interface{}) (int, int, bool) {
    ......
	if rf.State == "leader"{
		appEntries := Entry{Term :rf.GetCurrTerm(),Command:command}
		term = appEntries.Term
		rf.log = append(rf.log, appEntries)
		index = len(rf.log)-1
	} else {
		isLeader = false
	}
	return index, term, isLeader
}
func (rf *Raft) LeaderRt(){
    ......//这里是执行发送心跳包的过程
	rf.mu.RLock()
	logLen := len(rf.log)-1
	currTerm := rf.currentTerm
	rf.mu.RUnlock()
	go rf.CommitLog(logLen, currTerm)//CommitLog执行了发送AppendEntries的具体过程
	......
}
```
在`Raft`协议中为了保证大家的`Commit`的日志是同步的，采取了`Leader`overwrite `Follower`的`log`的方式，为了保证`Leader`的`AppendEntriesRPC`请求能够保证整个集群的日志的同步，接收`AppendEntriesRPC`的服务器需要保证两件事情，第一个是要保证`Candidate`的日志是相对更加`up-to-date`( Section 5.4.1)，否则就不能让`Candidate`被选成`Leader`。另一个则是需要保证`AppendEntriesRPC`是完全复制`Leader`的`log`,文章提出通过在进行添加新日志之前进行一次Consistency Check(Section 5.3)，只有通过Consistency Check才可以进行进一步的`AppendEntries`操作。一旦Leader的`AppendEntriesRPC`被拒绝了，Leader会不断的寻找之前的`Log Entity`,从能通过Consistency Check的那个`Entity`开始复制日志。

### 1.3 Log Replication的实现难点
我个人觉得这里实现的难点有两点，一点是如何实现`Leader`和`Follower`之间Consistency Check，另一点则是如何注意自身的Term过期的处理。
我实现Consistency Check是采用在第一次`AppendEntriesRPC`失败之后，调用一个`rectifyAppendEntries()`,这一函数用于处理寻找到一致的`Log Entity`并且复制这之后的整个日志。

值得一提的是，在实现这一系列完整的`AppendEntries`的功能时，我们依旧需要注意在1.1里所提到的自身的Term过期的问题。同时，我们在这里注意一点问题，就是当收到自己的Term过期的消息时，`Leader`会立刻退回`Follower`状态，这时候后面所需要执行的`AppendEntriesRPC`都不应该继续执行。
我的实现如下:
```go
func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
 	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
 	if ok{
 		rf.IsOutOfTime(reply.Term)//每次发送完都要注意自己的Term的过期问题
 	}
 	return ok
}

func (rf *Raft) rectifyAppendEntries(server int,idx int) bool{
	i := idx
	for {
			reply := &AppendEntriesReply{}
			rf.mu.RLock()
			args := &AppendEntriesArgs{Term:rf.currentTerm,
				                       LeaderId:     rf.me,
				                       Entries:      rf.log[i:i+1],
				                       PrevLogIndex: i-1,
				                       PrevLogTerm:  rf.log[i-1].Term,
				                       LeaderCommit: rf.cmtIndex,
			                          }
			rf.mu.RUnlock()
			if rf.GetStt() != "leader"{//每次发送完RPC之后都要查看自己是否因为Term过期而落回Follower状态
				return false
			}
			ok := rf.sendAppendEntries(server, args, reply)
			if ok{
				if reply.Success == true{
					break
				}
			}
			i = i-1
	}
	reply := &AppendEntriesReply{}
	rf.mu.RLock()
	args := &AppendEntriesArgs{Term:         rf.currentTerm,
		                       LeaderId:     rf.me,
		                       Entries:      rf.log[i+1:idx+1],
		                       PrevLogIndex: i,
		                       PrevLogTerm:  rf.log[i].Term,
		                       LeaderCommit: rf.cmtIndex,
	                          }
	rf.mu.RUnlock()
	if rf.GetStt() != "leader"{
		return false
	}
	ok := rf.sendAppendEntries(server, args, reply)
	return ok
}
```

