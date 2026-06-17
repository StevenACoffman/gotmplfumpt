package main

import (
{{ range .Imports }}
"{{ . }}"
{{ end }}
)

func F() {}
