package media

import "time"

// seconds converts a floating-point number of seconds to a Duration.
func seconds(s float64) time.Duration { return time.Duration(s * float64(time.Second)) }
