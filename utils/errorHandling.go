package utils

// This is meant to simplify error checking throughout the application.
// It may also be expanded upon later for more specific error checking or handling and better logging of errors

// Will panic if error is not nil
func PanicCheck(err error) {
	if err != nil {
		panic(err)
	}
}
