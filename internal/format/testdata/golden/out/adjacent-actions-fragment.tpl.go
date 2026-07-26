{{ range .Events }}{{ range . }}
		type {{ .Name }} struct{}
{{ end }}{{ end }}
