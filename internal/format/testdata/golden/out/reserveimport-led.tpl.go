{{ reserveImport "fmt" }}
{{ reserveImport "io" }}

func Greet(w io.Writer) {
	fmt.Fprintln(w, "hi {{ .Name }}")
}
