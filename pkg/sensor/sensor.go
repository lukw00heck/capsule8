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
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	api "github.com/capsule8/capsule8/api/v0"

	"github.com/capsule8/capsule8/pkg/config"
	"github.com/capsule8/capsule8/pkg/expression"
	"github.com/capsule8/capsule8/pkg/stream"
	"github.com/capsule8/capsule8/pkg/sys"
	"github.com/capsule8/capsule8/pkg/sys/perf"
	"github.com/golang/glog"

	"golang.org/x/sys/unix"
)

// Number of random bytes to generate for Sensor Id
const sensorIDLengthBytes = 32

// Sensor represents the state of a sensor instance.
type Sensor struct {
	// Unique Id for this sensor. Sensor Ids are ephemeral.
	ID string

	// Sensor-unique event sequence number. Each event sent from this
	// sensor to any subscription has a unique sequence number for the
	// indicated sensor Id.
	sequenceNumber uint64

	// Record the value of CLOCK_MONOTONIC_RAW when the sensor starts.
	// All event monotimes are relative to this value.
	bootMonotimeNanos int64

	// Metrics counters for this sensor
	Metrics MetricsCounters

	// If temporary fs mounts are made at startup, they're stored here.
	perfEventMountPoint string
	traceFSMountPoint   string

	// A sensor-global event monitor that is used for events to aid in
	// caching process information
	Monitor *perf.EventMonitor

	// Per-sensor caches and monitors
	ProcessCache   ProcessInfoCache
	ContainerCache *ContainerCache
	dockerMonitor  *dockerMonitor
	ociMonitor     *ociMonitor

	// Mapping of event ids to data streams (subscriptions)
	eventMap *safeSubscriptionMap

	// Used by syscall events to handle syscall enter events with
	// argument filters
	dummySyscallEventID    uint64
	dummySyscallEventCount int64
}

// NewSensor creates a new Sensor instance.
func NewSensor() (*Sensor, error) {
	randomBytes := make([]byte, sensorIDLengthBytes)
	rand.Read(randomBytes)
	sensorID := hex.EncodeToString(randomBytes[:])

	ts := unix.Timespec{}
	unix.ClockGettime(unix.CLOCK_MONOTONIC_RAW, &ts)
	bootMonotimeNanos := ts.Nsec + (ts.Sec * int64(time.Second))

	s := &Sensor{
		ID:                sensorID,
		bootMonotimeNanos: bootMonotimeNanos,
		eventMap:          newSafeSubscriptionMap(),
	}

	return s, nil
}

// Start starts a sensor instance running.
func (s *Sensor) Start() error {
	var err error

	// We require that our run dir (usually /var/run/capsule8) exists.
	// Ensure that now before proceeding any further.
	err = os.MkdirAll(config.Global.RunDir, 0700)
	if err != nil {
		glog.Warningf("Couldn't mkdir %s: %s",
			config.Global.RunDir, err)
		return err
	}

	// If there is no mounted tracefs, the Sensor really can't do anything.
	// Try mounting our own private mount of it.
	if !config.Sensor.DontMountTracing && len(sys.TracingDir()) == 0 {
		// If we couldn't find one, try mounting our own private one
		glog.V(2).Info("Can't find mounted tracefs, mounting one")
		err = s.mountTraceFS()
		if err != nil {
			glog.V(1).Info(err)
			return err
		}
	}

	// If there is no mounted cgroupfs for the perf_event cgroup, we can't
	// efficiently separate processes in monitored containers from host
	// processes. We can run without it, but it's better performance when
	// available.
	if !config.Sensor.DontMountPerfEvent && len(sys.PerfEventDir()) == 0 {
		glog.V(2).Info("Can't find mounted perf_event cgroupfs, mounting one")
		err = s.mountPerfEventCgroupFS()
		if err != nil {
			glog.V(1).Info(err)
			// This is not a fatal error condition, proceed on
		}
	}

	// Create the sensor-global event monitor. This EventMonitor instance
	// will be used for all perf_event events
	err = s.createEventMonitor()
	if err != nil {
		s.Stop()
		return err
	}

	s.ContainerCache = NewContainerCache(s)
	s.ProcessCache = NewProcessInfoCache(s)

	if len(config.Sensor.DockerContainerDir) > 0 {
		s.dockerMonitor = newDockerMonitor(s,
			config.Sensor.DockerContainerDir)
	}
	/* Temporarily disable the OCI monitor until a better means of
	   supporting it is found.
	if len(config.Sensor.OciContainerDir) > 0 {
		s.ociMonitor = newOciMonitor(s, config.Sensor.OciContainerDir)
	}
	*/

	// Make sure that all events registered with the sensor's event monitor
	// are active
	s.Monitor.EnableAll()

	return nil
}

