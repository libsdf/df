package log

type Handler interface {
	Format() string
	SetFormat(string)
	Write(*Message)
	CleanUp()
}
