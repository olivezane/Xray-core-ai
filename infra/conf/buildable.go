package conf

// Buildable is what a section of the configuration model does to become the
// value the runtime uses. The value it builds is not always a protobuf message:
// a setting that is a plain Go value names itself and crosses the API as JSON.
type Buildable interface {
	Build() (any, error)
}
