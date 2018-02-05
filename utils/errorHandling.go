package utils

// Will panic if error is not nil
func PanicCheck(err error) {
	if err != nil {
		panic(err)
	}
}
