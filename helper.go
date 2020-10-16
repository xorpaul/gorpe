package main

import (
	"net/http"
	"strconv"
)

// Exit is a helper function to output a check result in a standardized way
func (cr checkResult) Exit(w http.ResponseWriter) {
	if !(cr.returncode != 0 || cr.returncode != 1 || cr.returncode != 2 || cr.returncode != 3) {
		cr.returncode = 3
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(cr.text + "\nResult Code: " + strconv.Itoa(cr.returncode) + "\n"))
	return
}
