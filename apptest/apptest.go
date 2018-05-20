// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
// Testing harness for holochain applications

package apptest

import (
	"encoding/json"
	"errors"
	"fmt"
	. "github.com/holochain/holochain-proto"
	"github.com/holochain/holochain-proto/ui"
	"github.com/shirou/gopsutil/process"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func toString(input interface{}) string {
	// @TODO this should probably act according the function schema
	// not just the return value
	var output string
	switch t := input.(type) {
	case []byte:
		output = string(t)
	case string:
		output = t
	default:
		output = fmt.Sprintf("%v", t)
	}
	return output
}

type replacements struct {
	h          *Holochain
	r1, r2, r3 string
	repetition string
	history    *history
	fixtures   TestFixtures
	pairs      map[string]string
}

func replaceWithResultsFromHistory(output string, re *regexp.Regexp, r *replacements) string {
	matches := re.FindAllStringSubmatch(output, -1)
	if len(matches) > 0 {
		for _, m := range matches {
			resultIdx, err := strconv.Atoi(m[2])
			if err != nil {
				panic(err)
			}
			var nthResult string
			if len(r.history.results) > resultIdx {
				nthResult = fmt.Sprintf("%v", r.history.results[resultIdx])
			} else {
				nthResult = "<bad-result-index>"
			}
			output = strings.Replace(output, m[1], nthResult, -1)
		}
	}
	return output
}

// testStringReplacements inserts special values into testing input and output values for matching
func testStringReplacements(input string, r *replacements) string {
	output := input
	h := r.h
	// look for %hn% in the string and do the replacements for recent hashes
	re := regexp.MustCompile(`(\%h([0-9]*)\%)`)
	var matches [][]string
	matches = re.FindAllStringSubmatch(output, -1)
	if len(matches) > 0 {
		for _, m := range matches {
			var hashIdx int
			if m[2] != "" {
				var err error
				hashIdx, err = strconv.Atoi(m[2])
				if err != nil {
					panic(err)
				}
			}
			entry := h.Chain().Nth(hashIdx)
			var hash string
			if entry != nil {
				hash = entry.EntryLink.String()
			} else {
				hash = fmt.Sprintf("<%d: entry doesn't exist>", hashIdx)
			}
			output = strings.Replace(output, m[1], hash, -1)
		}
	}

	re = regexp.MustCompile(`(\%result([0-9]+)\%)`)
	output = replaceWithResultsFromHistory(output, re, r)

	// this allows us to replace json results
	re = regexp.MustCompile(`(\{"\%result\%":([0-9]+)\})`)
	output = replaceWithResultsFromHistory(output, re, r)

	for i, f := range r.fixtures.Agents {
		output = strings.Replace(output, "%agent"+fmt.Sprintf("%d", i)+"%", f.Hash, -1)
		output = strings.Replace(output, "%agent"+fmt.Sprintf("%d", i)+"_str%", f.Identity, -1)
	}
	output = strings.Replace(output, "%reps%", r.repetition, -1)
	output = strings.Replace(output, "%r1%", r.r1, -1)
	output = strings.Replace(output, "%r2%", r.r2, -1)
	output = strings.Replace(output, "%r3%", r.r3, -1)
	output = strings.Replace(output, "%dna%", h.DNAHash().String(), -1)
	output = strings.Replace(output, "%agent%", h.AgentHash().String(), -1)
	output = strings.Replace(output, "%agenttop%", h.AgentTopHash().String(), -1)
	output = strings.Replace(output, "%agentstr%", string(h.Agent().Identity()), -1)
	output = strings.Replace(output, "%key%", h.NodeIDStr(), -1)

	for key, val := range r.pairs {
		output = strings.Replace(output, key, val, -1)
	}

	// look for %mx.y% in the string and do the replacements from last matches
	re = regexp.MustCompile(`(\%m([0-9])\.([0-9])\%)`)
	matches = re.FindAllStringSubmatch(output, -1)
	if len(matches) > 0 {
		for _, m := range matches {
			matchIdx, err := strconv.Atoi(m[2])
			if err != nil {
				panic(err)
			}
			subMatch, err := strconv.Atoi(m[3])
			if err != nil {
				panic(err)
			}
			if matchIdx < 1 || matchIdx > 3 {
				panic("please pick a match between 1 & 3")
			}
			if subMatch < len(r.history.lastMatches[matchIdx-1]) {
				output = strings.Replace(output, m[1], r.history.lastMatches[matchIdx-1][subMatch], -1)
			}
		}
	}

	return output
}

// TestScenario runs the tests of a single role in a scenario
func TestScenario(h *Holochain, scenario string, role string, replacementPairs map[string]string, benchmarks bool, bridgeApps []BridgeApp) (err error, testErrs []error) {
	var config *TestConfig
	dir := filepath.Join(h.TestPath(), scenario)

	config, err = LoadTestConfig(dir)
	if err != nil {
		return
	}
	var testSet TestSet
	testSet, err = LoadTestFile(dir, role+".json")
	if err != nil {
		return
	}
	if testSet.Identity != "" {
		SetIdentity(h, AgentIdentity(testSet.Identity))
	}

	err = initChainForTest(h, true)
	if err != nil {
		err = fmt.Errorf("Error initializing chain for scenario role %s: %v", role, err)
		return
	}

	err = buildBridges(h, "", bridgeApps)
	if err != nil {
		err = fmt.Errorf("couldn't build bridges for scenario. err: %v", err)
		return
	}

	if config.GossipInterval > 0 {
		h.Config.SetGossipInterval(time.Duration(config.GossipInterval) * time.Millisecond)
	} else {
		h.Config.SetGossipInterval(0)
	}
	h.StartBackgroundTasks()

	var b *benchmark
	if benchmarks {
		b = StartBench(h)
	}
	testErrs = DoTests(h, role, testSet, time.Duration(config.Duration)*time.Second, replacementPairs)
	if benchmarks {
		b.End()
		logBenchmark(&h.Config.Loggers.TestInfo, fmt.Sprintf("%s-%s", scenario, role), b)
	}

	return
}

func waitTill(start time.Time, till time.Duration) {
	elapsed := time.Now().Sub(start)
	toWait := till - elapsed
	if toWait > 0 {
		time.Sleep(toWait)
	}
}

type history struct {
	results     []interface{}
	lastResults [3]interface{}
	lastMatches [3][]string
}

type benchmark struct {
	ElapsedTime time.Duration
	CPU         float64
	Memory      float64
	ChainGrowth int64
	DHTGrowth   int64
	BytesSent   int64
	GossipSent  int64
	start       time.Time
	h           *Holochain
	process     *process.Process
}

// StartBench returns a benchmark data struct set up so that a subsequent call to
// EndBench can then complete and set the values
func StartBench(h *Holochain) *benchmark {
	if BytesSentChan != nil {
		panic("Benchmarking already started, can't restart!")
	}
	b := benchmark{
		h:           h,
		start:       time.Now(),
		ChainGrowth: FileSize(h.DBPath(), StoreFileName),
		DHTGrowth:   FileSize(h.DBPath(), DHTStoreFileName),
	}

	process, err := process.NewProcess(int32(os.Getpid()))
	if err != nil {
		panic("unable to get process at start benchmark: " + err.Error())
	}
	b.process = process
	times, err := process.Times()
	if err != nil {
		panic("failed to get times: " + err.Error())
	}
	b.CPU = times.Total()
	//	b.Memory = sysInfo.Memory
	BytesSentChan = make(chan BytesSent, 100)
	go func() {
		for BytesSentChan != nil {
			b.updateBytesSent(BytesSentChan)
		}
	}()
	return &b
}

func (b *benchmark) updateBytesSent(bsc chan BytesSent) {
	bs := <-bsc
	switch bs.MsgType {
	case VALIDATE_MIGRATE_REQUEST:
		fallthrough
	case GOSSIP_REQUEST:
		fallthrough
	case VALIDATE_PUT_REQUEST:
		fallthrough
	case VALIDATE_LINK_REQUEST:
		fallthrough
	case VALIDATE_DEL_REQUEST:
		fallthrough
	case VALIDATE_MOD_REQUEST:
		b.GossipSent += bs.Bytes
	default:
		b.BytesSent += bs.Bytes
	}
}

// End completes setting the values of the passed in benchmark struct which must
// be initialized by a call to StartBench
func (b *benchmark) End() {
	bsc := BytesSentChan
	BytesSentChan = nil
	times, err := b.process.Times()
	if err != nil {
		panic("unable to get cpu/memory data on end benchmark: " + err.Error())
	}

	b.CPU = times.Total() - b.CPU

	b.ElapsedTime = time.Now().Sub(b.start)
	b.ChainGrowth = FileSize(b.h.DBPath(), StoreFileName) - b.ChainGrowth
	b.DHTGrowth = FileSize(b.h.DBPath(), DHTStoreFileName) - b.DHTGrowth
	for len(bsc) > 0 {
		b.updateBytesSent(bsc)
	}

	return
}

// DoTests runs through all the tests in a TestSet and returns any errors encountered
// TODO: this code can cause crazy race conditions because lastResults and lastMatches get
// passed into go routines that run asynchronously.  We should probably reimplement this with
// channels or some other thread-safe queues.
func DoTests(h *Holochain, name string, testSet TestSet, minTime time.Duration, replacementPairs map[string]string) (errs []error) {
	var history history
	tests := testSet.Tests
	done := make(chan bool, len(tests))
	startTime := time.Now()

	benchmarks := make(map[string]*benchmark)

	var count int
	// queue up any timed tests into go routines
	for i, t := range tests {
		if t.Time == 0 {
			continue
		}
		count++
		go func(index int, test TestData) {
			waitTill(startTime, test.Time*time.Millisecond)
			err := DoTest(h, name, index, testSet.Fixtures, test, startTime, &history, replacementPairs, benchmarks, testSet.Benchmark)
			if err != nil {
				errs = append(errs, err)
			}
			done <- true
		}(i, t)
	}

	// run all the non timed tests.
	for i, t := range tests {
		if t.Time > 0 {
			continue
		}

		err := DoTest(h, name, i, testSet.Fixtures, t, startTime, &history, replacementPairs, benchmarks, testSet.Benchmark)
		if err != nil {
			errs = append(errs, err)
		}

	}

	// wait for all the timed tests to complete
	for i := 0; i < count; i++ {
		<-done
	}

	// check to see if we still need to stay alive more
	if minTime > 0 {
		waitTill(startTime, minTime)
	}

	if len(benchmarks) > 0 {
		logBenchmarkTotals(&h.Config.Loggers.TestInfo, benchmarks)
	}
	return
}

func logBenchmarkTotals(log *Logger, benchmarks map[string]*benchmark) {
	var total benchmark
	for _, b := range benchmarks {
		total.ElapsedTime += b.ElapsedTime
		total.ChainGrowth += b.ChainGrowth
		total.DHTGrowth += b.DHTGrowth
		total.CPU += b.CPU
		total.BytesSent += b.BytesSent
		total.GossipSent += b.GossipSent
	}
	log.Logf(`Benchmark Summary:
   Total elapsed time: %v
   Total chain growth: %.2fK bytes
   Total DHT growth: %.2fK bytes
   Total Bytes sent: %.2fK
   Total Gossip sent: %.2fK
   Total CPU use: %.2fms
`, total.ElapsedTime, toK(total.ChainGrowth), toK(total.DHTGrowth), toK(total.BytesSent), toK(total.GossipSent), total.CPU*1000)

}

func logBenchmark(log *Logger, name string, b *benchmark) {
	log.Logf(`Benchmark for %s:
   Elapsed time: %v
   Chain growth: %.2fK bytes
   DHT growth: %.2fK bytes
   BytesSent: %.2fK
   GossipSent: %.2fK
   CPU: %.2fms
`, name, b.ElapsedTime, toK(b.ChainGrowth), toK(b.DHTGrowth), toK(b.BytesSent), toK(b.GossipSent), b.CPU*1000)
}

func toK(bytes int64) float64 {
	return float64(bytes) / 1024.0
}

func toStringByType(data interface{}) (output string, err error) {
	switch t := data.(type) {
	case string:
		output = t
	case map[string]interface{}:
		inputByteArray, err := json.Marshal(data)
		if err == nil {
			output = string(inputByteArray)
		}
	default:
		output = fmt.Sprintf("%v", data)
	}
	return
}

// DoTest runs a singe test.
func DoTest(h *Holochain, name string, i int, fixtures TestFixtures, t TestData, startTime time.Time, history *history, replacementPairs map[string]string, benchmarks map[string]*benchmark, benchmarkAllTests bool) (err error) {
	info := &h.Config.Loggers.TestInfo
	passed := &h.Config.Loggers.TestPassed
	failed := &h.Config.Loggers.TestFailed

	// set up the input and output values by converting them according the
	// the function's defined calling type.
	var byType bool
	var input, output string
	if t.Raw {
		byType = true
	} else {
		var zome *Zome
		zome, err = h.GetZome(t.Zome)
		if err != nil {
			err = fmt.Errorf("error getting zome %s: %v", t.Zome, err)
			return
		}
		var fndef *FunctionDef
		fndef, err = zome.GetFunctionDef(t.FnName)
		if err != nil {
			err = fmt.Errorf("error getting function definition for %s: %v", t.FnName, err)
			return
		}
		if fndef.CallingType == JSON_CALLING {
			var b []byte
			b, err = json.Marshal(t.Input)
			if err != nil {
				err = fmt.Errorf("error converting Input '%v' to JSON: %v", t.Input, err)
				return
			}
			input = string(b)
			b, err = json.Marshal(t.Output)
			if err != nil {
				err = fmt.Errorf("error converting Input '%s' to JSON: %v", t.Input, err)
				return
			}
			output = string(b)

		} else {
			byType = true
		}

	}
	if byType {
		input, err = toStringByType(t.Input)
		if err != nil {
			err = fmt.Errorf("error converting Input '%s' to string:%v", t.Input, err)
			return
		}
		output, err = toStringByType(t.Output)
		if err != nil {
			err = fmt.Errorf("error converting Output '%v' to string:%v", t.Output, err)
			return
		}
	}

	h.Debugf("------------------------------")
	description := t.Convey
	if description == "" {
		description = fmt.Sprintf("%v", t)
	}
	elapsed := time.Now().Sub(startTime) / time.Millisecond
	var repetitions int
	if t.Repeat == 0 {
		repetitions = 1
	} else {
		repetitions = t.Repeat
	}

	replacements := replacements{h: h, history: history, fixtures: fixtures, pairs: replacementPairs}
	origInput := input
	for r := 0; r < repetitions; r++ {
		input = origInput // gotta do this so %reps% substitution will work
		var rStr, testID string
		if t.Repeat > 0 {
			rStr = fmt.Sprintf(".%d", r)
			testID = fmt.Sprintf("%s:%d.%d", name, i, r)
		} else {
			testID = fmt.Sprintf("%s:%d", name, i)
		}
		info.Logf("Test '%s.%d%s' t+%dms: %s", name, i, rStr, elapsed, description)
		if t.Wait > 0 {
			info.Logf("   waiting %dms...", t.Wait)
			time.Sleep(time.Millisecond * t.Wait)
			elapsed := time.Now().Sub(startTime) / time.Millisecond
			info.Logf("   test '%s.%d%s' continuing at t+%dms", name, i, rStr, elapsed)
		}

		h.Debugf("Input before replacement: %s", input)
		replacements.repetition = fmt.Sprintf("%d", r)
		replacements.r1 = strings.Trim(fmt.Sprintf("%v", history.lastResults[0]), "\"")
		replacements.r2 = strings.Trim(fmt.Sprintf("%v", history.lastResults[1]), "\"")
		replacements.r3 = strings.Trim(fmt.Sprintf("%v", history.lastResults[2]), "\"")
		input = testStringReplacements(input, &replacements)
		h.Debugf("Input after replacement: %s", input)
		//====================

		var actualResult interface{}
		var actualError error
		var b *benchmark
		if benchmarkAllTests || t.Benchmark {
			b = StartBench(h)
		}
		if t.Raw {
			n, _, err := h.MakeRibosome(t.Zome)
			if err != nil {
				actualError = err
			} else {
				actualResult, actualError = n.Run(input)
			}
		} else {
			actualResult, actualError = h.Call(t.Zome, t.FnName, input, t.Exposure)
		}
		if benchmarkAllTests || t.Benchmark {
			b.End()
			benchmarks[testID] = b
			logBenchmark(info, testID, b)
		}

		var expectedResult = output
		var expectedErrorObject = t.Err
		var expectedErrorMessage = t.ErrMsg
		var expectedError string

		if expectedErrorObject != nil {
			expectedError, err = toStringByType(expectedErrorObject)
			if err != nil {
				panic("unable to covert expected error to string")
			}
		} else if expectedErrorMessage != "" {
			expectedError = expectedErrorMessage
		}
		var expectedResultRegexp = t.Regexp
		//====================
		history.lastResults[2] = history.lastResults[1]
		history.lastResults[1] = history.lastResults[0]
		history.lastResults[0] = actualResult
		history.results = append(history.results, actualResult)
		if expectedError != "" {
			expectedError = testStringReplacements(expectedError, &replacements)
			comparisonString := fmt.Sprintf("\nTest: %s\n\tExpected error:\t%v\n\tGot error:\t\t%v", testID, expectedError, actualError)
			var actualErrorStr string
			if actualError != nil {
				actualErrorStr = actualError.Error()
				if expectedErrorMessage != "" {
					re := regexp.MustCompile(`"errorMessage":"([^"]+)"`)
					x := re.FindStringSubmatch(actualErrorStr)
					if len(x) == 0 {
						panic("expected to find an error message in " + actualErrorStr)
					}
					actualErrorStr = x[1]
				}
			}
			if actualError == nil || (actualErrorStr != expectedError) {
				failed.Logf("\n=====================\n%s\n\tfailed! m(\n=====================", comparisonString)
				err = errors.New(expectedError)
			} else {
				// all fine
				h.Debugf("%s\n\tpassed :D", comparisonString)
				err = nil
			}
		} else {
			if actualError != nil {
				expectedResult = testStringReplacements(expectedResult, &replacements)
				errorString := fmt.Sprintf("\nTest: %s\n\tExpected:\t%s\n\tGot Error:\t\t%s\n", testID, expectedResult, actualError)
				err = errors.New(errorString)
				failed.Logf(fmt.Sprintf("\n=====================\n%s\n\tfailed! m(\n=====================", errorString))
			} else {
				var resultString = toString(actualResult)
				var match bool
				var comparisonString string
				if expectedResultRegexp != "" {
					h.Debugf("Test %s matching against regexp...", testID)
					expectedResultRegexp = testStringReplacements(expectedResultRegexp, &replacements)
					comparisonString = fmt.Sprintf("\nTest: %s\n\tExpected regexp:\t%v\n\tGot:\t\t%v", testID, expectedResultRegexp, resultString)
					re, matchError := regexp.Compile(expectedResultRegexp)
					if matchError != nil {
						Infof(err.Error())
					} else {
						matches := re.FindStringSubmatch(resultString)
						history.lastMatches[2] = history.lastMatches[1]
						history.lastMatches[1] = history.lastMatches[0]
						history.lastMatches[0] = matches
						if len(matches) > 0 {
							match = true
						}
					}

				} else {
					h.Debugf("Test %s matching against string...", testID)
					expectedResult = testStringReplacements(expectedResult, &replacements)
					comparisonString = fmt.Sprintf("\nTest: %s\n\tExpected:\t%v\n\tGot:\t\t%v", testID, expectedResult, resultString)
					match = (resultString == expectedResult)
				}

				if match {
					h.Debugf("%s\n\tpassed! :D", comparisonString)
					passed.Log("passed! ✔")
				} else {
					err = errors.New(comparisonString)
					failed.Logf(fmt.Sprintf("\n=====================\n%s\n\tfailed! m(\n=====================", comparisonString))
				}
			}
		}
	}
	return
}

// Test loops through each of the test files in path calling the functions specified
// This function is useful only in the context of developing a holochain and will return
// an error if the chain has already been started (i.e. has genesis entries)
func Test(h *Holochain, bridgeApps []BridgeAppForTests, forceBenchmark bool) []error {
	return test(h, "", bridgeApps, false)
}

// TestOne tests a single test file
// This function is useful only in the context of developing a holochain and will return
// an error if the chain has already been started (i.e. has genesis entries)
func TestOne(h *Holochain, one string, bridgeApps []BridgeAppForTests, forceBenchmark bool) []error {
	return test(h, one, bridgeApps, forceBenchmark)
}

func initChainForTest(h *Holochain, reset bool) (err error) {
	if reset {
		err = h.Reset()
		if err != nil {
			return
		}
	}
	_, err = h.GenChain()
	if err != nil {
		return
	}
	err = h.Activate()
	if err != nil {
		return
	}
	return
}

func StartBridgeApp(h *Holochain, port string) (bridgeAppServer *ui.WebServer, err error) {
	// setup bridge app
	err = initChainForTest(h, true)
	if err != nil {
		err = fmt.Errorf("couldn't initialize bridge for %s for test. err:%v", h.DNAHash().String(), err.Error())
		return
	}

	bridgeAppServer = ui.NewWebServer(h, port)
	bridgeAppServer.Start()

	return
}

func StopBridgeApps(bridgeAppServers []*ui.WebServer) {
	// stop all the bridge web servers
	for _, server := range bridgeAppServers {
		server.Stop()
	}
	// then wait for them to complete
	for _, server := range bridgeAppServers {
		server.Wait()
	}
}

func buildBridges(h *Holochain, port string, bridgeApps []BridgeApp) (err error) {
	// build a bridge to all the bridge apps
	for _, app := range bridgeApps {
		if app.Side == BridgeCaller {
			err = h.BuildBridgeToCaller(&app, port)
		} else {
			err = h.BuildBridgeToCallee(&app)
		}
		if err != nil {
			return
		}
	}
	return
}

type BridgeAppForTests struct {
	H         *Holochain
	BridgeApp BridgeApp
}

// BuildBridges starts up the bridged apps and builds bridges to/from them for Holochain h
func BuildBridges(h *Holochain, port string, bridgeApps []BridgeAppForTests) (bridgeAppServers []*ui.WebServer, err error) {
	var bApps []BridgeApp
	for _, app := range bridgeApps {
		bApps = append(bApps, app.BridgeApp)
		var bridgeAppServer *ui.WebServer
		bridgeAppServer, err = StartBridgeApp(app.H, app.BridgeApp.Port)
		if err != nil {
			return
		}
		bridgeAppServers = append(bridgeAppServers, bridgeAppServer)
	}

	err = buildBridges(h, port, bApps)
	if err != nil {
		panic(err)
	}
	return
}

func test(h *Holochain, one string, bridgeApps []BridgeAppForTests, forceBenchmark bool) []error {

	var err error
	var errs []error
	if h.Started() {
		err = errors.New("chain already started")
		return []error{err}
	}

	path := h.TestPath()

	// load up the test files into the tests array
	var tests, errorLoad = LoadTestFiles(path)
	if errorLoad != nil {
		return []error{errorLoad}
	}
	info := h.Config.Loggers.TestInfo
	passed := h.Config.Loggers.TestPassed
	failed := h.Config.Loggers.TestFailed

	defaultIdentity := h.Agent().Identity()
	for name, ts := range tests {
		if forceBenchmark {
			ts.Benchmark = true
		}
		if one != "" && name != one {
			continue
		}
		info.Log("========================================")
		info.Logf("Test: '%s' starting...", name)
		info.Log("========================================")
		// setup the genesis entries
		identity := defaultIdentity
		if ts.Identity != "" {
			identity = AgentIdentity(ts.Identity)
		}
		SetIdentity(h, identity)
		h.Debugf("Setting identity to:%v", h.Agent().Identity())
		err = initChainForTest(h, true)
		var ers []error
		if err != nil {
			err = fmt.Errorf("couldn't initialize chain for test. err: %v", err)
			failed.Log(err.Error())
			ers = []error{err}
		} else {

			var bridgeAppServers []*ui.WebServer
			bridgeAppServers, err = BuildBridges(h, "", bridgeApps)
			if err != nil {
				err = fmt.Errorf("couldn't build bridges for test. err: %v", err)
				failed.Log(err.Error())
				ers = []error{err}
			} else {
				ers = DoTests(h, name, ts, 0, nil)

				StopBridgeApps(bridgeAppServers)
			}
		}
		errs = append(errs, ers...)
		// restore the state for the next test file
		e := h.Reset()
		if e != nil {
			panic(e)
		}
	}
	if len(errs) == 0 {
		passed.Log(fmt.Sprintf("\n==================================================================\n\t\t+++++ All tests passed :D +++++\n=================================================================="))
	} else {
		failed.Logf(fmt.Sprintf("\n==================================================================\n\t\t+++++ %d test(s) failed :( +++++\n==================================================================", len(errs)))
	}
	return errs
}
