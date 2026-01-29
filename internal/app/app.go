package app

import "sync"

var once sync.Once
var app *App

type App struct {
	// Add fields as necessary
}