// Stop stops a running sensor instance.
func (s *Sensor) Stop() {
	if s.Monitor != nil {
		glog.V(2).Info("Stopping sensor-global EventMonitor")
		s.Monitor.Close()
		s.Monitor = nil
		glog.V(2).Info("Sensor-global EventMonitor stopped successfully")
	}

	if len(s.traceFSMountPoint) > 0 {
		s.unmountTraceFS()
	}

	if len(s.perfEventMountPoint) > 0 {
		s.unmountPerfEventCgroupFS()
	}
}

func (s *Sensor) dispatchSample(eventID uint64, sample perf.EventMonitorSample) {
	if sample.Err != nil {
		glog.Warning(sample.Err)
	}

	event, ok := sample.DecodedSample.(*api.TelemetryEvent)
	if !ok || event == nil {
		return
	}

	eventMap := s.eventMap.getMap()
	subscriptions, ok := eventMap[eventID]
	if !ok {
		return
	}

	for _, s := range subscriptions {
		if s.data == nil {
			continue
		}
		if s.filter != nil {
			v, err := s.filter.Evaluate(
				expression.FieldTypeMap(sample.Fields),
				expression.FieldValueMap(sample.DecodedData))
			if err != nil {
				glog.V(1).Infof("Expression evaluation error: %s", err)
				continue
			}
			if !expression.IsValueTrue(v) {
				continue
			}
		}
		glog.V(2).Infof("Sending %+v", event)
		s.data <- event
	}
}

func (s *Sensor) mountTraceFS() error {
	dir := filepath.Join(config.Global.RunDir, "tracing")
	err := sys.MountTempFS("tracefs", dir, "tracefs", 0, "")
	if err == nil {
		s.traceFSMountPoint = dir
	}
	return err
}

func (s *Sensor) unmountTraceFS() {
	err := sys.UnmountTempFS(s.traceFSMountPoint, "tracefs")
	if err == nil {
		s.traceFSMountPoint = ""
	} else {
		glog.V(2).Infof("Could not unmount %s: %s",
			s.traceFSMountPoint, err)
	}
}

func (s *Sensor) mountPerfEventCgroupFS() error {
	dir := filepath.Join(config.Global.RunDir, "perf_event")
	err := sys.MountTempFS("cgroup", dir, "cgroup", 0, "perf_event")
	if err == nil {
		s.perfEventMountPoint = dir
	}
	return err
}

func (s *Sensor) unmountPerfEventCgroupFS() {
	err := sys.UnmountTempFS(s.perfEventMountPoint, "cgroup")
	if err == nil {
		s.perfEventMountPoint = ""
	} else {
		glog.V(2).Infof("Could not unmount %s: %s",
			s.perfEventMountPoint, err)
	}
}

func (s *Sensor) currentMonotimeNanos() int64 {
	ts := unix.Timespec{}
	unix.ClockGettime(unix.CLOCK_MONOTONIC_RAW, &ts)
	d := ts.Nsec + (ts.Sec * int64(time.Second))
	return d - s.bootMonotimeNanos
}

func (s *Sensor) nextSequenceNumber() uint64 {
	// The first sequence number is intentionally 1 to disambiguate
	// from no sequence number being included in the protobuf message.
	s.sequenceNumber++
	return s.sequenceNumber
}

// NewEvent creates a new API Event instance with common sensor-specific fields
// correctly populated.
func (s *Sensor) NewEvent() *api.TelemetryEvent {
	monotime := s.currentMonotimeNanos()
	sequenceNumber := s.nextSequenceNumber()

	var b []byte
	buf := bytes.NewBuffer(b)

	binary.Write(buf, binary.LittleEndian, s.ID)
	binary.Write(buf, binary.LittleEndian, sequenceNumber)
	binary.Write(buf, binary.LittleEndian, monotime)

	h := sha256.Sum256(buf.Bytes())
	eventID := hex.EncodeToString(h[:])

	s.Metrics.Events++

	return &api.TelemetryEvent{
		Id:                   eventID,
		SensorId:             s.ID,
		SensorMonotimeNanos:  monotime,
		SensorSequenceNumber: sequenceNumber,
	}
}

