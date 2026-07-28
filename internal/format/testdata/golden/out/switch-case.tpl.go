{{ reserveImport "fmt" }}
func (e Color) IsValid() bool {
	switch e {
	case ColorRed, ColorBlue:
		return true
	default:
		return false
	}
}
