package vos

import (
	"log"
)

const debugLog = false

func l(fname, path string) {
	if debugLog {
		log.Println(fname, path)
	}
}
