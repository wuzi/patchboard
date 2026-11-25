package auth

// ArgonPeak reports the highest number of argon2 computations seen running at once.
func ArgonPeak() int64 { return argonPeak.Load() }

// ResetArgonPeak clears the peak counter.
func ResetArgonPeak() { argonPeak.Store(0) }
