package xrpld

// stringUpto returns the first n bytes of the given string.
func stringUpto(str string, n int64) string {
	if int64(len(str)) < n {
		return str
	}

	return str[:n]
}
