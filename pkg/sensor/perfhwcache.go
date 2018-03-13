// Copyright 2017 Capsule8, Inc.
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

package sensor

import (
//	"flag"
//	"os"
//	"os/signal"
	"runtime"
//	"log"
	api "github.com/capsule8/capsule8/api/v0"

	"github.com/capsule8/capsule8/pkg/expression"
	"github.com/capsule8/capsule8/pkg/sys"
	"github.com/capsule8/capsule8/pkg/sys/perf"

	"github.com/golang/glog"
)

var perfhwcacheEventTypes = expression.FieldTypeMap{
	"llcloads":      int32(api.ValueType_UINT64),
	"llcloadmisses": int32(api.ValueType_UINT64),
}

type eventCounters struct {
        LLCLoads      uint64
        LLCLoadMisses uint64
}

type perfhwcacheFilter struct {
        sensor *Sensor
        counters []eventCounters
}


const (
        // How many cache loads to sample on. After each sample period
        // of this many cache loads, the cache miss rate is calculated
        // and examined. This value tunes the trade-off between CPU
        // load and detection accuracy.
        llcLoadSampleSize = 10000

        // perf_event_attr config value for LL cache loads
        perfConfigLLCLoads = perf.PERF_COUNT_HW_CACHE_LL |
                (perf.PERF_COUNT_HW_CACHE_OP_READ << 8) |
                (perf.PERF_COUNT_HW_CACHE_RESULT_ACCESS << 16)

        // perf_event_attr config value for LL cache misses
        perfConfigLLCLoadMisses = perf.PERF_COUNT_HW_CACHE_LL |
                (perf.PERF_COUNT_HW_CACHE_OP_READ << 8) |
                (perf.PERF_COUNT_HW_CACHE_RESULT_MISS << 16)
)


/*

func (c *perfhwcacheFilter) decodePerfHWCacheEvent(
	sample *perf.SampleRecord,
	data perf.TraceEventSampleData,
) (interface{}, error) {
	e := c.sensor.NewEvent()
	e.Event = &api.TelemetryEvent_Perfhwcache{
		Perfhwcache: &api.PerfHWCacheEvent{
			Index:      data["index"].(uint64),
			Characters: data["characters"].(string),
		},
	}

	return e, nil
}

func generateCharactersPerf(start, length uint64) string {
	bytes := make([]byte, length)
	for i := uint64(0); i < length; i++ {
//		bytes[i] = ' ' + byte((start+i)%95)
		bytes[i] = '1'
	}
	return string(bytes)
}

*/



//decode function




/*

func registerPerfHWCacheEvents(
	sensor *Sensor,
	groupID int32,
	eventMap subscriptionMap,
	events []*api.PerfHWCacheEventFilter,
) {
	f := perfhwcacheFilter{sensor: sensor}
	eventID, err := sensor.Monitor.RegisterExternalEvent("perfhwcache",
		f.decodePerfHWCacheEvent, perfhwcacheEventTypes)
	if err != nil {
		glog.V(1).Infof("Could not register perfhwcache event: %v", err)
		return
	}

	done := make(chan struct{})
	nperfhwcache := 0
	for _, e := range events {
		// XXX there should be a maximum bound here too ...
		if e.Length == 0 || e.Length > 1<<16 {
			glog.V(1).Info("Perfhwcache length out of range (%d)", e.Length)
			continue
		}
		nperfhwcache++

		go func() {
			index := uint64(0)
			length := e.Length
			for {
				select {
				case <-done:
					return
				default:
					monoNow := sys.CurrentMonotonicRaw()
					sampleID := perf.SampleID{
						Time: uint64(monoNow),
					}
					s := generateCharactersPerf(index, length)
					data := perf.TraceEventSampleData{
						"index":      index,
						"characters": s,
					}
					index += length
					sensor.Monitor.EnqueueExternalSample(
						eventID, sampleID, data)
				}
			}
		}()
	}
	if nperfhwcache == 0 {
		sensor.Monitor.UnregisterEvent(eventID)
		return
	}

	s := eventMap.subscribe(eventID)
	s.unregister = func(eventID uint64, s *subscription) {
		sensor.Monitor.UnregisterEvent(eventID)
		close(done)
	}
}

*/

