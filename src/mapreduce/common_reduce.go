package mapreduce

import (
	"encoding/json"
	"os"
	"sort"
)

type ArrOfKV []KeyValue

func (D ArrOfKV) Len() int{
	return len(D)
}

func (D ArrOfKV) Swap(i, j int){
	D[i], D[j] = D[j], D[i]
}

func (D ArrOfKV) Less(i, j int) bool {
	return D[i].Key<D[j].Key
}

// doReduce does the job of a reduce worker: it reads the intermediate
// key/value pairs (produced by the map phase) for this task, sorts the
// intermediate key/value pairs by key, calls the user-defined reduce function
// (reduceF) for each key, and writes the output to disk.
func doReduce(
	jobName string, // the name of the whole MapReduce job
	reduceTaskNumber int, // which reduce task this is
	nMap int, // the number of map tasks that were run ("M" in the paper)
	reduceF func(key string, values []string) string,
) {
	// TODO:
	// You will need to write this function.
	// You can find the intermediate file for this reduce task from map task number
	// m using reduceName(jobName, m, reduceTaskNumber).
	// Remember that you've encoded the values in the intermediate files, so you
	// will need to decode them. If you chose to use JSON, you can read out
	// multiple decoded values by creating a decoder, and then repeatedly calling
	// .Decode() on it until Decode() returns an error.
	//
	// You should write the reduced output in as JSON encoded KeyValue
	// objects to a file named mergeName(jobName, reduceTaskNumber). We require
	// you to use JSON here because that is what the merger than combines the
	// output from all the reduce tasks expects. There is nothing "special" about
	// JSON -- it is just the marshalling format we chose to use. It will look
	// something like this:
	//
	// enc := json.NewEncoder(mergeFile)
	// for key in ... {
	// 	enc.Encode(KeyValue{key, reduceF(...)})
	// }
	// file.Close()
	var kvPairs []KeyValue
	var temp KeyValue
	for i := 0 ; i < nMap ; i++{
		ItmFile, _ := os.Open(reduceName(jobName, i, reduceTaskNumber))
		dec := json.NewDecoder(ItmFile)
		err := dec.Decode(&temp)
		kvPairs = append(kvPairs, temp)
		for err == nil {
			err = dec.Decode(&temp)
			kvPairs = append(kvPairs, temp)			
		}
		ItmFile.Close()
	}

	sort.Sort(ArrOfKV(kvPairs))

	var arr_str []string
	var arr_key []string
	for _, kv := range kvPairs{
		arr_key = append(arr_key, kv.Key)
		arr_str = append(arr_str, kv.Value)
	}

	mergeFile, _ := os.Create(mergeName(jobName, reduceTaskNumber))
	enc := json.NewEncoder(mergeFile)
	j := 0
	for i := 1 ;i < len(arr_key) ; i++ {
		if arr_key[i] != arr_key[i-1]{
			enc.Encode(KeyValue{arr_key[j], reduceF(arr_key[j],arr_str[j:i])})
			j = i
		}
	}
	enc.Encode(KeyValue{arr_key[j], reduceF(arr_key[j],arr_str[j:])})
	mergeFile.Close()
}
