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
