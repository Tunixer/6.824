package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import "sync"
import "labrpc"

// import "bytes"
// import "labgob"



//
// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in Lab 3 you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh; at that point you can add fields to
// ApplyMsg, but set CommandValid to false for these other uses.
//
type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int
}


/*
Entry is the type of element in Log
*/
type Entry struct{
	Term int
	Command interface{}
}

//
// A Go object implementing a single Raft peer.
//
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	muVote    sync.Mutex
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]

	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.
	
	//State Indicate present state. One of "Leader","Candidate","Follower"
	State       String 
	//Persistent state on all servers:
	currentTerm int
	votedFor    int
	log         []Entry

	//Volatile state on all servers:
	commitIndex int
	lastApplied int
	
	//Volatile state on leaders:
	nextIndex   []int
	matchIndex  []int

	//Synchronization Channel
	heartbeatCh chan bool
	BecomeLeader chan bool
	OutOfTimeCh chan int
	CommitCh chan int
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {

	var term int
	var isleader bool

	// Your code here (2A).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	term = rf.currentTerm
	isleader = false
	if rf.State == "Leader"{
		isleader = true
	}

	return term, isleader
}

func (rf *Raft) GetStt() string {
	/*
	This function may not need use RLock
	*/
	rf.mu.RLock()
	defer rf.mu.Unlock()
	stt := rf.State
	return stt
}

func (rf *Raft) GetCurrTerm() int {
	term := rf.currentTerm
	return term
}

func (rf *Raft) IsOutOfTime(thisTerm int) (bool,bool){
	currTerm := rf.GetCurrTerm()
	if currTerm < thisTerm {
		go func(){
			if rf.GetCurrTerm() < thisTerm {
				rf.OutOfTimeCh <- thisTerm
			}
		}()
		/*
		2019/3/9
		this position may need Mutex
		*/
		rf.State = "follower"
		rf.currentTerm = thisTerm
		rf.votedFor = -1
		rf.voteCount = 0
		return true, false
	} else if currTerm == thisTerm {
		return false, false
	} else {
		return false, true
	}
}

//
// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
//
func (rf *Raft) persist() {
	// Your code here (2C).
	// Example:
	// w := new(bytes.Buffer)
	// e := labgob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// data := w.Bytes()
	// rf.persister.SaveRaftState(data)
}


//
// restore previously persisted state.
//
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	// Your code here (2C).
	// Example:
	// r := bytes.NewBuffer(data)
	// d := labgob.NewDecoder(r)
	// var xxx
	// var yyy
	// if d.Decode(&xxx) != nil ||
	//    d.Decode(&yyy) != nil {
	//   error...
	// } else {
	//   rf.xxx = xxx
	//   rf.yyy = yyy
	// }
}




//
// example RequestVote RPC arguments structure.
// field names must start with capital letters!
//
type RequestVoteArgs struct {
	// Your data here (2A, 2B).
	Term int
	CandidateId int
	LastLogIndex int
	LastLogTerm int
}

//
// example RequestVote RPC reply structure.
// field names must start with capital letters!
//
type RequestVoteReply struct {
	// Your data here (2A).
	Term int
	VoteGranted bool
}
/*
2019/3/9
return true if Log1 is more up-to-date than Log2
*/
func IsUptoDate(idx1 int, term1 int, idx2 int, term2 int) bool {
	res := false
	if (term1 > term2){
		res = true
	}
	if (term1 == term2) && (idx1 > idx2){
		res = true
	}
	return res
}

