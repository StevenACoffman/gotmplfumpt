package main

func F() int {
	{{ if .Positive }}
	return 1
	{{ else }}
	return -1
	{{ end }}
}