// NewEventFromContainer creates a new API Event instance using a specific
// container ID.
func (s *Sensor) NewEventFromContainer(containerID string) *api.TelemetryEvent {
	e := s.NewEvent()
	e.ContainerId = containerID
	return e
}

// NewEventFromSample creates a new API Event instance using perf_event sample
// information. If the sample comes from the calling process, no event will be
// created, and the return will be nil.
func (s *Sensor) NewEventFromSample(
	sample *perf.SampleRecord,
	data perf.TraceEventSampleData,
) *api.TelemetryEvent {
	var leader, task *Task

	// When both the sensor and the process generating the sample are in
	// containers, the sample.Pid and sample.Tid fields will be zero.
	// Use "common_pid" from the trace event data instead.
	pid, _ := data["common_pid"].(int32)
	if pid != 0 {
		task, leader = s.ProcessCache.LookupTaskAndLeader(int(pid))
		if leader.IsSensor() {
			return nil
		}
	}

	e := s.NewEvent()
	e.SensorMonotimeNanos = int64(sample.Time) - s.bootMonotimeNanos
	e.Cpu = int32(sample.CPU)

	e.ProcessPid = pid
	if pid != 0 {
		e.ProcessId = leader.ProcessID()
		e.ProcessTgid = int32(task.TGID)

		if c := task.Creds; c != nil {
			e.Credentials = &api.Credentials{
				Uid:   c.UID,
				Gid:   c.GID,
				Euid:  c.EUID,
				Egid:  c.EGID,
				Suid:  c.SUID,
				Sgid:  c.SGID,
				Fsuid: c.FSUID,
				Fsgid: c.FSGID,
			}
		}

		if i := s.ProcessCache.LookupTaskContainerInfo(leader); i != nil {
			e.ContainerId = i.ID
			e.ContainerName = i.Name
			e.ImageId = i.ImageID
			e.ImageName = i.ImageName
		}
	}

	return e
}

func (s *Sensor) buildMonitorGroups() ([]string, []int, error) {
	var (
		cgroupList []string
		pidList    []int
		system     bool
	)

	cgroups := make(map[string]bool)
	for _, cgroup := range config.Sensor.CgroupName {
		if len(cgroup) == 0 || cgroup == "/" {
			system = true
			continue
		}
		if cgroups[cgroup] {
			continue
		}
		cgroups[cgroup] = true
		cgroupList = append(cgroupList, cgroup)
	}

	// Try a system-wide perf event monitor if requested or as
	// a fallback if no cgroups were requested
	if system || len(sys.PerfEventDir()) == 0 || len(cgroupList) == 0 {
		glog.V(1).Info("Creating new system-wide event monitor")
		pidList = append(pidList, -1)
	}

	return cgroupList, pidList, nil
}

func (s *Sensor) createEventMonitor() error {
	eventMonitorOptions := []perf.EventMonitorOption{}

	if len(s.traceFSMountPoint) > 0 {
		eventMonitorOptions = append(eventMonitorOptions,
			perf.WithTracingDir(s.traceFSMountPoint))
	}

	cgroups, pids, err := s.buildMonitorGroups()
	if err != nil {
		return err
	}

	if len(cgroups) == 0 && len(pids) == 0 {
		glog.Fatal("Can't create event monitor with no cgroups or pids")
	}

	if len(cgroups) > 0 {
		var perfEventDir string
		if len(s.perfEventMountPoint) > 0 {
			perfEventDir = s.perfEventMountPoint
		} else {
			perfEventDir = sys.PerfEventDir()
		}
		if len(perfEventDir) > 0 {
			glog.V(1).Infof("Creating new perf event monitor on cgroups %s",
				strings.Join(cgroups, ","))

			eventMonitorOptions = append(eventMonitorOptions,
				perf.WithPerfEventDir(perfEventDir),
				perf.WithCgroups(cgroups))
		}
	}

	if len(pids) > 0 {
		glog.V(1).Info("Creating new system-wide event monitor")
		eventMonitorOptions = append(eventMonitorOptions,
			perf.WithPids(pids))
	}

	s.Monitor, err = perf.NewEventMonitor(eventMonitorOptions...)
	if err != nil {
		// If a cgroup-specific event monitor could not be created,
		// fall back to a system-wide event monitor.
		if len(cgroups) > 0 &&
			(len(pids) == 0 || (len(pids) == 1 && pids[0] == -1)) {

			glog.Warningf("Couldn't create perf event monitor on cgroups %s: %s",
				strings.Join(cgroups, ","), err)

			glog.V(1).Info("Creating new system-wide event monitor")
			s.Monitor, err = perf.NewEventMonitor()
		}
		if err != nil {
			glog.V(1).Infof("Couldn't create event monitor: %s", err)
			return err
		}
	}

	go func() {
		err := s.Monitor.Run(s.dispatchSample)
		if err != nil {
			glog.Fatal(err)
		}
		glog.V(2).Info("EventMonitor.Run() returned; exiting goroutine")
	}()

	return nil
}