//
// example RequestVote RPC handler.
//
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	//
	currTerm := rf.currentTerm
	currIdx := len(rf.log) - 1
	/*
	2019/3/8
	This if statement is used to predict whether we should transfer 
	rf.State from "Leader" to "follower"
	Cautious:
	1. Maybe this time, 
	*/
	if currTerm < args.Term {
		/*
		// Maybe we do not need OutOfTimech
		go func(){
			if rf.GetCurrTerm() < args.Term {
				rf.OutOfTimeCh <- args.Term
			}
		}()
		*/
		rf.State = "follower"
		rf.votedFor = -1
		rf.voteCount = 0
	}

	if args.Term < currTerm {
		reply.Term = currTerm
		reply.VoteGranted = false
		return
	}
	if rf.State == "follower"{
		cdt1 := (rf.votedFor == -1) || (rf.votedFor == args.CandidateId) 
		cdt2 := IsUptoDate(currIdx, rf.log[currIdx].Term, args.LastLogIndex, args.LastLogTerm)
		if cdt1 && !cdt2 {
			reply.Term = currTerm
			reply.VoteGranted = true
			rf.votedFor = args.CandidateId
			return
		} else{
			reply.Term = currTerm
			reply.VoteGranted = false
			return		
		}
	}

}

func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	if ok{
		rf.IsOutOfTime(reply.Term)
	}
	return ok
}

func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	oot1, oot2 := rf.IsOutOfTime(args.Term)
	if len(args.Entries) == 0{
		if oot1 {
			reply.Term = rf.GetCurrTerm()
			return
		}
		rf.heartbeatCh <- true
		reply.Term = rf.GetCurrTerm()
		cdt := false
		if len(rf.log) >= args.PrevLogIndex + 1 {
			if rf.log[args.PrevLogIndex].Term == args.PrevLogTerm{
				cdt = true
			}
		}
		if rf.lastApplied < args.LeaderCommit && cdt {
//			fmt.Printf("%v %v 's Log %v in Term %v When HB\n rf.cmtIndex:%v args.LeaderCommit:%v\n",
//				rf.State,rf.me,rf.log,rf.currentTerm,rf.cmtIndex,args.LeaderCommit)
			rf.cmtIndex = args.LeaderCommit
			go func(){
				rf.CommitCh <- rf.cmtIndex
			}()
		}
		return
	}else{
		if oot2{
			reply.Success = false
			reply.Term = rf.GetCurrTerm()
			return
		}
		cdt := false
		if len(rf.log) >= args.PrevLogIndex + 1 {
			if rf.log[args.PrevLogIndex].Term == args.PrevLogTerm{
				cdt = true
			}
		}

		if cdt{
//			fmt.Printf("%v %v 's Log %v in Term %v Before Append Entries\n",
//				rf.State,rf.me,rf.log,rf.currentTerm)
			if len(rf.log) == args.PrevLogIndex + 1{
				rf.log = append(rf.log,args.Entries...)
			}
			if len(rf.log) > args.PrevLogIndex + 1{
				rf.log = append(rf.log[:args.PrevLogIndex + 1],args.Entries...)
			}

			if rf.lastApplied < args.LeaderCommit{
				rf.cmtIndex = args.LeaderCommit
				go func(){
					rf.CommitCh <- rf.cmtIndex
				}()
			}
//			fmt.Printf("%v %v 's Log %v in Term %v After Append Entries From Leader %v in Term:%v\n",
//				rf.State,rf.me,rf.log,rf.currentTerm,args.LeaderId,args.Term)
		}
		reply.Term = rf.currentTerm
		reply.Success = cdt
		go rf.persist()
	}
}



//
// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// The labrpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost.
// Call() sends a request and waits for a reply. If a reply arrives
// within a timeout interval, Call() returns true; otherwise
// Call() returns false. Thus Call() may not return for a while.
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply.
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
//
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	if ok {
		rf.muVote.Lock()
		if reply.VoteGranted{
			rf.voteCount += 1
			if rf.State == "Candidate" && rf.voteCount > len(rf.peers)/2 {
				rf.BecomeLeader <- true
			}
		}
		rf.IsOutOfTime(reply.Term)
		rf.muVote.Unlock()
	}
	return ok
}


//
// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
//
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	index := -1
	term := -1
	isLeader := true

	// Your code here (2B).
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

//
// the tester calls Kill() when a Raft instance won't
// be needed again. you are not required to do anything
// in Kill(), but it might be convenient to (for example)
// turn off debug output from this instance.
//
func (rf *Raft) Kill() {
	// Your code here, if desired.
}

