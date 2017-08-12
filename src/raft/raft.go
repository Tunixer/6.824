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

import(
	"sync"
	"labrpc"
	"time"
	"math/rand"
//	"fmt"
)

// import "bytes"
// import "encoding/gob"



//
// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make().
//
type ApplyMsg struct {
	Index       int
	Command     interface{}
	UseSnapshot bool   // ignore for lab2; only used in lab3
	Snapshot    []byte // ignore for lab2; only used in lab3
}


type Entry struct{
	Index int
	term int
	Command interface{}
}
//
// A Go object implementing a single Raft peer.
//
type Raft struct {
	mu        sync.RWMutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]

	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.
	//Persistent state
	State       string
	currentTerm int
	votedFor    int
	voteCount   int
	log         []Entry

	//Volatile State
	cmtIndex    int//commited index
	lastApplied int

	//Volatile State on Leaders
	nextIndex   []int
	matchIndex  []int

	heartbeatCh chan bool
	BecomeLeader chan bool
	OutOfTimeCh chan int
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {

	var term int
	var isleader bool
	// Your code here (2A).
	rf.mu.RLock()
	term = rf.currentTerm
	if rf.State == "leader"{
		isleader = true
	} else {
		isleader = false
	}
	rf.mu.RUnlock()
	return term, isleader
}

func (rf *Raft) GetCurrTerm() int {
	rf.mu.RLock()
	term := rf.currentTerm
	rf.mu.RUnlock()
	return term
}

func (rf *Raft) GetStt() string {
	rf.mu.RLock()
	stt := rf.State
	rf.mu.RUnlock()
	return stt
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
	// e := gob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// data := w.Bytes()
	// rf.persister.SaveRaftState(data)
}

//
// restore previously persisted state.
//
func (rf *Raft) readPersist(data []byte) {
	// Your code here (2C).
	// Example:
	// r := bytes.NewBuffer(data)
	// d := gob.NewDecoder(r)
	// d.Decode(&rf.xxx)
	// d.Decode(&rf.yyy)
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
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

//
// example RequestVote RPC handler.
//
type AppendEntriesArgs struct {
	Term int //Leader's term
	LeaderId int //Used by followers to redirect clients
	PrevLogIndex int
	PrevLogTerm int
	Entries[] Entry
	LeaderCommit int
}

type AppendEntriesReply struct{
	Term int
	Success bool
}

func (rf *Raft) IsOutOfTime(thisTerm int) (bool,bool){
	currTerm := rf.GetCurrTerm()
	if currTerm < thisTerm {
		go func(){
			if rf.GetCurrTerm() < thisTerm {
				rf.OutOfTimeCh <- thisTerm
			}
		}()
		rf.mu.Lock()
		rf.State = "follower"
		rf.currentTerm = thisTerm
		rf.votedFor = -1
		rf.voteCount = 0
		rf.mu.Unlock()
		return true, false
	} else if currTerm == thisTerm {
		return false, false
	} else {
		return false, true
	}
}


//true if Log1 is more up-to-date than Log2
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

func (rf *Raft) RequestVote(args RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).
	oot, _ := rf.IsOutOfTime(args.Term)
	if oot {
	}

	rf.mu.RLock()
	currTerm := rf.currentTerm
	currIdx := len(rf.log) - 1
	rf.mu.RUnlock()

	temp_state := rf.GetStt()
	reply.Term = currTerm
	switch temp_state {
	case "leader","candidade":
		reply.Term = currTerm
		reply.VoteGranted = false
		return
	case "follower":
		if args.Term < currTerm{
			reply.Term = currTerm
			reply.VoteGranted = false
			return
		}
		rf.mu.RLock()
		cdt1 := (rf.votedFor == -1) || (rf.votedFor == args.CandidateId) 
		cdt2 := IsUptoDate(currIdx, rf.log[currIdx].term, args.LastLogIndex, args.LastLogTerm)
		rf.mu.RUnlock()
		if cdt1 && !cdt2 {
			reply.Term = currTerm
			reply.VoteGranted = true
			rf.mu.Lock()
			rf.votedFor = args.CandidateId
			rf.mu.Unlock()
			return
		} else{
			reply.Term = currTerm
			reply.VoteGranted = false
			return		
		}
	}
}

func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	oot1, _ := rf.IsOutOfTime(args.Term)
	if oot1 {
		reply.Term = rf.GetCurrTerm()
		return
	}
	rf.heartbeatCh <- true
	reply.Term = rf.GetCurrTerm()
	return
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

	ok := rf.peers[server].Call("Raft.RequestVote", *args, reply)
	if ok {
		if reply.VoteGranted{
			rf.mu.Lock()
			rf.voteCount += 1
			if rf.State == "candidate" && rf.voteCount > len(rf.peers)/2 {
				rf.BecomeLeader <- true
			}
			rf.mu.Unlock()
		}
		rf.IsOutOfTime(reply.Term)
	}
	return ok
}

func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
 	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
 	if ok{
 		rf.IsOutOfTime(reply.Term)
 	}
 	return ok
}

//
// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election.
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

func ElectTimeOut() int {
	res := rand.Intn(300) + 800
	return res
}

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
					rf.mu.Unlock()
				}
			}
		case "candidate":
			rf.CandidateRt()//candidate routine
		case "leader":
			rf.LeaderRt()
		}
		rf.mu.Lock()
		rf.currentTerm += 1
		rf.mu.Unlock()
	}
}

func (rf *Raft) CandidateRt(){
	time.Sleep(17 * time.Millisecond)

	rf.mu.Lock()
	rf.votedFor = rf.me
	rf.voteCount = 1
	rf.mu.Unlock()
	var args RequestVoteArgs
	rf.mu.RLock()
	args.CandidateId = rf.me
	args.Term = rf.currentTerm
	args.LastLogIndex = len(rf.log)-1
	args.LastLogTerm = rf.log[args.LastLogIndex].term
	rf.mu.RUnlock()
	for k, _ := range rf.peers{
		if k!= rf.me && rf.GetStt() == "candidate"{
			var reply RequestVoteReply
			go rf.sendRequestVote(k, &args, &reply)
		}
	}
	select{
	case <- rf.BecomeLeader:
		rf.mu.Lock()
		rf.State = "leader"
		rf.mu.Unlock()
		return
	case <- time.After(666* time.Millisecond):
		if rf.State != "leader" {
			rf.State = "follower"
		}
		return
	}
}

func (rf *Raft) LeaderRt(){
	time.Sleep(10 * time.Millisecond)
	var args AppendEntriesArgs
	args.LeaderId = rf.me
	args.Term = rf.GetCurrTerm()
	for k, _ := range rf.peers{
		if k!= rf.me && rf.GetStt() == "leader"{
			var reply AppendEntriesReply
			go rf.sendAppendEntries(k, &args, &reply)
		}
	}

	select{
	case <-rf.OutOfTimeCh:
	}
}

func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me
	rf.State = "follower"
	rf.votedFor = -1
	rf.currentTerm = 0
	rf.heartbeatCh = make(chan bool)
	rf.BecomeLeader = make(chan bool)
	rf.log = append(rf.log,Entry{0,0,nil})
	// Your initialization code here (2A, 2B, 2C).

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	go rf.Loop()
	return rf
}
