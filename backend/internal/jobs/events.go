package jobs

// Event types this package publishes on the hub.
const (
	EventCreated  = "job.created"
	EventProgress = "job.progress"
	EventState    = "job.state"
	EventConflict = "job.conflict"
	EventDone     = "job.done"
)
