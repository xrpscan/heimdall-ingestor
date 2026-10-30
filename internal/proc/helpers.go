package proc

// stringUpto returns the first n bytes of the given string.
func stringUpto(str string, n int) string {
	if len(str) < n {
		return str
	}

	return str[:n]
}
