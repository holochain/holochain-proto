// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
// Testing harness for holochain applications

package apptest

import (
	"encoding/json"
	"errors"
	"fmt"
	. "github.com/metacurrency/holochain"
	"github.com/metacurrency/holochain/ui"
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
	serverID   string
	repetition string
	history    *history
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
			hash := h.Chain().Nth(hashIdx).EntryLink
			output = strings.Replace(output, m[1], hash.String(), -1)
		}
	}

	// this is a hack.  The clone id is put into the identity by hcdev we can get
	// out with this regex without having to create more code to pass it around.
	re = regexp.MustCompile(`.*.([0-9]+)@.*`)
	x := re.FindStringSubmatch(string(h.Agent().Identity()))
	var clone string
	if len(x) > 0 {
		clone = x[1]
	}
	output = strings.Replace(output, "%clone%", clone, -1)

	re = regexp.MustCompile(`(\%result([0-9]+)\%)`)
	output = replaceWithResultsFromHistory(output, re, r)

	// this allows us to replace json results
	re = regexp.MustCompile(`(\{"\%result\%":([0-9]+)\})`)
	output = replaceWithResultsFromHistory(output, re, r)

	output = strings.Replace(output, "%server%", r.serverID, -1)
	output = strings.Replace(output, "%reps%", r.repetition, -1)
	output = strings.Replace(output, "%r1%", r.r1, -1)
	output = strings.Replace(output, "%r2%", r.r2, -1)
	output = strings.Replace(output, "%r3%", r.r3, -1)
	output = strings.Replace(output, "%dna%", h.DNAHash().String(), -1)
	output = strings.Replace(output, "%agent%", h.AgentHash().String(), -1)
	output = strings.Replace(output, "%agenttop%", h.AgentTopHash().String(), -1)
	output = strings.Replace(output, "%agentstr%", string(h.Agent().Identity()), -1)
	output = strings.Replace(output, "%key%", h.NodeIDStr(), -1)

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
func TestScenario(h *Holochain, dir string, role string, serverID string) (err error, testErrs []error) {
	var config *TestConfig
	config, err = LoadTestConfig(dir)
	if err != nil {
		return
	}
	var tests []TestData
	tests, err = LoadTestFile(dir, role+".json")
	if err != nil {
		return
	}

	// setup the genesis entries
	err = h.Reset()
	if err != nil {
		panic("reset err")
	}

	_, err = h.GenChain()
	if err != nil {
		panic("gen err " + err.Error())
	}

	err = h.Activate()
	if err != nil {
		return
	}

	if config.GossipInterval > 0 {
		h.Config.SetGossipInterval(time.Duration(config.GossipInterval) * time.Millisecond)
	} else {
		h.Config.SetGossipInterval(0)
	}
	h.StartBackgroundTasks()

	testErrs = DoTests(h, role, tests, time.Duration(config.Duration)*time.Second, serverID)

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

// DoTests runs through all the tests in a TestData array and returns any errors encountered
// TODO: this code can cause crazy race conditions because lastResults and lastMatches get
// passed into go routines that run asynchronously.  We should probably reimplement this with
// channels or some other thread-safe queues.
func DoTests(h *Holochain, name string, tests []TestData, minTime time.Duration, serverID string) (errs []error) {
	var history history
	done := make(chan bool, len(tests))
	startTime := time.Now()

	var count int
	// queue up any timed tests into go routines
	for i, t := range tests {
		if t.Time == 0 {
			continue
		}
		count++
		go func(index int, test TestData) {
			waitTill(startTime, test.Time*time.Millisecond)
			err := DoTest(h, name, index, test, startTime, &history, serverID)
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

		err := DoTest(h, name, i, t, startTime, &history, serverID)
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

	return
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
func DoTest(h *Holochain, name string, i int, t TestData, startTime time.Time, history *history, serverID string) (err error) {
	info := h.Config.Loggers.TestInfo
	passed := h.Config.Loggers.TestPassed
	failed := h.Config.Loggers.TestFailed

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

	replacements := replacements{h: h, serverID: serverID, history: history}
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
		var expectedResult, expectedError = output, t.Err
		var expectedResultRegexp = t.Regexp
		//====================
		history.lastResults[2] = history.lastResults[1]
		history.lastResults[1] = history.lastResults[0]
		history.lastResults[0] = actualResult
		history.results = append(history.results, actualResult)
		if expectedError != "" {
			expectedError = testStringReplacements(expectedError, &replacements)
			comparisonString := fmt.Sprintf("\nTest: %s\n\tExpected error:\t%v\n\tGot error:\t\t%v", testID, expectedError, actualError)
			if actualError == nil || (actualError.Error() != expectedError) {
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
func Test(h *Holochain, bridgeApps []BridgeApp) []error {
	return test(h, "", bridgeApps)
}

// TestOne tests a single test file
// This function is useful only in the context of developing a holochain and will return
// an error if the chain has already been started (i.e. has genesis entries)
func TestOne(h *Holochain, one string, bridgeApps []BridgeApp) []error {
	return test(h, one, bridgeApps)
}

func InitChain(h *Holochain, reset bool) (err error) {
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

func BuildBridges(h *Holochain, port string, bridgeApps []BridgeApp) (bridgeAppServers []*ui.WebServer, err error) {
	bridgeAppServers = make([]*ui.WebServer, len(bridgeApps))

	// setup any bridges
	for i, app := range bridgeApps {
		InitChain(app.H, true)
		if err != nil {
			err = fmt.Errorf("couldn't initialize bridge for %s for test. err:%v", app.H.DNAHash().String(), err.Error())
			return
		}

		bridgeAppServers[i] = ui.NewWebServer(app.H, app.Port)
		bridgeAppServers[i].Start()

		err = h.BuildBridge(&app, port)
		if err != nil {
			panic(err)
		}
	}
	return
}

func test(h *Holochain, one string, bridgeApps []BridgeApp) []error {

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

	for name, ts := range tests {
		if one != "" && name != one {
			continue
		}
		info.Log("========================================")
		info.Logf("Test: '%s' starting...", name)
		info.Log("========================================")
		// setup the genesis entries
		err = InitChain(h, true)
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
				//	go h.dht.HandleChangeReqs()
				ers = DoTests(h, name, ts, 0, "")

				// stop all the bridge web servers
				for _, server := range bridgeAppServers {
					server.Stop()
				}
				// then wait for them to complete
				for _, server := range bridgeAppServers {
					server.Wait()
				}
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