//
// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
//
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me

	// Your initialization code here (2A, 2B, 2C).
	rf.State = "follower"
	rf.votedFor = -1
	rf.currentTerm = 0
	rf.lastApplied = 0
	rf.cmtIndex = 0
	//the first element in the log must be nil to be maintained easily
	rf.log = append(rf.log,Entry{0,nil})

	rf.heartbeatCh = make(chan bool)
	rf.OutOfTimeCh = make(chan int)
	rf.BecomeLeader = make(chan bool)
	rf.CommitCh = make(chan int)
	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	go Loop()
	go rf.CommitMonitor(applyCh)
	return rf
}


func ElectTimeOut() int {
	res := rand.Intn(50) + 300
	return res
}

/*
2019/3/8
a typical state machine, and states transfer according to 
*/

func (rf *Raft) Loop(){
	TimeOutCon := 0
	for {
		TimeOutCon = ElectTimeOut()
		switch rf.GetStt(){
		case "Follower":
			select{
			case <- rf.heartbeatCh:
			case <- time.After(time.Duration(TimeOutCon) * time.Millisecond):
				if rf.GetStt() != "Leader"{
					rf.mu.Lock()
					rf.State = "Candidate"
					rf.currentTerm += 1
					rf.mu.Unlock()
				}
			}
		case "Candidate":
			rf.CandidateRt()
		case "Leader":
			rf.LeaderRt()
		}
	}
}

func (rf *Raft) LeaderRt(){
	TimeOutCon := ElectTimeOut()
	rf.mu.Lock()
	if rf.State == "Leader"{
		var args AppendEntriesArgs
		args.LeaderId = rf.me
		args.Term = rf.GetCurrTerm()
		for k, _ := range rf.peers{
			if k!= rf.me && rf.State == "leader"{
				var reply AppendEntriesReply
	
				args.LeaderCommit = rf.cmtIndex
				args.PrevLogIndex = rf.cmtIndex
				args.PrevLogTerm = rf.log[rf.cmtIndex].Term
	
				go rf.sendAppendEntries(k, &args, &reply)
			}
		}
		logLen := len(rf.log)-1
		currTerm := rf.currentTerm
		go rf.CommitLog(logLen, currTerm)
		select{
		case <- time.After((time.Duration(TimeOutCon))/8 * time.Millisecond):	
		}
	}
	rf.mu.Unlock()
}


func (rf *Raft) CandidateRt(){
	TimeOutCon := ElectTimeOut()
	rf.mu.Lock()
	defer rf.mu.Unlock()
	rf.votedFor = rf.me
	rf.voteCount = 1

	var args RequestVoteArgs
	args.CandidateId = rf.me
	args.Term = rf.currentTerm
	args.LastLogIndex = len(rf.log)-1
	args.LastLogTerm = rf.log[args.LastLogIndex].Term
	for k, _ := range rf.peers{
		if k!= rf.me && rf.State == "Candidate"{
			var reply RequestVoteReply
			go rf.sendRequestVote(k, &args, &reply)
		}
	}
	select{
	case <- rf.BecomeLeader:
		rf.State = "Leader"
		for i:=0 ;i < len(rf.peers);i++{
			rf.nextIndex = append(rf.nextIndex, -1)
		}
		
		return
	case <- time.After((time.Duration(TimeOutCon)) * time.Millisecond):
		if rf.State != "Leader" {
			rf.currentTerm += 1
		}
		return
	}
	case <- rf.BecomeLeader:
}


func (rf *Raft) CommitMonitor(applyCh chan ApplyMsg){
	for{
		select{
		case <- rf.CommitCh:
			go func(){
				rf.mu.Lock()
				defer rf.mu.Unlock()
				oldApplied := rf.lastApplied
				for i:=oldApplied+1 ;i<= rf.cmtIndex;i++ {
					if i> len(rf.log)-1{
						return
					}
					appendMsg := ApplyMsg{Index: i, Command: rf.log[i].Command}
					applyCh <- appendMsg
					rf.lastApplied = i
//					fmt.Printf("%v %v Apply Log: %v in Term %v\n",rf.State,rf.me,appendMsg,rf.currentTerm)
				}
				go rf.persist()
				return
			}()
		}
	}
}