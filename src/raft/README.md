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
Leader的选举
