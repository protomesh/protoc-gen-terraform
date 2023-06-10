package main

type selectorMaker interface {
	makeSelector(string) string
}
