package proc

// xrplEpochToUnixEpoch converts XPRL epoch time (January 1, 2000)
// to Unix epoch time (January 1, 1970).
func xrplEpochToUnixEpoch(xrplEpoch uint64) uint64 {
	const offset = 946684800
	return xrplEpoch + offset
}
