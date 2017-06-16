package container

import "github.com/capsule8/reactive8/pkg/stream"

// State represents the state of a container instance
type State uint

// Possible states for a container instance
const (
	_ State = iota
	ContainerCreated
	ContainerStarted
	ContainerStopped
	ContainerRemoved
)

// Event represents a container lifecycle event containing fields common
// to all supported container runtimes.
type Event struct {
	ID    string
	State State

	ImageID string
	Image   string
	Pid     uint32
	Cgroup  string
}

func processEvents(e interface{}) interface{} {
	var ev *Event

	switch e.(type) {
	case *dockerEvent:
		e := e.(*dockerEvent)
		if e.State == dockerContainerCreated {
			ev = &Event{
				ID:    e.ID,
				State: ContainerCreated,

				ImageID: e.ImageID,
				Image:   e.Image,
			}

		} else if e.State == dockerContainerRunning {
			//
			// Even though we can also trigger the STARTED
			// state on ociContainerRunning, this event
			// happens first, so we use it. If the subscriber
			// needs data that's in the OCI config, then we'll
			// need to delay the event for it.
			//
			ev = &Event{
				ID:    e.ID,
				State: ContainerStarted,

				ImageID: e.ImageID,
				Image:   e.Image,
				Pid:     uint32(e.Pid),
			}

		} else if e.State == dockerContainerDead {
			ev = &Event{
				ID:    e.ID,
				State: ContainerRemoved,

				ImageID: e.ImageID,
				Image:   e.Image,
			}
		}

	case *ociEvent:
		e := e.(*ociEvent)
		if e.State == ociRunning {
			ev = &Event{
				ID:     e.ID,
				State:  ContainerStarted,
				Image:  e.Image,
				Cgroup: e.CgroupsPath,
			}

		} else if e.State == ociStopped {
			ev = &Event{
				ID:    e.ID,
				State: ContainerStopped,
			}
		}
	}

	return ev

}

func filterNils(e interface{}) bool {
	if e != nil {
		ev := e.(*Event)
		return ev != nil
	}

	return e != nil
}

// NewEventStream creates a new stream of container lifecycle
// events.
func NewEventStream() (*stream.Stream, error) {
	//
	// Join upstream Docker and OCI container event streams
	//
	dockerEvents, err := NewDockerEventStream()
	if err != nil {
		return nil, err
	}

	ociEvents, err := NewOciEventStream()
	if err != nil {
		return nil, err
	}

	s := stream.Join(dockerEvents, ociEvents)
	s = stream.Map(s, processEvents)
	s = stream.Filter(s, filterNils)

	return s, nil

}