func (t *perfhwcacheFilter) decodeConfigLLCLoads(
        sample *perf.SampleRecord,
        counters map[uint64]uint64,
        totalTimeElapsed uint64,
        totalTimeRunning uint64,
) (interface{}, error) {

        cpu := sample.CPU
//        prevCounters := t.counters[cpu]
        t.counters[cpu] = eventCounters{
                LLCLoads:      counters[perfConfigLLCLoads],
                LLCLoadMisses: counters[perfConfigLLCLoadMisses],
        }
/*
        counterDeltas := eventCounters{
                LLCLoads:      t.counters[cpu].LLCLoads - prevCounters.LLCLoads,
                LLCLoadMisses: t.counters[cpu].LLCLoadMisses - prevCounters.LLCLoadMisses,
        }
*/

	e := t.sensor.NewEvent()
        e.Event = &api.TelemetryEvent_Perfhwcache{
                Perfhwcache: &api.PerfHWCacheEvent{
                        Llcloads:      t.counters[cpu].LLCLoads,
                        Llcloadmisses: t.counters[cpu].LLCLoadMisses,
                },
        }

        return e, nil
}



func registerPerfHWCacheEvents(
        sensor *Sensor,
        groupID int32,
        eventMap subscriptionMap,
        events []*api.PerfHWCacheEventFilter,
) {

//	flag.Set("logtostderr", "true")
//        flag.Parse()

//        glog.Infof("Starting Capsule8 cache side channel detector")
        tracker := perfhwcacheFilter{
                sensor:   sensor,
                counters: make([]eventCounters, runtime.NumCPU()),
        }

        // Create our event group to read LL cache accesses and misses
        //
        // We ask the kernel to sample every llcLoadSampleSize LLC
        // loads. During each sample, the LLC load misses are also
        // recorded, as well as CPU number, PID/TID, and sample time.
        attr := perf.EventAttr{
                SamplePeriod: llcLoadSampleSize,
                SampleType:   perf.PERF_SAMPLE_TID | perf.PERF_SAMPLE_CPU,
        }
        eventID, err := sensor.Monitor.RegisterHardwareCacheEventGroup(
                []uint64{
                        perfConfigLLCLoads,
                        perfConfigLLCLoadMisses,
                },
                tracker.decodeConfigLLCLoads,
                perf.WithEventAttr(&attr))
        if err != nil {
//		log.Println("attr %s", attr)
//		log.Println("tracker %s", tracker)
//		log.Println("tracker_decode %s", tracker.decodeConfigLLCLoads)
                glog.Fatalf("Could not register hardware cache event: %s", err)
        }

//	log.Println("hello")

	done := make(chan struct{})
        nperfhwcache := 0
        for _, e := range events {
                // XXX there should be a maximum bound here too ...
                if e.Length == 0 {
                        glog.V(1).Info("Perfhwcache length out of range (%d)", e.Length)
                        continue
                }
                nperfhwcache++

                go func() {
                        llcloads := uint64(1)
			llcloadmisses := uint64(0)
//                        length := e.Length
                        for {
                                select {
                                case <-done:
                                        return
                                default:
                                        monoNow := sys.CurrentMonotonicRaw()
                                        sampleID := perf.SampleID{
                                                Time: uint64(monoNow),
                                        }

					data := perf.TraceEventSampleData{
                                                "llcloads": llcloads,
                                                "llcloadmisses": llcloadmisses,
                                        }
//                                        index += length
                                        sensor.Monitor.EnqueueExternalSample(
                                                uint64(eventID), sampleID, data)
                                }
                        }
                }()
        }
        if nperfhwcache == 0 {
                sensor.Monitor.UnregisterEvent(uint64(eventID))
                return
        }

        s := eventMap.subscribe(uint64(eventID))
        s.unregister = func(eventID uint64, s *subscription) {
                sensor.Monitor.UnregisterEvent(eventID)
                close(done)
        }
}