func (s *Sensor) createPerfEventStream(sub *api.Subscription) (*stream.Stream, error) {
	eventMap := newSubscriptionMap()

	groupName := fmt.Sprintf("Subscription %p", sub)
	groupID, err := s.Monitor.RegisterEventGroup(groupName)
	if err != nil {
		return nil, err
	}

	registerContainerEvents(s, groupID, eventMap, sub.EventFilter.ContainerEvents)
	registerFileEvents(s, groupID, eventMap, sub.EventFilter.FileEvents)
	registerKernelEvents(s, groupID, eventMap, sub.EventFilter.KernelEvents)
	registerNetworkEvents(s, groupID, eventMap, sub.EventFilter.NetworkEvents)
	registerProcessEvents(s, groupID, eventMap, sub.EventFilter.ProcessEvents)
	registerSyscallEvents(s, groupID, eventMap, sub.EventFilter.SyscallEvents)
	registerChargenEvents(s, groupID, eventMap, sub.EventFilter.ChargenEvents)
	registerTimerEvents(s, groupID, eventMap, sub.EventFilter.TickerEvents)

	if len(eventMap) == 0 {
		return nil, errors.New("Invalid subscription (no filters specified)")
	}

	ctrl := make(chan interface{})
	data := make(chan interface{}, config.Sensor.ChannelBufferLength)

	eventMap.forEach(func(eventID, subscriptionID uint64, s *subscription) {
		s.data = data
	})
	subscriptionID := s.eventMap.subscribe(eventMap)
	glog.V(2).Infof("Subscription %d registered", subscriptionID)

	go func() {
		defer close(data)

		for {
			if _, ok := <-ctrl; !ok {
				glog.V(2).Infof("Subscription %d control channel closed",
					subscriptionID)

				s.Monitor.UnregisterEventGroup(groupID)
				s.eventMap.unsubscribe(subscriptionID, nil)
				return
			}
		}
	}()

	s.Monitor.EnableGroup(groupID)

	return &stream.Stream{
		Ctrl: ctrl,
		Data: data,
	}, nil
}

func (s *Sensor) applyModifiers(eventStream *stream.Stream, modifier api.Modifier) *stream.Stream {
	if modifier.Throttle != nil {
		eventStream = stream.Throttle(eventStream, *modifier.Throttle)
	}

	if modifier.Limit != nil {
		eventStream = stream.Limit(eventStream, *modifier.Limit)
	}

	return eventStream
}

// NewSubscription creates a new telemetry subscription from the given
// api.Subscription descriptor. NewSubscription returns a stream.Stream of
// api.Events matching the specified filters. Closing the Stream cancels the
// subscription.
func (s *Sensor) NewSubscription(sub *api.Subscription) (*stream.Stream, error) {
	glog.V(1).Infof("Subscribing to %+v", sub)
	if sub.EventFilter == nil {
		return nil, errors.New("Invalid subscription (no EventFilter)")
	}

	eventStream, err := s.createPerfEventStream(sub)
	if err != nil {
		return nil, err
	}

	if sub.ContainerFilter != nil {
		// Filter stream as requested by subscriber in the
		// specified ContainerFilter to restrict the events to
		// those matching the specified container ids, names,
		// images, etc.
		cef := newContainerFilter(sub.ContainerFilter)
		eventStream = stream.Filter(eventStream, cef.FilterFunc)
		eventStream = stream.Do(eventStream, cef.DoFunc)
	}

	if sub.Modifier != nil {
		eventStream = s.applyModifiers(eventStream, *sub.Modifier)
	}

	s.Metrics.Subscriptions++

	return eventStream, nil
}
