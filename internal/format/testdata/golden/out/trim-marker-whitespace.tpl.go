{{ range .Fields }}
	{{ .Name }}  {{- .Tag }} {{ .Type }}
{{ end }}
