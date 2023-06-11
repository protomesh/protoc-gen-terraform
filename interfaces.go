package main

type selectorMaker interface {
	makeSelector(string) string
}

type mapIndexMaker interface {
	makeMapIndex(string) string
}
