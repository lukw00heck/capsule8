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
	"log"
	"runtime"

	api "github.com/capsule8/capsule8/api/v0"

	"github.com/capsule8/capsule8/pkg/expression"
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
	sensor   *Sensor
	counters []eventCounters
}

const (
	// perf_event_attr config value for LL cache loads
	perfConfigLLCLoads = perf.PERF_COUNT_HW_CACHE_LL |
		(perf.PERF_COUNT_HW_CACHE_OP_READ << 8) |
		(perf.PERF_COUNT_HW_CACHE_RESULT_ACCESS << 16)

	// perf_event_attr config value for LL cache misses
	perfConfigLLCLoadMisses = perf.PERF_COUNT_HW_CACHE_LL |
		(perf.PERF_COUNT_HW_CACHE_OP_READ << 8) |
		(perf.PERF_COUNT_HW_CACHE_RESULT_MISS << 16)
)


func (t *perfhwcacheFilter) decodeConfigLLCLoads(
	sample *perf.SampleRecord,
	counters map[uint64]uint64,
	totalTimeElapsed uint64,
	totalTimeRunning uint64,
) (interface{}, error) {

	cpu := sample.CPU
	prevCounters := t.counters[cpu]
	t.counters[cpu] = eventCounters{
		LLCLoads:      counters[perfConfigLLCLoads],
		LLCLoadMisses: counters[perfConfigLLCLoadMisses],
	}

	counterDeltas := eventCounters{
		LLCLoads:      t.counters[cpu].LLCLoads - prevCounters.LLCLoads,
		LLCLoadMisses: t.counters[cpu].LLCLoadMisses - prevCounters.LLCLoadMisses,
	}

	e := t.sensor.NewEvent()
	e.Event = &api.TelemetryEvent_Perfhwcache{
		Perfhwcache: &api.PerfHWCacheEvent{
			Llcloads:      counterDeltas.LLCLoads,
			Llcloadmisses: counterDeltas.LLCLoadMisses,
		},
	}

	log.Printf("event %+%v", e)

	return e, nil
}

func registerPerfHWCacheEvents(
	sensor *Sensor,
	groupID int32,
	eventMap subscriptionMap,
	events []*api.PerfHWCacheEventFilter,
) {

	for _, e := range events {
		tracker := perfhwcacheFilter{
			sensor:   sensor,
			counters: make([]eventCounters, runtime.NumCPU()),
		}

		// Create our event group to read LL cache accesses and misses
		// We ask the kernel to sample every llcLoadSampleSize LLC
		// loads. During each sample, the LLC load misses are also
		// recorded, as well as CPU number, PID/TID, and sample time.
		attr := perf.EventAttr{
			SamplePeriod: e.Numllcloads,
			SampleType: perf.PERF_SAMPLE_TID | perf.PERF_SAMPLE_CPU,
		}
		eventID, err := sensor.Monitor.RegisterHardwareCacheEventGroup(
			[]uint64{
				perfConfigLLCLoads,
				perfConfigLLCLoadMisses,
			},
			tracker.decodeConfigLLCLoads,
			perf.WithEventAttr(&attr))
		if err != nil {
			glog.Fatalf("Could not register hardware cache event: %s", err)
		} 

		glog.Info("Monitoring for cache side channels")
		tracker.sensor.Monitor.EnableGroup(eventID)

		done := make(chan struct{})
		nperfhwcache := 0

		if e.Numllcloads == 0 || e.Numllcloads < 10000 {
			glog.V(1).Info("Sample period not optimal (%d)", e.Numllcloads)
			continue
		}
		nperfhwcache++

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

}
